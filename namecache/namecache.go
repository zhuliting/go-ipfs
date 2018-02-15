// Package namecache implements background following (resolution and pinning) of names
package namecache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	bstore "github.com/ipfs/go-ipfs/blocks/blockstore"
	namesys "github.com/ipfs/go-ipfs/namesys"
	path "github.com/ipfs/go-ipfs/path"
	pin "github.com/ipfs/go-ipfs/pin"
	uio "github.com/ipfs/go-ipfs/unixfs/io"

	logging "gx/ipfs/QmRb5jh8z2E8hMGN2tkvs1yHynUanqnZ3UeKwgN1i9P1F8/go-log"
	cid "gx/ipfs/QmcZfnkapfECQGcLZaf9B79NRg7cRa9EnZh4LSbkCzwNvY/go-cid"
	ipld "gx/ipfs/Qme5bWv7wtjUNGsK2BNGVUFPKiuxWrsqrtvYwCLRw8YFES/go-ipld-format"
)

const (
	DefaultFollowInterval = 1 * time.Hour
	resolveTimeout        = 1 * time.Minute
)

var log = logging.Logger("namecache")

// NameCache represents a following cache of names
type NameCache interface {
	// Follow starts following name, pinning it if dopin is true
	Follow(name string, dopin bool, followInterval time.Duration) error
	// Unofollow cancels a follow
	Unfollow(name string) error
	// ListFollows returns a list of followed names
	ListFollows() []string
}

type nameCache struct {
	nsys    namesys.NameSystem
	pinning pin.Pinner
	dag     ipld.NodeGetter
	bstore  bstore.GCBlockstore

	ctx     context.Context
	follows map[string]func()
	mx      sync.Mutex
}

func NewNameCache(ctx context.Context, nsys namesys.NameSystem, pinning pin.Pinner, dag ipld.NodeGetter, bstore bstore.GCBlockstore) NameCache {
	return &nameCache{
		ctx:     ctx,
		nsys:    nsys,
		pinning: pinning,
		dag:     dag,
		bstore:  bstore,
		follows: make(map[string]func()),
	}
}

// Follow spawns a goroutine that periodically resolves a name
// and (when dopin is true) pins it in the background
func (nc *nameCache) Follow(name string, dopin bool, followInterval time.Duration) error {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	if _, ok := nc.follows[name]; ok {
		return fmt.Errorf("Already following %s", name)
	}

	ctx, cancel := context.WithCancel(nc.ctx)
	go nc.followName(ctx, name, dopin, followInterval)
	nc.follows[name] = cancel

	return nil
}

// Unfollow cancels a follow
func (nc *nameCache) Unfollow(name string) error {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	if !strings.HasPrefix(name, "/ipns/") {
		name = "/ipns/" + name
	}

	cancel, ok := nc.follows[name]
	if !ok {
		return fmt.Errorf("Unknown name %s", name)
	}

	cancel()
	delete(nc.follows, name)
	return nil
}

// ListFollows returns a list of names currently being followed
func (nc *nameCache) ListFollows() []string {
	nc.mx.Lock()
	defer nc.mx.Unlock()

	follows := make([]string, 0, len(nc.follows))
	for name, _ := range nc.follows {
		follows = append(follows, name)
	}

	return follows
}

func (nc *nameCache) followName(ctx context.Context, name string, dopin bool, followInterval time.Duration) {
	// if cid != nil, we have created a new pin that is updated on changes and
	// unpinned on cancel
	cid, err := nc.resolveAndPin(ctx, name, dopin)
	if err != nil {
		log.Errorf("Error following %s: %s", name, err.Error())
	}

	ticker := time.NewTicker(followInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if cid != nil {
				cid, err = nc.resolveAndUpdate(ctx, name, cid)
			} else {
				cid, err = nc.resolveAndPin(ctx, name, dopin)
			}

			if err != nil {
				log.Errorf("Error following %s: %s", name, err.Error())
			}

		case <-ctx.Done():
			if cid != nil {
				err = nc.unpin(cid)
				if err != nil {
					log.Errorf("Error unpinning %s: %s", name, err.Error())
				}
			}
			return
		}
	}
}

func (nc *nameCache) resolveAndPin(ctx context.Context, name string, dopin bool) (*cid.Cid, error) {
	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return nil, err
	}

	if !dopin {
		return nil, nil
	}

	cid, err := pathToCid(ptr)
	if err != nil {
		return nil, err
	}

	defer nc.bstore.PinLock().Unlock()

	_, pinned, err := nc.pinning.IsPinned(cid)
	if pinned || err != nil {
		return nil, err
	}

	n, err := nc.pathToNode(ctx, ptr)
	if err != nil {
		return nil, err
	}

	log.Debugf("pinning %s", cid.String())

	err = nc.pinning.Pin(ctx, n, true)
	if err != nil {
		return nil, err
	}

	err = nc.pinning.Flush()

	return cid, err
}

func (nc *nameCache) resolveAndUpdate(ctx context.Context, name string, oldcid *cid.Cid) (*cid.Cid, error) {

	ptr, err := nc.resolve(ctx, name)
	if err != nil {
		return nil, err
	}

	newcid, err := pathToCid(ptr)
	if err != nil {
		return nil, err
	}

	if newcid.Equals(oldcid) {
		return oldcid, nil
	}

	defer nc.bstore.PinLock().Unlock()

	log.Debugf("Updating pin %s -> %s", oldcid.String(), newcid.String())

	err = nc.pinning.Update(ctx, oldcid, newcid, true)
	if err != nil {
		return oldcid, err
	}

	err = nc.pinning.Flush()

	return newcid, err
}

func (nc *nameCache) unpin(cid *cid.Cid) error {
	defer nc.bstore.PinLock().Unlock()

	err := nc.pinning.Unpin(nc.ctx, cid, true)
	if err != nil {
		return err
	}

	return nc.pinning.Flush()
}

func (nc *nameCache) resolve(ctx context.Context, name string) (path.Path, error) {
	log.Debugf("resolving %s", name)

	rctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	p, err := nc.nsys.Resolve(rctx, name)
	if err != nil {
		return "", err
	}

	log.Debugf("resolved %s to %s", name, p)

	return p, nil
}

func pathToCid(p path.Path) (*cid.Cid, error) {
	return cid.Decode(p.Segments()[1])
}

func (nc *nameCache) pathToNode(ctx context.Context, p path.Path) (ipld.Node, error) {
	r := &path.Resolver{
		DAG:         nc.dag,
		ResolveOnce: uio.ResolveUnixfsOnce,
	}

	return r.ResolvePath(ctx, p)
}
