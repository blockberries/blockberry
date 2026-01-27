package mempool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/config"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)

	// Verify built-in types are registered
	assert.True(t, f.Has(TypeSimple))
	assert.True(t, f.Has(TypePriority))
	assert.True(t, f.Has(TypeTTL))

	// Verify looseberry is not registered by default (external plugin)
	assert.False(t, f.Has(TypeLooseberry))
}

func TestFactory_Register(t *testing.T) {
	f := NewFactory()

	// Register a custom mempool
	customCalled := false
	f.Register("custom", func(cfg *config.MempoolConfig) (Mempool, error) {
		customCalled = true
		return NewSimpleMempool(cfg.MaxTxs, cfg.MaxBytes), nil
	})

	assert.True(t, f.Has("custom"))

	// Create custom mempool
	cfg := &config.MempoolConfig{
		Type:     "custom",
		MaxTxs:   100,
		MaxBytes: 1024,
	}

	mp, err := f.Create(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)
	assert.True(t, customCalled)
}

func TestFactory_Unregister(t *testing.T) {
	f := NewFactory()

	// Verify simple is registered
	assert.True(t, f.Has(TypeSimple))

	// Unregister it
	f.Unregister(TypeSimple)

	// Verify it's gone
	assert.False(t, f.Has(TypeSimple))

	// Attempt to create should fail
	cfg := &config.MempoolConfig{Type: "simple"}
	_, err := f.Create(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mempool type")
}

func TestFactory_Create_Simple(t *testing.T) {
	f := NewFactory()

	cfg := &config.MempoolConfig{
		Type:     "simple",
		MaxTxs:   100,
		MaxBytes: 1024 * 1024,
	}

	mp, err := f.Create(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)

	// Verify it's a working mempool
	mp.SetTxValidator(AcceptAllTxValidator)
	err = mp.AddTx([]byte("test tx"))
	require.NoError(t, err)
	assert.Equal(t, 1, mp.Size())
}

func TestFactory_Create_Priority(t *testing.T) {
	f := NewFactory()

	cfg := &config.MempoolConfig{
		Type:     "priority",
		MaxTxs:   100,
		MaxBytes: 1024 * 1024,
	}

	mp, err := f.Create(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)

	// Verify it's a working mempool
	mp.SetTxValidator(AcceptAllTxValidator)
	err = mp.AddTx([]byte("test tx"))
	require.NoError(t, err)
	assert.Equal(t, 1, mp.Size())

	// Verify it's actually a PriorityMempool
	_, ok := mp.(*PriorityMempool)
	assert.True(t, ok, "expected PriorityMempool type")
}

func TestFactory_Create_TTL(t *testing.T) {
	f := NewFactory()

	cfg := &config.MempoolConfig{
		Type:            "ttl",
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             config.Duration(time.Hour),
		CleanupInterval: config.Duration(time.Minute),
	}

	mp, err := f.Create(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)

	// Verify it's a working mempool
	mp.SetTxValidator(AcceptAllTxValidator)
	err = mp.AddTx([]byte("test tx"))
	require.NoError(t, err)
	assert.Equal(t, 1, mp.Size())

	// Verify it's actually a TTLMempool
	ttlMp, ok := mp.(*TTLMempool)
	assert.True(t, ok, "expected TTLMempool type")

	// Stop cleanup goroutine
	if ttlMp != nil {
		ttlMp.Stop()
	}
}

func TestFactory_Create_Unknown(t *testing.T) {
	f := NewFactory()

	cfg := &config.MempoolConfig{
		Type: "unknown",
	}

	_, err := f.Create(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mempool type: unknown")
}

func TestFactory_Types(t *testing.T) {
	f := NewFactory()

	types := f.Types()
	assert.Len(t, types, 3) // simple, priority, ttl

	// Convert to map for easier checking
	typeMap := make(map[MempoolType]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	assert.True(t, typeMap[TypeSimple])
	assert.True(t, typeMap[TypePriority])
	assert.True(t, typeMap[TypeTTL])
}

func TestFactory_Override(t *testing.T) {
	f := NewFactory()

	// Override simple with custom implementation
	customCalled := false
	f.Register(TypeSimple, func(cfg *config.MempoolConfig) (Mempool, error) {
		customCalled = true
		return NewSimpleMempool(999, 999), nil
	})

	cfg := &config.MempoolConfig{
		Type:     "simple",
		MaxTxs:   100,
		MaxBytes: 1024,
	}

	mp, err := f.Create(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)
	assert.True(t, customCalled, "custom constructor should be called")
}

func TestDefaultFactory(t *testing.T) {
	// Verify default factory exists and works
	require.NotNil(t, DefaultFactory)

	assert.True(t, DefaultFactory.Has(TypeSimple))
	assert.True(t, DefaultFactory.Has(TypePriority))
	assert.True(t, DefaultFactory.Has(TypeTTL))
}

func TestCreateFromConfig(t *testing.T) {
	cfg := &config.MempoolConfig{
		Type:     "simple",
		MaxTxs:   50,
		MaxBytes: 512,
	}

	mp, err := CreateFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)
}

func TestRegisterMempool(t *testing.T) {
	// Store original state
	originalHas := DefaultFactory.Has("test-mempool")
	defer func() {
		// Cleanup
		DefaultFactory.Unregister("test-mempool")
	}()

	assert.False(t, originalHas)

	// Register new mempool type
	RegisterMempool("test-mempool", func(cfg *config.MempoolConfig) (Mempool, error) {
		return NewSimpleMempool(cfg.MaxTxs, cfg.MaxBytes), nil
	})

	assert.True(t, DefaultFactory.Has("test-mempool"))
}

func TestFactory_ConcurrentAccess(t *testing.T) {
	f := NewFactory()

	done := make(chan bool)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = f.Has(TypeSimple)
				_ = f.Types()
			}
			done <- true
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(i int) {
			typeName := MempoolType("concurrent-" + string(rune('a'+i)))
			for j := 0; j < 100; j++ {
				f.Register(typeName, func(cfg *config.MempoolConfig) (Mempool, error) {
					return NewSimpleMempool(1, 1), nil
				})
				f.Unregister(typeName)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestNewSimpleMempoolFromConfig(t *testing.T) {
	cfg := &config.MempoolConfig{
		MaxTxs:   200,
		MaxBytes: 2048,
	}

	mp, err := NewSimpleMempoolFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)

	_, ok := mp.(*SimpleMempool)
	assert.True(t, ok)
}

func TestNewPriorityMempoolFromConfig(t *testing.T) {
	cfg := &config.MempoolConfig{
		MaxTxs:   200,
		MaxBytes: 2048,
	}

	mp, err := NewPriorityMempoolFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)

	_, ok := mp.(*PriorityMempool)
	assert.True(t, ok)
}

func TestNewTTLMempoolFromConfig(t *testing.T) {
	cfg := &config.MempoolConfig{
		MaxTxs:          200,
		MaxBytes:        2048,
		TTL:             config.Duration(30 * time.Minute),
		CleanupInterval: config.Duration(5 * time.Minute),
	}

	mp, err := NewTTLMempoolFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, mp)

	ttlMp, ok := mp.(*TTLMempool)
	assert.True(t, ok)

	// Cleanup
	if ttlMp != nil {
		ttlMp.Stop()
	}
}

func TestMempoolTypeConstants(t *testing.T) {
	// Verify type constants have expected values
	assert.Equal(t, MempoolType("simple"), TypeSimple)
	assert.Equal(t, MempoolType("priority"), TypePriority)
	assert.Equal(t, MempoolType("ttl"), TypeTTL)
	assert.Equal(t, MempoolType("looseberry"), TypeLooseberry)
}
