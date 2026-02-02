package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/abi"
)

func TestNullApplication_Creation(t *testing.T) {
	app := NewNullApplication()
	require.NotNil(t, app)
	require.Equal(t, uint64(0), app.LastBlockHeight)
	require.Nil(t, app.LastBlockHash)
	require.Len(t, app.AppHash, 32)
}

func TestNullApplication_Info(t *testing.T) {
	app := NewNullApplication()
	info := app.Info()
	assert.Equal(t, "null-application", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestNullApplication_CheckTx(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Should accept any transaction
	result := app.CheckTx(ctx, &abi.Transaction{Data: nil})
	assert.True(t, result.IsOK())

	result = app.CheckTx(ctx, &abi.Transaction{Data: []byte{}})
	assert.True(t, result.IsOK())

	result = app.CheckTx(ctx, &abi.Transaction{Data: []byte("valid transaction")})
	assert.True(t, result.IsOK())
}

func TestNullApplication_BlockFlow(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Begin block
	height := uint64(100)
	prevHash := []byte("prevhash")
	err := app.BeginBlock(ctx, &abi.BlockHeader{Height: height, PrevHash: prevHash})
	require.NoError(t, err)
	require.Equal(t, height, app.LastBlockHeight)
	require.Equal(t, prevHash, app.LastBlockHash)

	// Execute transactions
	for i := 0; i < 10; i++ {
		tx := &abi.Transaction{Data: []byte{byte(i)}}
		result := app.ExecuteTx(ctx, tx)
		assert.True(t, result.IsOK())
	}

	// End block
	endResult := app.EndBlock(ctx)
	assert.NotNil(t, endResult)

	// Commit
	commitResult := app.Commit(ctx)
	assert.NotNil(t, commitResult)
	assert.Equal(t, app.AppHash, commitResult.AppHash)
}

func TestNullApplication_Query(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Query should return OK
	result := app.Query(ctx, &abi.QueryRequest{Path: "path", Data: []byte("data")})
	assert.True(t, result.IsOK())

	// Empty query should also work
	result = app.Query(ctx, &abi.QueryRequest{Path: "", Data: nil})
	assert.True(t, result.IsOK())
}

func TestNullApplication_ImplementsInterface(t *testing.T) {
	// This test verifies that NullApplication implements abi.Application
	var app abi.Application = NewNullApplication()
	require.NotNil(t, app)
}

func TestNullApplication_MultipleBlocks(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Process multiple blocks
	for height := uint64(1); height <= 5; height++ {
		prevHash := []byte{byte(height)}

		err := app.BeginBlock(ctx, &abi.BlockHeader{Height: height, PrevHash: prevHash})
		require.NoError(t, err)

		// Execute a transaction per block
		result := app.ExecuteTx(ctx, &abi.Transaction{Data: []byte{byte(height)}})
		assert.True(t, result.IsOK())

		endResult := app.EndBlock(ctx)
		assert.NotNil(t, endResult)

		commitResult := app.Commit(ctx)
		assert.NotNil(t, commitResult)

		// State should be updated
		require.Equal(t, height, app.LastBlockHeight)
		require.Equal(t, prevHash, app.LastBlockHash)
	}
}

func TestNullApplication_InitChain(t *testing.T) {
	app := NewNullApplication()
	err := app.InitChain(&abi.Genesis{ChainID: "test-chain"})
	require.NoError(t, err)
}
