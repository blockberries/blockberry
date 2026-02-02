package abi

import (
	"testing"
	"time"
)

func TestNullMetrics_Consensus(t *testing.T) {
	m := NullMetrics{}

	// All methods should be no-ops (not panic)
	m.ConsensusHeight(100)
	m.ConsensusRound(5)
	m.ConsensusStep("prevote")
	m.ConsensusBlockCommitted(500 * time.Millisecond)
	m.ConsensusBlockSize(1024 * 1024)
}

func TestNullMetrics_Mempool(t *testing.T) {
	m := NullMetrics{}

	m.MempoolSize(50, 1024*1024)
	m.MempoolTxAdded()
	m.MempoolTxRemoved(TxRemovalReasonCommitted)
	m.MempoolTxRejected(TxRejectionReasonFull)
}

func TestNullMetrics_App(t *testing.T) {
	m := NullMetrics{}

	m.AppBeginBlock(10 * time.Millisecond)
	m.AppExecuteTx(5*time.Millisecond, true)
	m.AppExecuteTx(5*time.Millisecond, false)
	m.AppEndBlock(8 * time.Millisecond)
	m.AppCommit(100 * time.Millisecond)
	m.AppQuery(2*time.Millisecond, "/store/key")
	m.AppCheckTx(1*time.Millisecond, true)
	m.AppCheckTx(1*time.Millisecond, false)
}

func TestNullMetrics_Network(t *testing.T) {
	m := NullMetrics{}

	m.NetworkPeers(10)
	m.NetworkBytesSent("pex", 1024)
	m.NetworkBytesReceived("transactions", 4096)
	m.NetworkMessageSent("blocks")
	m.NetworkMessageReceived("consensus")
	m.NetworkMessageError("pex", "decode_error")
}

func TestNullMetrics_Storage(t *testing.T) {
	m := NullMetrics{}

	m.BlockStoreHeight(50000)
	m.BlockStoreSizeBytes(100 * 1024 * 1024)
	m.StateStoreCommit(50 * time.Millisecond)
	m.StateStoreGet(100 * time.Microsecond)
	m.StateStoreSet(200 * time.Microsecond)
}

func TestNullMetrics_Sync(t *testing.T) {
	m := NullMetrics{}

	m.SyncProgress(1000, 50000)
	m.SyncBlocksReceived(100)
	m.SyncDuration(30 * time.Second)
}

func TestNullMetrics_Interface(t *testing.T) {
	// Verify NullMetrics satisfies the Metrics interface
	var _ Metrics = NullMetrics{}
}

func TestDefaultMetricsConfig(t *testing.T) {
	cfg := DefaultMetricsConfig()

	if !cfg.Enabled {
		t.Error("Enabled should be true by default")
	}
	if cfg.Namespace != "blockberry" {
		t.Errorf("Namespace = %q, want %q", cfg.Namespace, "blockberry")
	}
	if cfg.Subsystem != "" {
		t.Errorf("Subsystem = %q, want empty", cfg.Subsystem)
	}
	if cfg.Labels != nil {
		t.Errorf("Labels = %v, want nil", cfg.Labels)
	}
}

func TestDefaultMetricsConfig_HistogramBuckets(t *testing.T) {
	cfg := DefaultMetricsConfig()

	// Verify latency buckets
	if len(cfg.HistogramBuckets.LatencyBuckets) == 0 {
		t.Error("LatencyBuckets should not be empty")
	}

	// Verify size buckets
	if len(cfg.HistogramBuckets.SizeBuckets) == 0 {
		t.Error("SizeBuckets should not be empty")
	}

	// Verify duration buckets
	if len(cfg.HistogramBuckets.DurationBuckets) == 0 {
		t.Error("DurationBuckets should not be empty")
	}

	// Verify buckets are in ascending order
	for i := 1; i < len(cfg.HistogramBuckets.LatencyBuckets); i++ {
		if cfg.HistogramBuckets.LatencyBuckets[i] <= cfg.HistogramBuckets.LatencyBuckets[i-1] {
			t.Error("LatencyBuckets should be in ascending order")
		}
	}
}

func TestMetricNameConstants(t *testing.T) {
	// Verify metric names follow naming conventions
	tests := []struct {
		name     string
		expected string
	}{
		{MetricConsensusHeight, "consensus_height"},
		{MetricConsensusRound, "consensus_round"},
		{MetricMempoolSize, "mempool_size"},
		{MetricMempoolBytes, "mempool_bytes"},
		{MetricAppBeginBlock, "app_begin_block_seconds"},
		{MetricAppExecuteTx, "app_execute_tx_seconds"},
		{MetricNetworkPeers, "network_peers"},
		{MetricBlockStoreHeight, "blockstore_height"},
		{MetricSyncProgress, "sync_progress"},
	}

	for _, tt := range tests {
		if tt.name != tt.expected {
			t.Errorf("Metric constant = %q, want %q", tt.name, tt.expected)
		}
	}
}

func TestLabelConstants(t *testing.T) {
	// Verify label names
	tests := []struct {
		name     string
		expected string
	}{
		{LabelStream, "stream"},
		{LabelPath, "path"},
		{LabelReason, "reason"},
		{LabelErrorType, "error_type"},
		{LabelSuccess, "success"},
		{LabelAccepted, "accepted"},
	}

	for _, tt := range tests {
		if tt.name != tt.expected {
			t.Errorf("Label constant = %q, want %q", tt.name, tt.expected)
		}
	}
}

func TestTxRemovalReasons(t *testing.T) {
	reasons := []string{
		TxRemovalReasonCommitted,
		TxRemovalReasonExpired,
		TxRemovalReasonEvicted,
		TxRemovalReasonReplaced,
		TxRemovalReasonInvalid,
	}

	// Verify all reasons are unique
	seen := make(map[string]bool)
	for _, r := range reasons {
		if seen[r] {
			t.Errorf("Duplicate removal reason: %q", r)
		}
		seen[r] = true
	}
}

func TestTxRejectionReasons(t *testing.T) {
	reasons := []string{
		TxRejectionReasonFull,
		TxRejectionReasonTooLarge,
		TxRejectionReasonInvalid,
		TxRejectionReasonDuplicate,
		TxRejectionReasonLowGas,
		TxRejectionReasonRateLimited,
	}

	// Verify all reasons are unique
	seen := make(map[string]bool)
	for _, r := range reasons {
		if seen[r] {
			t.Errorf("Duplicate rejection reason: %q", r)
		}
		seen[r] = true
	}
}
