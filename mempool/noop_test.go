package mempool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

func TestNoOpMempool_InterfaceCompliance(t *testing.T) {
	var _ Mempool = (*NoOpMempool)(nil)
}

func TestNewNoOpMempool(t *testing.T) {
	mp := NewNoOpMempool()
	require.NotNil(t, mp)
}

func TestNoOpMempool_AddTx(t *testing.T) {
	mp := NewNoOpMempool()
	err := mp.AddTx([]byte("test tx"))
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrMempoolClosed)
}

func TestNoOpMempool_RemoveTxs(t *testing.T) {
	mp := NewNoOpMempool()
	// Should not panic
	mp.RemoveTxs([][]byte{{1, 2, 3}})
}

func TestNoOpMempool_ReapTxs(t *testing.T) {
	mp := NewNoOpMempool()
	txs := mp.ReapTxs(1000)
	assert.Nil(t, txs)
}

func TestNoOpMempool_HasTx(t *testing.T) {
	mp := NewNoOpMempool()
	assert.False(t, mp.HasTx([]byte{1, 2, 3}))
}

func TestNoOpMempool_GetTx(t *testing.T) {
	mp := NewNoOpMempool()
	tx, err := mp.GetTx([]byte{1, 2, 3})
	assert.Nil(t, tx)
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrTxNotFound)
}

func TestNoOpMempool_Size(t *testing.T) {
	mp := NewNoOpMempool()
	assert.Equal(t, 0, mp.Size())
}

func TestNoOpMempool_SizeBytes(t *testing.T) {
	mp := NewNoOpMempool()
	assert.Equal(t, int64(0), mp.SizeBytes())
}

func TestNoOpMempool_Flush(t *testing.T) {
	mp := NewNoOpMempool()
	// Should not panic
	mp.Flush()
}

func TestNoOpMempool_TxHashes(t *testing.T) {
	mp := NewNoOpMempool()
	hashes := mp.TxHashes()
	assert.Nil(t, hashes)
}

func TestNoOpMempool_SetTxValidator(t *testing.T) {
	mp := NewNoOpMempool()
	// Should not panic
	mp.SetTxValidator(AcceptAllTxValidator)
}
