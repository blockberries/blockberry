package metrics

import (
	"time"
)

// FrameworkMetricsAdapter wraps PrometheusMetrics to implement the FrameworkMetrics interface.
// This adapter bridges the Prometheus metrics implementation with the framework-level interface.
type FrameworkMetricsAdapter struct {
	prom *PrometheusMetrics
}

// NewFrameworkMetricsAdapter creates a new adapter that implements FrameworkMetrics.
func NewFrameworkMetricsAdapter(namespace string) *FrameworkMetricsAdapter {
	return &FrameworkMetricsAdapter{
		prom: NewPrometheusMetrics(namespace),
	}
}

// NewFrameworkMetricsAdapterFrom wraps an existing PrometheusMetrics instance.
func NewFrameworkMetricsAdapterFrom(prom *PrometheusMetrics) *FrameworkMetricsAdapter {
	return &FrameworkMetricsAdapter{prom: prom}
}

// Consensus metrics

func (a *FrameworkMetricsAdapter) ConsensusHeight(height uint64) {
	a.prom.SetBlockHeight(int64(height))
}

func (a *FrameworkMetricsAdapter) ConsensusRound(round uint32) {
	// Round is tracked internally but not directly exposed in current Prometheus metrics
}

func (a *FrameworkMetricsAdapter) ConsensusStep(step string) {
	// Step is tracked internally but not directly exposed in current Prometheus metrics
}

func (a *FrameworkMetricsAdapter) ConsensusBlockCommitted(duration time.Duration) {
	a.prom.ObserveBlockLatency(duration)
}

func (a *FrameworkMetricsAdapter) ConsensusBlockSize(bytes int64) {
	a.prom.SetBlockSize(int(bytes))
}

// Mempool metrics

func (a *FrameworkMetricsAdapter) MempoolSize(count int, bytes int64) {
	a.prom.SetMempoolSize(count)
	a.prom.SetMempoolBytes(bytes)
}

func (a *FrameworkMetricsAdapter) MempoolTxAdded() {
	a.prom.IncTxsReceived()
}

func (a *FrameworkMetricsAdapter) MempoolTxRemoved(reason string) {
	a.prom.IncTxsEvicted(reason)
}

func (a *FrameworkMetricsAdapter) MempoolTxRejected(reason string) {
	a.prom.IncTxsRejected(reason)
}

// Application metrics

func (a *FrameworkMetricsAdapter) AppExecuteBlock(duration time.Duration) {
	a.prom.ObserveBlockLatency(duration)
}

func (a *FrameworkMetricsAdapter) AppCommit(duration time.Duration) {
	a.prom.ObserveStateStoreLatency(OpCommit, duration)
}

func (a *FrameworkMetricsAdapter) AppQuery(duration time.Duration, path string) {
	// Query latency; no direct mapping but could use state store latency
}

func (a *FrameworkMetricsAdapter) AppCheckTx(duration time.Duration, accepted bool) {
	// CheckTx timing; no direct mapping in current metrics
}

// Network metrics

func (a *FrameworkMetricsAdapter) NetworkPeers(count int) {
	a.prom.SetPeersTotal("total", count)
}

func (a *FrameworkMetricsAdapter) NetworkBytesSent(stream string, bytes int) {
	a.prom.ObserveMessageSize(stream, bytes)
	a.prom.IncMessagesSent(stream)
}

func (a *FrameworkMetricsAdapter) NetworkBytesReceived(stream string, bytes int) {
	a.prom.ObserveMessageSize(stream, bytes)
	a.prom.IncMessagesReceived(stream)
}

func (a *FrameworkMetricsAdapter) NetworkMessageSent(stream string) {
	a.prom.IncMessagesSent(stream)
}

func (a *FrameworkMetricsAdapter) NetworkMessageReceived(stream string) {
	a.prom.IncMessagesReceived(stream)
}

func (a *FrameworkMetricsAdapter) NetworkMessageError(stream string, errorType string) {
	a.prom.IncMessageErrors(stream, errorType)
}

// Storage metrics

func (a *FrameworkMetricsAdapter) BlockStoreHeight(height uint64) {
	a.prom.SetBlockHeight(int64(height))
}

func (a *FrameworkMetricsAdapter) BlockStoreSizeBytes(bytes int64) {
	a.prom.SetStateStoreSize(bytes)
}

func (a *FrameworkMetricsAdapter) StateStoreCommit(duration time.Duration) {
	a.prom.ObserveStateStoreLatency(OpCommit, duration)
}

func (a *FrameworkMetricsAdapter) StateStoreGet(duration time.Duration) {
	a.prom.IncStateStoreGets()
	a.prom.ObserveStateStoreLatency(OpGet, duration)
}

func (a *FrameworkMetricsAdapter) StateStoreSet(duration time.Duration) {
	a.prom.IncStateStoreSets()
	a.prom.ObserveStateStoreLatency(OpSet, duration)
}

// Sync metrics

func (a *FrameworkMetricsAdapter) SyncProgress(current, target uint64) {
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

func (a *FrameworkMetricsAdapter) SyncBlocksReceived(count int) {
	a.prom.IncSyncBlocksReceived(count)
}

func (a *FrameworkMetricsAdapter) SyncDuration(duration time.Duration) {
	a.prom.ObserveSyncDuration(duration)
}

// Handler returns the underlying Prometheus HTTP handler.
func (a *FrameworkMetricsAdapter) Handler() any {
	return a.prom.Handler()
}

// Prometheus returns the underlying PrometheusMetrics for direct access.
func (a *FrameworkMetricsAdapter) Prometheus() *PrometheusMetrics {
	return a.prom
}

// Ensure FrameworkMetricsAdapter implements FrameworkMetrics.
var _ FrameworkMetrics = (*FrameworkMetricsAdapter)(nil)
