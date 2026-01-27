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
type Application interface {
	// Info returns metadata about the application.
	// Called on startup to verify application state matches the blockchain.
	Info() ApplicationInfo

	// InitChain is called once at genesis to initialize the application.
	// The genesis parameter contains initial validators and app-specific state.
	InitChain(genesis *Genesis) error

	// CheckTx validates a transaction for inclusion in the mempool.
	// This is a lightweight check before block execution.
	// Returns a result indicating whether the transaction should be accepted.
	//
	// CheckTx may be called concurrently from multiple goroutines.
	// The implementation must be thread-safe.
	CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult

	// BeginBlock is called at the start of block processing.
	// The application should prepare for transaction execution.
	//
	// The header contains block metadata including height, time, and proposer.
	// Evidence of validator misbehavior is also included.
	BeginBlock(ctx context.Context, header *BlockHeader) error

	// ExecuteTx executes a transaction during block processing.
	// Called once for each transaction in the block, in order.
	//
	// Unlike CheckTx, this must update application state.
	// Returns the execution result including events and state changes.
	ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult

	// EndBlock is called after all transactions have been processed.
	// The application can return validator updates and consensus parameter changes.
	EndBlock(ctx context.Context) *EndBlockResult

	// Commit finalizes the block and persists state changes.
	// Returns the new application state hash.
	//
	// After Commit returns, the state changes are permanent and the
	// application should be ready for the next block.
	Commit(ctx context.Context) *CommitResult

	// Query reads application state.
	// This can be called at any time and should not modify state.
	//
	// Query may be called concurrently from multiple goroutines.
	// The implementation must be thread-safe.
	Query(ctx context.Context, req *QueryRequest) *QueryResponse
}
