package metrics

import (
	"testing"
	"time"

	"github.com/blockberries/blockberry/pkg/abi"
)

func TestNewABIMetricsAdapter(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")
	if adapter == nil {
		t.Fatal("NewABIMetricsAdapter returned nil")
	}
	if adapter.prom == nil {
		t.Fatal("adapter.prom is nil")
	}
}

func TestNewABIMetricsAdapterFrom(t *testing.T) {
	prom := NewPrometheusMetrics("test")
	adapter := NewABIMetricsAdapterFrom(prom)
	if adapter == nil {
		t.Fatal("NewABIMetricsAdapterFrom returned nil")
	}
	if adapter.prom != prom {
		t.Error("adapter.prom should be the provided instance")
	}
}

func TestABIMetricsAdapter_Consensus(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	// These should not panic
	adapter.ConsensusHeight(100)
	adapter.ConsensusRound(5)
	adapter.ConsensusStep("prevote")
	adapter.ConsensusBlockCommitted(500 * time.Millisecond)
	adapter.ConsensusBlockSize(1024 * 1024)
}

func TestABIMetricsAdapter_Mempool(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	adapter.MempoolSize(50, 1024*1024)
	adapter.MempoolTxAdded()
	adapter.MempoolTxRemoved("committed")
	adapter.MempoolTxRejected("mempool_full")
}

func TestABIMetricsAdapter_App(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	adapter.AppBeginBlock(10 * time.Millisecond)
	adapter.AppExecuteTx(5*time.Millisecond, true)
	adapter.AppExecuteTx(5*time.Millisecond, false)
	adapter.AppEndBlock(8 * time.Millisecond)
	adapter.AppCommit(100 * time.Millisecond)
	adapter.AppQuery(2*time.Millisecond, "/store/key")
	adapter.AppCheckTx(1*time.Millisecond, true)
	adapter.AppCheckTx(1*time.Millisecond, false)
}

func TestABIMetricsAdapter_Network(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	adapter.NetworkPeers(10)
	adapter.NetworkBytesSent("pex", 1024)
	adapter.NetworkBytesReceived("transactions", 4096)
	adapter.NetworkMessageSent("blocks")
	adapter.NetworkMessageReceived("consensus")
	adapter.NetworkMessageError("pex", "decode_error")
}

func TestABIMetricsAdapter_Storage(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	adapter.BlockStoreHeight(50000)
	adapter.BlockStoreSizeBytes(100 * 1024 * 1024)
	adapter.StateStoreCommit(50 * time.Millisecond)
	adapter.StateStoreGet(100 * time.Microsecond)
	adapter.StateStoreSet(200 * time.Microsecond)
}

func TestABIMetricsAdapter_Sync(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	// Test syncing state
	adapter.SyncProgress(1000, 50000)
	adapter.SyncBlocksReceived(100)
	adapter.SyncDuration(30 * time.Second)

	// Test synced state
	adapter.SyncProgress(50000, 50000)
}

func TestABIMetricsAdapter_SyncZeroTarget(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	// Should not panic with zero target
	adapter.SyncProgress(0, 0)
}

func TestABIMetricsAdapter_Handler(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	handler := adapter.Handler()
	if handler == nil {
		t.Error("Handler() returned nil")
	}
}

func TestABIMetricsAdapter_Prometheus(t *testing.T) {
	adapter := NewABIMetricsAdapter("test")

	prom := adapter.Prometheus()
	if prom == nil {
		t.Error("Prometheus() returned nil")
	}
	if prom != adapter.prom {
		t.Error("Prometheus() should return the internal instance")
	}
}

func TestABIMetricsAdapter_Interface(t *testing.T) {
	// Verify ABIMetricsAdapter implements abi.Metrics at compile time
	var _ abi.Metrics = (*ABIMetricsAdapter)(nil)
}
