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

	t.Run("ignores unknown peer", func(t *testing.T) {
		unknownPeer := peer.ID("unknown")
		scorer.AddPenalty(unknownPeer, 10, ReasonInvalidMessage, "should not panic")

		points := scorer.GetPenaltyPoints(unknownPeer)
		require.Equal(t, int64(0), points)
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
