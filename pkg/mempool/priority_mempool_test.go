package mempool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
)

func TestPriorityMempool_Basic(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024 * 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)
	require.NotNil(t, m)

	// Add a transaction
	tx := []byte("test transaction")
	err := m.AddTx(tx)
	require.NoError(t, err)

	assert.Equal(t, 1, m.Size())
	assert.Equal(t, int64(len(tx)), m.SizeBytes())

	// Check existence
	hash := types.HashTx(tx)
	assert.True(t, m.HasTx(hash))

	// Get transaction
	retrieved, err := m.GetTx(hash)
	require.NoError(t, err)
	assert.Equal(t, tx, retrieved)
}

func TestPriorityMempool_PriorityOrdering(t *testing.T) {
	// Use a priority function that returns the first byte as priority
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       100,
		MaxBytes:     1024 * 1024,
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Add transactions with different priorities (first byte determines priority)
	txLow := []byte{10, 0, 0, 0}   // Priority 10
	txMed := []byte{50, 0, 0, 0}   // Priority 50
	txHigh := []byte{100, 0, 0, 0} // Priority 100

	require.NoError(t, m.AddTx(txLow))
	require.NoError(t, m.AddTx(txMed))
	require.NoError(t, m.AddTx(txHigh))

	// ReapTxs should return highest priority first
	txs := m.ReapTxs(1024)
	require.Len(t, txs, 3)

	// Verify order: highest priority first
	assert.Equal(t, txHigh, txs[0])
	assert.Equal(t, txMed, txs[1])
	assert.Equal(t, txLow, txs[2])
}

func TestPriorityMempool_Eviction(t *testing.T) {
	// Use first byte as priority
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       3,
		MaxBytes:     1024 * 1024,
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Fill the mempool
	tx1 := []byte{10, 1, 1, 1} // Priority 10
	tx2 := []byte{20, 2, 2, 2} // Priority 20
	tx3 := []byte{30, 3, 3, 3} // Priority 30

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))
	require.NoError(t, m.AddTx(tx3))
	assert.Equal(t, 3, m.Size())

	// Add a higher priority transaction - should evict lowest (tx1)
	txNew := []byte{50, 4, 4, 4} // Priority 50
	require.NoError(t, m.AddTx(txNew))
	assert.Equal(t, 3, m.Size())

	// Verify tx1 was evicted
	hash1 := types.HashTx(tx1)
	assert.False(t, m.HasTx(hash1))

	// Verify others still exist
	assert.True(t, m.HasTx(types.HashTx(tx2)))
	assert.True(t, m.HasTx(types.HashTx(tx3)))
	assert.True(t, m.HasTx(types.HashTx(txNew)))
}

func TestPriorityMempool_NoEvictionForLowerPriority(t *testing.T) {
	// Use first byte as priority
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       3,
		MaxBytes:     1024 * 1024,
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Fill with high priority transactions
	tx1 := []byte{80, 1, 1, 1}  // Priority 80
	tx2 := []byte{90, 2, 2, 2}  // Priority 90
	tx3 := []byte{100, 3, 3, 3} // Priority 100

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))
	require.NoError(t, m.AddTx(tx3))

	// Try to add a lower priority transaction - should fail
	txLow := []byte{10, 4, 4, 4} // Priority 10
	err := m.AddTx(txLow)
	assert.ErrorIs(t, err, types.ErrMempoolFull)
	assert.Equal(t, 3, m.Size())

	// All original transactions should still exist
	assert.True(t, m.HasTx(types.HashTx(tx1)))
	assert.True(t, m.HasTx(types.HashTx(tx2)))
	assert.True(t, m.HasTx(types.HashTx(tx3)))
}

func TestPriorityMempool_ByteLimitEviction(t *testing.T) {
	// Use first byte as priority
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       100,
		MaxBytes:     20, // Very small limit
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Add low priority transaction (5 bytes)
	txLow := []byte{10, 1, 2, 3, 4} // Priority 10, 5 bytes
	require.NoError(t, m.AddTx(txLow))

	// Add medium priority transaction (5 bytes)
	txMed := []byte{50, 1, 2, 3, 4} // Priority 50, 5 bytes
	require.NoError(t, m.AddTx(txMed))

	// Add high priority transaction (5 bytes)
	txHigh := []byte{100, 1, 2, 3, 4} // Priority 100, 5 bytes
	require.NoError(t, m.AddTx(txHigh))

	// Now at 15 bytes

	// Add another high priority (should evict lowest to make room)
	txNew := []byte{90, 1, 2, 3, 4, 5, 6} // Priority 90, 7 bytes
	require.NoError(t, m.AddTx(txNew))

	// txLow should have been evicted
	assert.False(t, m.HasTx(types.HashTx(txLow)))
}

func TestPriorityMempool_DuplicateRejection(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	tx := []byte("duplicate test")
	require.NoError(t, m.AddTx(tx))

	// Adding same transaction again should fail
	err := m.AddTx(tx)
	assert.ErrorIs(t, err, types.ErrTxAlreadyExists)
	assert.Equal(t, 1, m.Size())
}

func TestPriorityMempool_NilTransaction(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024,
	}
	m := NewPriorityMempool(cfg)

	err := m.AddTx(nil)
	assert.ErrorIs(t, err, types.ErrInvalidTx)
}

func TestPriorityMempool_RemoveTxs(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	tx1 := []byte("tx1")
	tx2 := []byte("tx2")
	tx3 := []byte("tx3")

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))
	require.NoError(t, m.AddTx(tx3))
	assert.Equal(t, 3, m.Size())

	// Remove tx1 and tx3
	m.RemoveTxs([][]byte{types.HashTx(tx1), types.HashTx(tx3)})

	assert.Equal(t, 1, m.Size())
	assert.False(t, m.HasTx(types.HashTx(tx1)))
	assert.True(t, m.HasTx(types.HashTx(tx2)))
	assert.False(t, m.HasTx(types.HashTx(tx3)))
}

func TestPriorityMempool_Flush(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	require.NoError(t, m.AddTx([]byte("tx1")))
	require.NoError(t, m.AddTx([]byte("tx2")))
	require.NoError(t, m.AddTx([]byte("tx3")))
	assert.Equal(t, 3, m.Size())

	m.Flush()

	assert.Equal(t, 0, m.Size())
	assert.Equal(t, int64(0), m.SizeBytes())
}

func TestPriorityMempool_TxHashes(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	tx1 := []byte("tx1")
	tx2 := []byte("tx2")

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))

	hashes := m.TxHashes()
	assert.Len(t, hashes, 2)

	// Verify all hashes are present
	hashSet := make(map[string]bool)
	for _, h := range hashes {
		hashSet[string(h)] = true
	}
	assert.True(t, hashSet[string(types.HashTx(tx1))])
	assert.True(t, hashSet[string(types.HashTx(tx2))])
}

func TestPriorityMempool_GetPriority(t *testing.T) {
	priorityFunc := func(tx []byte) int64 {
		return int64(len(tx) * 10)
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       100,
		MaxBytes:     1024,
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	tx := []byte("hello") // 5 bytes -> priority 50
	require.NoError(t, m.AddTx(tx))

	hash := types.HashTx(tx)
	assert.Equal(t, int64(50), m.GetPriority(hash))

	// Non-existent transaction
	assert.Equal(t, int64(0), m.GetPriority([]byte("nonexistent")))
}

func TestPriorityMempool_HighestPriority(t *testing.T) {
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       100,
		MaxBytes:     1024,
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Empty mempool
	assert.Equal(t, int64(0), m.HighestPriority())

	require.NoError(t, m.AddTx([]byte{10})) // Priority 10
	assert.Equal(t, int64(10), m.HighestPriority())

	require.NoError(t, m.AddTx([]byte{50})) // Priority 50
	assert.Equal(t, int64(50), m.HighestPriority())

	require.NoError(t, m.AddTx([]byte{30}))         // Priority 30
	assert.Equal(t, int64(50), m.HighestPriority()) // Still 50
}

func TestPriorityMempool_SizePriorityFunc(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:       100,
		MaxBytes:     1024,
		PriorityFunc: SizePriorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Smaller transactions should have higher priority
	txSmall := []byte("hi")                                      // 2 bytes
	txLarge := []byte("hello world this is a large transaction") // 39 bytes

	require.NoError(t, m.AddTx(txSmall))
	require.NoError(t, m.AddTx(txLarge))

	hashSmall := types.HashTx(txSmall)
	hashLarge := types.HashTx(txLarge)

	// Small tx should have higher priority
	assert.Greater(t, m.GetPriority(hashSmall), m.GetPriority(hashLarge))
}

func TestPriorityMempool_ConcurrentAccess(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   1000,
		MaxBytes: 1024 * 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Add transactions concurrently
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tx := []byte{byte(n), byte(n + 1), byte(n + 2), byte(n + 3)}
			if err := m.AddTx(tx); err != nil && err != types.ErrTxAlreadyExists {
				errChan <- err
			}
		}(i)
	}

	// Read concurrently
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Size()
			_ = m.SizeBytes()
			_ = m.TxHashes()
			_ = m.ReapTxs(100)
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPriorityMempool_Interface(t *testing.T) {
	// Verify PriorityMempool implements Mempool interface
	var _ Mempool = (*PriorityMempool)(nil)
}

func TestPriorityMempool_ReapTxsMaxBytes(t *testing.T) {
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := PriorityMempoolConfig{
		MaxTxs:       100,
		MaxBytes:     1024,
		PriorityFunc: priorityFunc,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Add transactions
	tx1 := []byte{100, 1, 1, 1, 1} // Priority 100, 5 bytes
	tx2 := []byte{90, 2, 2, 2, 2}  // Priority 90, 5 bytes
	tx3 := []byte{80, 3, 3, 3, 3}  // Priority 80, 5 bytes

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))
	require.NoError(t, m.AddTx(tx3))

	// Reap with limit of 10 bytes - should get 2 highest priority
	txs := m.ReapTxs(10)
	require.Len(t, txs, 2)
	assert.Equal(t, tx1, txs[0]) // Highest priority
	assert.Equal(t, tx2, txs[1]) // Second highest
}

func TestPriorityMempool_SetPriorityFunc(t *testing.T) {
	cfg := PriorityMempoolConfig{
		MaxTxs:   100,
		MaxBytes: 1024,
	}
	m := NewPriorityMempool(cfg)
	m.SetTxValidator(AcceptAllTxValidator)

	// Add with default priority
	tx := []byte("test")
	require.NoError(t, m.AddTx(tx))

	hash := types.HashTx(tx)
	assert.Equal(t, int64(0), m.GetPriority(hash)) // Default returns 0

	// Change priority function (won't affect existing)
	m.SetPriorityFunc(func(tx []byte) int64 {
		return 100
	})

	// Add new transaction
	tx2 := []byte("test2")
	require.NoError(t, m.AddTx(tx2))

	hash2 := types.HashTx(tx2)
	assert.Equal(t, int64(100), m.GetPriority(hash2))

	// Old transaction still has old priority
	assert.Equal(t, int64(0), m.GetPriority(hash))
}
