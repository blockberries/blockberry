package sync

import (
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/blockstore"
	"github.com/blockberries/blockberry/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

// Block sync message type IDs from schema.
const (
	TypeIDBlocksRequest  cramberry.TypeID = 137
	TypeIDBlocksResponse cramberry.TypeID = 138
)

// SyncState represents the current synchronization state.
type SyncState int

const (
	// StateSynced indicates the node is fully synced with peers.
	StateSynced SyncState = iota
	// StateSyncing indicates the node is catching up with peers.
	StateSyncing
)

// BlockValidator is a callback for validating blocks before storing.
// This is a required component - blocks will not be accepted without validation.
type BlockValidator func(height int64, hash, data []byte) error

// DefaultBlockValidator is a fail-closed validator that rejects all blocks.
// This is used when no validator is explicitly set, ensuring blocks are never
// accepted without proper validation. Applications MUST provide their own
// validator to accept blocks.
var DefaultBlockValidator BlockValidator = func(height int64, hash, data []byte) error {
	return fmt.Errorf("%w: no block validator configured", types.ErrNoBlockValidator)
}

// AcceptAllBlockValidator is a validator that accepts all blocks.
// WARNING: This should ONLY be used for testing purposes.
// Production systems must use a proper validator that verifies block integrity.
var AcceptAllBlockValidator BlockValidator = func(height int64, hash, data []byte) error {
	return nil
}

// SyncReactor handles block synchronization with peers.
type SyncReactor struct {
	// Dependencies
	blockStore  blockstore.BlockStore
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// Configuration
	syncInterval time.Duration
	batchSize    int32

	// Block validation callback
	validator BlockValidator

	// Sync state
	state       SyncState
	peerHeights map[peer.ID]int64

	// Pending sync requests
	pendingSince map[peer.ID]int64 // peerID -> requested since height
	lastRequest  time.Time

	mu sync.RWMutex

	// Lifecycle
	running bool
	stop    chan struct{}
	wg      sync.WaitGroup

	// Sync completion callback
	onSyncComplete func()
}

// NewSyncReactor creates a new sync reactor.
func NewSyncReactor(
	blockStore blockstore.BlockStore,
	network *p2p.Network,
	peerManager *p2p.PeerManager,
	syncInterval time.Duration,
	batchSize int32,
) *SyncReactor {
	return &SyncReactor{
		blockStore:   blockStore,
		network:      network,
		peerManager:  peerManager,
		syncInterval: syncInterval,
		batchSize:    batchSize,
		state:        StateSynced,
		peerHeights:  make(map[peer.ID]int64),
		pendingSince: make(map[peer.ID]int64),
		stop:         make(chan struct{}),
	}
}

// Name returns the component name for identification.
func (r *SyncReactor) Name() string {
	return "sync-reactor"
}

// SetValidator sets the block validation callback.
func (r *SyncReactor) SetValidator(v BlockValidator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validator = v
}

// SetOnSyncComplete sets the callback for when sync completes.
func (r *SyncReactor) SetOnSyncComplete(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onSyncComplete = fn
}

// Start begins the sync loop.
// Returns ErrNoBlockValidator if no block validator has been set.
// This is a safety measure to prevent accepting unvalidated blocks.
func (r *SyncReactor) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}

	// Fail-closed: require a block validator to be set
	if r.validator == nil {
		r.mu.Unlock()
		return types.ErrNoBlockValidator
	}

	r.running = true
	r.stop = make(chan struct{})
	r.mu.Unlock()

	r.wg.Add(1)
	go r.syncLoop()

	return nil
}

// Stop halts the sync loop.
func (r *SyncReactor) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	close(r.stop)
	r.mu.Unlock()

	r.wg.Wait()
	return nil
}

// IsRunning returns whether the reactor is running.
func (r *SyncReactor) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// State returns the current sync state.
func (r *SyncReactor) State() SyncState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// IsSyncing returns true if currently syncing.
func (r *SyncReactor) IsSyncing() bool {
	return r.State() == StateSyncing
}

// syncLoop periodically checks if we need to sync and requests blocks.
func (r *SyncReactor) syncLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.checkSync()
		}
	}
}

// checkSync determines if we need to sync and initiates requests.
func (r *SyncReactor) checkSync() {
	if r.peerManager == nil || r.network == nil || r.blockStore == nil {
		return
	}

	ourHeight := r.blockStore.Height()
	maxPeerHeight := r.getMaxPeerHeight()

	if maxPeerHeight <= ourHeight {
		// We're caught up
		r.transitionToSynced()
		return
	}

	// We're behind - start syncing
	r.transitionToSyncing()

	// Request blocks from a peer who has them
	r.requestBlocks(ourHeight)
}

// getMaxPeerHeight returns the maximum height among all connected peers.
func (r *SyncReactor) getMaxPeerHeight() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var maxHeight int64
	for _, height := range r.peerHeights {
		if height > maxHeight {
			maxHeight = height
		}
	}
	return maxHeight
}

// transitionToSynced transitions to synced state.
func (r *SyncReactor) transitionToSynced() {
	r.mu.Lock()
	if r.state == StateSynced {
		r.mu.Unlock()
		return
	}
	r.state = StateSynced
	callback := r.onSyncComplete
	r.mu.Unlock()

	if callback != nil {
		callback()
	}
}

// transitionToSyncing transitions to syncing state.
func (r *SyncReactor) transitionToSyncing() {
	r.mu.Lock()
	r.state = StateSyncing
	r.mu.Unlock()
}

// requestBlocks requests blocks from peers starting from our current height.
func (r *SyncReactor) requestBlocks(since int64) {
	peers := r.getPeersWithHeight(since + 1)
	if len(peers) == 0 {
		return
	}

	// Request from first available peer
	peerID := peers[0]

	r.mu.Lock()
	// Check if we already have a pending request from this peer
	if _, pending := r.pendingSince[peerID]; pending {
		r.mu.Unlock()
		return
	}
	r.pendingSince[peerID] = since + 1
	r.lastRequest = time.Now()
	r.mu.Unlock()

	if err := r.SendBlocksRequest(peerID, since+1); err != nil {
		r.mu.Lock()
		delete(r.pendingSince, peerID)
		r.mu.Unlock()
	}
}

// getPeersWithHeight returns peers that have blocks above the given height.
func (r *SyncReactor) getPeersWithHeight(minHeight int64) []peer.ID {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var peers []peer.ID
	for peerID, height := range r.peerHeights {
		if height >= minHeight {
			peers = append(peers, peerID)
		}
	}
	return peers
}

// UpdatePeerHeight updates the known height for a peer.
func (r *SyncReactor) UpdatePeerHeight(peerID peer.ID, height int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peerHeights[peerID] = height
}

// SendBlocksRequest sends a BlocksRequest to a peer.
func (r *SyncReactor) SendBlocksRequest(peerID peer.ID, since int64) error {
	if r.network == nil {
		return nil
	}

	req := &schema.BlocksRequest{
		BatchSize: &r.batchSize,
		Since:     &since,
	}

	data, err := r.encodeMessage(TypeIDBlocksRequest, req)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamBlockSync, data)
}

// HandleMessage processes incoming block sync messages.
func (r *SyncReactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("blocksync: %w: empty message", types.ErrInvalidMessage)
	}

	reader := cramberry.NewReader(data)
	typeID := reader.ReadTypeID()
	if reader.Err() != nil {
		return fmt.Errorf("blocksync: %w: failed to read type ID", types.ErrInvalidMessage)
	}

	remaining := reader.Remaining()

	switch typeID {
	case TypeIDBlocksRequest:
		if err := r.handleBlocksRequest(peerID, remaining); err != nil {
			return fmt.Errorf("blocksync/request: %w", err)
		}
		return nil
	case TypeIDBlocksResponse:
		if err := r.handleBlocksResponse(peerID, remaining); err != nil {
			return fmt.Errorf("blocksync/response: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("blocksync: %w: type ID %d", types.ErrUnknownMessageType, typeID)
	}
}

// handleBlocksRequest responds with blocks from the store.
func (r *SyncReactor) handleBlocksRequest(peerID peer.ID, data []byte) error {
	var req schema.BlocksRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	// Validate required fields
	if req.BatchSize == nil || req.Since == nil {
		return types.ErrInvalidMessage
	}

	// Validate and clamp batch size to safe limits
	batchSize := types.ClampBatchSize(*req.BatchSize, types.MaxBatchSize)

	// Validate height
	since := *req.Since
	if err := types.ValidateHeight(since); err != nil {
		return types.ErrInvalidMessage
	}

	if r.blockStore == nil {
		return nil
	}
	ourHeight := r.blockStore.Height()

	blocks := make([]schema.BlockData, 0, batchSize)
	for height := since; height <= ourHeight && len(blocks) < int(batchSize); height++ {
		hash, blockData, err := r.blockStore.LoadBlock(height)
		if err != nil {
			// Block not found or error, stop here
			break
		}

		blocks = append(blocks, schema.BlockData{
			Height: &height,
			Hash:   hash,
			Data:   blockData,
		})

		// Mark block as sent to this peer
		if r.peerManager != nil {
			_ = r.peerManager.MarkBlockSent(peerID, height)
		}
	}

	if len(blocks) == 0 {
		return nil
	}

	resp := &schema.BlocksResponse{
		Blocks: blocks,
	}

	return r.sendBlocksResponse(peerID, resp)
}

// sendBlocksResponse sends a BlocksResponse to a peer.
func (r *SyncReactor) sendBlocksResponse(peerID peer.ID, resp *schema.BlocksResponse) error {
	if r.network == nil {
		return nil
	}

	data, err := r.encodeMessage(TypeIDBlocksResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamBlockSync, data)
}

// handleBlocksResponse processes received blocks.
func (r *SyncReactor) handleBlocksResponse(peerID peer.ID, data []byte) error {
	var resp schema.BlocksResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	// Clear pending request for this peer
	r.mu.Lock()
	delete(r.pendingSince, peerID)
	r.mu.Unlock()

	if r.blockStore == nil {
		return nil
	}

	// Empty response is valid (peer may not have requested blocks)
	if len(resp.Blocks) == 0 {
		r.checkSync()
		return nil
	}

	// Verify blocks are contiguous and in order.
	// First, validate all blocks in the response are properly structured
	// and check for contiguity before storing any.
	expectedHeight := r.blockStore.Height() + 1
	for i, block := range resp.Blocks {
		// Validate block data structure
		if err := types.ValidateBlockData(block.Height, block.Hash, block.Data); err != nil {
			if r.network != nil {
				_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock, err.Error())
			}
			return fmt.Errorf("%w: invalid block data at index %d", types.ErrInvalidBlock, i)
		}

		height := *block.Height

		// First block must start at our expected height (or we already have it)
		if i == 0 {
			if height < expectedHeight {
				// Peer sent blocks we already have, skip to find starting point
				if r.blockStore.HasBlock(height) {
					expectedHeight = height + 1
					continue
				}
			}
			if height > expectedHeight {
				// Gap detected - peer skipped blocks
				if r.network != nil {
					_ = r.network.AddPenalty(peerID, p2p.PenaltyProtocolViolation, p2p.ReasonProtocolViolation,
						fmt.Sprintf("non-contiguous blocks: expected height %d, got %d", expectedHeight, height))
				}
				return fmt.Errorf("%w: expected height %d, got %d", types.ErrNonContiguousBlock, expectedHeight, height)
			}
			expectedHeight = height + 1
		} else {
			// Subsequent blocks must be contiguous
			if height != expectedHeight {
				if r.network != nil {
					_ = r.network.AddPenalty(peerID, p2p.PenaltyProtocolViolation, p2p.ReasonProtocolViolation,
						fmt.Sprintf("non-contiguous blocks: expected height %d, got %d", expectedHeight, height))
				}
				return fmt.Errorf("%w: expected height %d, got %d", types.ErrNonContiguousBlock, expectedHeight, height)
			}
			expectedHeight++
		}
	}

	// Now process and store the blocks (we know they're contiguous)
	for _, block := range resp.Blocks {
		height := *block.Height

		// Skip if we already have this block
		if r.blockStore.HasBlock(height) {
			continue
		}

		// Verify hash matches content using constant-time comparison to prevent timing attacks
		computedHash := types.HashBlock(block.Data)
		if !types.HashEqual(computedHash, block.Hash) {
			if r.network != nil {
				_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock, "block hash mismatch")
			}
			continue
		}

		// Validate block using the configured validator or DefaultBlockValidator
		// This is fail-closed: if no validator is set, blocks are rejected
		r.mu.RLock()
		validator := r.validator
		r.mu.RUnlock()

		if validator == nil {
			validator = DefaultBlockValidator
		}

		if err := validator(height, block.Hash, block.Data); err != nil {
			if r.network != nil {
				_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock, err.Error())
			}
			continue
		}

		// Store block
		if err := r.blockStore.SaveBlock(height, block.Hash, block.Data); err != nil {
			// Could be duplicate or storage error
			continue
		}

		// Mark as received from this peer
		if r.peerManager != nil {
			_ = r.peerManager.MarkBlockReceived(peerID, height)
		}
	}

	// Check if we need more blocks
	r.checkSync()

	return nil
}

// OnPeerConnected is called when a new peer connects.
func (r *SyncReactor) OnPeerConnected(peerID peer.ID, height int64) {
	r.UpdatePeerHeight(peerID, height)
}

// OnPeerDisconnected cleans up state for a disconnected peer.
func (r *SyncReactor) OnPeerDisconnected(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.peerHeights, peerID)
	delete(r.pendingSince, peerID)
}

// encodeMessage encodes a message with its type ID prefix.
func (r *SyncReactor) encodeMessage(typeID cramberry.TypeID, msg interface {
	MarshalCramberry() ([]byte, error)
}) ([]byte, error) {
	msgData, err := msg.MarshalCramberry()
	if err != nil {
		return nil, err
	}

	w := cramberry.GetWriter()
	defer cramberry.PutWriter(w)

	w.WriteTypeID(typeID)
	w.WriteRawBytes(msgData)

	if w.Err() != nil {
		return nil, w.Err()
	}

	return w.BytesCopy(), nil
}
