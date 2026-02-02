// Package p2p provides peer-to-peer networking for the blockchain node.
package p2p

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

// LRU cache size limits to prevent unbounded memory growth.
// These limits ensure that even long-running nodes with high traffic
// don't consume excessive memory tracking transaction/block history.
const (
	// MaxKnownTxsPerPeer is the maximum number of transaction hashes to track
	// per peer for deduplication. At ~32 bytes per hash, this is ~640KB per peer.
	MaxKnownTxsPerPeer = 20000

	// MaxKnownBlocksPerPeer is the maximum number of block heights to track
	// per peer. Blocks are referenced by height (int64), so this is minimal memory.
	MaxKnownBlocksPerPeer = 2000
)

// PeerState tracks the state of a connected peer including
// exchanged transactions, blocks, and scoring information.
// All public methods are thread-safe.
type PeerState struct {
	// Identity
	PeerID    peer.ID
	PublicKey []byte

	// Exchange tracking - LRU caches to avoid unbounded memory growth
	// These track what we've sent/received to avoid duplicate sends.
	txsSent        *lru.Cache[string, struct{}]
	txsReceived    *lru.Cache[string, struct{}]
	blocksSent     *lru.Cache[int64, struct{}]
	blocksReceived *lru.Cache[int64, struct{}]

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
// Panics if LRU cache creation fails (indicates programming error with invalid size constants).
func NewPeerState(peerID peer.ID, isOutbound bool) *PeerState {
	now := time.Now()

	// Create LRU caches - panic on failure since it indicates invalid configuration
	txsSent, err := lru.New[string, struct{}](MaxKnownTxsPerPeer)
	if err != nil {
		panic("failed to create txsSent LRU cache: " + err.Error())
	}
	txsReceived, err := lru.New[string, struct{}](MaxKnownTxsPerPeer)
	if err != nil {
		panic("failed to create txsReceived LRU cache: " + err.Error())
	}
	blocksSent, err := lru.New[int64, struct{}](MaxKnownBlocksPerPeer)
	if err != nil {
		panic("failed to create blocksSent LRU cache: " + err.Error())
	}
	blocksReceived, err := lru.New[int64, struct{}](MaxKnownBlocksPerPeer)
	if err != nil {
		panic("failed to create blocksReceived LRU cache: " + err.Error())
	}

	return &PeerState{
		PeerID:         peerID,
		txsSent:        txsSent,
		txsReceived:    txsReceived,
		blocksSent:     blocksSent,
		blocksReceived: blocksReceived,
		LastSeen:       now,
		ConnectedAt:    now,
		IsOutbound:     isOutbound,
	}
}

// SetPublicKey sets the peer's public key after handshake.
// Makes a defensive copy of the provided slice.
func (ps *PeerState) SetPublicKey(pubKey []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.PublicKey = append([]byte(nil), pubKey...)
}

// MarkTxSent records that we sent a transaction to this peer.
func (ps *PeerState) MarkTxSent(txHash []byte) {
	// LRU cache is thread-safe, no lock needed
	ps.txsSent.Add(string(txHash), struct{}{})
}

// MarkTxReceived records that we received a transaction from this peer.
func (ps *PeerState) MarkTxReceived(txHash []byte) {
	// LRU cache is thread-safe, no lock needed
	ps.txsReceived.Add(string(txHash), struct{}{})
}

// MarkBlockSent records that we sent a block to this peer.
func (ps *PeerState) MarkBlockSent(height int64) {
	// LRU cache is thread-safe, no lock needed
	ps.blocksSent.Add(height, struct{}{})
}

// MarkBlockReceived records that we received a block from this peer.
func (ps *PeerState) MarkBlockReceived(height int64) {
	// LRU cache is thread-safe, no lock needed
	ps.blocksReceived.Add(height, struct{}{})
}

// ShouldSendTx returns true if we should send a transaction to this peer.
// Returns false if we've already sent it or received it from them.
func (ps *PeerState) ShouldSendTx(txHash []byte) bool {
	key := string(txHash)
	// LRU cache is thread-safe, no lock needed
	return !ps.txsSent.Contains(key) && !ps.txsReceived.Contains(key)
}

// ShouldSendBlock returns true if we should send a block to this peer.
// Returns false if we've already sent it or received it from them.
func (ps *PeerState) ShouldSendBlock(height int64) bool {
	// LRU cache is thread-safe, no lock needed
	return !ps.blocksSent.Contains(height) && !ps.blocksReceived.Contains(height)
}

// HasTx returns true if we've exchanged this transaction with the peer.
func (ps *PeerState) HasTx(txHash []byte) bool {
	key := string(txHash)
	// LRU cache is thread-safe, no lock needed
	return ps.txsSent.Contains(key) || ps.txsReceived.Contains(key)
}

// HasBlock returns true if we've exchanged this block with the peer.
func (ps *PeerState) HasBlock(height int64) bool {
	// LRU cache is thread-safe, no lock needed
	return ps.blocksSent.Contains(height) || ps.blocksReceived.Contains(height)
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
	// LRU cache is thread-safe, no lock needed
	return ps.txsSent.Len()
}

// TxsReceivedCount returns the number of transactions received from this peer.
func (ps *PeerState) TxsReceivedCount() int {
	// LRU cache is thread-safe, no lock needed
	return ps.txsReceived.Len()
}

// BlocksSentCount returns the number of blocks sent to this peer.
func (ps *PeerState) BlocksSentCount() int {
	// LRU cache is thread-safe, no lock needed
	return ps.blocksSent.Len()
}

// BlocksReceivedCount returns the number of blocks received from this peer.
func (ps *PeerState) BlocksReceivedCount() int {
	// LRU cache is thread-safe, no lock needed
	return ps.blocksReceived.Len()
}

// SetSeed marks this peer as a seed node.
func (ps *PeerState) SetSeed(isSeed bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.IsSeed = isSeed
}

// ClearTxHistory clears all transaction history for this peer.
// This can be useful when reconnecting or resetting peer state.
func (ps *PeerState) ClearTxHistory() {
	ps.txsSent.Purge()
	ps.txsReceived.Purge()
}

// ClearBlockHistory clears all block history for this peer.
// This can be useful when reconnecting or resetting peer state.
func (ps *PeerState) ClearBlockHistory() {
	ps.blocksSent.Purge()
	ps.blocksReceived.Purge()
}
