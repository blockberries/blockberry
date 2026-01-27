package looseberry

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/mempool"
	"github.com/blockberries/looseberry"
	lbnetwork "github.com/blockberries/looseberry/network"
	lbtypes "github.com/blockberries/looseberry/types"
)

// newTestLooseberryConfig creates a looseberry config for testing.
func newTestLooseberryConfig() *looseberry.Config {
	cfg := looseberry.DefaultConfig()
	cfg.ValidatorIndex = 0
	cfg.Storage.InMemory = true // Use in-memory for tests
	return cfg
}

// mockValidatorSet implements mempool.ValidatorSet for testing.
type mockValidatorSet struct {
	count int
}

func (m *mockValidatorSet) Count() int {
	return m.count
}

func (m *mockValidatorSet) GetPublicKey(index uint16) []byte {
	if int(index) >= m.count {
		return nil
	}
	// Return a dummy 32-byte public key
	pk := make([]byte, 32)
	pk[0] = byte(index)
	return pk
}

func (m *mockValidatorSet) F() int {
	return (m.count - 1) / 3
}

func (m *mockValidatorSet) Quorum() int {
	return m.count - m.F()
}

func (m *mockValidatorSet) VerifySignature(index uint16, digest []byte, sig []byte) bool {
	return true
}

// mockNetwork implements lbnetwork.Network for testing.
type mockNetwork struct {
	validatorID uint16

	batchMessages        chan *lbnetwork.BatchMessage
	batchAckMessages     chan *lbnetwork.BatchAckMessage
	batchRequestMessages chan *lbnetwork.BatchRequestMessage
	headerMessages       chan *lbnetwork.HeaderMessage
	voteMessages         chan *lbnetwork.VoteMessage
	certificateMessages  chan *lbnetwork.CertificateMessage
	syncRequests         chan *lbnetwork.SyncRequest
	syncResponses        chan *lbnetwork.SyncResponseMessage
}

func newMockNetwork(validatorID uint16) *mockNetwork {
	return &mockNetwork{
		validatorID:          validatorID,
		batchMessages:        make(chan *lbnetwork.BatchMessage, 100),
		batchAckMessages:     make(chan *lbnetwork.BatchAckMessage, 100),
		batchRequestMessages: make(chan *lbnetwork.BatchRequestMessage, 100),
		headerMessages:       make(chan *lbnetwork.HeaderMessage, 100),
		voteMessages:         make(chan *lbnetwork.VoteMessage, 100),
		certificateMessages:  make(chan *lbnetwork.CertificateMessage, 100),
		syncRequests:         make(chan *lbnetwork.SyncRequest, 100),
		syncResponses:        make(chan *lbnetwork.SyncResponseMessage, 100),
	}
}

func (m *mockNetwork) BroadcastBatch(batch *lbtypes.Batch) error            { return nil }
func (m *mockNetwork) BroadcastHeader(header *lbtypes.Header) error         { return nil }
func (m *mockNetwork) BroadcastCertificate(cert *lbtypes.Certificate) error { return nil }
func (m *mockNetwork) SendVote(validator uint16, vote *lbtypes.Vote) error  { return nil }
func (m *mockNetwork) SendBatchAck(validator uint16, ack *lbnetwork.BatchAckMessage) error {
	return nil
}
func (m *mockNetwork) SendBatchRequest(validator uint16, req *lbnetwork.BatchRequestMessage) error {
	return nil
}
func (m *mockNetwork) SendSyncRequest(validator uint16, req *lbnetwork.SyncRequest) error {
	return nil
}
func (m *mockNetwork) SendSyncResponse(validator uint16, resp *lbnetwork.SyncResponse) error {
	return nil
}

func (m *mockNetwork) BatchMessages() <-chan *lbnetwork.BatchMessage       { return m.batchMessages }
func (m *mockNetwork) BatchAckMessages() <-chan *lbnetwork.BatchAckMessage { return m.batchAckMessages }
func (m *mockNetwork) BatchRequestMessages() <-chan *lbnetwork.BatchRequestMessage {
	return m.batchRequestMessages
}
func (m *mockNetwork) HeaderMessages() <-chan *lbnetwork.HeaderMessage { return m.headerMessages }
func (m *mockNetwork) VoteMessages() <-chan *lbnetwork.VoteMessage     { return m.voteMessages }
func (m *mockNetwork) CertificateMessages() <-chan *lbnetwork.CertificateMessage {
	return m.certificateMessages
}
func (m *mockNetwork) SyncRequests() <-chan *lbnetwork.SyncRequest          { return m.syncRequests }
func (m *mockNetwork) SyncResponses() <-chan *lbnetwork.SyncResponseMessage { return m.syncResponses }
func (m *mockNetwork) ValidatorID() uint16                                  { return m.validatorID }
func (m *mockNetwork) Start() error                                         { return nil }
func (m *mockNetwork) Stop() error                                          { return nil }

// Verify mockNetwork implements lbnetwork.Network
var _ lbnetwork.Network = (*mockNetwork)(nil)

// newTestAdapter creates an adapter with a mock validator set and network for testing.
func newTestAdapter(t *testing.T) *Adapter {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// Set validator set before starting (required by looseberry)
	mockValSet := &mockValidatorSet{count: 4}
	adapter.UpdateValidatorSet(mockValSet)

	// Set mock network (required by looseberry)
	mockNet := newMockNetwork(0)
	adapter.lb.SetNetwork(mockNet)

	return adapter
}

func TestNewAdapter_NilConfig(t *testing.T) {
	adapter, err := NewAdapter(nil)
	assert.Nil(t, adapter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestNewAdapter_NilLooseberryConfig(t *testing.T) {
	cfg := &Config{
		ValidatorIndex: 0,
	}
	adapter, err := NewAdapter(cfg)
	assert.Nil(t, adapter)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "looseberry config is required")
}

func TestNewAdapter_Success(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)
	require.NotNil(t, adapter)

	assert.Equal(t, "looseberry-adapter", adapter.Name())
	assert.False(t, adapter.IsRunning())
}

func TestAdapter_StartStop(t *testing.T) {
	adapter := newTestAdapter(t)

	// Start
	err := adapter.Start()
	require.NoError(t, err)
	assert.True(t, adapter.IsRunning())

	// Double start should error
	err = adapter.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Stop
	err = adapter.Stop()
	require.NoError(t, err)
	assert.False(t, adapter.IsRunning())

	// Double stop should be idempotent
	err = adapter.Stop()
	require.NoError(t, err)
}

func TestAdapter_AddTx(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// Add transaction
	tx := []byte("test transaction")
	err = adapter.AddTx(tx)
	require.NoError(t, err)

	// Check size
	assert.Equal(t, 1, adapter.Size())
	assert.Greater(t, adapter.SizeBytes(), int64(0))
}

func TestAdapter_HasTx(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// Transaction not present initially
	hash := []byte{1, 2, 3, 4}
	assert.False(t, adapter.HasTx(hash))
}

func TestAdapter_GetTx_NotSupported(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// GetTx is not supported in DAG mempool
	tx, err := adapter.GetTx([]byte{1, 2, 3})
	assert.Nil(t, tx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestAdapter_RemoveTxs_NoOp(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// RemoveTxs is a no-op for DAG mempools
	adapter.RemoveTxs([][]byte{{1, 2, 3}})
}

func TestAdapter_Flush(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// Add some transactions
	for i := range 5 {
		err = adapter.AddTx([]byte{byte(i)})
		require.NoError(t, err)
	}

	assert.Equal(t, 5, adapter.Size())

	// Flush
	adapter.Flush()
	assert.Equal(t, 0, adapter.Size())
}

func TestAdapter_TxHashes(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// TxHashes returns nil (not implemented for DAG mempool)
	hashes := adapter.TxHashes()
	assert.Nil(t, hashes)
}

func TestAdapter_ReapTxs(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// ReapTxs returns empty when no certified batches
	txs := adapter.ReapTxs(1024 * 1024)
	assert.Empty(t, txs)
}

func TestAdapter_ReapCertifiedBatches(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// ReapCertifiedBatches returns empty when no certified batches
	batches := adapter.ReapCertifiedBatches(1024 * 1024)
	assert.Empty(t, batches)
}

func TestAdapter_CurrentRound(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// Initial round should be 0
	round := adapter.CurrentRound()
	assert.Equal(t, uint64(0), round)
}

func TestAdapter_NotifyCommitted(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// NotifyCommitted should not panic
	adapter.NotifyCommitted(1)
}

func TestAdapter_DAGMetrics(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	metrics := adapter.DAGMetrics()
	require.NotNil(t, metrics)
	assert.Equal(t, uint64(0), metrics.CurrentRound)
}

func TestAdapter_StreamConfigs(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	configs := adapter.StreamConfigs()
	require.Len(t, configs, 3)

	// Verify stream names
	names := make(map[string]bool)
	for _, c := range configs {
		names[c.Name] = true
		assert.True(t, c.Encrypted)
		assert.Greater(t, c.RateLimit, 0)
		assert.Greater(t, c.MaxMessageSize, 0)
		assert.Equal(t, "looseberry", c.Owner)
	}

	assert.True(t, names["looseberry-batches"])
	assert.True(t, names["looseberry-headers"])
	assert.True(t, names["looseberry-sync"])
}

func TestAdapter_SetNetwork(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// SetNetwork should not panic
	adapter.SetNetwork(nil)
}

func TestAdapter_SetTxValidator(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// Set a validator
	validator := func(tx []byte) error {
		return nil
	}
	adapter.SetTxValidator(validator)

	// The validator wrapper should be set
	require.NotNil(t, cfg.LooseberryConfig.TxValidator)
}

func TestAdapter_UpdateValidatorSet(t *testing.T) {
	adapter := newTestAdapter(t)

	err := adapter.Start()
	require.NoError(t, err)
	defer adapter.Stop()

	// Create new mock validator set
	mockValSet := &mockValidatorSet{
		count: 7,
	}

	// UpdateValidatorSet should not panic
	adapter.UpdateValidatorSet(mockValSet)
}

// Test interface compliance
func TestAdapter_InterfaceCompliance(t *testing.T) {
	cfg := &Config{
		ValidatorIndex:   0,
		LooseberryConfig: newTestLooseberryConfig(),
	}

	adapter, err := NewAdapter(cfg)
	require.NoError(t, err)

	// Verify interface implementations
	var _ mempool.Mempool = adapter
	var _ mempool.DAGMempool = adapter
	var _ mempool.NetworkAwareMempool = adapter
}
