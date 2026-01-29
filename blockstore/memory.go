package blockstore

import (
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/blockberry/types"
)

// MemoryBlockStore implements BlockStore with in-memory storage.
// Primarily used for testing.
type MemoryBlockStore struct {
	blocks   map[int64]blockEntry
	byHash   map[string]int64
	height   int64
	base     int64
	pruneCfg *PruneConfig
	mu       sync.RWMutex
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

	// Check for hash collision (different height with same hash)
	if existingHeight, exists := m.byHash[string(hash)]; exists && existingHeight != height {
		return fmt.Errorf("%w: hash already exists at height %d", types.ErrBlockAlreadyExists, existingHeight)
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

// Prune removes blocks before the given height.
// Blocks that should be kept according to the prune config are preserved.
func (m *MemoryBlockStore) Prune(beforeHeight int64) (*PruneResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	start := time.Now()

	// Validate inputs
	if beforeHeight <= 0 {
		return nil, ErrInvalidPruneHeight
	}

	if beforeHeight > m.height {
		return nil, ErrPruneHeightTooHigh
	}

	// Nothing to prune if base is already at or above target
	if m.base >= beforeHeight {
		return &PruneResult{
			PrunedCount: 0,
			NewBase:     m.base,
			Duration:    time.Since(start),
		}, nil
	}

	var prunedCount int64
	var bytesFreed int64
	newBase := beforeHeight

	// Find blocks to prune
	for height := m.base; height < beforeHeight; height++ {
		entry, exists := m.blocks[height]
		if !exists {
			continue
		}

		// Check if this block should be kept
		if m.pruneCfg != nil && m.pruneCfg.ShouldKeep(height, m.height) {
			if height < newBase {
				newBase = height
			}
			continue
		}

		// Delete the block
		bytesFreed += int64(len(entry.data) + len(entry.hash))
		delete(m.byHash, string(entry.hash))
		delete(m.blocks, height)
		prunedCount++
	}

	// Update base
	if newBase > m.base {
		m.base = newBase
	}

	return &PruneResult{
		PrunedCount: prunedCount,
		NewBase:     m.base,
		BytesFreed:  bytesFreed,
		Duration:    time.Since(start),
	}, nil
}

// PruneConfig returns the current pruning configuration.
func (m *MemoryBlockStore) PruneConfig() *PruneConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pruneCfg
}

// SetPruneConfig updates the pruning configuration.
func (m *MemoryBlockStore) SetPruneConfig(cfg *PruneConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneCfg = cfg
}

// Ensure MemoryBlockStore implements PrunableBlockStore.
var _ PrunableBlockStore = (*MemoryBlockStore)(nil)
