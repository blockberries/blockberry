package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestAllStreams(t *testing.T) {
	streams := AllStreams()

	require.Len(t, streams, 7)
	require.Contains(t, streams, StreamPEX)
	require.Contains(t, streams, StreamTransactions)
	require.Contains(t, streams, StreamBlockSync)
	require.Contains(t, streams, StreamStateSync)
	require.Contains(t, streams, StreamBlocks)
	require.Contains(t, streams, StreamConsensus)
	require.Contains(t, streams, StreamHousekeeping)
}

func TestStreamConstants(t *testing.T) {
	// Verify stream constants are defined
	require.Equal(t, "handshake", StreamHandshake)
	require.Equal(t, "pex", StreamPEX)
	require.Equal(t, "transactions", StreamTransactions)
	require.Equal(t, "blocksync", StreamBlockSync)
	require.Equal(t, "statesync", StreamStateSync)
	require.Equal(t, "blocks", StreamBlocks)
	require.Equal(t, "consensus", StreamConsensus)
	require.Equal(t, "housekeeping", StreamHousekeeping)
}

func TestNewNetwork(t *testing.T) {
	// NewNetwork requires a glueberry.Node which we can't easily mock,
	// so we skip the actual construction test here.
	// The integration tests would cover this path.
	t.Skip("requires glueberry.Node mock")
}

func TestTempBanEntry(t *testing.T) {
	entry := &TempBanEntry{
		ExpiresAt: time.Now().Add(time.Hour),
		Reason:    "test reason",
	}

	require.Equal(t, "test reason", entry.Reason)
	require.True(t, time.Now().Before(entry.ExpiresAt))
}

func TestNetwork_TempBans_DirectAccess(t *testing.T) {
	// Test the temp bans map directly without needing a full Network
	tempBans := make(map[peer.ID]*TempBanEntry)
	peerID := peer.ID("test-peer")

	// Initially not banned
	_, ok := tempBans[peerID]
	require.False(t, ok)

	// Add a temp ban
	tempBans[peerID] = &TempBanEntry{
		ExpiresAt: time.Now().Add(time.Hour),
		Reason:    "chain ID mismatch",
	}

	// Now banned
	entry, ok := tempBans[peerID]
	require.True(t, ok)
	require.Equal(t, "chain ID mismatch", entry.Reason)
	require.True(t, time.Now().Before(entry.ExpiresAt))

	// Test expired ban
	tempBans[peerID] = &TempBanEntry{
		ExpiresAt: time.Now().Add(-time.Hour), // Already expired
		Reason:    "old ban",
	}
	entry = tempBans[peerID]
	require.True(t, time.Now().After(entry.ExpiresAt))
}

func TestNetwork_CleanupExpiredTempBans_Logic(t *testing.T) {
	// Test the cleanup logic without a full Network
	tempBans := make(map[peer.ID]*TempBanEntry)

	// Add some bans - some expired, some not
	peer1 := peer.ID("peer-1")
	peer2 := peer.ID("peer-2")
	peer3 := peer.ID("peer-3")

	tempBans[peer1] = &TempBanEntry{
		ExpiresAt: time.Now().Add(-time.Hour), // Expired
		Reason:    "reason 1",
	}
	tempBans[peer2] = &TempBanEntry{
		ExpiresAt: time.Now().Add(time.Hour), // Not expired
		Reason:    "reason 2",
	}
	tempBans[peer3] = &TempBanEntry{
		ExpiresAt: time.Now().Add(-time.Minute), // Expired
		Reason:    "reason 3",
	}

	require.Len(t, tempBans, 3)

	// Simulate cleanup
	now := time.Now()
	removed := 0
	for peerID, entry := range tempBans {
		if now.After(entry.ExpiresAt) {
			delete(tempBans, peerID)
			removed++
		}
	}

	require.Equal(t, 2, removed)
	require.Len(t, tempBans, 1)
	require.Contains(t, tempBans, peer2)
}

func TestNetwork_IsTempBanned_Logic(t *testing.T) {
	tempBans := make(map[peer.ID]*TempBanEntry)
	peerID := peer.ID("test-peer")

	// Not banned
	entry, ok := tempBans[peerID]
	require.False(t, ok || (entry != nil && time.Now().Before(entry.ExpiresAt)))

	// Active ban
	tempBans[peerID] = &TempBanEntry{
		ExpiresAt: time.Now().Add(time.Hour),
		Reason:    "test",
	}
	entry, ok = tempBans[peerID]
	require.True(t, ok && time.Now().Before(entry.ExpiresAt))

	// Expired ban
	tempBans[peerID] = &TempBanEntry{
		ExpiresAt: time.Now().Add(-time.Hour),
		Reason:    "expired",
	}
	entry, ok = tempBans[peerID]
	require.True(t, ok)
	require.False(t, time.Now().Before(entry.ExpiresAt))
}
