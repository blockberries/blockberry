package p2p

import (
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Basic(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"test": 10, // 10 messages per second
			},
			BurstSize: 5,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// First few messages should be allowed (burst)
	for range 5 {
		assert.True(t, rl.Allow(peerID, "test", 100))
	}

	// Next message should be throttled (burst exhausted)
	assert.False(t, rl.Allow(peerID, "test", 100))

	// Wait for token refill
	time.Sleep(200 * time.Millisecond) // Should get ~2 tokens

	// Should be allowed again
	assert.True(t, rl.Allow(peerID, "test", 100))
}

func TestRateLimiter_ByteLimit(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"test": 1000, // High message rate
			},
			BytesPerSecond: 1000, // 1000 bytes per second
			BurstSize:      10,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Large message should be allowed initially (within burst)
	assert.True(t, rl.Allow(peerID, "test", 500))

	// Another large message should be throttled
	assert.False(t, rl.Allow(peerID, "test", 600))

	// Small message should still work
	assert.True(t, rl.Allow(peerID, "test", 100))
}

func TestRateLimiter_PerPeer(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"test": 10,
			},
			BurstSize: 3,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")

	// Exhaust peer1's burst
	for range 3 {
		assert.True(t, rl.Allow(peer1, "test", 0))
	}
	assert.False(t, rl.Allow(peer1, "test", 0))

	// peer2 should still have full burst
	for range 3 {
		assert.True(t, rl.Allow(peer2, "test", 0))
	}
	assert.False(t, rl.Allow(peer2, "test", 0))
}

func TestRateLimiter_PerStream(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"stream1": 10,
				"stream2": 10,
			},
			BurstSize: 2,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Exhaust stream1 burst
	assert.True(t, rl.Allow(peerID, "stream1", 0))
	assert.True(t, rl.Allow(peerID, "stream1", 0))
	assert.False(t, rl.Allow(peerID, "stream1", 0))

	// stream2 should still work
	assert.True(t, rl.Allow(peerID, "stream2", 0))
	assert.True(t, rl.Allow(peerID, "stream2", 0))
	assert.False(t, rl.Allow(peerID, "stream2", 0))
}

func TestRateLimiter_AllowN(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"test": 10,
			},
			BurstSize: 10,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Allow batch of 5
	assert.True(t, rl.AllowN(peerID, "test", 5, 0))

	// Allow another batch of 5
	assert.True(t, rl.AllowN(peerID, "test", 5, 0))

	// Next batch should be throttled
	assert.False(t, rl.AllowN(peerID, "test", 5, 0))
}

func TestRateLimiter_RemovePeer(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits:          DefaultRateLimits(),
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Trigger limiter creation
	rl.Allow(peerID, StreamTransactions, 0)
	assert.Equal(t, 1, rl.PeerCount())

	// Remove peer
	rl.RemovePeer(peerID)
	assert.Equal(t, 0, rl.PeerCount())
}

func TestRateLimiter_ResetPeer(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"test": 10,
			},
			BurstSize: 3,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Exhaust burst
	for range 3 {
		rl.Allow(peerID, "test", 0)
	}
	assert.False(t, rl.Allow(peerID, "test", 0))

	// Reset peer
	rl.ResetPeer(peerID)

	// Should have full burst again
	for range 3 {
		assert.True(t, rl.Allow(peerID, "test", 0))
	}
}

func TestRateLimiter_CleanupIdlePeers(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits:          DefaultRateLimits(),
		CleanupInterval: 50 * time.Millisecond,
		PeerIdleTimeout: 100 * time.Millisecond,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Create limiter
	rl.Allow(peerID, StreamTransactions, 0)
	assert.Equal(t, 1, rl.PeerCount())

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Peer should be cleaned up
	assert.Equal(t, 0, rl.PeerCount())
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits:          DefaultRateLimits(),
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	var wg sync.WaitGroup

	// Multiple goroutines accessing same peer
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			peerID := peer.ID("shared-peer")
			for range 100 {
				rl.Allow(peerID, StreamTransactions, 100)
			}
		}()
	}

	// Multiple goroutines accessing different peers
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			peerID := peer.ID(string(rune(n)))
			for range 100 {
				rl.Allow(peerID, StreamTransactions, 100)
			}
		}(i)
	}

	wg.Wait()
}

func TestRateLimiter_DefaultLimits(t *testing.T) {
	limits := DefaultRateLimits()

	assert.Greater(t, limits.BytesPerSecond, int64(0))
	assert.Greater(t, limits.BurstSize, 0)
	assert.NotEmpty(t, limits.MessagesPerSecond)

	// Check all stream types have limits
	assert.Contains(t, limits.MessagesPerSecond, StreamHandshake)
	assert.Contains(t, limits.MessagesPerSecond, StreamPEX)
	assert.Contains(t, limits.MessagesPerSecond, StreamTransactions)
	assert.Contains(t, limits.MessagesPerSecond, StreamBlockSync)
	assert.Contains(t, limits.MessagesPerSecond, StreamBlocks)
	assert.Contains(t, limits.MessagesPerSecond, StreamConsensus)
	assert.Contains(t, limits.MessagesPerSecond, StreamHousekeeping)
}

func TestRateLimiter_SetLimits(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits:          DefaultRateLimits(),
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	newLimits := RateLimits{
		MessagesPerSecond: map[string]float64{
			"custom": 5,
		},
		BytesPerSecond: 5000,
		BurstSize:      3,
	}

	rl.SetLimits(newLimits)

	retrieved := rl.GetLimits()
	assert.Equal(t, int64(5000), retrieved.BytesPerSecond)
	assert.Equal(t, 3, retrieved.BurstSize)
	assert.Equal(t, float64(5), retrieved.MessagesPerSecond["custom"])
}

func TestRateLimiter_GetLimits(t *testing.T) {
	limits := RateLimits{
		MessagesPerSecond: map[string]float64{
			"test": 10,
		},
		BytesPerSecond: 1000,
		BurstSize:      5,
	}

	cfg := RateLimiterConfig{
		Limits:          limits,
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	retrieved := rl.GetLimits()
	assert.Equal(t, limits.BytesPerSecond, retrieved.BytesPerSecond)
	assert.Equal(t, limits.BurstSize, retrieved.BurstSize)
	assert.Equal(t, limits.MessagesPerSecond["test"], retrieved.MessagesPerSecond["test"])

	// Verify it's a copy (modifying returned value doesn't affect original)
	retrieved.BurstSize = 999
	original := rl.GetLimits()
	assert.Equal(t, 5, original.BurstSize)
}

func TestRateLimiter_UnknownStream(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"known": 10,
			},
			BurstSize: 5,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Unknown stream should be allowed (no limit configured)
	for range 100 {
		assert.True(t, rl.Allow(peerID, "unknown", 0))
	}
}

func TestRateLimiter_ZeroByteLimit(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{
				"test": 1000,
			},
			BytesPerSecond: 0, // Unlimited
			BurstSize:      100,
		},
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	peerID := peer.ID("test-peer")

	// Large messages should all be allowed
	for range 100 {
		assert.True(t, rl.Allow(peerID, "test", 1_000_000))
	}
}

func TestTokenBucket(t *testing.T) {
	// Test basic token bucket behavior
	bucket := newTokenBucket(10, 5) // 10 tokens/sec, burst of 5

	// Initial burst
	for range 5 {
		require.True(t, bucket.allow(1))
	}

	// Should be empty
	assert.False(t, bucket.allow(1))

	// Wait for refill
	time.Sleep(200 * time.Millisecond) // Should get ~2 tokens

	// Should have tokens again
	assert.True(t, bucket.allow(1))
}

func TestRateLimiter_PeerCount(t *testing.T) {
	cfg := RateLimiterConfig{
		Limits:          DefaultRateLimits(),
		CleanupInterval: time.Hour,
		PeerIdleTimeout: time.Hour,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	assert.Equal(t, 0, rl.PeerCount())

	// Add some peers
	rl.Allow(peer.ID("peer1"), StreamTransactions, 0)
	assert.Equal(t, 1, rl.PeerCount())

	rl.Allow(peer.ID("peer2"), StreamTransactions, 0)
	assert.Equal(t, 2, rl.PeerCount())

	rl.Allow(peer.ID("peer3"), StreamTransactions, 0)
	assert.Equal(t, 3, rl.PeerCount())

	// Same peer shouldn't increase count
	rl.Allow(peer.ID("peer1"), StreamTransactions, 0)
	assert.Equal(t, 3, rl.PeerCount())
}
