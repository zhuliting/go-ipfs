package bitswap

import (
	"context"

	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"

	testutil "gx/ipfs/QmW1vsTgpWmkmyopPPVfXcMjbR9qEkzDyeZgF9TXgZx8PU/go-testutil"
	mockpeernet "gx/ipfs/QmY4UBVbJ8mv1PnXsZXFYwJY3C3NqWm4XWSDoUcSBgJrfB/go-libp2p/p2p/net/mock"
	mockrouting "gx/ipfs/QmYWUgR4in6E2ooANpHQDYejuoWvVCLe8sCtuTaCPhgvhP/go-ipfs-routing/mock"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	ds "gx/ipfs/QmeiCcJfDW1GJnWUArudsv5rQsihpi4oyddPhdqo3CfX6i/go-datastore"
)

type peernet struct {
	mockpeernet.Mocknet
	routingserver mockrouting.Server
}

func StreamNet(ctx context.Context, net mockpeernet.Mocknet, rs mockrouting.Server) (Network, error) {
	return &peernet{net, rs}, nil
}

func (pn *peernet) Adapter(p testutil.Identity) bsnet.BitSwapNetwork {
	client, err := pn.Mocknet.AddPeer(p.PrivateKey(), p.Address())
	if err != nil {
		panic(err.Error())
	}
	routing := pn.routingserver.ClientWithDatastore(context.TODO(), p, ds.NewMapDatastore())
	return bsnet.NewFromIpfsHost(client, routing)
}

func (pn *peernet) HasPeer(p peer.ID) bool {
	for _, member := range pn.Mocknet.Peers() {
		if p == member {
			return true
		}
	}
	return false
}

var _ Network = (*peernet)(nil)
