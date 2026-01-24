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

func TestNewHousekeepingReactor(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	require.NotNil(t, reactor)
	require.Equal(t, time.Second, reactor.probeInterval)
}

func TestHousekeepingReactor_StartStop(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, 100*time.Millisecond)

	// Should not be running initially
	require.False(t, reactor.IsRunning())

	// Start
	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.IsRunning())

	// Start again should be no-op
	err = reactor.Start()
	require.NoError(t, err)

	// Stop
	err = reactor.Stop()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())

	// Stop again should be no-op
	err = reactor.Stop()
	require.NoError(t, err)
}

func TestHousekeepingReactor_HandleMessageEmpty(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	err := reactor.HandleMessage(peer.ID("peer1"), nil)
	require.ErrorIs(t, err, types.ErrInvalidMessage)

	err = reactor.HandleMessage(peer.ID("peer1"), []byte{})
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestHousekeepingReactor_HandleMessageUnknownType(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	w := cramberry.GetWriter()
	w.WriteTypeID(255)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.ErrorIs(t, err, types.ErrUnknownMessageType)
}

func TestHousekeepingReactor_EncodeDecodeLatencyRequest(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	timestamp := time.Now().UnixNano()
	req := &schema.LatencyRequest{
		Timestamp: &timestamp,
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDLatencyRequest, req)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDLatencyRequest, typeID)

	// Decode message
	var decoded schema.LatencyRequest
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.NotNil(t, decoded.Timestamp)
	require.Equal(t, timestamp, *decoded.Timestamp)
}

func TestHousekeepingReactor_EncodeDecodeLatencyResponse(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	latency := int64(12345)
	resp := &schema.LatencyResponse{
		Latency: &latency,
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDLatencyResponse, resp)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDLatencyResponse, typeID)

	// Decode message
	var decoded schema.LatencyResponse
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.NotNil(t, decoded.Latency)
	require.Equal(t, latency, *decoded.Latency)
}

func TestHousekeepingReactor_HandleLatencyRequest(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	// Create request with timestamp from past
	timestamp := time.Now().Add(-10 * time.Millisecond).UnixNano()
	req := &schema.LatencyRequest{
		Timestamp: &timestamp,
	}

	// Encode request
	data, err := reactor.encodeMessage(TypeIDLatencyRequest, req)
	require.NoError(t, err)

	// Handle request (no network, so response won't be sent)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestHousekeepingReactor_HandleLatencyResponse(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	peerID := peer.ID("peer1")

	// Add a pending probe
	requestTime := time.Now().Add(-10 * time.Millisecond).UnixNano()
	reactor.mu.Lock()
	reactor.pendingProbes[peerID] = requestTime
	reactor.mu.Unlock()

	// Create response
	latency := int64(5000000) // 5ms in nanoseconds
	resp := &schema.LatencyResponse{
		Latency: &latency,
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDLatencyResponse, resp)
	require.NoError(t, err)

	// Handle response
	err = reactor.HandleMessage(peerID, data)
	require.NoError(t, err)

	// Pending probe should be cleared
	reactor.mu.RLock()
	_, exists := reactor.pendingProbes[peerID]
	reactor.mu.RUnlock()
	require.False(t, exists)
}

func TestHousekeepingReactor_HandleLatencyResponseNoPending(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	// Create response without pending probe
	latency := int64(5000000)
	resp := &schema.LatencyResponse{
		Latency: &latency,
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDLatencyResponse, resp)
	require.NoError(t, err)

	// Handle response - should not error
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestHousekeepingReactor_HandleFirewallRequest(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	endpoint := "127.0.0.1:8080"
	req := &schema.FirewallRequest{
		Endpoint: &endpoint,
	}

	// Encode request
	data, err := reactor.encodeMessage(TypeIDFirewallRequest, req)
	require.NoError(t, err)

	// Handle request (no network, so response won't be sent)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestHousekeepingReactor_HandleFirewallResponse(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	endpoint := "127.0.0.1:8080"
	resp := &schema.FirewallResponse{
		Endpoint: &endpoint,
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDFirewallResponse, resp)
	require.NoError(t, err)

	// Handle response (stub - just logs)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestHousekeepingReactor_OnPeerDisconnected(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	peerID := peer.ID("peer1")

	// Add pending probe
	reactor.mu.Lock()
	reactor.pendingProbes[peerID] = time.Now().UnixNano()
	reactor.mu.Unlock()

	require.Equal(t, 1, reactor.GetPendingProbes())

	// Disconnect peer
	reactor.OnPeerDisconnected(peerID)

	// Pending probe should be cleared
	require.Equal(t, 0, reactor.GetPendingProbes())
}

func TestHousekeepingReactor_TypeIDConstants(t *testing.T) {
	require.Equal(t, cramberry.TypeID(140), TypeIDLatencyRequest)
	require.Equal(t, cramberry.TypeID(141), TypeIDLatencyResponse)
	require.Equal(t, cramberry.TypeID(142), TypeIDFirewallRequest)
	require.Equal(t, cramberry.TypeID(143), TypeIDFirewallResponse)
}

func TestHousekeepingReactor_SendLatencyRequestNilNetwork(t *testing.T) {
	reactor := NewHousekeepingReactor(nil, nil, time.Second)

	// Should not error with nil network
	err := reactor.SendLatencyRequest(peer.ID("peer1"), time.Now().UnixNano())
	require.NoError(t, err)
}
