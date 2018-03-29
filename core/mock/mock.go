package coremock

import (
	"context"

	commands "github.com/ipfs/go-ipfs/commands"
	core "github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"

	pstore "gx/ipfs/QmT1hUXbRnjpWxGWAuRXUiVeyU5yrA7HNFieUBqUDcfgYm/go-libp2p-peerstore"
	testutil "gx/ipfs/QmW1vsTgpWmkmyopPPVfXcMjbR9qEkzDyeZgF9TXgZx8PU/go-testutil"
	libp2p "gx/ipfs/QmY4UBVbJ8mv1PnXsZXFYwJY3C3NqWm4XWSDoUcSBgJrfB/go-libp2p"
	mocknet "gx/ipfs/QmY4UBVbJ8mv1PnXsZXFYwJY3C3NqWm4XWSDoUcSBgJrfB/go-libp2p/p2p/net/mock"
	host "gx/ipfs/QmZLmWTC8eS1LojwosxjZpyDJtuTZx9bUVb2ZWmCm7hkT5/go-libp2p-host"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	datastore "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
	syncds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore/sync"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	ctx := context.Background()

	// effectively offline, only peer in its network
	return core.NewNode(ctx, &core.BuildCfg{
		Online: true,
		Host:   MockHostOption(mocknet.New(ctx)),
	})
}

func MockHostOption(mn mocknet.Mocknet) core.HostOption {
	return func(ctx context.Context, id peer.ID, ps pstore.Peerstore, _ ...libp2p.Option) (host.Host, error) {
		return mn.AddPeerWithPeerstore(id, ps)
	}
}

func MockCmdsCtx() (commands.Context, error) {
	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return commands.Context{}, err
	}
	p := ident.ID()

	conf := config.Config{
		Identity: config.Identity{
			PeerID: p.String(),
		},
	}

	r := &repo.Mock{
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
		C: conf,
	}

	node, err := core.NewNode(context.Background(), &core.BuildCfg{
		Repo: r,
	})
	if err != nil {
		return commands.Context{}, err
	}

	return commands.Context{
		Online:     true,
		ConfigRoot: "/tmp/.mockipfsconfig",
		LoadConfig: func(path string) (*config.Config, error) {
			return &conf, nil
		},
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, nil
		},
	}, nil
}
