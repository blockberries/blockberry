package types

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
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

	// Should accept any transaction
	err := app.CheckTx(nil)
	require.NoError(t, err)

	err = app.CheckTx([]byte{})
	require.NoError(t, err)

	err = app.CheckTx([]byte("valid transaction"))
	require.NoError(t, err)
}

func TestNullApplication_BlockFlow(t *testing.T) {
	app := NewNullApplication()

	// Begin block
	height := int64(100)
	hash := []byte("blockhash")
	err := app.BeginBlock(height, hash)
	require.NoError(t, err)
	require.Equal(t, height, app.LastBlockHeight)
	require.Equal(t, hash, app.LastBlockHash)

	// Deliver transactions
	for i := 0; i < 10; i++ {
		tx := []byte{byte(i)}
		deliverErr := app.DeliverTx(tx)
		require.NoError(t, deliverErr)
	}

	// End block
	err = app.EndBlock()
	require.NoError(t, err)

	// Commit
	appHash, err := app.Commit()
	require.NoError(t, err)
	require.Equal(t, app.AppHash, appHash)
}

func TestNullApplication_Query(t *testing.T) {
	app := NewNullApplication()

	// Query should return nil
	result, err := app.Query("path", []byte("data"))
	require.NoError(t, err)
	require.Nil(t, result)

	// Empty query should also work
	result, err = app.Query("", nil)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestNullApplication_HandleConsensusMessage(t *testing.T) {
	app := NewNullApplication()

	// Should accept any message
	err := app.HandleConsensusMessage(peer.ID("peer1"), []byte("consensus data"))
	require.NoError(t, err)

	err = app.HandleConsensusMessage(peer.ID("peer2"), nil)
	require.NoError(t, err)
}

func TestNullApplication_ImplementsInterface(t *testing.T) {
	// This test verifies that NullApplication implements Application
	var app Application = NewNullApplication()
	require.NotNil(t, app)
}

func TestNullApplication_MultipleBlocks(t *testing.T) {
	app := NewNullApplication()

	// Process multiple blocks
	for height := int64(1); height <= 5; height++ {
		hash := []byte{byte(height)}

		err := app.BeginBlock(height, hash)
		require.NoError(t, err)

		// Deliver a transaction per block
		err = app.DeliverTx([]byte{byte(height)})
		require.NoError(t, err)

		err = app.EndBlock()
		require.NoError(t, err)

		appHash, err := app.Commit()
		require.NoError(t, err)
		require.NotNil(t, appHash)

		// State should be updated
		require.Equal(t, height, app.LastBlockHeight)
		require.Equal(t, hash, app.LastBlockHash)
	}
}
