package p2p

import (
	"errors"
	"sync"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  StreamConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  StreamConfig{Name: "test", Encrypted: true, RateLimit: 100, MaxMessageSize: 1024},
			wantErr: false,
		},
		{
			name:    "valid config with defaults",
			config:  StreamConfig{Name: "test"},
			wantErr: false,
		},
		{
			name:    "empty name",
			config:  StreamConfig{Name: ""},
			wantErr: true,
		},
		{
			name:    "negative rate limit",
			config:  StreamConfig{Name: "test", RateLimit: -1},
			wantErr: true,
		},
		{
			name:    "negative max message size",
			config:  StreamConfig{Name: "test", MaxMessageSize: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidStreamConfig))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStreamConfig_Clone(t *testing.T) {
	original := StreamConfig{
		Name:           "test",
		Encrypted:      true,
		MessageTypes:   []uint16{1, 2, 3},
		RateLimit:      100,
		MaxMessageSize: 1024,
		Owner:          "mempool",
	}

	clone := original.Clone()

	// Verify values are equal
	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.Encrypted, clone.Encrypted)
	assert.Equal(t, original.MessageTypes, clone.MessageTypes)
	assert.Equal(t, original.RateLimit, clone.RateLimit)
	assert.Equal(t, original.MaxMessageSize, clone.MaxMessageSize)
	assert.Equal(t, original.Owner, clone.Owner)

	// Verify MessageTypes is a deep copy
	clone.MessageTypes[0] = 99
	assert.NotEqual(t, original.MessageTypes[0], clone.MessageTypes[0])
}

func TestStreamConfig_CloneNilMessageTypes(t *testing.T) {
	original := StreamConfig{Name: "test"}
	clone := original.Clone()
	assert.Nil(t, clone.MessageTypes)
}

func TestNewStreamRegistry(t *testing.T) {
	registry := NewStreamRegistry()
	require.NotNil(t, registry)
	assert.Equal(t, 0, registry.Count())
}

func TestStreamRegistry_Register(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Register(StreamConfig{Name: "test", Encrypted: true})
	require.NoError(t, err)

	assert.True(t, registry.Has("test"))
	assert.Equal(t, 1, registry.Count())
}

func TestStreamRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Register(StreamConfig{Name: "test"})
	require.NoError(t, err)

	err = registry.Register(StreamConfig{Name: "test"})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrStreamAlreadyRegistered))
}

func TestStreamRegistry_RegisterInvalid(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Register(StreamConfig{Name: ""})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidStreamConfig))
}

func TestStreamRegistry_Unregister(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Register(StreamConfig{Name: "test"})
	require.NoError(t, err)

	err = registry.Unregister("test")
	require.NoError(t, err)

	assert.False(t, registry.Has("test"))
	assert.Equal(t, 0, registry.Count())
}

func TestStreamRegistry_UnregisterNotFound(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Unregister("nonexistent")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrStreamNotFound))
}

func TestStreamRegistry_UnregisterWithHandler(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Register(StreamConfig{Name: "test"})
	require.NoError(t, err)

	err = registry.RegisterHandler("test", func(peer.ID, []byte) error { return nil })
	require.NoError(t, err)

	err = registry.Unregister("test")
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrStreamInUse))
}

func TestStreamRegistry_Get(t *testing.T) {
	registry := NewStreamRegistry()

	original := StreamConfig{
		Name:           "test",
		Encrypted:      true,
		RateLimit:      100,
		MaxMessageSize: 1024,
		Owner:          "mempool",
	}
	err := registry.Register(original)
	require.NoError(t, err)

	retrieved := registry.Get("test")
	require.NotNil(t, retrieved)

	assert.Equal(t, original.Name, retrieved.Name)
	assert.Equal(t, original.Encrypted, retrieved.Encrypted)
	assert.Equal(t, original.RateLimit, retrieved.RateLimit)
	assert.Equal(t, original.MaxMessageSize, retrieved.MaxMessageSize)
	assert.Equal(t, original.Owner, retrieved.Owner)
}

func TestStreamRegistry_GetNotFound(t *testing.T) {
	registry := NewStreamRegistry()

	retrieved := registry.Get("nonexistent")
	assert.Nil(t, retrieved)
}

func TestStreamRegistry_GetReturnsClone(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.Register(StreamConfig{Name: "test", RateLimit: 100})
	require.NoError(t, err)

	retrieved := registry.Get("test")
	require.NotNil(t, retrieved)

	// Modify the retrieved config
	retrieved.RateLimit = 999

	// Original should be unchanged
	original := registry.Get("test")
	assert.Equal(t, 100, original.RateLimit)
}

func TestStreamRegistry_All(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "test1"})
	_ = registry.Register(StreamConfig{Name: "test2"})
	_ = registry.Register(StreamConfig{Name: "test3"})

	all := registry.All()
	assert.Len(t, all, 3)

	names := make(map[string]bool)
	for _, cfg := range all {
		names[cfg.Name] = true
	}
	assert.True(t, names["test1"])
	assert.True(t, names["test2"])
	assert.True(t, names["test3"])
}

func TestStreamRegistry_AllEmpty(t *testing.T) {
	registry := NewStreamRegistry()

	all := registry.All()
	assert.Empty(t, all)
}

func TestStreamRegistry_Names(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "test1"})
	_ = registry.Register(StreamConfig{Name: "test2"})

	names := registry.Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "test1")
	assert.Contains(t, names, "test2")
}

func TestStreamRegistry_Has(t *testing.T) {
	registry := NewStreamRegistry()

	assert.False(t, registry.Has("test"))

	_ = registry.Register(StreamConfig{Name: "test"})
	assert.True(t, registry.Has("test"))
}

func TestStreamRegistry_RegisterHandler(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "test"})

	called := false
	handler := func(peer.ID, []byte) error {
		called = true
		return nil
	}

	err := registry.RegisterHandler("test", handler)
	require.NoError(t, err)

	retrieved := registry.GetHandler("test")
	require.NotNil(t, retrieved)

	// Call the handler to verify it's the same one
	_ = retrieved("", nil)
	assert.True(t, called)
}

func TestStreamRegistry_RegisterHandlerNotFound(t *testing.T) {
	registry := NewStreamRegistry()

	err := registry.RegisterHandler("nonexistent", func(peer.ID, []byte) error { return nil })
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrStreamNotFound))
}

func TestStreamRegistry_RegisterHandlerNil(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "test"})
	_ = registry.RegisterHandler("test", func(peer.ID, []byte) error { return nil })

	// Setting nil handler removes it
	err := registry.RegisterHandler("test", nil)
	require.NoError(t, err)

	retrieved := registry.GetHandler("test")
	assert.Nil(t, retrieved)

	// Now unregister should work
	err = registry.Unregister("test")
	assert.NoError(t, err)
}

func TestStreamRegistry_GetHandlerNotFound(t *testing.T) {
	registry := NewStreamRegistry()

	retrieved := registry.GetHandler("nonexistent")
	assert.Nil(t, retrieved)
}

func TestStreamRegistry_ByOwner(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "mempool1", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "mempool2", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "consensus1", Owner: "consensus"})

	mempoolStreams := registry.ByOwner("mempool")
	assert.Len(t, mempoolStreams, 2)

	consensusStreams := registry.ByOwner("consensus")
	assert.Len(t, consensusStreams, 1)

	unknownStreams := registry.ByOwner("unknown")
	assert.Empty(t, unknownStreams)
}

func TestStreamRegistry_UnregisterByOwner(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "mempool1", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "mempool2", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "consensus1", Owner: "consensus"})

	count := registry.UnregisterByOwner("mempool")
	assert.Equal(t, 2, count)

	assert.False(t, registry.Has("mempool1"))
	assert.False(t, registry.Has("mempool2"))
	assert.True(t, registry.Has("consensus1"))
}

func TestStreamRegistry_UnregisterByOwnerSkipsWithHandler(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "mempool1", Owner: "mempool"})
	_ = registry.Register(StreamConfig{Name: "mempool2", Owner: "mempool"})
	_ = registry.RegisterHandler("mempool1", func(peer.ID, []byte) error { return nil })

	count := registry.UnregisterByOwner("mempool")
	assert.Equal(t, 1, count) // Only mempool2 unregistered

	assert.True(t, registry.Has("mempool1"))  // Still has handler
	assert.False(t, registry.Has("mempool2")) // Removed
}

func TestStreamRegistry_ForceUnregister(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "test"})
	_ = registry.RegisterHandler("test", func(peer.ID, []byte) error { return nil })

	// Normal unregister should fail
	err := registry.Unregister("test")
	assert.Error(t, err)

	// Force unregister should succeed
	removed := registry.ForceUnregister("test")
	assert.True(t, removed)
	assert.False(t, registry.Has("test"))
}

func TestStreamRegistry_ForceUnregisterNotFound(t *testing.T) {
	registry := NewStreamRegistry()

	removed := registry.ForceUnregister("nonexistent")
	assert.False(t, removed)
}

func TestStreamRegistry_Clear(t *testing.T) {
	registry := NewStreamRegistry()

	_ = registry.Register(StreamConfig{Name: "test1"})
	_ = registry.Register(StreamConfig{Name: "test2"})
	assert.Equal(t, 2, registry.Count())

	registry.Clear()
	assert.Equal(t, 0, registry.Count())
}

func TestStreamRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewStreamRegistry()

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrent registrations
	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "stream" + string(rune('a'+idx%26))
			_ = registry.Register(StreamConfig{Name: name})
		}(i)
	}

	// Concurrent reads
	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = registry.All()
			_ = registry.Names()
			_ = registry.Count()
		}()
	}

	wg.Wait()
	// Just verify no panics occurred
}

func TestRegisterBuiltinStreams(t *testing.T) {
	registry := NewStreamRegistry()

	err := RegisterBuiltinStreams(registry)
	require.NoError(t, err)

	// Verify all built-in streams are registered
	assert.True(t, registry.Has(StreamPEX))
	assert.True(t, registry.Has(StreamTransactions))
	assert.True(t, registry.Has(StreamBlockSync))
	assert.True(t, registry.Has(StreamBlocks))
	assert.True(t, registry.Has(StreamConsensus))
	assert.True(t, registry.Has(StreamHousekeeping))
	assert.Equal(t, 6, registry.Count())
}

func TestRegisterBuiltinStreams_Duplicate(t *testing.T) {
	registry := NewStreamRegistry()

	// Pre-register one stream
	_ = registry.Register(StreamConfig{Name: StreamPEX})

	err := RegisterBuiltinStreams(registry)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), StreamPEX)
}

func TestStreamRegistry_InterfaceCompliance(t *testing.T) {
	var _ StreamRegistry = (*InMemoryStreamRegistry)(nil)
}

func TestStreamConfig_ValidateZeroValues(t *testing.T) {
	// Zero values for RateLimit and MaxMessageSize are valid (means unlimited/default)
	cfg := StreamConfig{Name: "test", RateLimit: 0, MaxMessageSize: 0}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestStreamHandler_Type(t *testing.T) {
	// Verify the handler signature works as expected
	var handler StreamHandler = func(peerID peer.ID, data []byte) error {
		return nil
	}
	assert.NotNil(t, handler)

	err := handler("", nil)
	assert.NoError(t, err)
}

func TestStreamHandler_ReturnsError(t *testing.T) {
	expectedErr := errors.New("test error")
	var handler StreamHandler = func(peer.ID, []byte) error {
		return expectedErr
	}

	err := handler("", nil)
	assert.Equal(t, expectedErr, err)
}
