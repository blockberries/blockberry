package blockstore

import (
	"github.com/blockberries/blockberry/types"
)

// NoOpBlockStore is a block store that does nothing.
// It rejects all save operations and reports no blocks.
// Use this for node roles that don't need block storage (e.g., seed nodes).
type NoOpBlockStore struct{}

// NewNoOpBlockStore creates a new no-op block store.
func NewNoOpBlockStore() *NoOpBlockStore {
	return &NoOpBlockStore{}
}

// SaveBlock always returns an error as no-op store doesn't save blocks.
func (s *NoOpBlockStore) SaveBlock(_ int64, _ []byte, _ []byte) error {
	return types.ErrStoreClosed
}

// LoadBlock always returns ErrBlockNotFound.
func (s *NoOpBlockStore) LoadBlock(_ int64) ([]byte, []byte, error) {
	return nil, nil, types.ErrBlockNotFound
}

// LoadBlockByHash always returns ErrBlockNotFound.
func (s *NoOpBlockStore) LoadBlockByHash(_ []byte) (int64, []byte, error) {
	return 0, nil, types.ErrBlockNotFound
}

// HasBlock always returns false.
func (s *NoOpBlockStore) HasBlock(_ int64) bool {
	return false
}

// Height always returns 0.
func (s *NoOpBlockStore) Height() int64 {
	return 0
}

// Base always returns 0.
func (s *NoOpBlockStore) Base() int64 {
	return 0
}

// Close does nothing.
func (s *NoOpBlockStore) Close() error {
	return nil
}

// Verify NoOpBlockStore implements BlockStore interface.
var _ BlockStore = (*NoOpBlockStore)(nil)
