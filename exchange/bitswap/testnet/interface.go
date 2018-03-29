package bitswap

import (
	bsnet "github.com/ipfs/go-ipfs/exchange/bitswap/network"
	"gx/ipfs/QmW1vsTgpWmkmyopPPVfXcMjbR9qEkzDyeZgF9TXgZx8PU/go-testutil"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

type Network interface {
	Adapter(testutil.Identity) bsnet.BitSwapNetwork

	HasPeer(peer.ID) bool
}
