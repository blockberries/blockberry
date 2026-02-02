package sync

import (
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/blockstore"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/pkg/types"
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

	// Start without validator should fail (fail-closed)
	err := reactor.Start()
	require.ErrorIs(t, err, types.ErrNoBlockValidator)
	require.False(t, reactor.IsRunning())

	// Set validator and start
	reactor.SetValidator(types.AcceptAllBlockValidator)
	err = reactor.Start()
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

func TestSyncReactor_StartRequiresValidator(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, 100*time.Millisecond, 50)

	// Without validator, Start should fail
	err := reactor.Start()
	require.ErrorIs(t, err, types.ErrNoBlockValidator)

	// With validator, Start should succeed
	reactor.SetValidator(types.AcceptAllBlockValidator)
	err = reactor.Start()
	require.NoError(t, err)

	// Cleanup
	_ = reactor.Stop()
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

	// Set validator to accept all blocks (for testing)
	reactor.SetValidator(types.AcceptAllBlockValidator)

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

func TestSyncReactor_HandleBlocksResponseWithoutValidator(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// No validator set - blocks should NOT be stored (fail-closed behavior)
	// This tests the DefaultBlockValidator behavior

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

	// Handle response - should not error but block should not be stored
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Block should NOT be in store (no validator = no acceptance)
	require.False(t, store.HasBlock(1))
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
	reactor.pendingRequests[peerID] = &PendingRequest{
		PeerID:      peerID,
		StartHeight: 50,
		EndHeight:   99,
		RequestedAt: time.Now(),
	}
	reactor.nextRequestHeight = 100
	reactor.mu.Unlock()

	// Disconnect peer
	reactor.OnPeerDisconnected(peerID)

	// State should be cleaned up
	reactor.mu.RLock()
	_, heightExists := reactor.peerHeights[peerID]
	_, pendingExists := reactor.pendingRequests[peerID]
	nextHeight := reactor.nextRequestHeight
	reactor.mu.RUnlock()
	require.False(t, heightExists)
	require.False(t, pendingExists)
	// Next request height should be reset to the pending request's start
	require.Equal(t, int64(50), nextHeight)
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

func TestDefaultBlockValidator(t *testing.T) {
	// DefaultBlockValidator should reject all blocks
	err := DefaultBlockValidator(1, []byte("hash"), []byte("data"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrNoBlockValidator)
}

func TestAcceptAllBlockValidator(t *testing.T) {
	// types.AcceptAllBlockValidator should accept all blocks
	err := types.AcceptAllBlockValidator(1, []byte("hash"), []byte("data"))
	require.NoError(t, err)
}

func TestSyncReactor_HandleDuplicateBlock(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)

	// Set validator to accept all blocks (for testing)
	reactor.SetValidator(types.AcceptAllBlockValidator)

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

func TestSyncReactor_HandleBlocksResponseContiguous(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)
	reactor.SetValidator(types.AcceptAllBlockValidator)

	// Create contiguous blocks starting at height 1
	block1Data := []byte("block 1 data")
	block2Data := []byte("block 2 data")
	block3Data := []byte("block 3 data")
	hash1 := types.HashBlock(block1Data)
	hash2 := types.HashBlock(block2Data)
	hash3 := types.HashBlock(block3Data)
	height1 := int64(1)
	height2 := int64(2)
	height3 := int64(3)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height1, Hash: hash1, Data: block1Data},
			{Height: &height2, Hash: hash2, Data: block2Data},
			{Height: &height3, Hash: hash3, Data: block3Data},
		},
	}

	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// All blocks should be stored
	require.True(t, store.HasBlock(1))
	require.True(t, store.HasBlock(2))
	require.True(t, store.HasBlock(3))
	require.Equal(t, int64(3), store.Height())
}

func TestSyncReactor_HandleBlocksResponseNonContiguous(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)
	reactor.SetValidator(types.AcceptAllBlockValidator)

	// Create non-contiguous blocks (gap at height 2)
	block1Data := []byte("block 1 data")
	block3Data := []byte("block 3 data")
	hash1 := types.HashBlock(block1Data)
	hash3 := types.HashBlock(block3Data)
	height1 := int64(1)
	height3 := int64(3) // Gap! Missing height 2

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height1, Hash: hash1, Data: block1Data},
			{Height: &height3, Hash: hash3, Data: block3Data},
		},
	}

	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrNonContiguousBlock)

	// No blocks should be stored due to non-contiguity
	require.False(t, store.HasBlock(1))
	require.False(t, store.HasBlock(3))
}

func TestSyncReactor_HandleBlocksResponseWrongStartHeight(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)
	reactor.SetValidator(types.AcceptAllBlockValidator)

	// Pre-populate store with block 1
	existingData := []byte("existing block")
	existingHash := types.HashBlock(existingData)
	require.NoError(t, store.SaveBlock(1, existingHash, existingData))

	// Create blocks starting at height 3 (should start at 2)
	block3Data := []byte("block 3 data")
	hash3 := types.HashBlock(block3Data)
	height3 := int64(3)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height3, Hash: hash3, Data: block3Data},
		},
	}

	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrNonContiguousBlock)

	// Block 3 should not be stored
	require.False(t, store.HasBlock(3))
}

func TestSyncReactor_HandleBlocksResponseEmptyResponse(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)
	reactor.SetValidator(types.AcceptAllBlockValidator)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{},
	}

	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Empty response should be handled gracefully
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestSyncReactor_HandleBlocksResponseWithExistingBlocks(t *testing.T) {
	store := blockstore.NewMemoryBlockStore()
	reactor := NewSyncReactor(store, nil, nil, time.Second, 50)
	reactor.SetValidator(types.AcceptAllBlockValidator)

	// Pre-populate store with blocks 1 and 2
	block1Data := []byte("block 1 data")
	block2Data := []byte("block 2 data")
	hash1 := types.HashBlock(block1Data)
	hash2 := types.HashBlock(block2Data)
	require.NoError(t, store.SaveBlock(1, hash1, block1Data))
	require.NoError(t, store.SaveBlock(2, hash2, block2Data))

	// Create response with blocks 1, 2, 3 (1 and 2 we already have)
	block3Data := []byte("block 3 data")
	hash3 := types.HashBlock(block3Data)
	height1 := int64(1)
	height2 := int64(2)
	height3 := int64(3)

	resp := &schema.BlocksResponse{
		Blocks: []schema.BlockData{
			{Height: &height1, Hash: hash1, Data: block1Data},
			{Height: &height2, Hash: hash2, Data: block2Data},
			{Height: &height3, Hash: hash3, Data: block3Data},
		},
	}

	data, err := reactor.encodeMessage(TypeIDBlocksResponse, resp)
	require.NoError(t, err)

	// Should succeed - blocks 1 and 2 are skipped, block 3 is added
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// All blocks should be in store
	require.True(t, store.HasBlock(1))
	require.True(t, store.HasBlock(2))
	require.True(t, store.HasBlock(3))
	require.Equal(t, int64(3), store.Height())
}

func TestSyncReactor_ParallelSync_Configuration(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	// Check defaults
	require.Equal(t, DefaultMaxParallel, reactor.maxParallel)
	require.Equal(t, DefaultRequestTimeout, reactor.requestTimeout)

	// Set custom values
	reactor.SetMaxParallel(8)
	require.Equal(t, 8, reactor.maxParallel)

	// Min value enforcement
	reactor.SetMaxParallel(0)
	require.Equal(t, 1, reactor.maxParallel)

	reactor.SetMaxParallel(-5)
	require.Equal(t, 1, reactor.maxParallel)

	// Set timeout
	reactor.SetRequestTimeout(60 * time.Second)
	require.Equal(t, 60*time.Second, reactor.requestTimeout)

	// Min value enforcement
	reactor.SetRequestTimeout(100 * time.Millisecond)
	require.Equal(t, time.Second, reactor.requestTimeout)
}

func TestSyncReactor_ParallelSync_PendingRequests(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)
	peerID1 := peer.ID("peer1")
	peerID2 := peer.ID("peer2")

	// Add pending requests
	reactor.mu.Lock()
	reactor.pendingRequests[peerID1] = &PendingRequest{
		PeerID:      peerID1,
		StartHeight: 1,
		EndHeight:   50,
		RequestedAt: time.Now(),
	}
	reactor.pendingRequests[peerID2] = &PendingRequest{
		PeerID:      peerID2,
		StartHeight: 51,
		EndHeight:   100,
		RequestedAt: time.Now(),
	}
	reactor.mu.Unlock()

	require.Len(t, reactor.pendingRequests, 2)

	// Disconnect one peer
	reactor.OnPeerDisconnected(peerID1)

	reactor.mu.RLock()
	require.Len(t, reactor.pendingRequests, 1)
	require.Contains(t, reactor.pendingRequests, peerID2)
	reactor.mu.RUnlock()
}

func TestSyncReactor_ParallelSync_TimeoutCleanup(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)
	reactor.SetRequestTimeout(100 * time.Millisecond) // Short timeout for test

	peerID := peer.ID("peer1")

	// Add an old pending request
	reactor.mu.Lock()
	reactor.pendingRequests[peerID] = &PendingRequest{
		PeerID:      peerID,
		StartHeight: 1,
		EndHeight:   50,
		RequestedAt: time.Now().Add(-time.Second), // Already timed out
	}
	reactor.nextRequestHeight = 51
	reactor.mu.Unlock()

	// Cleanup should remove the timed out request
	reactor.mu.Lock()
	reactor.cleanupTimedOutRequestsLocked()
	reactor.mu.Unlock()

	reactor.mu.RLock()
	require.Len(t, reactor.pendingRequests, 0)
	// Next request height should be reset
	require.Equal(t, int64(1), reactor.nextRequestHeight)
	reactor.mu.RUnlock()
}

func TestSyncReactor_ParallelSync_GetPeersWithHeightLocked(t *testing.T) {
	reactor := NewSyncReactor(nil, nil, nil, time.Second, 50)

	// Add peer heights
	reactor.mu.Lock()
	reactor.peerHeights[peer.ID("peer1")] = 100
	reactor.peerHeights[peer.ID("peer2")] = 50
	reactor.peerHeights[peer.ID("peer3")] = 150
	peers := reactor.getPeersWithHeightLocked(75)
	reactor.mu.Unlock()

	require.Len(t, peers, 2)
	// Both peer1 (100) and peer3 (150) should be included
	peerSet := make(map[peer.ID]bool)
	for _, p := range peers {
		peerSet[p] = true
	}
	require.True(t, peerSet[peer.ID("peer1")])
	require.True(t, peerSet[peer.ID("peer3")])
	require.False(t, peerSet[peer.ID("peer2")])
}

func TestPendingRequest_Structure(t *testing.T) {
	now := time.Now()
	req := &PendingRequest{
		PeerID:      peer.ID("test-peer"),
		StartHeight: 100,
		EndHeight:   199,
		RequestedAt: now,
	}

	require.Equal(t, peer.ID("test-peer"), req.PeerID)
	require.Equal(t, int64(100), req.StartHeight)
	require.Equal(t, int64(199), req.EndHeight)
	require.Equal(t, now, req.RequestedAt)
}
