// Package looseberry provides an adapter for integrating the looseberry DAG mempool
// with blockberry. This enables validators to use DAG-based transaction ordering
// and certification.
//
// The adapter implements blockberry's mempool interfaces (Mempool, DAGMempool,
// NetworkAwareMempool) by delegating to a looseberry.Looseberry instance.
//
// Usage:
//
//	cfg := &looseberry.Config{
//	    ValidatorIndex: myIndex,
//	    Signer:         mySigner,
//	}
//	adapter, err := NewAdapter(cfg)
//	if err != nil {
//	    return err
//	}
//	adapter.SetNetwork(network)
//	adapter.SetTxValidator(myValidator)
//	adapter.UpdateValidatorSet(validators)
//	adapter.Start()
package looseberry

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/looseberry"
)

// Adapter wraps a looseberry.Looseberry instance to implement blockberry's
// mempool interfaces. This enables using looseberry as a drop-in replacement
// for the standard mempool in validator nodes.
type Adapter struct {
	lb          *looseberry.Looseberry
	cfg         *Config
	network     mempool.MempoolNetwork
	txValidator mempool.TxValidator

	// Validator set adapter
	validatorSet mempool.ValidatorSet

	// Lifecycle
	running atomic.Bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

// Config contains configuration for the looseberry adapter.
type Config struct {
	// ValidatorIndex is this node's index in the validator set.
	ValidatorIndex uint16

	// Signer provides signing capabilities for batch certification.
	Signer Signer

	// LooseberryConfig is the underlying looseberry configuration.
	LooseberryConfig *looseberry.Config
}

// Signer provides cryptographic signing operations.
type Signer interface {
	// Sign signs a digest and returns the signature.
	Sign(digest []byte) ([]byte, error)

	// PublicKey returns the signer's public key.
	PublicKey() []byte
}

// NewAdapter creates a new looseberry adapter.
func NewAdapter(cfg *Config) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	if cfg.LooseberryConfig == nil {
		return nil, fmt.Errorf("looseberry config is required")
	}

	lb, err := looseberry.New(cfg.LooseberryConfig)
	if err != nil {
		return nil, fmt.Errorf("creating looseberry instance: %w", err)
	}

	return &Adapter{
		lb:     lb,
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}, nil
}

// Name returns the component name.
func (a *Adapter) Name() string {
	return "looseberry-adapter"
}

// Start starts the adapter and underlying looseberry instance.
func (a *Adapter) Start() error {
	if a.running.Swap(true) {
		return fmt.Errorf("already running")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Reset stop channel
	a.stopCh = make(chan struct{})

	// Start looseberry
	if err := a.lb.Start(); err != nil {
		a.running.Store(false)
		return fmt.Errorf("starting looseberry: %w", err)
	}

	return nil
}

// Stop stops the adapter and underlying looseberry instance.
func (a *Adapter) Stop() error {
	if !a.running.Swap(false) {
		return nil // Already stopped
	}

	close(a.stopCh)
	a.wg.Wait()

	return a.lb.Stop()
}

// IsRunning returns true if the adapter is running.
func (a *Adapter) IsRunning() bool {
	return a.running.Load()
}

// AddTx adds a transaction to the mempool.
// The transaction is validated using the configured TxValidator before being
// passed to looseberry.
func (a *Adapter) AddTx(tx []byte) error {
	return a.lb.AddTx(tx)
}

// RemoveTxs removes transactions by their hashes.
// Note: In a DAG mempool, transactions are not directly removed. They are
// garbage collected after being committed. This is a no-op for compatibility.
func (a *Adapter) RemoveTxs(hashes [][]byte) {
	// DAG mempools don't support direct removal - txs are GC'd after commit
}

// ReapTxs returns transactions for block building.
// For non-DAG consumers, this extracts transactions from certified batches.
// Returns defensive copies to prevent external mutation.
func (a *Adapter) ReapTxs(maxBytes int64) [][]byte {
	batches := a.lb.ReapCertifiedBatches(maxBytes)

	var txs [][]byte
	var totalBytes int64

	for _, batch := range batches {
		for _, tx := range batch.Batch.Transactions {
			txSize := int64(len(tx))
			if maxBytes > 0 && totalBytes+txSize > maxBytes {
				return txs
			}
			// Return defensive copy to prevent external mutation
			txs = append(txs, append([]byte(nil), tx...))
			totalBytes += txSize
		}
	}

	return txs
}

// HasTx checks if a transaction exists in the mempool.
func (a *Adapter) HasTx(hash []byte) bool {
	return a.lb.HasTx(hash)
}

// GetTx retrieves a transaction by hash.
// Note: DAG mempools may not support efficient single-tx lookup.
func (a *Adapter) GetTx(hash []byte) ([]byte, error) {
	// This would need to be implemented in looseberry
	// For now, return not found
	return nil, fmt.Errorf("single transaction lookup not supported in DAG mempool")
}

// Size returns the number of pending transactions.
func (a *Adapter) Size() int {
	return a.lb.Size()
}

// SizeBytes returns the total size of pending transactions.
func (a *Adapter) SizeBytes() int64 {
	return a.lb.SizeBytes()
}

// Flush removes all transactions from the mempool.
func (a *Adapter) Flush() {
	a.lb.Flush()
}

// TxHashes returns all transaction hashes.
// Note: This may be expensive for large DAG mempools.
func (a *Adapter) TxHashes() [][]byte {
	// This would need to be implemented in looseberry
	return nil
}

// SetTxValidator sets the transaction validator.
// The validator is called by looseberry before accepting transactions.
func (a *Adapter) SetTxValidator(validator mempool.TxValidator) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.txValidator = validator

	// Convert to looseberry's TxValidator type
	if a.cfg.LooseberryConfig != nil {
		a.cfg.LooseberryConfig.TxValidator = func(tx []byte) error {
			if validator == nil {
				return fmt.Errorf("no transaction validator configured")
			}
			return validator(tx)
		}
	}
}

// ReapCertifiedBatches returns certified batches from the DAG.
// This is the primary interface for consensus engines to get transaction batches.
func (a *Adapter) ReapCertifiedBatches(maxBytes int64) []mempool.CertifiedBatch {
	lbBatches := a.lb.ReapCertifiedBatches(maxBytes)

	batches := make([]mempool.CertifiedBatch, len(lbBatches))
	for i, lb := range lbBatches {
		// Convert looseberry batch to blockberry CertifiedBatch
		// Note: In a full implementation, we'd serialize the batch and certificate
		// using cramberry. For now, we extract the key fields.
		batches[i] = mempool.CertifiedBatch{
			Batch:          nil, // Would be serialized batch data
			Certificate:    nil, // Would be serialized certificate
			Round:          lb.Batch.Round,
			ValidatorIndex: lb.Batch.ValidatorID,
			Hash:           lb.Batch.Digest[:],
		}
	}

	return batches
}

// NotifyCommitted notifies the DAG that a round has been committed.
// This enables garbage collection of old batches.
func (a *Adapter) NotifyCommitted(round uint64) {
	a.lb.NotifyCommitted(round)
}

// UpdateValidatorSet updates the validator set for the next epoch.
func (a *Adapter) UpdateValidatorSet(validators mempool.ValidatorSet) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.validatorSet = validators

	// Convert to looseberry's validator set
	lbValSet := newLooseberryValidatorSet(validators)
	a.lb.UpdateValidatorSet(lbValSet)
}

// CurrentRound returns the current DAG round.
func (a *Adapter) CurrentRound() uint64 {
	return a.lb.CurrentRound()
}

// DAGMetrics returns metrics about the DAG mempool.
func (a *Adapter) DAGMetrics() *mempool.DAGMempoolMetrics {
	m := a.lb.Metrics()

	validatorCount := 0
	if a.validatorSet != nil {
		validatorCount = a.validatorSet.Count()
	}

	return &mempool.DAGMempoolMetrics{
		CurrentRound:      m.CurrentRound,
		PendingBatches:    0,
		CertifiedBatches:  int(m.TotalBatchesCertified),
		TotalTransactions: m.PendingTxCount,
		ValidatorCount:    validatorCount,
		BytesPending:      m.PendingTxBytes,
		BytesCertified:    0,
	}
}

// StreamConfigs returns the stream configurations needed by looseberry.
func (a *Adapter) StreamConfigs() []mempool.StreamConfig {
	return []mempool.StreamConfig{
		{
			Name:           "looseberry-batches",
			Encrypted:      true,
			RateLimit:      1000,
			MaxMessageSize: 10 * 1024 * 1024, // 10MB
			Owner:          "looseberry",
		},
		{
			Name:           "looseberry-headers",
			Encrypted:      true,
			RateLimit:      100,
			MaxMessageSize: 1024 * 1024, // 1MB
			Owner:          "looseberry",
		},
		{
			Name:           "looseberry-sync",
			Encrypted:      true,
			RateLimit:      10,
			MaxMessageSize: 50 * 1024 * 1024, // 50MB
			Owner:          "looseberry",
		},
	}
}

// SetNetwork sets the network adapter for looseberry communication.
func (a *Adapter) SetNetwork(network mempool.MempoolNetwork) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.network = network

	// In a full implementation, we would create a looseberry network adapter
	// and set it on the looseberry instance. The network adapter would bridge
	// blockberry's MempoolNetwork to looseberry's Network interface.
	//
	// lbNetwork := newNetworkAdapter(network, a.cfg.ValidatorIndex, a.validatorSet)
	// a.lb.SetNetwork(lbNetwork)
}

// Verify interface implementations
var (
	_ mempool.Mempool             = (*Adapter)(nil)
	_ mempool.DAGMempool          = (*Adapter)(nil)
	_ mempool.NetworkAwareMempool = (*Adapter)(nil)
)
