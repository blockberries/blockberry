package types

import (
	"context"

	"github.com/blockberries/blockberry/pkg/abi"
)

// NullApplication is a no-op implementation of abi.Application.
// It accepts all transactions and returns empty results.
// Useful for testing and as a starting point for new applications.
type NullApplication struct {
	// LastBlockHeight tracks the last committed block height.
	LastBlockHeight uint64

	// LastBlockHash stores the last committed block hash.
	LastBlockHash []byte

	// AppHash is the current application hash (returned by Commit).
	AppHash []byte
}

// NewNullApplication creates a new NullApplication.
func NewNullApplication() *NullApplication {
	return &NullApplication{
		AppHash: make([]byte, 32), // Empty 32-byte hash
	}
}

// Ensure NullApplication implements abi.Application.
var _ abi.Application = (*NullApplication)(nil)

// Info returns application metadata.
func (app *NullApplication) Info() abi.ApplicationInfo {
	return abi.ApplicationInfo{
		Name:    "null-application",
		Version: "1.0.0",
		AppHash: app.AppHash,
		Height:  app.LastBlockHeight,
	}
}

// InitChain initializes the chain. Always returns nil.
func (app *NullApplication) InitChain(genesis *abi.Genesis) error {
	return nil
}

// CheckTx validates a transaction. Always accepts (returns CodeOK).
func (app *NullApplication) CheckTx(ctx context.Context, tx *abi.Transaction) *abi.TxCheckResult {
	return &abi.TxCheckResult{Code: abi.CodeOK}
}

// BeginBlock starts processing a new block.
func (app *NullApplication) BeginBlock(ctx context.Context, header *abi.BlockHeader) error {
	app.LastBlockHeight = header.Height
	app.LastBlockHash = header.PrevHash
	return nil
}

// ExecuteTx executes a transaction. Always succeeds (returns CodeOK).
func (app *NullApplication) ExecuteTx(ctx context.Context, tx *abi.Transaction) *abi.TxExecResult {
	return &abi.TxExecResult{Code: abi.CodeOK}
}

// EndBlock ends block processing.
func (app *NullApplication) EndBlock(ctx context.Context) *abi.EndBlockResult {
	return &abi.EndBlockResult{}
}

// Commit persists the application state and returns the app hash.
func (app *NullApplication) Commit(ctx context.Context) *abi.CommitResult {
	return &abi.CommitResult{AppHash: app.AppHash}
}

// Query performs a read-only query. Always returns empty result.
func (app *NullApplication) Query(ctx context.Context, req *abi.QueryRequest) *abi.QueryResponse {
	return &abi.QueryResponse{Code: abi.CodeOK}
}
