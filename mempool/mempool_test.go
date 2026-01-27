package mempool

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/types"
)

func TestNewMempool(t *testing.T) {
	cfg := config.MempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024 * 1024,
	}

	mp := NewMempool(cfg)
	require.NotNil(t, mp)
	require.Equal(t, 0, mp.Size())
	require.Equal(t, int64(0), mp.SizeBytes())
}

func TestDefaultTxValidator(t *testing.T) {
	// DefaultTxValidator should reject all transactions
	err := DefaultTxValidator([]byte("tx"))
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrNoTxValidator)
}

func TestAcceptAllTxValidator(t *testing.T) {
	// AcceptAllTxValidator should accept all transactions
	err := AcceptAllTxValidator([]byte("tx"))
	require.NoError(t, err)
}

func TestSimpleMempool_AddTx(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator) // Set validator for testing

	t.Run("adds transaction", func(t *testing.T) {
		tx := []byte("transaction 1")
		err := mp.AddTx(tx)
		require.NoError(t, err)
		require.Equal(t, 1, mp.Size())
		require.Equal(t, int64(len(tx)), mp.SizeBytes())
	})

	t.Run("rejects duplicate", func(t *testing.T) {
		tx := []byte("transaction 1")
		err := mp.AddTx(tx)
		require.ErrorIs(t, err, types.ErrTxAlreadyExists)
		require.Equal(t, 1, mp.Size())
	})

	t.Run("rejects nil transaction", func(t *testing.T) {
		err := mp.AddTx(nil)
		require.ErrorIs(t, err, types.ErrInvalidTx)
	})

	t.Run("adds multiple transactions", func(t *testing.T) {
		for i := 2; i <= 10; i++ {
			tx := []byte(fmt.Sprintf("transaction %d", i))
			err := mp.AddTx(tx)
			require.NoError(t, err)
		}
		require.Equal(t, 10, mp.Size())
	})
}

func TestSimpleMempool_MaxTxs(t *testing.T) {
	mp := NewSimpleMempool(5, 0) // Max 5 txs, no byte limit
	mp.SetTxValidator(AcceptAllTxValidator)

	for i := 0; i < 5; i++ {
		err := mp.AddTx([]byte{byte(i)})
		require.NoError(t, err)
	}

	err := mp.AddTx([]byte{5})
	require.ErrorIs(t, err, types.ErrMempoolFull)
	require.Equal(t, 5, mp.Size())
}

func TestSimpleMempool_MaxBytes(t *testing.T) {
	mp := NewSimpleMempool(0, 100) // No tx limit, max 100 bytes
	mp.SetTxValidator(AcceptAllTxValidator)

	// Add 90 bytes
	err := mp.AddTx(make([]byte, 90))
	require.NoError(t, err)

	// Adding 20 more bytes would exceed limit
	err = mp.AddTx(make([]byte, 20))
	require.ErrorIs(t, err, types.ErrMempoolFull)

	// Adding 10 bytes should work
	err = mp.AddTx(make([]byte, 10))
	require.NoError(t, err)
	require.Equal(t, int64(100), mp.SizeBytes())
}

func TestSimpleMempool_HasTx(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	tx := []byte("test transaction")
	hash := types.HashTx(tx)

	require.False(t, mp.HasTx(hash))

	require.NoError(t, mp.AddTx(tx))
	require.True(t, mp.HasTx(hash))

	unknownHash := types.HashBytes([]byte("unknown"))
	require.False(t, mp.HasTx(unknownHash))
}

func TestSimpleMempool_GetTx(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	tx := []byte("test transaction")
	hash := types.HashTx(tx)

	t.Run("returns error for non-existent", func(t *testing.T) {
		_, err := mp.GetTx(hash)
		require.ErrorIs(t, err, types.ErrTxNotFound)
	})

	require.NoError(t, mp.AddTx(tx))

	t.Run("returns existing transaction", func(t *testing.T) {
		retrieved, err := mp.GetTx(hash)
		require.NoError(t, err)
		require.Equal(t, tx, retrieved)
	})
}

func TestSimpleMempool_RemoveTxs(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	// Add some transactions
	txs := make([][]byte, 5)
	hashes := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		txs[i] = []byte(fmt.Sprintf("tx%d", i))
		hashes[i] = types.HashTx(txs[i])
		require.NoError(t, mp.AddTx(txs[i]))
	}

	require.Equal(t, 5, mp.Size())

	t.Run("removes specified transactions", func(t *testing.T) {
		mp.RemoveTxs([][]byte{hashes[0], hashes[2]})
		require.Equal(t, 3, mp.Size())
		require.False(t, mp.HasTx(hashes[0]))
		require.True(t, mp.HasTx(hashes[1]))
		require.False(t, mp.HasTx(hashes[2]))
		require.True(t, mp.HasTx(hashes[3]))
		require.True(t, mp.HasTx(hashes[4]))
	})

	t.Run("ignores non-existent hashes", func(t *testing.T) {
		unknownHash := types.HashBytes([]byte("unknown"))
		mp.RemoveTxs([][]byte{unknownHash})
		require.Equal(t, 3, mp.Size())
	})

	t.Run("updates size bytes", func(t *testing.T) {
		sizeBefore := mp.SizeBytes()
		mp.RemoveTxs([][]byte{hashes[1]})
		require.Less(t, mp.SizeBytes(), sizeBefore)
	})
}

func TestSimpleMempool_ReapTxs(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	// Add transactions in order
	for i := 0; i < 5; i++ {
		tx := []byte(fmt.Sprintf("tx%d_data", i))
		require.NoError(t, mp.AddTx(tx))
	}

	t.Run("returns transactions in insertion order", func(t *testing.T) {
		txs := mp.ReapTxs(0) // No limit
		require.Len(t, txs, 5)
		for i, tx := range txs {
			expected := []byte(fmt.Sprintf("tx%d_data", i))
			require.Equal(t, expected, tx)
		}
	})

	t.Run("respects max bytes limit", func(t *testing.T) {
		// Each tx is about 8 bytes, get first 2
		txs := mp.ReapTxs(16)
		require.Len(t, txs, 2)
	})

	t.Run("transactions remain in mempool", func(t *testing.T) {
		require.Equal(t, 5, mp.Size())
	})
}

func TestSimpleMempool_Flush(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	for i := 0; i < 10; i++ {
		require.NoError(t, mp.AddTx([]byte{byte(i)}))
	}

	require.Equal(t, 10, mp.Size())
	require.Greater(t, mp.SizeBytes(), int64(0))

	mp.Flush()

	require.Equal(t, 0, mp.Size())
	require.Equal(t, int64(0), mp.SizeBytes())
}

func TestSimpleMempool_TxHashes(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	// Add transactions
	expectedHashes := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		tx := []byte{byte(i)}
		expectedHashes[i] = types.HashTx(tx)
		require.NoError(t, mp.AddTx(tx))
	}

	hashes := mp.TxHashes()
	require.Len(t, hashes, 5)

	for i, h := range hashes {
		require.Equal(t, expectedHashes[i], h)
	}
}

func TestSimpleMempool_Concurrent(t *testing.T) {
	mp := NewSimpleMempool(1000, 10*1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	const numGoroutines = 10
	const txsPerGoroutine = 50

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*txsPerGoroutine)

	// Concurrent adds
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < txsPerGoroutine; i++ {
				tx := []byte(fmt.Sprintf("g%d_tx%d", goroutineID, i))
				if err := mp.AddTx(tx); err != nil {
					errors <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}

	require.Equal(t, numGoroutines*txsPerGoroutine, mp.Size())

	// Concurrent reads
	wg = sync.WaitGroup{}
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < txsPerGoroutine; i++ {
				tx := []byte(fmt.Sprintf("g%d_tx%d", goroutineID, i))
				hash := types.HashTx(tx)
				if !mp.HasTx(hash) {
					t.Errorf("tx not found: g%d_tx%d", goroutineID, i)
				}
			}
		}(g)
	}

	wg.Wait()
}

func TestSimpleMempool_InsertionOrder(t *testing.T) {
	mp := NewSimpleMempool(100, 1024*1024)
	mp.SetTxValidator(AcceptAllTxValidator)

	// Add in specific order
	order := []string{"first", "second", "third", "fourth", "fifth"}
	for _, s := range order {
		require.NoError(t, mp.AddTx([]byte(s)))
	}

	// Remove middle one
	mp.RemoveTxs([][]byte{types.HashTx([]byte("third"))})

	// Reap should maintain order (minus removed)
	txs := mp.ReapTxs(0)
	require.Len(t, txs, 4)
	require.Equal(t, []byte("first"), txs[0])
	require.Equal(t, []byte("second"), txs[1])
	require.Equal(t, []byte("fourth"), txs[2])
	require.Equal(t, []byte("fifth"), txs[3])
}

func BenchmarkSimpleMempool_AddTx(b *testing.B) {
	mp := NewSimpleMempool(b.N+1, int64(b.N*100))
	mp.SetTxValidator(AcceptAllTxValidator)

	txs := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		txs[i] = []byte(fmt.Sprintf("transaction_%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mp.AddTx(txs[i])
	}
}
