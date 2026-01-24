# Blockberry Implementation Plan

This document outlines the comprehensive implementation plan for blockberry. Tasks are organized into phases with dependencies clearly marked.

## Phase Overview

| Phase | Name | Description | Dependencies |
|-------|------|-------------|--------------|
| 1 | Project Setup | Build system, deps, base structure | None |
| 2 | Configuration | Config loading and validation | Phase 1 |
| 3 | Types & Errors | Common types, error definitions | Phase 1 |
| 4 | Block Store | Block persistence layer | Phase 2, 3 |
| 5 | State Store | IAVL-based merkleized KV store | Phase 2, 3 |
| 6 | Mempool | Hash-based transaction pool | Phase 3 |
| 7 | P2P Layer | Glueberry integration, peer management | Phase 2, 3 |
| 8 | Handshake | Connection establishment protocol | Phase 7 |
| 9 | PEX | Peer exchange and discovery | Phase 7, 8 |
| 10 | Transactions | Transaction gossiping | Phase 6, 7 |
| 11 | Block Sync | Historical block synchronization | Phase 4, 7 |
| 12 | Block Propagation | Real-time block broadcast | Phase 4, 7 |
| 13 | Consensus Integration | Consensus stream pass-through | Phase 7 |
| 14 | Housekeeping | Latency probes, firewall detection | Phase 7 |
| 15 | Node Coordinator | Main node orchestration | All above |
| 16 | Application Interface | ABCI-like app integration | Phase 15 |
| 17 | Integration Testing | End-to-end tests | All above |
| 18 | Documentation | API docs, examples | All above |

---

## Phase 1: Project Setup

**Status: COMPLETED**

### 1.1 Initialize Go Module
- [x] Create `go.mod` with module `github.com/blockberries/blockberry`
- [x] Add dependencies:
  - `github.com/blockberries/glueberry`
  - `github.com/blockberries/cramberry`
  - `github.com/cosmos/iavl`
  - `github.com/syndtr/goleveldb` (or `github.com/dgraph-io/badger`)
  - `github.com/BurntSushi/toml`
  - `github.com/stretchr/testify`
- [x] Run `go mod tidy`

### 1.2 Create Makefile
- [x] `build` target: compile all packages
- [x] `test` target: run tests with race detection (`go test -race ./...`)
- [x] `lint` target: run golangci-lint
- [x] `generate` target: regenerate cramberry code
- [x] `clean` target: remove build artifacts

### 1.3 Create Directory Structure
- [x] Create package directories as per ARCHITECTURE.md
- [x] Add `.gitignore` for Go projects
- [x] Create empty `PROGRESS_REPORT.md`

### 1.4 Setup Linting
- [x] Create `.golangci.yml` with appropriate rules
- [x] Ensure lint passes on empty project

**Implementation Notes:**
- Makefile includes additional targets: test-race, fmt, vet, coverage, tidy, check
- deps.go uses build tag to avoid being compiled into binaries while tracking dependencies
- golangci-lint configured to exclude generated schema/ directory and test files from certain linters
- Local replace directives used for glueberry and cramberry during development
- Package directories created with doc.go stubs for each package

---

## Phase 2: Configuration

**Status: COMPLETED**

### 2.1 Define Config Structures
- [x] Create `config/config.go`
- [x] Define `Config` struct with all nested configs:
  ```go
  type Config struct {
      Node        NodeConfig
      Network     NetworkConfig
      PEX         PEXConfig
      Mempool     MempoolConfig
      BlockStore  BlockStoreConfig
      StateStore  StateStoreConfig
      Housekeeping HousekeepingConfig
  }
  ```
- [x] Define `NodeConfig`: chain_id, protocol_version, private_key_path
- [x] Define `NetworkConfig`: listen_addrs, max_peers, timeouts, seeds
- [x] Define `PEXConfig`: enabled, request_interval, max_addresses
- [x] Define `MempoolConfig`: max_txs, max_bytes, cache_size
- [x] Define `BlockStoreConfig`: backend, path
- [x] Define `StateStoreConfig`: path, cache_size
- [x] Define `HousekeepingConfig`: latency_probe_interval

### 2.2 Config Loading
- [x] Implement `LoadConfig(path string) (*Config, error)`
- [x] Support TOML format
- [x] Apply sensible defaults for optional fields
- [x] Validate required fields

### 2.3 Config Validation
- [x] Validate chain_id is non-empty
- [x] Validate listen_addrs are valid multiaddrs
- [x] Validate paths are writable
- [x] Validate numeric ranges (max_peers > 0, etc.)

### 2.4 Default Config Generation
- [x] Implement `DefaultConfig() *Config`
- [x] Implement `WriteConfigFile(path string, cfg *Config) error`

### 2.5 Config Tests
- [x] Test loading valid config
- [x] Test loading config with missing required fields
- [x] Test default value application
- [x] Test validation errors

**Implementation Notes:**
- Custom `Duration` type wraps time.Duration for proper TOML serialization (e.g., "30s")
- Validation only applies to enabled features (e.g., PEX fields only validated if enabled)
- Zero cache sizes and zero max peers are valid (disables caching / node isolation)
- Sentinel errors allow callers to check specific validation failures with errors.Is()
- `EnsureDataDirs()` creates required directories
- 18 test functions with 40+ test cases covering all scenarios

---

## Phase 3: Types & Errors

**Status: COMPLETED**

### 3.1 Common Types
- [x] Create `types/types.go`
- [x] Define type aliases for clarity:
  ```go
  type Height int64
  type Hash []byte
  type Tx []byte
  type Block []byte
  ```
- [x] Define `TxHash(tx Tx) Hash` helper function

### 3.2 Error Definitions
- [x] Create `types/errors.go`
- [x] Define sentinel errors:
  ```go
  var (
      ErrPeerNotFound      = errors.New("peer not found")
      ErrBlockNotFound     = errors.New("block not found")
      ErrTxNotFound        = errors.New("transaction not found")
      ErrMempoolFull       = errors.New("mempool is full")
      ErrInvalidTx         = errors.New("invalid transaction")
      ErrChainIDMismatch   = errors.New("chain ID mismatch")
      ErrVersionMismatch   = errors.New("protocol version mismatch")
      ErrInvalidMessage    = errors.New("invalid message format")
      ErrPeerBlacklisted   = errors.New("peer is blacklisted")
      ErrConnectionClosed  = errors.New("connection closed")
  )
  ```

### 3.3 Hash Functions
- [x] Create `types/hash.go`
- [x] Implement `HashTx(tx Tx) Hash` using SHA256
- [x] Implement `HashBlock(block Block) Hash`
- [x] Add tests for hash functions

**Implementation Notes:**
- Types include helper methods: Height.String(), Hash.String()/Bytes()/IsEmpty()/Equal(), Tx.String()/Bytes()/Size()
- PeerID type added for peer identifier with String() and IsEmpty() methods
- HashFromHex() parses hash from hexadecimal string
- 32 sentinel errors covering: peer, block, transaction, mempool, connection, message, state, sync, and node errors
- HashBytes() and HashConcat() added for arbitrary data and merkle tree operations
- 14 test functions with 60+ test cases, including benchmarks for hash operations
- Hash functions return nil for nil input (not empty hash)
- String() methods truncate long values for readability

---

## Phase 4: Block Store

**Status: COMPLETED**

### 4.1 Define Interface
- [x] Create `blockstore/store.go`
- [x] Define `BlockStore` interface:
  ```go
  type BlockStore interface {
      SaveBlock(height int64, hash []byte, data []byte) error
      LoadBlock(height int64) (hash []byte, data []byte, error)
      LoadBlockByHash(hash []byte) (height int64, data []byte, error)
      HasBlock(height int64) bool
      Height() int64
      Base() int64
      Close() error
  }
  ```

### 4.2 LevelDB Implementation
- [x] Create `blockstore/leveldb.go`
- [x] Implement key encoding:
  - Height → Hash: `"H:" + binary.BigEndian(height)`
  - Hash → Data: `"B:" + hash`
  - Metadata: `"M:height"`, `"M:base"`
- [x] Implement `NewLevelDBBlockStore(path string) (*LevelDBBlockStore, error)`
- [x] Implement all interface methods
- [x] Handle atomic batch writes for SaveBlock

### 4.3 Block Store Tests
- [x] Test SaveBlock and LoadBlock
- [x] Test LoadBlockByHash
- [x] Test HasBlock
- [x] Test Height/Base tracking
- [x] Test persistence across restarts
- [x] Test concurrent access

### 4.4 (Optional) BadgerDB Implementation
- [ ] Create `blockstore/badger.go` (if needed)
- [ ] Implement same interface

**Implementation Notes:**
- BlockInfo struct for block metadata
- Height stored as big-endian uint64 for proper lexicographic ordering in LevelDB
- Block value includes height prefix for reverse lookup by hash
- Base tracks earliest block (for pruned stores), Height tracks latest block
- Empty store returns 0 for both Height() and Base()
- Sync writes enabled for durability (`Sync: true`)
- Thread-safe with `sync.RWMutex`
- 12 test functions + 2 benchmarks covering: creation, reopening, sequential blocks, duplicate rejection, out-of-order blocks, large blocks (1MB), concurrent access (10 goroutines × 20 blocks)

---

## Phase 5: State Store

**Status: COMPLETED**

### 5.1 Define Interface
- [x] Create `statestore/store.go`
- [x] Define `StateStore` interface:
  ```go
  type StateStore interface {
      Get(key []byte) ([]byte, error)
      Has(key []byte) (bool, error)
      Set(key []byte, value []byte) error
      Delete(key []byte) error
      Commit() (hash []byte, version int64, error)
      RootHash() []byte
      Version() int64
      LoadVersion(version int64) error
      GetProof(key []byte) (*Proof, error)
      Close() error
  }
  ```

### 5.2 IAVL Implementation
- [x] Create `statestore/iavl.go`
- [x] Import `github.com/cosmos/iavl`
- [x] Implement `NewIAVLStore(path string, cacheSize int) (*IAVLStore, error)`
- [x] Wrap IAVL MutableTree
- [x] Implement all interface methods
- [x] Handle version loading on startup

### 5.3 Proof Type
- [x] Define `Proof` struct wrapping IAVL proof
- [x] Implement proof verification method

### 5.4 State Store Tests
- [x] Test Get/Set/Delete operations
- [x] Test Commit and RootHash
- [x] Test version loading
- [x] Test proof generation and verification
- [x] Test persistence across restarts
- [x] Test concurrent access

**Implementation Notes:**
- Uses cosmos/iavl v1.3.5 for merkle tree implementation
- Uses iavl/db.NewGoLevelDB for persistent storage, iavl/db.NewMemDB for in-memory testing
- NewMemoryIAVLStore() for in-memory store (useful for testing)
- Proof struct includes: Key, Value, Exists, RootHash, Version, ProofBytes (serialized ICS23 commitment proof)
- Additional methods: VersionExists(), GetVersioned() for time-travel queries
- Working tree changes visible immediately via RootHash(), only persisted via Commit()
- Thread-safe with `sync.RWMutex`
- 15 test functions with 40+ test cases covering versioning, proofs, concurrent access, large values (1MB), many keys (1000)

---

## Phase 6: Mempool

**Status: COMPLETED**

### 6.1 Define Interface
- [x] Create `mempool/mempool.go`
- [x] Define `Mempool` interface:
  ```go
  type Mempool interface {
      AddTx(tx []byte) error
      RemoveTxs(hashes [][]byte)
      ReapTxs(maxBytes int64) [][]byte
      HasTx(hash []byte) bool
      GetTx(hash []byte) ([]byte, error)
      Size() int
      SizeBytes() int64
      Flush()
  }
  ```

### 6.2 Simple Mempool Implementation
- [x] Create `mempool/simple_mempool.go`
- [x] Implement `SimpleMempool` struct:
  ```go
  type SimpleMempool struct {
      txs       map[string][]byte    // hash → tx data
      order     [][]byte             // insertion order (tx hashes)
      maxTxs    int
      maxBytes  int64
      sizeBytes int64
      mu        sync.RWMutex
  }
  ```
- [x] Implement all interface methods
- [x] Enforce size limits in AddTx
- [x] Maintain insertion order for ReapTxs

### 6.3 Mempool Configuration
- [x] Support configuration from `MempoolConfig`
- [x] Factory function: `NewMempool(cfg MempoolConfig) Mempool`

### 6.4 Mempool Tests
- [x] Test AddTx and GetTx
- [x] Test RemoveTxs
- [x] Test ReapTxs ordering and size limits
- [x] Test HasTx
- [x] Test size limits enforcement
- [x] Test Flush
- [x] Test concurrent access
- [x] Benchmark AddTx operation

**Implementation Notes:**
- Originally implemented with merkle tree (MerkleMempool), later refactored to simple hash-based storage
- Merkle tree was unnecessary since mempool doesn't need to prove tx membership to peers
- TxHashes() method added for transaction hash listing (used by TransactionsReactor)
- ReapTxs returns copies, originals stay in mempool
- Insertion order preserved even after removals
- Uses types.HashTx for transaction hashing (SHA-256)
- 12 test functions + 1 benchmark covering: duplicates, nil tx, max limits, concurrent access (10 goroutines × 50 txs)

---

## Phase 7: P2P Layer

**Status: COMPLETED**

### 7.1 Peer State
- [x] Create `p2p/peer_state.go`
- [x] Define `PeerState` struct:
  ```go
  type PeerState struct {
      PeerID         peer.ID
      PublicKey      []byte
      TxsSent        map[string]bool
      TxsReceived    map[string]bool
      BlocksSent     map[int64]bool
      BlocksReceived map[int64]bool
      Score          int64
      PenaltyPoints  int64
      LastSeen       time.Time
      Latency        time.Duration
      ConnectedAt    time.Time
      IsSeed         bool
      IsOutbound     bool
  }
  ```
- [x] Implement methods for tracking exchanged items
- [x] Implement penalty point accumulation and decay

### 7.2 Peer Manager
- [x] Create `p2p/peer_manager.go`
- [x] Implement `PeerManager` struct
- [x] Track all connected peers
- [x] Implement `AddPeer`, `RemovePeer`, `GetPeer`
- [x] Implement `GetPeerState(peerID) *PeerState`
- [x] Implement `MarkTxSent`, `MarkTxReceived`
- [x] Implement `MarkBlockSent`, `MarkBlockReceived`
- [x] Implement `ShouldSendTx(peerID, txHash) bool`
- [x] Implement `ShouldSendBlock(peerID, height) bool`

### 7.3 Peer Scoring
- [x] Create `p2p/scoring.go`
- [x] Define penalty point thresholds
- [x] Implement `AddPenalty(peerID, points int, reason string)`
- [x] Implement `GetBanDuration(peerID) time.Duration`
- [x] Implement decay goroutine (1 point/hour)

### 7.4 Glueberry Wrapper
- [x] Create `p2p/network.go`
- [x] Wrap glueberry.Node with blockberry-specific logic
- [x] Define stream names as constants:
  ```go
  const (
      StreamHandshake    = "handshake"
      StreamPEX          = "pex"
      StreamTransactions = "transactions"
      StreamBlockSync    = "blocksync"
      StreamBlocks       = "blocks"
      StreamConsensus    = "consensus"
      StreamHousekeeping = "housekeeping"
  )
  ```
- [x] Implement `Send(peerID, stream, data)` wrapper
- [x] Implement `Broadcast(stream, data)` wrapper

### 7.5 P2P Tests
- [x] Test PeerState tracking
- [x] Test PeerManager operations
- [x] Test scoring and penalty system
- [x] Test concurrent access to peer state

**Implementation Notes:**
- PeerState includes: ShouldSendTx/ShouldSendBlock, HasTx/HasBlock, count methods for sent/received items
- PeerManager includes: PeersToSendTx/PeersToSendBlock for broadcast filtering, SetPublicKey for post-handshake
- PeerScorer: Penalty thresholds (Warn: 10, Ban: 100), penalty values for various violations (InvalidMessage: 5, InvalidBlock: 20, ProtocolViolation: 50, ChainMismatch: 100)
- Ban duration uses exponential backoff (1h, 2h, 4h... up to 24h max), shift capped at 5 to prevent overflow
- Event logging with RecentEvents(count), capped at 1000 events in memory
- Network includes: AllStreams(), BroadcastTx/BroadcastBlock with dedup tracking, PrepareStreams/FinalizeHandshake/CompleteHandshake, BlacklistPeer
- 24 test functions with 70+ test cases, all pass with race detection

---

## Phase 8: Handshake

**Status: COMPLETED**

### 8.1 Handshake Handler
- [x] Create `handlers/handshake.go`
- [x] Define `HandshakeHandler` struct
- [x] Implement state machine for handshake:
  - StateInit → HelloSent → HelloReceived → StreamsPrepared → Finalized

### 8.2 HelloRequest Handling
- [x] On `StateConnected` event, send `HelloRequest`
- [x] On receiving `HelloRequest`:
  - Validate chain_id matches → else blacklist
  - Validate version matches → else blacklist
  - Send `HelloResponse` with acceptance and public key

### 8.3 HelloResponse Handling
- [x] On receiving `HelloResponse`:
  - If not accepted, disconnect
  - Call `glueberry.PrepareStreams` with peer's public key
  - Send `HelloFinalize`

### 8.4 HelloFinalize Handling
- [x] On receiving `HelloFinalize`:
  - If both sides ready, call `glueberry.FinalizeHandshake`
  - Transition to `StateEstablished`
  - Initialize peer state in PeerManager

### 8.5 Handshake Tests
- [x] Test successful handshake between two nodes
- [x] Test chain_id mismatch rejection
- [x] Test version mismatch rejection
- [x] Test handshake timeout handling
- [x] Test concurrent handshakes with multiple peers

**Implementation Notes:**
- State machine: StateInit → StateHelloSent → StateHelloReceived → StateResponseSent → StateResponseReceived → StateFinalizeSent → StateComplete
- Message type IDs match schema: HelloRequest (128), HelloResponse (129), HelloFinalize (130)
- Per-peer handshake state tracking with PeerHandshakeState struct
- HelloRequest includes: node_id, version, chain_id, timestamp, latest_height
- Chain ID and version mismatches result in peer blacklisting (permanent ban)
- Handshake rejection (accepted=false) and failure (success=false) both disconnect peer
- Early HelloFinalize receipt (before we've sent ours) is gracefully handled
- Nil network/peerManager checks for testability without full P2P stack
- 10 test functions covering encode/decode, validation, dispatch, and constants

---

## Phase 9: PEX (Peer Exchange)

**Status: COMPLETED**

### 9.1 Address Book
- [x] Create `pex/address_book.go`
- [x] Define `AddressBook` struct:
  ```go
  type AddressBook struct {
      Peers  map[peer.ID]*AddressEntry
      Seeds  []SeedNode
      path   string
      mu     sync.RWMutex
  }
  ```
- [x] Implement JSON persistence
- [x] Implement `AddPeer`, `RemovePeer`, `GetPeers`
- [x] Implement `MarkSeed(peerID)` to mark discovered peer as seed
- [x] Implement `GetPeersForExchange(lastSeen time.Time, max int)`

### 9.2 Seed Node Handling
- [x] Load seeds from config on startup
- [x] Connect to seed nodes on bootstrap
- [x] Mark seed connections appropriately in PeerState

### 9.3 PEX Reactor
- [x] Create `pex/reactor.go`
- [x] Implement periodic `AddressRequest` sending (configurable interval)
- [x] Handle `AddressRequest`:
  - Filter peers by last_seen
  - Exclude blacklisted peers
  - Return up to max_addresses
- [x] Handle `AddressResponse`:
  - Add new peers to address book
  - Update existing peer info
  - Optionally initiate connections

### 9.4 Connection Management
- [x] Respect max_inbound_peers and max_outbound_peers limits
- [x] Implement peer selection strategy for new connections
- [x] Prefer peers with better scores

### 9.5 PEX Tests
- [x] Test AddressBook persistence
- [x] Test seed node loading
- [x] Test PEX message exchange
- [x] Test peer limit enforcement
- [x] Test address filtering by last_seen

**Implementation Notes:**
- AddressEntry struct includes: Multiaddr, NodeID, LastSeen, Latency, IsSeed, LastAttempt, AttemptCount
- JSON persistence with atomic writes (write to .tmp, then rename), file permissions 0600
- Connection attempt tracking with exponential backoff (1s, 2s, 4s... up to 1h max)
- GetPeersToConnect sorts by latency then last seen for connection selection
- Prune() removes stale peers but preserves seeds
- PEX message type IDs: AddressRequest (131), AddressResponse (132)
- Reactor includes requestLoop() and connectionLoop() background goroutines
- ensureOutboundConnections() maintains outbound peer count
- PEX responses exclude the requesting peer
- Address response peers with invalid node IDs are silently skipped
- Added to PeerManager: GetConnectedPeers(), OutboundPeerCount(), InboundPeerCount()
- Added to Network: ConnectMultiaddr() for connecting via multiaddr string
- 27 test functions with 50+ test cases covering persistence, filtering, backoff, concurrency

---

## Phase 10: Transactions

**Status: COMPLETED**

### 10.1 Transactions Reactor
- [x] Create `handlers/transactions.go`
- [x] Define `TransactionsReactor` struct

### 10.2 TransactionsRequest/Response
- [x] Implement periodic transaction gossip:
  - Send `TransactionsRequest` with batch_size
  - On receiving request, respond with tx hashes from mempool
- [x] Handle `TransactionsResponse`:
  - Check which txs we already have (mempool.HasTx)
  - Track received tx hashes

### 10.3 TransactionDataRequest/Response
- [x] Request full tx data for missing transactions
- [x] On receiving data request, send tx data from mempool
- [x] On receiving data response:
  - Validate and add to mempool
  - Mark as received from peer

### 10.4 Per-Peer Tracking
- [x] Use PeerManager to track txs sent/received
- [x] Avoid re-requesting txs from same peer
- [x] Avoid re-sending txs to peers who already have them

### 10.5 Transactions Tests
- [x] Test transaction gossip between two nodes
- [x] Test batch request/response
- [x] Test missing tx data requests
- [x] Test per-peer tracking prevents duplicates
- [x] Test with full mempool (reject new txs)

**Implementation Notes:**
- Message types: TransactionsRequest (133), TransactionsResponse (134), TransactionDataRequest (135), TransactionDataResponse (136)
- Added TxHashes() method to Mempool interface to support hash listing
- Background gossip loop with configurable interval (default 5 seconds)
- Pending request map prevents duplicate data requests
- Hash verification with penalty on mismatch (PenaltyInvalidTx)
- 17 test functions covering all functionality

---

## Phase 11: Block Sync

**Status: COMPLETED**

### 11.1 Sync State Machine
- [x] Create `sync/reactor.go`
- [x] Define sync states: `Synced`, `Syncing`
- [x] Track peer heights
- [x] Determine when to start/stop syncing

### 11.2 BlocksRequest/Response
- [x] Implement `BlocksRequest` sending:
  - Request batch of blocks starting from `since` height
- [x] Handle `BlocksRequest`:
  - Load blocks from BlockStore
  - Return up to batch_size blocks

### 11.3 Block Validation and Storage
- [x] On receiving `BlocksResponse`:
  - Validate blocks (app callback)
  - Store in BlockStore sequentially
  - Update sync progress

### 11.4 Sync Completion
- [x] Detect when caught up to peers
- [x] Transition from Syncing to Synced
- [x] Notify application of sync completion

### 11.5 Block Sync Tests
- [x] Test syncing from peer with more blocks
- [x] Test batch requests
- [x] Test sync completion detection
- [x] Test handling of missing blocks
- [x] Test sync with multiple peers

**Implementation Notes:**
- Message types: BlocksRequest (137), BlocksResponse (138)
- Created `blockstore/memory.go` with MemoryBlockStore for testing
- Added ErrBlockExists alias and ErrInvalidBlock to types/errors.go
- State machine with automatic transitions based on peer heights
- Optional validator callback for application-level block validation
- Sync completion callback notification (onSyncComplete)
- 18 test functions covering all functionality

---

## Phase 12: Block Propagation

**Status: COMPLETED**

### 12.1 Block Reactor
- [x] Create `handlers/blocks.go`
- [x] Define `BlockReactor` struct

### 12.2 Block Broadcasting
- [x] Implement `BroadcastBlock(height, hash, data)`:
  - Send `BlockData` to all connected peers
  - Track blocks sent per peer
  - Skip peers who already have block

### 12.3 Block Reception
- [x] Handle incoming `BlockData`:
  - Check if already have block (peer tracking)
  - Validate block (app callback)
  - Store in BlockStore
  - Mark as received from peer
  - Relay to other peers who don't have it

### 12.4 Block Propagation Tests
- [x] Test broadcasting new block
- [x] Test receiving and storing block
- [x] Test relay to other peers
- [x] Test duplicate block rejection
- [x] Test per-peer tracking

**Implementation Notes:**
- Message type: BlockData (139)
- Hash verification before storage with penalty on mismatch
- Optional validator callback for application-level validation
- onBlockReceived callback for application notification
- Uses PeerManager.PeersToSendBlock/MarkBlockSent for deduplication
- Stateless reactor design (no per-peer state maintained)
- 12 test functions covering all functionality

---

## Phase 13: Consensus Integration

**Status: COMPLETED**

### 13.1 Consensus Handler Interface
- [x] Create `handlers/consensus.go`
- [x] Define `ConsensusHandler` interface:
  ```go
  type ConsensusHandler interface {
      HandleConsensusMessage(peerID peer.ID, data []byte) error
  }
  ```

### 13.2 Consensus Message Pass-through
- [x] Route incoming consensus stream messages to handler
- [x] Provide `SendConsensusMessage(peerID, data)` method
- [x] Provide `BroadcastConsensusMessage(data)` method

### 13.3 Consensus Tests
- [x] Test message routing to handler
- [x] Test SendConsensusMessage
- [x] Test BroadcastConsensusMessage
- [x] Test with mock consensus handler

**Implementation Notes:**
- Pass-through design: blockberry doesn't interpret consensus messages
- ConsensusReactor routes raw bytes to registered ConsensusHandler
- SetHandler/GetHandler for dynamic handler registration
- Silent ignore when no handler is registered (graceful degradation)
- Empty message rejection with ErrInvalidMessage
- 10 test functions with mock handlers for testing

---

## Phase 14: Housekeeping

**Status: COMPLETED**

### 14.1 Housekeeping Reactor
- [x] Create `handlers/housekeeping.go`
- [x] Define `HousekeepingReactor` struct

### 14.2 Latency Probes
- [x] Implement periodic `LatencyRequest` sending
- [x] Handle `LatencyRequest`:
  - Calculate processing time
  - Send `LatencyResponse`
- [x] Handle `LatencyResponse`:
  - Calculate RTT
  - Update peer latency in PeerState
  - Use for peer scoring

### 14.3 Firewall Detection (Stub)
- [x] Handle `FirewallRequest` (stub, log and respond)
- [x] Handle `FirewallResponse` (stub, log result)
- [ ] TODO: Implement actual reachability testing

### 14.4 Housekeeping Tests
- [x] Test latency probe exchange
- [x] Test latency calculation and storage
- [x] Test firewall message handling (stub)

**Implementation Notes:**
- Message types: LatencyRequest (140), LatencyResponse (141), FirewallRequest (142), FirewallResponse (143)
- Background probe loop with configurable interval (probeInterval)
- Pending probe tracking (peerID → request timestamp) for RTT calculation
- Added UpdateLatency method to PeerManager
- RTT calculated as time.Duration for consistent time handling
- Firewall detection is stubbed (echoes endpoint back)
- 14 test functions covering all functionality

---

## Phase 15: Node Coordinator

**Status: COMPLETED**

### 15.1 Node Structure
- [x] Create `node/node.go`
- [x] Define `Node` struct aggregating all components
- [x] Wire dependencies between components

### 15.2 Node Lifecycle
- [x] Implement `NewNode(cfg *Config, opts ...Option) (*Node, error)`:
  - Load or generate private key
  - Initialize all stores
  - Initialize glueberry node
  - Initialize all reactors
  - Wire message routing
- [x] Implement `Start() error`:
  - Start glueberry
  - Start background goroutines (PEX, housekeeping)
  - Connect to seed nodes
  - Begin event loop
- [x] Implement `Stop() error`:
  - Graceful shutdown
  - Close all connections
  - Close all stores

### 15.3 Event Loop
- [x] Handle glueberry events (StateConnected, StateEstablished, etc.)
- [x] Route messages to appropriate reactors
- [x] Handle errors and disconnections

### 15.4 Node Options
- [x] Implement functional options pattern:
  ```go
  type Option func(*Node)
  func WithMempool(mp Mempool) Option
  func WithBlockStore(bs BlockStore) Option
  func WithConsensusHandler(ch ConsensusHandler) Option
  ```

### 15.5 Node Tests
- [x] Test node creation with default config
- [x] Test node start and stop
- [x] Test node with custom components
- [ ] Integration test: two nodes connecting (deferred to Phase 17)

**Implementation Notes:**
- Single node.go file (lifecycle and options inline for simplicity)
- Key loading supports binary and hex-encoded formats
- loadOrGenerateKey creates key with 0600 permissions
- Event loop in background goroutine with select on stop channel
- All reactors started/stopped in order with rollback on failure
- Penalties added for any message handling errors
- Added GetPeerInfo method to HandshakeHandler for peer height access
- 8 test functions covering key loading, multiaddr parsing, and options

---

## Phase 16: Application Interface

**Status: COMPLETED**

### 16.1 Application Interface Definition
- [x] Create `types/application.go`
- [x] Define `Application` interface:
  ```go
  type Application interface {
      // Transaction validation for mempool
      CheckTx(tx []byte) error

      // Block execution
      BeginBlock(height int64, hash []byte) error
      DeliverTx(tx []byte) error
      EndBlock() error
      Commit() (appHash []byte, error)

      // State queries
      Query(path string, data []byte) ([]byte, error)

      // Consensus handling
      HandleConsensusMessage(peerID peer.ID, data []byte) error
  }
  ```

### 16.2 Application Integration
- [ ] Wire Application.CheckTx to mempool AddTx validation (deferred to integration)
- [ ] Wire Application block methods to block commit flow (deferred to integration)
- [x] Application embeds ConsensusHandler for consensus stream compatibility

### 16.3 Null Application
- [x] Create `types/null_app.go`
- [x] Implement no-op Application for testing
- [x] CheckTx always returns nil
- [x] Block methods are no-ops

### 16.4 Application Tests
- [x] Test with null application
- [x] Test CheckTx accepts all transactions
- [x] Test block commit flow (BeginBlock, DeliverTx, EndBlock, Commit)

**Implementation Notes:**
- Application interface includes ConsensusHandler for unified handling
- Added TxValidator and BlockValidator function types for callback-based validation
- NullApplication tracks LastBlockHeight, LastBlockHash, AppHash
- AppHash initialized to 32 zero bytes (standard hash size)
- Compile-time interface check: `var _ Application = (*NullApplication)(nil)`
- 7 test functions covering creation, CheckTx, block flow, Query, and consensus

---

## Phase 17: Integration Testing

### 17.1 Test Utilities
- [ ] Create `testing/helpers.go`
- [ ] Helper to create test nodes with random ports
- [ ] Helper to wait for connection establishment
- [ ] Helper to create mock Application

### 17.2 Two-Node Integration Tests
- [ ] Test: Two nodes connect and complete handshake
- [ ] Test: PEX exchange between nodes
- [ ] Test: Transaction gossip between nodes
- [ ] Test: Block sync from peer
- [ ] Test: Block propagation to peer
- [ ] Test: Consensus message exchange

### 17.3 Multi-Node Integration Tests
- [ ] Test: Three+ nodes form mesh network
- [ ] Test: Transaction propagates through network
- [ ] Test: Block propagates through network
- [ ] Test: New node syncs from existing network

### 17.4 Failure Scenario Tests
- [ ] Test: Node disconnection and reconnection
- [ ] Test: Malformed message handling
- [ ] Test: Peer blacklisting on violation
- [ ] Test: Timeout handling

### 17.5 Performance Tests
- [ ] Benchmark: Transaction throughput
- [ ] Benchmark: Block propagation latency
- [ ] Benchmark: Sync speed for large block histories

---

## Phase 18: Documentation

### 18.1 API Documentation
- [ ] Add godoc comments to all public types and methods
- [ ] Create examples in `example_test.go` files

### 18.2 Usage Examples
- [ ] Create `examples/simple_node/` - minimal node setup
- [ ] Create `examples/custom_mempool/` - custom mempool implementation
- [ ] Create `examples/mock_consensus/` - mock consensus for testing

### 18.3 Configuration Guide
- [ ] Document all configuration options
- [ ] Provide example `config.toml` files
- [ ] Document environment variable overrides (if supported)

### 18.4 Integration Guide
- [ ] Document how to integrate blockberry into an application
- [ ] Document Application interface implementation
- [ ] Document consensus stream usage

---

## Task Tracking Template

For each task, use this template in PROGRESS_REPORT.md:

```markdown
## [Phase X.Y] Task Name

**Status:** Completed / In Progress / Blocked

**Files Modified/Created:**
- `path/to/file1.go`
- `path/to/file2.go`

**Functionality Implemented:**
- Description of what was done

**Test Coverage:**
- List of test functions added
- Coverage percentage if available

**Design Decisions:**
- Any notable decisions made
- Trade-offs considered

**Issues/Notes:**
- Any issues encountered
- Future improvements identified
```

---

## Dependencies Graph

```
Phase 1 (Setup)
    │
    ├──▶ Phase 2 (Config)
    │       │
    │       └──┬──▶ Phase 4 (BlockStore)
    │          │
    │          ├──▶ Phase 5 (StateStore)
    │          │
    │          └──▶ Phase 7 (P2P)
    │                  │
    │                  ├──▶ Phase 8 (Handshake)
    │                  │       │
    │                  │       └──▶ Phase 9 (PEX)
    │                  │
    │                  ├──▶ Phase 11 (BlockSync)
    │                  │
    │                  ├──▶ Phase 12 (BlockProp)
    │                  │
    │                  ├──▶ Phase 13 (Consensus)
    │                  │
    │                  └──▶ Phase 14 (Housekeeping)
    │
    └──▶ Phase 3 (Types)
            │
            └──▶ Phase 6 (Mempool)
                    │
                    └──▶ Phase 10 (Transactions)

All Phases ──▶ Phase 15 (Node) ──▶ Phase 16 (App Interface) ──▶ Phase 17 (Integration) ──▶ Phase 18 (Docs)
```

---

## Estimated Effort

| Phase | Complexity | Notes |
|-------|------------|-------|
| 1. Setup | Low | Boilerplate |
| 2. Config | Low | Straightforward |
| 3. Types | Low | Simple definitions |
| 4. BlockStore | Medium | Storage layer |
| 5. StateStore | Medium | IAVL integration |
| 6. Mempool | Medium | Hash-based storage |
| 7. P2P | Medium | Glueberry integration |
| 8. Handshake | Medium | Protocol state machine |
| 9. PEX | Medium | Discovery logic |
| 10. Transactions | Medium | Gossip protocol |
| 11. BlockSync | Medium | Sync state machine |
| 12. BlockProp | Low-Medium | Simpler than sync |
| 13. Consensus | Low | Pass-through |
| 14. Housekeeping | Low | Utility functions |
| 15. Node | Medium-High | Orchestration |
| 16. App Interface | Medium | Integration layer |
| 17. Integration | High | Complex scenarios |
| 18. Docs | Low | Documentation |
