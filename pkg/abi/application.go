package abi

import "context"

// Application is the main interface applications must implement.
// This is the primary contract between the blockchain framework and application logic.
//
// All methods are invoked by the framework; applications respond.
// Methods are called in a specific order during block execution:
//
//  1. BeginBlock - Called once at the start of block processing
//  2. ExecuteTx - Called for each transaction in the block (in order)
//  3. EndBlock - Called once after all transactions are processed
//  4. Commit - Called to finalize and persist state changes
//
// CheckTx and Query can be called at any time and may run concurrently.
//
// Transactions are treated as opaque bytes. The framework handles ordering
// and finality; the application handles interpretation and execution.
type Application interface {
	// InitChain is called once at genesis to initialize the application.
	// validators contains the initial validator set, appState contains
	// application-specific genesis state (opaque to framework).
	InitChain(ctx context.Context, validators []Validator, appState []byte) error

	// CheckTx validates a transaction for inclusion in the mempool.
	// This is a lightweight check before block execution.
	// Returns nil if the transaction should be accepted, or an error
	// describing why it is invalid.
	//
	// CheckTx may be called concurrently from multiple goroutines.
	// The implementation must be thread-safe.
	CheckTx(ctx context.Context, tx []byte) error

	// BeginBlock is called at the start of block processing.
	// The application should prepare for transaction execution.
	//
	// The header contains block metadata including height, time, and proposer.
	BeginBlock(ctx context.Context, header *BlockHeader) error

	// ExecuteTx executes a transaction during block processing.
	// Called once for each transaction in the block, in order.
	//
	// Unlike CheckTx, this must update application state.
	// Returns the execution result including events and gas usage.
	ExecuteTx(ctx context.Context, tx []byte) (*TxResult, error)

	// EndBlock is called after all transactions have been processed.
	// The application can return validator updates and consensus parameter changes.
	EndBlock(ctx context.Context) (*EndBlockResult, error)

	// Commit finalizes the block and persists state changes.
	// Returns the new application state hash.
	//
	// After Commit returns, the state changes are permanent and the
	// application should be ready for the next block.
	Commit(ctx context.Context) (*CommitResult, error)

	// Query reads application state.
	// This can be called at any time and should not modify state.
	// The height parameter specifies which historical state to query (0 = latest).
	//
	// Query may be called concurrently from multiple goroutines.
	// The implementation must be thread-safe.
	Query(ctx context.Context, path string, data []byte, height int64) (*QueryResult, error)
}
