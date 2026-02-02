package types

import (
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVoteTypeConstants(t *testing.T) {
	assert.Equal(t, VoteType(1), VoteTypePrevote)
	assert.Equal(t, VoteType(2), VoteTypePrecommit)
}

func TestDefaultCallbacks(t *testing.T) {
	cb := DefaultCallbacks()
	require.NotNil(t, cb)

	// All callbacks should be nil
	assert.Nil(t, cb.OnTxReceived)
	assert.Nil(t, cb.OnTxValidated)
	assert.Nil(t, cb.OnBlockReceived)
	assert.Nil(t, cb.OnPeerConnected)
	assert.Nil(t, cb.OnConsensusMessage)
}

func TestCallbacks_Clone(t *testing.T) {
	t.Run("nil callbacks", func(t *testing.T) {
		var cb *NodeCallbacks
		clone := cb.Clone()
		assert.Nil(t, clone)
	})

	t.Run("with callbacks set", func(t *testing.T) {
		called := false
		cb := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				called = true
			},
		}

		clone := cb.Clone()
		require.NotNil(t, clone)

		// Clone should have the same callback
		clone.OnTxReceived("test-peer", nil)
		assert.True(t, called)
	})

	t.Run("modifying clone does not affect original", func(t *testing.T) {
		originalCalled := false
		newCalled := false

		cb := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				originalCalled = true
			},
		}

		clone := cb.Clone()
		clone.OnTxReceived = func(peerID peer.ID, tx []byte) {
			newCalled = true
		}

		// Call original
		cb.OnTxReceived("test", nil)
		assert.True(t, originalCalled)
		assert.False(t, newCalled)

		// Call clone
		originalCalled = false
		clone.OnTxReceived("test", nil)
		assert.False(t, originalCalled)
		assert.True(t, newCalled)
	})
}

func TestCallbacks_Merge(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var cb *NodeCallbacks
		other := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {},
		}
		result := cb.Merge(other)
		assert.Nil(t, result)
	})

	t.Run("nil other", func(t *testing.T) {
		cb := &NodeCallbacks{}
		result := cb.Merge(nil)
		assert.Equal(t, cb, result)
	})

	t.Run("merge overwrites nil callbacks", func(t *testing.T) {
		cb := &NodeCallbacks{}
		called := false
		other := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				called = true
			},
		}

		cb.Merge(other)
		require.NotNil(t, cb.OnTxReceived)

		cb.OnTxReceived("test", nil)
		assert.True(t, called)
	})

	t.Run("merge overwrites existing callbacks", func(t *testing.T) {
		originalCalled := false
		newCalled := false

		cb := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				originalCalled = true
			},
		}
		other := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				newCalled = true
			},
		}

		cb.Merge(other)
		cb.OnTxReceived("test", nil)

		assert.False(t, originalCalled)
		assert.True(t, newCalled)
	})

	t.Run("merge preserves unset callbacks", func(t *testing.T) {
		originalCalled := false

		cb := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				originalCalled = true
			},
		}
		other := &NodeCallbacks{
			// OnTxReceived is not set
			OnBlockReceived: func(peerID peer.ID, height int64, hash, data []byte) {},
		}

		cb.Merge(other)

		// Original callback should still be there
		cb.OnTxReceived("test", nil)
		assert.True(t, originalCalled)

		// New callback should be added
		assert.NotNil(t, cb.OnBlockReceived)
	})

	t.Run("merge returns self for chaining", func(t *testing.T) {
		cb := &NodeCallbacks{}
		other := &NodeCallbacks{}

		result := cb.Merge(other)
		assert.Equal(t, cb, result)
	})
}

func TestCallbacks_SafeInvocation(t *testing.T) {
	t.Run("nil callbacks struct", func(t *testing.T) {
		var cb *NodeCallbacks

		// These should not panic
		cb.InvokeTxReceived("test", nil)
		cb.InvokeTxValidated(nil, nil)
		cb.InvokeTxBroadcast(nil, nil)
		cb.InvokeTxAdded(nil, nil)
		cb.InvokeTxRemoved(nil, "")
		cb.InvokeBlockReceived("test", 1, nil, nil)
		cb.InvokeBlockValidated(1, nil, nil)
		cb.InvokeBlockCommitted(1, nil)
		cb.InvokeBlockStored(1, nil)
		cb.InvokePeerConnected("test", true)
		cb.InvokePeerHandshaked("test", nil)
		cb.InvokePeerDisconnected("test")
		cb.InvokePeerPenalized("test", 10, "test")
		cb.InvokeConsensusMessage("test", nil)
		cb.InvokeSyncStarted(0, 100)
		cb.InvokeSyncProgress(50, 100)
		cb.InvokeSyncCompleted(100)
	})

	t.Run("nil callback functions", func(t *testing.T) {
		cb := &NodeCallbacks{}

		// These should not panic
		cb.InvokeTxReceived("test", nil)
		cb.InvokeTxValidated(nil, nil)
		cb.InvokeTxBroadcast(nil, nil)
		cb.InvokeTxAdded(nil, nil)
		cb.InvokeTxRemoved(nil, "")
		cb.InvokeBlockReceived("test", 1, nil, nil)
		cb.InvokeBlockValidated(1, nil, nil)
		cb.InvokeBlockCommitted(1, nil)
		cb.InvokeBlockStored(1, nil)
		cb.InvokePeerConnected("test", true)
		cb.InvokePeerHandshaked("test", nil)
		cb.InvokePeerDisconnected("test")
		cb.InvokePeerPenalized("test", 10, "test")
		cb.InvokeConsensusMessage("test", nil)
		cb.InvokeSyncStarted(0, 100)
		cb.InvokeSyncProgress(50, 100)
		cb.InvokeSyncCompleted(100)
	})

	t.Run("set callbacks are invoked", func(t *testing.T) {
		txReceivedCalled := false
		txValidatedCalled := false
		blockReceivedCalled := false
		peerConnectedCalled := false
		syncCompletedCalled := false

		cb := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				txReceivedCalled = true
			},
			OnTxValidated: func(tx []byte, err error) {
				txValidatedCalled = true
			},
			OnBlockReceived: func(peerID peer.ID, height int64, hash, data []byte) {
				blockReceivedCalled = true
			},
			OnPeerConnected: func(peerID peer.ID, isOutbound bool) {
				peerConnectedCalled = true
			},
			OnSyncCompleted: func(height int64) {
				syncCompletedCalled = true
			},
		}

		cb.InvokeTxReceived("test", []byte("tx"))
		cb.InvokeTxValidated([]byte("tx"), nil)
		cb.InvokeBlockReceived("test", 1, []byte("hash"), []byte("data"))
		cb.InvokePeerConnected("test", true)
		cb.InvokeSyncCompleted(100)

		assert.True(t, txReceivedCalled)
		assert.True(t, txValidatedCalled)
		assert.True(t, blockReceivedCalled)
		assert.True(t, peerConnectedCalled)
		assert.True(t, syncCompletedCalled)
	})

	t.Run("callback receives correct parameters", func(t *testing.T) {
		var receivedPeerID peer.ID
		var receivedTx []byte
		var receivedErr error

		cb := &NodeCallbacks{
			OnTxReceived: func(peerID peer.ID, tx []byte) {
				receivedPeerID = peerID
				receivedTx = tx
			},
			OnTxValidated: func(tx []byte, err error) {
				receivedErr = err
			},
		}

		expectedPeer := peer.ID("test-peer-id")
		expectedTx := []byte("test-transaction")
		expectedErr := errors.New("validation failed")

		cb.InvokeTxReceived(expectedPeer, expectedTx)
		cb.InvokeTxValidated(expectedTx, expectedErr)

		assert.Equal(t, expectedPeer, receivedPeerID)
		assert.Equal(t, expectedTx, receivedTx)
		assert.Equal(t, expectedErr, receivedErr)
	})
}

func TestPeerInfo(t *testing.T) {
	info := &PeerInfo{
		NodeID:          "abc123",
		ChainID:         "test-chain",
		ProtocolVersion: 1,
		Height:          100,
	}

	assert.Equal(t, "abc123", info.NodeID)
	assert.Equal(t, "test-chain", info.ChainID)
	assert.Equal(t, int32(1), info.ProtocolVersion)
	assert.Equal(t, int64(100), info.Height)
}

func TestCallbacks_AllInvokers(t *testing.T) {
	// Test that all invokers work correctly
	counts := make(map[string]int)

	cb := &NodeCallbacks{
		OnTxReceived:       func(peer.ID, []byte) { counts["TxReceived"]++ },
		OnTxValidated:      func([]byte, error) { counts["TxValidated"]++ },
		OnTxBroadcast:      func([]byte, []peer.ID) { counts["TxBroadcast"]++ },
		OnTxAdded:          func([]byte, []byte) { counts["TxAdded"]++ },
		OnTxRemoved:        func([]byte, string) { counts["TxRemoved"]++ },
		OnBlockReceived:    func(peer.ID, int64, []byte, []byte) { counts["BlockReceived"]++ },
		OnBlockValidated:   func(int64, []byte, error) { counts["BlockValidated"]++ },
		OnBlockCommitted:   func(int64, []byte) { counts["BlockCommitted"]++ },
		OnBlockStored:      func(int64, []byte) { counts["BlockStored"]++ },
		OnPeerConnected:    func(peer.ID, bool) { counts["PeerConnected"]++ },
		OnPeerHandshaked:   func(peer.ID, *PeerInfo) { counts["PeerHandshaked"]++ },
		OnPeerDisconnected: func(peer.ID) { counts["PeerDisconnected"]++ },
		OnPeerPenalized:    func(peer.ID, int64, string) { counts["PeerPenalized"]++ },
		OnConsensusMessage: func(peer.ID, []byte) { counts["ConsensusMessage"]++ },
		OnSyncStarted:      func(int64, int64) { counts["SyncStarted"]++ },
		OnSyncProgress:     func(int64, int64) { counts["SyncProgress"]++ },
		OnSyncCompleted:    func(int64) { counts["SyncCompleted"]++ },
	}

	// Invoke all
	cb.InvokeTxReceived("p", nil)
	cb.InvokeTxValidated(nil, nil)
	cb.InvokeTxBroadcast(nil, nil)
	cb.InvokeTxAdded(nil, nil)
	cb.InvokeTxRemoved(nil, "")
	cb.InvokeBlockReceived("p", 0, nil, nil)
	cb.InvokeBlockValidated(0, nil, nil)
	cb.InvokeBlockCommitted(0, nil)
	cb.InvokeBlockStored(0, nil)
	cb.InvokePeerConnected("p", false)
	cb.InvokePeerHandshaked("p", nil)
	cb.InvokePeerDisconnected("p")
	cb.InvokePeerPenalized("p", 0, "")
	cb.InvokeConsensusMessage("p", nil)
	cb.InvokeSyncStarted(0, 0)
	cb.InvokeSyncProgress(0, 0)
	cb.InvokeSyncCompleted(0)

	// Verify all were called exactly once
	expected := []string{
		"TxReceived", "TxValidated", "TxBroadcast", "TxAdded", "TxRemoved",
		"BlockReceived", "BlockValidated", "BlockCommitted", "BlockStored",
		"PeerConnected", "PeerHandshaked", "PeerDisconnected", "PeerPenalized",
		"ConsensusMessage", "SyncStarted", "SyncProgress", "SyncCompleted",
	}

	for _, name := range expected {
		assert.Equal(t, 1, counts[name], "callback %s should be called exactly once", name)
	}
}
