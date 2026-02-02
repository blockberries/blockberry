package statestore

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshot_Constants(t *testing.T) {
	assert.Equal(t, 10*1024*1024, DefaultChunkSize)
	assert.Equal(t, 1, SnapshotVersion)
}

func TestSnapshot_Errors(t *testing.T) {
	assert.Error(t, ErrSnapshotNotFound)
	assert.Error(t, ErrSnapshotExists)
	assert.Error(t, ErrSnapshotInvalid)
	assert.Error(t, ErrSnapshotChunkNotFound)
	assert.Error(t, ErrSnapshotCorrupt)
}

func TestFileSnapshotStore_Create(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a memory IAVL store with some data
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Add some data
	for i := 0; i < 10; i++ {
		err := store.Set([]byte{byte(i)}, []byte{byte(i * 2)})
		require.NoError(t, err)
	}
	_, _, err = store.Commit()
	require.NoError(t, err)

	// Create snapshot store
	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)
	snapshotStore.SetChunkSize(1024) // Small chunk size for testing

	// Create snapshot
	snapshot, err := snapshotStore.Create(1)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	assert.Equal(t, uint32(SnapshotVersion), snapshot.Version)
	assert.Equal(t, int64(1), snapshot.Height)
	assert.NotEmpty(t, snapshot.Hash)
	assert.Greater(t, snapshot.Chunks, 0)
	assert.NotEmpty(t, snapshot.AppHash)
	assert.False(t, snapshot.CreatedAt.IsZero())
}

func TestFileSnapshotStore_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	// Create snapshots at different heights
	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)
	snapshotStore.SetChunkSize(1024)

	// Create multiple versions
	for height := int64(1); height <= 3; height++ {
		err := store.Set([]byte{byte(height)}, []byte{byte(height)})
		require.NoError(t, err)
		_, _, err = store.Commit()
		require.NoError(t, err)

		_, err = snapshotStore.Create(height)
		require.NoError(t, err)
	}

	// List snapshots
	snapshots, err := snapshotStore.List()
	require.NoError(t, err)
	assert.Len(t, snapshots, 3)

	// Should be sorted by height descending
	assert.Equal(t, int64(3), snapshots[0].Height)
	assert.Equal(t, int64(2), snapshots[1].Height)
	assert.Equal(t, int64(1), snapshots[2].Height)
}

func TestFileSnapshotStore_Load(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	_ = store.Set([]byte("key"), []byte("value"))
	_, _, _ = store.Commit()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	// Create snapshot
	created, err := snapshotStore.Create(1)
	require.NoError(t, err)

	// Load snapshot
	loaded, err := snapshotStore.Load(created.Hash)
	require.NoError(t, err)

	assert.Equal(t, created.Version, loaded.Version)
	assert.Equal(t, created.Height, loaded.Height)
	assert.Equal(t, created.Chunks, loaded.Chunks)
	assert.True(t, bytes.Equal(created.Hash, loaded.Hash))
}

func TestFileSnapshotStore_Load_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	_, err = snapshotStore.Load([]byte("nonexistent"))
	assert.ErrorIs(t, err, ErrSnapshotNotFound)
}

func TestFileSnapshotStore_LoadChunk(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	_ = store.Set([]byte("key"), []byte("value"))
	_, _, _ = store.Commit()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)
	snapshotStore.SetChunkSize(100) // Small chunks

	snapshot, err := snapshotStore.Create(1)
	require.NoError(t, err)

	// Load each chunk
	for i := 0; i < snapshot.Chunks; i++ {
		chunk, err := snapshotStore.LoadChunk(snapshot.Hash, i)
		require.NoError(t, err)
		assert.Equal(t, i, chunk.Index)
		assert.NotEmpty(t, chunk.Hash)
		assert.NotEmpty(t, chunk.Data)
	}
}

func TestFileSnapshotStore_LoadChunk_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	_ = store.Set([]byte("key"), []byte("value"))
	_, _, _ = store.Commit()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	snapshot, err := snapshotStore.Create(1)
	require.NoError(t, err)

	// Try to load non-existent chunk
	_, err = snapshotStore.LoadChunk(snapshot.Hash, 9999)
	assert.ErrorIs(t, err, ErrSnapshotChunkNotFound)
}

func TestFileSnapshotStore_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	_ = store.Set([]byte("key"), []byte("value"))
	_, _, _ = store.Commit()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	snapshot, err := snapshotStore.Create(1)
	require.NoError(t, err)

	// Verify it exists
	assert.True(t, snapshotStore.Has(snapshot.Hash))

	// Delete
	err = snapshotStore.Delete(snapshot.Hash)
	require.NoError(t, err)

	// Verify it's gone
	assert.False(t, snapshotStore.Has(snapshot.Hash))
}

func TestFileSnapshotStore_Delete_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	err = snapshotStore.Delete([]byte("nonexistent"))
	assert.ErrorIs(t, err, ErrSnapshotNotFound)
}

func TestFileSnapshotStore_Prune(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	// Create 5 snapshots
	for i := 1; i <= 5; i++ {
		_ = store.Set([]byte{byte(i)}, []byte{byte(i)})
		_, _, _ = store.Commit()
		_, err := snapshotStore.Create(int64(i))
		require.NoError(t, err)
	}

	snapshots, _ := snapshotStore.List()
	assert.Len(t, snapshots, 5)

	// Prune keeping only 2 recent
	err = snapshotStore.Prune(2)
	require.NoError(t, err)

	snapshots, _ = snapshotStore.List()
	assert.Len(t, snapshots, 2)

	// Should have heights 5 and 4 (most recent)
	assert.Equal(t, int64(5), snapshots[0].Height)
	assert.Equal(t, int64(4), snapshots[1].Height)
}

func TestFileSnapshotStore_Has(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer store.Close()

	_ = store.Set([]byte("key"), []byte("value"))
	_, _, _ = store.Commit()

	snapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "snapshots"), store)
	require.NoError(t, err)

	// Initially doesn't have any snapshot
	assert.False(t, snapshotStore.Has([]byte("nonexistent")))

	// Create snapshot
	snapshot, err := snapshotStore.Create(1)
	require.NoError(t, err)

	// Now it should have it
	assert.True(t, snapshotStore.Has(snapshot.Hash))
}

func TestFileSnapshotStore_Import(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapshot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create source store with data
	sourceStore, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer sourceStore.Close()

	// Add data to source
	for i := 0; i < 10; i++ {
		err := sourceStore.Set([]byte{byte(i)}, []byte{byte(i * 2)})
		require.NoError(t, err)
	}
	_, _, err = sourceStore.Commit()
	require.NoError(t, err)

	// Create snapshot from source
	sourceSnapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "source_snapshots"), sourceStore)
	require.NoError(t, err)

	snapshot, err := sourceSnapshotStore.Create(1)
	require.NoError(t, err)

	// Collect chunks
	var chunks [][]byte
	for i := 0; i < snapshot.Chunks; i++ {
		chunk, err := sourceSnapshotStore.LoadChunk(snapshot.Hash, i)
		require.NoError(t, err)
		chunks = append(chunks, chunk.Data)
	}

	// Create target store (empty)
	targetStore, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	defer targetStore.Close()

	targetSnapshotStore, err := NewFileSnapshotStore(filepath.Join(tmpDir, "target_snapshots"), targetStore)
	require.NoError(t, err)

	// Import snapshot
	chunkProvider := NewMemoryChunkProvider(chunks)
	err = targetSnapshotStore.Import(snapshot, chunkProvider)
	require.NoError(t, err)

	// Verify data was imported
	for i := 0; i < 10; i++ {
		value, err := targetStore.Get([]byte{byte(i)})
		require.NoError(t, err)
		assert.Equal(t, []byte{byte(i * 2)}, value)
	}
}

func TestMemoryChunkProvider(t *testing.T) {
	chunks := [][]byte{
		[]byte("chunk0"),
		[]byte("chunk1"),
		[]byte("chunk2"),
	}

	provider := NewMemoryChunkProvider(chunks)

	assert.Equal(t, 3, provider.ChunkCount())

	for i := 0; i < 3; i++ {
		chunk, err := provider.GetChunk(i)
		require.NoError(t, err)
		assert.Equal(t, chunks[i], chunk)
	}

	// Invalid index
	_, err := provider.GetChunk(-1)
	assert.ErrorIs(t, err, ErrSnapshotChunkNotFound)

	_, err = provider.GetChunk(100)
	assert.ErrorIs(t, err, ErrSnapshotChunkNotFound)
}

func TestSplitIntoChunks(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		chunkSize int
		expected  int
	}{
		{
			name:      "empty data",
			data:      nil,
			chunkSize: 10,
			expected:  0,
		},
		{
			name:      "single chunk",
			data:      []byte("hello"),
			chunkSize: 10,
			expected:  1,
		},
		{
			name:      "exact chunks",
			data:      []byte("helloworld"),
			chunkSize: 5,
			expected:  2,
		},
		{
			name:      "partial last chunk",
			data:      []byte("hello world!"),
			chunkSize: 5,
			expected:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := splitIntoChunks(tt.data, tt.chunkSize)
			assert.Len(t, chunks, tt.expected)

			// Verify chunks can be reassembled
			if len(chunks) > 0 {
				var reassembled []byte
				for _, chunk := range chunks {
					reassembled = append(reassembled, chunk...)
				}
				assert.Equal(t, tt.data, reassembled)
			}
		})
	}
}

func TestEncodeDecodeSnapshotMetadata(t *testing.T) {
	original := &Snapshot{
		Version:   1,
		Height:    12345,
		Hash:      []byte("snapshot_hash"),
		ChunkSize: 1024,
		Chunks:    10,
		AppHash:   []byte("app_hash"),
		CreatedAt: time.Now().Truncate(time.Nanosecond),
		Metadata:  []byte("metadata"),
	}

	encoded, err := encodeSnapshotMetadata(original)
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	decoded, err := decodeSnapshotMetadata(encoded)
	require.NoError(t, err)

	assert.Equal(t, original.Version, decoded.Version)
	assert.Equal(t, original.Height, decoded.Height)
	assert.Equal(t, original.Hash, decoded.Hash)
	assert.Equal(t, original.ChunkSize, decoded.ChunkSize)
	assert.Equal(t, original.Chunks, decoded.Chunks)
	assert.Equal(t, original.AppHash, decoded.AppHash)
	assert.Equal(t, original.CreatedAt.UnixNano(), decoded.CreatedAt.UnixNano())
	assert.Equal(t, original.Metadata, decoded.Metadata)
}

func TestEncodeInt64(t *testing.T) {
	testCases := []int64{0, 1, 255, 65535, 1 << 32, 1 << 62}

	for _, v := range testCases {
		encoded := encodeInt64(v)
		assert.Len(t, encoded, 8)
	}
}

func TestSnapshot_Interface(t *testing.T) {
	// Verify FileSnapshotStore implements SnapshotStore
	var _ SnapshotStore = (*FileSnapshotStore)(nil)
}

func TestSnapshotInfo_Fields(t *testing.T) {
	info := &SnapshotInfo{
		Height:    100,
		Hash:      []byte("hash"),
		Chunks:    5,
		Size:      1024,
		CreatedAt: time.Now(),
	}

	assert.Equal(t, int64(100), info.Height)
	assert.Equal(t, []byte("hash"), info.Hash)
	assert.Equal(t, 5, info.Chunks)
	assert.Equal(t, int64(1024), info.Size)
	assert.False(t, info.CreatedAt.IsZero())
}

func TestSnapshotChunk_Fields(t *testing.T) {
	chunk := &SnapshotChunk{
		Index: 0,
		Hash:  []byte("chunk_hash"),
		Data:  []byte("chunk_data"),
	}

	assert.Equal(t, 0, chunk.Index)
	assert.Equal(t, []byte("chunk_hash"), chunk.Hash)
	assert.Equal(t, []byte("chunk_data"), chunk.Data)
}
