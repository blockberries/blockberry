package statestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultStatePruneConfig(t *testing.T) {
	cfg := DefaultStatePruneConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, int64(100), cfg.KeepRecent)
	assert.Equal(t, int64(1000), cfg.KeepEvery)
	assert.Equal(t, time.Hour, cfg.Interval)
	assert.True(t, cfg.Enabled)
}

func TestArchiveStatePruneConfig(t *testing.T) {
	cfg := ArchiveStatePruneConfig()
	require.NotNil(t, cfg)
	assert.False(t, cfg.Enabled)
}

func TestStatePruneConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *StatePruneConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: false,
		},
		{
			name:    "valid default",
			cfg:     DefaultStatePruneConfig(),
			wantErr: false,
		},
		{
			name:    "valid archive",
			cfg:     ArchiveStatePruneConfig(),
			wantErr: false,
		},
		{
			name: "negative keep_recent",
			cfg: &StatePruneConfig{
				KeepRecent: -1,
				Enabled:    true,
			},
			wantErr: true,
		},
		{
			name: "negative keep_every",
			cfg: &StatePruneConfig{
				KeepEvery: -1,
				Enabled:   true,
			},
			wantErr: true,
		},
		{
			name: "negative interval",
			cfg: &StatePruneConfig{
				Interval: -1,
				Enabled:  true,
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

func TestStatePruneConfig_ShouldKeep(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *StatePruneConfig
		version        int64
		currentVersion int64
		want           bool
	}{
		{
			name:           "nil config keeps all",
			cfg:            nil,
			version:        1,
			currentVersion: 1000,
			want:           true,
		},
		{
			name: "disabled keeps all",
			cfg: &StatePruneConfig{
				Enabled: false,
			},
			version:        1,
			currentVersion: 1000,
			want:           true,
		},
		{
			name: "recent version kept",
			cfg: &StatePruneConfig{
				KeepRecent: 100,
				Enabled:    true,
			},
			version:        950,
			currentVersion: 1000,
			want:           true,
		},
		{
			name: "old version pruned",
			cfg: &StatePruneConfig{
				KeepRecent: 100,
				Enabled:    true,
			},
			version:        800,
			currentVersion: 1000,
			want:           false,
		},
		{
			name: "checkpoint version kept",
			cfg: &StatePruneConfig{
				KeepRecent: 100,
				KeepEvery:  1000,
				Enabled:    true,
			},
			version:        1000,
			currentVersion: 5000,
			want:           true,
		},
		{
			name: "non-checkpoint version pruned",
			cfg: &StatePruneConfig{
				KeepRecent: 100,
				KeepEvery:  1000,
				Enabled:    true,
			},
			version:        999,
			currentVersion: 5000,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ShouldKeep(tt.version, tt.currentVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatePruneConfig_CalculatePruneTarget(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *StatePruneConfig
		currentVersion int64
		want           int64
	}{
		{
			name:           "nil config returns 0",
			cfg:            nil,
			currentVersion: 1000,
			want:           0,
		},
		{
			name: "disabled returns 0",
			cfg: &StatePruneConfig{
				Enabled: false,
			},
			currentVersion: 1000,
			want:           0,
		},
		{
			name: "normal calculation",
			cfg: &StatePruneConfig{
				KeepRecent: 100,
				Enabled:    true,
			},
			currentVersion: 1000,
			want:           900,
		},
		{
			name: "target would be negative",
			cfg: &StatePruneConfig{
				KeepRecent: 100,
				Enabled:    true,
			},
			currentVersion: 50,
			want:           0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.CalculatePruneTarget(tt.currentVersion)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIAVLStore_PruneVersions(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Create 10 versions
	for i := 1; i <= 10; i++ {
		err := store.Set([]byte("key"), []byte{byte(i)})
		require.NoError(t, err)
		_, _, err = store.Commit()
		require.NoError(t, err)
	}

	oldest, newest := store.AvailableVersions()
	assert.Equal(t, int64(1), oldest)
	assert.Equal(t, int64(10), newest)

	// Prune versions up to 5
	result, err := store.PruneVersions(6)
	require.NoError(t, err)
	assert.Greater(t, result.PrunedCount, int64(0))
	assert.Equal(t, int64(10), result.NewestVersion)
	assert.Greater(t, result.Duration, time.Duration(0))

	// Verify old versions are gone
	oldest, newest = store.AvailableVersions()
	assert.GreaterOrEqual(t, oldest, int64(6))
	assert.Equal(t, int64(10), newest)
}

func TestIAVLStore_PruneVersions_InvalidVersion(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Prune with version 0
	_, err = store.PruneVersions(0)
	assert.ErrorIs(t, err, ErrInvalidPruneVersion)

	// Prune with negative version
	_, err = store.PruneVersions(-1)
	assert.ErrorIs(t, err, ErrInvalidPruneVersion)
}

func TestIAVLStore_PruneVersions_VersionTooHigh(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Create some versions
	for i := 1; i <= 5; i++ {
		_ = store.Set([]byte("key"), []byte{byte(i)})
		_, _, _ = store.Commit()
	}

	// Prune beyond current version
	_, err = store.PruneVersions(100)
	assert.Error(t, err)
}

func TestIAVLStore_AvailableVersions(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Initially no versions
	oldest, newest := store.AvailableVersions()
	assert.Equal(t, int64(0), oldest)
	assert.Equal(t, int64(0), newest)

	// Create versions
	for i := 1; i <= 5; i++ {
		_ = store.Set([]byte("key"), []byte{byte(i)})
		_, _, _ = store.Commit()
	}

	oldest, newest = store.AvailableVersions()
	assert.Equal(t, int64(1), oldest)
	assert.Equal(t, int64(5), newest)
}

func TestIAVLStore_AllVersions(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Create versions
	for i := 1; i <= 5; i++ {
		_ = store.Set([]byte("key"), []byte{byte(i)})
		_, _, _ = store.Commit()
	}

	versions := store.AllVersions()
	assert.Len(t, versions, 5)
	for i, v := range versions {
		assert.Equal(t, int64(i+1), v)
	}
}

func TestIAVLStore_StatePruneConfig(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Initially nil
	assert.Nil(t, store.StatePruneConfig())

	// Set config
	cfg := DefaultStatePruneConfig()
	store.SetStatePruneConfig(cfg)
	assert.Equal(t, cfg, store.StatePruneConfig())
}

func TestBackgroundStatePruner_StartStop(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Create some versions
	for i := 1; i <= 100; i++ {
		_ = store.Set([]byte("key"), []byte{byte(i)})
		_, _, _ = store.Commit()
	}

	cfg := &StatePruneConfig{
		KeepRecent: 10,
		Interval:   50 * time.Millisecond,
		Enabled:    true,
	}
	store.SetStatePruneConfig(cfg)

	pruner := NewBackgroundStatePruner(store, cfg)

	// Start pruner
	err = pruner.Start()
	require.NoError(t, err)
	assert.True(t, pruner.IsRunning())

	// Wait for at least one prune
	time.Sleep(150 * time.Millisecond)

	// Stop pruner
	err = pruner.Stop()
	require.NoError(t, err)
	assert.False(t, pruner.IsRunning())
}

func TestBackgroundStatePruner_Disabled(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	cfg := ArchiveStatePruneConfig()
	pruner := NewBackgroundStatePruner(store, cfg)

	err = pruner.Start()
	assert.ErrorIs(t, err, ErrStatePruningDisabled)
}

func TestBackgroundStatePruner_PruneNow(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Create some versions
	for i := 1; i <= 100; i++ {
		_ = store.Set([]byte("key"), []byte{byte(i)})
		_, _, _ = store.Commit()
	}

	cfg := &StatePruneConfig{
		KeepRecent: 10,
		Interval:   time.Hour,
		Enabled:    true,
	}
	store.SetStatePruneConfig(cfg)

	pruner := NewBackgroundStatePruner(store, cfg)

	// PruneNow should work without starting
	result, err := pruner.PruneNow()
	require.NoError(t, err)
	assert.Greater(t, result.PrunedCount, int64(0))
}
