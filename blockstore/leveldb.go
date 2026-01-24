package blockstore

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/blockberries/blockberry/types"
)

// Key prefixes for LevelDB storage.
var (
	prefixHeight  = []byte("H:") // Height -> Hash mapping
	prefixBlock   = []byte("B:") // Hash -> Block data mapping
	keyMetaHeight = []byte("M:height")
	keyMetaBase   = []byte("M:base")
)

// LevelDBBlockStore implements BlockStore using LevelDB.
type LevelDBBlockStore struct {
	db     *leveldb.DB
	path   string
	height int64
	base   int64
	mu     sync.RWMutex
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
