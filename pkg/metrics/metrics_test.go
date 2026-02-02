package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusMetrics_Creation(t *testing.T) {
	m := NewPrometheusMetrics("test")
	require.NotNil(t, m)
	require.NotNil(t, m.registry)
}

func TestPrometheusMetrics_PeerMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	// Set peer totals
	m.SetPeersTotal(DirectionInbound, 5)
	m.SetPeersTotal(DirectionOutbound, 3)

	// Increment connections
	m.IncPeerConnections(ResultSuccess)
	m.IncPeerConnections(ResultSuccess)
	m.IncPeerConnections(ResultFailure)

	// Increment disconnections
	m.IncPeerDisconnections(ReasonTimeout)
	m.IncPeerDisconnections(ReasonPenalized)

	// Verify metrics are served
	handler := m.HTTPHandler()
	require.NotNil(t, handler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_peers_total")
	assert.Contains(t, body, "test_peer_connections_total")
	assert.Contains(t, body, "test_peer_disconnections_total")
}

func TestPrometheusMetrics_BlockMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	m.SetBlockHeight(12345)
	m.IncBlocksReceived()
	m.IncBlocksReceived()
	m.IncBlocksProposed()
	m.ObserveBlockLatency(100 * time.Millisecond)
	m.SetBlockSize(1024 * 1024)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_block_height")
	assert.Contains(t, body, "test_blocks_received_total")
	assert.Contains(t, body, "test_blocks_proposed_total")
	assert.Contains(t, body, "test_block_latency_seconds")
	assert.Contains(t, body, "test_block_size_bytes")
}

func TestPrometheusMetrics_TransactionMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	m.SetMempoolSize(100)
	m.SetMempoolBytes(50000)
	m.IncTxsReceived()
	m.IncTxsProposed()
	m.IncTxsRejected(ReasonMempoolFull)
	m.IncTxsRejected(ReasonTxTooLarge)
	m.IncTxsEvicted(EvictionTTLExpired)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_mempool_size")
	assert.Contains(t, body, "test_mempool_bytes")
	assert.Contains(t, body, "test_txs_received_total")
	assert.Contains(t, body, "test_txs_proposed_total")
	assert.Contains(t, body, "test_txs_rejected_total")
	assert.Contains(t, body, "test_txs_evicted_total")
}

func TestPrometheusMetrics_SyncMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	m.SetSyncState(SyncStateSyncing)
	m.SetSyncProgress(0.75)
	m.SetSyncPeerHeight(20000)
	m.IncSyncBlocksReceived(100)
	m.ObserveSyncDuration(30 * time.Second)

	// Change sync state
	m.SetSyncState(SyncStateSynced)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_sync_state")
	assert.Contains(t, body, "test_sync_progress")
	assert.Contains(t, body, "test_sync_peer_height")
	assert.Contains(t, body, "test_sync_blocks_received_total")
	assert.Contains(t, body, "test_sync_duration_seconds")
}

func TestPrometheusMetrics_MessageMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	m.IncMessagesReceived(StreamTransactions)
	m.IncMessagesReceived(StreamBlocks)
	m.IncMessagesSent(StreamPEX)
	m.IncMessageErrors(StreamConsensus, ErrorValidate)
	m.ObserveMessageSize(StreamBlockSync, 1024)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_messages_received_total")
	assert.Contains(t, body, "test_messages_sent_total")
	assert.Contains(t, body, "test_message_errors_total")
	assert.Contains(t, body, "test_message_size_bytes")
}

func TestPrometheusMetrics_LatencyMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	m.SetPeerLatency("peer1", 50*time.Millisecond)
	m.SetPeerLatency("peer2", 100*time.Millisecond)
	m.ObservePeerRTT(75 * time.Millisecond)
	m.ObservePeerRTT(125 * time.Millisecond)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_peer_latency_seconds")
	assert.Contains(t, body, "test_peer_rtt_seconds")
}

func TestPrometheusMetrics_StateStoreMetrics(t *testing.T) {
	m := NewPrometheusMetrics("test")

	m.SetStateStoreVersion(42)
	m.SetStateStoreSize(1024 * 1024 * 100)
	m.IncStateStoreGets()
	m.IncStateStoreSets()
	m.IncStateStoreDeletes()
	m.ObserveStateStoreLatency(OpGet, 1*time.Millisecond)
	m.ObserveStateStoreLatency(OpSet, 5*time.Millisecond)
	m.ObserveStateStoreLatency(OpCommit, 100*time.Millisecond)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	assert.Contains(t, body, "test_statestore_version")
	assert.Contains(t, body, "test_statestore_size_bytes")
	assert.Contains(t, body, "test_statestore_gets_total")
	assert.Contains(t, body, "test_statestore_sets_total")
	assert.Contains(t, body, "test_statestore_deletes_total")
	assert.Contains(t, body, "test_statestore_latency_seconds")
}

func TestPrometheusMetrics_Handler(t *testing.T) {
	m := NewPrometheusMetrics("test")

	// Test that Handler() returns a valid HTTP handler
	handler := m.Handler()
	require.NotNil(t, handler)

	httpHandler, ok := handler.(http.Handler)
	require.True(t, ok)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	httpHandler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/plain")
}

func TestPrometheusMetrics_MetricsFormat(t *testing.T) {
	m := NewPrometheusMetrics("blockberry")

	m.SetBlockHeight(999)
	m.SetPeersTotal(DirectionInbound, 10)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify Prometheus format with HELP and TYPE comments
	assert.Contains(t, body, "# HELP blockberry_block_height")
	assert.Contains(t, body, "# TYPE blockberry_block_height gauge")
	assert.Contains(t, body, "blockberry_block_height 999")
}

func TestNopMetrics_Creation(t *testing.T) {
	m := NewNopMetrics()
	require.NotNil(t, m)
}

func TestNopMetrics_AllMethodsNoop(t *testing.T) {
	m := NewNopMetrics()

	// All these should be no-ops and not panic
	m.SetPeersTotal(DirectionInbound, 5)
	m.IncPeerConnections(ResultSuccess)
	m.IncPeerDisconnections(ReasonTimeout)

	m.SetBlockHeight(12345)
	m.IncBlocksReceived()
	m.IncBlocksProposed()
	m.ObserveBlockLatency(100 * time.Millisecond)
	m.SetBlockSize(1024)

	m.SetMempoolSize(100)
	m.SetMempoolBytes(50000)
	m.IncTxsReceived()
	m.IncTxsProposed()
	m.IncTxsRejected(ReasonMempoolFull)
	m.IncTxsEvicted(EvictionTTLExpired)

	m.SetSyncState(SyncStateSyncing)
	m.SetSyncProgress(0.75)
	m.SetSyncPeerHeight(20000)
	m.IncSyncBlocksReceived(100)
	m.ObserveSyncDuration(30 * time.Second)

	m.IncMessagesReceived(StreamTransactions)
	m.IncMessagesSent(StreamPEX)
	m.IncMessageErrors(StreamConsensus, ErrorValidate)
	m.ObserveMessageSize(StreamBlockSync, 1024)

	m.SetPeerLatency("peer1", 50*time.Millisecond)
	m.ObservePeerRTT(75 * time.Millisecond)

	m.SetStateStoreVersion(42)
	m.SetStateStoreSize(1024 * 1024)
	m.IncStateStoreGets()
	m.IncStateStoreSets()
	m.IncStateStoreDeletes()
	m.ObserveStateStoreLatency(OpGet, 1*time.Millisecond)
}

func TestNopMetrics_HandlerReturnsNil(t *testing.T) {
	m := NewNopMetrics()
	assert.Nil(t, m.Handler())
}

func TestMetricsInterface(t *testing.T) {
	// Verify both implementations satisfy the interface
	var _ Metrics = (*PrometheusMetrics)(nil)
	var _ Metrics = (*NopMetrics)(nil)
}

func TestPrometheusMetrics_ConcurrentAccess(t *testing.T) {
	m := NewPrometheusMetrics("test")

	done := make(chan struct{})
	defer close(done)

	// Start multiple goroutines updating metrics concurrently
	for range 10 {
		go func() {
			for j := range 100 {
				select {
				case <-done:
					return
				default:
					m.SetBlockHeight(int64(j))
					m.IncBlocksReceived()
					m.SetMempoolSize(j)
					m.IncMessagesReceived(StreamTransactions)
					m.ObservePeerRTT(time.Duration(j) * time.Millisecond)
				}
			}
		}()
	}

	// Wait for goroutines to finish
	time.Sleep(100 * time.Millisecond)

	// Metrics should still be servable
	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestLabelConstants(t *testing.T) {
	// Verify label constants are defined correctly
	assert.Equal(t, "inbound", DirectionInbound)
	assert.Equal(t, "outbound", DirectionOutbound)

	assert.Equal(t, "success", ResultSuccess)
	assert.Equal(t, "failure", ResultFailure)

	assert.Equal(t, "synced", SyncStateSynced)
	assert.Equal(t, "syncing", SyncStateSyncing)

	// Verify stream labels match p2p package
	assert.Equal(t, "handshake", StreamHandshake)
	assert.Equal(t, "pex", StreamPEX)
	assert.Equal(t, "transactions", StreamTransactions)
	assert.Equal(t, "blocksync", StreamBlockSync)
	assert.Equal(t, "blocks", StreamBlocks)
	assert.Equal(t, "consensus", StreamConsensus)
	assert.Equal(t, "housekeeping", StreamHousekeeping)
}

func TestPrometheusMetrics_Namespace(t *testing.T) {
	m := NewPrometheusMetrics("mynode")

	m.SetBlockHeight(100)

	handler := m.HTTPHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()

	// All metrics should have the namespace prefix
	assert.Contains(t, body, "mynode_block_height")
	assert.True(t, strings.Contains(body, "mynode_") || strings.Contains(body, "# HELP mynode_"))
}
