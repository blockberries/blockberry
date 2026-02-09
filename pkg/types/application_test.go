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
	require.Equal(t, int64(0), app.LastBlockHeight)
	require.Nil(t, app.LastBlockHash)
	require.Len(t, app.AppHash, 32)
}

func TestNullApplication_CheckTx(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Should accept any transaction
	err := app.CheckTx(ctx, nil)
	assert.NoError(t, err)

	err = app.CheckTx(ctx, []byte{})
	assert.NoError(t, err)

	err = app.CheckTx(ctx, []byte("valid transaction"))
	assert.NoError(t, err)
}

func TestNullApplication_BlockFlow(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Begin block
	height := int64(100)
	prevHash := []byte("prevhash")
	err := app.BeginBlock(ctx, &abi.BlockHeader{Height: height, LastBlockHash: prevHash})
	require.NoError(t, err)
	require.Equal(t, height, app.LastBlockHeight)
	require.Equal(t, prevHash, app.LastBlockHash)

	// Execute transactions
	for i := 0; i < 10; i++ {
		result, err := app.ExecuteTx(ctx, []byte{byte(i)})
		require.NoError(t, err)
		assert.True(t, result.IsSuccess())
	}

	// End block
	endResult, err := app.EndBlock(ctx)
	require.NoError(t, err)
	assert.NotNil(t, endResult)

	// Commit
	commitResult, err := app.Commit(ctx)
	require.NoError(t, err)
	assert.NotNil(t, commitResult)
	assert.Equal(t, app.AppHash, commitResult.AppHash)
}

func TestNullApplication_Query(t *testing.T) {
	app := NewNullApplication()
	ctx := context.Background()

	// Query should return OK
	result, err := app.Query(ctx, "path", []byte("data"), 0)
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())

	// Empty query should also work
	result, err = app.Query(ctx, "", nil, 0)
	require.NoError(t, err)
	assert.True(t, result.IsSuccess())
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
	for height := int64(1); height <= 5; height++ {
		prevHash := []byte{byte(height)}

		err := app.BeginBlock(ctx, &abi.BlockHeader{Height: height, LastBlockHash: prevHash})
		require.NoError(t, err)

		// Execute a transaction per block
		result, err := app.ExecuteTx(ctx, []byte{byte(height)})
		require.NoError(t, err)
		assert.True(t, result.IsSuccess())

		endResult, err := app.EndBlock(ctx)
		require.NoError(t, err)
		assert.NotNil(t, endResult)

		commitResult, err := app.Commit(ctx)
		require.NoError(t, err)
		assert.NotNil(t, commitResult)

		// State should be updated
		require.Equal(t, height, app.LastBlockHeight)
		require.Equal(t, prevHash, app.LastBlockHash)
	}
}

func TestNullApplication_InitChain(t *testing.T) {
	app := NewNullApplication()
	err := app.InitChain(context.Background(), nil, nil)
	require.NoError(t, err)
}
