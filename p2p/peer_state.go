// Package p2p provides peer-to-peer networking for the blockchain node.
package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// PeerState tracks the state of a connected peer including
// exchanged transactions, blocks, and scoring information.
type PeerState struct {
	// Identity
	PeerID    peer.ID
	PublicKey []byte

	// Exchange tracking - what we've sent/received to avoid duplicates
	TxsSent        map[string]bool
	TxsReceived    map[string]bool
	BlocksSent     map[int64]bool
	BlocksReceived map[int64]bool

	// Scoring
	Score         int64
	PenaltyPoints int64

	// Timing
	LastSeen    time.Time
	Latency     time.Duration
	ConnectedAt time.Time

	// Connection info
	IsSeed     bool
	IsOutbound bool

	mu sync.RWMutex
}

// NewPeerState creates a new peer state for a connected peer.
func NewPeerState(peerID peer.ID, isOutbound bool) *PeerState {
	now := time.Now()
	return &PeerState{
		PeerID:         peerID,
		TxsSent:        make(map[string]bool),
		TxsReceived:    make(map[string]bool),
		BlocksSent:     make(map[int64]bool),
		BlocksReceived: make(map[int64]bool),
		LastSeen:       now,
		ConnectedAt:    now,
		IsOutbound:     isOutbound,
	}
}

// SetPublicKey sets the peer's public key after handshake.
func (ps *PeerState) SetPublicKey(pubKey []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.PublicKey = pubKey
}

// MarkTxSent records that we sent a transaction to this peer.
func (ps *PeerState) MarkTxSent(txHash []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.TxsSent[string(txHash)] = true
}

// MarkTxReceived records that we received a transaction from this peer.
func (ps *PeerState) MarkTxReceived(txHash []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.TxsReceived[string(txHash)] = true
}

// MarkBlockSent records that we sent a block to this peer.
func (ps *PeerState) MarkBlockSent(height int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.BlocksSent[height] = true
}

// MarkBlockReceived records that we received a block from this peer.
func (ps *PeerState) MarkBlockReceived(height int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.BlocksReceived[height] = true
}

// ShouldSendTx returns true if we should send a transaction to this peer.
// Returns false if we've already sent it or received it from them.
func (ps *PeerState) ShouldSendTx(txHash []byte) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	key := string(txHash)
	return !ps.TxsSent[key] && !ps.TxsReceived[key]
}

// ShouldSendBlock returns true if we should send a block to this peer.
// Returns false if we've already sent it or received it from them.
func (ps *PeerState) ShouldSendBlock(height int64) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return !ps.BlocksSent[height] && !ps.BlocksReceived[height]
}

// HasTx returns true if we've exchanged this transaction with the peer.
func (ps *PeerState) HasTx(txHash []byte) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	key := string(txHash)
	return ps.TxsSent[key] || ps.TxsReceived[key]
}

// HasBlock returns true if we've exchanged this block with the peer.
func (ps *PeerState) HasBlock(height int64) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.BlocksSent[height] || ps.BlocksReceived[height]
}

// UpdateLastSeen updates the last seen timestamp.
func (ps *PeerState) UpdateLastSeen() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.LastSeen = time.Now()
}

// UpdateLatency updates the measured latency.
func (ps *PeerState) UpdateLatency(latency time.Duration) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.Latency = latency
	ps.LastSeen = time.Now()
}

// AddPenalty adds penalty points to this peer.
func (ps *PeerState) AddPenalty(points int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.PenaltyPoints += points
}

// DecayPenalty reduces penalty points by the given amount.
func (ps *PeerState) DecayPenalty(points int64) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.PenaltyPoints -= points
	if ps.PenaltyPoints < 0 {
		ps.PenaltyPoints = 0
	}
}

// GetPenaltyPoints returns the current penalty points.
func (ps *PeerState) GetPenaltyPoints() int64 {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.PenaltyPoints
}

// GetLatency returns the measured latency.
func (ps *PeerState) GetLatency() time.Duration {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.Latency
}

// GetLastSeen returns the last seen timestamp.
func (ps *PeerState) GetLastSeen() time.Time {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.LastSeen
}

// ConnectionDuration returns how long this peer has been connected.
func (ps *PeerState) ConnectionDuration() time.Duration {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return time.Since(ps.ConnectedAt)
}

// TxsSentCount returns the number of transactions sent to this peer.
func (ps *PeerState) TxsSentCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.TxsSent)
}

// TxsReceivedCount returns the number of transactions received from this peer.
func (ps *PeerState) TxsReceivedCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.TxsReceived)
}

// BlocksSentCount returns the number of blocks sent to this peer.
func (ps *PeerState) BlocksSentCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.BlocksSent)
}

// BlocksReceivedCount returns the number of blocks received from this peer.
func (ps *PeerState) BlocksReceivedCount() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return len(ps.BlocksReceived)
}

// SetSeed marks this peer as a seed node.
func (ps *PeerState) SetSeed(isSeed bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.IsSeed = isSeed
}
