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
	LastBlockHeight int64

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


// InitChain initializes the chain. Always returns nil.
func (app *NullApplication) InitChain(ctx context.Context, validators []abi.Validator, appState []byte) error {
	return nil
}

// CheckTx validates a transaction. Always accepts (returns nil).
func (app *NullApplication) CheckTx(ctx context.Context, tx []byte) error {
	return nil
}

// BeginBlock starts processing a new block.
func (app *NullApplication) BeginBlock(ctx context.Context, header *abi.BlockHeader) error {
	app.LastBlockHeight = header.Height
	app.LastBlockHash = header.LastBlockHash
	return nil
}

// ExecuteTx executes a transaction. Always succeeds (returns Code 0).
func (app *NullApplication) ExecuteTx(ctx context.Context, tx []byte) (*abi.TxResult, error) {
	return &abi.TxResult{Code: 0}, nil
}

// EndBlock ends block processing.
func (app *NullApplication) EndBlock(ctx context.Context) (*abi.EndBlockResult, error) {
	return &abi.EndBlockResult{}, nil
}

// Commit persists the application state and returns the app hash.
func (app *NullApplication) Commit(ctx context.Context) (*abi.CommitResult, error) {
	return &abi.CommitResult{AppHash: app.AppHash}, nil
}

// Query performs a read-only query. Always returns empty result.
func (app *NullApplication) Query(ctx context.Context, path string, data []byte, height int64) (*abi.QueryResult, error) {
	return &abi.QueryResult{Code: 0}, nil
}
