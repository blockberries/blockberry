package node

import (
	"errors"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/internal/p2p"
)

// NetworkBridge provides a public interface to the node's network layer.
// It implements mempool.MempoolNetwork and consensus.Network, allowing
// external modules to send and broadcast messages without importing
// internal packages.
type NetworkBridge struct {
	network *p2p.Network
}

func newNetworkBridge(network *p2p.Network) *NetworkBridge {
	return &NetworkBridge{network: network}
}

// Send sends data to a specific peer on a stream.
func (b *NetworkBridge) Send(peerID peer.ID, stream string, data []byte) error {
	return b.network.Send(peerID, stream, data)
}

// Broadcast sends data to all connected peers on a stream.
// Errors from individual peer sends are joined into a single error.
func (b *NetworkBridge) Broadcast(stream string, data []byte) error {
	errs := b.network.Broadcast(stream, data)
	return errors.Join(errs...)
}

// ConnectedPeers returns the list of connected peer IDs.
func (b *NetworkBridge) ConnectedPeers() []peer.ID {
	return b.network.ConnectedPeers()
}
