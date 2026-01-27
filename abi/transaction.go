package abi

import (
	"crypto/sha256"
	"errors"
	"time"
)

// Transaction represents a blockchain transaction.
// The Data field is opaque to the framework; only the application interprets it.
type Transaction struct {
	// Hash is the SHA-256 hash of Data. Computed automatically if empty.
	Hash []byte

	// Data is the raw transaction bytes (opaque to framework).
	Data []byte

	// Sender is the optional extracted sender address (for deduplication).
	Sender []byte

	// Nonce is the optional sequence number (for ordering).
	Nonce uint64

	// GasLimit is the optional maximum computation units allowed.
	GasLimit uint64

	// GasPrice is the optional price per computation unit.
	GasPrice uint64

	// Priority is the derived priority for ordering (higher = more important).
	Priority int64
}

// ComputeHash computes and sets the transaction hash if not already set.
func (tx *Transaction) ComputeHash() []byte {
	if len(tx.Hash) == 0 && len(tx.Data) > 0 {
		h := sha256.Sum256(tx.Data)
		tx.Hash = h[:]
	}
	return tx.Hash
}

// ValidateBasic performs basic validation of the transaction structure.
func (tx *Transaction) ValidateBasic() error {
	if len(tx.Data) == 0 {
		return errors.New("transaction data is empty")
	}
	return nil
}

// Size returns the size of the transaction in bytes.
func (tx *Transaction) Size() int {
	return len(tx.Data)
}

// CheckTxMode indicates the context of transaction validation.
type CheckTxMode int

const (
	// CheckTxNew indicates a new transaction from a client or peer.
	CheckTxNew CheckTxMode = iota

	// CheckTxRecheck indicates re-validation after a block commit.
	CheckTxRecheck

	// CheckTxRecovery indicates validation during crash recovery.
	CheckTxRecovery
)

// String returns a human-readable description of the mode.
func (m CheckTxMode) String() string {
	switch m {
	case CheckTxNew:
		return "New"
	case CheckTxRecheck:
		return "Recheck"
	case CheckTxRecovery:
		return "Recovery"
	default:
		return "Unknown"
	}
}

// TxCheckResult is returned from Application.CheckTx.
// It indicates whether a transaction should be accepted into the mempool.
type TxCheckResult struct {
	// Code indicates success (0) or failure (non-zero).
	Code ResultCode

	// Error provides a human-readable error message if Code != 0.
	Error error

	// GasWanted is the estimated gas needed for execution.
	GasWanted uint64

	// Priority is the transaction priority for ordering in the mempool.
	Priority int64

	// Sender is the extracted sender address (for mempool deduplication).
	Sender []byte

	// Nonce is the extracted nonce (for ordering same-sender transactions).
	Nonce uint64

	// Data is optional modified transaction data.
	Data []byte

	// RecheckAfter suggests re-checking after this block height.
	RecheckAfter uint64

	// ExpireAfter suggests expiration after this duration.
	ExpireAfter time.Duration
}

// IsOK returns true if the check succeeded.
func (r *TxCheckResult) IsOK() bool {
	return r != nil && r.Code.IsOK()
}

// TxExecResult is returned from Application.ExecuteTx during block execution.
type TxExecResult struct {
	// Code indicates success (0) or failure (non-zero).
	Code ResultCode

	// Error provides a human-readable error message if Code != 0.
	Error error

	// GasUsed is the actual gas consumed during execution.
	GasUsed uint64

	// Events are the events emitted during execution.
	Events []Event

	// Data is optional return data from execution.
	Data []byte
}

// IsOK returns true if the execution succeeded.
func (r *TxExecResult) IsOK() bool {
	return r != nil && r.Code.IsOK()
}
