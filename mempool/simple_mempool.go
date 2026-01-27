package mempool

import (
	"sync"

	"github.com/blockberries/blockberry/types"
)

// SimpleMempool implements Mempool with a simple hash-based storage.
type SimpleMempool struct {
	// txs maps transaction hash (as string) to transaction data
	txs map[string][]byte

	// order maintains insertion order for ReapTxs
	order [][]byte

	// Configuration
	maxTxs   int
	maxBytes int64

	// Current state
	sizeBytes int64

	// Transaction validation
	validator TxValidator

	mu sync.RWMutex
}

// NewSimpleMempool creates a new simple mempool.
func NewSimpleMempool(maxTxs int, maxBytes int64) *SimpleMempool {
	return &SimpleMempool{
		txs:      make(map[string][]byte),
		order:    make([][]byte, 0),
		maxTxs:   maxTxs,
		maxBytes: maxBytes,
	}
}

// AddTx adds a transaction to the mempool.
// The transaction is validated before being added.
// If no validator is set, DefaultTxValidator (reject all) is used.
func (m *SimpleMempool) AddTx(tx []byte) error {
	if tx == nil {
		return types.ErrInvalidTx
	}

	hash := types.HashTx(tx)
	hashKey := string(hash)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.txs[hashKey]; exists {
		return types.ErrTxAlreadyExists
	}

	// Validate transaction using validator or default (fail-closed)
	validator := m.validator
	if validator == nil {
		validator = DefaultTxValidator
	}
	if err := validator(tx); err != nil {
		return err
	}

	// Check capacity
	txSize := int64(len(tx))
	if m.maxTxs > 0 && len(m.txs) >= m.maxTxs {
		return types.ErrMempoolFull
	}
	if m.maxBytes > 0 && m.sizeBytes+txSize > m.maxBytes {
		return types.ErrMempoolFull
	}

	// Add transaction
	m.txs[hashKey] = tx
	m.order = append(m.order, hash)
	m.sizeBytes += txSize

	return nil
}

// RemoveTxs removes transactions by their hashes.
func (m *SimpleMempool) RemoveTxs(hashes [][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, hash := range hashes {
		hashKey := string(hash)
		if tx, exists := m.txs[hashKey]; exists {
			m.sizeBytes -= int64(len(tx))
			delete(m.txs, hashKey)
		}
	}

	// Rebuild order list (remove deleted hashes)
	newOrder := make([][]byte, 0, len(m.order))
	for _, hash := range m.order {
		if _, exists := m.txs[string(hash)]; exists {
			newOrder = append(newOrder, hash)
		}
	}
	m.order = newOrder
}

// ReapTxs returns up to maxBytes worth of transactions in insertion order.
func (m *SimpleMempool) ReapTxs(maxBytes int64) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if maxBytes <= 0 {
		maxBytes = m.maxBytes
	}

	result := make([][]byte, 0, len(m.order))
	var totalBytes int64

	for _, hash := range m.order {
		tx := m.txs[string(hash)]
		txSize := int64(len(tx))

		if maxBytes > 0 && totalBytes+txSize > maxBytes {
			break
		}

		result = append(result, tx)
		totalBytes += txSize
	}

	return result
}

// HasTx checks if a transaction with the given hash exists.
func (m *SimpleMempool) HasTx(hash []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.txs[string(hash)]
	return exists
}

// GetTx retrieves a transaction by its hash.
func (m *SimpleMempool) GetTx(hash []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tx, exists := m.txs[string(hash)]
	if !exists {
		return nil, types.ErrTxNotFound
	}
	return tx, nil
}

// Size returns the number of transactions in the mempool.
func (m *SimpleMempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.txs)
}

// SizeBytes returns the total size in bytes of all transactions.
func (m *SimpleMempool) SizeBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sizeBytes
}

// Flush removes all transactions from the mempool.
func (m *SimpleMempool) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.txs = make(map[string][]byte)
	m.order = m.order[:0]
	m.sizeBytes = 0
}

// TxHashes returns all transaction hashes in the mempool.
func (m *SimpleMempool) TxHashes() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([][]byte, len(m.order))
	copy(result, m.order)
	return result
}

// SetTxValidator sets the transaction validation function.
func (m *SimpleMempool) SetTxValidator(validator TxValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validator = validator
}
