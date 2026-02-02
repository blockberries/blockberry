package blockstore

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/blockberries/blockberry/types"
	loosetypes "github.com/blockberries/looseberry/types"
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

	// Certificate storage
	certs        map[string]*loosetypes.Certificate // digest -> certificate
	certsByRound map[uint64][]string                // round -> list of digests
	certsByHeight map[int64][]string                // height -> list of digests
	batches      map[string]*loosetypes.Batch       // digest -> batch
}

type blockEntry struct {
	hash []byte
	data []byte
}

// NewMemoryBlockStore creates a new in-memory block store.
func NewMemoryBlockStore() *MemoryBlockStore {
	return &MemoryBlockStore{
		blocks:        make(map[int64]blockEntry),
		byHash:        make(map[string]int64),
		certs:         make(map[string]*loosetypes.Certificate),
		certsByRound:  make(map[uint64][]string),
		certsByHeight: make(map[int64][]string),
		batches:       make(map[string]*loosetypes.Batch),
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

	// Store defensive copies to prevent external mutation
	m.blocks[height] = blockEntry{
		hash: append([]byte(nil), hash...),
		data: append([]byte(nil), data...),
	}
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
// Returns defensive copies to prevent external mutation of stored data.
func (m *MemoryBlockStore) LoadBlock(height int64) (hash, data []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.blocks[height]
	if !exists {
		return nil, nil, types.ErrBlockNotFound
	}
	return append([]byte(nil), entry.hash...), append([]byte(nil), entry.data...), nil
}

// LoadBlockByHash retrieves a block by its hash.
// Returns defensive copy to prevent external mutation of stored data.
func (m *MemoryBlockStore) LoadBlockByHash(hash []byte) (height int64, data []byte, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	h, exists := m.byHash[string(hash)]
	if !exists {
		return 0, nil, types.ErrBlockNotFound
	}

	entry := m.blocks[h]
	return h, append([]byte(nil), entry.data...), nil
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

// Ensure MemoryBlockStore implements PrunableBlockStore and CertificateBlockStore.
var _ PrunableBlockStore = (*MemoryBlockStore)(nil)
var _ CertificateBlockStore = (*MemoryBlockStore)(nil)

// SaveCertificate persists a DAG certificate.
func (m *MemoryBlockStore) SaveCertificate(cert *loosetypes.Certificate) error {
	if cert == nil {
		return fmt.Errorf("cannot save nil certificate")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	digest := cert.Digest()
	digestKey := string(digest[:])

	if _, exists := m.certs[digestKey]; exists {
		return types.ErrCertificateAlreadyExists
	}

	// Store a clone to prevent external mutation
	m.certs[digestKey] = cert.Clone()

	// Update round index
	round := cert.Round()
	m.certsByRound[round] = append(m.certsByRound[round], digestKey)

	return nil
}

// GetCertificate retrieves a certificate by its digest.
func (m *MemoryBlockStore) GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	digestKey := string(digest[:])
	cert, exists := m.certs[digestKey]
	if !exists {
		return nil, types.ErrCertificateNotFound
	}

	// Return a clone to prevent external mutation
	return cert.Clone(), nil
}

// GetCertificatesForRound retrieves all certificates for a given DAG round.
func (m *MemoryBlockStore) GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	digests, exists := m.certsByRound[round]
	if !exists || len(digests) == 0 {
		return nil, nil
	}

	certs := make([]*loosetypes.Certificate, 0, len(digests))
	for _, digestKey := range digests {
		cert, exists := m.certs[digestKey]
		if exists {
			certs = append(certs, cert.Clone())
		}
	}

	// Sort by validator index for deterministic ordering
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].Author() < certs[j].Author()
	})

	return certs, nil
}

// GetCertificatesForHeight retrieves all certificates committed at a given block height.
func (m *MemoryBlockStore) GetCertificatesForHeight(height int64) ([]*loosetypes.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	digests, exists := m.certsByHeight[height]
	if !exists || len(digests) == 0 {
		return nil, nil
	}

	certs := make([]*loosetypes.Certificate, 0, len(digests))
	for _, digestKey := range digests {
		cert, exists := m.certs[digestKey]
		if exists {
			certs = append(certs, cert.Clone())
		}
	}

	// Sort by validator index for deterministic ordering
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].Author() < certs[j].Author()
	})

	return certs, nil
}

// SetCertificateBlockHeight updates the block height index for a certificate.
func (m *MemoryBlockStore) SetCertificateBlockHeight(digest loosetypes.Hash, height int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	digestKey := string(digest[:])
	if _, exists := m.certs[digestKey]; !exists {
		return types.ErrCertificateNotFound
	}

	// Add to height index
	m.certsByHeight[height] = append(m.certsByHeight[height], digestKey)
	return nil
}

// SaveBatch persists a transaction batch.
func (m *MemoryBlockStore) SaveBatch(batch *loosetypes.Batch) error {
	if batch == nil {
		return fmt.Errorf("cannot save nil batch")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	digestKey := string(batch.Digest[:])
	if _, exists := m.batches[digestKey]; exists {
		return types.ErrBatchAlreadyExists
	}

	// Store a clone to prevent external mutation
	m.batches[digestKey] = batch.Clone()
	return nil
}

// GetBatch retrieves a batch by its digest.
func (m *MemoryBlockStore) GetBatch(digest loosetypes.Hash) (*loosetypes.Batch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	digestKey := string(digest[:])
	batch, exists := m.batches[digestKey]
	if !exists {
		return nil, types.ErrBatchNotFound
	}

	// Return a clone to prevent external mutation
	return batch.Clone(), nil
}
