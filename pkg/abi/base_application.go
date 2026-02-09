package abi

import (
	"context"
	"errors"
)

// BaseApplication provides default fail-closed implementations of the Application interface.
// Applications can embed this to get safe defaults and only override methods they need.
//
// SECURITY: All methods default to rejecting operations. This follows the fail-closed
// security principle - the system is secure by default and requires explicit enabling.
type BaseApplication struct{}

// Ensure BaseApplication implements Application.
var _ Application = (*BaseApplication)(nil)

// InitChain accepts genesis initialization.
func (app *BaseApplication) InitChain(ctx context.Context, validators []Validator, appState []byte) error {
	return nil
}

// CheckTx rejects all transactions by default (fail-closed).
func (app *BaseApplication) CheckTx(ctx context.Context, tx []byte) error {
	return errors.New("CheckTx not implemented: application must override this method")
}

// BeginBlock is a no-op by default.
func (app *BaseApplication) BeginBlock(ctx context.Context, header *BlockHeader) error {
	return nil
}

// ExecuteTx rejects all transactions by default (fail-closed).
func (app *BaseApplication) ExecuteTx(ctx context.Context, tx []byte) (*TxResult, error) {
	return nil, errors.New("ExecuteTx not implemented: application must override this method")
}

// EndBlock returns an empty result by default.
func (app *BaseApplication) EndBlock(ctx context.Context) (*EndBlockResult, error) {
	return &EndBlockResult{}, nil
}

// Commit returns an empty result by default.
// Applications MUST override this to return a proper app hash.
func (app *BaseApplication) Commit(ctx context.Context) (*CommitResult, error) {
	return &CommitResult{}, nil
}

// Query rejects all queries by default (fail-closed).
func (app *BaseApplication) Query(ctx context.Context, path string, data []byte, height int64) (*QueryResult, error) {
	return nil, errors.New("Query not implemented: application must override this method")
}

// AcceptAllApplication is a test helper that accepts everything.
// DO NOT use in production.
type AcceptAllApplication struct{}

// Ensure AcceptAllApplication implements Application.
var _ Application = (*AcceptAllApplication)(nil)

// InitChain accepts genesis initialization.
func (app *AcceptAllApplication) InitChain(ctx context.Context, validators []Validator, appState []byte) error {
	return nil
}

// CheckTx accepts all transactions.
func (app *AcceptAllApplication) CheckTx(ctx context.Context, tx []byte) error {
	return nil
}

// BeginBlock is a no-op.
func (app *AcceptAllApplication) BeginBlock(ctx context.Context, header *BlockHeader) error {
	return nil
}

// ExecuteTx accepts all transactions.
func (app *AcceptAllApplication) ExecuteTx(ctx context.Context, tx []byte) (*TxResult, error) {
	return &TxResult{Code: 0}, nil
}

// EndBlock returns an empty result.
func (app *AcceptAllApplication) EndBlock(ctx context.Context) (*EndBlockResult, error) {
	return &EndBlockResult{}, nil
}

// Commit returns an empty result.
func (app *AcceptAllApplication) Commit(ctx context.Context) (*CommitResult, error) {
	return &CommitResult{}, nil
}

// Query returns empty results.
func (app *AcceptAllApplication) Query(ctx context.Context, path string, data []byte, height int64) (*QueryResult, error) {
	return &QueryResult{Code: 0}, nil
}
