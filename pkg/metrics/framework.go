package metrics

import (
	"time"
)

// FrameworkMetrics defines the interface for collecting application and framework metrics.
// All methods must be thread-safe and non-blocking.
// Implementations should gracefully handle being called with invalid values.
type FrameworkMetrics interface {
	// Consensus metrics
	ConsensusHeight(height uint64)
	ConsensusRound(round uint32)
	ConsensusStep(step string)
	ConsensusBlockCommitted(duration time.Duration)
	ConsensusBlockSize(bytes int64)

	// Mempool metrics
	MempoolSize(count int, bytes int64)
	MempoolTxAdded()
	MempoolTxRemoved(reason string)
	MempoolTxRejected(reason string)

	// Application metrics
	AppExecuteBlock(duration time.Duration)
	AppCommit(duration time.Duration)
	AppQuery(duration time.Duration, path string)
	AppCheckTx(duration time.Duration, accepted bool)

	// Network metrics
	NetworkPeers(count int)
	NetworkBytesSent(stream string, bytes int)
	NetworkBytesReceived(stream string, bytes int)
	NetworkMessageSent(stream string)
	NetworkMessageReceived(stream string)
	NetworkMessageError(stream string, errorType string)

	// Storage metrics
	BlockStoreHeight(height uint64)
	BlockStoreSizeBytes(bytes int64)
	StateStoreCommit(duration time.Duration)
	StateStoreGet(duration time.Duration)
	StateStoreSet(duration time.Duration)

	// Sync metrics
	SyncProgress(current, target uint64)
	SyncBlocksReceived(count int)
	SyncDuration(duration time.Duration)
}

// NullFrameworkMetrics is a no-op implementation of FrameworkMetrics.
// Use this when metrics collection is disabled.
type NullFrameworkMetrics struct{}

func (NullFrameworkMetrics) ConsensusHeight(height uint64)                       {}
func (NullFrameworkMetrics) ConsensusRound(round uint32)                         {}
func (NullFrameworkMetrics) ConsensusStep(step string)                           {}
func (NullFrameworkMetrics) ConsensusBlockCommitted(duration time.Duration)      {}
func (NullFrameworkMetrics) ConsensusBlockSize(bytes int64)                      {}
func (NullFrameworkMetrics) MempoolSize(count int, bytes int64)                  {}
func (NullFrameworkMetrics) MempoolTxAdded()                                     {}
func (NullFrameworkMetrics) MempoolTxRemoved(reason string)                      {}
func (NullFrameworkMetrics) MempoolTxRejected(reason string)                     {}
func (NullFrameworkMetrics) AppExecuteBlock(duration time.Duration)              {}
func (NullFrameworkMetrics) AppCommit(duration time.Duration)                    {}
func (NullFrameworkMetrics) AppQuery(duration time.Duration, path string)        {}
func (NullFrameworkMetrics) AppCheckTx(duration time.Duration, accepted bool)    {}
func (NullFrameworkMetrics) NetworkPeers(count int)                              {}
func (NullFrameworkMetrics) NetworkBytesSent(stream string, bytes int)           {}
func (NullFrameworkMetrics) NetworkBytesReceived(stream string, bytes int)       {}
func (NullFrameworkMetrics) NetworkMessageSent(stream string)                    {}
func (NullFrameworkMetrics) NetworkMessageReceived(stream string)                {}
func (NullFrameworkMetrics) NetworkMessageError(stream string, errorType string) {}
func (NullFrameworkMetrics) BlockStoreHeight(height uint64)                      {}
func (NullFrameworkMetrics) BlockStoreSizeBytes(bytes int64)                     {}
func (NullFrameworkMetrics) StateStoreCommit(duration time.Duration)             {}
func (NullFrameworkMetrics) StateStoreGet(duration time.Duration)                {}
func (NullFrameworkMetrics) StateStoreSet(duration time.Duration)                {}
func (NullFrameworkMetrics) SyncProgress(current, target uint64)                 {}
func (NullFrameworkMetrics) SyncBlocksReceived(count int)                        {}
func (NullFrameworkMetrics) SyncDuration(duration time.Duration)                 {}

// Ensure NullFrameworkMetrics implements FrameworkMetrics.
var _ FrameworkMetrics = NullFrameworkMetrics{}

// FrameworkMetricsConfig contains configuration for framework metrics collection.
type FrameworkMetricsConfig struct {
	// Enabled enables metrics collection.
	Enabled bool

	// Namespace is the prefix for all metric names.
	Namespace string

	// Subsystem is an additional prefix between namespace and metric name.
	Subsystem string

	// Labels are additional labels to add to all metrics.
	Labels map[string]string

	// HistogramBuckets configures histogram bucket boundaries.
	HistogramBuckets HistogramBuckets
}

// HistogramBuckets defines bucket boundaries for histogram metrics.
type HistogramBuckets struct {
	// LatencyBuckets for operation latencies (in seconds).
	LatencyBuckets []float64

	// SizeBuckets for size measurements (in bytes).
	SizeBuckets []float64

	// DurationBuckets for longer durations (in seconds).
	DurationBuckets []float64
}

// DefaultFrameworkMetricsConfig returns sensible defaults for framework metrics configuration.
func DefaultFrameworkMetricsConfig() FrameworkMetricsConfig {
	return FrameworkMetricsConfig{
		Enabled:   true,
		Namespace: "blockberry",
		Subsystem: "",
		Labels:    nil,
		HistogramBuckets: HistogramBuckets{
			LatencyBuckets:  []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			SizeBuckets:     []float64{100, 1000, 10000, 100000, 1000000, 10000000},
			DurationBuckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60, 120, 300},
		},
	}
}

// Framework metric name constants.
const (
	// Consensus metrics
	MetricConsensusHeight         = "consensus_height"
	MetricConsensusRound          = "consensus_round"
	MetricConsensusStep           = "consensus_step"
	MetricConsensusBlockCommitted = "consensus_block_committed_seconds"
	MetricConsensusBlockSize      = "consensus_block_size_bytes"

	// Mempool metrics
	MetricMempoolSize       = "mempool_size"
	MetricMempoolBytes      = "mempool_bytes"
	MetricMempoolTxAdded    = "mempool_tx_added_total"
	MetricMempoolTxRemoved  = "mempool_tx_removed_total"
	MetricMempoolTxRejected = "mempool_tx_rejected_total"

	// Application metrics
	MetricAppExecuteBlock = "app_execute_block_seconds"
	MetricAppCommit       = "app_commit_seconds"
	MetricAppQuery        = "app_query_seconds"
	MetricAppCheckTx      = "app_check_tx_seconds"

	// Network metrics
	MetricNetworkPeers            = "network_peers"
	MetricNetworkBytesSent        = "network_bytes_sent_total"
	MetricNetworkBytesReceived    = "network_bytes_received_total"
	MetricNetworkMessagesSent     = "network_messages_sent_total"
	MetricNetworkMessagesReceived = "network_messages_received_total"
	MetricNetworkMessageErrors    = "network_message_errors_total"

	// Storage metrics
	MetricBlockStoreHeight = "blockstore_height"
	MetricBlockStoreSize   = "blockstore_size_bytes"
	MetricStateStoreCommit = "statestore_commit_seconds"
	MetricStateStoreGet    = "statestore_get_seconds"
	MetricStateStoreSet    = "statestore_set_seconds"

	// Sync metrics
	MetricSyncProgress       = "sync_progress"
	MetricSyncBlocksReceived = "sync_blocks_received_total"
	MetricSyncDuration       = "sync_duration_seconds"
)

// Framework label name constants.
const (
	LabelStream    = "stream"
	LabelPath      = "path"
	LabelReason    = "reason"
	LabelErrorType = "error_type"
	LabelSuccess   = "success"
	LabelAccepted  = "accepted"
)

// Tx removal reason labels.
const (
	TxRemovalReasonCommitted = "committed"
	TxRemovalReasonExpired   = "expired"
	TxRemovalReasonEvicted   = "evicted"
	TxRemovalReasonReplaced  = "replaced"
	TxRemovalReasonInvalid   = "invalid"
)

// Tx rejection reason labels.
const (
	TxRejectionReasonFull        = "mempool_full"
	TxRejectionReasonTooLarge    = "too_large"
	TxRejectionReasonInvalid     = "invalid"
	TxRejectionReasonDuplicate   = "duplicate"
	TxRejectionReasonLowGas      = "low_gas"
	TxRejectionReasonRateLimited = "rate_limited"
)
