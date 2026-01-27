package kv

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/abi"
)

func createTestIndexer(t *testing.T) (*TxIndexer, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "indexer-test-*")
	require.NoError(t, err)

	idx, err := NewTxIndexer(dir, true)
	require.NoError(t, err)

	cleanup := func() {
		idx.Close()
		os.RemoveAll(dir)
	}

	return idx, cleanup
}

func TestTxIndexer_IndexAndGet(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	result := &abi.TxIndexResult{
		Hash:   []byte("tx-hash-1"),
		Height: 100,
		Index:  0,
		Result: &abi.TxExecResult{
			Code: abi.CodeOK,
			Events: []abi.Event{
				{
					Type: "transfer",
					Attributes: []abi.Attribute{
						{Key: "sender", Value: []byte("alice"), Index: true},
						{Key: "recipient", Value: []byte("bob"), Index: true},
						{Key: "amount", Value: []byte("100"), Index: false},
					},
				},
			},
		},
	}

	// Index the transaction
	err := idx.Index(result)
	require.NoError(t, err)

	// Get it back
	got, err := idx.Get([]byte("tx-hash-1"))
	require.NoError(t, err)
	require.Equal(t, result.Hash, got.Hash)
	require.Equal(t, result.Height, got.Height)
	require.Equal(t, result.Index, got.Index)
}

func TestTxIndexer_Get_NotFound(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	_, err := idx.Get([]byte("nonexistent"))
	require.ErrorIs(t, err, abi.ErrTxNotFound)
}

func TestTxIndexer_Has(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Initially doesn't have
	require.False(t, idx.Has([]byte("tx-hash-1")))

	// Index a transaction
	err := idx.Index(&abi.TxIndexResult{
		Hash:   []byte("tx-hash-1"),
		Height: 100,
	})
	require.NoError(t, err)

	// Now has it
	require.True(t, idx.Has([]byte("tx-hash-1")))
}

func TestTxIndexer_Delete(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Index a transaction
	err := idx.Index(&abi.TxIndexResult{
		Hash:   []byte("tx-hash-1"),
		Height: 100,
		Result: &abi.TxExecResult{
			Events: []abi.Event{
				{Type: "test", Attributes: []abi.Attribute{{Key: "key", Value: []byte("value"), Index: true}}},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, idx.Has([]byte("tx-hash-1")))

	// Delete it
	err = idx.Delete([]byte("tx-hash-1"))
	require.NoError(t, err)
	require.False(t, idx.Has([]byte("tx-hash-1")))

	// Get should return not found
	_, err = idx.Get([]byte("tx-hash-1"))
	require.ErrorIs(t, err, abi.ErrTxNotFound)
}

func TestTxIndexer_SearchByHeight(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Index transactions at different heights
	for i := uint64(1); i <= 10; i++ {
		err := idx.Index(&abi.TxIndexResult{
			Hash:   []byte{byte(i)},
			Height: i * 10,
		})
		require.NoError(t, err)
	}

	tests := []struct {
		name     string
		query    string
		expected int
	}{
		{"exact match", "tx.height=50", 1},
		{"greater than", "tx.height>50", 5},
		{"greater than or equal", "tx.height>=50", 6},
		{"less than", "tx.height<50", 4},
		{"less than or equal", "tx.height<=50", 5},
		{"no match", "tx.height=999", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := idx.Search(tt.query, 1, 100)
			require.NoError(t, err)
			require.Equal(t, tt.expected, total)
			require.Equal(t, tt.expected, len(results))
		})
	}
}

func TestTxIndexer_SearchByEvent(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Index transactions with events
	err := idx.Index(&abi.TxIndexResult{
		Hash:   []byte("tx-1"),
		Height: 100,
		Result: &abi.TxExecResult{
			Events: []abi.Event{
				{
					Type: "transfer",
					Attributes: []abi.Attribute{
						{Key: "sender", Value: []byte("alice"), Index: true},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	err = idx.Index(&abi.TxIndexResult{
		Hash:   []byte("tx-2"),
		Height: 101,
		Result: &abi.TxExecResult{
			Events: []abi.Event{
				{
					Type: "transfer",
					Attributes: []abi.Attribute{
						{Key: "sender", Value: []byte("bob"), Index: true},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// Search by event
	results, total, err := idx.Search("transfer.sender='alice'", 1, 100)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []byte("tx-1"), results[0].Hash)

	// Search bob
	results, total, err = idx.Search("transfer.sender='bob'", 1, 100)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []byte("tx-2"), results[0].Hash)

	// No match
	results, total, err = idx.Search("transfer.sender='charlie'", 1, 100)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, results)
}

func TestTxIndexer_SearchPagination(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Index 25 transactions
	for i := uint64(1); i <= 25; i++ {
		err := idx.Index(&abi.TxIndexResult{
			Hash:   []byte{byte(i)},
			Height: 100, // All at same height
		})
		require.NoError(t, err)
	}

	// First page
	results, total, err := idx.Search("tx.height=100", 1, 10)
	require.NoError(t, err)
	require.Equal(t, 25, total)
	require.Equal(t, 10, len(results))

	// Second page
	results, total, err = idx.Search("tx.height=100", 2, 10)
	require.NoError(t, err)
	require.Equal(t, 25, total)
	require.Equal(t, 10, len(results))

	// Third page (partial)
	results, total, err = idx.Search("tx.height=100", 3, 10)
	require.NoError(t, err)
	require.Equal(t, 25, total)
	require.Equal(t, 5, len(results))

	// Beyond last page
	results, total, err = idx.Search("tx.height=100", 4, 10)
	require.NoError(t, err)
	require.Equal(t, 25, total)
	require.Equal(t, 0, len(results))
}

func TestTxIndexer_SearchInvalidQuery(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	tests := []string{
		"",
		"invalid",
		"no.operator.here",
		"tx.height>abc", // invalid number
	}

	for _, query := range tests {
		t.Run(query, func(t *testing.T) {
			_, _, err := idx.Search(query, 1, 10)
			require.Error(t, err)
		})
	}
}

func TestTxIndexer_Batch(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	batch := idx.Batch()

	// Add multiple transactions
	for i := 0; i < 10; i++ {
		err := batch.Add(&abi.TxIndexResult{
			Hash:   []byte{byte(i)},
			Height: uint64(100 + i),
		})
		require.NoError(t, err)
	}

	require.Equal(t, 10, batch.Size())

	// Not yet visible
	require.False(t, idx.Has([]byte{0}))

	// Commit
	err := batch.Commit()
	require.NoError(t, err)

	// Now visible
	for i := 0; i < 10; i++ {
		require.True(t, idx.Has([]byte{byte(i)}))
	}
}

func TestTxIndexer_BatchDelete(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Index some transactions
	for i := 0; i < 5; i++ {
		err := idx.Index(&abi.TxIndexResult{
			Hash:   []byte{byte(i)},
			Height: uint64(100 + i),
		})
		require.NoError(t, err)
	}

	// Delete in batch
	batch := idx.Batch()
	err := batch.Delete([]byte{0})
	require.NoError(t, err)
	err = batch.Delete([]byte{1})
	require.NoError(t, err)

	// Still visible before commit
	require.True(t, idx.Has([]byte{0}))

	// Commit
	err = batch.Commit()
	require.NoError(t, err)

	// Now deleted
	require.False(t, idx.Has([]byte{0}))
	require.False(t, idx.Has([]byte{1}))
	require.True(t, idx.Has([]byte{2}))
}

func TestTxIndexer_BatchDiscard(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	batch := idx.Batch()

	err := batch.Add(&abi.TxIndexResult{
		Hash:   []byte("tx-1"),
		Height: 100,
	})
	require.NoError(t, err)

	require.Equal(t, 1, batch.Size())

	// Discard
	batch.Discard()

	require.Equal(t, 0, batch.Size())

	// Should not be indexed
	require.False(t, idx.Has([]byte("tx-1")))
}

func TestTxIndexer_Close(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Index something
	err := idx.Index(&abi.TxIndexResult{
		Hash:   []byte("tx-1"),
		Height: 100,
	})
	require.NoError(t, err)

	// Close
	err = idx.Close()
	require.NoError(t, err)

	// Operations should fail
	_, err = idx.Get([]byte("tx-1"))
	require.ErrorIs(t, err, abi.ErrIndexCorrupted)

	require.False(t, idx.Has([]byte("tx-1")))

	err = idx.Index(&abi.TxIndexResult{Hash: []byte("tx-2"), Height: 101})
	require.ErrorIs(t, err, abi.ErrIndexCorrupted)
}

func TestTxIndexer_IndexNilResult(t *testing.T) {
	idx, cleanup := createTestIndexer(t)
	defer cleanup()

	// Should not panic on nil
	err := idx.Index(nil)
	require.NoError(t, err)

	// Should not panic on empty hash
	err = idx.Index(&abi.TxIndexResult{Hash: nil})
	require.NoError(t, err)
}

func TestTxIndexer_IndexAllEvents(t *testing.T) {
	// Test with indexAllEvents = false
	dir, err := os.MkdirTemp("", "indexer-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	idx, err := NewTxIndexer(dir, false) // Don't index all events
	require.NoError(t, err)
	defer idx.Close()

	// Index with non-indexed event
	err = idx.Index(&abi.TxIndexResult{
		Hash:   []byte("tx-1"),
		Height: 100,
		Result: &abi.TxExecResult{
			Events: []abi.Event{
				{
					Type: "test",
					Attributes: []abi.Attribute{
						{Key: "indexed", Value: []byte("yes"), Index: true},
						{Key: "not_indexed", Value: []byte("no"), Index: false},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	// Should find indexed attribute
	results, total, err := idx.Search("test.indexed='yes'", 1, 10)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, results, 1)

	// Should NOT find non-indexed attribute
	results, total, err = idx.Search("test.not_indexed='no'", 1, 10)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, results)
}

func TestKeyConstruction(t *testing.T) {
	t.Run("txHashKey", func(t *testing.T) {
		key := txHashKey([]byte("abc"))
		require.True(t, len(key) > 3)
		require.Equal(t, prefixTxByHash, key[:len(prefixTxByHash)])
	})

	t.Run("txHeightKey", func(t *testing.T) {
		key := txHeightKey(12345, []byte("hash"))
		require.True(t, len(key) > len(prefixTxByHeight)+8)
		require.Equal(t, prefixTxByHeight, key[:len(prefixTxByHeight)])
	})

	t.Run("txEventKey", func(t *testing.T) {
		key := txEventKey("transfer", "sender", []byte("alice"), []byte("hash"))
		require.Contains(t, string(key), "transfer")
		require.Contains(t, string(key), "sender")
		require.Contains(t, string(key), "alice")
	})

	t.Run("encodeHeight", func(t *testing.T) {
		b := encodeHeight(0x0102030405060708)
		require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}, b)
	})
}
