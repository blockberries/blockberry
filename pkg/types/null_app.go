package types

import (
	"context"

	"github.com/blockberries/bapi"
	bapitypes "github.com/blockberries/bapi/types"
)

// NullApplication is a no-op implementation of bapi.Lifecycle.
// It accepts all transactions and returns empty results.
// Useful for testing and as a starting point for new applications.
type NullApplication struct {
	// AppHash is the current application hash.
	AppHash bapitypes.AppHash
}

// NewNullApplication creates a new NullApplication.
func NewNullApplication() *NullApplication {
	return &NullApplication{}
}

// Ensure NullApplication implements bapi.Lifecycle.
var _ bapi.Lifecycle = (*NullApplication)(nil)

// Handshake handles startup handshake. Returns empty response.
func (app *NullApplication) Handshake(_ context.Context, _ bapitypes.HandshakeRequest) (bapitypes.HandshakeResponse, error) {
	return bapitypes.HandshakeResponse{
		AppHash: &app.AppHash,
	}, nil
}

// CheckTx validates a transaction. Always accepts.
func (app *NullApplication) CheckTx(_ context.Context, _ bapitypes.Tx, _ bapitypes.MempoolContext) (bapitypes.GateVerdict, error) {
	return bapitypes.GateVerdict{Code: 0}, nil
}

// ExecuteBlock executes a finalized block. Returns empty outcome with AppHash.
func (app *NullApplication) ExecuteBlock(_ context.Context, block bapitypes.FinalizedBlock) (bapitypes.BlockOutcome, error) {
	outcomes := make([]bapitypes.TxOutcome, len(block.Txs))
	for i := range block.Txs {
		outcomes[i] = bapitypes.TxOutcome{Index: uint32(i), Code: 0}
	}
	return bapitypes.BlockOutcome{
		TxOutcomes: outcomes,
		AppHash:    app.AppHash,
	}, nil
}

// Commit persists the application state. Returns empty result.
func (app *NullApplication) Commit(_ context.Context) (bapitypes.CommitResult, error) {
	return bapitypes.CommitResult{}, nil
}

// Query performs a read-only query. Always returns empty result.
func (app *NullApplication) Query(_ context.Context, _ bapitypes.StateQuery) (bapitypes.StateQueryResult, error) {
	return bapitypes.StateQueryResult{Code: 0}, nil
}
