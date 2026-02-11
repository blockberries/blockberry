// Package kv provides a LevelDB-based transaction indexer.
package kv

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"

	"github.com/blockberries/blockberry/pkg/indexer"
)

// Key prefixes for different index types.
var (
	// Primary index: hash -> TxIndexResult
	prefixTxByHash = []byte("th/")

	// Height index: height -> list of tx hashes
	prefixTxByHeight = []byte("ht/")

	// Event attribute index: event.key/value -> list of tx hashes
	prefixTxByEvent = []byte("ev/")
)

// TxIndexer implements indexer.TxIndexer using LevelDB.
type TxIndexer struct {
	db     *leveldb.DB
	path   string
	closed bool
	mu     sync.RWMutex

	// Configuration
	indexAllEvents bool
}

// NewTxIndexer creates a new KV-based transaction indexer.
func NewTxIndexer(path string, indexAllEvents bool) (*TxIndexer, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, fmt.Errorf("opening leveldb: %w", err)
	}

	return &TxIndexer{
		db:             db,
		path:           path,
		indexAllEvents: indexAllEvents,
	}, nil
}

// Index stores a transaction result for later retrieval.
func (idx *TxIndexer) Index(result *indexer.TxIndexResult) error {
	if result == nil || len(result.Hash) == 0 {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return indexer.ErrIndexCorrupted
	}

	batch := new(leveldb.Batch)

	// Store primary record
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}
	batch.Put(txHashKey(result.Hash), data)

	// Index by height
	heightKey := txHeightKey(result.Height, result.Hash)
	batch.Put(heightKey, result.Hash)

	// Index events
	if result.Result != nil {
		for _, event := range result.Result.Events {
			for _, attr := range event.Attributes {
				if idx.indexAllEvents || attr.Index {
					eventKey := txEventKey(event.Kind, attr.Key, attr.Value, result.Hash)
					batch.Put(eventKey, result.Hash)
				}
			}
		}
	}

	return idx.db.Write(batch, nil)
}

// Get retrieves a transaction result by hash.
func (idx *TxIndexer) Get(hash []byte) (*indexer.TxIndexResult, error) {
	if len(hash) == 0 {
		return nil, indexer.ErrTxNotFound
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, indexer.ErrIndexCorrupted
	}

	data, err := idx.db.Get(txHashKey(hash), nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, indexer.ErrTxNotFound
		}
		return nil, fmt.Errorf("getting tx: %w", err)
	}

	var result indexer.TxIndexResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling result: %w", err)
	}

	return &result, nil
}

// Search finds transactions matching the query.
// Supported query formats:
//   - "tx.height=100" - exact height match
//   - "tx.height>100" - height greater than
//   - "tx.height<100" - height less than
//   - "tx.height>=100" - height greater than or equal
//   - "tx.height<=100" - height less than or equal
//   - "transfer.sender='value'" - event attribute match
func (idx *TxIndexer) Search(query string, page, perPage int) ([]*indexer.TxIndexResult, int, error) {
	if query == "" {
		return nil, 0, indexer.ErrInvalidQuery
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return nil, 0, indexer.ErrIndexCorrupted
	}

	// Parse query
	hashes, err := idx.executeQuery(query)
	if err != nil {
		return nil, 0, err
	}

	total := len(hashes)
	if total == 0 {
		return nil, 0, nil
	}

	// Apply pagination
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 30
	}
	if perPage > 100 {
		perPage = 100
	}

	start := (page - 1) * perPage
	if start >= total {
		return nil, total, nil
	}

	end := start + perPage
	if end > total {
		end = total
	}

	// Fetch results
	results := make([]*indexer.TxIndexResult, 0, end-start)
	for _, hash := range hashes[start:end] {
		result, err := idx.getUnlocked(hash)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, total, nil
}

// executeQuery parses and executes a query, returning matching tx hashes.
func (idx *TxIndexer) executeQuery(query string) ([][]byte, error) {
	query = strings.TrimSpace(query)

	// Parse query condition
	var op string
	var parts []string

	for _, operator := range []string{">=", "<=", "!=", "=", ">", "<"} {
		if strings.Contains(query, operator) {
			parts = strings.SplitN(query, operator, 2)
			op = operator
			break
		}
	}

	if len(parts) != 2 {
		return nil, indexer.ErrInvalidQuery
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Remove quotes from value
	value = strings.Trim(value, "'\"")

	// Handle height queries
	if key == "tx.height" {
		return idx.searchByHeight(op, value)
	}

	// Handle event queries (format: "event_type.attribute_key")
	eventParts := strings.SplitN(key, ".", 2)
	if len(eventParts) == 2 {
		return idx.searchByEvent(eventParts[0], eventParts[1], op, value)
	}

	return nil, indexer.ErrInvalidQuery
}

// searchByHeight searches for transactions by block height.
func (idx *TxIndexer) searchByHeight(op, value string) ([][]byte, error) {
	height, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return nil, indexer.ErrInvalidQuery
	}

	var hashes [][]byte

	switch op {
	case "=":
		// Exact height match
		prefix := append(prefixTxByHeight, encodeHeight(height)...)
		iter := idx.db.NewIterator(util.BytesPrefix(prefix), nil)
		defer iter.Release()
		for iter.Next() {
			hashes = append(hashes, copyBytes(iter.Value()))
		}

	case ">":
		// Greater than
		start := append(prefixTxByHeight, encodeHeight(height+1)...)
		end := append(prefixTxByHeight, encodeHeight(^uint64(0))...)
		iter := idx.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
		defer iter.Release()
		for iter.Next() {
			hashes = append(hashes, copyBytes(iter.Value()))
		}

	case ">=":
		// Greater than or equal
		start := append(prefixTxByHeight, encodeHeight(height)...)
		end := append(prefixTxByHeight, encodeHeight(^uint64(0))...)
		iter := idx.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
		defer iter.Release()
		for iter.Next() {
			hashes = append(hashes, copyBytes(iter.Value()))
		}

	case "<":
		// Less than
		start := prefixTxByHeight
		end := append(prefixTxByHeight, encodeHeight(height)...)
		iter := idx.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
		defer iter.Release()
		for iter.Next() {
			hashes = append(hashes, copyBytes(iter.Value()))
		}

	case "<=":
		// Less than or equal
		start := prefixTxByHeight
		end := append(prefixTxByHeight, encodeHeight(height+1)...)
		iter := idx.db.NewIterator(&util.Range{Start: start, Limit: end}, nil)
		defer iter.Release()
		for iter.Next() {
			hashes = append(hashes, copyBytes(iter.Value()))
		}

	default:
		return nil, indexer.ErrInvalidQuery
	}

	return hashes, nil
}

// searchByEvent searches for transactions by event attribute.
func (idx *TxIndexer) searchByEvent(eventType, attrKey, op, value string) ([][]byte, error) {
	if op != "=" {
		return nil, indexer.ErrInvalidQuery
	}

	// Build prefix for the event search
	prefix := append(prefixTxByEvent, []byte(eventType+"/"+attrKey+"/"+value+"/")...)

	var hashes [][]byte
	iter := idx.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	for iter.Next() {
		hashes = append(hashes, copyBytes(iter.Value()))
	}

	return hashes, nil
}

// getUnlocked retrieves a result without locking (caller must hold lock).
func (idx *TxIndexer) getUnlocked(hash []byte) (*indexer.TxIndexResult, error) {
	data, err := idx.db.Get(txHashKey(hash), nil)
	if err != nil {
		return nil, err
	}

	var result indexer.TxIndexResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Delete removes a transaction from the index.
func (idx *TxIndexer) Delete(hash []byte) error {
	if len(hash) == 0 {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return indexer.ErrIndexCorrupted
	}

	// Get the result first to clean up secondary indexes
	result, err := idx.getUnlocked(hash)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil
		}
		return err
	}

	batch := new(leveldb.Batch)

	// Delete primary record
	batch.Delete(txHashKey(hash))

	// Delete height index
	heightKey := txHeightKey(result.Height, hash)
	batch.Delete(heightKey)

	// Delete event indexes
	if result.Result != nil {
		for _, event := range result.Result.Events {
			for _, attr := range event.Attributes {
				if idx.indexAllEvents || attr.Index {
					eventKey := txEventKey(event.Kind, attr.Key, attr.Value, hash)
					batch.Delete(eventKey)
				}
			}
		}
	}

	return idx.db.Write(batch, nil)
}

// Has returns true if the transaction is indexed.
func (idx *TxIndexer) Has(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.closed {
		return false
	}

	ok, _ := idx.db.Has(txHashKey(hash), nil)
	return ok
}

// Batch starts a batch indexing operation.
func (idx *TxIndexer) Batch() indexer.TxIndexBatch {
	return &txBatch{
		idx:     idx,
		batch:   new(leveldb.Batch),
		results: make([]*indexer.TxIndexResult, 0),
		deletes: make([][]byte, 0),
	}
}

// Close closes the indexer.
func (idx *TxIndexer) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.closed {
		return nil
	}

	idx.closed = true
	return idx.db.Close()
}

// txBatch implements indexer.TxIndexBatch.
type txBatch struct {
	idx     *TxIndexer
	batch   *leveldb.Batch
	results []*indexer.TxIndexResult
	deletes [][]byte
	mu      sync.Mutex
}

// Add adds a transaction to the batch.
func (b *txBatch) Add(result *indexer.TxIndexResult) error {
	if result == nil || len(result.Hash) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Store for later processing
	b.results = append(b.results, result)

	// Add to batch
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}
	b.batch.Put(txHashKey(result.Hash), data)

	// Index by height
	heightKey := txHeightKey(result.Height, result.Hash)
	b.batch.Put(heightKey, result.Hash)

	// Index events
	if result.Result != nil {
		for _, event := range result.Result.Events {
			for _, attr := range event.Attributes {
				if b.idx.indexAllEvents || attr.Index {
					eventKey := txEventKey(event.Kind, attr.Key, attr.Value, result.Hash)
					b.batch.Put(eventKey, result.Hash)
				}
			}
		}
	}

	return nil
}

// Delete marks a transaction for deletion.
func (b *txBatch) Delete(hash []byte) error {
	if len(hash) == 0 {
		return nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.deletes = append(b.deletes, hash)
	b.batch.Delete(txHashKey(hash))

	return nil
}

// Commit writes all batched operations.
func (b *txBatch) Commit() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.idx.mu.Lock()
	defer b.idx.mu.Unlock()

	if b.idx.closed {
		return indexer.ErrIndexCorrupted
	}

	// Process deletes - need to clean up secondary indexes
	for _, hash := range b.deletes {
		result, err := b.idx.getUnlocked(hash)
		if err != nil {
			continue
		}

		// Delete height index
		heightKey := txHeightKey(result.Height, hash)
		b.batch.Delete(heightKey)

		// Delete event indexes
		if result.Result != nil {
			for _, event := range result.Result.Events {
				for _, attr := range event.Attributes {
					if b.idx.indexAllEvents || attr.Index {
						eventKey := txEventKey(event.Kind, attr.Key, attr.Value, hash)
						b.batch.Delete(eventKey)
					}
				}
			}
		}
	}

	return b.idx.db.Write(b.batch, nil)
}

// Discard discards all batched operations.
func (b *txBatch) Discard() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.batch.Reset()
	b.results = nil
	b.deletes = nil
}

// Size returns the number of operations in the batch.
func (b *txBatch) Size() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.results) + len(b.deletes)
}

// Key construction helpers

func txHashKey(hash []byte) []byte {
	return append(prefixTxByHash, hash...)
}

func txHeightKey(height uint64, hash []byte) []byte {
	key := make([]byte, len(prefixTxByHeight)+8+len(hash))
	copy(key, prefixTxByHeight)
	binary.BigEndian.PutUint64(key[len(prefixTxByHeight):], height)
	copy(key[len(prefixTxByHeight)+8:], hash)
	return key
}

func txEventKey(eventKind, attrKey, attrValue string, hash []byte) []byte {
	// Format: ev/<event_kind>/<attr_key>/<attr_value>/<hash>
	var buf bytes.Buffer
	buf.Write(prefixTxByEvent)
	buf.WriteString(eventKind)
	buf.WriteByte('/')
	buf.WriteString(attrKey)
	buf.WriteByte('/')
	buf.WriteString(attrValue)
	buf.WriteByte('/')
	buf.Write(hash)
	return buf.Bytes()
}

func encodeHeight(height uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, height)
	return b
}

func copyBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// Ensure TxIndexer implements indexer.TxIndexer.
var _ indexer.TxIndexer = (*TxIndexer)(nil)
