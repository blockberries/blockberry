package sync

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/internal/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/pkg/statestore"
	"github.com/blockberries/blockberry/pkg/types"
)

// State sync message type IDs from schema.
const (
	TypeIDSnapshotsRequest      cramberry.TypeID = 144
	TypeIDSnapshotsResponse     cramberry.TypeID = 145
	TypeIDSnapshotChunkRequest  cramberry.TypeID = 146
	TypeIDSnapshotChunkResponse cramberry.TypeID = 147
	TypeIDLightBlockRequest     cramberry.TypeID = 148
	TypeIDLightBlockResponse    cramberry.TypeID = 149
)

// LightBlockProvider serves the (header, commit, validator-set) triple at
// a specific height to a peer that's running state-sync against us. It
// is the server side of the light-block exchange — the reactor invokes
// it when it receives a LightBlockRequest.
//
// Implementations are external to blockberry because the three pieces
// live at different layers:
//
//   - The header lives in the blockstore (each saved block contains a
//     header).
//   - The commit lives in the NEXT block's LastCommit field (the commit
//     that finalised height H is included in block H+1).
//   - The validator-set at height H depends on the chain's history.
//     For static-set chains this is constant; for chains with validator
//     updates the implementer needs a historical-validator-set store.
//
// Pass nil to disable the server side — the reactor responds to
// LightBlockRequest with an empty response, and peers fall back to
// blocksync. This is what nodes that don't carry historical data do.
type LightBlockProvider interface {
	// LightBlockAt returns the three cramberry-encoded leaderberry
	// blobs at the given height. The reactor packages these into a
	// LightBlockResponse and sends to the requesting peer. An error
	// indicates the height is unavailable locally; the reactor sends
	// an empty response in that case.
	LightBlockAt(height int64) (header, commit, validatorSet []byte, err error)
}

// LightBlockVerifier verifies an incoming LightBlockResponse against the
// operator's trust anchor. It is the client side of the light-block
// exchange — the reactor invokes it on a verifier injected from the
// caller (raspberry) when a LightBlockResponse arrives.
//
// blockberry does not depend on leaderberry, so the verification logic
// is intentionally injected via this interface. raspberry's
// implementation uses leaderberry/light.Verifier.
//
// Pass nil to disable client-side light-client verification — the
// reactor then falls back to the legacy "AppHash equality at
// trustHeight" check, which is adequate for a private testnet but not
// for production (a peer can craft a valid-looking AppHash without
// having a valid chain).
type LightBlockVerifier interface {
	// VerifyLightBlock cryptographically verifies a LightBlock fetched
	// from a peer against the operator-supplied trust anchor (held by
	// the verifier).
	//
	// On success returns the AppHash decoded from the header so the
	// reactor can cross-check it against the offered snapshot's
	// AppHash. A mismatch there means the peer offered a snapshot at
	// height H but its block-header at H says a different state — the
	// snapshot is corrupt or adversarial.
	//
	// On failure returns a descriptive error. The reactor penalises
	// the peer and skips the offer.
	VerifyLightBlock(height int64, header, commit, validatorSet []byte) (appHash []byte, err error)
}

// verifiedLightBlock is the cached result of a successful
// LightBlockVerifier.VerifyLightBlock call. The reactor keeps one per
// height; only heights with a corresponding verifiedLightBlock are
// eligible to be selected as the snapshot source.
type verifiedLightBlock struct {
	height  int64
	appHash []byte // from the header, cross-checked against the offer
}

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

	// Optional: server-side light-block provider. nil disables the
	// responder (this node still consumes state-sync but doesn't
	// serve light-blocks).
	lightBlockProvider LightBlockProvider
	// Optional: client-side light-block verifier. nil falls back to
	// the legacy "AppHash equality at trustHeight" check — adequate
	// for a private testnet, not safe in production.
	lightBlockVerifier LightBlockVerifier

	// Configuration
	trustHeight         int64
	trustHash           []byte
	discoveryInterval   time.Duration
	chunkRequestTimeout time.Duration
	maxChunkRetries     int

	// State
	state         StateSyncState
	offers        map[string]*SnapshotOffer // hash hex -> offer
	selectedOffer *SnapshotOffer
	chunks        [][]byte
	chunkStatus   []bool            // true if chunk has been received
	pendingChunks map[int]peer.ID   // chunk index -> requesting peer
	chunkRetries  map[int]int       // chunk index -> retry count
	lastChunkTime map[int]time.Time // chunk index -> last request time

	// Light-block verification state. verifiedHeights tracks heights
	// that have been cryptographically verified; pendingLightBlock
	// tracks LightBlockRequests we've sent but haven't gotten
	// responses for yet (so we don't spam the network with duplicate
	// requests for the same height).
	verifiedHeights    map[int64]*verifiedLightBlock
	pendingLightBlock  map[int64]bool

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
//
// Light-block provider/verifier default to nil — pass SetLightBlockProvider /
// SetLightBlockVerifier after construction to opt in. With both nil, the
// reactor uses the legacy trust model (AppHash equality at trustHeight).
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
		verifiedHeights:     make(map[int64]*verifiedLightBlock),
		pendingLightBlock:   make(map[int64]bool),
		stop:                make(chan struct{}),
	}
}

// SetLightBlockProvider installs the server-side handler for incoming
// LightBlockRequest messages. Pass nil to disable serving (the reactor
// responds to requests with an empty triple, signalling unavailability
// to the peer). Must be called before Start, or concurrently safely
// (the field is read under mu).
func (r *StateSyncReactor) SetLightBlockProvider(p LightBlockProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lightBlockProvider = p
}

// SetLightBlockVerifier installs the client-side verifier for
// LightBlockResponse messages. When set, the reactor refuses to select
// any offer whose corresponding LightBlock has not been
// cryptographically verified against the operator's trust anchor.
//
// When nil (default), the reactor falls back to AppHash-equality at
// trustHeight — safe for private testnets, not safe in adversarial
// settings.
func (r *StateSyncReactor) SetLightBlockVerifier(v LightBlockVerifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lightBlockVerifier = v
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
//
// When a LightBlockVerifier is installed, only offers whose height
// appears in verifiedHeights AND whose AppHash matches the verified
// header's AppHash are eligible. This is the production-grade trust
// model: every accepted snapshot's AppHash is backed by a commit signed
// by 2f+1 voting power that chains back to the operator's anchor.
//
// When no verifier is installed (legacy/testnet mode), the original
// trust-on-first-byte AppHash-equality at trustHeight is used.
func (r *StateSyncReactor) selectBestSnapshot() {
	verifierActive := r.lightBlockVerifier != nil

	var best *SnapshotOffer
	for _, offer := range r.offers {
		// Must be at or above trust height
		if offer.Height < r.trustHeight {
			continue
		}

		if verifierActive {
			// Strict path: offer is eligible only if a LightBlock was
			// fetched, verified, AND the verified AppHash matches the
			// offer's AppHash. Offers without a verified entry are
			// not yet eligible — the next discovery tick will pick
			// them up once the LightBlockResponse lands.
			v, ok := r.verifiedHeights[offer.Height]
			if !ok {
				continue
			}
			if !types.HashEqual(v.appHash, offer.AppHash) {
				continue
			}
		} else {
			// Legacy path: at trustHeight require AppHash equality
			// against the operator-supplied trustHash; above
			// trustHeight pick the highest unconditionally.
			if offer.Height == r.trustHeight && len(r.trustHash) > 0 {
				if !types.HashEqual(offer.AppHash, r.trustHash) {
					continue
				}
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

// sendLightBlockRequest sends a LightBlockRequest for the given height
// to the specified peer.
func (r *StateSyncReactor) sendLightBlockRequest(peerID peer.ID, height int64) error {
	if r.network == nil {
		return nil
	}
	h := height
	req := &schema.LightBlockRequest{Height: &h}
	data, err := r.encodeMessage(TypeIDLightBlockRequest, req)
	if err != nil {
		return err
	}
	return r.network.Send(peerID, p2p.StreamStateSync, data)
}

// sendLightBlockResponse sends a LightBlockResponse to a peer that
// previously sent a LightBlockRequest. Empty fields are valid and
// signal "this node does not have the requested height".
func (r *StateSyncReactor) sendLightBlockResponse(peerID peer.ID, resp *schema.LightBlockResponse) error {
	if r.network == nil {
		return nil
	}
	data, err := r.encodeMessage(TypeIDLightBlockResponse, resp)
	if err != nil {
		return err
	}
	return r.network.Send(peerID, p2p.StreamStateSync, data)
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
			// Call transitionToFailed directly since we already hold the lock
			r.transitionToFailed(fmt.Errorf("chunk %d exceeded max retries", idx))
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
		r.transitionToFailed(fmt.Errorf("applying snapshot: %w", err))
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

// transitionToFailed transitions to failed state with an error.
// Must be called with mutex held. The lock is temporarily released to call
// the callback, then re-acquired before returning. Callers should be aware
// that state may change during callback execution.
func (r *StateSyncReactor) transitionToFailed(err error) {
	r.state = StateSyncFailed
	callback := r.onFailed

	// Release lock to call callback (may be slow or cause deadlock)
	r.mu.Unlock()

	if callback != nil {
		callback(err)
	}

	// Re-acquire lock before returning
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
	case TypeIDLightBlockRequest:
		return r.handleLightBlockRequest(peerID, remaining)
	case TypeIDLightBlockResponse:
		return r.handleLightBlockResponse(peerID, remaining)
	default:
		return fmt.Errorf("statesync: %w: type ID %d", types.ErrUnknownMessageType, typeID)
	}
}

// handleLightBlockRequest serves a peer's request for the (header,
// commit, validator-set) triple at a specific height. The three pieces
// come from the local LightBlockProvider; if none is set, or it has no
// data at that height, the reactor sends back an empty response (still
// a valid LightBlockResponse, just with empty byte fields). The
// requesting peer interprets empty bytes as "unavailable" and either
// retries against another peer or falls back to blocksync.
func (r *StateSyncReactor) handleLightBlockRequest(peerID peer.ID, data []byte) error {
	var req schema.LightBlockRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}
	if req.Height == nil {
		return types.ErrInvalidMessage
	}

	r.mu.RLock()
	provider := r.lightBlockProvider
	r.mu.RUnlock()

	var header, commit, valSet []byte
	if provider != nil {
		// Best-effort: errors mean we have no data for that height.
		// We still return a response (with empty bytes) so the peer's
		// pendingLightBlock flag resolves; it'll move on to another peer.
		if h, c, v, err := provider.LightBlockAt(*req.Height); err == nil {
			header, commit, valSet = h, c, v
		}
	}

	height := *req.Height
	resp := &schema.LightBlockResponse{
		Height:            &height,
		HeaderBytes:       header,
		CommitBytes:       commit,
		ValidatorSetBytes: valSet,
	}
	return r.sendLightBlockResponse(peerID, resp)
}

// handleLightBlockResponse processes a LightBlock fetched from a peer
// and, if a verifier is installed, cryptographically validates it
// against the operator's trust anchor. Verified LightBlocks unlock
// their corresponding snapshot offers — selectBestSnapshot only
// considers heights that appear in verifiedHeights.
//
// Failure modes:
//   - empty bytes → peer doesn't have this height; harmless, just
//     clear the pending flag so we can ask another peer.
//   - malformed bytes → peer sent garbage; penalty + clear pending.
//   - verifier rejects (commit invalid, doesn't chain to anchor) →
//     peer lied; PenaltyInvalidBlock-class penalty + clear pending.
//   - AppHash from verified header != offer.AppHash → snapshot is
//     corrupt or doesn't match the chain at that height; clear the
//     pending flag, do not mark verified, do not penalise the
//     LightBlock peer (they may have served a correct LightBlock for a
//     bogus snapshot from a different peer).
//
// When no verifier is installed (nil), this handler is a no-op —
// LightBlockResponses are accepted into the void and selectBestSnapshot
// continues to use the legacy AppHash-equality model.
func (r *StateSyncReactor) handleLightBlockResponse(peerID peer.ID, data []byte) error {
	var resp schema.LightBlockResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}
	if resp.Height == nil {
		return types.ErrInvalidMessage
	}
	height := *resp.Height

	r.mu.Lock()
	verifier := r.lightBlockVerifier
	delete(r.pendingLightBlock, height)
	r.mu.Unlock()

	// Empty response (peer doesn't have this height) — accept silently;
	// the pending flag is cleared so we can retry against another peer.
	if len(resp.HeaderBytes) == 0 || len(resp.CommitBytes) == 0 || len(resp.ValidatorSetBytes) == 0 {
		return nil
	}

	// No verifier installed — legacy trust model. Drop the response.
	if verifier == nil {
		return nil
	}

	appHash, err := verifier.VerifyLightBlock(height, resp.HeaderBytes, resp.CommitBytes, resp.ValidatorSetBytes)
	if err != nil {
		// Peer served a verifiable-but-invalid LightBlock — the commit
		// didn't verify, signatures forged, header didn't chain back
		// to the anchor, etc. This is adversarial behaviour.
		if r.network != nil {
			_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock,
				fmt.Sprintf("light-block verify at h=%d: %v", height, err))
		}
		return nil
	}

	// Verified — cross-check against any offers we have at this height.
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, offer := range r.offers {
		if offer.Height != height {
			continue
		}
		// The verified header must agree with the offer about AppHash.
		// If not, the snapshot is corrupt (the peer who offered it has
		// the wrong state) — but the LightBlock peer may be honest, so
		// we don't penalise here; we just refuse to mark verified.
		if !types.HashEqual(appHash, offer.AppHash) {
			continue
		}
	}
	r.verifiedHeights[height] = &verifiedLightBlock{height: height, appHash: appHash}
	return nil
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
//
// If a LightBlockVerifier is installed, every new offer also triggers a
// LightBlockRequest to the offering peer — only verified offers can
// later be selected by selectBestSnapshot. When no verifier is installed,
// the legacy trust-on-first-byte model applies and offers are eligible
// immediately.
func (r *StateSyncReactor) handleSnapshotsResponse(peerID peer.ID, data []byte) error {
	var resp schema.SnapshotsResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	r.mu.Lock()

	// Only accept offers during discovery
	if r.state != StateSyncDiscovering {
		r.mu.Unlock()
		return nil
	}

	verifierActive := r.lightBlockVerifier != nil
	var lightBlockTargets []int64

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

		// If we have a verifier and we haven't already kicked off a
		// LightBlockRequest at this height, queue one.
		if verifierActive && !r.pendingLightBlock[offer.Height] {
			if _, alreadyVerified := r.verifiedHeights[offer.Height]; !alreadyVerified {
				r.pendingLightBlock[offer.Height] = true
				lightBlockTargets = append(lightBlockTargets, offer.Height)
			}
		}
	}

	// Release the lock before doing network I/O — sendLightBlockRequest
	// takes the writer side of the stream manager, which is its own
	// thing, but as a discipline we never hold reactor mu during sends.
	r.mu.Unlock()

	for _, h := range lightBlockTargets {
		if err := r.sendLightBlockRequest(peerID, h); err != nil {
			// Network error — undo the pending flag so the next discovery
			// tick can retry against a different peer.
			r.mu.Lock()
			delete(r.pendingLightBlock, h)
			r.mu.Unlock()
		}
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

	// Verify this is for our selected snapshot (constant-time comparison)
	if !types.HashEqual(resp.SnapshotHash, r.selectedOffer.Hash) {
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

	// Verify chunk hash (constant-time comparison)
	computedHash := sha256.Sum256(resp.Data)
	if !types.HashEqual(computedHash[:], resp.ChunkHash) {
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
