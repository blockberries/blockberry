package abi

// TxIndexer provides transaction indexing and search capabilities.
// Implementations store transaction execution results and allow retrieval
// by hash or by searching indexed attributes.
type TxIndexer interface {
	// Index stores a transaction result for later retrieval.
	// The transaction is indexed by its hash and by any indexed attributes
	// in its events.
	Index(result *TxIndexResult) error

	// Get retrieves a transaction result by hash.
	// Returns ErrTxNotFound if the transaction is not indexed.
	Get(hash []byte) (*TxIndexResult, error)

	// Search finds transactions matching the query.
	// The query syntax is implementation-specific but typically supports:
	//   - "tx.height=100" - exact match
	//   - "tx.height>100" - comparison
	//   - "transfer.sender='addr'" - event attribute match
	// Returns matching results up to the limit.
	Search(query string, page, perPage int) ([]*TxIndexResult, int, error)

	// Delete removes a transaction from the index.
	// This is typically used during pruning.
	Delete(hash []byte) error

	// Has returns true if the transaction is indexed.
	Has(hash []byte) bool

	// Batch starts a batch indexing operation for efficiency.
	// Call Commit() on the returned batch when done.
	Batch() TxIndexBatch
}

// TxIndexResult contains a transaction and its execution metadata.
type TxIndexResult struct {
	// Hash is the transaction hash.
	Hash []byte

	// Height is the block height containing this transaction.
	Height uint64

	// Index is the transaction's index within the block.
	Index uint32

	// Result is the execution result including events.
	Result *TxResult
}

// TxIndexBatch allows batching multiple index operations.
type TxIndexBatch interface {
	// Add adds a transaction to the batch.
	Add(result *TxIndexResult) error

	// Delete marks a transaction for deletion.
	Delete(hash []byte) error

	// Commit writes all batched operations.
	Commit() error

	// Discard discards all batched operations.
	Discard()

	// Size returns the number of operations in the batch.
	Size() int
}

// BlockIndexer provides block indexing capabilities.
type BlockIndexer interface {
	// IndexBlock indexes a block's events.
	IndexBlock(height uint64, events []Event) error

	// SearchBlocks finds blocks matching the query.
	SearchBlocks(query string, page, perPage int) ([]uint64, int, error)

	// Has returns true if the block is indexed.
	Has(height uint64) bool
}

// IndexerConfig contains configuration for indexers.
type IndexerConfig struct {
	// Indexer specifies the indexer type ("kv", "psql", "null").
	Indexer string

	// DBPath is the path to the index database (for "kv" indexer).
	DBPath string

	// PSQLConn is the PostgreSQL connection string (for "psql" indexer).
	PSQLConn string

	// IndexAllEvents indexes all events, not just those with Index=true.
	IndexAllEvents bool
}

// DefaultIndexerConfig returns sensible defaults for indexer configuration.
func DefaultIndexerConfig() IndexerConfig {
	return IndexerConfig{
		Indexer:        "kv",
		DBPath:         "data/tx_index.db",
		IndexAllEvents: false,
	}
}

// NullTxIndexer is a no-op indexer that doesn't store anything.
// Use this when transaction indexing is not needed.
type NullTxIndexer struct{}

// Index is a no-op.
func (i *NullTxIndexer) Index(result *TxIndexResult) error {
	return nil
}

// Get always returns ErrTxNotFound.
func (i *NullTxIndexer) Get(hash []byte) (*TxIndexResult, error) {
	return nil, ErrTxNotFound
}

// Search always returns empty results.
func (i *NullTxIndexer) Search(query string, page, perPage int) ([]*TxIndexResult, int, error) {
	return nil, 0, nil
}

// Delete is a no-op.
func (i *NullTxIndexer) Delete(hash []byte) error {
	return nil
}

// Has always returns false.
func (i *NullTxIndexer) Has(hash []byte) bool {
	return false
}

// Batch returns a no-op batch.
func (i *NullTxIndexer) Batch() TxIndexBatch {
	return &nullBatch{}
}

// nullBatch is a no-op batch.
type nullBatch struct{}

func (b *nullBatch) Add(result *TxIndexResult) error { return nil }
func (b *nullBatch) Delete(hash []byte) error        { return nil }
func (b *nullBatch) Commit() error                   { return nil }
func (b *nullBatch) Discard()                        {}
func (b *nullBatch) Size() int                       { return 0 }

// Ensure NullTxIndexer implements TxIndexer.
var _ TxIndexer = (*NullTxIndexer)(nil)

// Common indexer errors.
var (
	ErrTxNotFound     = &IndexError{Code: 1, Message: "transaction not found"}
	ErrInvalidQuery   = &IndexError{Code: 2, Message: "invalid query"}
	ErrIndexFull      = &IndexError{Code: 3, Message: "index storage full"}
	ErrIndexCorrupted = &IndexError{Code: 4, Message: "index corrupted"}
)

// IndexError represents an indexer error.
type IndexError struct {
	Code    int
	Message string
}

// Error implements the error interface.
func (e *IndexError) Error() string {
	return e.Message
}
