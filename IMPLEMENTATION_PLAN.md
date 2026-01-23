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
| 6 | Mempool | Merkleized transaction pool | Phase 3 |
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

### 1.1 Initialize Go Module
- [ ] Create `go.mod` with module `github.com/blockberries/blockberry`
- [ ] Add dependencies:
  - `github.com/blockberries/glueberry`
  - `github.com/blockberries/cramberry`
  - `github.com/cosmos/iavl`
  - `github.com/syndtr/goleveldb` (or `github.com/dgraph-io/badger`)
  - `github.com/BurntSushi/toml`
  - `github.com/stretchr/testify`
- [ ] Run `go mod tidy`

### 1.2 Create Makefile
- [ ] `build` target: compile all packages
- [ ] `test` target: run tests with race detection (`go test -race ./...`)
- [ ] `lint` target: run golangci-lint
- [ ] `generate` target: regenerate cramberry code
- [ ] `clean` target: remove build artifacts

### 1.3 Create Directory Structure
- [ ] Create package directories as per ARCHITECTURE.md
- [ ] Add `.gitignore` for Go projects
- [ ] Create empty `PROGRESS_REPORT.md`

### 1.4 Setup Linting
- [ ] Create `.golangci.yml` with appropriate rules
- [ ] Ensure lint passes on empty project

---

## Phase 2: Configuration

### 2.1 Define Config Structures
- [ ] Create `config/config.go`
- [ ] Define `Config` struct with all nested configs:
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
- [ ] Define `NodeConfig`: chain_id, protocol_version, private_key_path
- [ ] Define `NetworkConfig`: listen_addrs, max_peers, timeouts, seeds
- [ ] Define `PEXConfig`: enabled, request_interval, max_addresses
- [ ] Define `MempoolConfig`: max_txs, max_bytes, cache_size
- [ ] Define `BlockStoreConfig`: backend, path
- [ ] Define `StateStoreConfig`: path, cache_size
- [ ] Define `HousekeepingConfig`: latency_probe_interval

### 2.2 Config Loading
- [ ] Implement `LoadConfig(path string) (*Config, error)`
- [ ] Support TOML format
- [ ] Apply sensible defaults for optional fields
- [ ] Validate required fields

### 2.3 Config Validation
- [ ] Validate chain_id is non-empty
- [ ] Validate listen_addrs are valid multiaddrs
- [ ] Validate paths are writable
- [ ] Validate numeric ranges (max_peers > 0, etc.)

### 2.4 Default Config Generation
- [ ] Implement `DefaultConfig() *Config`
- [ ] Implement `WriteConfigFile(path string, cfg *Config) error`

### 2.5 Config Tests
- [ ] Test loading valid config
- [ ] Test loading config with missing required fields
- [ ] Test default value application
- [ ] Test validation errors

---

## Phase 3: Types & Errors

### 3.1 Common Types
- [ ] Create `types/types.go`
- [ ] Define type aliases for clarity:
  ```go
  type Height int64
  type Hash []byte
  type Tx []byte
  type Block []byte
  ```
- [ ] Define `TxHash(tx Tx) Hash` helper function

### 3.2 Error Definitions
- [ ] Create `types/errors.go`
- [ ] Define sentinel errors:
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
- [ ] Create `types/hash.go`
- [ ] Implement `HashTx(tx Tx) Hash` using SHA256
- [ ] Implement `HashBlock(block Block) Hash`
- [ ] Add tests for hash functions

---

## Phase 4: Block Store

### 4.1 Define Interface
- [ ] Create `blockstore/store.go`
- [ ] Define `BlockStore` interface:
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
- [ ] Create `blockstore/leveldb.go`
- [ ] Implement key encoding:
  - Height → Hash: `"H:" + binary.BigEndian(height)`
  - Hash → Data: `"B:" + hash`
  - Metadata: `"M:height"`, `"M:base"`
- [ ] Implement `NewLevelDBBlockStore(path string) (*LevelDBBlockStore, error)`
- [ ] Implement all interface methods
- [ ] Handle atomic batch writes for SaveBlock

### 4.3 Block Store Tests
- [ ] Test SaveBlock and LoadBlock
- [ ] Test LoadBlockByHash
- [ ] Test HasBlock
- [ ] Test Height/Base tracking
- [ ] Test persistence across restarts
- [ ] Test concurrent access

### 4.4 (Optional) BadgerDB Implementation
- [ ] Create `blockstore/badger.go` (if needed)
- [ ] Implement same interface

---

## Phase 5: State Store

### 5.1 Define Interface
- [ ] Create `statestore/store.go`
- [ ] Define `StateStore` interface:
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
- [ ] Create `statestore/iavl.go`
- [ ] Import `github.com/cosmos/iavl`
- [ ] Implement `NewIAVLStore(path string, cacheSize int) (*IAVLStore, error)`
- [ ] Wrap IAVL MutableTree
- [ ] Implement all interface methods
- [ ] Handle version loading on startup

### 5.3 Proof Type
- [ ] Define `Proof` struct wrapping IAVL proof
- [ ] Implement proof verification method

### 5.4 State Store Tests
- [ ] Test Get/Set/Delete operations
- [ ] Test Commit and RootHash
- [ ] Test version loading
- [ ] Test proof generation and verification
- [ ] Test persistence across restarts
- [ ] Test concurrent access

---

## Phase 6: Mempool

### 6.1 Define Interface
- [ ] Create `mempool/mempool.go`
- [ ] Define `Mempool` interface:
  ```go
  type Mempool interface {
      AddTx(tx []byte) error
      RemoveTxs(hashes [][]byte)
      ReapTxs(maxBytes int64) [][]byte
      HasTx(hash []byte) bool
      GetTx(hash []byte) ([]byte, error)
      Size() int
      SizeBytes() int64
      RootHash() []byte
      Flush()
  }
  ```

### 6.2 Merkle Tree for Mempool
- [ ] Create `mempool/merkle.go`
- [ ] Implement in-memory merkle tree for tx hashes
- [ ] Support efficient insert/delete
- [ ] Maintain root hash updated on every operation

### 6.3 Merkleized Mempool Implementation
- [ ] Create `mempool/merkle_mempool.go`
- [ ] Implement `MerkleMempool` struct:
  ```go
  type MerkleMempool struct {
      txs        map[string][]byte    // hash → tx data
      merkleTree *MerkleTree          // for root hash
      order      []string             // insertion order for ReapTxs
      maxTxs     int
      maxBytes   int64
      mu         sync.RWMutex
  }
  ```
- [ ] Implement all interface methods
- [ ] Enforce size limits in AddTx
- [ ] Maintain merkle tree on add/remove

### 6.4 Mempool Configuration
- [ ] Support configuration from `MempoolConfig`
- [ ] Factory function: `NewMempool(cfg MempoolConfig) Mempool`

### 6.5 Mempool Tests
- [ ] Test AddTx and GetTx
- [ ] Test RemoveTxs
- [ ] Test ReapTxs ordering and size limits
- [ ] Test HasTx with merkle proof efficiency
- [ ] Test RootHash updates
- [ ] Test size limits enforcement
- [ ] Test Flush
- [ ] Test concurrent access
- [ ] Benchmark AddTx, HasTx operations

---

## Phase 7: P2P Layer

### 7.1 Peer State
- [ ] Create `p2p/peer_state.go`
- [ ] Define `PeerState` struct:
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
- [ ] Implement methods for tracking exchanged items
- [ ] Implement penalty point accumulation and decay

### 7.2 Peer Manager
- [ ] Create `p2p/peer_manager.go`
- [ ] Implement `PeerManager` struct
- [ ] Track all connected peers
- [ ] Implement `AddPeer`, `RemovePeer`, `GetPeer`
- [ ] Implement `GetPeerState(peerID) *PeerState`
- [ ] Implement `MarkTxSent`, `MarkTxReceived`
- [ ] Implement `MarkBlockSent`, `MarkBlockReceived`
- [ ] Implement `ShouldSendTx(peerID, txHash) bool`
- [ ] Implement `ShouldSendBlock(peerID, height) bool`

### 7.3 Peer Scoring
- [ ] Create `p2p/scoring.go`
- [ ] Define penalty point thresholds
- [ ] Implement `AddPenalty(peerID, points int, reason string)`
- [ ] Implement `GetBanDuration(peerID) time.Duration`
- [ ] Implement decay goroutine (1 point/hour)

### 7.4 Glueberry Wrapper
- [ ] Create `p2p/network.go`
- [ ] Wrap glueberry.Node with blockberry-specific logic
- [ ] Define stream names as constants:
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
- [ ] Implement `Send(peerID, stream, data)` wrapper
- [ ] Implement `Broadcast(stream, data)` wrapper

### 7.5 P2P Tests
- [ ] Test PeerState tracking
- [ ] Test PeerManager operations
- [ ] Test scoring and penalty system
- [ ] Test concurrent access to peer state

---

## Phase 8: Handshake

### 8.1 Handshake Handler
- [ ] Create `handlers/handshake.go`
- [ ] Define `HandshakeHandler` struct
- [ ] Implement state machine for handshake:
  - StateInit → HelloSent → HelloReceived → StreamsPrepared → Finalized

### 8.2 HelloRequest Handling
- [ ] On `StateConnected` event, send `HelloRequest`
- [ ] On receiving `HelloRequest`:
  - Validate chain_id matches → else blacklist
  - Validate version matches → else blacklist
  - Send `HelloResponse` with acceptance and public key

### 8.3 HelloResponse Handling
- [ ] On receiving `HelloResponse`:
  - If not accepted, disconnect
  - Call `glueberry.PrepareStreams` with peer's public key
  - Send `HelloFinalize`

### 8.4 HelloFinalize Handling
- [ ] On receiving `HelloFinalize`:
  - If both sides ready, call `glueberry.FinalizeHandshake`
  - Transition to `StateEstablished`
  - Initialize peer state in PeerManager

### 8.5 Handshake Tests
- [ ] Test successful handshake between two nodes
- [ ] Test chain_id mismatch rejection
- [ ] Test version mismatch rejection
- [ ] Test handshake timeout handling
- [ ] Test concurrent handshakes with multiple peers

---

## Phase 9: PEX (Peer Exchange)

### 9.1 Address Book
- [ ] Create `pex/address_book.go`
- [ ] Define `AddressBook` struct:
  ```go
  type AddressBook struct {
      Peers  map[peer.ID]*AddressEntry
      Seeds  []SeedNode
      path   string
      mu     sync.RWMutex
  }
  ```
- [ ] Implement JSON persistence
- [ ] Implement `AddPeer`, `RemovePeer`, `GetPeers`
- [ ] Implement `MarkSeed(peerID)` to mark discovered peer as seed
- [ ] Implement `GetPeersForExchange(lastSeen time.Time, max int)`

### 9.2 Seed Node Handling
- [ ] Load seeds from config on startup
- [ ] Connect to seed nodes on bootstrap
- [ ] Mark seed connections appropriately in PeerState

### 9.3 PEX Reactor
- [ ] Create `pex/reactor.go`
- [ ] Implement periodic `AddressRequest` sending (configurable interval)
- [ ] Handle `AddressRequest`:
  - Filter peers by last_seen
  - Exclude blacklisted peers
  - Return up to max_addresses
- [ ] Handle `AddressResponse`:
  - Add new peers to address book
  - Update existing peer info
  - Optionally initiate connections

### 9.4 Connection Management
- [ ] Respect max_inbound_peers and max_outbound_peers limits
- [ ] Implement peer selection strategy for new connections
- [ ] Prefer peers with better scores

### 9.5 PEX Tests
- [ ] Test AddressBook persistence
- [ ] Test seed node loading
- [ ] Test PEX message exchange
- [ ] Test peer limit enforcement
- [ ] Test address filtering by last_seen

---

## Phase 10: Transactions

### 10.1 Transactions Reactor
- [ ] Create `handlers/transactions.go`
- [ ] Define `TransactionsReactor` struct

### 10.2 TransactionsRequest/Response
- [ ] Implement periodic transaction gossip:
  - Send `TransactionsRequest` with batch_size
  - On receiving request, respond with tx hashes from mempool
- [ ] Handle `TransactionsResponse`:
  - Check which txs we already have (mempool.HasTx)
  - Track received tx hashes

### 10.3 TransactionDataRequest/Response
- [ ] Request full tx data for missing transactions
- [ ] On receiving data request, send tx data from mempool
- [ ] On receiving data response:
  - Validate and add to mempool
  - Mark as received from peer

### 10.4 Per-Peer Tracking
- [ ] Use PeerManager to track txs sent/received
- [ ] Avoid re-requesting txs from same peer
- [ ] Avoid re-sending txs to peers who already have them

### 10.5 Transactions Tests
- [ ] Test transaction gossip between two nodes
- [ ] Test batch request/response
- [ ] Test missing tx data requests
- [ ] Test per-peer tracking prevents duplicates
- [ ] Test with full mempool (reject new txs)

---

## Phase 11: Block Sync

### 11.1 Sync State Machine
- [ ] Create `sync/reactor.go`
- [ ] Define sync states: `Synced`, `Syncing`
- [ ] Track peer heights
- [ ] Determine when to start/stop syncing

### 11.2 BlocksRequest/Response
- [ ] Implement `BlocksRequest` sending:
  - Request batch of blocks starting from `since` height
- [ ] Handle `BlocksRequest`:
  - Load blocks from BlockStore
  - Return up to batch_size blocks

### 11.3 Block Validation and Storage
- [ ] On receiving `BlocksResponse`:
  - Validate blocks (app callback)
  - Store in BlockStore sequentially
  - Update sync progress

### 11.4 Sync Completion
- [ ] Detect when caught up to peers
- [ ] Transition from Syncing to Synced
- [ ] Notify application of sync completion

### 11.5 Block Sync Tests
- [ ] Test syncing from peer with more blocks
- [ ] Test batch requests
- [ ] Test sync completion detection
- [ ] Test handling of missing blocks
- [ ] Test sync with multiple peers

---

## Phase 12: Block Propagation

### 12.1 Block Reactor
- [ ] Create `handlers/blocks.go`
- [ ] Define `BlockReactor` struct

### 12.2 Block Broadcasting
- [ ] Implement `BroadcastBlock(height, hash, data)`:
  - Send `BlockData` to all connected peers
  - Track blocks sent per peer
  - Skip peers who already have block

### 12.3 Block Reception
- [ ] Handle incoming `BlockData`:
  - Check if already have block (peer tracking)
  - Validate block (app callback)
  - Store in BlockStore
  - Mark as received from peer
  - Relay to other peers who don't have it

### 12.4 Block Propagation Tests
- [ ] Test broadcasting new block
- [ ] Test receiving and storing block
- [ ] Test relay to other peers
- [ ] Test duplicate block rejection
- [ ] Test per-peer tracking

---

## Phase 13: Consensus Integration

### 13.1 Consensus Handler Interface
- [ ] Create `handlers/consensus.go`
- [ ] Define `ConsensusHandler` interface:
  ```go
  type ConsensusHandler interface {
      HandleConsensusMessage(peerID peer.ID, data []byte) error
  }
  ```

### 13.2 Consensus Message Pass-through
- [ ] Route incoming consensus stream messages to handler
- [ ] Provide `SendConsensusMessage(peerID, data)` method
- [ ] Provide `BroadcastConsensusMessage(data)` method

### 13.3 Consensus Tests
- [ ] Test message routing to handler
- [ ] Test SendConsensusMessage
- [ ] Test BroadcastConsensusMessage
- [ ] Test with mock consensus handler

---

## Phase 14: Housekeeping

### 14.1 Housekeeping Reactor
- [ ] Create `handlers/housekeeping.go`
- [ ] Define `HousekeepingReactor` struct

### 14.2 Latency Probes
- [ ] Implement periodic `LatencyRequest` sending
- [ ] Handle `LatencyRequest`:
  - Calculate processing time
  - Send `LatencyResponse`
- [ ] Handle `LatencyResponse`:
  - Calculate RTT
  - Update peer latency in PeerState
  - Use for peer scoring

### 14.3 Firewall Detection (Stub)
- [ ] Handle `FirewallRequest` (stub, log and respond)
- [ ] Handle `FirewallResponse` (stub, log result)
- [ ] TODO: Implement actual reachability testing

### 14.4 Housekeeping Tests
- [ ] Test latency probe exchange
- [ ] Test latency calculation and storage
- [ ] Test firewall message handling (stub)

---

## Phase 15: Node Coordinator

### 15.1 Node Structure
- [ ] Create `node/node.go`
- [ ] Define `Node` struct aggregating all components
- [ ] Wire dependencies between components

### 15.2 Node Lifecycle
- [ ] Create `node/lifecycle.go`
- [ ] Implement `NewNode(cfg *Config) (*Node, error)`:
  - Load or generate private key
  - Initialize all stores
  - Initialize glueberry node
  - Initialize all reactors
  - Wire message routing
- [ ] Implement `Start() error`:
  - Start glueberry
  - Start background goroutines (PEX, housekeeping)
  - Connect to seed nodes
  - Begin event loop
- [ ] Implement `Stop() error`:
  - Graceful shutdown
  - Close all connections
  - Close all stores

### 15.3 Event Loop
- [ ] Handle glueberry events (StateConnected, StateEstablished, etc.)
- [ ] Route messages to appropriate reactors
- [ ] Handle errors and disconnections

### 15.4 Node Options
- [ ] Create `node/options.go`
- [ ] Implement functional options pattern:
  ```go
  type Option func(*Node)
  func WithMempool(mp Mempool) Option
  func WithBlockStore(bs BlockStore) Option
  func WithConsensusHandler(ch ConsensusHandler) Option
  ```

### 15.5 Node Tests
- [ ] Test node creation with default config
- [ ] Test node start and stop
- [ ] Test node with custom components
- [ ] Integration test: two nodes connecting

---

## Phase 16: Application Interface

### 16.1 Application Interface Definition
- [ ] Create `types/application.go`
- [ ] Define `Application` interface:
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
      ConsensusHandler
  }
  ```

### 16.2 Application Integration
- [ ] Wire Application.CheckTx to mempool AddTx validation
- [ ] Wire Application block methods to block commit flow
- [ ] Wire Application.ConsensusHandler to consensus stream

### 16.3 Null Application
- [ ] Create `types/null_app.go`
- [ ] Implement no-op Application for testing
- [ ] CheckTx always returns nil
- [ ] Block methods are no-ops

### 16.4 Application Tests
- [ ] Test with null application
- [ ] Test CheckTx integration with mempool
- [ ] Test block commit flow

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
| 6. Mempool | Medium-High | Merkle tree complexity |
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
