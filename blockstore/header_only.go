package blockstore

import (
	"sync"

	"github.com/blockberries/blockberry/types"
)

// HeaderOnlyBlockStore stores only block headers (hash and height), not full block data.
// Use this for light clients that verify headers but don't store full blocks.
type HeaderOnlyBlockStore struct {
	headers map[int64][]byte // height -> hash
	byHash  map[string]int64 // hex(hash) -> height
	height  int64
	base    int64
	mu      sync.RWMutex
}

// NewHeaderOnlyBlockStore creates a new header-only block store.
func NewHeaderOnlyBlockStore() *HeaderOnlyBlockStore {
	return &HeaderOnlyBlockStore{
		headers: make(map[int64][]byte),
		byHash:  make(map[string]int64),
	}
}

// SaveBlock stores the block header (hash only, discards data).
func (s *HeaderOnlyBlockStore) SaveBlock(height int64, hash []byte, _ []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.headers[height]; exists {
		return types.ErrBlockAlreadyExists
	}

	// Store hash only, discard block data
	hashCopy := make([]byte, len(hash))
	copy(hashCopy, hash)

	s.headers[height] = hashCopy
	s.byHash[string(hash)] = height

	if height > s.height {
		s.height = height
	}
	if s.base == 0 || height < s.base {
		s.base = height
	}

	return nil
}

// LoadBlock returns the hash but empty data (header-only store).
func (s *HeaderOnlyBlockStore) LoadBlock(height int64) ([]byte, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash, exists := s.headers[height]
	if !exists {
		return nil, nil, types.ErrBlockNotFound
	}

	// Return hash but no data
	return hash, nil, nil
}

// LoadBlockByHash returns the height but empty data (header-only store).
func (s *HeaderOnlyBlockStore) LoadBlockByHash(hash []byte) (int64, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	height, exists := s.byHash[string(hash)]
	if !exists {
		return 0, nil, types.ErrBlockNotFound
	}

	// Return height but no data
	return height, nil, nil
}

// HasBlock checks if a header exists at the given height.
func (s *HeaderOnlyBlockStore) HasBlock(height int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.headers[height]
	return exists
}

// Height returns the latest block height.
func (s *HeaderOnlyBlockStore) Height() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.height
}

// Base returns the earliest available block height.
func (s *HeaderOnlyBlockStore) Base() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.base
}

// Close clears the in-memory state.
func (s *HeaderOnlyBlockStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headers = make(map[int64][]byte)
	s.byHash = make(map[string]int64)
	s.height = 0
	s.base = 0
	return nil
}

// GetHash returns the hash for a given height.
// This is a header-specific method not in the BlockStore interface.
func (s *HeaderOnlyBlockStore) GetHash(height int64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash, exists := s.headers[height]
	if !exists {
		return nil, types.ErrBlockNotFound
	}
	return hash, nil
}

// Verify HeaderOnlyBlockStore implements BlockStore interface.
var _ BlockStore = (*HeaderOnlyBlockStore)(nil)
