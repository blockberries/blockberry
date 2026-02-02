package metrics

import (
	"time"

	"github.com/blockberries/blockberry/pkg/abi"
)

// ABIMetricsAdapter wraps PrometheusMetrics to implement the abi.Metrics interface.
// This adapter bridges the existing Prometheus metrics implementation with the
// new ABI-defined interface.
type ABIMetricsAdapter struct {
	prom *PrometheusMetrics
}

// NewABIMetricsAdapter creates a new adapter that implements abi.Metrics.
func NewABIMetricsAdapter(namespace string) *ABIMetricsAdapter {
	return &ABIMetricsAdapter{
		prom: NewPrometheusMetrics(namespace),
	}
}

// NewABIMetricsAdapterFrom wraps an existing PrometheusMetrics instance.
func NewABIMetricsAdapterFrom(prom *PrometheusMetrics) *ABIMetricsAdapter {
	return &ABIMetricsAdapter{prom: prom}
}

// Consensus metrics

func (a *ABIMetricsAdapter) ConsensusHeight(height uint64) {
	a.prom.SetBlockHeight(int64(height))
}

func (a *ABIMetricsAdapter) ConsensusRound(round uint32) {
	// Round is tracked internally but not directly exposed in current Prometheus metrics
	// This is a no-op for compatibility
}

func (a *ABIMetricsAdapter) ConsensusStep(step string) {
	// Step is tracked internally but not directly exposed in current Prometheus metrics
	// This is a no-op for compatibility
}

func (a *ABIMetricsAdapter) ConsensusBlockCommitted(duration time.Duration) {
	a.prom.ObserveBlockLatency(duration)
}

func (a *ABIMetricsAdapter) ConsensusBlockSize(bytes int64) {
	a.prom.SetBlockSize(int(bytes))
}

// Mempool metrics

func (a *ABIMetricsAdapter) MempoolSize(count int, bytes int64) {
	a.prom.SetMempoolSize(count)
	a.prom.SetMempoolBytes(bytes)
}

func (a *ABIMetricsAdapter) MempoolTxAdded() {
	a.prom.IncTxsReceived()
}

func (a *ABIMetricsAdapter) MempoolTxRemoved(reason string) {
	a.prom.IncTxsEvicted(reason)
}

func (a *ABIMetricsAdapter) MempoolTxRejected(reason string) {
	a.prom.IncTxsRejected(reason)
}

// Application metrics

func (a *ABIMetricsAdapter) AppBeginBlock(duration time.Duration) {
	// App timing is tracked per-operation; no direct mapping in current metrics
	// This could be added as a histogram in a future enhancement
}

func (a *ABIMetricsAdapter) AppExecuteTx(duration time.Duration, success bool) {
	// App timing is tracked per-operation; no direct mapping in current metrics
	// This could be added as a histogram in a future enhancement
	if success {
		a.prom.IncTxsProposed()
	}
}

func (a *ABIMetricsAdapter) AppEndBlock(duration time.Duration) {
	// App timing is tracked per-operation; no direct mapping in current metrics
}

func (a *ABIMetricsAdapter) AppCommit(duration time.Duration) {
	// App commit timing could be mapped to state store commit
	a.prom.ObserveStateStoreLatency(OpCommit, duration)
}

func (a *ABIMetricsAdapter) AppQuery(duration time.Duration, path string) {
	// Query latency; no direct mapping but could use state store latency
}

func (a *ABIMetricsAdapter) AppCheckTx(duration time.Duration, accepted bool) {
	// CheckTx timing; no direct mapping in current metrics
}

// Network metrics

func (a *ABIMetricsAdapter) NetworkPeers(count int) {
	// Set total peers (direction-agnostic view)
	a.prom.SetPeersTotal("total", count)
}

func (a *ABIMetricsAdapter) NetworkBytesSent(stream string, bytes int) {
	a.prom.ObserveMessageSize(stream, bytes)
	a.prom.IncMessagesSent(stream)
}

func (a *ABIMetricsAdapter) NetworkBytesReceived(stream string, bytes int) {
	a.prom.ObserveMessageSize(stream, bytes)
	a.prom.IncMessagesReceived(stream)
}

func (a *ABIMetricsAdapter) NetworkMessageSent(stream string) {
	a.prom.IncMessagesSent(stream)
}

func (a *ABIMetricsAdapter) NetworkMessageReceived(stream string) {
	a.prom.IncMessagesReceived(stream)
}

func (a *ABIMetricsAdapter) NetworkMessageError(stream string, errorType string) {
	a.prom.IncMessageErrors(stream, errorType)
}

// Storage metrics

func (a *ABIMetricsAdapter) BlockStoreHeight(height uint64) {
	a.prom.SetBlockHeight(int64(height))
}

func (a *ABIMetricsAdapter) BlockStoreSizeBytes(bytes int64) {
	// Block store size is tracked as state store size
	a.prom.SetStateStoreSize(bytes)
}

func (a *ABIMetricsAdapter) StateStoreCommit(duration time.Duration) {
	a.prom.ObserveStateStoreLatency(OpCommit, duration)
}

func (a *ABIMetricsAdapter) StateStoreGet(duration time.Duration) {
	a.prom.IncStateStoreGets()
	a.prom.ObserveStateStoreLatency(OpGet, duration)
}

func (a *ABIMetricsAdapter) StateStoreSet(duration time.Duration) {
	a.prom.IncStateStoreSets()
	a.prom.ObserveStateStoreLatency(OpSet, duration)
}

// Sync metrics

func (a *ABIMetricsAdapter) SyncProgress(current, target uint64) {
	if target > 0 {
		progress := float64(current) / float64(target)
		a.prom.SetSyncProgress(progress)
	}
	a.prom.SetSyncPeerHeight(int64(target))

	if current < target {
		a.prom.SetSyncState(SyncStateSyncing)
	} else {
		a.prom.SetSyncState(SyncStateSynced)
	}
}

func (a *ABIMetricsAdapter) SyncBlocksReceived(count int) {
	a.prom.IncSyncBlocksReceived(count)
}

func (a *ABIMetricsAdapter) SyncDuration(duration time.Duration) {
	a.prom.ObserveSyncDuration(duration)
}

// Handler returns the underlying Prometheus HTTP handler.
func (a *ABIMetricsAdapter) Handler() any {
	return a.prom.Handler()
}

// Prometheus returns the underlying PrometheusMetrics for direct access.
func (a *ABIMetricsAdapter) Prometheus() *PrometheusMetrics {
	return a.prom
}

// Ensure ABIMetricsAdapter implements abi.Metrics.
var _ abi.Metrics = (*ABIMetricsAdapter)(nil)
