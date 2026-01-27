package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNullConsensus(t *testing.T) {
	nc := NewNullConsensus()
	require.NotNil(t, nc)

	assert.Equal(t, "null-consensus", nc.Name())
	assert.False(t, nc.IsRunning())
	assert.Equal(t, int64(0), nc.GetHeight())
	assert.Equal(t, int32(0), nc.GetRound())
	assert.False(t, nc.IsValidator())
	assert.Nil(t, nc.ValidatorSet())
}

func TestNewNullConsensusFromConfig(t *testing.T) {
	cfg := &ConsensusConfig{
		Type:           string(TypeNone),
		TimeoutPropose: 3000,
	}

	engine, err := NewNullConsensusFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, engine)

	nc, ok := engine.(*NullConsensus)
	require.True(t, ok)
	assert.Equal(t, cfg, nc.config)
}

func TestNullConsensus_StartStop(t *testing.T) {
	nc := NewNullConsensus()

	// Start
	err := nc.Start()
	require.NoError(t, err)
	assert.True(t, nc.IsRunning())

	// Double start should fail
	err = nc.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Stop
	err = nc.Stop()
	require.NoError(t, err)
	assert.False(t, nc.IsRunning())

	// Stop again (idempotent)
	err = nc.Stop()
	require.NoError(t, err)
	assert.False(t, nc.IsRunning())

	// Restart
	err = nc.Start()
	require.NoError(t, err)
	assert.True(t, nc.IsRunning())
}

func TestNullConsensus_Initialize(t *testing.T) {
	nc := NewNullConsensus()

	cfg := &ConsensusConfig{
		Type:           string(TypeNone),
		TimeoutPropose: 5000,
	}

	deps := ConsensusDependencies{
		Config: cfg,
		Callbacks: &ConsensusCallbacks{
			OnBlockCommitted: func(height int64, hash []byte) error {
				return nil
			},
		},
	}

	err := nc.Initialize(deps)
	require.NoError(t, err)
	assert.Equal(t, cfg, nc.config)
}

func TestNullConsensus_Initialize_NilConfig(t *testing.T) {
	nc := NewNullConsensus()

	// Pre-set config
	presetCfg := &ConsensusConfig{Type: "preset"}
	nc.config = presetCfg

	// Initialize with nil config should keep preset
	deps := ConsensusDependencies{
		Config: nil,
	}

	err := nc.Initialize(deps)
	require.NoError(t, err)
	assert.Equal(t, presetCfg, nc.config)
}

func TestNullConsensus_ProcessBlock(t *testing.T) {
	nc := NewNullConsensus()

	block := &Block{
		Height: 100,
		Round:  2,
		Hash:   []byte{1, 2, 3, 4},
	}

	err := nc.ProcessBlock(block)
	require.NoError(t, err)

	assert.Equal(t, int64(100), nc.GetHeight())
	assert.Equal(t, int32(2), nc.GetRound())
}

func TestNullConsensus_ProcessBlock_NilBlock(t *testing.T) {
	nc := NewNullConsensus()

	err := nc.ProcessBlock(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil block")
}

func TestNullConsensus_ProcessBlock_WithCallback(t *testing.T) {
	nc := NewNullConsensus()

	var callbackCalled bool
	var callbackHeight int64
	var callbackHash []byte

	deps := ConsensusDependencies{
		Callbacks: &ConsensusCallbacks{
			OnBlockCommitted: func(height int64, hash []byte) error {
				callbackCalled = true
				callbackHeight = height
				callbackHash = hash
				return nil
			},
		},
	}

	err := nc.Initialize(deps)
	require.NoError(t, err)

	block := &Block{
		Height: 50,
		Round:  1,
		Hash:   []byte{5, 6, 7, 8},
	}

	err = nc.ProcessBlock(block)
	require.NoError(t, err)

	assert.True(t, callbackCalled)
	assert.Equal(t, int64(50), callbackHeight)
	assert.Equal(t, []byte{5, 6, 7, 8}, callbackHash)
}

func TestNullConsensus_ProcessBlock_NilCallbacks(t *testing.T) {
	nc := NewNullConsensus()

	// No initialization, deps.Callbacks is nil
	block := &Block{
		Height: 10,
		Round:  0,
		Hash:   []byte{1},
	}

	// Should not panic
	err := nc.ProcessBlock(block)
	require.NoError(t, err)
	assert.Equal(t, int64(10), nc.GetHeight())
}

func TestNullConsensus_ProcessBlock_CallbackError(t *testing.T) {
	nc := NewNullConsensus()

	deps := ConsensusDependencies{
		Callbacks: &ConsensusCallbacks{
			OnBlockCommitted: func(height int64, hash []byte) error {
				return assert.AnError
			},
		},
	}

	err := nc.Initialize(deps)
	require.NoError(t, err)

	block := &Block{
		Height: 1,
		Hash:   []byte{1},
	}

	err = nc.ProcessBlock(block)
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestNullConsensus_IsValidator(t *testing.T) {
	nc := NewNullConsensus()

	// NullConsensus never considers itself a validator
	assert.False(t, nc.IsValidator())
}

func TestNullConsensus_ValidatorSet(t *testing.T) {
	nc := NewNullConsensus()

	// NullConsensus returns nil validator set
	assert.Nil(t, nc.ValidatorSet())
}

func TestNullConsensus_InterfaceCompliance(t *testing.T) {
	var _ ConsensusEngine = (*NullConsensus)(nil)
}

func TestNullConsensus_ConcurrentProcessBlock(t *testing.T) {
	nc := NewNullConsensus()

	done := make(chan bool)
	numGoroutines := 10
	numBlocks := 100

	for i := range numGoroutines {
		go func(n int) {
			for j := range numBlocks {
				block := &Block{
					Height: int64(n*numBlocks + j),
					Round:  int32(n),
					Hash:   []byte{byte(n), byte(j)},
				}
				_ = nc.ProcessBlock(block)
			}
			done <- true
		}(i)
	}

	for range numGoroutines {
		<-done
	}

	// Height should be one of the block heights (last one is not deterministic)
	height := nc.GetHeight()
	assert.GreaterOrEqual(t, height, int64(0))
	assert.Less(t, height, int64(numGoroutines*numBlocks))
}
