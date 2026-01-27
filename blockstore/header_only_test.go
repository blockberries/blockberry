package blockstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

func TestHeaderOnlyBlockStore_InterfaceCompliance(t *testing.T) {
	var _ BlockStore = (*HeaderOnlyBlockStore)(nil)
}

func TestNewHeaderOnlyBlockStore(t *testing.T) {
	store := NewHeaderOnlyBlockStore()
	require.NotNil(t, store)
	assert.Equal(t, int64(0), store.Height())
	assert.Equal(t, int64(0), store.Base())
}

func TestHeaderOnlyBlockStore_SaveBlock(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	hash := []byte{1, 2, 3, 4}
	data := []byte("block data that should be discarded")

	err := store.SaveBlock(1, hash, data)
	require.NoError(t, err)

	assert.Equal(t, int64(1), store.Height())
	assert.Equal(t, int64(1), store.Base())
}

func TestHeaderOnlyBlockStore_SaveBlock_Duplicate(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	hash := []byte{1, 2, 3, 4}
	err := store.SaveBlock(1, hash, []byte("data"))
	require.NoError(t, err)

	// Duplicate should fail
	err = store.SaveBlock(1, hash, []byte("data"))
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrBlockAlreadyExists)
}

func TestHeaderOnlyBlockStore_LoadBlock(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	hash := []byte{1, 2, 3, 4}
	data := []byte("block data that should be discarded")
	_ = store.SaveBlock(1, hash, data)

	// Load should return hash but no data
	loadedHash, loadedData, err := store.LoadBlock(1)
	require.NoError(t, err)
	assert.Equal(t, hash, loadedHash)
	assert.Nil(t, loadedData) // Data should be nil for header-only store
}

func TestHeaderOnlyBlockStore_LoadBlock_NotFound(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	_, _, err := store.LoadBlock(999)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrBlockNotFound)
}

func TestHeaderOnlyBlockStore_LoadBlockByHash(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	hash := []byte{1, 2, 3, 4}
	_ = store.SaveBlock(5, hash, []byte("data"))

	// Load by hash should return height but no data
	height, data, err := store.LoadBlockByHash(hash)
	require.NoError(t, err)
	assert.Equal(t, int64(5), height)
	assert.Nil(t, data)
}

func TestHeaderOnlyBlockStore_LoadBlockByHash_NotFound(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	_, _, err := store.LoadBlockByHash([]byte{9, 9, 9})
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrBlockNotFound)
}

func TestHeaderOnlyBlockStore_HasBlock(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	assert.False(t, store.HasBlock(1))

	_ = store.SaveBlock(1, []byte{1, 2, 3}, []byte("data"))

	assert.True(t, store.HasBlock(1))
	assert.False(t, store.HasBlock(2))
}

func TestHeaderOnlyBlockStore_Height_Base(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	// Start with out of order blocks
	_ = store.SaveBlock(10, []byte{10}, []byte("data"))
	assert.Equal(t, int64(10), store.Height())
	assert.Equal(t, int64(10), store.Base())

	_ = store.SaveBlock(5, []byte{5}, []byte("data"))
	assert.Equal(t, int64(10), store.Height())
	assert.Equal(t, int64(5), store.Base())

	_ = store.SaveBlock(15, []byte{15}, []byte("data"))
	assert.Equal(t, int64(15), store.Height())
	assert.Equal(t, int64(5), store.Base())
}

func TestHeaderOnlyBlockStore_Close(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	_ = store.SaveBlock(1, []byte{1}, []byte("data"))
	assert.True(t, store.HasBlock(1))

	err := store.Close()
	require.NoError(t, err)

	// After close, state is cleared
	assert.False(t, store.HasBlock(1))
	assert.Equal(t, int64(0), store.Height())
}

func TestHeaderOnlyBlockStore_GetHash(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	hash := []byte{1, 2, 3, 4}
	_ = store.SaveBlock(1, hash, []byte("data"))

	// GetHash is a header-specific method
	loadedHash, err := store.GetHash(1)
	require.NoError(t, err)
	assert.Equal(t, hash, loadedHash)

	// Not found
	_, err = store.GetHash(999)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrBlockNotFound)
}

func TestHeaderOnlyBlockStore_HashCopy(t *testing.T) {
	store := NewHeaderOnlyBlockStore()

	hash := []byte{1, 2, 3, 4}
	_ = store.SaveBlock(1, hash, []byte("data"))

	// Modify original hash
	hash[0] = 99

	// Stored hash should not be affected
	loadedHash, _, _ := store.LoadBlock(1)
	assert.Equal(t, byte(1), loadedHash[0])
}
