package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestNewPeerScorer(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)

	require.NotNil(t, scorer)
	require.NotNil(t, scorer.peerManager)
	require.NotNil(t, scorer.banCounts)
}

func TestPeerScorer_AddPenalty(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	t.Run("adds penalty to existing peer", func(t *testing.T) {
		scorer.AddPenalty(peerID, PenaltyInvalidMessage, ReasonInvalidMessage, "test message")

		points := scorer.GetPenaltyPoints(peerID)
		require.Equal(t, int64(PenaltyInvalidMessage), points)
	})

	t.Run("accumulates penalties", func(t *testing.T) {
		scorer.AddPenalty(peerID, PenaltyInvalidTx, ReasonInvalidTx, "invalid tx")

		points := scorer.GetPenaltyPoints(peerID)
		require.Equal(t, int64(PenaltyInvalidMessage+PenaltyInvalidTx), points)
	})

	t.Run("tracks disconnected peer in history", func(t *testing.T) {
		unknownPeer := peer.ID("unknown")
		scorer.AddPenalty(unknownPeer, 10, ReasonInvalidMessage, "should track in history")

		// Disconnected peers are tracked via penalty history
		points := scorer.GetEffectivePenaltyPoints(unknownPeer)
		require.Equal(t, int64(10), points)
	})
}

func TestPeerScorer_ShouldBan(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	t.Run("should not ban under threshold", func(t *testing.T) {
		scorer.AddPenalty(peerID, PenaltyThresholdBan-1, ReasonProtocolViolation, "almost ban")
		require.False(t, scorer.ShouldBan(peerID))
	})

	t.Run("should ban at threshold", func(t *testing.T) {
		scorer.AddPenalty(peerID, 1, ReasonTimeout, "triggers ban")
		require.True(t, scorer.ShouldBan(peerID))
	})

	t.Run("should not ban unknown peer", func(t *testing.T) {
		require.False(t, scorer.ShouldBan(peer.ID("unknown")))
	})
}

func TestPeerScorer_BanDuration(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")

	t.Run("first ban is base duration", func(t *testing.T) {
		duration := scorer.GetBanDuration(peerID)
		require.Equal(t, BanDurationBase, duration)
	})

	t.Run("ban duration escalates", func(t *testing.T) {
		scorer.RecordBan(peerID)
		duration := scorer.GetBanDuration(peerID)
		require.Equal(t, BanDurationBase*2, duration)

		scorer.RecordBan(peerID)
		duration = scorer.GetBanDuration(peerID)
		require.Equal(t, BanDurationBase*4, duration)
	})

	t.Run("ban duration caps at max", func(t *testing.T) {
		// Record many bans to exceed max
		for i := 0; i < 10; i++ {
			scorer.RecordBan(peerID)
		}
		duration := scorer.GetBanDuration(peerID)
		require.Equal(t, BanDurationMax, duration)
	})

	t.Run("reset ban count", func(t *testing.T) {
		scorer.ResetBanCount(peerID)
		duration := scorer.GetBanDuration(peerID)
		require.Equal(t, BanDurationBase, duration)
	})
}

func TestPeerScorer_DecayPenalties(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)

	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")
	pm.AddPeer(peer1, true)
	pm.AddPeer(peer2, true)

	scorer.AddPenalty(peer1, 50, ReasonProtocolViolation, "test")
	scorer.AddPenalty(peer2, 30, ReasonInvalidBlock, "test")

	t.Run("decays all peers", func(t *testing.T) {
		scorer.DecayPenalties(10)

		require.Equal(t, int64(40), scorer.GetPenaltyPoints(peer1))
		require.Equal(t, int64(20), scorer.GetPenaltyPoints(peer2))
	})

	t.Run("decay doesn't go negative", func(t *testing.T) {
		scorer.DecayPenalties(100)

		require.Equal(t, int64(0), scorer.GetPenaltyPoints(peer1))
		require.Equal(t, int64(0), scorer.GetPenaltyPoints(peer2))
	})
}

func TestPeerScorer_EventLogging(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	t.Run("logs penalty events", func(t *testing.T) {
		scorer.AddPenalty(peerID, 5, ReasonInvalidMessage, "test event 1")
		scorer.AddPenalty(peerID, 10, ReasonInvalidBlock, "test event 2")

		events := scorer.RecentEvents(10)
		require.Len(t, events, 2)
		require.Equal(t, int64(5), events[0].Points)
		require.Equal(t, ReasonInvalidMessage, events[0].Reason)
		require.Equal(t, "test event 1", events[0].Message)
		require.Equal(t, int64(10), events[1].Points)
	})

	t.Run("limits returned events", func(t *testing.T) {
		events := scorer.RecentEvents(1)
		require.Len(t, events, 1)
		require.Equal(t, int64(10), events[0].Points)
	})

	t.Run("counts peer events", func(t *testing.T) {
		count := scorer.PeerEventsCount(peerID)
		require.Equal(t, 2, count)
	})

	t.Run("trims excess events", func(t *testing.T) {
		// Add more events than maxEvents
		scorer.maxEvents = 5
		for i := 0; i < 10; i++ {
			scorer.AddPenalty(peerID, 1, ReasonTimeout, "bulk event")
		}

		events := scorer.RecentEvents(100)
		require.LessOrEqual(t, len(events), 5)
	})
}

func TestPeerScorer_StartDecayLoop(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	scorer.AddPenalty(peerID, 50, ReasonProtocolViolation, "test")

	stopCh := make(chan struct{})

	// Start decay loop (won't actually decay in this test since we're not waiting an hour)
	scorer.StartDecayLoop(stopCh)

	// Verify we can stop it without hanging
	close(stopCh)

	// Give goroutine time to exit
	time.Sleep(10 * time.Millisecond)
}

func TestPenaltyConstants(t *testing.T) {
	// Verify penalty constants are sensible
	require.Greater(t, PenaltyThresholdBan, PenaltyThresholdWarn)
	require.Greater(t, PenaltyProtocolViolation, PenaltyInvalidMessage)
	require.Greater(t, PenaltyChainMismatch, PenaltyProtocolViolation)
	require.Equal(t, int64(100), int64(PenaltyChainMismatch))
	require.Equal(t, int64(100), int64(PenaltyVersionMismatch))
}

func TestPenaltyReasons(t *testing.T) {
	// Verify penalty reasons are defined
	reasons := []PenaltyReason{
		ReasonInvalidMessage,
		ReasonInvalidBlock,
		ReasonInvalidTx,
		ReasonTimeout,
		ReasonDuplicateMessage,
		ReasonProtocolViolation,
		ReasonChainMismatch,
		ReasonVersionMismatch,
	}

	for _, reason := range reasons {
		require.NotEmpty(t, string(reason))
	}
}

func TestPeerScorer_PenaltyPersistence(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")

	t.Run("tracks penalties without connected peer", func(t *testing.T) {
		// No peer added to peer manager
		scorer.AddPenalty(peerID, 50, ReasonProtocolViolation, "test penalty")

		// Should be tracked in penalty history
		points := scorer.GetEffectivePenaltyPoints(peerID)
		require.Equal(t, int64(50), points)
	})

	t.Run("accumulates penalties for disconnected peer", func(t *testing.T) {
		scorer.AddPenalty(peerID, 25, ReasonInvalidBlock, "another penalty")

		points := scorer.GetEffectivePenaltyPoints(peerID)
		require.Equal(t, int64(75), points)
	})

	t.Run("ShouldBan works for disconnected peers", func(t *testing.T) {
		scorer.AddPenalty(peerID, 25, ReasonTimeout, "push to ban threshold")

		require.True(t, scorer.ShouldBan(peerID))
	})
}

func TestPeerScorer_PenaltyRecord(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")

	t.Run("initializes record correctly", func(t *testing.T) {
		scorer.AddPenalty(peerID, 10, ReasonTimeout, "test")

		scorer.mu.RLock()
		record := scorer.penaltyHistory[peerID]
		scorer.mu.RUnlock()

		require.NotNil(t, record)
		require.Equal(t, int64(10), record.Points)
		require.NotZero(t, record.LastDecay)
		require.Equal(t, 0, record.BanCount)
	})

	t.Run("tracks ban count in record", func(t *testing.T) {
		scorer.RecordBan(peerID)

		scorer.mu.RLock()
		record := scorer.penaltyHistory[peerID]
		scorer.mu.RUnlock()

		require.Equal(t, 1, record.BanCount)
		require.NotZero(t, record.LastBanEnd)
	})

	t.Run("reset clears ban tracking", func(t *testing.T) {
		scorer.ResetBanCount(peerID)

		scorer.mu.RLock()
		record := scorer.penaltyHistory[peerID]
		scorer.mu.RUnlock()

		require.Equal(t, 0, record.BanCount)
		require.True(t, record.LastBanEnd.IsZero())
	})
}

func TestPeerScorer_WallClockDecay(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")

	t.Run("decay applies based on time elapsed", func(t *testing.T) {
		// Add penalty and manually adjust LastDecay to simulate time passing
		scorer.mu.Lock()
		record := &PenaltyRecord{
			Points:    100,
			LastDecay: time.Now().Add(-3 * time.Hour), // 3 hours ago
		}
		scorer.penaltyHistory[peerID] = record
		scorer.mu.Unlock()

		// Get points - should apply 3 hours of decay (3 * PenaltyDecayRate = 3 points)
		points := scorer.GetEffectivePenaltyPoints(peerID)
		require.Equal(t, int64(97), points)
	})

	t.Run("decay updates LastDecay timestamp", func(t *testing.T) {
		scorer.mu.RLock()
		record := scorer.penaltyHistory[peerID]
		scorer.mu.RUnlock()

		// LastDecay should be updated to now
		require.WithinDuration(t, time.Now(), record.LastDecay, time.Second)
	})

	t.Run("decay does not go negative", func(t *testing.T) {
		// Set up record with low points and old timestamp
		scorer.mu.Lock()
		record := &PenaltyRecord{
			Points:    5,
			LastDecay: time.Now().Add(-100 * time.Hour), // 100 hours ago
		}
		scorer.penaltyHistory[peer.ID("decay-test")] = record
		scorer.mu.Unlock()

		points := scorer.GetEffectivePenaltyPoints(peer.ID("decay-test"))
		require.Equal(t, int64(0), points)
	})

	t.Run("no decay for recent timestamps", func(t *testing.T) {
		peerID2 := peer.ID("recent-peer")
		scorer.mu.Lock()
		record := &PenaltyRecord{
			Points:    50,
			LastDecay: time.Now().Add(-30 * time.Minute), // 30 minutes ago (< 1 hour)
		}
		scorer.penaltyHistory[peerID2] = record
		scorer.mu.Unlock()

		points := scorer.GetEffectivePenaltyPoints(peerID2)
		require.Equal(t, int64(50), points) // No decay yet
	})

	t.Run("decay applies before adding new penalty", func(t *testing.T) {
		peerID3 := peer.ID("accumulate-peer")
		scorer.mu.Lock()
		record := &PenaltyRecord{
			Points:    50,
			LastDecay: time.Now().Add(-2 * time.Hour),
		}
		scorer.penaltyHistory[peerID3] = record
		scorer.mu.Unlock()

		// Add new penalty - should decay first (50 - 2 = 48), then add 10
		scorer.AddPenalty(peerID3, 10, ReasonTimeout, "test")

		points := scorer.GetEffectivePenaltyPoints(peerID3)
		require.Equal(t, int64(58), points)
	})
}

func TestPeerScorer_GetPenaltyPointsFallback(t *testing.T) {
	pm := NewPeerManager()
	scorer := NewPeerScorer(pm)
	peerID := peer.ID("test-peer")

	t.Run("uses connected peer state when available", func(t *testing.T) {
		pm.AddPeer(peerID, true)
		scorer.AddPenalty(peerID, 25, ReasonInvalidMessage, "test")

		// GetPenaltyPoints should return connected peer's state
		points := scorer.GetPenaltyPoints(peerID)
		require.Equal(t, int64(25), points)
	})

	t.Run("falls back to history for disconnected peer", func(t *testing.T) {
		disconnectedPeer := peer.ID("disconnected")
		scorer.AddPenalty(disconnectedPeer, 30, ReasonInvalidBlock, "test")

		// No connected state, should use history
		points := scorer.GetPenaltyPoints(disconnectedPeer)
		require.Equal(t, int64(30), points)
	})

	t.Run("returns zero for unknown peer", func(t *testing.T) {
		points := scorer.GetPenaltyPoints(peer.ID("totally-unknown"))
		require.Equal(t, int64(0), points)
	})
}

func TestPeerScorer_NilPeerManager(t *testing.T) {
	// Test with nil peer manager
	scorer := NewPeerScorer(nil)
	peerID := peer.ID("test-peer")

	t.Run("AddPenalty works without peer manager", func(t *testing.T) {
		scorer.AddPenalty(peerID, 25, ReasonTimeout, "test")

		points := scorer.GetEffectivePenaltyPoints(peerID)
		require.Equal(t, int64(25), points)
	})

	t.Run("GetPenaltyPoints falls back to history", func(t *testing.T) {
		points := scorer.GetPenaltyPoints(peerID)
		require.Equal(t, int64(25), points)
	})

	t.Run("ShouldBan works", func(t *testing.T) {
		scorer.AddPenalty(peerID, 75, ReasonChainMismatch, "push to ban")
		require.True(t, scorer.ShouldBan(peerID))
	})
}
