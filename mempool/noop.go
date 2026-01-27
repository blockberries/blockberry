package mempool

import (
	"github.com/blockberries/blockberry/types"
)

// NoOpMempool is a mempool implementation that does nothing.
// It rejects all transactions and always reports being empty.
// Use this for node roles that don't need a mempool (e.g., seed nodes).
type NoOpMempool struct{}

// NewNoOpMempool creates a new no-op mempool.
func NewNoOpMempool() *NoOpMempool {
	return &NoOpMempool{}
}

// AddTx always returns an error as no-op mempool doesn't accept transactions.
func (m *NoOpMempool) AddTx(_ []byte) error {
	return types.ErrMempoolClosed
}

// RemoveTxs does nothing.
func (m *NoOpMempool) RemoveTxs(_ [][]byte) {}

// ReapTxs always returns an empty slice.
func (m *NoOpMempool) ReapTxs(_ int64) [][]byte {
	return nil
}

// HasTx always returns false.
func (m *NoOpMempool) HasTx(_ []byte) bool {
	return false
}

// GetTx always returns an error.
func (m *NoOpMempool) GetTx(_ []byte) ([]byte, error) {
	return nil, types.ErrTxNotFound
}

// Size always returns 0.
func (m *NoOpMempool) Size() int {
	return 0
}

// SizeBytes always returns 0.
func (m *NoOpMempool) SizeBytes() int64 {
	return 0
}

// Flush does nothing.
func (m *NoOpMempool) Flush() {}

// TxHashes always returns an empty slice.
func (m *NoOpMempool) TxHashes() [][]byte {
	return nil
}

// SetTxValidator does nothing.
func (m *NoOpMempool) SetTxValidator(_ TxValidator) {}

// Verify NoOpMempool implements Mempool interface.
var _ Mempool = (*NoOpMempool)(nil)
