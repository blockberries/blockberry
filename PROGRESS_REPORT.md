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

## [Phase 6] Mempool

**Status:** Completed (Later refactored - see Mempool Refactor below)

**Files Created:**
- `mempool/mempool.go` - Mempool interface and factory function
- `mempool/simple_mempool.go` - SimpleMempool implementation (hash-based storage)
- `mempool/mempool_test.go` - SimpleMempool test suite

**Files Modified:**
- `mempool/doc.go` - Removed (replaced by mempool.go)

**Functionality Implemented:**

Mempool Interface (`mempool.go`):
- `AddTx(tx)` - Add transaction to mempool
- `RemoveTxs(hashes)` - Remove transactions by hash
- `ReapTxs(maxBytes)` - Get transactions in insertion order up to size limit
- `HasTx(hash)` - Check if transaction exists
- `GetTx(hash)` - Retrieve transaction by hash
- `Size()` - Number of transactions
- `SizeBytes()` - Total bytes of all transactions
- `Flush()` - Remove all transactions
- `NewMempool(cfg)` - Factory function using config

SimpleMempool (`simple_mempool.go`):
- `NewSimpleMempool(maxTxs, maxBytes)` - Create with limits
- Stores tx hash → tx data mapping
- Maintains insertion order for ReapTxs
- Enforces maxTxs and maxBytes limits
- Thread-safe with `sync.RWMutex`
- `TxHashes()` - Get all transaction hashes

**Test Coverage:**
- 12 test functions + 1 benchmark
- SimpleMempool tests: add/get, duplicates, nil tx, max limits, has/get, remove, reap ordering, flush, tx hashes, concurrent access (10 goroutines × 50 txs), insertion order preservation
- All tests pass with race detection
- Benchmark for AddTx operation

**Design Decisions:**
- Simple hash-based storage (no merkle tree - peers don't need membership proofs)
- ReapTxs returns copies, originals stay in mempool
- Insertion order preserved even after removals
- Uses types.HashTx for transaction hashing (SHA-256)

---

## [Phase 7] P2P Layer

**Status:** Completed

**Files Created:**
- `p2p/peer_state.go` - PeerState for tracking individual peer information
- `p2p/peer_manager.go` - PeerManager for managing all connected peers
- `p2p/scoring.go` - PeerScorer for penalty-based peer reputation
- `p2p/network.go` - Network wrapper for glueberry node
- `p2p/peer_state_test.go` - PeerState test suite
- `p2p/peer_manager_test.go` - PeerManager test suite
- `p2p/scoring_test.go` - PeerScorer test suite
- `p2p/network_test.go` - Network test suite

**Files Modified:**
- `p2p/doc.go` - Removed (replaced by implementation files)

**Functionality Implemented:**

PeerState (`peer_state.go`):
- Tracks peer identity (PeerID, PublicKey, IsOutbound, IsSeed)
- Tracks connection timing (ConnectedAt, LastSeen, Latency)
- Tracks exchanged transactions (TxsSent, TxsReceived maps)
- Tracks exchanged blocks (BlocksSent, BlocksReceived maps)
- Penalty point management (AddPenalty, DecayPenalty, GetPenaltyPoints)
- `ShouldSendTx(hash)` / `ShouldSendBlock(height)` - Avoid duplicate sends
- `HasTx(hash)` / `HasBlock(height)` - Check if peer has item
- Count methods (TxsSentCount, TxsReceivedCount, BlocksSentCount, BlocksReceivedCount)
- Thread-safe with `sync.RWMutex`

PeerManager (`peer_manager.go`):
- `AddPeer(peerID, isOutbound)` - Add peer and create state
- `RemovePeer(peerID)` - Remove peer
- `GetPeer(peerID)` / `HasPeer(peerID)` - Lookup peer
- `AllPeers()` / `AllPeerIDs()` - List all peers
- `PeerCount()` - Number of connected peers
- `MarkTxSent/Received(peerID, txHash)` - Track tx exchanges
- `MarkBlockSent/Received(peerID, height)` - Track block exchanges
- `ShouldSendTx/Block(peerID, item)` - Check before sending
- `PeersToSendTx(hash)` / `PeersToSendBlock(height)` - Get peers needing item
- `SetPublicKey(peerID, pubKey)` - Set peer's public key after handshake
- Thread-safe with `sync.RWMutex`

PeerScorer (`scoring.go`):
- Penalty point thresholds: Warn (10), Ban (100)
- Penalty values: InvalidMessage (5), InvalidBlock (20), InvalidTx (5), Timeout (2), DuplicateMessage (1), ProtocolViolation (50), ChainMismatch (100), VersionMismatch (100)
- `AddPenalty(peerID, points, reason, message)` - Add penalty points
- `GetPenaltyPoints(peerID)` - Get current points
- `ShouldBan(peerID)` - Check if threshold exceeded
- `GetBanDuration(peerID)` - Exponential backoff (1h, 2h, 4h... up to 24h max)
- `RecordBan(peerID)` / `ResetBanCount(peerID)` - Track ban history
- `DecayPenalties(points)` - Apply decay to all peers
- `StartDecayLoop(stop)` - Background goroutine for hourly decay
- Event logging with `RecentEvents(count)` and `PeerEventsCount(peerID)`
- Capped at 1000 events in memory

Network (`network.go`):
- Stream name constants: handshake, pex, transactions, blocksync, blocks, consensus, housekeeping
- `AllStreams()` - Returns all encrypted stream names
- `NewNetwork(node)` - Wraps glueberry.Node
- `Start()` / `Stop()` - Lifecycle management
- `PeerID()` / `PublicKey()` - Local identity
- `Connect(peerID)` / `Disconnect(peerID)` - Connection management
- `Send(peerID, streamName, data)` - Send to specific peer
- `Broadcast(streamName, data)` - Send to all peers
- `BroadcastTx(txHash, data)` - Broadcast tx with dedup tracking
- `BroadcastBlock(height, data)` - Broadcast block with dedup tracking
- `PrepareStreams()` / `FinalizeHandshake()` / `CompleteHandshake()` - Two-phase handshake
- `BlacklistPeer(peerID)` - Ban and disconnect peer
- `AddPenalty(peerID, points, reason, message)` - Add penalty, auto-ban if threshold
- `OnPeerConnected/Disconnected()` - Event handlers
- `OnTxReceived(peerID, txHash)` / `OnBlockReceived(peerID, height)` - Track incoming
- `Messages()` / `Events()` - Access incoming channels
- `ConnectionState(peerID)` / `PeerCount()` - Status methods

**Test Coverage:**
- 24 test functions with 70+ test cases
- PeerState tests: creation, tx tracking, block tracking, penalties, timing, public key, seed flag
- PeerManager tests: add/remove, lookup, AllPeers, tx tracking, block tracking, PeersToSend, concurrent access (10 goroutines × 50 ops), SetPublicKey
- PeerScorer tests: AddPenalty, accumulation, ShouldBan, ban duration escalation, decay, event logging, decay loop
- Network tests: AllStreams, stream constants
- All tests pass with race detection

**Design Decisions:**
- Tx/block tracking uses string keys from hash bytes for map efficiency
- Penalty decay is 1 point per hour via background goroutine
- Ban duration doubles each time (exponential backoff) up to 24h max
- Network wraps glueberry.Node rather than embedding for better encapsulation
- Stream names are constants to prevent typos
- PeerScorer logs events for debugging/monitoring (capped at 1000)
- Shift amount capped at 5 in GetBanDuration to prevent integer overflow

---

## [Phase 8] Handshake

**Status:** Completed

**Files Created:**
- `handlers/handshake.go` - HandshakeHandler for connection establishment protocol
- `handlers/handshake_test.go` - Comprehensive test suite

**Files Modified:**
- `handlers/doc.go` - Retained as package documentation

**Functionality Implemented:**

HandshakeHandler (`handshake.go`):
- Manages handshake protocol for peer connections
- State machine: StateInit → StateHelloSent → StateHelloReceived → StateResponseSent → StateResponseReceived → StateFinalizeSent → StateComplete
- Per-peer handshake state tracking with PeerHandshakeState struct
- Message type ID constants matching schema (128, 129, 130)

HelloRequest Handling:
- `OnPeerConnected(peerID, isOutbound)` - Initiates handshake by sending HelloRequest
- `sendHelloRequest(peerID)` - Sends HelloRequest with node_id, version, chain_id, timestamp, latest_height
- `handleHelloRequest(peerID, data)` - Validates chain_id and version, blacklists on mismatch, sends HelloResponse

HelloResponse Handling:
- `sendHelloResponse(peerID, accepted)` - Sends HelloResponse with acceptance and public key
- `handleHelloResponse(peerID, data)` - Checks acceptance, calls PrepareStreams with peer's public key, sends HelloFinalize

HelloFinalize Handling:
- `sendHelloFinalize(peerID, success)` - Sends HelloFinalize with success flag
- `handleHelloFinalize(peerID, data)` - Calls FinalizeHandshake, stores peer's public key in PeerManager, transitions to StateComplete

Message Encoding:
- `HandleMessage(peerID, data)` - Dispatches incoming messages by type ID
- `encodeHandshakeMessage(typeID, msg)` - Encodes message with type ID prefix
- Uses cramberry for serialization

Utility Methods:
- `OnPeerDisconnected(peerID)` - Cleans up handshake state
- `GetPeerState(peerID)` - Returns handshake state for peer
- `IsHandshakeComplete(peerID)` - Checks if handshake is complete
- `PeerCount()` - Returns number of peers with active handshakes

**Test Coverage:**
- 10 test functions with 20+ test cases
- TestHandshakeState: initial state, state after init, is handshake complete, peer count, cleanup on disconnect
- TestEncodeDecodeHelloRequest: encode/decode round-trip
- TestEncodeDecodeHelloResponse: encode/decode round-trip
- TestEncodeDecodeHelloFinalize: encode/decode round-trip
- TestHandleHelloRequestValidation: chain ID mismatch, version mismatch, missing required field
- TestHandleHelloResponseValidation: no handshake state, rejection response
- TestHandleHelloFinalizeValidation: no handshake state, failure response, early finalize
- TestHandleMessageDispatch: empty message, unknown type ID, valid HelloRequest dispatch
- TestHandshakeConstants: verifies type ID constants match schema
- All tests pass with race detection

**Design Decisions:**
- State machine tracks both local and remote handshake progress
- Nil network/peerManager checks for testability without full P2P stack
- Type ID prefix written before message data for polymorphic decoding
- Chain ID and version mismatches result in peer blacklisting (permanent ban)
- Handshake rejection (accepted=false) and failure (success=false) both disconnect peer
- Early HelloFinalize receipt (before we've sent ours) is gracefully handled

---

## [Phase 9] PEX (Peer Exchange)

**Status:** Completed

**Files Created:**
- `pex/address_book.go` - AddressBook with JSON persistence
- `pex/reactor.go` - PEX reactor for peer exchange protocol
- `pex/address_book_test.go` - AddressBook test suite
- `pex/reactor_test.go` - Reactor test suite

**Files Modified:**
- `p2p/peer_manager.go` - Added GetConnectedPeers, OutboundPeerCount, InboundPeerCount
- `p2p/network.go` - Added ConnectMultiaddr for connecting via multiaddr string

**Functionality Implemented:**

AddressBook (`address_book.go`):
- `NewAddressBook(path)` - Create address book with optional persistence
- `Load()` / `Save()` - JSON persistence with atomic writes
- `AddPeer(peerID, multiaddr, nodeID, latency)` - Add or update peer
- `RemovePeer(peerID)` - Remove peer
- `GetPeer(peerID)` / `HasPeer(peerID)` - Lookup peer
- `GetPeers()` / `Size()` - List all peers
- `MarkSeed(peerID)` - Mark peer as seed node
- `UpdateLastSeen(peerID)` / `UpdateLatency(peerID, latency)` - Update peer info
- `RecordAttempt(peerID)` / `ResetAttempts(peerID)` - Connection attempt tracking
- `GetPeersForExchange(lastSeenAfter, max)` - Get peers for PEX response
- `GetPeersToConnect(exclude, max)` - Get connection candidates with backoff
- `AddSeeds(addrs)` / `GetSeeds()` - Seed node management
- `Prune(maxAge)` - Remove stale peers (preserves seeds)

AddressEntry struct:
- Multiaddr, NodeID, LastSeen, Latency
- IsSeed, LastAttempt, AttemptCount

Reactor (`reactor.go`):
- `NewReactor(enabled, requestInterval, maxAddresses, addressBook, network, peerManager, maxInbound, maxOutbound)` - Create reactor
- `Start()` / `Stop()` - Lifecycle management
- `HandleMessage(peerID, data)` - Process incoming PEX messages
- `handleAddressRequest(peerID, data)` - Respond with peer addresses
- `handleAddressResponse(peerID, data)` - Add received peers to address book
- `sendAddressRequest(peerID)` - Send address request
- `sendAddressResponse(peerID, resp)` - Send address response
- `encodeMessage(typeID, msg)` - Encode message with type ID prefix
- `requestLoop()` - Periodic address requests to connected peers
- `connectionLoop()` - Periodic connection management
- `ensureOutboundConnections()` - Maintain outbound peer count
- `OnPeerConnected(peerID, multiaddr, isOutbound)` - Handle new connection
- `OnPeerDisconnected(peerID)` - Handle disconnection
- `GetAddressBook()` / `IsRunning()` - Accessor methods
- `RequestAddresses(peerID)` - Manual address request

PeerManager Additions:
- `GetConnectedPeers()` - Returns all connected peer IDs
- `OutboundPeerCount()` - Count of outbound peers
- `InboundPeerCount()` - Count of inbound peers

Network Additions:
- `ConnectMultiaddr(addrStr)` - Connect using multiaddr string

**Test Coverage:**
- 27 test functions with 50+ test cases
- AddressBook tests: creation, add/get/update/remove peer, has peer, size, get peers, mark seed, update last seen, update latency, record/reset attempts, peers for exchange (filtering, limiting, sorting), peers to connect (backoff, exclusion, sorting), seeds, prune, save/load persistence, empty path handling, concurrency (4 goroutines)
- Reactor tests: creation, start/stop, disabled start, empty message, invalid type ID, address request handling, address response handling, invalid node ID handling, peer connected/disconnected, encode message, type ID constants, requester exclusion, missing fields handling
- All tests pass with race detection

**Design Decisions:**
- AddressBook stores peers by peer.ID with full entry metadata
- JSON persistence with atomic writes (write to .tmp, then rename)
- Connection attempts tracked with exponential backoff (1s, 2s, 4s... up to 1h)
- Peers sorted by latency then last seen for connection selection
- PEX responses exclude the requesting peer
- Address response peers with invalid node IDs are silently skipped
- Reactor handles nil network/peerManager gracefully for testing
- File permissions set to 0600 for security

---

## Mempool Refactor

**Status:** Completed

**Reason:** The merkle tree implementation was unnecessary since the mempool does not need to prove to other peers that it has or doesn't have a specific transaction. Simplified to a basic hash-based storage.

**Files Deleted:**
- `mempool/merkle.go` - Merkle tree implementation
- `mempool/merkle_mempool.go` - MerkleMempool implementation
- `mempool/merkle_test.go` - Merkle tree tests

**Files Created:**
- `mempool/simple_mempool.go` - SimpleMempool with hash-based storage

**Files Modified:**
- `mempool/mempool.go` - Removed RootHash() from interface, updated factory function
- `mempool/mempool_test.go` - Updated tests to use SimpleMempool, removed RootHash tests
- `CLAUDE.md` - Updated mempool description to reflect hash-based storage

**Changes:**
- Removed `RootHash()` method from Mempool interface
- Replaced MerkleMempool with SimpleMempool
- Removed in-memory merkle tree implementation
- Simplified storage to plain hash map with insertion order tracking
- All existing functionality preserved (AddTx, RemoveTxs, ReapTxs, etc.)

**Test Coverage:**
- 12 test functions + 1 benchmark
- All tests pass with race detection

---

## [Phase 10] Transactions

**Status:** Completed

**Files Created:**
- `handlers/transactions.go` - TransactionsReactor for transaction gossiping
- `handlers/transactions_test.go` - Transaction reactor test suite

**Files Modified:**
- `mempool/mempool.go` - Added TxHashes() to Mempool interface

**Functionality Implemented:**

TransactionsReactor (`handlers/transactions.go`):
- `NewTransactionsReactor(mempool, network, peerManager, requestInterval, batchSize)` - Create reactor
- `Start()` / `Stop()` - Lifecycle management with background gossip loop
- `HandleMessage(peerID, data)` - Process incoming transaction messages
- `SendTransactionsRequest(peerID)` - Request tx hashes from peer
- `BroadcastTx(tx)` - Broadcast new transaction to all peers
- `OnPeerDisconnected(peerID)` - Cleanup pending requests

Message Handlers:
- `TransactionsRequest` (type 133) - Responds with tx hashes from mempool
- `TransactionsResponse` (type 134) - Checks for missing txs, requests data
- `TransactionDataRequest` (type 135) - Responds with full tx data
- `TransactionDataResponse` (type 136) - Validates hash, adds to mempool

Per-Peer Tracking:
- Tracks pending data requests per peer
- Uses PeerManager.MarkTxSent/MarkTxReceived to avoid duplicates
- Uses PeerManager.ShouldSendTx to filter txs peer already has
- Hash verification with penalty on mismatch

**Test Coverage:**
- 17 test functions covering:
  - Reactor creation and lifecycle (start/stop)
  - Message encoding/decoding for all 4 message types
  - Request/response handlers
  - Hash mismatch detection
  - Pending request tracking
  - Peer disconnect cleanup
  - Duplicate transaction handling

**Design Decisions:**
- Background gossip loop with configurable interval
- Pending request map prevents duplicate data requests
- Hash verification catches malicious/corrupted data
- Graceful handling of nil dependencies for testing
- Reactor handles nil network/peerManager gracefully

---

## [Phase 11] Block Sync

**Status:** Completed

**Files Created:**
- `sync/reactor.go` - SyncReactor for block synchronization
- `sync/reactor_test.go` - Sync reactor test suite
- `blockstore/memory.go` - In-memory block store for testing

**Files Modified:**
- `types/errors.go` - Added ErrBlockExists alias and ErrInvalidBlock

**Functionality Implemented:**

SyncReactor (`sync/reactor.go`):
- `NewSyncReactor(blockStore, network, peerManager, syncInterval, batchSize)` - Create reactor
- `Start()` / `Stop()` - Lifecycle management with background sync loop
- `HandleMessage(peerID, data)` - Process incoming block sync messages
- `SendBlocksRequest(peerID, since)` - Request blocks from peer
- `UpdatePeerHeight(peerID, height)` - Track peer block heights
- `SetValidator(fn)` - Set block validation callback
- `SetOnSyncComplete(fn)` - Set sync completion callback
- `OnPeerConnected(peerID, height)` / `OnPeerDisconnected(peerID)` - Peer lifecycle

Sync State Machine:
- `StateSynced` - Node is caught up with peers
- `StateSyncing` - Node is catching up
- Automatic state transitions based on peer heights

Message Handlers:
- `BlocksRequest` (type 137) - Responds with blocks from BlockStore
- `BlocksResponse` (type 138) - Validates, stores blocks, continues syncing

Block Validation:
- Hash verification before storage
- Optional validator callback for application-level validation
- Penalty on hash mismatch via PeerManager

MemoryBlockStore (`blockstore/memory.go`):
- In-memory implementation for testing
- Implements full BlockStore interface
- Thread-safe with RWMutex

**Test Coverage:**
- 18 test functions covering:
  - Reactor creation and lifecycle
  - Message encoding/decoding
  - Block request/response handlers
  - Hash mismatch detection
  - Custom validator rejection
  - Peer height tracking
  - State transitions
  - Sync complete callback
  - Duplicate block handling

**Design Decisions:**
- Background sync loop with configurable interval
- Tracks peer heights to select sync targets
- Pending request tracking prevents duplicate requests
- Hash verification catches malicious/corrupted blocks
- Optional validator callback for application-level validation
- Memory block store for testing without disk I/O

---

## [Phase 12] Block Propagation

**Status:** Completed

**Files Created:**
- `handlers/blocks.go` - BlockReactor for real-time block propagation
- `handlers/blocks_test.go` - Block reactor test suite

**Functionality Implemented:**

BlockReactor (`handlers/blocks.go`):
- `NewBlockReactor(blockStore, network, peerManager)` - Create reactor
- `HandleMessage(peerID, data)` - Process incoming block messages
- `BroadcastBlock(height, hash, data)` - Broadcast new block to all peers
- `SetValidator(fn)` - Set block validation callback
- `SetOnBlockReceived(fn)` - Set new block callback
- `OnPeerDisconnected(peerID)` - Peer lifecycle (no-op)

Message Handler:
- `BlockData` (type 139) - Receives, validates, stores, and relays blocks

Block Processing:
- Hash verification before storage
- Optional validator callback for application-level validation
- Penalty on hash mismatch via PeerManager
- Automatic relay to peers who don't have the block
- onBlockReceived callback for application notification

Block Broadcasting:
- Uses PeerManager.PeersToSendBlock to find peers
- Tracks sent blocks via PeerManager.MarkBlockSent
- Excludes peer who sent the block during relay

**Test Coverage:**
- 12 test functions covering:
  - Reactor creation
  - Message encoding/decoding
  - Block data handling
  - Hash mismatch detection
  - Custom validator rejection
  - Duplicate block handling
  - Block received callback
  - Missing field rejection
  - Nil dependency handling

**Design Decisions:**
- Stateless reactor (no per-peer state)
- Hash verification before storage
- Relay to all peers who don't have the block
- Separate from sync reactor for real-time propagation

---

## [Phase 13] Consensus Integration

**Status:** Completed

**Files Created:**
- `handlers/consensus.go` - ConsensusReactor for consensus message pass-through
- `handlers/consensus_test.go` - Consensus reactor test suite

**Functionality Implemented:**

ConsensusHandler Interface:
```go
type ConsensusHandler interface {
    HandleConsensusMessage(peerID peer.ID, data []byte) error
}
```

ConsensusReactor (`handlers/consensus.go`):
- `NewConsensusReactor(network, peerManager)` - Create reactor
- `SetHandler(handler)` / `GetHandler()` - Register application handler
- `HandleMessage(peerID, data)` - Route messages to handler
- `SendConsensusMessage(peerID, data)` - Send to specific peer
- `BroadcastConsensusMessage(data)` - Send to all peers
- `OnPeerDisconnected(peerID)` - Peer lifecycle (no-op)

**Test Coverage:**
- 10 test functions covering:
  - Reactor creation
  - Handler registration
  - Empty message rejection
  - Message routing to handler
  - Multiple message handling
  - Handler error propagation
  - Nil dependency handling

**Design Decisions:**
- Pass-through design - blockberry doesn't interpret consensus messages
- Application registers handler to receive messages
- SendConsensusMessage for unicast, BroadcastConsensusMessage for multicast
- Silent ignore when no handler registered
- Stateless reactor (no per-peer state)

---
