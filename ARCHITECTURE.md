# Blockberry Architecture

## Overview

Blockberry is a modular blockchain node framework written in Go. It provides the foundational infrastructure for building blockchain nodes while leaving consensus algorithm implementation to the integrating application.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              APPLICATION LAYER                               │
│                    (Consensus Engine, Business Logic, RPC)                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                               BLOCKBERRY                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │    Node     │  │   Mempool   │  │ BlockStore  │  │     StateStore      │ │
│  │ Coordinator │  │  (Priority/ │  │  Interface  │  │  (IAVL + ICS23)     │ │
│  │             │  │   TTL)      │  │             │  │                     │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ PeerManager │  │     PEX     │  │  BlockSync  │  │      Handlers       │ │
│  │ & RateLimiter│ │   Reactor   │  │   Reactor   │  │  (Stream-specific)  │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│  ┌─────────────┐  ┌─────────────┐                                           │
│  │   Metrics   │  │   Logging   │                                           │
│  │ (Prometheus)│  │   (slog)    │                                           │
│  └─────────────┘  └─────────────┘                                           │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                               GLUEBERRY                                      │
│              (P2P Networking, Encrypted Streams, Reconnection)               │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                                LIBP2P                                        │
│                    (Transport, Multiplexing, NAT Traversal)                  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Core Design Principles

1. **Modularity**: Pluggable interfaces for mempool, block storage, and application logic
2. **Consensus Agnosticism**: The framework handles networking and storage; applications implement consensus
3. **Opaque Data**: Blocks and transactions are treated as opaque `[]byte` by blockberry
4. **Efficient Sync**: Merkleized data structures for fast existence proofs and sync
5. **Peer Accountability**: Track per-peer state to avoid redundant transfers and enable scoring
6. **Dual-Mode Operation**: Supports both simple mempool (full nodes) and DAG mempool (validators)

## Module Dependencies

```
github.com/blockberries/blockberry
    ├── github.com/blockberries/glueberry v1.2.9   (P2P networking)
    ├── github.com/blockberries/cramberry v1.5.3   (Serialization)
    ├── github.com/cosmos/iavl v1.3.5              (Merkleized KV store)
    ├── github.com/cosmos/ics23/go v0.10.0         (Proof verification)
    ├── github.com/prometheus/client_golang        (Metrics)
    ├── github.com/hashicorp/golang-lru/v2         (LRU caches)
    └── github.com/syndtr/goleveldb                (Storage backend)
```

---

## Component Architecture

### 1. Node (Coordinator)

The `Node` is the main entry point that orchestrates all components.

```go
type Node struct {
    config      *Config
    glueberry   *glueberry.Node
    mempool     Mempool
    blockStore  BlockStore
    stateStore  StateStore
    peerManager *PeerManager

    // Reactors (message handlers)
    pexReactor    *PexReactor
    txReactor     *TransactionsReactor
    blockReactor  *BlockReactor
    syncReactor   *BlockSyncReactor
    housekeeper   *HousekeepingReactor

    // App callbacks
    consensusHandler ConsensusHandler

    // Lifecycle
    ctx    context.Context
    cancel context.CancelFunc
}
```

**Responsibilities:**
- Initialize and wire all components
- Start/stop glueberry node
- Route incoming messages to appropriate reactors
- Handle connection lifecycle events
- Expose APIs for application integration

### 2. Streams and Message Flow

Blockberry uses glueberry's encrypted stream multiplexing with the following streams:

| Stream Name | Direction | Message Type | Purpose |
|-------------|-----------|--------------|---------|
| `handshake` | Bidirectional | `HandshakeMessage` | Connection establishment (unencrypted) |
| `pex` | Bidirectional | `PexMessage` | Peer discovery and exchange |
| `transactions` | Bidirectional | `TransactionsMessage` | Transaction gossiping |
| `blocksync` | Bidirectional | `BlockSyncMessage` | Historical block synchronization |
| `blocks` | Bidirectional | `BlockMessage` | Real-time block propagation |
| `consensus` | Bidirectional | Raw `[]byte` | Application-defined consensus |
| `housekeeping` | Bidirectional | `HousekeepingMessages` | Latency probes, firewall detection |

**Note**: When integrated with Looseberry (validators), three additional streams are used:

| Stream | Direction | Message Type | Purpose |
|--------|-----------|--------------|---------|
| `looseberry-batches` | Bidirectional | Raw `[]byte` | Worker-to-worker batch dissemination |
| `looseberry-headers` | Bidirectional | Raw `[]byte` | Primary-to-primary headers and votes |
| `looseberry-sync` | Bidirectional | Raw `[]byte` | Certificate synchronization |

#### Message Flow Diagram

```
┌──────────────┐                                    ┌──────────────┐
│   Node A     │                                    │   Node B     │
└──────┬───────┘                                    └───────┬──────┘
       │                                                    │
       │  ══════════════ HANDSHAKE PHASE ═══════════════   │
       │                                                    │
       │  [StateConnected event]                            │
       │─────────────── HelloRequest ─────────────────────▶│
       │                                                    │
       │                    [Validate chain_id, version]    │
       │◀──────────────── HelloResponse ───────────────────│
       │                                                    │
       │  [PrepareStreams with pubkey]                      │
       │─────────────── HelloFinalize ────────────────────▶│
       │                                                    │
       │                    [PrepareStreams + Finalize]     │
       │◀──────────────── HelloFinalize ───────────────────│
       │                                                    │
       │  [StateEstablished - encrypted streams ready]      │
       │                                                    │
       │  ══════════════ OPERATIONAL PHASE ════════════════│
       │                                                    │
       │◀─────────────── PexMessage ───────────────────────│
       │─────────────── TransactionsMessage ──────────────▶│
       │◀─────────────── BlockMessage ─────────────────────│
       │─────────────── ConsensusMessage ─────────────────▶│
       │                                                    │
```

### 3. Handshake Protocol

The handshake validates peer compatibility before establishing encrypted communication.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         HANDSHAKE FLOW                                   │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Glueberry connects (libp2p level)                                   │
│     └─▶ StateConnected event fired                                      │
│                                                                          │
│  2. Both nodes send HelloRequest:                                       │
│     ┌──────────────────────────────────────┐                            │
│     │ HelloRequest                         │                            │
│     │   node_id:      string (libp2p ID)   │                            │
│     │   version:      int32  (protocol)    │                            │
│     │   chain_id:     string               │                            │
│     │   timestamp:    int64  (unix nano)   │                            │
│     │   inbound_url:  string (optional)    │                            │
│     │   inbound_port: int32  (optional)    │                            │
│     └──────────────────────────────────────┘                            │
│                                                                          │
│  3. Each node validates the peer's HelloRequest:                        │
│     - chain_id must match ──▶ else BLACKLIST                            │
│     - version must match  ──▶ else BLACKLIST                            │
│                                                                          │
│  4. If valid, send HelloResponse:                                       │
│     ┌──────────────────────────────────────┐                            │
│     │ HelloResponse                        │                            │
│     │   accepted:   bool (true if valid)   │                            │
│     │   public_key: bytes (Ed25519 pubkey) │                            │
│     └──────────────────────────────────────┘                            │
│                                                                          │
│  5. On receiving HelloResponse with accepted=true:                      │
│     - Call glueberry.PrepareStreams(peerID, pubkey, streamNames)        │
│     - Send HelloFinalize{success: true}                                 │
│                                                                          │
│  6. On receiving HelloFinalize with success=true:                       │
│     - Call glueberry.FinalizeHandshake(peerID)                         │
│     - State transitions to StateEstablished                             │
│                                                                          │
│  7. Encrypted streams now active for all registered stream names        │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 4. Mempool

The mempool manages pending transactions awaiting inclusion in blocks. Blockberry supports two mempool implementations:

1. **Simple Mempool**: Hash-based in-memory storage (used by full nodes)
2. **DAG Mempool**: Looseberry integration (used by validators)

#### Base Mempool Interface

```go
type Mempool interface {
    // AddTx adds a transaction to the mempool.
    // Returns error if transaction is invalid or mempool is full.
    AddTx(tx []byte) error

    // RemoveTxs removes transactions (typically after block commit).
    RemoveTxs(hashes [][]byte)

    // ReapTxs returns transactions for block proposal up to maxBytes.
    ReapTxs(maxBytes int64) [][]byte

    // HasTx checks if a transaction exists in the mempool.
    HasTx(hash []byte) bool

    // GetTx retrieves a transaction by hash.
    GetTx(hash []byte) ([]byte, error)

    // Size returns the number of transactions.
    Size() int

    // SizeBytes returns total size in bytes.
    SizeBytes() int64

    // Flush removes all transactions.
    Flush()
}
```

#### Extended DAGMempool Interface

For validators using Looseberry, an extended interface provides DAG-specific methods:

```go
type DAGMempool interface {
    Mempool  // Embeds base interface

    // Pull certified batches for block building
    // Returns batches in deterministic order: by round ASC, then by validator index ASC
    // Only returns batches from rounds > lastCommittedRound
    ReapCertifiedBatches(maxBytes int64) []CertifiedBatch

    // Notify that consensus has committed up to a round
    // Enables garbage collection of older rounds
    NotifyCommitted(round uint64)

    // Update validator set (epoch change)
    UpdateValidatorSet(validators ValidatorSet)

    // Get current DAG round
    CurrentRound() uint64
}

type CertifiedBatch struct {
    Certificate *looseberry.Certificate
    Batches     []*looseberry.Batch
}
```

#### Hash-Based In-Memory Implementation

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         MEMPOOL STRUCTURE                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│   Primary Storage: map[txHash][]byte                                    │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │  hash1 → tx1 data                                               │   │
│   │  hash2 → tx2 data                                               │   │
│   │  hash3 → tx3 data                                               │   │
│   │  ...                                                            │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
│   Insertion Order: []txHash (for ReapTxs)                               │
│   ┌─────────────────────────────────────────────────────────────────┐   │
│   │  [hash1, hash2, hash3, ...]                                     │   │
│   └─────────────────────────────────────────────────────────────────┘   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key Features:**
- O(1) hash map lookups for HasTx and GetTx
- Maintains insertion order for fair transaction ordering
- Configurable size limits (count and bytes)
- Thread-safe with RWMutex

#### Priority-Based Mempool

For applications that need transaction ordering by priority (e.g., gas price):

```go
type PriorityMempool struct {
    txs          map[string]*mempoolTx
    heap         txHeap              // Max-heap for priority ordering
    maxTxs       int
    maxBytes     int64
    priorityFunc PriorityFunc        // Configurable priority calculation
}
```

**Features:**
- Max-heap ordering for O(1) highest priority access
- Configurable `PriorityFunc` for custom priority calculation
- Automatic eviction of lowest priority transactions when full
- Built-in priority functions: `DefaultPriorityFunc`, `SizePriorityFunc`

#### TransactionsReactor Modes

The TransactionsReactor operates in two modes depending on node type:

**Active Mode (Full Nodes)**:
- Actively gossips transactions to peers
- Periodically requests transactions from connected peers
- Adds received transactions to simple mempool
- Used by full nodes that don't participate in consensus

**Passive Mode (Validators with Looseberry)**:
- Receives transactions from peers but does NOT initiate gossip
- Routes received transactions to Looseberry.AddTx()
- Disables outbound gossip requests
- Used by validators running DAG mempool

```go
type TransactionsReactor struct {
    mempool      Mempool           // Simple mempool (nil for validators)
    looseberry   DAGMempool        // Looseberry integration (nil for full nodes)
    passiveMode  bool              // true if looseberry != nil

    // Gossip loop only runs if !passiveMode
    gossipTicker *time.Ticker
}

func (r *TransactionsReactor) handleTransactionDataResponse(peerID peer.ID, data []byte) error {
    // ... deserialize and validate ...

    for _, txData := range resp.Transactions {
        // Route to appropriate mempool
        if r.looseberry != nil {
            // Validator: route to Looseberry
            _ = r.looseberry.AddTx(txData.Data)
        } else if r.mempool != nil {
            // Full node: add to simple mempool
            _ = r.mempool.AddTx(txData.Data)
        }
    }

    return nil
}
```

This dual-mode design ensures:
- Full nodes actively gossip transactions to the network
- Validators receive transactions from full nodes without re-gossiping
- Looseberry's DAG protocol handles validator-to-validator dissemination
- No redundant transaction propagation

#### TTL Mempool

Extends PriorityMempool with automatic transaction expiration:

```go
type TTLMempool struct {
    // Wraps priority-based ordering with expiration
    txs             map[string]*ttlTx
    priorityHeap    ttlTxHeap
    ttl             time.Duration      // Default TTL
    cleanupInterval time.Duration      // Background cleanup interval
}
```

**Features:**
- Background goroutine for automatic expired transaction removal
- `AddTxWithTTL(tx, ttl)` for custom per-transaction TTL
- `GetTTL(hash)`, `ExtendTTL(hash, extension)`, `SetTTL(hash, expiresAt)`
- `SizeActive()` returns count of non-expired transactions
- `ReapTxs` automatically excludes expired transactions

### 5. Block Store

Persistent storage for committed blocks.

#### Interface

```go
type BlockStore interface {
    // SaveBlock persists a block.
    SaveBlock(height int64, hash []byte, data []byte) error

    // LoadBlock retrieves a block by height.
    LoadBlock(height int64) (hash []byte, data []byte, error)

    // LoadBlockByHash retrieves a block by hash.
    LoadBlockByHash(hash []byte) (height int64, data []byte, error)

    // HasBlock checks if a block exists.
    HasBlock(height int64) bool

    // Height returns the latest block height.
    Height() int64

    // Base returns the earliest available block height (for pruned nodes).
    Base() int64

    // Close closes the store.
    Close() error
}
```

#### Storage Schema

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        BLOCK STORE SCHEMA                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Key-Value Layout:                                                       │
│                                                                          │
│  ┌────────────────────────┬──────────────────────────────────────────┐  │
│  │ Key                    │ Value                                    │  │
│  ├────────────────────────┼──────────────────────────────────────────┤  │
│  │ "H:" + height (8 byte) │ block hash (32 bytes)                    │  │
│  │ "B:" + hash (32 byte)  │ block data (variable)                    │  │
│  │ "M:height"             │ latest height (8 bytes)                  │  │
│  │ "M:base"               │ base height (8 bytes)                    │  │
│  └────────────────────────┴──────────────────────────────────────────┘  │
│                                                                          │
│  Lookup Patterns:                                                        │
│  - By height: H:{height} → hash → B:{hash} → data                       │
│  - By hash:   B:{hash} → data (height stored in block)                  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 6. State Store (IAVL)

Merkleized key-value store for application state using the cosmos/iavl library.

```go
type StateStore interface {
    // Get retrieves a value by key.
    Get(key []byte) ([]byte, error)

    // Has checks if a key exists.
    Has(key []byte) (bool, error)

    // Set stores a key-value pair.
    Set(key []byte, value []byte) error

    // Delete removes a key.
    Delete(key []byte) error

    // Commit persists all changes and returns the new root hash.
    Commit() (hash []byte, version int64, error)

    // RootHash returns the current merkle root.
    RootHash() []byte

    // Version returns the current version.
    Version() int64

    // LoadVersion loads a specific version.
    LoadVersion(version int64) error

    // GetProof generates a merkle proof for a key.
    GetProof(key []byte) (*Proof, error)

    // Close closes the store.
    Close() error
}
```

**IAVL Properties:**
- Immutable, versioned Merkle tree
- O(log n) reads, writes, and proofs
- Supports historical queries by version
- Used by Cosmos SDK, battle-tested

### 7. Peer Manager

Tracks per-peer state for efficient sync and reputation.

```go
type PeerManager struct {
    peers map[peer.ID]*PeerState
    mu    sync.RWMutex
}

type PeerState struct {
    // Identity
    PeerID    peer.ID
    PublicKey []byte

    // Exchange tracking with LRU caches (bounded memory)
    knownTxs    *lru.Cache[string, struct{}]   // MaxKnownTxsPerPeer = 20000
    knownBlocks *lru.Cache[string, struct{}]   // MaxKnownBlocksPerPeer = 2000

    // Scoring
    Score         int64
    PenaltyPoints int64
    LastSeen      time.Time
    Latency       time.Duration

    // Connection info
    ConnectedAt   time.Time
    IsSeed        bool
    IsOutbound    bool
}
```

**Memory Management:**
- LRU caches prevent unbounded memory growth from transaction/block tracking
- `MaxKnownTxsPerPeer = 20000` entries per peer
- `MaxKnownBlocksPerPeer = 2000` entries per peer
- Thread-safe with `hashicorp/golang-lru/v2`

### 7a. Rate Limiter

Per-peer, per-stream rate limiting using token bucket algorithm:

```go
type RateLimiter struct {
    peers   map[peer.ID]*peerLimiter
    limits  RateLimits
}

type RateLimits struct {
    MessagesPerSecond map[string]float64  // Per-stream limits
    BytesPerSecond    int64               // Overall bandwidth limit
    BurstSize         int                 // Tokens for burst
}
```

#### Per-Peer Tracking Rationale

When gossiping transactions:
1. Node A sends `TransactionsResponse` with tx hashes it has
2. Node B checks which it already has (hash lookup in mempool)
3. Node B requests full data only for missing txs via `TransactionDataRequest`
4. Both nodes track what was exchanged to avoid re-sending

```
┌──────────────┐                              ┌──────────────┐
│   Node A     │                              │   Node B     │
├──────────────┤                              ├──────────────┤
│ Mempool:     │                              │ Mempool:     │
│  tx1, tx2,   │                              │  tx1, tx3    │
│  tx4, tx5    │                              │              │
└──────┬───────┘                              └───────┬──────┘
       │                                              │
       │ ─── TransactionsRequest{batch_size: 10} ───▶│
       │                                              │
       │◀── TransactionsResponse{[tx1,tx2,tx4,tx5]} ─│
       │                                              │
       │    [B checks: has tx1, missing tx2,tx4,tx5] │
       │                                              │
       │◀─ TransactionDataRequest{[tx2,tx4,tx5]} ────│
       │                                              │
       │── TransactionDataResponse{[tx2,tx4,tx5]} ──▶│
       │                                              │
       │    [A marks: sent tx2,tx4,tx5 to B]         │
       │    [B marks: received tx2,tx4,tx5 from A]   │
       │                                              │
       │    [Next round: A won't re-send these]      │
```

### 8. Peer Exchange (PEX)

Discovers and shares peer addresses.

#### Address Book Structure

```go
type AddressBook struct {
    // Regular peers discovered through operation
    Peers map[peer.ID]*AddressEntry

    // Seed nodes from config (used for bootstrap)
    Seeds []SeedNode

    // Persisted to JSON
    path string
    mu   sync.RWMutex
}

type AddressEntry struct {
    PeerID     peer.ID
    Multiaddrs []multiaddr.Multiaddr
    LastSeen   time.Time
    Latency    time.Duration
    IsSeed     bool              // Can be marked as seed after discovery
    Score      int64
    Source     string            // "config", "pex", "manual"
}

type SeedNode struct {
    Multiaddr multiaddr.Multiaddr
    PeerID    peer.ID           // Optional, discovered on connect
}
```

#### PEX Protocol

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          PEX PROTOCOL                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  1. Periodic Request (configurable interval, e.g., 30 seconds):         │
│                                                                          │
│     ┌──────────────────────────────────────┐                            │
│     │ AddressRequest                       │                            │
│     │   last_seen: int64 (unix timestamp) │ ◀── Only peers seen after  │
│     └──────────────────────────────────────┘                            │
│                                                                          │
│  2. Response with known peers:                                          │
│                                                                          │
│     ┌──────────────────────────────────────┐                            │
│     │ AddressResponse                      │                            │
│     │   peers: []AddressInfo              │                            │
│     │     - multiaddr: string              │                            │
│     │     - last_seen: int64               │                            │
│     │     - latency: int32 (ms)            │                            │
│     │     - node_id: string                │                            │
│     └──────────────────────────────────────┘                            │
│                                                                          │
│  3. On receiving AddressResponse:                                       │
│     - Add new peers to address book                                     │
│     - Update existing peer info                                         │
│     - Optionally initiate connections to new peers                      │
│                                                                          │
│  Constraints:                                                            │
│     - Max peers in response (configurable)                              │
│     - Don't share blacklisted peers                                     │
│     - Respect peer's requested last_seen filter                         │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 9. Block Synchronization

Syncs historical blocks from peers.

#### Sync States

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        BLOCK SYNC STATES                                 │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌─────────────┐     peer height > our height      ┌─────────────────┐  │
│  │   SYNCED    │ ─────────────────────────────────▶│    SYNCING      │  │
│  │  (normal    │                                   │  (catching up)  │  │
│  │  operation) │◀───────────────────────────────── │                 │  │
│  └─────────────┘     caught up to peer height      └─────────────────┘  │
│                                                                          │
│  SYNCED State:                                                           │
│    - Process blocks as they arrive on "blocks" stream                   │
│    - Participate in consensus (if validator)                            │
│                                                                          │
│  SYNCING State:                                                          │
│    - Request blocks in batches via "blocksync" stream                   │
│    - Don't participate in consensus                                     │
│    - Verify and store blocks sequentially                               │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

#### Block Sync Protocol

```
┌──────────────┐                              ┌──────────────┐
│   Node A     │                              │   Node B     │
│  (syncing)   │                              │  (synced)    │
│  height: 100 │                              │  height: 500 │
└──────┬───────┘                              └───────┬──────┘
       │                                              │
       │ ─── BlocksRequest{batch_size:50, since:100}─▶│
       │                                              │
       │◀── BlocksResponse{blocks:[101..150]} ────────│
       │                                              │
       │    [A validates and stores blocks 101-150]   │
       │                                              │
       │ ─── BlocksRequest{batch_size:50, since:150}─▶│
       │                                              │
       │◀── BlocksResponse{blocks:[151..200]} ────────│
       │                                              │
       │    [... continues until caught up ...]       │
```

### 10. Real-Time Block Propagation

New blocks are broadcast immediately via the "blocks" stream.

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     BLOCK PROPAGATION                                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  When a node commits a new block:                                       │
│                                                                          │
│  1. Store in block store                                                │
│  2. Update state store (commit IAVL)                                    │
│  3. Remove included txs from mempool                                    │
│  4. Broadcast BlockData to all connected peers:                         │
│                                                                          │
│     ┌──────────────────────────────────────┐                            │
│     │ BlockData                            │                            │
│     │   height: int64                      │                            │
│     │   hash:   bytes                      │                            │
│     │   data:   bytes (opaque block)       │                            │
│     └──────────────────────────────────────┘                            │
│                                                                          │
│  5. Track: mark block as sent to each peer                              │
│                                                                          │
│  On receiving BlockData:                                                │
│  1. Check if already have block (peer tracking)                         │
│  2. If new: validate, store, propagate to other peers                   │
│  3. Mark: received from sender peer                                     │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 11. Consensus Stream

The consensus stream is a pass-through for application-defined consensus messages.

```go
type ConsensusHandler interface {
    // HandleConsensusMessage is called when a consensus message arrives.
    HandleConsensusMessage(peerID peer.ID, data []byte) error
}

// Node provides method to send consensus messages
func (n *Node) SendConsensusMessage(peerID peer.ID, data []byte) error {
    return n.glueberry.Send(peerID, "consensus", data)
}

func (n *Node) BroadcastConsensusMessage(data []byte) error {
    for _, peerID := range n.glueberry.ConnectedPeers() {
        n.glueberry.Send(peerID, "consensus", data)
    }
    return nil
}
```

### 12. Housekeeping

Utility functions for network health.

#### Latency Measurement

```
┌──────────────┐                              ┌──────────────┐
│   Node A     │                              │   Node B     │
└──────┬───────┘                              └───────┬──────┘
       │                                              │
       │ ─── LatencyRequest{timestamp: T1} ──────────▶│
       │                                              │
       │◀── LatencyResponse{latency: T2-T1} ──────────│
       │                                              │
       │    RTT = now - T1                            │
       │    One-way estimate = RTT / 2                │
       │    Peer's processing: latency field          │
       │                                              │
       │    [Used for peer scoring]                   │
```

#### Firewall Detection (TBD)

```
┌──────────────┐                              ┌──────────────┐
│   Node A     │                              │   Node B     │
│  (testing    │                              │  (helper)    │
│  reachability│                              │              │
└──────┬───────┘                              └───────┬──────┘
       │                                              │
       │ ─── FirewallRequest{endpoint: "ip:port"} ───▶│
       │                                              │
       │    [B attempts to connect to endpoint]       │
       │                                              │
       │◀── FirewallResponse{endpoint: "ip:port"} ───│
       │                                              │
       │    [A now knows if it's reachable]           │
```

---

## Observability

### Prometheus Metrics

Comprehensive metrics for monitoring node health and performance:

```go
type Metrics interface {
    // Peer metrics
    SetPeersTotal(direction string, count int)
    IncPeerConnections(result string)
    IncPeerDisconnections(reason string)

    // Block metrics
    SetBlockHeight(height int64)
    IncBlocksReceived()
    ObserveBlockLatency(seconds float64)
    ObserveBlockSize(bytes int)

    // Transaction metrics
    SetMempoolSize(count int)
    SetMempoolBytes(bytes int64)
    IncTxsReceived()
    IncTxsRejected(reason string)

    // Sync metrics
    SetSyncState(state string)
    SetSyncProgress(progress float64)

    // Message metrics
    IncMessagesReceived(stream string)
    IncMessagesSent(stream string)
    IncMessageErrors(stream, errorType string)

    // HTTP handler for /metrics endpoint
    Handler() any
}
```

**Implementations:**
- `PrometheusMetrics`: Full Prometheus implementation with GaugeVec, CounterVec, Histograms
- `NopMetrics`: Zero-overhead no-op implementation when metrics are disabled

### Structured Logging

Production-ready logging with slog integration:

```go
type Logger struct {
    *slog.Logger
}

// Attribute constructors for blockchain-specific fields
func Component(name string) slog.Attr
func PeerID(id peer.ID) slog.Attr
func Height(h int64) slog.Attr
func Hash(h []byte) slog.Attr
func Stream(name string) slog.Attr
func Duration(d time.Duration) slog.Attr
func Error(err error) slog.Attr
```

**Factory Functions:**
- `NewTextLogger(w, level)`: Human-readable text format
- `NewJSONLogger(w, level)`: Structured JSON format
- `NewProductionLogger()`: JSON to stdout at INFO level
- `NewDevelopmentLogger()`: Text to stdout at DEBUG level
- `NewNopLogger()`: Zero-overhead discarding logger

---

## Configuration

Configuration is loaded from `config.toml`:

```toml
[node]
chain_id = "testchain-1"
protocol_version = 1
private_key_path = "node_key.json"  # Ed25519 key

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
dial_timeout = "3s"
address_book_path = "addrbook.json"

[network.seeds]
# Seed nodes for bootstrap
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/12D3KooW...",
    "/ip4/seed2.example.com/tcp/26656/p2p/12D3KooW...",
]

[pex]
enabled = true
request_interval = "30s"
max_addresses_per_response = 100

[mempool]
max_txs = 5000
max_bytes = 1073741824  # 1GB
cache_size = 10000      # Recent tx hash cache

[blockstore]
backend = "leveldb"     # or "badgerdb"
path = "data/blockstore"

[statestore]
path = "data/state"
cache_size = 10000

[housekeeping]
latency_probe_interval = "60s"

[metrics]
enabled = true
namespace = "blockberry"
listen_addr = ":9090"

[logging]
level = "info"       # debug, info, warn, error
format = "json"      # text or json
output = "stdout"    # stdout, stderr, or file path
```

---

## Error Handling and Peer Penalties

```
┌─────────────────────────────────────────────────────────────────────────┐
│                     PEER PENALTY SYSTEM                                  │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Violation                      │ Action                                │
│  ──────────────────────────────────────────────────────────────────     │
│  Chain ID mismatch              │ Permanent blacklist                   │
│  Protocol version mismatch      │ Permanent blacklist                   │
│  Invalid message format         │ +10 penalty points, disconnect        │
│  Unexpected message on stream   │ +5 penalty points                     │
│  Timeout on expected response   │ +2 penalty points                     │
│  Sending known-bad data         │ +20 penalty points, disconnect        │
│                                                                          │
│  Penalty Thresholds:                                                    │
│  - 25 points: 1 hour ban                                                │
│  - 50 points: 24 hour ban                                               │
│  - 100 points: Permanent blacklist                                      │
│                                                                          │
│  Decay: -1 point per hour (minimum 0)                                   │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Application Integration (ABCI-like)

Blockberry defines an application interface (details TBD):

```go
type Application interface {
    // Transaction validation
    CheckTx(tx []byte) error

    // Block execution (called by consensus)
    BeginBlock(height int64, hash []byte) error
    DeliverTx(tx []byte) error
    EndBlock() error
    Commit() (appHash []byte, error)

    // State queries
    Query(path string, data []byte) ([]byte, error)

    // Consensus message handling
    HandleConsensusMessage(peerID peer.ID, data []byte) error
}
```

---

## Thread Safety

All public APIs are safe for concurrent use:
- Mempool: Protected by RWMutex
- BlockStore: Uses atomic transactions
- StateStore: IAVL handles concurrency
- PeerManager: Protected by RWMutex
- Glueberry: Thread-safe by design

---

## Integration with Raspberry

Blockberry serves as the core framework for the Raspberry blockchain node, integrating with:

- **Leaderberry**: BFT consensus engine (implements `ConsensusHandler` interface)
- **Looseberry**: DAG mempool for validators (implements `DAGMempool` interface)
- **Glueberry**: P2P networking layer (wrapped by blockberry's Node)

### Node Type Configurations

#### Validator Node

```go
// Validator initialization
node := blockberry.New(cfg,
    blockberry.WithGlueberry(glueNode),
    blockberry.WithMempool(looseberry),  // DAG mempool
    blockberry.WithConsensus(leaderberry),
    blockberry.WithApplication(app),
)

// Streams: handshake, pex, transactions (passive), blocksync,
//          blocks, consensus, housekeeping, looseberry-*
```

**Characteristics**:
- Uses Looseberry (DAG mempool)
- Runs Leaderberry (consensus engine)
- TransactionsReactor in passive mode
- All 10 streams active

#### Full Node

```go
// Full node initialization
node := blockberry.New(cfg,
    blockberry.WithGlueberry(glueNode),
    blockberry.WithMempool(simpleMempool),  // Simple mempool
    blockberry.WithApplication(app),
    // No consensus engine
)

// Streams: handshake, pex, transactions (active), blocksync,
//          blocks, housekeeping
```

**Characteristics**:
- Uses simple mempool
- No consensus engine
- TransactionsReactor in active mode
- 7 streams active (no looseberry-*, no consensus)

### Stream Registration

```go
// Validator stream setup (after glueberry handshake)
streamNames := []string{
    "pex",
    "transactions",    // Passive mode
    "blocksync",
    "blocks",
    "consensus",       // Consensus messages
    "housekeeping",
    "looseberry-batches",
    "looseberry-headers",
    "looseberry-sync",
}

// Full node stream setup
streamNames := []string{
    "pex",
    "transactions",    // Active mode
    "blocksync",
    "blocks",
    "housekeeping",
}
```

### Transaction Flow

**Validator Flow**:
```
Client RPC → Looseberry.AddTx()
Full Node Gossip → TransactionsReactor (passive) → Looseberry.AddTx()
Looseberry batches → certificates → ReapCertifiedBatches() → Leaderberry blocks
```

**Full Node Flow**:
```
Client RPC → Simple Mempool
Peer Gossip → TransactionsReactor (active) → Simple Mempool
Mempool → gossip to peers (including validators)
```

### Block Execution Integration

```go
func (n *Node) OnBlockCommit(block *Block) error {
    // 1. Execute block through application
    if err := n.app.ExecuteBlock(block); err != nil {
        return err
    }

    // 2. Save to block store
    if err := n.blockStore.SaveBlock(block); err != nil {
        return err
    }

    // 3. Update mempool
    if dagMempool, ok := n.mempool.(DAGMempool); ok {
        // Validator: notify Looseberry for GC
        dagMempool.NotifyCommitted(block.Header.DAGRound)
    } else {
        // Full node: remove committed txs
        txHashes := extractTxHashes(block)
        n.mempool.RemoveTxs(txHashes)
    }

    return nil
}
```

---

## Package Structure

```
github.com/blockberries/blockberry/
├── node/                 # Main Node type
│   ├── node.go
│   ├── lifecycle.go
│   └── options.go
├── mempool/              # Mempool interface and implementations
│   ├── mempool.go            # Interface
│   ├── simple_mempool.go     # Hash-based implementation
│   ├── priority_mempool.go   # Priority-based with heap ordering
│   └── ttl_mempool.go        # TTL with automatic expiration
├── blockstore/           # Block storage
│   ├── store.go          # Interface
│   ├── leveldb.go        # LevelDB implementation
│   └── store_test.go
├── statestore/           # IAVL state storage with ICS23 proofs
│   ├── store.go          # Interface + IAVL wrapper
│   ├── iavl_store.go     # IAVL implementation
│   └── store_test.go
├── pex/                  # Peer exchange
│   ├── reactor.go
│   ├── address_book.go
│   └── reactor_test.go
├── sync/                 # Block synchronization
│   ├── reactor.go
│   └── reactor_test.go
├── handlers/             # Message handlers
│   ├── handshake.go
│   ├── transactions.go
│   ├── blocks.go
│   └── housekeeping.go
├── p2p/                  # P2P abstractions over glueberry
│   ├── peer_manager.go
│   ├── peer_state.go     # LRU-based peer state tracking
│   ├── rate_limiter.go   # Token bucket rate limiting
│   └── scoring.go
├── metrics/              # Prometheus metrics
│   ├── metrics.go        # Interface
│   ├── prometheus.go     # Prometheus implementation
│   └── nop.go            # No-op implementation
├── logging/              # Structured logging
│   └── logger.go         # slog wrapper with attribute constructors
├── types/                # Common types
│   ├── block.go
│   ├── tx.go
│   ├── errors.go
│   └── validation.go     # Input validation functions
├── config/               # Configuration
│   ├── config.go         # Includes MetricsConfig, LoggingConfig
│   └── defaults.go
├── schema/               # Generated cramberry code
│   └── blockberry.go     # DO NOT EDIT
├── testing/              # Integration test helpers
└── examples/             # Example implementations
    ├── simple_node/
    ├── custom_mempool/
    └── mock_consensus/
```
