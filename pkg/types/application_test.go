package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/bapi"
	bapitypes "github.com/blockberries/bapi/types"
)

func TestNullApplication_Creation(t *testing.T) {
	app := NewNullApplication()
	require.NotNil(t, app)
	require.Equal(t, bapitypes.AppHash{}, app.AppHash)
}

func TestNullApplication_CheckTx(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Should accept any transaction
	verdict, err := app.CheckTx(ctx, nil, bapitypes.MempoolFirstSeen)
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), verdict.Code)

	verdict, err = app.CheckTx(ctx, bapitypes.Tx{}, bapitypes.MempoolFirstSeen)
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), verdict.Code)

	verdict, err = app.CheckTx(ctx, bapitypes.Tx("valid transaction"), bapitypes.MempoolFirstSeen)
	assert.NoError(t, err)
	assert.Equal(t, uint32(0), verdict.Code)
}

func TestNullApplication_ExecuteBlock(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	block := bapitypes.FinalizedBlock{
		Height: 100,
		Txs:    []bapitypes.Tx{[]byte{0}, []byte{1}, []byte{2}},
	}

	outcome, err := app.ExecuteBlock(ctx, block)
	require.NoError(t, err)
	require.Len(t, outcome.TxOutcomes, 3)
	for i, txOutcome := range outcome.TxOutcomes {
		assert.Equal(t, uint32(i), txOutcome.Index)
		assert.Equal(t, uint32(0), txOutcome.Code)
	}
}

func TestNullApplication_Commit(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	result, err := app.Commit(ctx)
	require.NoError(t, err)
	assert.Equal(t, bapitypes.CommitResult{}, result)
}

func TestNullApplication_Query(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Query should return code 0
	result, err := app.Query(ctx, bapitypes.StateQuery{Path: "/store/key", Data: []byte("data")})
	require.NoError(t, err)
	assert.Equal(t, uint32(0), result.Code)

	// Empty query should also work
	result, err = app.Query(ctx, bapitypes.StateQuery{})
	require.NoError(t, err)
	assert.Equal(t, uint32(0), result.Code)
}

func TestNullApplication_ImplementsInterface(t *testing.T) {
	// This test verifies that NullApplication implements bapi.Lifecycle
	var app bapi.Lifecycle = NewNullApplication()
	require.NotNil(t, app)
}

func TestNullApplication_Handshake(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	resp, err := app.Handshake(ctx, bapitypes.HandshakeRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp.AppHash)
	assert.Equal(t, &app.AppHash, resp.AppHash)
}

func TestNullApplication_MultipleBlocks(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Process multiple blocks
	for height := uint64(1); height <= 5; height++ {
		block := bapitypes.FinalizedBlock{
			Height: height,
			Txs:    []bapitypes.Tx{[]byte{byte(height)}},
		}

		outcome, err := app.ExecuteBlock(ctx, block)
		require.NoError(t, err)
		require.Len(t, outcome.TxOutcomes, 1)
		assert.Equal(t, uint32(0), outcome.TxOutcomes[0].Code)

		result, err := app.Commit(ctx)
		require.NoError(t, err)
		assert.Equal(t, bapitypes.CommitResult{}, result)
	}
}
