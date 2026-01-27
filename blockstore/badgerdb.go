package blockstore

import (
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"

	"github.com/blockberries/blockberry/types"
)

// BadgerDBBlockStore implements BlockStore using BadgerDB.
// BadgerDB is optimized for SSDs and offers better write performance
// than LevelDB for certain workloads.
type BadgerDBBlockStore struct {
	db       *badger.DB
	path     string
	height   int64
	base     int64
	pruneCfg *PruneConfig
	pruning  bool
	mu       sync.RWMutex
}

// BadgerDBOptions contains configuration options for BadgerDB.
type BadgerDBOptions struct {
	// SyncWrites ensures durability by syncing writes to disk.
	// Default: true
	SyncWrites bool

	// Compression enables Snappy compression for values.
	// Default: true
	Compression bool

	// ValueLogFileSize is the maximum size of a single value log file.
	// Default: 1GB
	ValueLogFileSize int64

	// MemTableSize is the size of the memtable.
	// Default: 64MB
	MemTableSize int64

	// NumMemtables is the number of memtables to keep in memory.
	// Default: 5
	NumMemtables int

	// NumLevelZeroTables is the maximum number of Level 0 tables before
	// compaction starts.
	// Default: 5
	NumLevelZeroTables int

	// NumLevelZeroTablesStall is the number of Level 0 tables that
	// triggers write stalling.
	// Default: 15
	NumLevelZeroTablesStall int

	// Logger is an optional logger for BadgerDB.
	// If nil, logging is disabled.
	Logger badger.Logger
}

// DefaultBadgerDBOptions returns sensible default options.
func DefaultBadgerDBOptions() *BadgerDBOptions {
	return &BadgerDBOptions{
		SyncWrites:              true,
		Compression:             true,
		ValueLogFileSize:        1 << 30, // 1GB
		MemTableSize:            64 << 20, // 64MB
		NumMemtables:            5,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 15,
	}
}

// NewBadgerDBBlockStore creates a new BadgerDB-backed block store.
func NewBadgerDBBlockStore(path string) (*BadgerDBBlockStore, error) {
	return NewBadgerDBBlockStoreWithOptions(path, DefaultBadgerDBOptions())
}

// NewBadgerDBBlockStoreWithOptions creates a new BadgerDB-backed block store
// with custom options.
func NewBadgerDBBlockStoreWithOptions(path string, opts *BadgerDBOptions) (*BadgerDBBlockStore, error) {
	if opts == nil {
		opts = DefaultBadgerDBOptions()
	}

	badgerOpts := badger.DefaultOptions(path)
	badgerOpts = badgerOpts.WithSyncWrites(opts.SyncWrites)
	badgerOpts = badgerOpts.WithValueLogFileSize(opts.ValueLogFileSize)
	badgerOpts = badgerOpts.WithMemTableSize(opts.MemTableSize)
	badgerOpts = badgerOpts.WithNumMemtables(opts.NumMemtables)
	badgerOpts = badgerOpts.WithNumLevelZeroTables(opts.NumLevelZeroTables)
	badgerOpts = badgerOpts.WithNumLevelZeroTablesStall(opts.NumLevelZeroTablesStall)

	if opts.Compression {
		badgerOpts = badgerOpts.WithCompression(options.Snappy)
	} else {
		badgerOpts = badgerOpts.WithCompression(options.None)
	}

	if opts.Logger != nil {
		badgerOpts = badgerOpts.WithLogger(opts.Logger)
	} else {
		badgerOpts = badgerOpts.WithLogger(nil)
	}

	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("opening badgerdb: %w", err)
	}

	store := &BadgerDBBlockStore{
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
func (s *BadgerDBBlockStore) loadMetadata() error {
	return s.db.View(func(txn *badger.Txn) error {
		// Load height
		item, err := txn.Get(keyMetaHeight)
		if err == nil {
			err = item.Value(func(val []byte) error {
				s.height = decodeInt64(val)
				return nil
			})
			if err != nil {
				return err
			}
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		// Load base
		item, err = txn.Get(keyMetaBase)
		if err == nil {
			err = item.Value(func(val []byte) error {
				s.base = decodeInt64(val)
				return nil
			})
			if err != nil {
				return err
			}
		} else if err != badger.ErrKeyNotFound {
			return err
		}

		return nil
	})
}

// SaveBlock persists a block at the given height.
func (s *BadgerDBBlockStore) SaveBlock(height int64, hash []byte, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if block already exists
	heightKey := makeHeightKey(height)
	exists := false
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(heightKey)
		if err == nil {
			exists = true
		} else if err != badger.ErrKeyNotFound {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("checking block existence: %w", err)
	}
	if exists {
		return types.ErrBlockAlreadyExists
	}

	// Use transaction for atomic write
	err = s.db.Update(func(txn *badger.Txn) error {
		// Store height -> hash mapping
		if err := txn.Set(heightKey, hash); err != nil {
			return err
		}

		// Store hash -> data mapping (include height in value for reverse lookup)
		blockKey := makeBlockKey(hash)
		blockValue := makeBlockValue(height, data)
		if err := txn.Set(blockKey, blockValue); err != nil {
			return err
		}

		// Update metadata
		if height > s.height {
			if err := txn.Set(keyMetaHeight, encodeInt64(height)); err != nil {
				return err
			}
		}
		if s.base == 0 || height < s.base {
			if err := txn.Set(keyMetaBase, encodeInt64(height)); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
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
func (s *BadgerDBBlockStore) LoadBlock(height int64) ([]byte, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var hash, data []byte

	err := s.db.View(func(txn *badger.Txn) error {
		// Get hash from height
		heightKey := makeHeightKey(height)
		item, err := txn.Get(heightKey)
		if err == badger.ErrKeyNotFound {
			return types.ErrBlockNotFound
		}
		if err != nil {
			return fmt.Errorf("getting hash for height %d: %w", height, err)
		}

		hash, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}

		// Get data from hash
		blockKey := makeBlockKey(hash)
		item, err = txn.Get(blockKey)
		if err == badger.ErrKeyNotFound {
			return types.ErrBlockNotFound
		}
		if err != nil {
			return fmt.Errorf("getting block data: %w", err)
		}

		blockValue, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		_, data = parseBlockValue(blockValue)
		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return hash, data, nil
}

// LoadBlockByHash retrieves a block by its hash.
func (s *BadgerDBBlockStore) LoadBlockByHash(hash []byte) (int64, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var height int64
	var data []byte

	err := s.db.View(func(txn *badger.Txn) error {
		blockKey := makeBlockKey(hash)
		item, err := txn.Get(blockKey)
		if err == badger.ErrKeyNotFound {
			return types.ErrBlockNotFound
		}
		if err != nil {
			return fmt.Errorf("getting block by hash: %w", err)
		}

		blockValue, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		height, data = parseBlockValue(blockValue)
		return nil
	})

	if err != nil {
		return 0, nil, err
	}

	return height, data, nil
}

// HasBlock checks if a block exists at the given height.
func (s *BadgerDBBlockStore) HasBlock(height int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	heightKey := makeHeightKey(height)
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get(heightKey)
		return err
	})
	return err == nil
}

// Height returns the latest block height.
func (s *BadgerDBBlockStore) Height() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.height
}

// Base returns the earliest available block height.
func (s *BadgerDBBlockStore) Base() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.base
}

// Close closes the database.
func (s *BadgerDBBlockStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// BlockCount returns the number of blocks stored.
func (s *BadgerDBBlockStore) BlockCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	_ = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefixHeight

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	return count
}

// Prune removes blocks before the given height.
// Blocks that should be kept according to the prune config are preserved.
func (s *BadgerDBBlockStore) Prune(beforeHeight int64) (*PruneResult, error) {
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

	// Collect keys to delete
	type deleteEntry struct {
		heightKey []byte
		blockKey  []byte
		dataSize  int
	}
	var toDelete []deleteEntry

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = prefixHeight

		it := txn.NewIterator(opts)
		defer it.Close()

		startKey := makeHeightKey(s.base)
		endKey := makeHeightKey(beforeHeight)

		for it.Seek(startKey); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			// Check if we've gone past the end
			if compareKeys(key, endKey) >= 0 {
				break
			}

			// Extract height from key
			height := decodeInt64(key[len(prefixHeight):])

			// Check if this block should be kept
			if s.pruneCfg != nil && s.pruneCfg.ShouldKeep(height, s.height) {
				// Track the lowest kept height as new base
				if height < newBase {
					newBase = height
				}
				continue
			}

			// Get hash from value
			hash, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			// Get data size
			blockKey := makeBlockKey(hash)
			dataSize := 0
			blockItem, err := txn.Get(blockKey)
			if err == nil {
				dataSize = int(blockItem.ValueSize())
			}

			toDelete = append(toDelete, deleteEntry{
				heightKey: append([]byte{}, key...),
				blockKey:  blockKey,
				dataSize:  dataSize,
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collecting blocks to prune: %w", err)
	}

	// Delete in batches using WriteBatch
	const batchSize = 1000
	for i := 0; i < len(toDelete); i += batchSize {
		end := i + batchSize
		if end > len(toDelete) {
			end = len(toDelete)
		}

		wb := s.db.NewWriteBatch()
		for _, entry := range toDelete[i:end] {
			if err := wb.Delete(entry.heightKey); err != nil {
				wb.Cancel()
				return nil, fmt.Errorf("deleting height key: %w", err)
			}
			if err := wb.Delete(entry.blockKey); err != nil {
				wb.Cancel()
				return nil, fmt.Errorf("deleting block key: %w", err)
			}
			bytesFreed += int64(entry.dataSize)
			prunedCount++
		}

		if err := wb.Flush(); err != nil {
			return nil, fmt.Errorf("flushing prune batch: %w", err)
		}
	}

	// Update base metadata if changed
	if newBase > s.base {
		err := s.db.Update(func(txn *badger.Txn) error {
			return txn.Set(keyMetaBase, encodeInt64(newBase))
		})
		if err != nil {
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
func (s *BadgerDBBlockStore) PruneConfig() *PruneConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pruneCfg
}

// SetPruneConfig updates the pruning configuration.
func (s *BadgerDBBlockStore) SetPruneConfig(cfg *PruneConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneCfg = cfg
}

// Compact triggers BadgerDB garbage collection to reclaim space.
// BadgerDB uses a value log, so GC is needed to reclaim space from deleted values.
func (s *BadgerDBBlockStore) Compact() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Run value log GC with 0.5 discard ratio
	// This means files with 50%+ garbage will be rewritten
	for {
		err := s.db.RunValueLogGC(0.5)
		if err == badger.ErrNoRewrite {
			// No more GC needed
			break
		}
		if err != nil {
			return fmt.Errorf("running value log gc: %w", err)
		}
	}

	return nil
}

// Sync ensures all data is flushed to disk.
func (s *BadgerDBBlockStore) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Sync()
}

// Size returns the approximate size of the database on disk.
func (s *BadgerDBBlockStore) Size() (int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lsmSize, vlogSize := s.db.Size()
	return lsmSize, vlogSize
}

// compareKeys compares two byte slices lexicographically.
func compareKeys(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// Ensure BadgerDBBlockStore implements PrunableBlockStore.
var _ PrunableBlockStore = (*BadgerDBBlockStore)(nil)

// BadgerDBIterator provides an iterator over BadgerDB blocks.
type BadgerDBIterator struct {
	txn    *badger.Txn
	it     *badger.Iterator
	store  *BadgerDBBlockStore
	prefix []byte
}

// NewBlockIterator creates an iterator over all blocks.
func (s *BadgerDBBlockStore) NewBlockIterator() *BadgerDBIterator {
	txn := s.db.NewTransaction(false)
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = true
	opts.Prefix = prefixHeight

	it := txn.NewIterator(opts)
	it.Rewind()

	return &BadgerDBIterator{
		txn:    txn,
		it:     it,
		store:  s,
		prefix: prefixHeight,
	}
}

// Valid returns true if the iterator is positioned at a valid item.
func (i *BadgerDBIterator) Valid() bool {
	return i.it.Valid()
}

// Next advances the iterator.
func (i *BadgerDBIterator) Next() {
	i.it.Next()
}

// Height returns the height of the current block.
func (i *BadgerDBIterator) Height() int64 {
	if !i.it.Valid() {
		return 0
	}
	key := i.it.Item().Key()
	return decodeInt64(key[len(i.prefix):])
}

// Hash returns the hash of the current block.
func (i *BadgerDBIterator) Hash() []byte {
	if !i.it.Valid() {
		return nil
	}
	hash, _ := i.it.Item().ValueCopy(nil)
	return hash
}

// Close releases resources held by the iterator.
func (i *BadgerDBIterator) Close() {
	i.it.Close()
	i.txn.Discard()
}
