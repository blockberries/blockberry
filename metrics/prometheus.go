package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics implements the Metrics interface using Prometheus.
type PrometheusMetrics struct {
	registry *prometheus.Registry

	// Peer metrics
	peersTotal         *prometheus.GaugeVec
	peerConnections    *prometheus.CounterVec
	peerDisconnections *prometheus.CounterVec

	// Block metrics
	blockHeight    prometheus.Gauge
	blocksReceived prometheus.Counter
	blocksProposed prometheus.Counter
	blockLatency   prometheus.Histogram
	blockSize      prometheus.Gauge

	// Transaction metrics
	mempoolSize  prometheus.Gauge
	mempoolBytes prometheus.Gauge
	txsReceived  prometheus.Counter
	txsProposed  prometheus.Counter
	txsRejected  *prometheus.CounterVec
	txsEvicted   *prometheus.CounterVec

	// Sync metrics
	syncState          *prometheus.GaugeVec
	syncProgress       prometheus.Gauge
	syncPeerHeight     prometheus.Gauge
	syncBlocksReceived prometheus.Counter
	syncDuration       prometheus.Histogram

	// Message metrics
	messagesReceived *prometheus.CounterVec
	messagesSent     *prometheus.CounterVec
	messageErrors    *prometheus.CounterVec
	messageSize      *prometheus.HistogramVec

	// Latency metrics
	peerLatency *prometheus.GaugeVec
	peerRTT     prometheus.Histogram

	// State store metrics
	stateStoreVersion prometheus.Gauge
	stateStoreSize    prometheus.Gauge
	stateStoreGets    prometheus.Counter
	stateStoreSets    prometheus.Counter
	stateStoreDeletes prometheus.Counter
	stateStoreLatency *prometheus.HistogramVec
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance.
func NewPrometheusMetrics(namespace string) *PrometheusMetrics {
	registry := prometheus.NewRegistry()

	m := &PrometheusMetrics{
		registry: registry,

		// Peer metrics
		peersTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peers_total",
				Help:      "Total number of connected peers",
			},
			[]string{"direction"},
		),
		peerConnections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "peer_connections_total",
				Help:      "Total number of peer connection attempts",
			},
			[]string{"result"},
		),
		peerDisconnections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "peer_disconnections_total",
				Help:      "Total number of peer disconnections",
			},
			[]string{"reason"},
		),

		// Block metrics
		blockHeight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "block_height",
				Help:      "Current block height",
			},
		),
		blocksReceived: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "blocks_received_total",
				Help:      "Total number of blocks received from peers",
			},
		),
		blocksProposed: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "blocks_proposed_total",
				Help:      "Total number of blocks proposed by this node",
			},
		),
		blockLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "block_latency_seconds",
				Help:      "Time from block proposal to receipt",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
		),
		blockSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "block_size_bytes",
				Help:      "Size of the latest block in bytes",
			},
		),

		// Transaction metrics
		mempoolSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "mempool_size",
				Help:      "Number of transactions in the mempool",
			},
		),
		mempoolBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "mempool_bytes",
				Help:      "Total size of transactions in the mempool in bytes",
			},
		),
		txsReceived: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "txs_received_total",
				Help:      "Total number of transactions received",
			},
		),
		txsProposed: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "txs_proposed_total",
				Help:      "Total number of transactions proposed in blocks",
			},
		),
		txsRejected: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "txs_rejected_total",
				Help:      "Total number of rejected transactions",
			},
			[]string{"reason"},
		),
		txsEvicted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "txs_evicted_total",
				Help:      "Total number of evicted transactions",
			},
			[]string{"reason"},
		),

		// Sync metrics
		syncState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "sync_state",
				Help:      "Current sync state (1 = active)",
			},
			[]string{"state"},
		),
		syncProgress: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "sync_progress",
				Help:      "Sync progress as a ratio (0.0 - 1.0)",
			},
		),
		syncPeerHeight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "sync_peer_height",
				Help:      "Maximum block height among connected peers",
			},
		),
		syncBlocksReceived: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sync_blocks_received_total",
				Help:      "Total number of blocks received during sync",
			},
		),
		syncDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "sync_duration_seconds",
				Help:      "Time taken for sync operations",
				Buckets:   []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
			},
		),

		// Message metrics
		messagesReceived: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_received_total",
				Help:      "Total number of messages received",
			},
			[]string{"stream"},
		),
		messagesSent: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "messages_sent_total",
				Help:      "Total number of messages sent",
			},
			[]string{"stream"},
		),
		messageErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "message_errors_total",
				Help:      "Total number of message errors",
			},
			[]string{"stream", "error_type"},
		),
		messageSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "message_size_bytes",
				Help:      "Size of messages in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 2, 15), // 100 bytes to ~3.2 MB
			},
			[]string{"stream"},
		),

		// Latency metrics
		peerLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "peer_latency_seconds",
				Help:      "Current latency to peers",
			},
			[]string{"peer_id"},
		),
		peerRTT: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "peer_rtt_seconds",
				Help:      "Round-trip time to peers",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
			},
		),

		// State store metrics
		stateStoreVersion: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "statestore_version",
				Help:      "Current state store version",
			},
		),
		stateStoreSize: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "statestore_size_bytes",
				Help:      "Size of the state store in bytes",
			},
		),
		stateStoreGets: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "statestore_gets_total",
				Help:      "Total number of state store get operations",
			},
		),
		stateStoreSets: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "statestore_sets_total",
				Help:      "Total number of state store set operations",
			},
		),
		stateStoreDeletes: prometheus.NewCounter(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "statestore_deletes_total",
				Help:      "Total number of state store delete operations",
			},
		),
		stateStoreLatency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "statestore_latency_seconds",
				Help:      "Latency of state store operations",
				Buckets:   []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"op"},
		),
	}

	// Register all metrics
	m.registerMetrics()

	return m
}

func (m *PrometheusMetrics) registerMetrics() {
	m.registry.MustRegister(
		// Peer metrics
		m.peersTotal,
		m.peerConnections,
		m.peerDisconnections,

		// Block metrics
		m.blockHeight,
		m.blocksReceived,
		m.blocksProposed,
		m.blockLatency,
		m.blockSize,

		// Transaction metrics
		m.mempoolSize,
		m.mempoolBytes,
		m.txsReceived,
		m.txsProposed,
		m.txsRejected,
		m.txsEvicted,

		// Sync metrics
		m.syncState,
		m.syncProgress,
		m.syncPeerHeight,
		m.syncBlocksReceived,
		m.syncDuration,

		// Message metrics
		m.messagesReceived,
		m.messagesSent,
		m.messageErrors,
		m.messageSize,

		// Latency metrics
		m.peerLatency,
		m.peerRTT,

		// State store metrics
		m.stateStoreVersion,
		m.stateStoreSize,
		m.stateStoreGets,
		m.stateStoreSets,
		m.stateStoreDeletes,
		m.stateStoreLatency,
	)
}

// Peer metrics implementation

func (m *PrometheusMetrics) SetPeersTotal(direction string, count int) {
	m.peersTotal.WithLabelValues(direction).Set(float64(count))
}

func (m *PrometheusMetrics) IncPeerConnections(result string) {
	m.peerConnections.WithLabelValues(result).Inc()
}

func (m *PrometheusMetrics) IncPeerDisconnections(reason string) {
	m.peerDisconnections.WithLabelValues(reason).Inc()
}

// Block metrics implementation

func (m *PrometheusMetrics) SetBlockHeight(height int64) {
	m.blockHeight.Set(float64(height))
}

func (m *PrometheusMetrics) IncBlocksReceived() {
	m.blocksReceived.Inc()
}

func (m *PrometheusMetrics) IncBlocksProposed() {
	m.blocksProposed.Inc()
}

func (m *PrometheusMetrics) ObserveBlockLatency(latency time.Duration) {
	m.blockLatency.Observe(latency.Seconds())
}

func (m *PrometheusMetrics) SetBlockSize(size int) {
	m.blockSize.Set(float64(size))
}

// Transaction metrics implementation

func (m *PrometheusMetrics) SetMempoolSize(size int) {
	m.mempoolSize.Set(float64(size))
}

func (m *PrometheusMetrics) SetMempoolBytes(bytes int64) {
	m.mempoolBytes.Set(float64(bytes))
}

func (m *PrometheusMetrics) IncTxsReceived() {
	m.txsReceived.Inc()
}

func (m *PrometheusMetrics) IncTxsProposed() {
	m.txsProposed.Inc()
}

func (m *PrometheusMetrics) IncTxsRejected(reason string) {
	m.txsRejected.WithLabelValues(reason).Inc()
}

func (m *PrometheusMetrics) IncTxsEvicted(reason string) {
	m.txsEvicted.WithLabelValues(reason).Inc()
}

// Sync metrics implementation

func (m *PrometheusMetrics) SetSyncState(state string) {
	// Reset all states
	m.syncState.WithLabelValues(SyncStateSynced).Set(0)
	m.syncState.WithLabelValues(SyncStateSyncing).Set(0)
	// Set current state
	m.syncState.WithLabelValues(state).Set(1)
}

func (m *PrometheusMetrics) SetSyncProgress(progress float64) {
	m.syncProgress.Set(progress)
}

func (m *PrometheusMetrics) SetSyncPeerHeight(height int64) {
	m.syncPeerHeight.Set(float64(height))
}

func (m *PrometheusMetrics) IncSyncBlocksReceived(count int) {
	m.syncBlocksReceived.Add(float64(count))
}

func (m *PrometheusMetrics) ObserveSyncDuration(duration time.Duration) {
	m.syncDuration.Observe(duration.Seconds())
}

// Message metrics implementation

func (m *PrometheusMetrics) IncMessagesReceived(stream string) {
	m.messagesReceived.WithLabelValues(stream).Inc()
}

func (m *PrometheusMetrics) IncMessagesSent(stream string) {
	m.messagesSent.WithLabelValues(stream).Inc()
}

func (m *PrometheusMetrics) IncMessageErrors(stream, errorType string) {
	m.messageErrors.WithLabelValues(stream, errorType).Inc()
}

func (m *PrometheusMetrics) ObserveMessageSize(stream string, size int) {
	m.messageSize.WithLabelValues(stream).Observe(float64(size))
}

// Latency metrics implementation

func (m *PrometheusMetrics) SetPeerLatency(peerID string, latency time.Duration) {
	m.peerLatency.WithLabelValues(peerID).Set(latency.Seconds())
}

func (m *PrometheusMetrics) ObservePeerRTT(latency time.Duration) {
	m.peerRTT.Observe(latency.Seconds())
}

// State store metrics implementation

func (m *PrometheusMetrics) SetStateStoreVersion(version int64) {
	m.stateStoreVersion.Set(float64(version))
}

func (m *PrometheusMetrics) SetStateStoreSize(size int64) {
	m.stateStoreSize.Set(float64(size))
}

func (m *PrometheusMetrics) IncStateStoreGets() {
	m.stateStoreGets.Inc()
}

func (m *PrometheusMetrics) IncStateStoreSets() {
	m.stateStoreSets.Inc()
}

func (m *PrometheusMetrics) IncStateStoreDeletes() {
	m.stateStoreDeletes.Inc()
}

func (m *PrometheusMetrics) ObserveStateStoreLatency(op string, latency time.Duration) {
	m.stateStoreLatency.WithLabelValues(op).Observe(latency.Seconds())
}

// Handler returns an HTTP handler for serving metrics.
func (m *PrometheusMetrics) Handler() any {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	})
}

// HTTPHandler returns a typed HTTP handler for serving metrics.
func (m *PrometheusMetrics) HTTPHandler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	})
}
