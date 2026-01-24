package p2p

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/types"
)

// PeerManager manages all connected peers and their states.
// All public methods are thread-safe.
type PeerManager struct {
	peers map[peer.ID]*PeerState
	mu    sync.RWMutex
}

// NewPeerManager creates a new peer manager.
func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers: make(map[peer.ID]*PeerState),
	}
}

// AddPeer adds a new peer to the manager.
// Returns the peer state for further configuration.
func (pm *PeerManager) AddPeer(peerID peer.ID, isOutbound bool) *PeerState {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// If peer already exists, return existing state
	if state, exists := pm.peers[peerID]; exists {
		return state
	}

	state := NewPeerState(peerID, isOutbound)
	pm.peers[peerID] = state
	return state
}

// RemovePeer removes a peer from the manager.
func (pm *PeerManager) RemovePeer(peerID peer.ID) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.peers, peerID)
}

// GetPeer returns the peer state for a given peer ID.
// Returns nil if the peer is not found.
func (pm *PeerManager) GetPeer(peerID peer.ID) *PeerState {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.peers[peerID]
}

// HasPeer returns true if the peer exists in the manager.
func (pm *PeerManager) HasPeer(peerID peer.ID) bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	_, exists := pm.peers[peerID]
	return exists
}

// PeerCount returns the number of connected peers.
func (pm *PeerManager) PeerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.peers)
}

// AllPeers returns a copy of all peer states.
func (pm *PeerManager) AllPeers() []*PeerState {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	peers := make([]*PeerState, 0, len(pm.peers))
	for _, state := range pm.peers {
		peers = append(peers, state)
	}
	return peers
}

// AllPeerIDs returns all peer IDs.
func (pm *PeerManager) AllPeerIDs() []peer.ID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	ids := make([]peer.ID, 0, len(pm.peers))
	for id := range pm.peers {
		ids = append(ids, id)
	}
	return ids
}

// MarkTxSent marks a transaction as sent to a specific peer.
func (pm *PeerManager) MarkTxSent(peerID peer.ID, txHash []byte) error {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return types.ErrPeerNotFound
	}
	state.MarkTxSent(txHash)
	return nil
}

// MarkTxReceived marks a transaction as received from a specific peer.
func (pm *PeerManager) MarkTxReceived(peerID peer.ID, txHash []byte) error {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return types.ErrPeerNotFound
	}
	state.MarkTxReceived(txHash)
	return nil
}

// MarkBlockSent marks a block as sent to a specific peer.
func (pm *PeerManager) MarkBlockSent(peerID peer.ID, height int64) error {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return types.ErrPeerNotFound
	}
	state.MarkBlockSent(height)
	return nil
}

// MarkBlockReceived marks a block as received from a specific peer.
func (pm *PeerManager) MarkBlockReceived(peerID peer.ID, height int64) error {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return types.ErrPeerNotFound
	}
	state.MarkBlockReceived(height)
	return nil
}

// ShouldSendTx returns true if we should send a transaction to this peer.
func (pm *PeerManager) ShouldSendTx(peerID peer.ID, txHash []byte) bool {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return false
	}
	return state.ShouldSendTx(txHash)
}

// ShouldSendBlock returns true if we should send a block to this peer.
func (pm *PeerManager) ShouldSendBlock(peerID peer.ID, height int64) bool {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return false
	}
	return state.ShouldSendBlock(height)
}

// PeersToSendTx returns all peers that should receive a transaction.
func (pm *PeerManager) PeersToSendTx(txHash []byte) []peer.ID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []peer.ID
	for id, state := range pm.peers {
		if state.ShouldSendTx(txHash) {
			result = append(result, id)
		}
	}
	return result
}

// PeersToSendBlock returns all peers that should receive a block.
func (pm *PeerManager) PeersToSendBlock(height int64) []peer.ID {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []peer.ID
	for id, state := range pm.peers {
		if state.ShouldSendBlock(height) {
			result = append(result, id)
		}
	}
	return result
}

// UpdateLastSeen updates the last seen timestamp for a peer.
func (pm *PeerManager) UpdateLastSeen(peerID peer.ID) {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state != nil {
		state.UpdateLastSeen()
	}
}

// SetPublicKey sets the public key for a peer after handshake.
func (pm *PeerManager) SetPublicKey(peerID peer.ID, pubKey []byte) error {
	pm.mu.RLock()
	state := pm.peers[peerID]
	pm.mu.RUnlock()

	if state == nil {
		return types.ErrPeerNotFound
	}
	state.SetPublicKey(pubKey)
	return nil
}
