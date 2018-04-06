package p2p

import (
	p2phost "gx/ipfs/QmNmJZL7FQySMtE2BQuLMuZg2EB2CLEunJJUSVSc9YnnbV/go-libp2p-host"
	pstore "gx/ipfs/QmXauCuJzmzapetmC6W4TuDJLL1yFFrVzSHoWv8YdbmnxH/go-libp2p-peerstore"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
)

// P2P structure holds information on currently running streams/listeners
type P2P struct {
	Listeners ListenerRegistry
	Streams   StreamRegistry

	identity  peer.ID
	peerHost  p2phost.Host
	peerstore pstore.Peerstore
}

// CheckProtoExists checks whether a proto handler is registered to
// mux handler
func (p2p *P2P) CheckProtoExists(proto string) bool {
	protos := p2p.peerHost.Mux().Protocols()

	for _, p := range protos {
		if p != proto {
			continue
		}
		return true
	}
	return false
}
