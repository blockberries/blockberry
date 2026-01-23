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
│  │ Coordinator │  │  Interface  │  │  Interface  │  │   (IAVL-based)      │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │ PeerManager │  │     PEX     │  │  BlockSync  │  │      Handlers       │ │
│  │  & Scoring  │  │   Reactor   │  │   Reactor   │  │  (Stream-specific)  │ │
│  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────────────┘ │
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

## Module Dependencies

```
github.com/blockberries/blockberry
    ├── github.com/blockberries/glueberry     (P2P networking)
    ├── github.com/blockberries/cramberry     (Serialization)
    ├── github.com/cosmos/iavl                (Merkleized KV store)
    └── (storage backend: leveldb/badgerdb)
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

The mempool manages pending transactions awaiting inclusion in blocks.

#### Interface

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
    // Uses merkle proof for O(log n) lookup.
    HasTx(hash []byte) bool

    // GetTx retrieves a transaction by hash.
    GetTx(hash []byte) ([]byte, error)

    // Size returns the number of transactions.
    Size() int

    // SizeBytes returns total size in bytes.
    SizeBytes() int64

    // RootHash returns the current merkle root of all transactions.
    RootHash() []byte

    // Flush removes all transactions.
    Flush()
}
```

#### Merkleized In-Memory Implementation

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         MEMPOOL STRUCTURE                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│                         ┌──────────────┐                                │
│                         │  Merkle Root │ ◀── Updated on every AddTx     │
│                         └──────┬───────┘                                │
│                                │                                         │
│                    ┌───────────┴───────────┐                            │
│                    │                       │                             │
│               ┌────┴────┐             ┌────┴────┐                       │
│               │  Hash   │             │  Hash   │                       │
│               └────┬────┘             └────┬────┘                       │
│                    │                       │                             │
│            ┌───────┴───────┐       ┌───────┴───────┐                    │
│            │               │       │               │                     │
│       ┌────┴────┐    ┌────┴────┐  ┌────┴────┐    ┌────┴────┐           │
│       │  TxHash │    │  TxHash │  │  TxHash │    │  TxHash │           │
│       └─────────┘    └─────────┘  └─────────┘    └─────────┘           │
│                                                                          │
│   Secondary Index: map[txHash][]byte for O(1) data retrieval            │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key Features:**
- Merkle tree for fast existence proofs (O(log n) verification)
- Root hash computed on every modification
- Used for efficient transaction sync (compare roots, request missing)
- Configurable size limits (count and bytes)

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

    // Exchange tracking (avoid redundant sends)
    TxsSent       *bloom.BloomFilter  // or map[string]bool
    TxsReceived   *bloom.BloomFilter
    BlocksSent    map[int64]bool      // height → sent
    BlocksReceived map[int64]bool

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

#### Per-Peer Tracking Rationale

When gossiping transactions:
1. Node A sends `TransactionsResponse` with tx hashes it has
2. Node B checks which it already has (merkle proof from mempool)
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

## Package Structure

```
github.com/blockberries/blockberry/
├── node/                 # Main Node type
│   ├── node.go
│   ├── lifecycle.go
│   └── options.go
├── mempool/              # Mempool interface and implementations
│   ├── mempool.go        # Interface
│   ├── merkle_mempool.go # Merkleized implementation
│   └── mempool_test.go
├── blockstore/           # Block storage
│   ├── store.go          # Interface
│   ├── leveldb.go        # LevelDB implementation
│   └── store_test.go
├── statestore/           # IAVL state storage
│   ├── store.go          # Interface + IAVL wrapper
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
│   ├── peer_state.go
│   └── scoring.go
├── types/                # Common types
│   ├── block.go
│   ├── tx.go
│   └── errors.go
├── config/               # Configuration
│   ├── config.go
│   └── defaults.go
├── schema/               # Generated cramberry code
│   └── blockberry.go     # DO NOT EDIT
└── cmd/                  # Example/reference implementations
    └── blockberryd/
```
