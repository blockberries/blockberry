package blockstore

import (
	"crypto/rand"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
)

func TestNewLevelDBBlockStore(t *testing.T) {
	t.Run("creates new store", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "blocks")

		store, err := NewLevelDBBlockStore(path)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()

		require.Equal(t, int64(0), store.Height())
		require.Equal(t, int64(0), store.Base())
	})

	t.Run("reopens existing store", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "blocks")

		// Create and populate store
		store1, err := NewLevelDBBlockStore(path)
		require.NoError(t, err)

		hash := makeTestHash(1)
		data := []byte("block data")
		err = store1.SaveBlock(1, hash, data)
		require.NoError(t, err)
		require.NoError(t, store1.Close())

		// Reopen store
		store2, err := NewLevelDBBlockStore(path)
		require.NoError(t, err)
		defer store2.Close()

		require.Equal(t, int64(1), store2.Height())
		require.Equal(t, int64(1), store2.Base())

		// Verify data
		loadedHash, loadedData, err := store2.LoadBlock(1)
		require.NoError(t, err)
		require.Equal(t, hash, loadedHash)
		require.Equal(t, data, loadedData)
	})
}

func TestSaveBlock(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("saves first block", func(t *testing.T) {
		hash := makeTestHash(1)
		data := []byte("block 1 data")

		err := store.SaveBlock(1, hash, data)
		require.NoError(t, err)

		require.Equal(t, int64(1), store.Height())
		require.Equal(t, int64(1), store.Base())
		require.True(t, store.HasBlock(1))
	})

	t.Run("saves sequential blocks", func(t *testing.T) {
		for h := int64(2); h <= 5; h++ {
			hash := makeTestHash(int(h))
			data := []byte("block data")

			err := store.SaveBlock(h, hash, data)
			require.NoError(t, err)

			require.Equal(t, h, store.Height())
			require.Equal(t, int64(1), store.Base()) // Base stays at 1
		}
	})

	t.Run("rejects duplicate block", func(t *testing.T) {
		hash := makeTestHash(100)
		data := []byte("block data")

		err := store.SaveBlock(100, hash, data)
		require.NoError(t, err)

		// Try to save again at same height
		err = store.SaveBlock(100, hash, data)
		require.ErrorIs(t, err, types.ErrBlockAlreadyExists)
	})

	t.Run("handles out of order blocks", func(t *testing.T) {
		store2 := newTestStore(t)
		defer store2.Close()

		// Save block at height 10 first
		err := store2.SaveBlock(10, makeTestHash(10), []byte("block 10"))
		require.NoError(t, err)
		require.Equal(t, int64(10), store2.Height())
		require.Equal(t, int64(10), store2.Base())

		// Save block at height 5
		err = store2.SaveBlock(5, makeTestHash(5), []byte("block 5"))
		require.NoError(t, err)
		require.Equal(t, int64(10), store2.Height()) // Height stays at 10
		require.Equal(t, int64(5), store2.Base())    // Base updates to 5
	})
}

func TestLoadBlock(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Save some blocks
	blocks := map[int64]struct {
		hash []byte
		data []byte
	}{
		1: {makeTestHash(1), []byte("block 1 data")},
		2: {makeTestHash(2), []byte("block 2 data with more content")},
		3: {makeTestHash(3), []byte("block 3")},
	}

	for h, b := range blocks {
		err := store.SaveBlock(h, b.hash, b.data)
		require.NoError(t, err)
	}

	t.Run("loads existing blocks", func(t *testing.T) {
		for h, expected := range blocks {
			hash, data, err := store.LoadBlock(h)
			require.NoError(t, err)
			require.Equal(t, expected.hash, hash)
			require.Equal(t, expected.data, data)
		}
	})

	t.Run("returns error for non-existent block", func(t *testing.T) {
		_, _, err := store.LoadBlock(999)
		require.ErrorIs(t, err, types.ErrBlockNotFound)
	})

	t.Run("returns error for negative height", func(t *testing.T) {
		_, _, err := store.LoadBlock(-1)
		require.ErrorIs(t, err, types.ErrBlockNotFound)
	})
}

func TestLoadBlockByHash(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Save a block
	height := int64(42)
	hash := makeTestHash(42)
	data := []byte("block 42 data")

	err := store.SaveBlock(height, hash, data)
	require.NoError(t, err)

	t.Run("loads by hash", func(t *testing.T) {
		loadedHeight, loadedData, err := store.LoadBlockByHash(hash)
		require.NoError(t, err)
		require.Equal(t, height, loadedHeight)
		require.Equal(t, data, loadedData)
	})

	t.Run("returns error for non-existent hash", func(t *testing.T) {
		unknownHash := makeTestHash(999)
		_, _, err := store.LoadBlockByHash(unknownHash)
		require.ErrorIs(t, err, types.ErrBlockNotFound)
	})
}

func TestHasBlock(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	err := store.SaveBlock(5, makeTestHash(5), []byte("block 5"))
	require.NoError(t, err)

	require.True(t, store.HasBlock(5))
	require.False(t, store.HasBlock(1))
	require.False(t, store.HasBlock(10))
	require.False(t, store.HasBlock(-1))
}

func TestHeightAndBase(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Empty store
	require.Equal(t, int64(0), store.Height())
	require.Equal(t, int64(0), store.Base())

	// Add first block
	err := store.SaveBlock(100, makeTestHash(100), []byte("block"))
	require.NoError(t, err)
	require.Equal(t, int64(100), store.Height())
	require.Equal(t, int64(100), store.Base())

	// Add higher block
	err = store.SaveBlock(200, makeTestHash(200), []byte("block"))
	require.NoError(t, err)
	require.Equal(t, int64(200), store.Height())
	require.Equal(t, int64(100), store.Base())

	// Add lower block
	err = store.SaveBlock(50, makeTestHash(50), []byte("block"))
	require.NoError(t, err)
	require.Equal(t, int64(200), store.Height())
	require.Equal(t, int64(50), store.Base())
}

func TestBlockCount(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	require.Equal(t, 0, store.BlockCount())

	for i := 1; i <= 10; i++ {
		err := store.SaveBlock(int64(i), makeTestHash(i), []byte("block"))
		require.NoError(t, err)
	}

	require.Equal(t, 10, store.BlockCount())
}

func TestClose(t *testing.T) {
	store := newTestStore(t)

	err := store.Close()
	require.NoError(t, err)

	// Operations after close should fail
	err = store.SaveBlock(1, makeTestHash(1), []byte("data"))
	require.Error(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	const numGoroutines = 10
	const blocksPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*blocksPerGoroutine)

	// Concurrent writes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < blocksPerGoroutine; i++ {
				height := int64(goroutineID*blocksPerGoroutine + i + 1)
				hash := makeTestHash(int(height))
				data := []byte("block data")

				if err := store.SaveBlock(height, hash, data); err != nil {
					errors <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}

	// Verify all blocks were saved
	require.Equal(t, numGoroutines*blocksPerGoroutine, store.BlockCount())

	// Concurrent reads
	wg = sync.WaitGroup{}
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < blocksPerGoroutine; i++ {
				height := int64(goroutineID*blocksPerGoroutine + i + 1)
				_, _, err := store.LoadBlock(height)
				if err != nil {
					t.Errorf("failed to load block %d: %v", height, err)
				}
			}
		}(g)
	}

	wg.Wait()
}

func TestLargeBlocks(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Test with 1MB block
	largeData := make([]byte, 1024*1024)
	_, err := rand.Read(largeData)
	require.NoError(t, err)

	hash := types.HashBlock(largeData)

	err = store.SaveBlock(1, hash, largeData)
	require.NoError(t, err)

	loadedHash, loadedData, err := store.LoadBlock(1)
	require.NoError(t, err)
	require.Equal(t, []byte(hash), loadedHash)
	require.Equal(t, largeData, loadedData)
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	// Create store and add blocks
	store1, err := NewLevelDBBlockStore(path)
	require.NoError(t, err)

	for i := 1; i <= 10; i++ {
		saveErr := store1.SaveBlock(int64(i), makeTestHash(i), []byte("block data"))
		require.NoError(t, saveErr)
	}

	height1 := store1.Height()
	base1 := store1.Base()
	require.NoError(t, store1.Close())

	// Reopen and verify
	store2, err := NewLevelDBBlockStore(path)
	require.NoError(t, err)
	defer store2.Close()

	require.Equal(t, height1, store2.Height())
	require.Equal(t, base1, store2.Base())
	require.Equal(t, 10, store2.BlockCount())

	// Verify blocks are accessible
	for i := 1; i <= 10; i++ {
		require.True(t, store2.HasBlock(int64(i)))
	}
}

// Helper functions

func newTestStore(t *testing.T) *LevelDBBlockStore {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	require.NoError(t, err)
	return store
}

func makeTestHash(n int) []byte {
	data := make([]byte, 32)
	for i := 0; i < 32; i++ {
		data[i] = byte((n + i) % 256)
	}
	return data
}

func BenchmarkSaveBlock(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	data := make([]byte, 1024) // 1KB block

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash := makeTestHash(i)
		if err := store.SaveBlock(int64(i+1), hash, data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadBlock(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	// Pre-populate with blocks
	data := make([]byte, 1024)
	for i := 1; i <= 1000; i++ {
		hash := makeTestHash(i)
		if err := store.SaveBlock(int64(i), hash, data); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		height := int64((i % 1000) + 1)
		if _, _, err := store.LoadBlock(height); err != nil {
			b.Fatal(err)
		}
	}
}
