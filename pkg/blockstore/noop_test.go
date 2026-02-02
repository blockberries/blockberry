package blockstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
)

func TestNoOpBlockStore_InterfaceCompliance(t *testing.T) {
	var _ BlockStore = (*NoOpBlockStore)(nil)
}

func TestNewNoOpBlockStore(t *testing.T) {
	store := NewNoOpBlockStore()
	require.NotNil(t, store)
}

func TestNoOpBlockStore_SaveBlock(t *testing.T) {
	store := NewNoOpBlockStore()
	err := store.SaveBlock(1, []byte{1, 2, 3}, []byte("data"))
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrStoreClosed)
}

func TestNoOpBlockStore_LoadBlock(t *testing.T) {
	store := NewNoOpBlockStore()
	hash, data, err := store.LoadBlock(1)
	assert.Nil(t, hash)
	assert.Nil(t, data)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrBlockNotFound)
}

func TestNoOpBlockStore_LoadBlockByHash(t *testing.T) {
	store := NewNoOpBlockStore()
	height, data, err := store.LoadBlockByHash([]byte{1, 2, 3})
	assert.Equal(t, int64(0), height)
	assert.Nil(t, data)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrBlockNotFound)
}

func TestNoOpBlockStore_HasBlock(t *testing.T) {
	store := NewNoOpBlockStore()
	assert.False(t, store.HasBlock(1))
}

func TestNoOpBlockStore_Height(t *testing.T) {
	store := NewNoOpBlockStore()
	assert.Equal(t, int64(0), store.Height())
}

func TestNoOpBlockStore_Base(t *testing.T) {
	store := NewNoOpBlockStore()
	assert.Equal(t, int64(0), store.Base())
}

func TestNoOpBlockStore_Close(t *testing.T) {
	store := NewNoOpBlockStore()
	err := store.Close()
	require.NoError(t, err)
}
