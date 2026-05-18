package node

import (
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/internal/p2p"
	bbsync "github.com/blockberries/blockberry/internal/sync"
	"github.com/blockberries/blockberry/pkg/statestore"
)

// StateSyncOptions configures a StateSyncRunner. Zero-valued fields are
// replaced with defaults documented per-field.
type StateSyncOptions struct {
	// TrustHeight is the height of the operator-supplied trust anchor.
	// The runner accepts snapshots at this height only when the offer's
	// AppHash matches TrustHash. Required.
	TrustHeight int64
	// TrustHash is the AppHash the runner expects at TrustHeight.
	// Mismatched offers are skipped. Required.
	TrustHash []byte
	// DiscoveryInterval is how often the runner broadcasts a
	// "what snapshots do you have?" probe to peers while in the
	// discovery phase. Default 5s.
	DiscoveryInterval time.Duration
	// ChunkRequestTimeout bounds how long an outstanding chunk
	// request waits before being retried against a different peer.
	// Default 10s.
	ChunkRequestTimeout time.Duration
	// MaxChunkRetries bounds how many times a single chunk index may
	// be re-requested before the whole state-sync attempt fails with
	// onFailed. Default 3.
	MaxChunkRetries int
}

// withDefaults returns the options with zero-valued fields replaced by
// the documented defaults.
func (o StateSyncOptions) withDefaults() StateSyncOptions {
	if o.DiscoveryInterval <= 0 {
		o.DiscoveryInterval = 5 * time.Second
	}
	if o.ChunkRequestTimeout <= 0 {
		o.ChunkRequestTimeout = 10 * time.Second
	}
	if o.MaxChunkRetries <= 0 {
		o.MaxChunkRetries = 3
	}
	return o
}

// StateSyncRunner is the public-facing handle to blockberry's
// state-sync reactor. It wraps the internal implementation so external
// consumers (raspberry's state-sync bootstrap orchestrator, in
// particular) can drive the reactor without taking a dependency on
// blockberry's internal packages.
//
// Lifecycle: build via Node.NewStateSyncRunner, attach callbacks with
// SetOnComplete / SetOnFailed, register HandleMessage as the stream
// handler for the "statesync" encrypted stream, then call Start. The
// runner reports back through whichever callback fires first and
// guarantees at most one of {onComplete, onFailed} per run.
type StateSyncRunner struct {
	inner *bbsync.StateSyncReactor
}

// Start begins discovery and the chunk-fetch loop. Returns immediately;
// completion is reported asynchronously via the onComplete / onFailed
// callbacks set before Start.
func (r *StateSyncRunner) Start() error {
	return r.inner.Start()
}

// Stop halts the runner cleanly. Safe to call after a callback has
// fired or even before Start (idempotent).
func (r *StateSyncRunner) Stop() error {
	return r.inner.Stop()
}

// IsRunning reports whether Start has been called and Stop has not.
func (r *StateSyncRunner) IsRunning() bool {
	return r.inner.IsRunning()
}

// Progress returns the percentage of chunks received (0–100). 0 before
// download begins; 100 just before onComplete fires.
func (r *StateSyncRunner) Progress() int {
	return r.inner.Progress()
}

// SetOnComplete registers the success callback. The runner guarantees
// the callback is invoked at most once. Calling SetOnComplete after a
// callback has fired is allowed but has no effect.
func (r *StateSyncRunner) SetOnComplete(fn func(height int64, appHash []byte)) {
	r.inner.SetOnComplete(fn)
}

// SetOnFailed registers the failure callback. Same at-most-once
// guarantee as SetOnComplete.
func (r *StateSyncRunner) SetOnFailed(fn func(err error)) {
	r.inner.SetOnFailed(fn)
}

// HandleMessage routes an incoming "statesync" stream message into the
// runner. Intended to be registered as a custom stream handler via
// Node.RegisterStreamHandler — the network layer's message dispatcher
// will call this for every incoming statesync message from any peer.
func (r *StateSyncRunner) HandleMessage(peerID peer.ID, data []byte) error {
	return r.inner.HandleMessage(peerID, data)
}

// OnPeerConnected notifies the runner that a peer has completed
// handshake and is ready to receive snapshot probes. Optional — the
// runner's periodic discovery would eventually reach new peers anyway
// — but calling this on every handshake speeds up startup discovery.
func (r *StateSyncRunner) OnPeerConnected(peerID peer.ID) {
	r.inner.OnPeerConnected(peerID)
}

// OnPeerDisconnected notifies the runner that a peer has dropped so it
// can release any pending chunk requests targeting that peer (those
// chunks are immediately eligible for retry from a different peer).
func (r *StateSyncRunner) OnPeerDisconnected(peerID peer.ID) {
	r.inner.OnPeerDisconnected(peerID)
}

// LightBlockProvider is the server-side surface for the light-block
// exchange: given a height, return the cramberry-encoded leaderberry
// (header, commit, validator-set) triple. Re-exported from the
// internal reactor package for use by external consumers (raspberry's
// state-sync bootstrap, in particular).
//
// Implementations are necessarily external because the three pieces
// live at different layers: header in blockstore, commit in
// blockstore (in the NEXT block's LastCommit), validator-set in
// application-level historical storage.
type LightBlockProvider = bbsync.LightBlockProvider

// LightBlockVerifier is the client-side surface for the light-block
// exchange: given the three cramberry-encoded blobs at a height,
// cryptographically verify they chain back to the operator's trust
// anchor and return the AppHash from the header.
//
// Re-exported from the internal reactor package. raspberry implements
// this using leaderberry/light.Verifier.
type LightBlockVerifier = bbsync.LightBlockVerifier

// SetLightBlockProvider installs the server-side handler for incoming
// LightBlockRequest messages. Pass nil to disable serving. Must be
// called before Start, or concurrently safely (the field is read under
// the reactor's mu).
//
// Without a provider, this node still consumes state-sync but does not
// serve light-blocks to peers — they'll get empty responses and
// retry against another peer.
func (r *StateSyncRunner) SetLightBlockProvider(p LightBlockProvider) {
	r.inner.SetLightBlockProvider(p)
}

// SetLightBlockVerifier installs the client-side verifier for
// LightBlockResponse messages. When set, the runner refuses to select
// any snapshot offer whose corresponding LightBlock has not been
// cryptographically verified against the operator's trust anchor.
//
// When nil (default), the runner falls back to AppHash-equality at
// trustHeight — safe for a private testnet but not against an adversary
// who can construct a header with a matching AppHash from a different
// chain.
func (r *StateSyncRunner) SetLightBlockVerifier(v LightBlockVerifier) {
	r.inner.SetLightBlockVerifier(v)
}

// NewStateSyncRunner constructs a state-sync runner for this node. The
// runner is wired into this Node's network and peer manager so it
// receives connectivity events automatically — callers do not need to
// forward peer-connected/disconnected events themselves.
//
// snapshotStore is the on-disk snapshot repository the runner imports
// fetched chunks into. For raspberry's PunnetApplication this is the
// FileSnapshotStore returned by PunnetApplication.SnapshotStore().
//
// The returned runner has no callbacks set. Call SetOnComplete /
// SetOnFailed before Start.
//
// Note: the runner's HandleMessage must be registered with the network's
// message dispatch by the caller, via Node.RegisterStreamHandler with
// the StreamStateSync stream name. This is decoupled from the
// constructor so callers can choose whether (and when) to expose the
// receive path — for example, a node that only RUNS state-sync
// (consumer side) and never SERVES it can skip registration.
func (n *Node) NewStateSyncRunner(snapshotStore statestore.SnapshotStore, opts StateSyncOptions) *StateSyncRunner {
	o := opts.withDefaults()
	inner := bbsync.NewStateSyncReactor(
		n.network,
		n.network.PeerManager(),
		snapshotStore,
		o.TrustHeight,
		o.TrustHash,
		o.DiscoveryInterval,
		o.ChunkRequestTimeout,
		o.MaxChunkRetries,
	)
	return &StateSyncRunner{inner: inner}
}

// StateSyncStreamName is the encrypted stream name the runner
// communicates over. Exported so external callers (raspberry) can use
// the symbol when registering their stream handler without re-deriving
// the string.
const StateSyncStreamName = p2p.StreamStateSync
