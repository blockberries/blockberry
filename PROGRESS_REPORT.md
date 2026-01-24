# Blockberry Progress Report

This file tracks implementation progress. Each completed task should have a summary appended here before committing.

---

## [Phase 1] Project Setup

**Status:** Completed

**Files Created:**
- `go.mod` - Go module definition with all dependencies
- `go.sum` - Dependency checksums
- `Makefile` - Build automation (build, test, test-race, lint, generate, clean, coverage, check)
- `.golangci.yml` - Linter configuration
- `deps.go` - Dependency tracking file (build-tagged)
- `blockstore/doc.go` - Package documentation stub
- `config/doc.go` - Package documentation stub
- `handlers/doc.go` - Package documentation stub
- `mempool/doc.go` - Package documentation stub
- `node/doc.go` - Package documentation stub
- `p2p/doc.go` - Package documentation stub
- `pex/doc.go` - Package documentation stub
- `statestore/doc.go` - Package documentation stub
- `sync/doc.go` - Package documentation stub
- `types/doc.go` - Package documentation stub

**Functionality Implemented:**
- Go module initialized with dependencies:
  - `github.com/blockberries/glueberry` (P2P networking)
  - `github.com/blockberries/cramberry` (serialization)
  - `github.com/cosmos/iavl` (merkleized state store)
  - `github.com/syndtr/goleveldb` (block storage backend)
  - `github.com/BurntSushi/toml` (configuration)
  - `github.com/stretchr/testify` (testing)
- Makefile with targets: build, test, test-race, lint, fmt, vet, generate, clean, coverage, tidy, check
- Directory structure created per ARCHITECTURE.md
- golangci-lint configuration with appropriate linters enabled

**Test Coverage:**
- No tests yet (Phase 1 is setup only)
- Build, test, and lint all pass

**Design Decisions:**
- Using local replace directives for glueberry and cramberry during development
- deps.go uses build tag to avoid being compiled into binaries while tracking dependencies
- golangci-lint configured to exclude generated schema/ directory and test files from certain linters

---

## [Phase 2] Configuration

**Status:** Completed

**Files Created:**
- `config/config.go` - Complete configuration system

**Files Modified:**
- `config/doc.go` - Removed (replaced by config.go)
- `schema/blockberry.go` - Formatted by gofmt

**Functionality Implemented:**
- Configuration structures for all components:
  - `Config` - Main configuration aggregating all sub-configs
  - `NodeConfig` - chain_id, protocol_version, private_key_path
  - `NetworkConfig` - listen_addrs, max_peers, timeouts, seeds
  - `PEXConfig` - enabled, request_interval, max_addresses
  - `MempoolConfig` - max_txs, max_bytes, cache_size
  - `BlockStoreConfig` - backend (leveldb/badgerdb), path
  - `StateStoreConfig` - path, cache_size
  - `HousekeepingConfig` - latency_probe_interval
- TOML configuration loading with `LoadConfig(path)`
- Default configuration with `DefaultConfig()`
- Configuration writing with `WriteConfigFile(path, cfg)`
- Comprehensive validation for all config sections
- Custom `Duration` type for TOML-compatible time.Duration parsing
- `EnsureDataDirs()` to create required directories

**Test Coverage:**
- 18 test functions with 40+ test cases
- Tests for: default config, loading, partial loading, file not found, invalid TOML,
  validation errors, all config section validations, write/read round-trip,
  Duration marshaling/unmarshaling, directory creation
- All tests pass with race detection

**Design Decisions:**
- Duration type wraps time.Duration for proper TOML serialization (e.g., "30s")
- Validation only applies to enabled features (e.g., PEX fields only validated if enabled)
- Zero cache sizes are valid (disables caching)
- Zero max peers is valid (node can be isolated)
- Sentinel errors allow callers to check specific validation failures with errors.Is()

---

## [Phase 3] Types & Errors

**Status:** Completed

**Files Created:**
- `types/types.go` - Common type definitions (Height, Hash, Tx, Block, PeerID)
- `types/errors.go` - Sentinel error definitions for all components
- `types/hash.go` - Hash functions using SHA-256
- `types/types_test.go` - Tests for type methods
- `types/hash_test.go` - Tests for hash functions
- `types/errors_test.go` - Tests for error handling

**Files Modified:**
- `types/doc.go` - Removed (replaced by types.go)

**Functionality Implemented:**

Common Types:
- `Height` - Block height with String() and Int64() methods
- `Hash` - Cryptographic hash with String(), Bytes(), IsEmpty(), Equal() methods
- `Tx` - Opaque transaction bytes with String(), Bytes(), Size() methods
- `Block` - Opaque block bytes with String(), Bytes(), Size() methods
- `PeerID` - Peer identifier with String() and IsEmpty() methods
- `HashFromHex()` - Parse hash from hexadecimal string

Hash Functions:
- `HashTx(tx)` - SHA-256 hash of transaction
- `HashBlock(block)` - SHA-256 hash of block
- `HashBytes(data)` - SHA-256 hash of arbitrary bytes
- `HashConcat(left, right)` - Hash concatenation for merkle trees
- `EmptyHash()` - Hash of empty byte slice
- `HashSize` constant (32 bytes)

Sentinel Errors (32 total):
- Peer errors: ErrPeerNotFound, ErrPeerBlacklisted, ErrPeerAlreadyConnected, ErrMaxPeersReached
- Block errors: ErrBlockNotFound, ErrBlockAlreadyExists, ErrInvalidBlockHeight, ErrInvalidBlockHash
- Transaction errors: ErrTxNotFound, ErrTxAlreadyExists, ErrInvalidTx, ErrTxTooLarge
- Mempool errors: ErrMempoolFull, ErrMempoolClosed
- Connection errors: ErrChainIDMismatch, ErrVersionMismatch, ErrHandshakeFailed, ErrHandshakeTimeout, ErrConnectionClosed, ErrNotConnected
- Message errors: ErrInvalidMessage, ErrUnknownMessageType, ErrMessageTooLarge
- State errors: ErrKeyNotFound, ErrStoreClosed, ErrInvalidProof
- Sync errors: ErrAlreadySyncing, ErrNotSyncing, ErrSyncFailed
- Node errors: ErrNodeNotStarted, ErrNodeAlreadyStarted, ErrNodeStopped

**Test Coverage:**
- 14 test functions with 60+ test cases
- Tests for all type methods and edge cases
- Hash function tests verify determinism and SHA-256 correctness
- Error tests verify errors.Is() compatibility and distinctness
- Benchmarks for hash operations
- All tests pass with race detection

**Design Decisions:**
- Hash functions return nil for nil input (not empty hash)
- String() methods truncate long values for readability
- All errors are distinct and work with errors.Is() and fmt.Errorf wrapping
- Using SHA-256 for all hashing (standard, 32-byte output)

---

## [Phase 4] Block Store

**Status:** Completed

**Files Created:**
- `blockstore/store.go` - BlockStore interface definition
- `blockstore/leveldb.go` - LevelDB-backed implementation
- `blockstore/leveldb_test.go` - Comprehensive test suite with benchmarks

**Files Modified:**
- `blockstore/doc.go` - Removed (replaced by store.go)

**Functionality Implemented:**

BlockStore Interface (`store.go`):
- `SaveBlock(height, hash, data)` - Persist block at height
- `LoadBlock(height)` - Retrieve block by height (returns hash, data)
- `LoadBlockByHash(hash)` - Retrieve block by hash (returns height, data)
- `HasBlock(height)` - Check if block exists
- `Height()` - Get latest block height
- `Base()` - Get earliest available block height
- `Close()` - Release resources
- `BlockInfo` struct for block metadata

LevelDB Implementation (`leveldb.go`):
- Key prefixes: `H:` for height→hash, `B:` for hash→data, `M:` for metadata
- Atomic batch writes with `leveldb.Batch`
- Sync writes for durability (`Sync: true`)
- Metadata persistence (height, base) across restarts
- Thread-safe with `sync.RWMutex`
- Helper functions: `makeHeightKey`, `makeBlockKey`, `makeBlockValue`, `parseBlockValue`
- Int64 encoding using big-endian for proper key ordering

**Test Coverage:**
- 12 test functions + 2 benchmarks
- Tests for: creation, reopening, save/load, sequential blocks, duplicate rejection, out-of-order blocks, load by hash, HasBlock, Height/Base tracking, BlockCount, Close behavior, concurrent access (10 goroutines × 20 blocks), large blocks (1MB), persistence across restarts
- All tests pass with race detection
- Benchmarks for SaveBlock and LoadBlock operations

**Design Decisions:**
- Height stored as big-endian uint64 for proper lexicographic ordering in LevelDB
- Block value includes height prefix for reverse lookup by hash
- Base tracks earliest block (for pruned stores)
- Height tracks latest block
- Empty store returns 0 for both Height() and Base()
- Operations after Close() return errors
- Using `//nolint:gosec` for safe int64↔uint64 conversions (heights are always non-negative)

---

## [Phase 5] State Store

**Status:** Completed

**Files Created:**
- `statestore/store.go` - StateStore interface and Proof type definitions
- `statestore/iavl.go` - IAVL-backed implementation using cosmos/iavl
- `statestore/iavl_test.go` - Comprehensive test suite

**Files Modified:**
- `statestore/doc.go` - Removed (replaced by store.go)

**Functionality Implemented:**

StateStore Interface (`store.go`):
- `Get(key)` - Retrieve value for key
- `Has(key)` - Check if key exists
- `Set(key, value)` - Store key-value pair in working tree
- `Delete(key)` - Remove key from working tree
- `Commit()` - Save working tree as new version (returns hash, version)
- `RootHash()` - Get current tree root hash
- `Version()` - Get latest committed version
- `LoadVersion(version)` - Load specific historical version
- `GetProof(key)` - Generate merkle proof for key
- `Close()` - Release resources

Proof Type (`store.go`):
- `Key`, `Value`, `Exists` - Key and value info
- `RootHash`, `Version` - Tree state
- `ProofBytes` - Serialized ICS23 commitment proof
- `Verify()` - Placeholder for proof verification

IAVL Implementation (`iavl.go`):
- `NewIAVLStore(path, cacheSize)` - Create persistent store with LevelDB backend
- `NewMemoryIAVLStore(cacheSize)` - Create in-memory store for testing
- Uses `github.com/cosmos/iavl` v1.3.5 merkle tree
- Uses `github.com/cosmos/iavl/db` for database abstraction
- Thread-safe with `sync.RWMutex`
- Additional methods: `VersionExists()`, `GetVersioned()`
- Automatic loading of latest version on startup

**Test Coverage:**
- 15 test functions with 40+ test cases
- Tests for: creation, reopening, get/set, has/delete, commit, root hash, versioning (load, exists, get versioned), proofs (existing, non-existing), concurrent access (10 goroutines), persistence, large values (1MB), many keys (1000)
- All tests pass with race detection

**Design Decisions:**
- Uses cosmos/iavl v1.3.5 for merkle tree implementation
- Uses iavl/db.NewGoLevelDB for persistent storage
- Uses iavl/db.NewMemDB for in-memory testing
- Proof includes serialized ICS23 commitment proof bytes
- Working tree changes visible immediately via RootHash()
- Changes only persisted via Commit()
- LoadVersion allows time-travel to historical states

---
