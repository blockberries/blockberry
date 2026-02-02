package blockstore

import (
	"encoding/binary"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/blockberries/blockberry/types"
	"github.com/blockberries/cramberry/pkg/cramberry"
	loosetypes "github.com/blockberries/looseberry/types"
)

// Key prefixes for LevelDB storage.
var (
	prefixHeight  = []byte("H:") // Height -> Hash mapping
	prefixBlock   = []byte("B:") // Hash -> Block data mapping
	keyMetaHeight = []byte("M:height")
	keyMetaBase   = []byte("M:base")

	// Certificate storage prefixes
	prefixCert       = []byte("C:")  // C:<digest> -> Certificate data
	prefixCertRound  = []byte("CR:") // CR:<round>:<validator_idx> -> Certificate digest
	prefixCertHeight = []byte("CH:") // CH:<height>:<validator_idx> -> Certificate digest
	prefixBatch      = []byte("CB:") // CB:<digest> -> Batch data
)

// LevelDBBlockStore implements BlockStore using LevelDB.
type LevelDBBlockStore struct {
	db        *leveldb.DB
	path      string
	height    int64
	base      int64
	pruneCfg  *PruneConfig
	pruning   bool // Indicates if pruning is in progress
	mu        sync.RWMutex
}

// NewLevelDBBlockStore creates a new LevelDB-backed block store.
func NewLevelDBBlockStore(path string) (*LevelDBBlockStore, error) {
	db, err := leveldb.OpenFile(path, &opt.Options{
		NoSync: false, // Ensure durability
	})
	if err != nil {
		return nil, fmt.Errorf("opening leveldb: %w", err)
	}

	store := &LevelDBBlockStore{
		db:   db,
		path: path,
	}

	// Load metadata
	if err := store.loadMetadata(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading metadata: %w", err)
	}

	return store, nil
}

// loadMetadata loads the height and base from the database.
func (s *LevelDBBlockStore) loadMetadata() error {
	// Load height
	data, err := s.db.Get(keyMetaHeight, nil)
	if err == nil {
		s.height = decodeInt64(data)
	} else if err != leveldb.ErrNotFound {
		return err
	}

	// Load base
	data, err = s.db.Get(keyMetaBase, nil)
	if err == nil {
		s.base = decodeInt64(data)
	} else if err != leveldb.ErrNotFound {
		return err
	}

	return nil
}

// SaveBlock persists a block at the given height.
func (s *LevelDBBlockStore) SaveBlock(height int64, hash []byte, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if block already exists
	heightKey := makeHeightKey(height)
	exists, err := s.db.Has(heightKey, nil)
	if err != nil {
		return fmt.Errorf("checking block existence: %w", err)
	}
	if exists {
		return types.ErrBlockAlreadyExists
	}

	// Create batch for atomic write
	batch := new(leveldb.Batch)

	// Store height -> hash mapping
	batch.Put(heightKey, hash)

	// Store hash -> data mapping (include height in value for reverse lookup)
	blockKey := makeBlockKey(hash)
	blockValue := makeBlockValue(height, data)
	batch.Put(blockKey, blockValue)

	// Update metadata
	if height > s.height {
		batch.Put(keyMetaHeight, encodeInt64(height))
	}
	if s.base == 0 || height < s.base {
		batch.Put(keyMetaBase, encodeInt64(height))
	}

	// Write batch
	if err := s.db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("writing block: %w", err)
	}

	// Update in-memory state
	if height > s.height {
		s.height = height
	}
	if s.base == 0 || height < s.base {
		s.base = height
	}

	return nil
}

// LoadBlock retrieves a block by height.
func (s *LevelDBBlockStore) LoadBlock(height int64) ([]byte, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get hash from height
	heightKey := makeHeightKey(height)
	hash, err := s.db.Get(heightKey, nil)
	if err == leveldb.ErrNotFound {
		return nil, nil, types.ErrBlockNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("getting hash for height %d: %w", height, err)
	}

	// Get data from hash
	blockKey := makeBlockKey(hash)
	blockValue, err := s.db.Get(blockKey, nil)
	if err == leveldb.ErrNotFound {
		return nil, nil, types.ErrBlockNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("getting block data: %w", err)
	}

	_, data := parseBlockValue(blockValue)
	return hash, data, nil
}

// LoadBlockByHash retrieves a block by its hash.
func (s *LevelDBBlockStore) LoadBlockByHash(hash []byte) (int64, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blockKey := makeBlockKey(hash)
	blockValue, err := s.db.Get(blockKey, nil)
	if err == leveldb.ErrNotFound {
		return 0, nil, types.ErrBlockNotFound
	}
	if err != nil {
		return 0, nil, fmt.Errorf("getting block by hash: %w", err)
	}

	height, data := parseBlockValue(blockValue)
	return height, data, nil
}

// HasBlock checks if a block exists at the given height.
func (s *LevelDBBlockStore) HasBlock(height int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	heightKey := makeHeightKey(height)
	exists, _ := s.db.Has(heightKey, nil)
	return exists
}

// Height returns the latest block height.
func (s *LevelDBBlockStore) Height() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.height
}

// Base returns the earliest available block height.
func (s *LevelDBBlockStore) Base() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.base
}

// Close closes the database.
func (s *LevelDBBlockStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// BlockCount returns the number of blocks stored.
func (s *LevelDBBlockStore) BlockCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	iter := s.db.NewIterator(util.BytesPrefix(prefixHeight), nil)
	defer iter.Release()

	for iter.Next() {
		count++
	}
	return count
}

// Prune removes blocks before the given height.
// Blocks that should be kept according to the prune config are preserved.
func (s *LevelDBBlockStore) Prune(beforeHeight int64) (*PruneResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	// Validate inputs
	if beforeHeight <= 0 {
		return nil, ErrInvalidPruneHeight
	}

	if beforeHeight > s.height {
		return nil, ErrPruneHeightTooHigh
	}

	// Check if pruning is already in progress
	if s.pruning {
		return nil, ErrPruningInProgress
	}
	s.pruning = true
	defer func() { s.pruning = false }()

	// Nothing to prune if base is already at or above target
	if s.base >= beforeHeight {
		return &PruneResult{
			PrunedCount: 0,
			NewBase:     s.base,
			Duration:    time.Since(start),
		}, nil
	}

	var prunedCount int64
	var bytesFreed int64
	newBase := beforeHeight

	// Iterate through blocks from base to beforeHeight
	iter := s.db.NewIterator(&util.Range{
		Start: makeHeightKey(s.base),
		Limit: makeHeightKey(beforeHeight),
	}, nil)
	defer iter.Release()

	batch := new(leveldb.Batch)
	batchSize := 0
	const maxBatchSize = 1000 // Write in batches to avoid memory issues

	for iter.Next() {
		heightKey := iter.Key()
		hash := iter.Value()

		// Extract height from key
		height := decodeInt64(heightKey[len(prefixHeight):])

		// Check if this block should be kept
		if s.pruneCfg != nil && s.pruneCfg.ShouldKeep(height, s.height) {
			// Track the lowest kept height as new base
			if height < newBase {
				newBase = height
			}
			continue
		}

		// Delete height -> hash mapping
		batch.Delete(heightKey)

		// Delete hash -> data mapping
		blockKey := makeBlockKey(hash)
		if data, err := s.db.Get(blockKey, nil); err == nil {
			bytesFreed += int64(len(data))
		}
		batch.Delete(blockKey)

		prunedCount++
		batchSize++

		// Write batch if it gets too large
		if batchSize >= maxBatchSize {
			if err := s.db.Write(batch, nil); err != nil {
				return nil, fmt.Errorf("writing prune batch: %w", err)
			}
			batch.Reset()
			batchSize = 0
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterating blocks: %w", err)
	}

	// Write any remaining batch
	if batchSize > 0 {
		if err := s.db.Write(batch, nil); err != nil {
			return nil, fmt.Errorf("writing final prune batch: %w", err)
		}
	}

	// Update base metadata if changed
	if newBase > s.base {
		if err := s.db.Put(keyMetaBase, encodeInt64(newBase), nil); err != nil {
			return nil, fmt.Errorf("updating base metadata: %w", err)
		}
		s.base = newBase
	}

	return &PruneResult{
		PrunedCount: prunedCount,
		NewBase:     s.base,
		BytesFreed:  bytesFreed,
		Duration:    time.Since(start),
	}, nil
}

// PruneConfig returns the current pruning configuration.
func (s *LevelDBBlockStore) PruneConfig() *PruneConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pruneCfg
}

// SetPruneConfig updates the pruning configuration.
func (s *LevelDBBlockStore) SetPruneConfig(cfg *PruneConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneCfg = cfg
}

// Compact triggers LevelDB compaction to reclaim space after pruning.
func (s *LevelDBBlockStore) Compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.CompactRange(util.Range{})
}

// Ensure LevelDBBlockStore implements PrunableBlockStore and CertificateBlockStore.
var _ PrunableBlockStore = (*LevelDBBlockStore)(nil)
var _ CertificateBlockStore = (*LevelDBBlockStore)(nil)

// SaveCertificate persists a DAG certificate with multiple indexes.
// Indexes: primary by digest, secondary by round+validator, tertiary by height+validator.
func (s *LevelDBBlockStore) SaveCertificate(cert *loosetypes.Certificate) error {
	if cert == nil {
		return fmt.Errorf("cannot save nil certificate")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create defensive copy of digest
	digest := cert.Digest()
	digestCopy := make([]byte, len(digest))
	copy(digestCopy, digest[:])

	// Check if certificate already exists
	certKey := makeCertKey(digestCopy)
	exists, err := s.db.Has(certKey, nil)
	if err != nil {
		return fmt.Errorf("checking certificate existence: %w", err)
	}
	if exists {
		return types.ErrCertificateAlreadyExists
	}

	// Marshal certificate with cramberry
	data, err := cramberry.Marshal(cert)
	if err != nil {
		return fmt.Errorf("failed to marshal certificate: %w", err)
	}

	// Create batch for atomic write
	batch := new(leveldb.Batch)

	// Primary key: C:<digest>
	batch.Put(certKey, data)

	// Round index: CR:<round>:<validator_idx>
	roundKey := makeCertRoundKey(cert.Round(), cert.Author())
	batch.Put(roundKey, digestCopy)

	// Write batch
	if err := s.db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("writing certificate: %w", err)
	}

	return nil
}

// GetCertificate retrieves a certificate by its digest.
func (s *LevelDBBlockStore) GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := makeCertKey(digest[:])
	data, err := s.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, types.ErrCertificateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting certificate: %w", err)
	}

	var cert loosetypes.Certificate
	if err := cramberry.Unmarshal(data, &cert); err != nil {
		return nil, fmt.Errorf("failed to unmarshal certificate: %w", err)
	}

	return &cert, nil
}

// GetCertificatesForRound retrieves all certificates for a given DAG round.
func (s *LevelDBBlockStore) GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := makeCertRoundPrefix(round)
	iter := s.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	var certs []*loosetypes.Certificate
	for iter.Next() {
		digest := iter.Value()
		cert, err := s.getCertificateUnlocked(digest)
		if err != nil {
			return nil, fmt.Errorf("loading certificate for round %d: %w", round, err)
		}
		certs = append(certs, cert)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterating certificates for round %d: %w", round, err)
	}

	// Sort by validator index for deterministic ordering
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].Author() < certs[j].Author()
	})

	return certs, nil
}

// GetCertificatesForHeight retrieves all certificates committed at a given block height.
func (s *LevelDBBlockStore) GetCertificatesForHeight(height int64) ([]*loosetypes.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prefix := makeCertHeightPrefix(height)
	iter := s.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	var certs []*loosetypes.Certificate
	for iter.Next() {
		digest := iter.Value()
		cert, err := s.getCertificateUnlocked(digest)
		if err != nil {
			return nil, fmt.Errorf("loading certificate for height %d: %w", height, err)
		}
		certs = append(certs, cert)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("iterating certificates for height %d: %w", height, err)
	}

	// Sort by validator index for deterministic ordering
	sort.Slice(certs, func(i, j int) bool {
		return certs[i].Author() < certs[j].Author()
	})

	return certs, nil
}

// SetCertificateBlockHeight updates the block height index for a certificate.
func (s *LevelDBBlockStore) SetCertificateBlockHeight(digest loosetypes.Hash, height int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify certificate exists
	certKey := makeCertKey(digest[:])
	data, err := s.db.Get(certKey, nil)
	if err == leveldb.ErrNotFound {
		return types.ErrCertificateNotFound
	}
	if err != nil {
		return fmt.Errorf("checking certificate: %w", err)
	}

	// Unmarshal to get validator index
	var cert loosetypes.Certificate
	if err := cramberry.Unmarshal(data, &cert); err != nil {
		return fmt.Errorf("failed to unmarshal certificate: %w", err)
	}

	// Create height index entry
	heightKey := makeCertHeightKey(height, cert.Author())
	digestCopy := make([]byte, len(digest))
	copy(digestCopy, digest[:])

	if err := s.db.Put(heightKey, digestCopy, &opt.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("setting certificate height index: %w", err)
	}

	return nil
}

// SaveBatch persists a transaction batch.
func (s *LevelDBBlockStore) SaveBatch(batch *loosetypes.Batch) error {
	if batch == nil {
		return fmt.Errorf("cannot save nil batch")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create defensive copy of digest
	digest := batch.Digest
	digestCopy := make([]byte, len(digest))
	copy(digestCopy, digest[:])

	// Check if batch already exists
	batchKey := makeBatchKey(digestCopy)
	exists, err := s.db.Has(batchKey, nil)
	if err != nil {
		return fmt.Errorf("checking batch existence: %w", err)
	}
	if exists {
		return types.ErrBatchAlreadyExists
	}

	// Marshal batch with cramberry
	data, err := cramberry.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	// Write batch data
	if err := s.db.Put(batchKey, data, &opt.WriteOptions{Sync: true}); err != nil {
		return fmt.Errorf("writing batch: %w", err)
	}

	return nil
}

// GetBatch retrieves a batch by its digest.
func (s *LevelDBBlockStore) GetBatch(digest loosetypes.Hash) (*loosetypes.Batch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := makeBatchKey(digest[:])
	data, err := s.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, types.ErrBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting batch: %w", err)
	}

	var batch loosetypes.Batch
	if err := cramberry.Unmarshal(data, &batch); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch: %w", err)
	}

	return &batch, nil
}

// getCertificateUnlocked retrieves a certificate without holding the lock.
// Caller must hold at least a read lock.
func (s *LevelDBBlockStore) getCertificateUnlocked(digest []byte) (*loosetypes.Certificate, error) {
	key := makeCertKey(digest)
	data, err := s.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, types.ErrCertificateNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting certificate: %w", err)
	}

	var cert loosetypes.Certificate
	if err := cramberry.Unmarshal(data, &cert); err != nil {
		return nil, fmt.Errorf("failed to unmarshal certificate: %w", err)
	}

	return &cert, nil
}

// Key encoding helpers

func makeHeightKey(height int64) []byte {
	key := make([]byte, len(prefixHeight)+8)
	copy(key, prefixHeight)
	binary.BigEndian.PutUint64(key[len(prefixHeight):], uint64(height)) //nolint:gosec // height is always non-negative
	return key
}

func makeBlockKey(hash []byte) []byte {
	key := make([]byte, len(prefixBlock)+len(hash))
	copy(key, prefixBlock)
	copy(key[len(prefixBlock):], hash)
	return key
}

func makeBlockValue(height int64, data []byte) []byte {
	value := make([]byte, 8+len(data))
	binary.BigEndian.PutUint64(value[:8], uint64(height)) //nolint:gosec // height is always non-negative
	copy(value[8:], data)
	return value
}

func parseBlockValue(value []byte) (height int64, data []byte) {
	if len(value) < 8 {
		return 0, nil
	}
	height = decodeInt64(value[:8])
	data = value[8:]
	return
}

func encodeInt64(v int64) []byte {
	buf := make([]byte, 8)
	// Block heights are always non-negative, so this conversion is safe
	binary.BigEndian.PutUint64(buf, uint64(v)) //nolint:gosec // height is always non-negative
	return buf
}

func decodeInt64(data []byte) int64 {
	if len(data) < 8 {
		return 0
	}
	// Block heights stored are always non-negative and fit in int64
	return int64(binary.BigEndian.Uint64(data)) //nolint:gosec // stored heights are always valid
}

// Certificate key helpers

// makeCertKey creates a key for storing certificate data.
// Format: C:<digest>
func makeCertKey(digest []byte) []byte {
	key := make([]byte, len(prefixCert)+len(digest))
	copy(key, prefixCert)
	copy(key[len(prefixCert):], digest)
	return key
}

// makeCertRoundKey creates a key for the round index.
// Format: CR:<round>:<validator_idx>
// Uses fixed-width encoding for proper lexicographic ordering.
func makeCertRoundKey(round uint64, validatorIdx uint16) []byte {
	// CR: (3 bytes) + round (8 bytes) + validator (2 bytes) = 13 bytes
	key := make([]byte, len(prefixCertRound)+8+2)
	copy(key, prefixCertRound)
	binary.BigEndian.PutUint64(key[len(prefixCertRound):], round)
	binary.BigEndian.PutUint16(key[len(prefixCertRound)+8:], validatorIdx)
	return key
}

// makeCertRoundPrefix creates a prefix for iterating certificates by round.
// Format: CR:<round>:
func makeCertRoundPrefix(round uint64) []byte {
	// CR: (3 bytes) + round (8 bytes) = 11 bytes
	key := make([]byte, len(prefixCertRound)+8)
	copy(key, prefixCertRound)
	binary.BigEndian.PutUint64(key[len(prefixCertRound):], round)
	return key
}

// makeCertHeightKey creates a key for the height index.
// Format: CH:<height>:<validator_idx>
// Uses fixed-width encoding for proper lexicographic ordering.
func makeCertHeightKey(height int64, validatorIdx uint16) []byte {
	// CH: (3 bytes) + height (8 bytes) + validator (2 bytes) = 13 bytes
	key := make([]byte, len(prefixCertHeight)+8+2)
	copy(key, prefixCertHeight)
	binary.BigEndian.PutUint64(key[len(prefixCertHeight):], uint64(height)) //nolint:gosec // height is always non-negative
	binary.BigEndian.PutUint16(key[len(prefixCertHeight)+8:], validatorIdx)
	return key
}

// makeCertHeightPrefix creates a prefix for iterating certificates by height.
// Format: CH:<height>:
func makeCertHeightPrefix(height int64) []byte {
	// CH: (3 bytes) + height (8 bytes) = 11 bytes
	key := make([]byte, len(prefixCertHeight)+8)
	copy(key, prefixCertHeight)
	binary.BigEndian.PutUint64(key[len(prefixCertHeight):], uint64(height)) //nolint:gosec // height is always non-negative
	return key
}

// makeBatchKey creates a key for storing batch data.
// Format: CB:<digest>
func makeBatchKey(digest []byte) []byte {
	key := make([]byte, len(prefixBatch)+len(digest))
	copy(key, prefixBatch)
	copy(key[len(prefixBatch):], digest)
	return key
}
