package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)

	// Verify built-in "none" type is registered
	assert.True(t, f.Has(TypeNone))
}

func TestFactory_Register(t *testing.T) {
	f := NewFactory()

	// Register custom type
	customType := ConsensusType("custom")
	constructor := func(cfg *ConsensusConfig) (ConsensusEngine, error) {
		return NewNullConsensus(), nil
	}

	f.Register(customType, constructor)
	assert.True(t, f.Has(customType))
}

func TestFactory_Register_Override(t *testing.T) {
	f := NewFactory()

	// Override built-in type
	overrideCalled := false
	f.Register(TypeNone, func(cfg *ConsensusConfig) (ConsensusEngine, error) {
		overrideCalled = true
		return NewNullConsensus(), nil
	})

	cfg := &ConsensusConfig{Type: string(TypeNone)}
	_, err := f.Create(cfg)
	require.NoError(t, err)
	assert.True(t, overrideCalled)
}

func TestFactory_Unregister(t *testing.T) {
	f := NewFactory()

	// Verify built-in exists
	assert.True(t, f.Has(TypeNone))

	// Unregister
	f.Unregister(TypeNone)
	assert.False(t, f.Has(TypeNone))

	// Creating with unregistered type should fail
	cfg := &ConsensusConfig{Type: string(TypeNone)}
	_, err := f.Create(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown consensus type")
}

func TestFactory_Create_Success(t *testing.T) {
	f := NewFactory()

	cfg := &ConsensusConfig{
		Type:           string(TypeNone),
		TimeoutPropose: 3000,
	}

	engine, err := f.Create(cfg)
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.Equal(t, "null-consensus", engine.Name())
}

func TestFactory_Create_NilConfig(t *testing.T) {
	f := NewFactory()

	engine, err := f.Create(nil)
	assert.Nil(t, engine)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consensus config is required")
}

func TestFactory_Create_UnknownType(t *testing.T) {
	f := NewFactory()

	cfg := &ConsensusConfig{Type: "unknown"}
	engine, err := f.Create(cfg)
	assert.Nil(t, engine)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown consensus type")
}

func TestFactory_Has(t *testing.T) {
	f := NewFactory()

	assert.True(t, f.Has(TypeNone))
	assert.False(t, f.Has(ConsensusType("nonexistent")))
}

func TestFactory_Types(t *testing.T) {
	f := NewFactory()

	types := f.Types()
	require.Len(t, types, 1)
	assert.Contains(t, types, TypeNone)

	// Register another type
	f.Register(ConsensusType("custom"), func(cfg *ConsensusConfig) (ConsensusEngine, error) {
		return nil, nil
	})

	types = f.Types()
	require.Len(t, types, 2)
	assert.Contains(t, types, TypeNone)
	assert.Contains(t, types, ConsensusType("custom"))
}

func TestDefaultFactory(t *testing.T) {
	require.NotNil(t, DefaultFactory)
	assert.True(t, DefaultFactory.Has(TypeNone))
}

func TestCreateFromConfig(t *testing.T) {
	cfg := &ConsensusConfig{Type: string(TypeNone)}
	engine, err := CreateFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.Equal(t, "null-consensus", engine.Name())
}

func TestRegisterConsensus(t *testing.T) {
	// Create a new factory to avoid polluting DefaultFactory
	originalFactory := DefaultFactory
	DefaultFactory = NewFactory()
	defer func() { DefaultFactory = originalFactory }()

	customType := ConsensusType("test-custom")

	// Verify not registered
	assert.False(t, DefaultFactory.Has(customType))

	// Register using convenience function
	RegisterConsensus(customType, func(cfg *ConsensusConfig) (ConsensusEngine, error) {
		return NewNullConsensus(), nil
	})

	// Now should be registered
	assert.True(t, DefaultFactory.Has(customType))

	// Should be able to create
	cfg := &ConsensusConfig{Type: string(customType)}
	engine, err := CreateFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, engine)
}

func TestFactory_ConcurrentAccess(t *testing.T) {
	f := NewFactory()

	done := make(chan bool)

	// Concurrent reads
	for i := range 10 {
		go func(n int) {
			for j := 0; j < 100; j++ {
				f.Has(TypeNone)
				f.Types()
			}
			done <- true
		}(i)
	}

	// Concurrent writes
	for i := range 10 {
		go func(n int) {
			customType := ConsensusType("concurrent-" + string(rune('0'+n)))
			for j := 0; j < 100; j++ {
				f.Register(customType, func(cfg *ConsensusConfig) (ConsensusEngine, error) {
					return nil, nil
				})
				f.Unregister(customType)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}
