package p2p

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/blockberries/glueberry/pkg/streams"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGlueberryStreamAdapter(t *testing.T) {
	registry := NewStreamRegistry()
	adapter := NewGlueberryStreamAdapter(registry)
	require.NotNil(t, adapter)
	assert.Equal(t, registry, adapter.Registry())
}

func TestGlueberryStreamAdapter_GetEncryptedStreamNames(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "encrypted1", Encrypted: true})
	_ = registry.Register(StreamConfig{Name: "encrypted2", Encrypted: true})
	_ = registry.Register(StreamConfig{Name: "unencrypted", Encrypted: false})

	adapter := NewGlueberryStreamAdapter(registry)
	names := adapter.GetEncryptedStreamNames()

	assert.Len(t, names, 2)
	assert.Contains(t, names, "encrypted1")
	assert.Contains(t, names, "encrypted2")
	assert.NotContains(t, names, "unencrypted")
}

func TestGlueberryStreamAdapter_GetAllStreamNames(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "stream1"})
	_ = registry.Register(StreamConfig{Name: "stream2"})

	adapter := NewGlueberryStreamAdapter(registry)
	names := adapter.GetAllStreamNames()

	assert.Len(t, names, 2)
	assert.Contains(t, names, "stream1")
	assert.Contains(t, names, "stream2")
}

func TestGlueberryStreamAdapter_RouteMessage(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test"})

	var called bool
	var receivedData []byte
	_ = registry.RegisterHandler("test", func(_ peer.ID, data []byte) error {
		called = true
		receivedData = data
		return nil
	})

	adapter := NewGlueberryStreamAdapter(registry)

	msg := streams.IncomingMessage{
		PeerID:     "peer1",
		StreamName: "test",
		Data:       []byte("hello"),
	}

	err := adapter.RouteMessage(msg)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, []byte("hello"), receivedData)
}

func TestGlueberryStreamAdapter_RouteMessage_NoHandler(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test"})

	adapter := NewGlueberryStreamAdapter(registry)

	msg := streams.IncomingMessage{
		PeerID:     "peer1",
		StreamName: "test",
		Data:       []byte("hello"),
	}

	err := adapter.RouteMessage(msg)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrStreamHandlerNotSet))
}

func TestGlueberryStreamAdapter_RouteMessage_StreamNotFound(t *testing.T) {
	registry := NewStreamRegistry()
	adapter := NewGlueberryStreamAdapter(registry)

	msg := streams.IncomingMessage{
		PeerID:     "peer1",
		StreamName: "nonexistent",
		Data:       []byte("hello"),
	}

	err := adapter.RouteMessage(msg)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrStreamNotFound))
}

func TestGlueberryStreamAdapter_RegisterStream(t *testing.T) {
	registry := NewStreamRegistry()
	adapter := NewGlueberryStreamAdapter(registry)

	err := adapter.RegisterStream(StreamConfig{Name: "test", Encrypted: true})
	require.NoError(t, err)

	assert.True(t, adapter.HasStream("test"))
}

func TestGlueberryStreamAdapter_UnregisterStream(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test"})

	adapter := NewGlueberryStreamAdapter(registry)

	err := adapter.UnregisterStream("test")
	require.NoError(t, err)

	assert.False(t, adapter.HasStream("test"))
}

func TestGlueberryStreamAdapter_SetHandler(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test"})

	adapter := NewGlueberryStreamAdapter(registry)

	handler := func(peer.ID, []byte) error { return nil }
	err := adapter.SetHandler("test", handler)
	require.NoError(t, err)

	retrieved := registry.GetHandler("test")
	assert.NotNil(t, retrieved)
}

func TestGlueberryStreamAdapter_GetStreamConfig(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test", RateLimit: 100})

	adapter := NewGlueberryStreamAdapter(registry)

	cfg := adapter.GetStreamConfig("test")
	require.NotNil(t, cfg)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, 100, cfg.RateLimit)
}

func TestGlueberryStreamAdapter_StreamsByOwner(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "mempool1", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "mempool2", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "consensus1", Owner: "consensus"})

	adapter := NewGlueberryStreamAdapter(registry)

	mempoolStreams := adapter.StreamsByOwner("mempool")
	assert.Len(t, mempoolStreams, 2)

	consensusStreams := adapter.StreamsByOwner("consensus")
	assert.Len(t, consensusStreams, 1)
}

func TestGlueberryStreamAdapter_UnregisterByOwner(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "mempool1", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "mempool2", Owner: "mempool"})

	adapter := NewGlueberryStreamAdapter(registry)

	count := adapter.UnregisterByOwner("mempool")
	assert.Equal(t, 2, count)
	assert.False(t, adapter.HasStream("mempool1"))
	assert.False(t, adapter.HasStream("mempool2"))
}

func TestObservableStreamAdapter_Callbacks(t *testing.T) {
	registry := NewStreamRegistry()
	adapter := NewObservableStreamAdapter(registry)

	var registeredName string
	var registeredStatus bool
	adapter.AddCallback(func(name string, registered bool) {
		registeredName = name
		registeredStatus = registered
	})

	// Test registration callback
	err := adapter.RegisterStreamWithCallback(StreamConfig{Name: "test"})
	require.NoError(t, err)
	assert.Equal(t, "test", registeredName)
	assert.True(t, registeredStatus)

	// Test unregistration callback
	err = adapter.UnregisterStreamWithCallback("test")
	require.NoError(t, err)
	assert.Equal(t, "test", registeredName)
	assert.False(t, registeredStatus)
}

func TestStreamRouter_Route(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test", MaxMessageSize: 1024})

	var called bool
	_ = registry.RegisterHandler("test", func(peer.ID, []byte) error {
		called = true
		return nil
	})

	adapter := NewGlueberryStreamAdapter(registry)
	router := NewStreamRouter(adapter, nil)

	msg := streams.IncomingMessage{
		PeerID:     "peer1",
		StreamName: "test",
		Data:       []byte("hello"),
	}

	err := router.Route(msg)
	require.NoError(t, err)
	assert.True(t, called)
}

func TestStreamRouter_Route_MessageTooLarge(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test", MaxMessageSize: 10})
	_ = registry.RegisterHandler("test", func(peer.ID, []byte) error { return nil })

	adapter := NewGlueberryStreamAdapter(registry)
	router := NewStreamRouter(adapter, nil)

	msg := streams.IncomingMessage{
		PeerID:     "peer1",
		StreamName: "test",
		Data:       make([]byte, 100), // Larger than MaxMessageSize
	}

	err := router.Route(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "message too large")
}

func TestStreamRouter_Route_WithRateLimiting(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test", RateLimit: 100})

	var callCount atomic.Int32
	_ = registry.RegisterHandler("test", func(peer.ID, []byte) error {
		callCount.Add(1)
		return nil
	})

	adapter := NewGlueberryStreamAdapter(registry)
	rateLimiter := NewRateLimiter(RateLimiterConfig{
		Limits: RateLimits{
			MessagesPerSecond: map[string]float64{"test": 1}, // 1 per second
			BurstSize:         1,
		},
	})
	defer rateLimiter.Stop()

	router := NewStreamRouter(adapter, rateLimiter)

	msg := streams.IncomingMessage{
		PeerID:     "peer1",
		StreamName: "test",
		Data:       []byte("hello"),
	}

	// First message should succeed
	err := router.Route(msg)
	require.NoError(t, err)
	assert.Equal(t, int32(1), callCount.Load())

	// Second message should be rate limited
	err = router.Route(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limit exceeded")
	assert.Equal(t, int32(1), callCount.Load())
}

func TestStreamRouter_RouteWithPeer(t *testing.T) {
	registry := NewStreamRegistry()
	_ = registry.Register(StreamConfig{Name: "test"})

	var receivedPeerID peer.ID
	var receivedData []byte
	_ = registry.RegisterHandler("test", func(peerID peer.ID, data []byte) error {
		receivedPeerID = peerID
		receivedData = data
		return nil
	})

	adapter := NewGlueberryStreamAdapter(registry)
	router := NewStreamRouter(adapter, nil)

	err := router.RouteWithPeer("peer123", "test", []byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, peer.ID("peer123"), receivedPeerID)
	assert.Equal(t, []byte("hello"), receivedData)
}
