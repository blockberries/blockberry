package mempool

import (
	"sync"
	"testing"
	"time"

	"github.com/blockberries/blockberry/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTTLMempool_Basic(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour, // Don't auto-cleanup during test
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("test transaction")
	err := m.AddTx(tx)
	require.NoError(t, err)

	assert.Equal(t, 1, m.Size())
	assert.Equal(t, int64(len(tx)), m.SizeBytes())

	hash := types.HashTx(tx)
	assert.True(t, m.HasTx(hash))

	retrieved, err := m.GetTx(hash)
	require.NoError(t, err)
	assert.Equal(t, tx, retrieved)
}

func TestTTLMempool_Expiration(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             50 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("expiring transaction")
	err := m.AddTx(tx)
	require.NoError(t, err)

	hash := types.HashTx(tx)
	assert.True(t, m.HasTx(hash))

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Transaction should be expired (not accessible)
	assert.False(t, m.HasTx(hash))
	_, err = m.GetTx(hash)
	assert.ErrorIs(t, err, types.ErrTxNotFound)

	// Wait for cleanup
	time.Sleep(50 * time.Millisecond)

	// Transaction should be removed by cleanup
	assert.Equal(t, 0, m.Size())
}

func TestTTLMempool_ReapExcludesExpired(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             50 * time.Millisecond,
		CleanupInterval: time.Hour, // Don't auto-cleanup
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	// Add a short-lived transaction
	txShort := []byte("short lived")
	require.NoError(t, m.AddTx(txShort))

	// Add a long-lived transaction
	txLong := []byte("long lived")
	require.NoError(t, m.AddTxWithTTL(txLong, time.Hour))

	assert.Equal(t, 2, m.Size())

	// Wait for short transaction to expire
	time.Sleep(100 * time.Millisecond)

	// Reap should only return the long-lived transaction
	txs := m.ReapTxs(1024)
	assert.Len(t, txs, 1)
	assert.Equal(t, txLong, txs[0])
}

func TestTTLMempool_GetTTL(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("test transaction")
	require.NoError(t, m.AddTx(tx))

	hash := types.HashTx(tx)
	ttl := m.GetTTL(hash)

	// TTL should be close to 1 hour
	assert.Greater(t, ttl, 59*time.Minute)
	assert.LessOrEqual(t, ttl, time.Hour)
}

func TestTTLMempool_ExtendTTL(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("test transaction")
	require.NoError(t, m.AddTx(tx))

	hash := types.HashTx(tx)
	originalTTL := m.GetTTL(hash)

	// Extend by 30 minutes
	success := m.ExtendTTL(hash, 30*time.Minute)
	assert.True(t, success)

	newTTL := m.GetTTL(hash)
	assert.Greater(t, newTTL, originalTTL)
}

func TestTTLMempool_SetTTL(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("test transaction")
	require.NoError(t, m.AddTx(tx))

	hash := types.HashTx(tx)

	// Set to expire in 10 minutes
	futureTime := time.Now().Add(10 * time.Minute)
	success := m.SetTTL(hash, futureTime)
	assert.True(t, success)

	ttl := m.GetTTL(hash)
	assert.Greater(t, ttl, 9*time.Minute)
	assert.LessOrEqual(t, ttl, 10*time.Minute)
}

func TestTTLMempool_AddTxWithTTL(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	// Add with custom TTL
	tx := []byte("custom ttl transaction")
	err := m.AddTxWithTTL(tx, 5*time.Minute)
	require.NoError(t, err)

	hash := types.HashTx(tx)
	ttl := m.GetTTL(hash)

	// TTL should be close to 5 minutes, not 1 hour
	assert.Greater(t, ttl, 4*time.Minute)
	assert.LessOrEqual(t, ttl, 5*time.Minute)
}

func TestTTLMempool_SizeActive(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             50 * time.Millisecond,
		CleanupInterval: time.Hour, // Don't auto-cleanup
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	// Add short-lived transaction
	txShort := []byte("short")
	require.NoError(t, m.AddTx(txShort))

	// Add long-lived transaction
	txLong := []byte("long")
	require.NoError(t, m.AddTxWithTTL(txLong, time.Hour))

	assert.Equal(t, 2, m.Size())
	assert.Equal(t, 2, m.SizeActive())

	// Wait for short transaction to expire
	time.Sleep(100 * time.Millisecond)

	// Size() still returns 2 (not cleaned up yet)
	// SizeActive() should return 1
	assert.Equal(t, 2, m.Size())
	assert.Equal(t, 1, m.SizeActive())
}

func TestTTLMempool_PriorityWithTTL(t *testing.T) {
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
		PriorityFunc:    priorityFunc,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	txLow := []byte{10, 0, 0}
	txMed := []byte{50, 0, 0}
	txHigh := []byte{100, 0, 0}

	require.NoError(t, m.AddTx(txLow))
	require.NoError(t, m.AddTx(txMed))
	require.NoError(t, m.AddTx(txHigh))

	// Reap should return highest priority first
	txs := m.ReapTxs(1024)
	require.Len(t, txs, 3)
	assert.Equal(t, txHigh, txs[0])
	assert.Equal(t, txMed, txs[1])
	assert.Equal(t, txLow, txs[2])
}

func TestTTLMempool_RemoveTxs(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx1 := []byte("tx1")
	tx2 := []byte("tx2")
	tx3 := []byte("tx3")

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))
	require.NoError(t, m.AddTx(tx3))

	m.RemoveTxs([][]byte{types.HashTx(tx1), types.HashTx(tx3)})

	assert.Equal(t, 1, m.Size())
	assert.False(t, m.HasTx(types.HashTx(tx1)))
	assert.True(t, m.HasTx(types.HashTx(tx2)))
	assert.False(t, m.HasTx(types.HashTx(tx3)))
}

func TestTTLMempool_Flush(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	require.NoError(t, m.AddTx([]byte("tx1")))
	require.NoError(t, m.AddTx([]byte("tx2")))
	assert.Equal(t, 2, m.Size())

	m.Flush()

	assert.Equal(t, 0, m.Size())
	assert.Equal(t, int64(0), m.SizeBytes())
}

func TestTTLMempool_ConcurrentAccess(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          1000,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: 10 * time.Millisecond,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	var wg sync.WaitGroup

	// Add transactions concurrently
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tx := []byte{byte(n), byte(n + 1), byte(n + 2)}
			_ = m.AddTx(tx)
		}(i)
	}

	// Read concurrently
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Size()
			_ = m.SizeActive()
			_ = m.TxHashes()
			_ = m.ReapTxs(100)
		}()
	}

	wg.Wait()
}

func TestTTLMempool_Interface(t *testing.T) {
	// Verify TTLMempool implements Mempool interface
	var _ Mempool = (*TTLMempool)(nil)
}

func TestTTLMempool_ManualCleanup(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             50 * time.Millisecond,
		CleanupInterval: time.Hour, // Disable auto-cleanup
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("test")
	require.NoError(t, m.AddTx(tx))
	assert.Equal(t, 1, m.Size())

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Still in mempool (not cleaned up)
	assert.Equal(t, 1, m.Size())

	// Manually trigger cleanup
	removed := m.removeExpired()
	assert.Equal(t, 1, removed)
	assert.Equal(t, 0, m.Size())
}

func TestTTLMempool_GetPriority(t *testing.T) {
	priorityFunc := func(tx []byte) int64 {
		return int64(len(tx) * 10)
	}

	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
		PriorityFunc:    priorityFunc,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx := []byte("hello") // 5 bytes -> priority 50
	require.NoError(t, m.AddTx(tx))

	hash := types.HashTx(tx)
	assert.Equal(t, int64(50), m.GetPriority(hash))
}

func TestTTLMempool_TxHashes(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	tx1 := []byte("tx1")
	tx2 := []byte("tx2")

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))

	hashes := m.TxHashes()
	assert.Len(t, hashes, 2)

	hashSet := make(map[string]bool)
	for _, h := range hashes {
		hashSet[string(h)] = true
	}
	assert.True(t, hashSet[string(types.HashTx(tx1))])
	assert.True(t, hashSet[string(types.HashTx(tx2))])
}

func TestTTLMempool_Eviction(t *testing.T) {
	priorityFunc := func(tx []byte) int64 {
		if len(tx) == 0 {
			return 0
		}
		return int64(tx[0])
	}

	cfg := TTLMempoolConfig{
		MaxTxs:          3,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
		PriorityFunc:    priorityFunc,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	// Fill mempool
	tx1 := []byte{10, 1} // Priority 10
	tx2 := []byte{20, 2} // Priority 20
	tx3 := []byte{30, 3} // Priority 30

	require.NoError(t, m.AddTx(tx1))
	require.NoError(t, m.AddTx(tx2))
	require.NoError(t, m.AddTx(tx3))

	// Add higher priority - should evict tx1
	txNew := []byte{50, 4} // Priority 50
	require.NoError(t, m.AddTx(txNew))

	assert.Equal(t, 3, m.Size())
	assert.False(t, m.HasTx(types.HashTx(tx1)))
	assert.True(t, m.HasTx(types.HashTx(tx2)))
	assert.True(t, m.HasTx(types.HashTx(tx3)))
	assert.True(t, m.HasTx(types.HashTx(txNew)))
}

func TestTTLMempool_ExpiredTTLMethods(t *testing.T) {
	cfg := TTLMempoolConfig{
		MaxTxs:          100,
		MaxBytes:        1024 * 1024,
		TTL:             time.Hour,
		CleanupInterval: time.Hour,
	}
	m := NewTTLMempool(cfg)
	defer m.Stop()

	// GetTTL for non-existent transaction
	assert.Equal(t, time.Duration(0), m.GetTTL([]byte("nonexistent")))

	// ExtendTTL for non-existent transaction
	assert.False(t, m.ExtendTTL([]byte("nonexistent"), time.Hour))

	// SetTTL for non-existent transaction
	assert.False(t, m.SetTTL([]byte("nonexistent"), time.Now().Add(time.Hour)))
}
