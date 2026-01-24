// Package mempool provides transaction pool interface and implementations.
package mempool

import (
	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/types"
)

// Mempool defines the interface for transaction pool management.
// Implementations must be safe for concurrent use.
type Mempool interface {
	// AddTx adds a transaction to the mempool.
	// Returns an error if the transaction already exists or the mempool is full.
	AddTx(tx []byte) error

	// RemoveTxs removes transactions by their hashes.
	// Silently ignores hashes that don't exist in the mempool.
	RemoveTxs(hashes [][]byte)

	// ReapTxs returns up to maxBytes worth of transactions in insertion order.
	// The returned transactions remain in the mempool.
	ReapTxs(maxBytes int64) [][]byte

	// HasTx checks if a transaction with the given hash exists in the mempool.
	HasTx(hash []byte) bool

	// GetTx retrieves a transaction by its hash.
	// Returns types.ErrTxNotFound if the transaction doesn't exist.
	GetTx(hash []byte) ([]byte, error)

	// Size returns the number of transactions in the mempool.
	Size() int

	// SizeBytes returns the total size in bytes of all transactions.
	SizeBytes() int64

	// RootHash returns the merkle root hash of all transaction hashes.
	// This provides a compact commitment to the mempool contents.
	RootHash() []byte

	// Flush removes all transactions from the mempool.
	Flush()
}

// TxInfo contains metadata about a transaction in the mempool.
type TxInfo struct {
	Hash      types.Hash
	Size      int
	Timestamp int64
}

// NewMempool creates a new mempool with the given configuration.
func NewMempool(cfg config.MempoolConfig) Mempool {
	return NewMerkleMempool(cfg.MaxTxs, cfg.MaxBytes)
}
