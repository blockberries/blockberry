package handlers

import (
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

// TestHandshakeState tests handshake state transitions.
func TestHandshakeState(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil, // Network not needed for state tests
		nil, // PeerManager not needed for state tests
		func() int64 { return 100 },
	)

	peerID := peer.ID("test-peer")

	t.Run("initial state", func(t *testing.T) {
		_, ok := h.GetPeerState(peerID)
		require.False(t, ok, "peer should not have state before connection")
	})

	t.Run("state after init", func(t *testing.T) {
		h.mu.Lock()
		h.states[peerID] = &PeerHandshakeState{State: StateInit}
		h.mu.Unlock()

		state, ok := h.GetPeerState(peerID)
		require.True(t, ok)
		require.Equal(t, StateInit, state)
	})

	t.Run("is handshake complete", func(t *testing.T) {
		require.False(t, h.IsHandshakeComplete(peerID))

		h.mu.Lock()
		h.states[peerID].State = StateComplete
		h.mu.Unlock()

		require.True(t, h.IsHandshakeComplete(peerID))
	})

	t.Run("peer count", func(t *testing.T) {
		require.Equal(t, 1, h.PeerCount())
	})

	t.Run("cleanup on disconnect", func(t *testing.T) {
		h.OnPeerDisconnected(peerID)
		require.Equal(t, 0, h.PeerCount())
	})
}

// TestEncodeDecodeHelloRequest tests HelloRequest encoding and decoding.
func TestEncodeDecodeHelloRequest(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	nodeID := "test-node"
	version := int32(1)
	chainID := "test-chain"
	timestamp := int64(12345)
	height := int64(100)

	req := &schema.HelloRequest{
		NodeId:       &nodeID,
		Version:      &version,
		ChainId:      &chainID,
		Timestamp:    &timestamp,
		LatestHeight: &height,
	}

	// Encode
	data, err := h.encodeHandshakeMessage(TypeIDHelloRequest, req)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDHelloRequest, typeID)

	// Decode message
	var decoded schema.HelloRequest
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.Equal(t, nodeID, *decoded.NodeId)
	require.Equal(t, version, *decoded.Version)
	require.Equal(t, chainID, *decoded.ChainId)
	require.Equal(t, timestamp, *decoded.Timestamp)
	require.Equal(t, height, *decoded.LatestHeight)
}

// TestEncodeDecodeHelloResponse tests HelloResponse encoding and decoding.
func TestEncodeDecodeHelloResponse(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	accepted := true
	pubKey := []byte("test-public-key")

	resp := &schema.HelloResponse{
		Accepted:  &accepted,
		PublicKey: pubKey,
	}

	// Encode
	data, err := h.encodeHandshakeMessage(TypeIDHelloResponse, resp)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDHelloResponse, typeID)

	// Decode message
	var decoded schema.HelloResponse
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.True(t, *decoded.Accepted)
	require.Equal(t, pubKey, decoded.PublicKey)
}

// TestEncodeDecodeHelloFinalize tests HelloFinalize encoding and decoding.
func TestEncodeDecodeHelloFinalize(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	success := true

	fin := &schema.HelloFinalize{
		Success: &success,
	}

	// Encode
	data, err := h.encodeHandshakeMessage(TypeIDHelloFinalize, fin)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDHelloFinalize, typeID)

	// Decode message
	var decoded schema.HelloFinalize
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.True(t, *decoded.Success)
}

// TestHandleHelloRequestValidation tests HelloRequest validation.
func TestHandleHelloRequestValidation(t *testing.T) {
	h := NewHandshakeHandler(
		"my-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil, // No network needed - will fail before sending
		nil,
		func() int64 { return 100 },
	)

	peerID := peer.ID("test-peer")

	// Initialize state for peer
	h.mu.Lock()
	h.states[peerID] = &PeerHandshakeState{SentRequest: true}
	h.mu.Unlock()

	t.Run("chain ID mismatch", func(t *testing.T) {
		nodeID := "peer-node"
		version := int32(1)
		wrongChainID := "wrong-chain"
		timestamp := int64(12345)
		height := int64(50)

		req := &schema.HelloRequest{
			NodeId:       &nodeID,
			Version:      &version,
			ChainId:      &wrongChainID,
			Timestamp:    &timestamp,
			LatestHeight: &height,
		}

		data, err := req.MarshalCramberry()
		require.NoError(t, err)

		// Should fail with chain ID mismatch (will also fail to blacklist due to nil network)
		err = h.handleHelloRequest(peerID, data)
		require.Error(t, err)
		require.ErrorIs(t, err, types.ErrChainIDMismatch)
	})

	t.Run("version mismatch", func(t *testing.T) {
		nodeID := "peer-node"
		wrongVersion := int32(99)
		chainID := "my-chain"
		timestamp := int64(12345)
		height := int64(50)

		req := &schema.HelloRequest{
			NodeId:       &nodeID,
			Version:      &wrongVersion,
			ChainId:      &chainID,
			Timestamp:    &timestamp,
			LatestHeight: &height,
		}

		data, err := req.MarshalCramberry()
		require.NoError(t, err)

		// Should fail with version mismatch
		err = h.handleHelloRequest(peerID, data)
		require.Error(t, err)
		require.ErrorIs(t, err, types.ErrVersionMismatch)
	})

	t.Run("missing required field", func(t *testing.T) {
		nodeID := "peer-node"
		// Missing version, chain_id, etc.
		req := &schema.HelloRequest{
			NodeId: &nodeID,
		}

		data, err := req.MarshalCramberry()
		require.NoError(t, err)

		err = h.handleHelloRequest(peerID, data)
		require.Error(t, err)
	})
}

// TestHandleHelloResponseValidation tests HelloResponse validation.
func TestHandleHelloResponseValidation(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	peerID := peer.ID("test-peer")

	t.Run("creates state if needed", func(t *testing.T) {
		accepted := true
		// Use valid 32-byte ed25519 public key
		validPubKey := make([]byte, 32)
		for i := range validPubKey {
			validPubKey[i] = byte(i)
		}
		resp := &schema.HelloResponse{
			Accepted:  &accepted,
			PublicKey: validPubKey,
		}

		data, err := resp.MarshalCramberry()
		require.NoError(t, err)

		// With nil network, PrepareStreams is skipped but state is created
		err = h.handleHelloResponse(peerID, data)
		require.NoError(t, err)

		h.mu.RLock()
		state := h.states[peerID]
		h.mu.RUnlock()
		require.NotNil(t, state)
		require.True(t, state.ReceivedResponse)
	})

	t.Run("rejection response", func(t *testing.T) {
		anotherPeerID := peer.ID("another-peer")
		// Set up state
		h.mu.Lock()
		h.states[anotherPeerID] = &PeerHandshakeState{SentRequest: true}
		h.mu.Unlock()

		accepted := false
		resp := &schema.HelloResponse{
			Accepted: &accepted,
		}

		data, err := resp.MarshalCramberry()
		require.NoError(t, err)

		// Should fail because peer rejected (will also fail to disconnect due to nil network)
		err = h.handleHelloResponse(anotherPeerID, data)
		require.Error(t, err)
		require.ErrorIs(t, err, types.ErrHandshakeFailed)
	})
}

// TestHandleHelloFinalizeValidation tests HelloFinalize validation.
func TestHandleHelloFinalizeValidation(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	peerID := peer.ID("test-peer")

	t.Run("creates state if needed and marks received", func(t *testing.T) {
		success := true
		fin := &schema.HelloFinalize{
			Success: &success,
		}

		data, err := fin.MarshalCramberry()
		require.NoError(t, err)

		err = h.handleHelloFinalize(peerID, data)
		require.NoError(t, err)

		h.mu.RLock()
		state := h.states[peerID]
		h.mu.RUnlock()
		require.NotNil(t, state)
		require.True(t, state.ReceivedFinalize)
	})

	t.Run("failure response", func(t *testing.T) {
		anotherPeerID := peer.ID("another-peer")
		h.mu.Lock()
		h.states[anotherPeerID] = &PeerHandshakeState{SentFinalize: true}
		h.mu.Unlock()

		success := false
		fin := &schema.HelloFinalize{
			Success: &success,
		}

		data, err := fin.MarshalCramberry()
		require.NoError(t, err)

		// Should fail because peer indicated failure (will also fail to disconnect due to nil network)
		err = h.handleHelloFinalize(anotherPeerID, data)
		require.Error(t, err)
		require.ErrorIs(t, err, types.ErrHandshakeFailed)
	})

	t.Run("early finalize - marks flag for later completion", func(t *testing.T) {
		earlyPeerID := peer.ID("early-peer")
		h.mu.Lock()
		h.states[earlyPeerID] = &PeerHandshakeState{ReceivedRequest: true}
		h.mu.Unlock()

		success := true
		fin := &schema.HelloFinalize{
			Success: &success,
		}

		data, err := fin.MarshalCramberry()
		require.NoError(t, err)

		// Should not error - just mark that we received their finalize
		err = h.handleHelloFinalize(earlyPeerID, data)
		require.NoError(t, err)

		h.mu.RLock()
		state := h.states[earlyPeerID]
		h.mu.RUnlock()
		require.True(t, state.ReceivedFinalize)
	})
}

// TestHandleMessageDispatch tests message type dispatch.
func TestHandleMessageDispatch(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	peerID := peer.ID("test-peer")

	t.Run("empty message", func(t *testing.T) {
		err := h.HandleMessage(peerID, []byte{})
		require.ErrorIs(t, err, types.ErrInvalidMessage)
	})

	t.Run("unknown type ID", func(t *testing.T) {
		// Write an unknown type ID
		w := cramberry.GetWriter()
		w.WriteTypeID(999)
		w.WriteRawBytes([]byte{0x00})
		data := w.BytesCopy()
		cramberry.PutWriter(w)

		err := h.HandleMessage(peerID, data)
		require.Error(t, err)
		require.ErrorIs(t, err, types.ErrInvalidMessage)
	})

	t.Run("valid HelloRequest dispatch", func(t *testing.T) {
		// Set up the handler state
		h.mu.Lock()
		h.states[peerID] = &PeerHandshakeState{SentRequest: true}
		h.mu.Unlock()

		nodeID := "peer-node"
		version := int32(1)
		chainID := "test-chain"
		timestamp := int64(12345)
		height := int64(50)

		req := &schema.HelloRequest{
			NodeId:       &nodeID,
			Version:      &version,
			ChainId:      &chainID,
			Timestamp:    &timestamp,
			LatestHeight: &height,
		}

		data, err := h.encodeHandshakeMessage(TypeIDHelloRequest, req)
		require.NoError(t, err)

		// With nil network, message handling succeeds (network calls are skipped)
		err = h.HandleMessage(peerID, data)
		require.NoError(t, err)

		// Verify state was updated
		h.mu.RLock()
		state := h.states[peerID]
		h.mu.RUnlock()
		require.True(t, state.ReceivedRequest)
		require.True(t, state.SentResponse)
		require.Equal(t, nodeID, state.PeerNodeID)
		require.Equal(t, height, state.PeerHeight)
	})
}

// TestHandshakeConstants verifies the type ID constants match schema.
func TestHandshakeConstants(t *testing.T) {
	// Verify our constants match the schema-defined type IDs
	require.Equal(t, cramberry.TypeID(128), TypeIDHelloRequest)
	require.Equal(t, cramberry.TypeID(129), TypeIDHelloResponse)
	require.Equal(t, cramberry.TypeID(130), TypeIDHelloFinalize)

	// Verify schema type IDs match
	req := &schema.HelloRequest{}
	resp := &schema.HelloResponse{}
	fin := &schema.HelloFinalize{}

	require.Equal(t, TypeIDHelloRequest, schema.HandshakeMessageTypeID(req))
	require.Equal(t, TypeIDHelloResponse, schema.HandshakeMessageTypeID(resp))
	require.Equal(t, TypeIDHelloFinalize, schema.HandshakeMessageTypeID(fin))
}

func TestHandshakeHandler_DefaultTimeouts(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	require.Equal(t, DefaultHandshakeTimeout, h.timeout)
	require.Equal(t, DefaultHandshakeCheckInterval, h.checkInterval)
}

func TestHandshakeHandler_StartStop(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil,
		nil,
		func() int64 { return 100 },
	)

	// Should not be running initially
	require.False(t, h.IsRunning())

	// Start
	err := h.Start()
	require.NoError(t, err)
	require.True(t, h.IsRunning())

	// Start again should be no-op
	err = h.Start()
	require.NoError(t, err)

	// Stop
	err = h.Stop()
	require.NoError(t, err)
	require.False(t, h.IsRunning())

	// Stop again should be no-op
	err = h.Stop()
	require.NoError(t, err)
}

func TestHandshakeHandler_CleanupStaleHandshakes(t *testing.T) {
	h := NewHandshakeHandler(
		"test-chain",
		1,
		"node-1",
		[]byte("pubkey-1"),
		nil, // Network nil so disconnect won't be called
		nil,
		func() int64 { return 100 },
	)
	h.timeout = 100 * time.Millisecond // Short timeout for testing

	peer1 := peer.ID("stale-peer")
	peer2 := peer.ID("fresh-peer")
	peer3 := peer.ID("complete-peer")

	// Add some handshake states
	h.mu.Lock()
	now := time.Now()
	// Stale incomplete handshake (old)
	h.states[peer1] = &PeerHandshakeState{
		State:     StateInit,
		StartedAt: now.Add(-200 * time.Millisecond), // Older than timeout
	}
	// Fresh incomplete handshake
	h.states[peer2] = &PeerHandshakeState{
		State:     StateInit,
		StartedAt: now, // Fresh
	}
	// Complete handshake (old, but should not be cleaned up)
	h.states[peer3] = &PeerHandshakeState{
		State:     StateComplete,
		StartedAt: now.Add(-200 * time.Millisecond), // Old but complete
	}
	h.mu.Unlock()

	// Run cleanup
	h.cleanupStaleHandshakes()

	// Give time for async disconnect (which won't happen with nil network)
	time.Sleep(10 * time.Millisecond)

	// Check results
	h.mu.RLock()
	defer h.mu.RUnlock()

	// peer1 should be removed (stale incomplete)
	_, exists1 := h.states[peer1]
	require.False(t, exists1, "stale incomplete handshake should be removed")

	// peer2 should still exist (fresh)
	_, exists2 := h.states[peer2]
	require.True(t, exists2, "fresh handshake should remain")

	// peer3 should still exist (complete, even though old)
	_, exists3 := h.states[peer3]
	require.True(t, exists3, "complete handshake should remain regardless of age")
}

func TestHandshakeHandler_TimeoutConstants(t *testing.T) {
	// Verify the default timeout constants are reasonable
	require.Equal(t, 30*time.Second, DefaultHandshakeTimeout)
	require.Equal(t, 5*time.Second, DefaultHandshakeCheckInterval)
}
