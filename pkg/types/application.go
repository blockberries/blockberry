// Package types provides common types used throughout blockberry.
//
// The Application interface has been moved to the abi package.
// Use abi.Application for all application implementations.
package types

// TxValidator is a function that validates transactions.
// Used by the mempool to check transactions before accepting them.
//
// Deprecated: Use abi.TxCheckResult-based validation instead.
// This type is kept for compatibility with existing mempool implementations.
type TxValidator func(tx []byte) error

// BlockValidator is a function that validates blocks.
// Used by sync and block reactors to verify blocks before storing.
type BlockValidator func(height int64, hash, data []byte) error

// AcceptAllBlockValidator is a BlockValidator that accepts all blocks without validation.
// This is useful for testing and development, but should NOT be used in production.
//
// Warning: Using this validator means blocks are not validated before being stored,
// which can lead to accepting invalid blocks.
var AcceptAllBlockValidator BlockValidator = func(height int64, hash, data []byte) error {
	return nil
}
