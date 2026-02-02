package handlers

import (
	"testing"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/blockstore"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/pkg/types"
)

func TestNewBlockReactor(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewBlockReactor(store, nil, nil)

	require.NotNil(t, reactor)
	require.Equal(t, store, reactor.blockStore)
}

func TestBlockReactor_HandleMessageEmpty(t *testing.T) {
	reactor := NewBlockReactor(nil, nil, nil)

	err := reactor.HandleMessage(peer.ID("peer1"), nil)
	require.ErrorIs(t, err, types.ErrInvalidMessage)

	err = reactor.HandleMessage(peer.ID("peer1"), []byte{})
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestBlockReactor_HandleMessageUnknownType(t *testing.T) {
	reactor := NewBlockReactor(nil, nil, nil)

	w := cramberry.GetWriter()
	w.WriteTypeID(255)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.ErrorIs(t, err, types.ErrUnknownMessageType)
}

func TestBlockReactor_EncodeDecodeBlockData(t *testing.T) {
	reactor := NewBlockReactor(nil, nil, nil)

	height := int64(100)
	hash := []byte("block hash")
	blockData := []byte("block data")

	// Encode
	data, err := reactor.encodeBlockData(height, hash, blockData)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDBlockData, typeID)

	// Decode message
	var decoded schema.BlockData
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.NotNil(t, decoded.Height)
	require.Equal(t, height, *decoded.Height)
	require.Equal(t, hash, decoded.Hash)
	require.Equal(t, blockData, decoded.Data)
}

func TestBlockReactor_HandleBlockData(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewBlockReactor(store, nil, nil)

	// Create block with correct hash
	blockData := []byte("block data content")
	hash := types.HashBlock(blockData)
	height := int64(1)

	// Encode block
	data, err := reactor.encodeBlockData(height, hash, blockData)
	require.NoError(t, err)

	// Handle message
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Block should now be in store
	require.True(t, store.HasBlock(1))

	loadedHash, loadedData, err := store.LoadBlock(1)
	require.NoError(t, err)
	require.Equal(t, string(hash), string(loadedHash))
	require.Equal(t, blockData, loadedData)
}

func TestBlockReactor_HandleBlockDataHashMismatch(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewBlockReactor(store, nil, nil)

	// Create block with wrong hash
	blockData := []byte("block data content")
	wrongHash := []byte("wrong hash that doesn't match")
	height := int64(1)

	// Encode block
	data, err := reactor.encodeBlockData(height, wrongHash, blockData)
	require.NoError(t, err)

	// Handle message - should not error but should reject the block
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Block should NOT be in store due to hash mismatch
	require.False(t, store.HasBlock(1))
}

func TestBlockReactor_HandleBlockDataWithValidator(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewBlockReactor(store, nil, nil)

	// Set validator that rejects all blocks
	reactor.SetValidator(func(height int64, hash, data []byte) error {
		return types.ErrInvalidBlock
	})

	// Create block with correct hash
	blockData := []byte("block data content")
	hash := types.HashBlock(blockData)
	height := int64(1)

	// Encode block
	data, err := reactor.encodeBlockData(height, hash, blockData)
	require.NoError(t, err)

	// Handle message
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Block should NOT be in store due to validation failure
	require.False(t, store.HasBlock(1))
}

func TestBlockReactor_HandleBlockDataDuplicate(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewBlockReactor(store, nil, nil)

	// Add block first
	blockData := []byte("block data content")
	hash := types.HashBlock(blockData)
	require.NoError(t, store.SaveBlock(1, hash, blockData))

	// Encode same block
	data, err := reactor.encodeBlockData(1, hash, blockData)
	require.NoError(t, err)

	// Handle message - should not error for duplicate
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Store should still have exactly 1 block
	require.Equal(t, int64(1), store.Height())
}

func TestBlockReactor_HandleBlockDataCallback(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewBlockReactor(store, nil, nil)

	var callbackCalled bool
	var receivedHeight int64
	var receivedHash []byte
	var receivedData []byte

	reactor.SetOnBlockReceived(func(height int64, hash, data []byte) {
		callbackCalled = true
		receivedHeight = height
		receivedHash = hash
		receivedData = data
	})

	// Create block with correct hash
	blockData := []byte("block data content")
	hash := types.HashBlock(blockData)
	height := int64(1)

	// Encode block
	data, err := reactor.encodeBlockData(height, hash, blockData)
	require.NoError(t, err)

	// Handle message
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Callback should have been called
	require.True(t, callbackCalled)
	require.Equal(t, height, receivedHeight)
	require.Equal(t, string(hash), string(receivedHash))
	require.Equal(t, blockData, receivedData)
}

func TestBlockReactor_HandleBlockDataMissingFields(t *testing.T) {
	reactor := NewBlockReactor(nil, nil, nil)

	// Create block with missing height
	block := &schema.BlockData{
		Height: nil,
		Hash:   []byte("hash"),
		Data:   []byte("data"),
	}

	msgData, err := block.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDBlockData)
	w.WriteRawBytes(msgData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestBlockReactor_TypeIDConstants(t *testing.T) {
	require.Equal(t, cramberry.TypeID(139), TypeIDBlockData)
}

func TestBlockReactor_OnPeerDisconnected(t *testing.T) {
	reactor := NewBlockReactor(nil, nil, nil)

	// Should not panic
	reactor.OnPeerDisconnected(peer.ID("peer1"))
}

func TestBlockReactor_BroadcastBlockNilDependencies(t *testing.T) {
	reactor := NewBlockReactor(nil, nil, nil)

	// Should not error with nil dependencies
	err := reactor.BroadcastBlock(1, []byte("hash"), []byte("data"))
	require.NoError(t, err)
}
