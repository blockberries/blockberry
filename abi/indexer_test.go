package abi

import (
	"testing"
)

func TestNullTxIndexer_Index(t *testing.T) {
	indexer := &NullTxIndexer{}

	result := &TxIndexResult{
		Hash:   []byte("test-hash"),
		Height: 100,
		Index:  0,
		Result: &TxExecResult{
			Code: CodeOK,
		},
	}

	err := indexer.Index(result)
	if err != nil {
		t.Errorf("Index() error = %v, want nil", err)
	}
}

func TestNullTxIndexer_Get(t *testing.T) {
	indexer := &NullTxIndexer{}

	result, err := indexer.Get([]byte("test-hash"))

	if result != nil {
		t.Errorf("Get() result = %v, want nil", result)
	}
	if err != ErrTxNotFound {
		t.Errorf("Get() error = %v, want ErrTxNotFound", err)
	}
}

func TestNullTxIndexer_Search(t *testing.T) {
	indexer := &NullTxIndexer{}

	results, total, err := indexer.Search("tx.height=100", 1, 10)

	if len(results) != 0 {
		t.Errorf("Search() results length = %d, want 0", len(results))
	}
	if total != 0 {
		t.Errorf("Search() total = %d, want 0", total)
	}
	if err != nil {
		t.Errorf("Search() error = %v, want nil", err)
	}
}

func TestNullTxIndexer_Delete(t *testing.T) {
	indexer := &NullTxIndexer{}

	err := indexer.Delete([]byte("test-hash"))
	if err != nil {
		t.Errorf("Delete() error = %v, want nil", err)
	}
}

func TestNullTxIndexer_Has(t *testing.T) {
	indexer := &NullTxIndexer{}

	has := indexer.Has([]byte("test-hash"))
	if has {
		t.Errorf("Has() = true, want false")
	}
}

func TestNullTxIndexer_Batch(t *testing.T) {
	indexer := &NullTxIndexer{}

	batch := indexer.Batch()
	if batch == nil {
		t.Fatal("Batch() returned nil")
	}

	// Test batch operations
	result := &TxIndexResult{
		Hash:   []byte("test-hash"),
		Height: 100,
		Index:  0,
	}

	if err := batch.Add(result); err != nil {
		t.Errorf("batch.Add() error = %v, want nil", err)
	}

	if err := batch.Delete([]byte("test-hash")); err != nil {
		t.Errorf("batch.Delete() error = %v, want nil", err)
	}

	if batch.Size() != 0 {
		t.Errorf("batch.Size() = %d, want 0", batch.Size())
	}

	if err := batch.Commit(); err != nil {
		t.Errorf("batch.Commit() error = %v, want nil", err)
	}

	// Discard should not panic
	batch.Discard()
}

func TestNullTxIndexer_ImplementsInterface(t *testing.T) {
	var _ TxIndexer = (*NullTxIndexer)(nil)
}

func TestIndexError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *IndexError
		want string
	}{
		{
			name: "ErrTxNotFound",
			err:  ErrTxNotFound,
			want: "transaction not found",
		},
		{
			name: "ErrInvalidQuery",
			err:  ErrInvalidQuery,
			want: "invalid query",
		},
		{
			name: "ErrIndexFull",
			err:  ErrIndexFull,
			want: "index storage full",
		},
		{
			name: "ErrIndexCorrupted",
			err:  ErrIndexCorrupted,
			want: "index corrupted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIndexerConfig_Default(t *testing.T) {
	config := DefaultIndexerConfig()

	if config.Indexer != "kv" {
		t.Errorf("Indexer = %q, want %q", config.Indexer, "kv")
	}
	if config.DBPath != "data/tx_index.db" {
		t.Errorf("DBPath = %q, want %q", config.DBPath, "data/tx_index.db")
	}
	if config.IndexAllEvents {
		t.Errorf("IndexAllEvents = true, want false")
	}
}

func TestTxIndexResult_Fields(t *testing.T) {
	result := &TxIndexResult{
		Hash:   []byte("test-hash"),
		Height: 12345,
		Index:  42,
		Result: &TxExecResult{
			Code:    CodeOK,
			Data:    []byte("result-data"),
			GasUsed: 50000,
		},
	}

	if string(result.Hash) != "test-hash" {
		t.Errorf("Hash = %q, want %q", string(result.Hash), "test-hash")
	}
	if result.Height != 12345 {
		t.Errorf("Height = %d, want %d", result.Height, 12345)
	}
	if result.Index != 42 {
		t.Errorf("Index = %d, want %d", result.Index, 42)
	}
	if result.Result.Code != CodeOK {
		t.Errorf("Result.Code = %d, want %d", result.Result.Code, CodeOK)
	}
}
