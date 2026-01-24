package sync

import (
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/blockstore"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

func TestNewSyncReactor(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	require.NotNil(t, reactor)
	require.Equal(t, store, reactor.blockStore)
	require.Equal(t, int32(50), reactor.batchSize)
	require.Equal(t, time.Second, reactor.syncInterval)
	require.Equal(t, StateSynced, reactor.State())
}

func TestSyncReactor_StartStop(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, 100*time.Millisecond, 50)

	// Should not be running initially
	require.False(t, reactor.IsRunning())

	// Start
	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.IsRunning())

	// Start again should be no-op
	err = reactor.Start()
	require.NoError(t, err)

	// Stop
	err = reactor.Stop()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())

	// Stop again should be no-op
	err = reactor.Stop()
	require.NoError(t, err)
}

func TestSyncReactor_HandleMessageEmpty(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	err := reactor.HandleMessage(peer.ID("peer1"), nil)
	require.ErrorIs(t, err, types.ErrInvalidMessage)

	err = reactor.HandleMessage(peer.ID("peer1"), []byte{})
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestSyncReactor_HandleMessageUnknownType(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	w := cramberry.GetWriter()
	w.WriteTypeID(255)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.ErrorIs(t, err, types.ErrUnknownMessageType)
}

func TestSyncReactor_EncodeDecodeBlocksRequest(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	batchSize := int32(100)
	since := int64(50)
	req := &schema.BlocksRequest{
		BatchSize: &batchSize,
		Since:     &since,
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDBlocksRequest, req)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDBlocksRequest, typeID)

	// Decode message
	var decoded schema.BlocksRequest
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.NotNil(t, decoded.BatchSize)
	require.NotNil(t, decoded.Since)
	require.Equal(t, batchSize, *decoded.BatchSize)
	require.Equal(t, since, *decoded.Since)
}

func TestSyncReactor_EncodeDecodeBlocksResponse(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	height1 := int64(1)
	height2 := int64(2)
	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height1, Hash: []byte("hash1"), Data: []byte("data1")},
			{Height: &height2, Hash: []byte("hash2"), Data: []byte("data2")},
		},
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDBlocksResponse, typeID)

	// Decode message
	var decoded schema.BlocksResponse
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.Len(t, decoded.Blocks, 2)
	require.Equal(t, int64(1), *decoded.Blocks[0].Height)
	require.Equal(t, int64(2), *decoded.Blocks[1].Height)
}

func TestSyncReactor_HandleBlocksRequest(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// Add some blocks to the store
	require.NoError(t, store.SaveBlock(1, []byte("hash1"), []byte("data1")))
	require.NoError(t, store.SaveBlock(2, []byte("hash2"), []byte("data2")))
	require.NoError(t, store.SaveBlock(3, []byte("hash3"), []byte("data3")))

	// Create request
	batchSize := int32(10)
	since := int64(1)
	req := &schema.BlocksRequest{
		BatchSize: &batchSize,
		Since:     &since,
	}

	// Encode request
	data, err := reactor.encodeMessage(TypeIDBlocksRequest, req)
	require.NoError(t, err)

	// Handle request (no network, so response won't be sent)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestSyncReactor_HandleBlocksResponse(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// Create blocks with correct hashes
	block1Data := []byte("block 1 data")
	block2Data := []byte("block 2 data")
	hash1 := types.HashBlock(block1Data)
	hash2 := types.HashBlock(block2Data)
	height1 := int64(1)
	height2 := int64(2)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height1, Hash: hash1, Data: block1Data},
			{Height: &height2, Hash: hash2, Data: block2Data},
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Handle response
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Blocks should now be in store
	require.True(t, store.HasBlock(1))
	require.True(t, store.HasBlock(2))

	loadedHash, loadedData, err := store.LoadBlock(1)
	require.NoError(t, err)
	require.Equal(t, string(hash1), string(loadedHash))
	require.Equal(t, block1Data, loadedData)
}

func TestSyncReactor_HandleBlocksResponseHashMismatch(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// Create block with wrong hash
	blockData := []byte("block data")
	wrongHash := []byte("wrong hash that doesn't match")
	height := int64(1)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height, Hash: wrongHash, Data: blockData},
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Handle response
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Block should NOT be in store due to hash mismatch
	require.False(t, store.HasBlock(1))
}

func TestSyncReactor_HandleBlocksResponseWithValidator(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// Set validator that rejects all blocks
	reactor.SetValidator(func(height int64, hash, data []byte) error {
		return types.ErrInvalidBlock
	})

	// Create block with correct hash
	blockData := []byte("block data")
	hash := types.HashBlock(blockData)
	height := int64(1)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height, Hash: hash, Data: blockData},
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Handle response
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Block should NOT be in store due to validation failure
	require.False(t, store.HasBlock(1))
}

func TestSyncReactor_PeerHeightTracking(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	peerID := peer.ID("peer1")

	// Initially no height
	reactor.mu.RLock()
	_, exists := reactor.peerHeights[peerID]
	reactor.mu.RUnlock()
	require.False(t, exists)

	// Update height
	reactor.UpdatePeerHeight(peerID, 100)

	reactor.mu.RLock()
	height := reactor.peerHeights[peerID]
	reactor.mu.RUnlock()
	require.Equal(t, int64(100), height)

	// Max peer height
	reactor.UpdatePeerHeight(peer.ID("peer2"), 150)
	require.Equal(t, int64(150), reactor.getMaxPeerHeight())
}

func TestSyncReactor_OnPeerDisconnected(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	peerID := peer.ID("peer1")

	// Add peer state
	reactor.mu.Lock()
	reactor.peerHeights[peerID] = 100
	reactor.pendingSince[peerID] = 50
	reactor.mu.Unlock()

	// Disconnect peer
	reactor.OnPeerDisconnected(peerID)

	// State should be cleaned up
	reactor.mu.RLock()
	_, heightExists := reactor.peerHeights[peerID]
	_, pendingExists := reactor.pendingSince[peerID]
	reactor.mu.RUnlock()
	require.False(t, heightExists)
	require.False(t, pendingExists)
}

func TestSyncReactor_StateTransitions(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	// Initially synced
	require.Equal(t, StateSynced, reactor.State())
	require.False(t, reactor.IsSyncing())

	// Transition to syncing
	reactor.transitionToSyncing()
	require.Equal(t, StateSyncing, reactor.State())
	require.True(t, reactor.IsSyncing())

	// Transition back to synced
	reactor.transitionToSynced()
	require.Equal(t, StateSynced, reactor.State())
	require.False(t, reactor.IsSyncing())
}

func TestSyncReactor_SyncCompleteCallback(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	called := false
	reactor.SetOnSyncComplete(func() {
		called = true
	})

	// Transition to syncing first
	reactor.transitionToSyncing()
	require.False(t, called)

	// Transition to synced should call callback
	reactor.transitionToSynced()
	require.True(t, called)

	// Calling again shouldn't trigger callback (already synced)
	called = false
	reactor.transitionToSynced()
	require.False(t, called)
}

func TestSyncReactor_OnPeerConnected(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	peerID := peer.ID("peer1")
	reactor.OnPeerConnected(peerID, 100)

	reactor.mu.RLock()
	height := reactor.peerHeights[peerID]
	reactor.mu.RUnlock()
	require.Equal(t, int64(100), height)
}

func TestSyncReactor_TypeIDConstants(t *testing.T) {
	require.Equal(t, cramberry.TypeID(137), TypeIDBlocksRequest)
	require.Equal(t, cramberry.TypeID(138), TypeIDBlocksResponse)
}

func TestSyncReactor_HandleDuplicateBlock(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// Add a block first
	blockData := []byte("block data")
	hash := types.HashBlock(blockData)
	require.NoError(t, store.SaveBlock(1, hash, blockData))

	height := int64(1)
	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height, Hash: hash, Data: blockData},
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Handle response - should not error even though block exists
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Store should still have exactly the original block
	require.Equal(t, int64(1), store.Height())
}

func TestSyncReactor_GetPeersWithHeight(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	// Add some peers with different heights
	reactor.UpdatePeerHeight(peer.ID("peer1"), 50)
	reactor.UpdatePeerHeight(peer.ID("peer2"), 100)
	reactor.UpdatePeerHeight(peer.ID("peer3"), 150)

	// Get peers with height >= 100
	peers := reactor.getPeersWithHeight(100)
	require.Len(t, peers, 2) // peer2 and peer3

	// Get peers with height >= 200
	peers = reactor.getPeersWithHeight(200)
	require.Len(t, peers, 0)
}
