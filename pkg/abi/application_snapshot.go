package abi

// SnapshotApplication extends Application with state sync capabilities.
// Applications implementing this interface can participate in state sync,
// allowing new nodes to bootstrap quickly without replaying all blocks.
//
// State sync works by:
// 1. A new node requests available snapshots from peers
// 2. The node selects a snapshot and requests chunks
// 3. Each chunk is applied to rebuild the application state
// 4. Once complete, the node can participate in consensus
type SnapshotApplication interface {
	Application

	// ListSnapshots returns a list of available snapshots.
	// Called by peers requesting snapshots for state sync.
	ListSnapshots() []*Snapshot

	// LoadSnapshotChunk loads a specific chunk from a snapshot.
	// Returns the chunk data or an error if the chunk is not available.
	LoadSnapshotChunk(height int64, format uint32, chunk uint32) ([]byte, error)

	// OfferSnapshot is called when a peer offers a snapshot for syncing.
	// The application should validate and decide whether to accept it.
	OfferSnapshot(snapshot *Snapshot) OfferResult

	// ApplySnapshotChunk applies a chunk to the application state.
	// Chunks must be applied in order (0, 1, 2, ...).
	// Returns the result indicating success, retry, or failure.
	ApplySnapshotChunk(chunk []byte, index uint32) ApplyResult
}

// Snapshot represents a point-in-time application state snapshot.
type Snapshot struct {
	// Height is the block height at which the snapshot was taken.
	Height int64

	// Format is the snapshot format version.
	// Applications can use different formats for different serialization schemes.
	Format uint32

	// Chunks is the number of chunks in this snapshot.
	// Chunks are numbered 0 to Chunks-1.
	Chunks uint32

	// Hash is the snapshot hash for verification.
	Hash []byte

	// Metadata contains application-specific snapshot metadata.
	Metadata []byte
}

// OfferResult indicates the application's response to a snapshot offer.
type OfferResult uint32

const (
	// OfferAccept indicates the snapshot is accepted and sync should begin.
	OfferAccept OfferResult = iota

	// OfferAbort indicates state sync should be aborted entirely.
	// This may be due to a critical error.
	OfferAbort

	// OfferReject indicates the snapshot is rejected.
	// The node should try other snapshots.
	OfferReject

	// OfferRejectFormat indicates the snapshot format is not supported.
	// The node should try snapshots with a different format.
	OfferRejectFormat

	// OfferRejectSender indicates the sender is not trusted.
	// The node should try snapshots from other senders.
	OfferRejectSender
)

// String returns a human-readable description of the offer result.
func (r OfferResult) String() string {
	switch r {
	case OfferAccept:
		return "Accept"
	case OfferAbort:
		return "Abort"
	case OfferReject:
		return "Reject"
	case OfferRejectFormat:
		return "RejectFormat"
	case OfferRejectSender:
		return "RejectSender"
	default:
		return "Unknown"
	}
}

// ApplyResult indicates the result of applying a snapshot chunk.
type ApplyResult uint32

const (
	// ApplyAccept indicates the chunk was applied successfully.
	// Continue with the next chunk.
	ApplyAccept ApplyResult = iota

	// ApplyAbort indicates state sync should be aborted entirely.
	// This may be due to a critical error.
	ApplyAbort

	// ApplyRetry indicates the chunk should be retried.
	// This may be due to a temporary error.
	ApplyRetry

	// ApplyRetrySnapshot indicates the entire snapshot should be retried.
	// This may be due to chunk corruption detected after partial application.
	ApplyRetrySnapshot

	// ApplyRejectSnapshot indicates the snapshot should be rejected.
	// The node should try a different snapshot.
	ApplyRejectSnapshot
)

// String returns a human-readable description of the apply result.
func (r ApplyResult) String() string {
	switch r {
	case ApplyAccept:
		return "Accept"
	case ApplyAbort:
		return "Abort"
	case ApplyRetry:
		return "Retry"
	case ApplyRetrySnapshot:
		return "RetrySnapshot"
	case ApplyRejectSnapshot:
		return "RejectSnapshot"
	default:
		return "Unknown"
	}
}

// BaseSnapshotApplication provides default implementations for SnapshotApplication.
// It returns empty/rejecting responses, following the fail-closed principle.
// Applications should embed this and override methods as needed.
type BaseSnapshotApplication struct {
	BaseApplication
}

// ListSnapshots returns no snapshots by default.
func (app *BaseSnapshotApplication) ListSnapshots() []*Snapshot {
	return nil
}

// LoadSnapshotChunk returns an error by default.
func (app *BaseSnapshotApplication) LoadSnapshotChunk(height int64, format uint32, chunk uint32) ([]byte, error) {
	return nil, ErrNotImplemented
}

// OfferSnapshot rejects all snapshots by default.
func (app *BaseSnapshotApplication) OfferSnapshot(snapshot *Snapshot) OfferResult {
	return OfferReject
}

// ApplySnapshotChunk rejects all chunks by default.
func (app *BaseSnapshotApplication) ApplySnapshotChunk(chunk []byte, index uint32) ApplyResult {
	return ApplyRejectSnapshot
}

// Ensure BaseSnapshotApplication implements SnapshotApplication.
var _ SnapshotApplication = (*BaseSnapshotApplication)(nil)
