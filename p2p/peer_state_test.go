package p2p

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestNewPeerState(t *testing.T) {
	peerID := peer.ID("test-peer-1")

	t.Run("creates outbound peer", func(t *testing.T) {
		ps := NewPeerState(peerID, true)
		require.Equal(t, peerID, ps.PeerID)
		require.True(t, ps.IsOutbound)
		// Verify LRU caches are initialized by checking initial counts
		require.Equal(t, 0, ps.TxsSentCount())
		require.Equal(t, 0, ps.TxsReceivedCount())
		require.Equal(t, 0, ps.BlocksSentCount())
		require.Equal(t, 0, ps.BlocksReceivedCount())
		require.False(t, ps.ConnectedAt.IsZero())
	})

	t.Run("creates inbound peer", func(t *testing.T) {
		ps := NewPeerState(peerID, false)
		require.False(t, ps.IsOutbound)
	})
}

func TestPeerState_TxTracking(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)
	txHash := []byte("tx-hash-1")

	t.Run("should send before any exchange", func(t *testing.T) {
		require.True(t, ps.ShouldSendTx(txHash))
		require.False(t, ps.HasTx(txHash))
	})

	t.Run("mark sent prevents duplicate", func(t *testing.T) {
		ps.MarkTxSent(txHash)
		require.False(t, ps.ShouldSendTx(txHash))
		require.True(t, ps.HasTx(txHash))
		require.Equal(t, 1, ps.TxsSentCount())
	})

	t.Run("mark received prevents duplicate", func(t *testing.T) {
		txHash2 := []byte("tx-hash-2")
		ps.MarkTxReceived(txHash2)
		require.False(t, ps.ShouldSendTx(txHash2))
		require.True(t, ps.HasTx(txHash2))
		require.Equal(t, 1, ps.TxsReceivedCount())
	})
}

func TestPeerState_BlockTracking(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	t.Run("should send before any exchange", func(t *testing.T) {
		require.True(t, ps.ShouldSendBlock(100))
		require.False(t, ps.HasBlock(100))
	})

	t.Run("mark sent prevents duplicate", func(t *testing.T) {
		ps.MarkBlockSent(100)
		require.False(t, ps.ShouldSendBlock(100))
		require.True(t, ps.HasBlock(100))
		require.Equal(t, 1, ps.BlocksSentCount())
	})

	t.Run("mark received prevents duplicate", func(t *testing.T) {
		ps.MarkBlockReceived(200)
		require.False(t, ps.ShouldSendBlock(200))
		require.True(t, ps.HasBlock(200))
		require.Equal(t, 1, ps.BlocksReceivedCount())
	})
}

func TestPeerState_Penalty(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	t.Run("starts at zero", func(t *testing.T) {
		require.Equal(t, int64(0), ps.GetPenaltyPoints())
	})

	t.Run("adds penalty", func(t *testing.T) {
		ps.AddPenalty(10)
		require.Equal(t, int64(10), ps.GetPenaltyPoints())

		ps.AddPenalty(5)
		require.Equal(t, int64(15), ps.GetPenaltyPoints())
	})

	t.Run("decays penalty", func(t *testing.T) {
		ps.DecayPenalty(3)
		require.Equal(t, int64(12), ps.GetPenaltyPoints())
	})

	t.Run("decay doesn't go negative", func(t *testing.T) {
		ps.DecayPenalty(100)
		require.Equal(t, int64(0), ps.GetPenaltyPoints())
	})
}

func TestPeerState_Timing(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	t.Run("updates last seen", func(t *testing.T) {
		time.Sleep(10 * time.Millisecond)
		oldLastSeen := ps.GetLastSeen()

		ps.UpdateLastSeen()
		require.True(t, ps.GetLastSeen().After(oldLastSeen))
	})

	t.Run("updates latency", func(t *testing.T) {
		ps.UpdateLatency(50 * time.Millisecond)
		require.Equal(t, 50*time.Millisecond, ps.GetLatency())
	})

	t.Run("connection duration", func(t *testing.T) {
		require.Greater(t, ps.ConnectionDuration(), time.Duration(0))
	})
}

func TestPeerState_PublicKey(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	pubKey := []byte("test-public-key")
	ps.SetPublicKey(pubKey)

	require.Equal(t, pubKey, ps.PublicKey)
}

func TestPeerState_Seed(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	require.False(t, ps.IsSeed)

	ps.SetSeed(true)
	require.True(t, ps.IsSeed)
}

func TestPeerState_LRUEviction(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	t.Run("tx cache evicts oldest entries", func(t *testing.T) {
		// Fill beyond capacity
		numEntries := MaxKnownTxsPerPeer + 100
		for i := 0; i < numEntries; i++ {
			txHash := []byte("tx:" + string([]byte{byte(i >> 16), byte(i >> 8), byte(i)}))
			ps.MarkTxSent(txHash)
		}

		// Count should be capped at max
		require.Equal(t, MaxKnownTxsPerPeer, ps.TxsSentCount())

		// Newest entry should remain
		lastIdx := numEntries - 1
		newestHash := []byte("tx:" + string([]byte{byte(lastIdx >> 16), byte(lastIdx >> 8), byte(lastIdx)}))
		require.True(t, ps.HasTx(newestHash))
	})

	t.Run("block cache evicts oldest entries", func(t *testing.T) {
		// Fill beyond capacity
		for i := int64(0); i < MaxKnownBlocksPerPeer+100; i++ {
			ps.MarkBlockSent(i)
		}

		// Count should be capped at max
		require.Equal(t, MaxKnownBlocksPerPeer, ps.BlocksSentCount())

		// Newest entry should remain
		require.True(t, ps.HasBlock(int64(MaxKnownBlocksPerPeer+99)))
	})
}

func TestPeerState_ClearHistory(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	// Add some data
	ps.MarkTxSent([]byte("tx1"))
	ps.MarkTxReceived([]byte("tx2"))
	ps.MarkBlockSent(100)
	ps.MarkBlockReceived(200)

	t.Run("clear tx history", func(t *testing.T) {
		ps.ClearTxHistory()
		require.Equal(t, 0, ps.TxsSentCount())
		require.Equal(t, 0, ps.TxsReceivedCount())
		require.False(t, ps.HasTx([]byte("tx1")))
	})

	t.Run("clear block history", func(t *testing.T) {
		ps.ClearBlockHistory()
		require.Equal(t, 0, ps.BlocksSentCount())
		require.Equal(t, 0, ps.BlocksReceivedCount())
		require.False(t, ps.HasBlock(100))
	})
}

func TestPeerState_ConcurrentAccess(t *testing.T) {
	ps := NewPeerState(peer.ID("test-peer"), true)

	// Concurrent transaction tracking
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				txHash := []byte{byte(id), byte(j)}
				ps.MarkTxSent(txHash)
				ps.HasTx(txHash)
				ps.ShouldSendTx(txHash)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or race - count should be reasonable
	require.LessOrEqual(t, ps.TxsSentCount(), 1000)
}
