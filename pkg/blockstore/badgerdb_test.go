package blockstore

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
)

func TestDefaultBadgerDBOptions(t *testing.T) {
	opts := DefaultBadgerDBOptions()
	require.NotNil(t, opts)
	assert.True(t, opts.SyncWrites)
	assert.True(t, opts.Compression)
	assert.Equal(t, int64(1<<30), opts.ValueLogFileSize)
	assert.Equal(t, int64(64<<20), opts.MemTableSize)
	assert.Equal(t, 5, opts.NumMemtables)
	assert.Equal(t, 5, opts.NumLevelZeroTables)
	assert.Equal(t, 15, opts.NumLevelZeroTablesStall)
}

func TestNewBadgerDBBlockStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	assert.Equal(t, int64(0), store.Height())
	assert.Equal(t, int64(0), store.Base())
}

func TestNewBadgerDBBlockStoreWithOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	opts := DefaultBadgerDBOptions()
	opts.SyncWrites = false
	opts.Compression = false

	store, err := NewBadgerDBBlockStoreWithOptions(tmpDir, opts)
	require.NoError(t, err)
	defer store.Close()

	assert.Equal(t, int64(0), store.Height())
}

func TestBadgerDBBlockStore_SaveAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Save a block
	hash := []byte("test_hash")
	data := []byte("test_block_data")
	err = store.SaveBlock(1, hash, data)
	require.NoError(t, err)

	// Load by height
	loadedHash, loadedData, err := store.LoadBlock(1)
	require.NoError(t, err)
	assert.Equal(t, hash, loadedHash)
	assert.Equal(t, data, loadedData)

	// Load by hash
	loadedHeight, loadedData, err := store.LoadBlockByHash(hash)
	require.NoError(t, err)
	assert.Equal(t, int64(1), loadedHeight)
	assert.Equal(t, data, loadedData)
}

func TestBadgerDBBlockStore_SaveDuplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	hash := []byte("test_hash")
	data := []byte("test_data")

	err = store.SaveBlock(1, hash, data)
	require.NoError(t, err)

	// Try to save duplicate
	err = store.SaveBlock(1, hash, data)
	assert.ErrorIs(t, err, types.ErrBlockAlreadyExists)
}

func TestBadgerDBBlockStore_LoadNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	_, _, err = store.LoadBlock(999)
	assert.ErrorIs(t, err, types.ErrBlockNotFound)

	_, _, err = store.LoadBlockByHash([]byte("nonexistent"))
	assert.ErrorIs(t, err, types.ErrBlockNotFound)
}

func TestBadgerDBBlockStore_HasBlock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Initially doesn't have block
	assert.False(t, store.HasBlock(1))

	// Save a block
	_ = store.SaveBlock(1, []byte("hash"), []byte("data"))

	// Now has block
	assert.True(t, store.HasBlock(1))
	assert.False(t, store.HasBlock(2))
}

func TestBadgerDBBlockStore_HeightAndBase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Initially 0
	assert.Equal(t, int64(0), store.Height())
	assert.Equal(t, int64(0), store.Base())

	// Save blocks non-sequentially
	_ = store.SaveBlock(5, []byte("hash5"), []byte("data"))
	assert.Equal(t, int64(5), store.Height())
	assert.Equal(t, int64(5), store.Base())

	_ = store.SaveBlock(10, []byte("hash10"), []byte("data"))
	assert.Equal(t, int64(10), store.Height())
	assert.Equal(t, int64(5), store.Base())

	_ = store.SaveBlock(1, []byte("hash1"), []byte("data"))
	assert.Equal(t, int64(10), store.Height())
	assert.Equal(t, int64(1), store.Base())
}

func TestBadgerDBBlockStore_BlockCount(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	assert.Equal(t, 0, store.BlockCount())

	for i := int64(1); i <= 10; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, []byte("data"))
	}

	assert.Equal(t, 10, store.BlockCount())
}

func TestBadgerDBBlockStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create store and save blocks
	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)

	for i := int64(1); i <= 5; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, []byte{byte(i * 2)})
	}

	store.Close()

	// Reopen store
	store, err = NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	assert.Equal(t, int64(5), store.Height())
	assert.Equal(t, int64(1), store.Base())
	assert.Equal(t, 5, store.BlockCount())

	// Verify data
	for i := int64(1); i <= 5; i++ {
		hash, data, err := store.LoadBlock(i)
		require.NoError(t, err)
		assert.Equal(t, []byte{byte(i)}, hash)
		assert.Equal(t, []byte{byte(i * 2)}, data)
	}
}

func TestBadgerDBBlockStore_Prune(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Add 100 blocks
	for i := int64(1); i <= 100; i++ {
		data := make([]byte, 100)
		_ = store.SaveBlock(i, []byte{byte(i)}, data)
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
		assert.False(t, store.HasBlock(i), "block %d should be pruned", i)
	}
	for i := int64(51); i <= 100; i++ {
		assert.True(t, store.HasBlock(i), "block %d should exist", i)
	}
}

func TestBadgerDBBlockStore_Prune_WithConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	store.SetPruneConfig(&PruneConfig{
		Strategy:   PruneDefault,
		KeepRecent: 10,
		KeepEvery:  10,
	})

	// Add 100 blocks
	for i := int64(1); i <= 100; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, []byte("data"))
	}

	// Prune - should keep recent 10 and checkpoints
	result, err := store.Prune(91)
	require.NoError(t, err)

	// Verify checkpoints were kept
	for _, h := range []int64{10, 20, 30, 40, 50, 60, 70, 80, 90} {
		assert.True(t, store.HasBlock(h), "checkpoint at %d should be kept", h)
	}

	// Verify non-checkpoints were pruned
	for _, h := range []int64{1, 15, 25, 55, 85} {
		assert.False(t, store.HasBlock(h), "block at %d should be pruned", h)
	}

	assert.Equal(t, int64(81), result.PrunedCount)
}

func TestBadgerDBBlockStore_Prune_InvalidHeight(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	_, err = store.Prune(0)
	assert.ErrorIs(t, err, ErrInvalidPruneHeight)

	_, err = store.Prune(-1)
	assert.ErrorIs(t, err, ErrInvalidPruneHeight)
}

func TestBadgerDBBlockStore_Prune_HeightTooHigh(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	for i := int64(1); i <= 10; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, nil)
	}

	_, err = store.Prune(100)
	assert.ErrorIs(t, err, ErrPruneHeightTooHigh)
}

func TestBadgerDBBlockStore_PruneConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Initially nil
	assert.Nil(t, store.PruneConfig())

	// Set config
	cfg := DefaultPruneConfig()
	store.SetPruneConfig(cfg)
	assert.Equal(t, cfg, store.PruneConfig())
}

func TestBadgerDBBlockStore_Compact(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Add and prune blocks to create garbage
	for i := int64(1); i <= 100; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, make([]byte, 1000))
	}

	_, _ = store.Prune(50)

	// Compact should not error
	err = store.Compact()
	require.NoError(t, err)
}

func TestBadgerDBBlockStore_Sync(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	_ = store.SaveBlock(1, []byte("hash"), []byte("data"))

	err = store.Sync()
	require.NoError(t, err)
}

func TestBadgerDBBlockStore_Size(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Add some data
	for i := int64(1); i <= 10; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, make([]byte, 1000))
	}

	lsmSize, vlogSize := store.Size()
	// Just verify we get non-negative values
	assert.GreaterOrEqual(t, lsmSize, int64(0))
	assert.GreaterOrEqual(t, vlogSize, int64(0))
}

func TestBadgerDBBlockStore_Iterator(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewBadgerDBBlockStore(tmpDir)
	require.NoError(t, err)
	defer store.Close()

	// Add blocks
	for i := int64(1); i <= 5; i++ {
		_ = store.SaveBlock(i, []byte{byte(i)}, []byte("data"))
	}

	// Iterate
	it := store.NewBlockIterator()
	defer it.Close()

	var heights []int64
	for it.Valid() {
		heights = append(heights, it.Height())
		hash := it.Hash()
		assert.NotNil(t, hash)
		it.Next()
	}

	assert.Equal(t, []int64{1, 2, 3, 4, 5}, heights)
}

func TestBadgerDBBlockStore_Interface(t *testing.T) {
	// Verify BadgerDBBlockStore implements PrunableBlockStore
	var _ PrunableBlockStore = (*BadgerDBBlockStore)(nil)
}

func TestCompareKeys(t *testing.T) {
	tests := []struct {
		a, b []byte
		want int
	}{
		{[]byte{1, 2, 3}, []byte{1, 2, 3}, 0},
		{[]byte{1, 2, 3}, []byte{1, 2, 4}, -1},
		{[]byte{1, 2, 4}, []byte{1, 2, 3}, 1},
		{[]byte{1, 2}, []byte{1, 2, 3}, -1},
		{[]byte{1, 2, 3}, []byte{1, 2}, 1},
		{nil, nil, 0},
		{[]byte{}, []byte{}, 0},
		{[]byte{1}, []byte{}, 1},
		{[]byte{}, []byte{1}, -1},
	}

	for _, tt := range tests {
		got := compareKeys(tt.a, tt.b)
		assert.Equal(t, tt.want, got, "compareKeys(%v, %v)", tt.a, tt.b)
	}
}
