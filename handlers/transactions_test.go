package handlers

import (
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/mempool"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

func TestNewTransactionsReactor(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	require.NotNil(t, reactor)
	require.Equal(t, mp, reactor.mempool)
	require.Equal(t, int32(50), reactor.batchSize)
	require.Equal(t, time.Second, reactor.requestInterval)
}

func TestTransactionsReactor_StartStop(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	reactor := NewTransactionsReactor(mp, nil, nil, 100*time.Millisecond, 50)

	// Should not be running initially
	require.False(t, reactor.IsRunning())

	// Start
	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.IsRunning())

	// Start again should be no-op
	err = reactor.Start()
	require.NoError(t, err)

	// Stop
	err = reactor.Stop()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())

	// Stop again should be no-op
	err = reactor.Stop()
	require.NoError(t, err)
}

func TestTransactionsReactor_HandleMessageEmpty(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	err := reactor.HandleMessage(peer.ID("peer1"), nil)
	require.ErrorIs(t, err, types.ErrInvalidMessage)

	err = reactor.HandleMessage(peer.ID("peer1"), []byte{})
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestTransactionsReactor_HandleMessageUnknownType(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	// Create a message with unknown type ID
	w := cramberry.GetWriter()
	w.WriteTypeID(255)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.ErrorIs(t, err, types.ErrUnknownMessageType)
}

func TestTransactionsReactor_EncodeDecodeTransactionsRequest(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	batchSize := int32(100)
	req := &schema.TransactionsRequest{
		BatchSize: &batchSize,
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDTransactionsRequest, req)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDTransactionsRequest, typeID)

	// Decode message
	var decoded schema.TransactionsRequest
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.NotNil(t, decoded.BatchSize)
	require.Equal(t, batchSize, *decoded.BatchSize)
}

func TestTransactionsReactor_EncodeDecodeTransactionsResponse(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	resp := &schema.TransactionsResponse{
		Transactions: []schema.TransactionHash{
			{Hash: []byte("hash1")},
			{Hash: []byte("hash2")},
		},
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDTransactionsResponse, resp)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDTransactionsResponse, typeID)

	// Decode message
	var decoded schema.TransactionsResponse
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.Len(t, decoded.Transactions, 2)
	require.Equal(t, []byte("hash1"), decoded.Transactions[0].Hash)
	require.Equal(t, []byte("hash2"), decoded.Transactions[1].Hash)
}

func TestTransactionsReactor_EncodeDecodeTransactionDataRequest(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	req := &schema.TransactionDataRequest{
		Transactions: []schema.TransactionHash{
			{Hash: []byte("hash1")},
		},
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDTransactionDataRequest, req)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDTransactionDataRequest, typeID)

	// Decode message
	var decoded schema.TransactionDataRequest
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.Len(t, decoded.Transactions, 1)
	require.Equal(t, []byte("hash1"), decoded.Transactions[0].Hash)
}

func TestTransactionsReactor_EncodeDecodeTransactionDataResponse(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	resp := &schema.TransactionDataResponse{
		Transactions: []schema.TransactionData{
			{Hash: []byte("hash1"), Data: []byte("data1")},
		},
	}

	// Encode
	data, err := reactor.encodeMessage(TypeIDTransactionDataResponse, resp)
	require.NoError(t, err)

	// Decode type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDTransactionDataResponse, typeID)

	// Decode message
	var decoded schema.TransactionDataResponse
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.Len(t, decoded.Transactions, 1)
	require.Equal(t, []byte("hash1"), decoded.Transactions[0].Hash)
	require.Equal(t, []byte("data1"), decoded.Transactions[0].Data)
}

func TestTransactionsReactor_HandleTransactionsRequest(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Add some transactions to the mempool
	tx1 := []byte("transaction 1")
	tx2 := []byte("transaction 2")
	require.NoError(t, mp.AddTx(tx1))
	require.NoError(t, mp.AddTx(tx2))

	// Create request
	batchSize := int32(10)
	req := &schema.TransactionsRequest{
		BatchSize: &batchSize,
	}

	// Encode request
	data, err := reactor.encodeMessage(TypeIDTransactionsRequest, req)
	require.NoError(t, err)

	// Handle request (no network, so response won't be sent but shouldn't error)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestTransactionsReactor_HandleTransactionsResponse(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Add one transaction
	tx1 := []byte("transaction 1")
	require.NoError(t, mp.AddTx(tx1))
	hash1 := types.HashTx(tx1)

	// Create response with one known and one unknown hash
	unknownHash := types.HashTx([]byte("unknown"))
	resp := &schema.TransactionsResponse{
		Transactions: []schema.TransactionHash{
			{Hash: hash1},       // Already have this
			{Hash: unknownHash}, // Don't have this
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDTransactionsResponse, resp)
	require.NoError(t, err)

	// Handle response (no network, so data request won't be sent)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestTransactionsReactor_HandleTransactionDataRequest(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Add a transaction
	tx := []byte("test transaction")
	require.NoError(t, mp.AddTx(tx))
	hash := types.HashTx(tx)

	// Create data request
	req := &schema.TransactionDataRequest{
		Transactions: []schema.TransactionHash{
			{Hash: hash},
			{Hash: []byte("unknown")}, // This one doesn't exist
		},
	}

	// Encode request
	data, err := reactor.encodeMessage(TypeIDTransactionDataRequest, req)
	require.NoError(t, err)

	// Handle request (no network, so response won't be sent)
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestTransactionsReactor_HandleTransactionDataResponse(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Create transaction with correct hash
	tx := []byte("test transaction data")
	hash := types.HashTx(tx)

	// Create response with correct data
	resp := &schema.TransactionDataResponse{
		Transactions: []schema.TransactionData{
			{Hash: hash, Data: tx},
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDTransactionDataResponse, resp)
	require.NoError(t, err)

	// Handle response
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Transaction should now be in mempool
	require.True(t, mp.HasTx(hash))
	gotTx, err := mp.GetTx(hash)
	require.NoError(t, err)
	require.Equal(t, tx, gotTx)
}

func TestTransactionsReactor_HandleTransactionDataResponseHashMismatch(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Create transaction with wrong hash
	tx := []byte("test transaction data")
	wrongHash := []byte("wrong hash that doesn't match")

	// Create response with mismatched hash
	resp := &schema.TransactionDataResponse{
		Transactions: []schema.TransactionData{
			{Hash: wrongHash, Data: tx},
		},
	}

	// Encode response
	data, err := reactor.encodeMessage(TypeIDTransactionDataResponse, resp)
	require.NoError(t, err)

	// Handle response - should not error but should reject the transaction
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Transaction should NOT be in mempool due to hash mismatch
	require.False(t, mp.HasTx(wrongHash))
	correctHash := types.HashTx(tx)
	require.False(t, mp.HasTx(correctHash))
}

func TestTransactionsReactor_OnPeerDisconnected(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	peerID := peer.ID("peer1")

	// Add some pending requests
	reactor.mu.Lock()
	reactor.pendingRequests[peerID] = map[string]time.Time{
		"hash1": time.Now(),
		"hash2": time.Now(),
	}
	reactor.mu.Unlock()

	// Disconnect peer
	reactor.OnPeerDisconnected(peerID)

	// Pending requests should be cleared
	reactor.mu.RLock()
	_, exists := reactor.pendingRequests[peerID]
	reactor.mu.RUnlock()
	require.False(t, exists)
}

func TestTransactionsReactor_PendingRequests(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	peerID := peer.ID("peer1")
	txHash := []byte("test hash")

	// Initially no pending request
	require.False(t, reactor.hasPendingRequest(peerID, txHash))

	// Add pending request
	reactor.mu.Lock()
	reactor.pendingRequests[peerID] = map[string]time.Time{
		string(txHash): time.Now(),
	}
	reactor.mu.Unlock()

	// Should now have pending request
	require.True(t, reactor.hasPendingRequest(peerID, txHash))

	// Clear it
	reactor.clearPendingRequest(peerID, txHash)

	// Should no longer have pending request
	require.False(t, reactor.hasPendingRequest(peerID, txHash))
}

func TestTransactionsReactor_TypeIDConstants(t *testing.T) {
	// Verify type IDs match the schema
	require.Equal(t, cramberry.TypeID(133), TypeIDTransactionsRequest)
	require.Equal(t, cramberry.TypeID(134), TypeIDTransactionsResponse)
	require.Equal(t, cramberry.TypeID(135), TypeIDTransactionDataRequest)
	require.Equal(t, cramberry.TypeID(136), TypeIDTransactionDataResponse)
}

func TestTransactionsReactor_HandleDuplicateTransaction(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Add a transaction
	tx := []byte("duplicate test transaction")
	hash := types.HashTx(tx)
	require.NoError(t, mp.AddTx(tx))

	// Try to add same transaction via data response
	resp := &schema.TransactionDataResponse{
		Transactions: []schema.TransactionData{
			{Hash: hash, Data: tx},
		},
	}

	data, err := reactor.encodeMessage(TypeIDTransactionDataResponse, resp)
	require.NoError(t, err)

	// Handle response - should not error even though tx already exists
	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Mempool should still have exactly 1 transaction
	require.Equal(t, 1, mp.Size())
}

func TestTransactionsReactor_DefaultMaxPendingAge(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	// Verify default maxPendingAge is set
	require.Equal(t, DefaultMaxPendingAge, reactor.maxPendingAge)
}

func TestTransactionsReactor_CleanupStaleRequests(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)
	reactor.maxPendingAge = 100 * time.Millisecond // Short timeout for testing

	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")

	// Add some pending requests
	reactor.mu.Lock()
	now := time.Now()
	// Stale request for peer1 (old)
	reactor.pendingRequests[peer1] = map[string]time.Time{
		"stale_hash": now.Add(-200 * time.Millisecond), // Older than maxPendingAge
		"fresh_hash": now,                              // Fresh
	}
	// All stale requests for peer2
	reactor.pendingRequests[peer2] = map[string]time.Time{
		"stale1": now.Add(-200 * time.Millisecond),
		"stale2": now.Add(-300 * time.Millisecond),
	}
	reactor.mu.Unlock()

	// Run cleanup
	reactor.cleanupStaleRequests()

	// Check results
	reactor.mu.RLock()
	defer reactor.mu.RUnlock()

	// peer1 should still exist with only the fresh hash
	pending1, exists1 := reactor.pendingRequests[peer1]
	require.True(t, exists1, "peer1 should still have pending requests")
	require.Len(t, pending1, 1, "peer1 should have exactly 1 pending request")
	_, hasFresh := pending1["fresh_hash"]
	require.True(t, hasFresh, "fresh_hash should remain")
	_, hasStale := pending1["stale_hash"]
	require.False(t, hasStale, "stale_hash should be removed")

	// peer2 should be completely removed (all requests were stale)
	_, exists2 := reactor.pendingRequests[peer2]
	require.False(t, exists2, "peer2 should be removed (all requests stale)")
}

func TestTransactionsReactor_CleanupStaleRequestsEmpty(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)

	// Run cleanup on empty pendingRequests - should not panic
	reactor.cleanupStaleRequests()

	reactor.mu.RLock()
	defer reactor.mu.RUnlock()
	require.Empty(t, reactor.pendingRequests)
}

func TestTransactionsReactor_CleanupStaleRequestsAllFresh(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)
	reactor.maxPendingAge = time.Hour // Long timeout

	peer1 := peer.ID("peer1")

	// Add fresh pending requests
	reactor.mu.Lock()
	now := time.Now()
	reactor.pendingRequests[peer1] = map[string]time.Time{
		"hash1": now,
		"hash2": now,
	}
	reactor.mu.Unlock()

	// Run cleanup
	reactor.cleanupStaleRequests()

	// All requests should remain
	reactor.mu.RLock()
	defer reactor.mu.RUnlock()

	pending, exists := reactor.pendingRequests[peer1]
	require.True(t, exists)
	require.Len(t, pending, 2)
}

// mockDAGMempool implements mempool.DAGMempool for testing passive mode.
type mockDAGMempool struct {
	mempool.Mempool
}

func (m *mockDAGMempool) Name() string    { return "mock-dag-mempool" }
func (m *mockDAGMempool) Start() error    { return nil }
func (m *mockDAGMempool) Stop() error     { return nil }
func (m *mockDAGMempool) IsRunning() bool { return true }
func (m *mockDAGMempool) ReapCertifiedBatches(maxBytes int64) []mempool.CertifiedBatch {
	return nil
}
func (m *mockDAGMempool) NotifyCommitted(round uint64)                       {}
func (m *mockDAGMempool) UpdateValidatorSet(validators mempool.ValidatorSet) {}
func (m *mockDAGMempool) CurrentRound() uint64                               { return 0 }
func (m *mockDAGMempool) DAGMetrics() *mempool.DAGMempoolMetrics {
	return &mempool.DAGMempoolMetrics{}
}

func TestTransactionsReactor_PassiveModeAutoDetection(t *testing.T) {
	// With simple mempool - should NOT be passive
	simpleMp := mempool.NewSimpleMempool(100, 1024*1024)
	reactor1 := NewTransactionsReactor(simpleMp, nil, nil, time.Second, 50)
	require.False(t, reactor1.IsPassiveMode())
	require.Nil(t, reactor1.DAGMempool())

	// With DAG mempool - should be passive
	dagMp := &mockDAGMempool{Mempool: simpleMp}
	reactor2 := NewTransactionsReactor(dagMp, nil, nil, time.Second, 50)
	require.True(t, reactor2.IsPassiveMode())
	require.NotNil(t, reactor2.DAGMempool())
	require.Equal(t, dagMp, reactor2.DAGMempool())
}

func TestTransactionsReactor_SetPassiveMode(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)

	// Initially not passive
	require.False(t, reactor.IsPassiveMode())

	// Enable passive mode
	reactor.SetPassiveMode(true)
	require.True(t, reactor.IsPassiveMode())

	// Disable passive mode
	reactor.SetPassiveMode(false)
	require.False(t, reactor.IsPassiveMode())
}

func TestTransactionsReactor_PassiveModeSkipsGossipLoop(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	reactor := NewTransactionsReactor(mp, nil, nil, 10*time.Millisecond, 50)
	reactor.SetPassiveMode(true)

	// Start in passive mode
	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.IsRunning())

	// In passive mode, no goroutine is started for gossip loop
	// The wg.Add(1) is only called when not in passive mode
	// We can verify this by checking that Stop() returns immediately

	// Stop
	err = reactor.Stop()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())
}

func TestTransactionsReactor_PassiveModeHandleMessageOnlyDataResponse(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)
	reactor.SetPassiveMode(true)

	// Create transaction data
	tx := []byte("test transaction in passive mode")
	hash := types.HashTx(tx)

	// Transaction data response should be processed in passive mode
	dataResp := &schema.TransactionDataResponse{
		Transactions: []schema.TransactionData{
			{Hash: hash, Data: tx},
		},
	}
	data, err := reactor.encodeMessage(TypeIDTransactionDataResponse, dataResp)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Transaction should be in mempool
	require.True(t, mp.HasTx(hash))
}

func TestTransactionsReactor_PassiveModeIgnoresGossipMessages(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)
	reactor.SetPassiveMode(true)

	// TransactionsRequest should be ignored
	batchSize := int32(10)
	req := &schema.TransactionsRequest{BatchSize: &batchSize}
	data, err := reactor.encodeMessage(TypeIDTransactionsRequest, req)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err) // Should not error, just ignored

	// TransactionsResponse should be ignored
	txResp := &schema.TransactionsResponse{
		Transactions: []schema.TransactionHash{{Hash: []byte("hash1")}},
	}
	data, err = reactor.encodeMessage(TypeIDTransactionsResponse, txResp)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err) // Should not error, just ignored

	// TransactionDataRequest should be ignored
	dataReq := &schema.TransactionDataRequest{
		Transactions: []schema.TransactionHash{{Hash: []byte("hash1")}},
	}
	data, err = reactor.encodeMessage(TypeIDTransactionDataRequest, dataReq)
	require.NoError(t, err)

	err = reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err) // Should not error, just ignored
}

func TestTransactionsReactor_PassiveModeUnknownTypeError(t *testing.T) {
	reactor := NewTransactionsReactor(nil, nil, nil, time.Second, 50)
	reactor.SetPassiveMode(true)

	// Unknown type should still return error in passive mode
	w := cramberry.GetWriter()
	w.WriteTypeID(255)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.ErrorIs(t, err, types.ErrUnknownMessageType)
}

func TestTransactionsReactor_PassiveModeBroadcastTxSkipped(t *testing.T) {
	mp := mempool.NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(mempool.AcceptAllTxValidator)
	reactor := NewTransactionsReactor(mp, nil, nil, time.Second, 50)
	reactor.SetPassiveMode(true)

	// BroadcastTx should return immediately in passive mode
	tx := []byte("test transaction")
	err := reactor.BroadcastTx(tx)
	require.NoError(t, err)

	// Even if we had network and peerManager, nothing would be sent
}
