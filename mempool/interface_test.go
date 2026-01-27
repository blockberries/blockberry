package mempool

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

// mockDAGMempool implements DAGMempool for testing.
type mockDAGMempool struct {
	*SimpleMempool
	round      uint64
	validators ValidatorSet
	batches    []CertifiedBatch
	running    bool
}

func newMockDAGMempool() *mockDAGMempool {
	return &mockDAGMempool{
		SimpleMempool: NewSimpleMempool(100, 1024*1024),
		round:         1,
		batches:       make([]CertifiedBatch, 0),
	}
}

func (m *mockDAGMempool) Start() error {
	m.running = true
	return nil
}

func (m *mockDAGMempool) Stop() error {
	m.running = false
	return nil
}

func (m *mockDAGMempool) IsRunning() bool {
	return m.running
}

func (m *mockDAGMempool) ReapCertifiedBatches(maxBytes int64) []CertifiedBatch {
	var result []CertifiedBatch
	var totalBytes int64
	for _, batch := range m.batches {
		if totalBytes+int64(len(batch.Batch)) > maxBytes {
			break
		}
		result = append(result, batch)
		totalBytes += int64(len(batch.Batch))
	}
	return result
}

func (m *mockDAGMempool) NotifyCommitted(round uint64) {
	// Remove batches up to committed round
	var remaining []CertifiedBatch
	for _, batch := range m.batches {
		if batch.Round > round {
			remaining = append(remaining, batch)
		}
	}
	m.batches = remaining
}

func (m *mockDAGMempool) UpdateValidatorSet(validators ValidatorSet) {
	m.validators = validators
}

func (m *mockDAGMempool) CurrentRound() uint64 {
	return m.round
}

func (m *mockDAGMempool) DAGMetrics() *DAGMempoolMetrics {
	return &DAGMempoolMetrics{
		CurrentRound:      m.round,
		PendingBatches:    0,
		CertifiedBatches:  len(m.batches),
		TotalTransactions: m.Size(),
		ValidatorCount:    4,
		BytesPending:      0,
		BytesCertified:    m.SizeBytes(),
	}
}

// Verify mockDAGMempool implements DAGMempool
var _ DAGMempool = (*mockDAGMempool)(nil)

// mockNetworkAwareMempool implements NetworkAwareMempool for testing.
type mockNetworkAwareMempool struct {
	*SimpleMempool
	network MempoolNetwork
	streams []StreamConfig
}

func newMockNetworkAwareMempool() *mockNetworkAwareMempool {
	return &mockNetworkAwareMempool{
		SimpleMempool: NewSimpleMempool(100, 1024*1024),
		streams: []StreamConfig{
			{Name: "test-batches", Encrypted: true, RateLimit: 100, Owner: "test"},
			{Name: "test-headers", Encrypted: true, RateLimit: 50, Owner: "test"},
		},
	}
}

func (m *mockNetworkAwareMempool) StreamConfigs() []StreamConfig {
	return m.streams
}

func (m *mockNetworkAwareMempool) SetNetwork(network MempoolNetwork) {
	m.network = network
}

// Verify mockNetworkAwareMempool implements NetworkAwareMempool
var _ NetworkAwareMempool = (*mockNetworkAwareMempool)(nil)

// mockMempoolNetwork implements MempoolNetwork for testing.
type mockMempoolNetwork struct {
	sentMessages []struct {
		peerID peer.ID
		stream string
		data   []byte
	}
	peers []peer.ID
}

func newMockMempoolNetwork() *mockMempoolNetwork {
	return &mockMempoolNetwork{
		peers: []peer.ID{"peer1", "peer2", "peer3"},
	}
}

func (n *mockMempoolNetwork) Send(peerID peer.ID, stream string, data []byte) error {
	n.sentMessages = append(n.sentMessages, struct {
		peerID peer.ID
		stream string
		data   []byte
	}{peerID, stream, data})
	return nil
}

func (n *mockMempoolNetwork) Broadcast(stream string, data []byte) error {
	for _, p := range n.peers {
		n.sentMessages = append(n.sentMessages, struct {
			peerID peer.ID
			stream string
			data   []byte
		}{p, stream, data})
	}
	return nil
}

func (n *mockMempoolNetwork) ConnectedPeers() []peer.ID {
	return n.peers
}

// Verify mockMempoolNetwork implements MempoolNetwork
var _ MempoolNetwork = (*mockMempoolNetwork)(nil)

// mockValidatorSet implements ValidatorSet for testing.
type mockValidatorSet struct {
	validators [][]byte
}

func newMockValidatorSet(count int) *mockValidatorSet {
	validators := make([][]byte, count)
	for i := 0; i < count; i++ {
		validators[i] = make([]byte, 32)
		validators[i][0] = byte(i)
	}
	return &mockValidatorSet{validators: validators}
}

func (v *mockValidatorSet) Count() int {
	return len(v.validators)
}

func (v *mockValidatorSet) GetPublicKey(index uint16) []byte {
	if int(index) >= len(v.validators) {
		return nil
	}
	return v.validators[index]
}

func (v *mockValidatorSet) VerifySignature(validatorIndex uint16, message, signature []byte) bool {
	// Mock always returns true for testing
	return int(validatorIndex) < len(v.validators)
}

func (v *mockValidatorSet) Quorum() int {
	// 2f+1 where f = (n-1)/3
	f := (len(v.validators) - 1) / 3
	return 2*f + 1
}

func (v *mockValidatorSet) F() int {
	return (len(v.validators) - 1) / 3
}

// Verify mockValidatorSet implements ValidatorSet
var _ ValidatorSet = (*mockValidatorSet)(nil)

// mockPrioritizedMempool implements PrioritizedMempool for testing.
type mockPrioritizedMempool struct {
	*SimpleMempool
	priorities map[string]int64
}

func newMockPrioritizedMempool() *mockPrioritizedMempool {
	return &mockPrioritizedMempool{
		SimpleMempool: NewSimpleMempool(100, 1024*1024),
		priorities:    make(map[string]int64),
	}
}

func (m *mockPrioritizedMempool) AddTxWithPriority(tx []byte, priority int64) error {
	if err := m.AddTx(tx); err != nil {
		return err
	}
	hash := string(types.HashTx(tx))
	m.priorities[hash] = priority
	return nil
}

func (m *mockPrioritizedMempool) GetPriority(hash []byte) int64 {
	return m.priorities[string(hash)]
}

// Verify mockPrioritizedMempool implements PrioritizedMempool
var _ PrioritizedMempool = (*mockPrioritizedMempool)(nil)

func TestDAGMempool_Interface(t *testing.T) {
	dag := newMockDAGMempool()
	dag.SetTxValidator(AcceptAllTxValidator)

	// Test Component interface
	err := dag.Start()
	require.NoError(t, err)
	assert.True(t, dag.IsRunning())

	// Test basic mempool operations
	err = dag.AddTx([]byte("tx1"))
	require.NoError(t, err)
	assert.Equal(t, 1, dag.Size())

	// Test DAG-specific operations
	assert.Equal(t, uint64(1), dag.CurrentRound())

	// Add certified batch
	dag.batches = append(dag.batches, CertifiedBatch{
		Batch:          []byte("batch1"),
		Certificate:    []byte("cert1"),
		Round:          1,
		ValidatorIndex: 0,
		Hash:           []byte("hash1"),
	})

	batches := dag.ReapCertifiedBatches(1000)
	assert.Len(t, batches, 1)
	assert.Equal(t, []byte("batch1"), batches[0].Batch)

	// Test NotifyCommitted
	dag.NotifyCommitted(1)
	batches = dag.ReapCertifiedBatches(1000)
	assert.Len(t, batches, 0)

	// Test DAGMetrics
	metrics := dag.DAGMetrics()
	assert.Equal(t, uint64(1), metrics.CurrentRound)
	assert.Equal(t, 1, metrics.TotalTransactions)

	// Test Stop
	err = dag.Stop()
	require.NoError(t, err)
	assert.False(t, dag.IsRunning())
}

func TestNetworkAwareMempool_Interface(t *testing.T) {
	mp := newMockNetworkAwareMempool()
	mp.SetTxValidator(AcceptAllTxValidator)

	// Test StreamConfigs
	configs := mp.StreamConfigs()
	assert.Len(t, configs, 2)
	assert.Equal(t, "test-batches", configs[0].Name)
	assert.True(t, configs[0].Encrypted)
	assert.Equal(t, 100, configs[0].RateLimit)
	assert.Equal(t, "test", configs[0].Owner)

	// Test SetNetwork
	network := newMockMempoolNetwork()
	mp.SetNetwork(network)
	assert.Equal(t, network, mp.network)

	// Test mempool still works after network set
	err := mp.AddTx([]byte("tx1"))
	require.NoError(t, err)
	assert.Equal(t, 1, mp.Size())
}

func TestMempoolNetwork_Interface(t *testing.T) {
	network := newMockMempoolNetwork()

	// Test Send
	err := network.Send("peer1", "test-stream", []byte("data"))
	require.NoError(t, err)
	assert.Len(t, network.sentMessages, 1)
	assert.Equal(t, peer.ID("peer1"), network.sentMessages[0].peerID)
	assert.Equal(t, "test-stream", network.sentMessages[0].stream)

	// Test Broadcast
	err = network.Broadcast("test-stream", []byte("broadcast"))
	require.NoError(t, err)
	assert.Len(t, network.sentMessages, 4) // 1 send + 3 broadcasts

	// Test ConnectedPeers
	peers := network.ConnectedPeers()
	assert.Len(t, peers, 3)
}

func TestValidatorSet_Interface(t *testing.T) {
	valSet := newMockValidatorSet(4)

	// Test Count
	assert.Equal(t, 4, valSet.Count())

	// Test GetPublicKey
	pk := valSet.GetPublicKey(0)
	assert.NotNil(t, pk)
	assert.Equal(t, byte(0), pk[0])

	// Test GetPublicKey out of range
	pk = valSet.GetPublicKey(100)
	assert.Nil(t, pk)

	// Test VerifySignature
	assert.True(t, valSet.VerifySignature(0, []byte("msg"), []byte("sig")))
	assert.False(t, valSet.VerifySignature(100, []byte("msg"), []byte("sig")))

	// Test Quorum (2f+1 where f=(n-1)/3)
	// For n=4: f=1, quorum=3
	assert.Equal(t, 3, valSet.Quorum())
	assert.Equal(t, 1, valSet.F())
}

func TestValidatorSet_QuorumCalculations(t *testing.T) {
	tests := []struct {
		validators int
		expectedF  int
		expectedQ  int
	}{
		{4, 1, 3},   // f=1, quorum=3
		{7, 2, 5},   // f=2, quorum=5
		{10, 3, 7},  // f=3, quorum=7
		{13, 4, 9},  // f=4, quorum=9
		{100, 33, 67}, // f=33, quorum=67
	}

	for _, tc := range tests {
		valSet := newMockValidatorSet(tc.validators)
		assert.Equal(t, tc.expectedF, valSet.F(), "F() for %d validators", tc.validators)
		assert.Equal(t, tc.expectedQ, valSet.Quorum(), "Quorum() for %d validators", tc.validators)
	}
}

func TestCertifiedBatch(t *testing.T) {
	batch := CertifiedBatch{
		Batch:          []byte("batch data"),
		Certificate:    []byte("certificate"),
		Round:          5,
		ValidatorIndex: 2,
		Hash:           []byte("batch hash"),
	}

	assert.Equal(t, []byte("batch data"), batch.Batch)
	assert.Equal(t, []byte("certificate"), batch.Certificate)
	assert.Equal(t, uint64(5), batch.Round)
	assert.Equal(t, uint16(2), batch.ValidatorIndex)
	assert.Equal(t, []byte("batch hash"), batch.Hash)

	// Transactions returns nil by default (placeholder)
	assert.Nil(t, batch.Transactions())
}

func TestDAGMempoolMetrics(t *testing.T) {
	metrics := &DAGMempoolMetrics{
		CurrentRound:      10,
		PendingBatches:    5,
		CertifiedBatches:  3,
		TotalTransactions: 100,
		ValidatorCount:    4,
		BytesPending:      5000,
		BytesCertified:    3000,
	}

	assert.Equal(t, uint64(10), metrics.CurrentRound)
	assert.Equal(t, 5, metrics.PendingBatches)
	assert.Equal(t, 3, metrics.CertifiedBatches)
	assert.Equal(t, 100, metrics.TotalTransactions)
	assert.Equal(t, 4, metrics.ValidatorCount)
	assert.Equal(t, int64(5000), metrics.BytesPending)
	assert.Equal(t, int64(3000), metrics.BytesCertified)
}

func TestStreamConfig(t *testing.T) {
	cfg := StreamConfig{
		Name:           "looseberry-batches",
		Encrypted:      true,
		RateLimit:      1000,
		MaxMessageSize: 10 * 1024 * 1024,
		Owner:          "looseberry",
	}

	assert.Equal(t, "looseberry-batches", cfg.Name)
	assert.True(t, cfg.Encrypted)
	assert.Equal(t, 1000, cfg.RateLimit)
	assert.Equal(t, 10*1024*1024, cfg.MaxMessageSize)
	assert.Equal(t, "looseberry", cfg.Owner)
}

func TestPrioritizedMempool_Interface(t *testing.T) {
	mp := newMockPrioritizedMempool()
	mp.SetTxValidator(AcceptAllTxValidator)

	// Add transactions with different priorities
	err := mp.AddTxWithPriority([]byte("low-priority"), 10)
	require.NoError(t, err)

	err = mp.AddTxWithPriority([]byte("high-priority"), 100)
	require.NoError(t, err)

	// Verify priorities
	lowHash := types.HashTx([]byte("low-priority"))
	highHash := types.HashTx([]byte("high-priority"))

	assert.Equal(t, int64(10), mp.GetPriority(lowHash))
	assert.Equal(t, int64(100), mp.GetPriority(highHash))

	// Non-existent tx returns 0
	assert.Equal(t, int64(0), mp.GetPriority([]byte("nonexistent")))
}

func TestValidatingMempool_Interface(t *testing.T) {
	// SimpleMempool already implements ValidatingMempool
	var mp ValidatingMempool = NewSimpleMempool(100, 1024*1024)

	// Without validator, should reject
	err := mp.AddTx([]byte("tx1"))
	assert.Error(t, err)

	// With validator, should accept
	mp.SetTxValidator(AcceptAllTxValidator)
	err = mp.AddTx([]byte("tx2"))
	assert.NoError(t, err)
	assert.Equal(t, 1, mp.Size())
}

func TestDAGMempool_ValidatorSetUpdate(t *testing.T) {
	dag := newMockDAGMempool()

	// Initially no validator set
	assert.Nil(t, dag.validators)

	// Update validator set
	valSet := newMockValidatorSet(4)
	dag.UpdateValidatorSet(valSet)

	assert.NotNil(t, dag.validators)
	assert.Equal(t, 4, dag.validators.Count())
}

func TestDAGMempool_BatchManagement(t *testing.T) {
	dag := newMockDAGMempool()
	dag.SetTxValidator(AcceptAllTxValidator)

	// Add batches for multiple rounds
	for round := uint64(1); round <= 5; round++ {
		dag.batches = append(dag.batches, CertifiedBatch{
			Batch: []byte("batch"),
			Round: round,
		})
	}

	// Should have 5 batches
	assert.Len(t, dag.batches, 5)

	// Commit round 3
	dag.NotifyCommitted(3)

	// Should have 2 batches remaining (rounds 4 and 5)
	assert.Len(t, dag.batches, 2)

	// Verify remaining rounds
	for _, batch := range dag.batches {
		assert.Greater(t, batch.Round, uint64(3))
	}
}

func TestDAGMempool_ReapCertifiedBatchesRespectsByteLimit(t *testing.T) {
	dag := newMockDAGMempool()

	// Add batches of varying sizes
	dag.batches = []CertifiedBatch{
		{Batch: make([]byte, 100), Round: 1},
		{Batch: make([]byte, 100), Round: 2},
		{Batch: make([]byte, 100), Round: 3},
		{Batch: make([]byte, 100), Round: 4},
	}

	// Reap with limit of 250 bytes (should get 2 batches)
	batches := dag.ReapCertifiedBatches(250)
	assert.Len(t, batches, 2)

	// Reap with limit of 500 bytes (should get all 4)
	batches = dag.ReapCertifiedBatches(500)
	assert.Len(t, batches, 4)

	// Reap with limit of 50 bytes (should get 0)
	batches = dag.ReapCertifiedBatches(50)
	assert.Len(t, batches, 0)
}
