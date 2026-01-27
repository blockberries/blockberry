package sync

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/statestore"
	"github.com/blockberries/blockberry/types"
)

// State sync message type IDs from schema.
const (
	TypeIDSnapshotsRequest       cramberry.TypeID = 144
	TypeIDSnapshotsResponse      cramberry.TypeID = 145
	TypeIDSnapshotChunkRequest   cramberry.TypeID = 146
	TypeIDSnapshotChunkResponse  cramberry.TypeID = 147
)

// StateSyncState represents the current state sync state.
type StateSyncState int

const (
	// StateSyncIdle indicates state sync is not active.
	StateSyncIdle StateSyncState = iota
	// StateSyncDiscovering indicates we are discovering available snapshots.
	StateSyncDiscovering
	// StateSyncDownloading indicates we are downloading a snapshot.
	StateSyncDownloading
	// StateSyncApplying indicates we are applying a snapshot.
	StateSyncApplying
	// StateSyncComplete indicates state sync has completed successfully.
	StateSyncComplete
	// StateSyncFailed indicates state sync has failed.
	StateSyncFailed
)

// String returns a string representation of the state sync state.
func (s StateSyncState) String() string {
	switch s {
	case StateSyncIdle:
		return "idle"
	case StateSyncDiscovering:
		return "discovering"
	case StateSyncDownloading:
		return "downloading"
	case StateSyncApplying:
		return "applying"
	case StateSyncComplete:
		return "complete"
	case StateSyncFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// SnapshotOffer represents a snapshot offered by a peer.
type SnapshotOffer struct {
	PeerID    peer.ID
	Height    int64
	Hash      []byte
	Chunks    int
	AppHash   []byte
	CreatedAt time.Time
}

// StateSyncReactor handles state synchronization via snapshots.
type StateSyncReactor struct {
	// Dependencies
	network       *p2p.Network
	peerManager   *p2p.PeerManager
	snapshotStore statestore.SnapshotStore

	// Configuration
	trustHeight         int64
	trustHash           []byte
	discoveryInterval   time.Duration
	chunkRequestTimeout time.Duration
	maxChunkRetries     int

	// State
	state          StateSyncState
	offers         map[string]*SnapshotOffer // hash hex -> offer
	selectedOffer  *SnapshotOffer
	chunks         [][]byte
	chunkStatus    []bool // true if chunk has been received
	pendingChunks  map[int]peer.ID // chunk index -> requesting peer
	chunkRetries   map[int]int     // chunk index -> retry count
	lastChunkTime  map[int]time.Time // chunk index -> last request time

	mu sync.RWMutex

	// Lifecycle
	running bool
	stop    chan struct{}
	wg      sync.WaitGroup

	// Callbacks
	onComplete func(height int64, appHash []byte)
	onFailed   func(err error)
}

// NewStateSyncReactor creates a new state sync reactor.
func NewStateSyncReactor(
	network *p2p.Network,
	peerManager *p2p.PeerManager,
	snapshotStore statestore.SnapshotStore,
	trustHeight int64,
	trustHash []byte,
	discoveryInterval time.Duration,
	chunkRequestTimeout time.Duration,
	maxChunkRetries int,
) *StateSyncReactor {
	return &StateSyncReactor{
		network:             network,
		peerManager:         peerManager,
		snapshotStore:       snapshotStore,
		trustHeight:         trustHeight,
		trustHash:           trustHash,
		discoveryInterval:   discoveryInterval,
		chunkRequestTimeout: chunkRequestTimeout,
		maxChunkRetries:     maxChunkRetries,
		state:               StateSyncIdle,
		offers:              make(map[string]*SnapshotOffer),
		pendingChunks:       make(map[int]peer.ID),
		chunkRetries:        make(map[int]int),
		lastChunkTime:       make(map[int]time.Time),
		stop:                make(chan struct{}),
	}
}

// Name returns the component name for identification.
func (r *StateSyncReactor) Name() string {
	return "statesync-reactor"
}

// SetOnComplete sets the callback for when state sync completes.
func (r *StateSyncReactor) SetOnComplete(fn func(height int64, appHash []byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onComplete = fn
}

// SetOnFailed sets the callback for when state sync fails.
func (r *StateSyncReactor) SetOnFailed(fn func(err error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onFailed = fn
}

// Start begins the state sync process.
func (r *StateSyncReactor) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.state = StateSyncDiscovering
	r.stop = make(chan struct{})
	r.mu.Unlock()

	r.wg.Add(1)
	go r.syncLoop()

	return nil
}

// Stop halts the state sync process.
func (r *StateSyncReactor) Stop() error {
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
func (r *StateSyncReactor) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// State returns the current state sync state.
func (r *StateSyncReactor) State() StateSyncState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Progress returns the current progress as a percentage (0-100).
func (r *StateSyncReactor) Progress() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.selectedOffer == nil || r.chunks == nil {
		return 0
	}

	received := 0
	for _, status := range r.chunkStatus {
		if status {
			received++
		}
	}

	if len(r.chunkStatus) == 0 {
		return 0
	}

	return (received * 100) / len(r.chunkStatus)
}

// syncLoop runs the main state sync loop.
func (r *StateSyncReactor) syncLoop() {
	defer r.wg.Done()

	discoveryTicker := time.NewTicker(r.discoveryInterval)
	defer discoveryTicker.Stop()

	chunkTicker := time.NewTicker(time.Second)
	defer chunkTicker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-discoveryTicker.C:
			r.discoverSnapshots()
		case <-chunkTicker.C:
			r.checkChunkTimeouts()
			r.requestMissingChunks()
		}
	}
}

// discoverSnapshots requests snapshot information from peers.
func (r *StateSyncReactor) discoverSnapshots() {
	r.mu.RLock()
	state := r.state
	r.mu.RUnlock()

	// Only discover in the discovering state
	if state != StateSyncDiscovering {
		return
	}

	if r.network == nil || r.peerManager == nil {
		return
	}

	// Request snapshots from all connected peers
	peers := r.peerManager.GetConnectedPeers()
	for _, peerID := range peers {
		_ = r.sendSnapshotsRequest(peerID, r.trustHeight)
	}

	// After discovery interval, select best snapshot if we have offers
	r.mu.Lock()
	if len(r.offers) > 0 && r.selectedOffer == nil {
		r.selectBestSnapshot()
	}
	r.mu.Unlock()
}

// selectBestSnapshot selects the best snapshot from available offers.
// Must be called with mutex held.
func (r *StateSyncReactor) selectBestSnapshot() {
	var best *SnapshotOffer
	for _, offer := range r.offers {
		// Must be at or above trust height
		if offer.Height < r.trustHeight {
			continue
		}

		// If we have trust hash, verify it matches at trust height
		if offer.Height == r.trustHeight && len(r.trustHash) > 0 {
			if !bytes.Equal(offer.AppHash, r.trustHash) {
				continue
			}
		}

		// Prefer highest height
		if best == nil || offer.Height > best.Height {
			best = offer
		}
	}

	if best == nil {
		return
	}

	r.selectedOffer = best
	r.chunks = make([][]byte, best.Chunks)
	r.chunkStatus = make([]bool, best.Chunks)
	r.state = StateSyncDownloading
}

// checkChunkTimeouts checks for timed out chunk requests.
func (r *StateSyncReactor) checkChunkTimeouts() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateSyncDownloading {
		return
	}

	now := time.Now()
	for idx, reqTime := range r.lastChunkTime {
		if now.Sub(reqTime) > r.chunkRequestTimeout {
			// Timeout - increment retry count
			r.chunkRetries[idx]++
			delete(r.pendingChunks, idx)
			delete(r.lastChunkTime, idx)
		}
	}
}

// requestMissingChunks requests any missing chunks.
func (r *StateSyncReactor) requestMissingChunks() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateSyncDownloading || r.selectedOffer == nil {
		return
	}

	// Check if all chunks received
	allReceived := true
	for _, status := range r.chunkStatus {
		if !status {
			allReceived = false
			break
		}
	}

	if allReceived {
		r.applySnapshotLocked()
		return
	}

	// Request missing chunks
	peers := r.getPeersWithSnapshot(r.selectedOffer.Hash)
	if len(peers) == 0 {
		return
	}

	peerIdx := 0
	for idx, status := range r.chunkStatus {
		if status {
			continue
		}

		// Check if already pending
		if _, pending := r.pendingChunks[idx]; pending {
			continue
		}

		// Check retry limit
		if r.chunkRetries[idx] >= r.maxChunkRetries {
			r.failWithError(fmt.Errorf("chunk %d exceeded max retries", idx))
			return
		}

		// Request from next available peer
		peerID := peers[peerIdx%len(peers)]
		peerIdx++

		if err := r.sendChunkRequestLocked(peerID, r.selectedOffer.Hash, idx); err != nil {
			continue
		}

		r.pendingChunks[idx] = peerID
		r.lastChunkTime[idx] = time.Now()
	}
}

// getPeersWithSnapshot returns peers that have offered the given snapshot.
func (r *StateSyncReactor) getPeersWithSnapshot(hash []byte) []peer.ID {
	var peers []peer.ID
	hashHex := fmt.Sprintf("%x", hash)

	for _, offer := range r.offers {
		if fmt.Sprintf("%x", offer.Hash) == hashHex {
			peers = append(peers, offer.PeerID)
		}
	}

	return peers
}

// applySnapshotLocked applies the downloaded snapshot.
// Must be called with mutex held.
func (r *StateSyncReactor) applySnapshotLocked() {
	r.state = StateSyncApplying

	// Unlock during apply since it may be slow
	offer := r.selectedOffer
	chunks := r.chunks
	r.mu.Unlock()

	// Create snapshot metadata
	snapshot := &statestore.Snapshot{
		Version:   statestore.SnapshotVersion,
		Height:    offer.Height,
		Hash:      offer.Hash,
		Chunks:    offer.Chunks,
		AppHash:   offer.AppHash,
		CreatedAt: offer.CreatedAt,
	}

	// Create chunk provider
	provider := statestore.NewMemoryChunkProvider(chunks)

	// Apply snapshot
	err := r.snapshotStore.Import(snapshot, provider)

	r.mu.Lock()
	if err != nil {
		r.failWithErrorLocked(fmt.Errorf("applying snapshot: %w", err))
		return
	}

	r.state = StateSyncComplete
	callback := r.onComplete
	height := offer.Height
	appHash := offer.AppHash
	r.mu.Unlock()

	if callback != nil {
		callback(height, appHash)
	}

	r.mu.Lock()
}

// failWithError transitions to failed state with an error.
func (r *StateSyncReactor) failWithError(err error) {
	r.mu.Lock()
	r.failWithErrorLocked(err)
	r.mu.Unlock()
}

// failWithErrorLocked transitions to failed state with an error.
// Must be called with mutex held.
func (r *StateSyncReactor) failWithErrorLocked(err error) {
	r.state = StateSyncFailed
	callback := r.onFailed
	r.mu.Unlock()

	if callback != nil {
		callback(err)
	}

	r.mu.Lock()
}

// HandleMessage processes incoming state sync messages.
func (r *StateSyncReactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("statesync: %w: empty message", types.ErrInvalidMessage)
	}

	reader := cramberry.NewReader(data)
	typeID := reader.ReadTypeID()
	if reader.Err() != nil {
		return fmt.Errorf("statesync: %w: failed to read type ID", types.ErrInvalidMessage)
	}

	remaining := reader.Remaining()

	switch typeID {
	case TypeIDSnapshotsRequest:
		return r.handleSnapshotsRequest(peerID, remaining)
	case TypeIDSnapshotsResponse:
		return r.handleSnapshotsResponse(peerID, remaining)
	case TypeIDSnapshotChunkRequest:
		return r.handleChunkRequest(peerID, remaining)
	case TypeIDSnapshotChunkResponse:
		return r.handleChunkResponse(peerID, remaining)
	default:
		return fmt.Errorf("statesync: %w: type ID %d", types.ErrUnknownMessageType, typeID)
	}
}

// handleSnapshotsRequest responds with available snapshots.
func (r *StateSyncReactor) handleSnapshotsRequest(peerID peer.ID, data []byte) error {
	var req schema.SnapshotsRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if req.MinHeight == nil {
		return types.ErrInvalidMessage
	}

	if r.snapshotStore == nil {
		return nil
	}

	// List available snapshots
	snapshots, err := r.snapshotStore.List()
	if err != nil {
		return nil // Don't fail on internal errors
	}

	// Filter by minimum height
	var filtered []schema.SnapshotMetadata
	for _, info := range snapshots {
		if info.Height < *req.MinHeight {
			continue
		}

		createdAt := info.CreatedAt.UnixNano()
		chunks := int32(info.Chunks)

		// Load full snapshot to get app hash
		snapshot, err := r.snapshotStore.Load(info.Hash)
		if err != nil {
			continue
		}

		filtered = append(filtered, schema.SnapshotMetadata{
			Height:    &info.Height,
			Hash:      info.Hash,
			Chunks:    &chunks,
			AppHash:   snapshot.AppHash,
			CreatedAt: createdAt,
		})
	}

	resp := &schema.SnapshotsResponse{
		Snapshots: filtered,
	}

	return r.sendSnapshotsResponse(peerID, resp)
}

// handleSnapshotsResponse processes snapshot offers from peers.
func (r *StateSyncReactor) handleSnapshotsResponse(peerID peer.ID, data []byte) error {
	var resp schema.SnapshotsResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Only accept offers during discovery
	if r.state != StateSyncDiscovering {
		return nil
	}

	for _, meta := range resp.Snapshots {
		if meta.Height == nil || meta.Hash == nil || meta.Chunks == nil || meta.AppHash == nil {
			continue
		}

		hashHex := fmt.Sprintf("%x", meta.Hash)

		// Store offer
		offer := &SnapshotOffer{
			PeerID:  peerID,
			Height:  *meta.Height,
			Hash:    meta.Hash,
			Chunks:  int(*meta.Chunks),
			AppHash: meta.AppHash,
		}
		if meta.CreatedAt != 0 {
			offer.CreatedAt = time.Unix(0, meta.CreatedAt)
		}

		r.offers[hashHex] = offer
	}

	return nil
}

// handleChunkRequest responds with a snapshot chunk.
func (r *StateSyncReactor) handleChunkRequest(peerID peer.ID, data []byte) error {
	var req schema.SnapshotChunkRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if req.SnapshotHash == nil || req.ChunkIndex == nil {
		return types.ErrInvalidMessage
	}

	if r.snapshotStore == nil {
		return nil
	}

	// Load chunk
	chunk, err := r.snapshotStore.LoadChunk(req.SnapshotHash, int(*req.ChunkIndex))
	if err != nil {
		return nil // Don't fail on missing chunks
	}

	chunkIndex := *req.ChunkIndex
	resp := &schema.SnapshotChunkResponse{
		SnapshotHash: req.SnapshotHash,
		ChunkIndex:   &chunkIndex,
		Data:         chunk.Data,
		ChunkHash:    chunk.Hash,
	}

	return r.sendChunkResponse(peerID, resp)
}

// handleChunkResponse processes a received chunk.
func (r *StateSyncReactor) handleChunkResponse(peerID peer.ID, data []byte) error {
	var resp schema.SnapshotChunkResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if resp.SnapshotHash == nil || resp.ChunkIndex == nil || resp.Data == nil || resp.ChunkHash == nil {
		return types.ErrInvalidMessage
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Only accept chunks during download
	if r.state != StateSyncDownloading || r.selectedOffer == nil {
		return nil
	}

	// Verify this is for our selected snapshot
	if !bytes.Equal(resp.SnapshotHash, r.selectedOffer.Hash) {
		return nil
	}

	idx := int(*resp.ChunkIndex)
	if idx < 0 || idx >= len(r.chunks) {
		return nil
	}

	// Already have this chunk
	if r.chunkStatus[idx] {
		return nil
	}

	// Verify chunk hash
	computedHash := sha256.Sum256(resp.Data)
	if !bytes.Equal(computedHash[:], resp.ChunkHash) {
		if r.network != nil {
			_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock, "chunk hash mismatch")
		}
		return nil
	}

	// Store chunk
	r.chunks[idx] = resp.Data
	r.chunkStatus[idx] = true
	delete(r.pendingChunks, idx)
	delete(r.lastChunkTime, idx)

	return nil
}

// sendSnapshotsRequest sends a SnapshotsRequest to a peer.
func (r *StateSyncReactor) sendSnapshotsRequest(peerID peer.ID, minHeight int64) error {
	if r.network == nil {
		return nil
	}

	req := &schema.SnapshotsRequest{
		MinHeight: &minHeight,
	}

	data, err := r.encodeMessage(TypeIDSnapshotsRequest, req)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamStateSync, data)
}

// sendSnapshotsResponse sends a SnapshotsResponse to a peer.
func (r *StateSyncReactor) sendSnapshotsResponse(peerID peer.ID, resp *schema.SnapshotsResponse) error {
	if r.network == nil {
		return nil
	}

	data, err := r.encodeMessage(TypeIDSnapshotsResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamStateSync, data)
}

// sendChunkRequestLocked sends a chunk request to a peer.
// Must be called with mutex held.
func (r *StateSyncReactor) sendChunkRequestLocked(peerID peer.ID, snapshotHash []byte, chunkIndex int) error {
	if r.network == nil {
		return nil
	}

	idx := int32(chunkIndex)
	req := &schema.SnapshotChunkRequest{
		SnapshotHash: snapshotHash,
		ChunkIndex:   &idx,
	}

	data, err := r.encodeMessage(TypeIDSnapshotChunkRequest, req)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamStateSync, data)
}

// sendChunkResponse sends a chunk response to a peer.
func (r *StateSyncReactor) sendChunkResponse(peerID peer.ID, resp *schema.SnapshotChunkResponse) error {
	if r.network == nil {
		return nil
	}

	data, err := r.encodeMessage(TypeIDSnapshotChunkResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamStateSync, data)
}

// encodeMessage encodes a message with its type ID prefix.
func (r *StateSyncReactor) encodeMessage(typeID cramberry.TypeID, msg interface {
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

// OnPeerConnected is called when a new peer connects.
func (r *StateSyncReactor) OnPeerConnected(peerID peer.ID) {
	r.mu.RLock()
	state := r.state
	r.mu.RUnlock()

	// Request snapshots from new peers during discovery
	if state == StateSyncDiscovering {
		_ = r.sendSnapshotsRequest(peerID, r.trustHeight)
	}
}

// OnPeerDisconnected cleans up state for a disconnected peer.
func (r *StateSyncReactor) OnPeerDisconnected(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove pending chunk requests from this peer
	for idx, reqPeer := range r.pendingChunks {
		if reqPeer == peerID {
			delete(r.pendingChunks, idx)
			delete(r.lastChunkTime, idx)
		}
	}
}
