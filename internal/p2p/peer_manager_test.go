package p2p

import (
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
)

func TestPeerManager_AddRemove(t *testing.T) {
	pm := NewPeerManager()
	peerID := peer.ID("test-peer-1")

	t.Run("add peer", func(t *testing.T) {
		state := pm.AddPeer(peerID, true)
		require.NotNil(t, state)
		require.Equal(t, peerID, state.PeerID)
		require.True(t, state.IsOutbound)
		require.Equal(t, 1, pm.PeerCount())
	})

	t.Run("add existing peer returns same state", func(t *testing.T) {
		state1 := pm.GetPeer(peerID)
		state2 := pm.AddPeer(peerID, false)
		require.Equal(t, state1, state2)
		require.Equal(t, 1, pm.PeerCount())
	})

	t.Run("has peer", func(t *testing.T) {
		require.True(t, pm.HasPeer(peerID))
		require.False(t, pm.HasPeer(peer.ID("unknown")))
	})

	t.Run("remove peer", func(t *testing.T) {
		pm.RemovePeer(peerID)
		require.Equal(t, 0, pm.PeerCount())
		require.False(t, pm.HasPeer(peerID))
		require.Nil(t, pm.GetPeer(peerID))
	})
}

func TestPeerManager_AllPeers(t *testing.T) {
	pm := NewPeerManager()

	// Add multiple peers
	for i := 0; i < 5; i++ {
		pm.AddPeer(peer.ID(string(rune('a'+i))), true)
	}

	t.Run("all peers", func(t *testing.T) {
		peers := pm.AllPeers()
		require.Len(t, peers, 5)
	})

	t.Run("all peer IDs", func(t *testing.T) {
		ids := pm.AllPeerIDs()
		require.Len(t, ids, 5)
	})
}

func TestPeerManager_TxTracking(t *testing.T) {
	pm := NewPeerManager()
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	txHash := []byte("tx-hash")

	t.Run("should send initially", func(t *testing.T) {
		require.True(t, pm.ShouldSendTx(peerID, txHash))
	})

	t.Run("mark sent", func(t *testing.T) {
		err := pm.MarkTxSent(peerID, txHash)
		require.NoError(t, err)
		require.False(t, pm.ShouldSendTx(peerID, txHash))
	})

	t.Run("mark received", func(t *testing.T) {
		txHash2 := []byte("tx-hash-2")
		err := pm.MarkTxReceived(peerID, txHash2)
		require.NoError(t, err)
		require.False(t, pm.ShouldSendTx(peerID, txHash2))
	})

	t.Run("error for unknown peer", func(t *testing.T) {
		err := pm.MarkTxSent(peer.ID("unknown"), txHash)
		require.ErrorIs(t, err, types.ErrPeerNotFound)
	})
}

func TestPeerManager_BlockTracking(t *testing.T) {
	pm := NewPeerManager()
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	t.Run("should send initially", func(t *testing.T) {
		require.True(t, pm.ShouldSendBlock(peerID, 100))
	})

	t.Run("mark sent", func(t *testing.T) {
		err := pm.MarkBlockSent(peerID, 100)
		require.NoError(t, err)
		require.False(t, pm.ShouldSendBlock(peerID, 100))
	})

	t.Run("mark received", func(t *testing.T) {
		err := pm.MarkBlockReceived(peerID, 200)
		require.NoError(t, err)
		require.False(t, pm.ShouldSendBlock(peerID, 200))
	})
}

func TestPeerManager_PeersToSend(t *testing.T) {
	pm := NewPeerManager()

	// Add multiple peers
	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")
	peer3 := peer.ID("peer3")

	pm.AddPeer(peer1, true)
	pm.AddPeer(peer2, true)
	pm.AddPeer(peer3, true)

	txHash := []byte("tx-hash")
	height := int64(100)

	t.Run("all peers should receive initially", func(t *testing.T) {
		txPeers := pm.PeersToSendTx(txHash)
		require.Len(t, txPeers, 3)

		blockPeers := pm.PeersToSendBlock(height)
		require.Len(t, blockPeers, 3)
	})

	t.Run("excludes peer that already has tx", func(t *testing.T) {
		err := pm.MarkTxSent(peer1, txHash)
		require.NoError(t, err)

		txPeers := pm.PeersToSendTx(txHash)
		require.Len(t, txPeers, 2)
	})

	t.Run("excludes peer that already has block", func(t *testing.T) {
		err := pm.MarkBlockReceived(peer2, height)
		require.NoError(t, err)

		blockPeers := pm.PeersToSendBlock(height)
		require.Len(t, blockPeers, 2)
	})
}

func TestPeerManager_Concurrent(t *testing.T) {
	pm := NewPeerManager()

	const numGoroutines = 10
	const opsPerGoroutine = 50

	var wg sync.WaitGroup

	// Concurrent adds
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				peerID := peer.ID(string(rune(id*100 + i)))
				pm.AddPeer(peerID, true)
			}
		}(g)
	}

	wg.Wait()

	require.Equal(t, numGoroutines*opsPerGoroutine, pm.PeerCount())

	// Concurrent reads
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				pm.AllPeers()
				pm.AllPeerIDs()
			}
		}(g)
	}

	wg.Wait()
}

func TestPeerManager_SetPublicKey(t *testing.T) {
	pm := NewPeerManager()
	peerID := peer.ID("test-peer")
	pm.AddPeer(peerID, true)

	pubKey := []byte("test-public-key")

	t.Run("set public key", func(t *testing.T) {
		err := pm.SetPublicKey(peerID, pubKey)
		require.NoError(t, err)

		state := pm.GetPeer(peerID)
		require.Equal(t, pubKey, state.PublicKey)
	})

	t.Run("error for unknown peer", func(t *testing.T) {
		err := pm.SetPublicKey(peer.ID("unknown"), pubKey)
		require.ErrorIs(t, err, types.ErrPeerNotFound)
	})
}
