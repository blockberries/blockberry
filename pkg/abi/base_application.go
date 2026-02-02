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

// Info returns basic application metadata.
func (app *BaseApplication) Info() ApplicationInfo {
	return ApplicationInfo{
		Name:    "base",
		Version: "0.0.0",
	}
}

// InitChain accepts genesis initialization.
func (app *BaseApplication) InitChain(genesis *Genesis) error {
	return nil
}

// CheckTx rejects all transactions by default (fail-closed).
func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
	return &TxCheckResult{
		Code:  CodeNotAuthorized,
		Error: errors.New("CheckTx not implemented: application must override this method"),
	}
}

// BeginBlock is a no-op by default.
func (app *BaseApplication) BeginBlock(ctx context.Context, header *BlockHeader) error {
	return nil
}

// ExecuteTx rejects all transactions by default (fail-closed).
func (app *BaseApplication) ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult {
	return &TxExecResult{
		Code:  CodeNotAuthorized,
		Error: errors.New("ExecuteTx not implemented: application must override this method"),
	}
}

// EndBlock returns an empty result by default.
func (app *BaseApplication) EndBlock(ctx context.Context) *EndBlockResult {
	return &EndBlockResult{}
}

// Commit returns an empty result by default.
// Applications MUST override this to return a proper app hash.
func (app *BaseApplication) Commit(ctx context.Context) *CommitResult {
	return &CommitResult{}
}

// Query rejects all queries by default (fail-closed).
func (app *BaseApplication) Query(ctx context.Context, req *QueryRequest) *QueryResponse {
	return &QueryResponse{
		Code:  CodeNotAuthorized,
		Error: errors.New("Query not implemented: application must override this method"),
	}
}

// AcceptAllApplication is a test helper that accepts everything.
// DO NOT use in production.
type AcceptAllApplication struct{}

// Ensure AcceptAllApplication implements Application.
var _ Application = (*AcceptAllApplication)(nil)

// Info returns test application metadata.
func (app *AcceptAllApplication) Info() ApplicationInfo {
	return ApplicationInfo{
		Name:    "accept-all",
		Version: "0.0.0",
	}
}

// InitChain accepts genesis initialization.
func (app *AcceptAllApplication) InitChain(genesis *Genesis) error {
	return nil
}

// CheckTx accepts all transactions.
func (app *AcceptAllApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
	return &TxCheckResult{Code: CodeOK}
}

// BeginBlock is a no-op.
func (app *AcceptAllApplication) BeginBlock(ctx context.Context, header *BlockHeader) error {
	return nil
}

// ExecuteTx accepts all transactions.
func (app *AcceptAllApplication) ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult {
	return &TxExecResult{Code: CodeOK}
}

// EndBlock returns an empty result.
func (app *AcceptAllApplication) EndBlock(ctx context.Context) *EndBlockResult {
	return &EndBlockResult{}
}

// Commit returns an empty result.
func (app *AcceptAllApplication) Commit(ctx context.Context) *CommitResult {
	return &CommitResult{}
}

// Query returns empty results.
func (app *AcceptAllApplication) Query(ctx context.Context, req *QueryRequest) *QueryResponse {
	return &QueryResponse{Code: CodeOK}
}
