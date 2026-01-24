package handlers

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/p2p"
	"github.com/blockberries/blockberry/types"
)

// ConsensusHandler defines the interface for handling consensus messages.
// Applications implement this interface to receive consensus messages from peers.
type ConsensusHandler interface {
	// HandleConsensusMessage processes an incoming consensus message from a peer.
	HandleConsensusMessage(peerID peer.ID, data []byte) error
}

// ConsensusReactor routes consensus messages between peers and the application.
// It acts as a pass-through, delegating message handling to the registered handler.
type ConsensusReactor struct {
	// Dependencies
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// Application handler
	handler ConsensusHandler

	mu sync.RWMutex
}

// NewConsensusReactor creates a new consensus reactor.
func NewConsensusReactor(
	network *p2p.Network,
	peerManager *p2p.PeerManager,
) *ConsensusReactor {
	return &ConsensusReactor{
		network:     network,
		peerManager: peerManager,
	}
}

// SetHandler sets the consensus handler for incoming messages.
func (r *ConsensusReactor) SetHandler(handler ConsensusHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
}

// GetHandler returns the current consensus handler.
func (r *ConsensusReactor) GetHandler() ConsensusHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handler
}

// HandleMessage processes an incoming consensus message.
// It passes the message directly to the registered handler.
func (r *ConsensusReactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	r.mu.RLock()
	handler := r.handler
	r.mu.RUnlock()

	if handler == nil {
		// No handler registered - silently ignore
		return nil
	}

	return handler.HandleConsensusMessage(peerID, data)
}

// SendConsensusMessage sends a consensus message to a specific peer.
func (r *ConsensusReactor) SendConsensusMessage(peerID peer.ID, data []byte) error {
	if r.network == nil {
		return nil
	}

	return r.network.Send(peerID, p2p.StreamConsensus, data)
}

// BroadcastConsensusMessage sends a consensus message to all connected peers.
func (r *ConsensusReactor) BroadcastConsensusMessage(data []byte) error {
	if r.network == nil || r.peerManager == nil {
		return nil
	}

	peers := r.peerManager.GetConnectedPeers()
	for _, peerID := range peers {
		if err := r.network.Send(peerID, p2p.StreamConsensus, data); err != nil {
			// Continue trying other peers
			continue
		}
	}

	return nil
}

// OnPeerDisconnected is called when a peer disconnects.
// ConsensusReactor doesn't maintain per-peer state, so this is a no-op.
func (r *ConsensusReactor) OnPeerDisconnected(peerID peer.ID) {
	// No per-peer state to clean up
}
