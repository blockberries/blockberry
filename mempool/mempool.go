// Package mempool provides transaction pool interface and implementations.
package mempool

import (
	"fmt"

	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/types"
)

// TxValidator is a callback for validating transactions before adding to mempool.
// Applications must provide their own validator to accept transactions.
type TxValidator func(tx []byte) error

// DefaultTxValidator is a fail-closed validator that rejects all transactions.
// This is used when no validator is explicitly set, ensuring transactions are never
// accepted without proper validation. Applications MUST provide their own
// validator to accept transactions.
var DefaultTxValidator TxValidator = func(tx []byte) error {
	return fmt.Errorf("%w: no transaction validator configured", types.ErrNoTxValidator)
}

// AcceptAllTxValidator is a validator that accepts all transactions.
// WARNING: This should ONLY be used for testing purposes.
// Production systems must use a proper validator that verifies transaction validity.
var AcceptAllTxValidator TxValidator = func(tx []byte) error {
	return nil
}

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

	// Flush removes all transactions from the mempool.
	Flush()

	// TxHashes returns all transaction hashes in the mempool.
	TxHashes() [][]byte

	// SetTxValidator sets the transaction validation function.
	// Must be called before AddTx to enable validation.
	// If not set, DefaultTxValidator (reject all) is used.
	SetTxValidator(validator TxValidator)
}

// TxInfo contains metadata about a transaction in the mempool.
type TxInfo struct {
	Hash      types.Hash
	Size      int
	Timestamp int64
}

// NewMempool creates a new mempool with the given configuration.
func NewMempool(cfg config.MempoolConfig) Mempool {
	return NewSimpleMempool(cfg.MaxTxs, cfg.MaxBytes)
}
