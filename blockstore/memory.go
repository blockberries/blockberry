package blockstore

import (
	"sync"

	"github.com/blockberries/blockberry/types"
)

// MemoryBlockStore implements BlockStore with in-memory storage.
// Primarily used for testing.
type MemoryBlockStore struct {
	blocks map[int64]blockEntry
	byHash map[string]int64
	height int64
	base   int64
	mu     sync.RWMutex
}

type blockEntry struct {
	hash []byte
	data []byte
}

// NewMemoryBlockStore creates a new in-memory block store.
func NewMemoryBlockStore() *MemoryBlockStore {
	return &MemoryBlockStore{
		blocks: make(map[int64]blockEntry),
		byHash: make(map[string]int64),
	}
}

// SaveBlock stores a block at the given height.
func (m *MemoryBlockStore) SaveBlock(height int64, hash, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.blocks[height]; exists {
		return types.ErrBlockExists
	}

	m.blocks[height] = blockEntry{hash: hash, data: data}
	m.byHash[string(hash)] = height

	if m.base == 0 || height < m.base {
		m.base = height
	}
	if height > m.height {
		m.height = height
	}

	return nil
}

// LoadBlock retrieves a block by height.
func (m *MemoryBlockStore) LoadBlock(height int64) (hash, data []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.blocks[height]
	if !exists {
		return nil, nil, types.ErrBlockNotFound
	}
	return entry.hash, entry.data, nil
}

// LoadBlockByHash retrieves a block by its hash.
func (m *MemoryBlockStore) LoadBlockByHash(hash []byte) (height int64, data []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	h, exists := m.byHash[string(hash)]
	if !exists {
		return 0, nil, types.ErrBlockNotFound
	}

	entry := m.blocks[h]
	return h, entry.data, nil
}

// HasBlock checks if a block exists at the given height.
func (m *MemoryBlockStore) HasBlock(height int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.blocks[height]
	return exists
}

// Height returns the latest block height.
func (m *MemoryBlockStore) Height() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.height
}

// Base returns the earliest available block height.
func (m *MemoryBlockStore) Base() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.base
}

// Close closes the store.
func (m *MemoryBlockStore) Close() error {
	return nil
}
