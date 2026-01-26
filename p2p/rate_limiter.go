package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// RateLimits defines rate limits for different message types.
type RateLimits struct {
	// Messages per second limits by stream
	MessagesPerSecond map[string]float64

	// Bytes per second limit (0 = unlimited)
	BytesPerSecond int64

	// Burst size (number of messages allowed in a burst)
	BurstSize int
}

// DefaultRateLimits returns sensible default rate limits.
func DefaultRateLimits() RateLimits {
	return RateLimits{
		MessagesPerSecond: map[string]float64{
			StreamHandshake:    1,    // 1 per second
			StreamPEX:          0.5,  // 1 per 2 seconds
			StreamTransactions: 100,  // 100 per second
			StreamBlockSync:    10,   // 10 per second
			StreamBlocks:       10,   // 10 per second
			StreamConsensus:    100,  // 100 per second
			StreamHousekeeping: 1,    // 1 per second
		},
		BytesPerSecond: 10 * 1024 * 1024, // 10 MB/s
		BurstSize:      10,
	}
}

// peerLimiter tracks rate limiting state for a single peer.
type peerLimiter struct {
	mu sync.Mutex

	// Token buckets for each stream
	buckets map[string]*tokenBucket

	// Byte bucket for overall bandwidth
	byteBucket *tokenBucket

	// Last activity time
	lastActivity time.Time
}

// tokenBucket implements a simple token bucket rate limiter.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64   // tokens per second
	lastRefill time.Time
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	maxTokens := float64(burst)
	if maxTokens < 1 {
		maxTokens = 1
	}
	return &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: rate,
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) allow(n float64) bool {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.lastRefill = now

	// Refill tokens
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}

	// Check if we have enough tokens
	if b.tokens >= n {
		b.tokens -= n
		return true
	}

	return false
}

// RateLimiter provides per-peer rate limiting for messages.
type RateLimiter struct {
	mu sync.RWMutex

	// Per-peer limiters
	peers map[peer.ID]*peerLimiter

	// Rate limits configuration
	limits RateLimits

	// Cleanup interval
	cleanupInterval time.Duration
	peerIdleTimeout time.Duration

	// Lifecycle
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// RateLimiterConfig holds configuration for the rate limiter.
type RateLimiterConfig struct {
	Limits          RateLimits
	CleanupInterval time.Duration
	PeerIdleTimeout time.Duration
}

// NewRateLimiter creates a new per-peer rate limiter.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	limits := cfg.Limits
	if limits.MessagesPerSecond == nil {
		limits = DefaultRateLimits()
	}

	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}

	peerIdleTimeout := cfg.PeerIdleTimeout
	if peerIdleTimeout <= 0 {
		peerIdleTimeout = 30 * time.Minute
	}

	rl := &RateLimiter{
		peers:           make(map[peer.ID]*peerLimiter),
		limits:          limits,
		cleanupInterval: cleanupInterval,
		peerIdleTimeout: peerIdleTimeout,
		stopCh:          make(chan struct{}),
	}

	rl.wg.Add(1)
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop removes idle peer limiters periodically.
func (rl *RateLimiter) cleanupLoop() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanupIdlePeers()
		}
	}
}

// cleanupIdlePeers removes limiters for peers that haven't been active.
func (rl *RateLimiter) cleanupIdlePeers() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for peerID, limiter := range rl.peers {
		limiter.mu.Lock()
		if now.Sub(limiter.lastActivity) > rl.peerIdleTimeout {
			delete(rl.peers, peerID)
		}
		limiter.mu.Unlock()
	}
}

// Stop stops the rate limiter cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	rl.wg.Wait()
}

// Allow checks if a message from a peer should be allowed.
// Returns true if the message is within rate limits, false if it should be throttled.
func (rl *RateLimiter) Allow(peerID peer.ID, stream string, messageSize int) bool {
	limiter := rl.getOrCreateLimiter(peerID)

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.lastActivity = time.Now()

	// Check stream-specific rate limit
	if bucket, exists := limiter.buckets[stream]; exists {
		if !bucket.allow(1) {
			return false
		}
	}

	// Check byte rate limit
	if limiter.byteBucket != nil && messageSize > 0 {
		if !limiter.byteBucket.allow(float64(messageSize)) {
			return false
		}
	}

	return true
}

// AllowN checks if n messages from a peer should be allowed.
func (rl *RateLimiter) AllowN(peerID peer.ID, stream string, n int, totalSize int) bool {
	limiter := rl.getOrCreateLimiter(peerID)

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	limiter.lastActivity = time.Now()

	// Check stream-specific rate limit
	if bucket, exists := limiter.buckets[stream]; exists {
		if !bucket.allow(float64(n)) {
			return false
		}
	}

	// Check byte rate limit
	if limiter.byteBucket != nil && totalSize > 0 {
		if !limiter.byteBucket.allow(float64(totalSize)) {
			return false
		}
	}

	return true
}

// getOrCreateLimiter gets or creates a limiter for a peer.
func (rl *RateLimiter) getOrCreateLimiter(peerID peer.ID) *peerLimiter {
	rl.mu.RLock()
	limiter, exists := rl.peers[peerID]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.peers[peerID]; exists {
		return limiter
	}

	// Create new limiter
	limiter = &peerLimiter{
		buckets:      make(map[string]*tokenBucket),
		lastActivity: time.Now(),
	}

	// Create buckets for each stream
	for stream, rate := range rl.limits.MessagesPerSecond {
		limiter.buckets[stream] = newTokenBucket(rate, rl.limits.BurstSize)
	}

	// Create byte bucket if configured
	if rl.limits.BytesPerSecond > 0 {
		limiter.byteBucket = newTokenBucket(float64(rl.limits.BytesPerSecond), int(rl.limits.BytesPerSecond))
	}

	rl.peers[peerID] = limiter
	return limiter
}

// RemovePeer removes the rate limiter state for a peer.
func (rl *RateLimiter) RemovePeer(peerID peer.ID) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.peers, peerID)
}

// SetLimits updates the rate limits.
// Note: This does not update existing peer limiters, only new ones.
func (rl *RateLimiter) SetLimits(limits RateLimits) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.limits = limits
}

// GetLimits returns the current rate limits.
func (rl *RateLimiter) GetLimits() RateLimits {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	// Return a copy
	limits := RateLimits{
		MessagesPerSecond: make(map[string]float64),
		BytesPerSecond:    rl.limits.BytesPerSecond,
		BurstSize:         rl.limits.BurstSize,
	}
	for k, v := range rl.limits.MessagesPerSecond {
		limits.MessagesPerSecond[k] = v
	}
	return limits
}

// PeerCount returns the number of tracked peers.
func (rl *RateLimiter) PeerCount() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return len(rl.peers)
}

// ResetPeer resets the rate limiter state for a specific peer.
// This allows the peer to send messages again after being throttled.
func (rl *RateLimiter) ResetPeer(peerID peer.ID) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Remove and let it be recreated on next message
	delete(rl.peers, peerID)
}
