package metrics

import (
	"testing"
	"time"
)

func TestNewFrameworkMetricsAdapter(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")
	if adapter == nil {
		t.Fatal("NewFrameworkMetricsAdapter returned nil")
	}
	if adapter.prom == nil {
		t.Fatal("adapter.prom is nil")
	}
}

func TestNewFrameworkMetricsAdapterFrom(t *testing.T) {
	prom := NewPrometheusMetrics("test")
	adapter := NewFrameworkMetricsAdapterFrom(prom)
	if adapter == nil {
		t.Fatal("NewFrameworkMetricsAdapterFrom returned nil")
	}
	if adapter.prom != prom {
		t.Error("adapter.prom should be the provided instance")
	}
}

func TestFrameworkMetricsAdapter_Consensus(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	// These should not panic
	adapter.ConsensusHeight(100)
	adapter.ConsensusRound(5)
	adapter.ConsensusStep("prevote")
	adapter.ConsensusBlockCommitted(500 * time.Millisecond)
	adapter.ConsensusBlockSize(1024 * 1024)
}

func TestFrameworkMetricsAdapter_Mempool(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	adapter.MempoolSize(50, 1024*1024)
	adapter.MempoolTxAdded()
	adapter.MempoolTxRemoved("committed")
	adapter.MempoolTxRejected("mempool_full")
}

func TestFrameworkMetricsAdapter_App(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	adapter.AppExecuteBlock(50 * time.Millisecond)
	adapter.AppCommit(100 * time.Millisecond)
	adapter.AppQuery(2*time.Millisecond, "/store/key")
	adapter.AppCheckTx(1*time.Millisecond, true)
	adapter.AppCheckTx(1*time.Millisecond, false)
}

func TestFrameworkMetricsAdapter_Network(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	adapter.NetworkPeers(10)
	adapter.NetworkBytesSent("pex", 1024)
	adapter.NetworkBytesReceived("transactions", 4096)
	adapter.NetworkMessageSent("blocks")
	adapter.NetworkMessageReceived("consensus")
	adapter.NetworkMessageError("pex", "decode_error")
}

func TestFrameworkMetricsAdapter_Storage(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	adapter.BlockStoreHeight(50000)
	adapter.BlockStoreSizeBytes(100 * 1024 * 1024)
	adapter.StateStoreCommit(50 * time.Millisecond)
	adapter.StateStoreGet(100 * time.Microsecond)
	adapter.StateStoreSet(200 * time.Microsecond)
}

func TestFrameworkMetricsAdapter_Sync(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	// Test syncing state
	adapter.SyncProgress(1000, 50000)
	adapter.SyncBlocksReceived(100)
	adapter.SyncDuration(30 * time.Second)

	// Test synced state
	adapter.SyncProgress(50000, 50000)
}

func TestFrameworkMetricsAdapter_SyncZeroTarget(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	// Should not panic with zero target
	adapter.SyncProgress(0, 0)
}

func TestFrameworkMetricsAdapter_Handler(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	handler := adapter.Handler()
	if handler == nil {
		t.Error("Handler() returned nil")
	}
}

func TestFrameworkMetricsAdapter_Prometheus(t *testing.T) {
	adapter := NewFrameworkMetricsAdapter("test")

	prom := adapter.Prometheus()
	if prom == nil {
		t.Error("Prometheus() returned nil")
	}
	if prom != adapter.prom {
		t.Error("Prometheus() should return the internal instance")
	}
}

func TestFrameworkMetricsAdapter_Interface(t *testing.T) {
	var _ FrameworkMetrics = (*FrameworkMetricsAdapter)(nil)
}

func TestNullFrameworkMetrics_Interface(t *testing.T) {
	var _ FrameworkMetrics = NullFrameworkMetrics{}
}
