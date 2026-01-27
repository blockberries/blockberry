package blockstore

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneStrategy_Constants(t *testing.T) {
	assert.Equal(t, PruneStrategy("nothing"), PruneNothing)
	assert.Equal(t, PruneStrategy("everything"), PruneEverything)
	assert.Equal(t, PruneStrategy("default"), PruneDefault)
	assert.Equal(t, PruneStrategy("custom"), PruneCustom)
}

func TestDefaultPruneConfig(t *testing.T) {
	cfg := DefaultPruneConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, PruneDefault, cfg.Strategy)
	assert.Equal(t, int64(1000), cfg.KeepRecent)
	assert.Equal(t, int64(10000), cfg.KeepEvery)
	assert.Equal(t, time.Hour, cfg.Interval)
}

func TestArchivePruneConfig(t *testing.T) {
	cfg := ArchivePruneConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, PruneNothing, cfg.Strategy)
}

func TestAggressivePruneConfig(t *testing.T) {
	cfg := AggressivePruneConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, PruneEverything, cfg.Strategy)
	assert.Equal(t, int64(100), cfg.KeepRecent)
	assert.Equal(t, 10*time.Minute, cfg.Interval)
}

func TestPruneConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *PruneConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name:    "valid default",
			cfg:     DefaultPruneConfig(),
			wantErr: false,
		},
		{
			name:    "valid archive",
			cfg:     ArchivePruneConfig(),
			wantErr: false,
		},
		{
			name: "invalid strategy",
			cfg: &PruneConfig{
				Strategy: "invalid",
			},
			wantErr: true,
		},
		{
			name: "negative keep_recent",
			cfg: &PruneConfig{
				Strategy:   PruneDefault,
				KeepRecent: -1,
			},
			wantErr: true,
		},
		{
			name: "negative keep_every",
			cfg: &PruneConfig{
				Strategy:  PruneDefault,
				KeepEvery: -1,
			},
			wantErr: true,
		},
		{
			name: "negative interval",
			cfg: &PruneConfig{
				Strategy: PruneDefault,
				Interval: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPruneConfig_ShouldKeep(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *PruneConfig
		height        int64
		currentHeight int64
		want          bool
	}{
		{
			name:          "nil config keeps all",
			cfg:           nil,
			height:        1,
			currentHeight: 1000,
			want:          true,
		},
		{
			name: "archive mode keeps all",
			cfg: &PruneConfig{
				Strategy: PruneNothing,
			},
			height:        1,
			currentHeight: 1000,
			want:          true,
		},
		{
			name: "recent block kept",
			cfg: &PruneConfig{
				Strategy:   PruneEverything,
				KeepRecent: 100,
			},
			height:        950,
			currentHeight: 1000,
			want:          true,
		},
		{
			name: "old block pruned",
			cfg: &PruneConfig{
				Strategy:   PruneEverything,
				KeepRecent: 100,
			},
			height:        800,
			currentHeight: 1000,
			want:          false,
		},
		{
			name: "checkpoint block kept",
			cfg: &PruneConfig{
				Strategy:   PruneDefault,
				KeepRecent: 100,
				KeepEvery:  1000,
			},
			height:        1000,
			currentHeight: 5000,
			want:          true,
		},
		{
			name: "non-checkpoint block pruned",
			cfg: &PruneConfig{
				Strategy:   PruneDefault,
				KeepRecent: 100,
				KeepEvery:  1000,
			},
			height:        999,
			currentHeight: 5000,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ShouldKeep(tt.height, tt.currentHeight)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPruneConfig_CalculatePruneTarget(t *testing.T) {
	tests := []struct {
		name          string
		cfg           *PruneConfig
		currentHeight int64
		want          int64
	}{
		{
			name:          "nil config returns 0",
			cfg:           nil,
			currentHeight: 1000,
			want:          0,
		},
		{
			name: "archive mode returns 0",
			cfg: &PruneConfig{
				Strategy: PruneNothing,
			},
			currentHeight: 1000,
			want:          0,
		},
		{
			name: "normal calculation",
			cfg: &PruneConfig{
				Strategy:   PruneDefault,
				KeepRecent: 100,
			},
			currentHeight: 1000,
			want:          900,
		},
		{
			name: "target would be negative",
			cfg: &PruneConfig{
				Strategy:   PruneDefault,
				KeepRecent: 100,
			},
			currentHeight: 50,
			want:          0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.CalculatePruneTarget(tt.currentHeight)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMemoryBlockStore_Prune(t *testing.T) {
	store := NewMemoryBlockStore()

	// Add 100 blocks
	for i := int64(1); i <= 100; i++ {
		hash := []byte{byte(i)}
		data := make([]byte, 100)
		err := store.SaveBlock(i, hash, data)
		require.NoError(t, err)
	}

	assert.Equal(t, int64(100), store.Height())
	assert.Equal(t, int64(1), store.Base())

	// Prune first 50 blocks
	result, err := store.Prune(51)
	require.NoError(t, err)
	assert.Equal(t, int64(50), result.PrunedCount)
	assert.Equal(t, int64(51), result.NewBase)
	assert.Greater(t, result.BytesFreed, int64(0))
	assert.Greater(t, result.Duration, time.Duration(0))

	assert.Equal(t, int64(51), store.Base())

	// Verify blocks were pruned
	for i := int64(1); i <= 50; i++ {
		assert.False(t, store.HasBlock(i))
	}
	for i := int64(51); i <= 100; i++ {
		assert.True(t, store.HasBlock(i))
	}
}

func TestMemoryBlockStore_Prune_WithConfig(t *testing.T) {
	store := NewMemoryBlockStore()
	store.SetPruneConfig(&PruneConfig{
		Strategy:   PruneDefault,
		KeepRecent: 10,
		KeepEvery:  10, // Keep blocks at heights 10, 20, 30...
	})

	// Add 100 blocks
	for i := int64(1); i <= 100; i++ {
		hash := []byte{byte(i)}
		data := make([]byte, 100)
		err := store.SaveBlock(i, hash, data)
		require.NoError(t, err)
	}

	// Prune - should keep recent 10 and checkpoints
	result, err := store.Prune(91)
	require.NoError(t, err)

	// Recent blocks (91-100) should be kept by KeepRecent
	// Checkpoints (10, 20, 30, 40, 50, 60, 70, 80, 90) should be kept
	// So pruned should be: 1-9, 11-19, 21-29, 31-39, 41-49, 51-59, 61-69, 71-79, 81-89
	// That's 9*9 = 81 blocks
	assert.Equal(t, int64(81), result.PrunedCount)

	// Verify checkpoints were kept
	for _, h := range []int64{10, 20, 30, 40, 50, 60, 70, 80, 90} {
		assert.True(t, store.HasBlock(h), "checkpoint at %d should be kept", h)
	}

	// Verify non-checkpoints were pruned
	for _, h := range []int64{1, 15, 25, 55, 85} {
		assert.False(t, store.HasBlock(h), "block at %d should be pruned", h)
	}
}

func TestMemoryBlockStore_Prune_InvalidHeight(t *testing.T) {
	store := NewMemoryBlockStore()

	// Prune with height 0
	_, err := store.Prune(0)
	assert.ErrorIs(t, err, ErrInvalidPruneHeight)

	// Prune with negative height
	_, err = store.Prune(-1)
	assert.ErrorIs(t, err, ErrInvalidPruneHeight)
}

func TestMemoryBlockStore_Prune_HeightTooHigh(t *testing.T) {
	store := NewMemoryBlockStore()

	// Add some blocks
	for i := int64(1); i <= 10; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, nil)
	}

	// Prune beyond current height
	_, err := store.Prune(100)
	assert.ErrorIs(t, err, ErrPruneHeightTooHigh)
}

func TestMemoryBlockStore_Prune_NothingToPrune(t *testing.T) {
	store := NewMemoryBlockStore()

	// Add some blocks
	for i := int64(1); i <= 10; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, nil)
	}

	// Prune at base
	result, err := store.Prune(1)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.PrunedCount)
}

func TestBackgroundPruner_StartStop(t *testing.T) {
	store := NewMemoryBlockStore()

	// Add some blocks
	for i := int64(1); i <= 100; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, nil)
	}

	cfg := &PruneConfig{
		Strategy:   PruneEverything,
		KeepRecent: 10,
		Interval:   50 * time.Millisecond,
	}
	store.SetPruneConfig(cfg)

	pruner := NewBackgroundPruner(store, cfg)

	var pruneCount atomic.Int32
	pruner.SetOnPrune(func(result *PruneResult) {
		pruneCount.Add(1)
	})

	// Start pruner
	err := pruner.Start()
	require.NoError(t, err)
	assert.True(t, pruner.IsRunning())

	// Wait for at least one prune
	time.Sleep(150 * time.Millisecond)

	// Stop pruner
	err = pruner.Stop()
	require.NoError(t, err)
	assert.False(t, pruner.IsRunning())

	// Should have pruned at least once
	assert.GreaterOrEqual(t, pruneCount.Load(), int32(1))
}

func TestBackgroundPruner_DoubleStart(t *testing.T) {
	store := NewMemoryBlockStore()
	cfg := &PruneConfig{
		Strategy:   PruneDefault,
		KeepRecent: 10,
		Interval:   time.Hour,
	}

	pruner := NewBackgroundPruner(store, cfg)

	err := pruner.Start()
	require.NoError(t, err)
	defer pruner.Stop()

	// Double start should fail
	err = pruner.Start()
	assert.ErrorIs(t, err, ErrPrunerAlreadyActive)
}

func TestBackgroundPruner_StopNotStarted(t *testing.T) {
	store := NewMemoryBlockStore()
	cfg := DefaultPruneConfig()

	pruner := NewBackgroundPruner(store, cfg)

	err := pruner.Stop()
	assert.ErrorIs(t, err, ErrPrunerNotActive)
}

func TestBackgroundPruner_DisabledPruning(t *testing.T) {
	store := NewMemoryBlockStore()
	cfg := ArchivePruneConfig()

	pruner := NewBackgroundPruner(store, cfg)

	err := pruner.Start()
	assert.ErrorIs(t, err, ErrPruningDisabled)
}

func TestBackgroundPruner_PruneNow(t *testing.T) {
	store := NewMemoryBlockStore()

	// Add some blocks
	for i := int64(1); i <= 100; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, nil)
	}

	cfg := &PruneConfig{
		Strategy:   PruneEverything,
		KeepRecent: 10,
		Interval:   time.Hour, // Long interval
	}
	store.SetPruneConfig(cfg)

	pruner := NewBackgroundPruner(store, cfg)

	// PruneNow should work without starting the pruner
	// KeepRecent=10 means keep blocks 91-100, prune target is 90
	// Prune(90) prunes blocks before height 90, so blocks 1-89 (89 blocks)
	result, err := pruner.PruneNow()
	require.NoError(t, err)
	assert.Equal(t, int64(89), result.PrunedCount)
}

func TestBackgroundPruner_OnError(t *testing.T) {
	store := NewMemoryBlockStore()
	cfg := &PruneConfig{
		Strategy:   PruneEverything,
		KeepRecent: 10,
		Interval:   50 * time.Millisecond,
	}

	var errorCount atomic.Int32
	pruner := NewBackgroundPruner(store, cfg)
	pruner.SetOnError(func(err error) {
		errorCount.Add(1)
	})

	// Don't set prune config on store - this won't cause errors,
	// but we can test the callback mechanism

	err := pruner.Start()
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = pruner.Stop()
	require.NoError(t, err)

	// No errors expected in normal operation
	assert.Equal(t, int32(0), errorCount.Load())
}
