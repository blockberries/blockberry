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
