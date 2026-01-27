// Package security provides security-related utilities for blockberry nodes.
package security

import (
	"sync"
	"time"

	"github.com/blockberries/blockberry/abi"
)

// TokenBucketLimiter implements abi.RateLimiter using the token bucket algorithm.
// Each key has its own bucket that refills at the configured rate.
type TokenBucketLimiter struct {
	cfg     abi.RateLimiterConfig
	buckets map[string]*bucket
	mu      sync.RWMutex
	done    chan struct{}
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewTokenBucketLimiter creates a new token bucket rate limiter.
func NewTokenBucketLimiter(cfg abi.RateLimiterConfig) *TokenBucketLimiter {
	l := &TokenBucketLimiter{
		cfg:     cfg,
		buckets: make(map[string]*bucket),
		done:    make(chan struct{}),
	}

	// Start cleanup goroutine
	if cfg.CleanupInterval > 0 {
		go l.cleanup()
	}

	return l
}

// Allow checks if a single request should be allowed.
func (l *TokenBucketLimiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN checks if n requests should be allowed.
func (l *TokenBucketLimiter) AllowN(key string, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{
			tokens:    float64(l.cfg.Burst),
			lastCheck: now,
		}
		l.buckets[key] = b
	}

	// Calculate tokens to add based on elapsed time
	elapsed := now.Sub(b.lastCheck)
	tokensToAdd := float64(l.cfg.Rate) * elapsed.Seconds() / l.cfg.Interval.Seconds()
	b.tokens += tokensToAdd
	b.lastCheck = now

	// Cap at burst size
	if b.tokens > float64(l.cfg.Burst) {
		b.tokens = float64(l.cfg.Burst)
	}

	// Check if we have enough tokens
	if b.tokens >= float64(n) {
		b.tokens -= float64(n)
		return true
	}

	return false
}

// Reset resets the rate limit for a key.
func (l *TokenBucketLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// Close releases resources.
func (l *TokenBucketLimiter) Close() {
	close(l.done)
}

// cleanup periodically removes stale entries.
func (l *TokenBucketLimiter) cleanup() {
	ticker := time.NewTicker(l.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanupStale()
		case <-l.done:
			return
		}
	}
}

// cleanupStale removes buckets that are at full capacity (inactive).
func (l *TokenBucketLimiter) cleanupStale() {
	l.mu.Lock()
	defer l.mu.Unlock()

	staleThreshold := 2 * l.cfg.CleanupInterval
	now := time.Now()

	for key, b := range l.buckets {
		if now.Sub(b.lastCheck) > staleThreshold && b.tokens >= float64(l.cfg.Burst) {
			delete(l.buckets, key)
		}
	}
}

// Size returns the number of tracked keys.
func (l *TokenBucketLimiter) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.buckets)
}

// Ensure TokenBucketLimiter implements abi.RateLimiter.
var _ abi.RateLimiter = (*TokenBucketLimiter)(nil)

// ConnectionTracker implements abi.ConnectionLimiter.
type ConnectionTracker struct {
	limits   abi.ResourceLimits
	inbound  map[string]bool
	outbound map[string]bool
	mu       sync.RWMutex
}

// NewConnectionTracker creates a new connection tracker.
func NewConnectionTracker(limits abi.ResourceLimits) *ConnectionTracker {
	return &ConnectionTracker{
		limits:   limits,
		inbound:  make(map[string]bool),
		outbound: make(map[string]bool),
	}
}

// CanAcceptInbound checks if an inbound connection can be accepted.
func (t *ConnectionTracker) CanAcceptInbound() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.inbound) >= t.limits.MaxInboundPeers {
		return false
	}
	if len(t.inbound)+len(t.outbound) >= t.limits.MaxPeers {
		return false
	}
	return true
}

// CanDialOutbound checks if an outbound connection can be made.
func (t *ConnectionTracker) CanDialOutbound() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.outbound) >= t.limits.MaxOutboundPeers {
		return false
	}
	if len(t.inbound)+len(t.outbound) >= t.limits.MaxPeers {
		return false
	}
	return true
}

// OnConnect is called when a connection is established.
func (t *ConnectionTracker) OnConnect(peerID []byte, isInbound bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := string(peerID)
	if isInbound {
		t.inbound[key] = true
	} else {
		t.outbound[key] = true
	}
}

// OnDisconnect is called when a connection is closed.
func (t *ConnectionTracker) OnDisconnect(peerID []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := string(peerID)
	delete(t.inbound, key)
	delete(t.outbound, key)
}

// InboundCount returns the current inbound connection count.
func (t *ConnectionTracker) InboundCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.inbound)
}

// OutboundCount returns the current outbound connection count.
func (t *ConnectionTracker) OutboundCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.outbound)
}

// TotalCount returns the total connection count.
func (t *ConnectionTracker) TotalCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.inbound) + len(t.outbound)
}

// Ensure ConnectionTracker implements abi.ConnectionLimiter.
var _ abi.ConnectionLimiter = (*ConnectionTracker)(nil)

// SlidingWindowLimiter implements rate limiting using a sliding window.
// This provides smoother rate limiting than fixed windows.
type SlidingWindowLimiter struct {
	cfg     abi.RateLimiterConfig
	windows map[string]*slidingWindow
	mu      sync.RWMutex
	done    chan struct{}
}

type slidingWindow struct {
	counts    []int
	timestamps []time.Time
	total     int
}

// NewSlidingWindowLimiter creates a new sliding window rate limiter.
func NewSlidingWindowLimiter(cfg abi.RateLimiterConfig) *SlidingWindowLimiter {
	l := &SlidingWindowLimiter{
		cfg:     cfg,
		windows: make(map[string]*slidingWindow),
		done:    make(chan struct{}),
	}

	if cfg.CleanupInterval > 0 {
		go l.cleanup()
	}

	return l
}

// Allow checks if a single request should be allowed.
func (l *SlidingWindowLimiter) Allow(key string) bool {
	return l.AllowN(key, 1)
}

// AllowN checks if n requests should be allowed.
func (l *SlidingWindowLimiter) AllowN(key string, n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.windows[key]
	if !ok {
		w = &slidingWindow{
			counts:    make([]int, 0, 100),
			timestamps: make([]time.Time, 0, 100),
		}
		l.windows[key] = w
	}

	// Remove expired entries
	cutoff := now.Add(-l.cfg.Interval)
	validIdx := 0
	for i, ts := range w.timestamps {
		if ts.After(cutoff) {
			validIdx = i
			break
		}
		w.total -= w.counts[i]
	}
	if validIdx > 0 {
		w.counts = w.counts[validIdx:]
		w.timestamps = w.timestamps[validIdx:]
	}

	// Check if we can allow
	if w.total+n > l.cfg.Rate {
		return false
	}

	// Add the request
	w.counts = append(w.counts, n)
	w.timestamps = append(w.timestamps, now)
	w.total += n

	return true
}

// Reset resets the rate limit for a key.
func (l *SlidingWindowLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.windows, key)
}

// Close releases resources.
func (l *SlidingWindowLimiter) Close() {
	close(l.done)
}

// cleanup periodically removes stale entries.
func (l *SlidingWindowLimiter) cleanup() {
	ticker := time.NewTicker(l.cfg.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanupStale()
		case <-l.done:
			return
		}
	}
}

// cleanupStale removes windows that are empty and stale.
func (l *SlidingWindowLimiter) cleanupStale() {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, w := range l.windows {
		if w.total == 0 && len(w.counts) == 0 {
			delete(l.windows, key)
		}
	}
}

// Size returns the number of tracked keys.
func (l *SlidingWindowLimiter) Size() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.windows)
}

// Ensure SlidingWindowLimiter implements abi.RateLimiter.
var _ abi.RateLimiter = (*SlidingWindowLimiter)(nil)
