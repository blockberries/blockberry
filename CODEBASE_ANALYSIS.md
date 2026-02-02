# Blockberry Codebase Analysis

**Generated:** 2026-02-02
**Analyzed Commit:** 2368b5f (chore: prepare for docs rewrite)

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Architecture Overview](#architecture-overview)
3. [Package-by-Package Analysis](#package-by-package-analysis)
4. [Code Patterns and Conventions](#code-patterns-and-conventions)
5. [Security Considerations](#security-considerations)
6. [Performance Optimizations](#performance-optimizations)
7. [Testing Strategy](#testing-strategy)
8. [Dependencies and Integration](#dependencies-and-integration)

---

## Executive Summary

Blockberry is a Go-based blockchain node framework built on top of two foundational libraries:

- **Glueberry**: Provides secure P2P networking with encrypted streams
- **Cramberry**: Offers high-performance binary serialization with polymorphic message support

The codebase demonstrates professional software engineering practices with:

- **Fail-closed security** by default (rejecting operations unless explicitly allowed)
- **Thread-safe concurrent design** using mutexes and atomic operations
- **Defensive programming** with deep copies to prevent mutation bugs
- **Pluggable architecture** with clear interface boundaries
- **Comprehensive configuration** via TOML files with validation

The project follows standard Go project layout with clear separation between:

- `/pkg` - Public API packages (safe to import externally)
- `/internal` - Private implementation (not importable externally)
- `/cmd` - CLI application entry points
- `/examples` - Example applications
- `/test` - Integration tests

---

## Architecture Overview

### Core Design Principles

1. **Separation of Concerns**: Blockberry is NOT a consensus engine. It provides the building blocks (networking, storage, mempool) that applications use to implement their own consensus.

2. **Application Binary Interface (ABI)**: Applications implement the `Application` interface to define custom blockchain logic. The framework handles all infrastructure concerns.

3. **Stream-Based Communication**: Seven encrypted P2P streams handle different message types:
   - `handshake` - Connection setup (unencrypted, built into glueberry)
   - `pex` - Peer exchange and discovery
   - `transactions` - Transaction gossiping
   - `blocksync` - Batch sync of historical blocks
   - `blocks` - Real-time block propagation
   - `consensus` - App-defined consensus messages (raw bytes)
   - `housekeeping` - Latency probes, firewall detection

4. **Node Roles**: Five distinct node roles with different capabilities:
   - `Validator` - Participates in consensus, stores full state
   - `Full` - Full node, doesn't participate in consensus
   - `Seed` - Provides peer discovery only
   - `Light` - Light client with minimal storage
   - `Archive` - Full node with complete historical data

### Component Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                        Application Layer                     │
│  (Implements Application interface: CheckTx, ExecuteTx, etc)│
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────┼─────────────────────────────────┐
│                         Node (pkg/node)                       │
│  - Lifecycle management                                       │
│  - Component coordination                                     │
│  - Event loop orchestration                                   │
└─────────────────────────────┬─────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼──────┐    ┌────────▼────────┐   ┌───────▼──────┐
│   Mempool    │    │  Block Storage  │   │ State Storage│
│  (pluggable) │    │   (pluggable)   │   │    (IAVL)    │
└──────────────┘    └─────────────────┘   └──────────────┘
        │                     │                     │
┌───────▼──────────────────────▼─────────────────────▼──────────┐
│              P2P Network Layer (internal/p2p)                  │
│  - Glueberry wrapper                                           │
│  - Peer management                                             │
│  - Stream routing                                              │
└────────────────────────────┬───────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼──────┐   ┌─────────▼────────┐  ┌──────▼────────┐
│  Handshake   │   │  PEX (Peer       │  │  Block Sync   │
│   Handler    │   │   Exchange)       │  │   Reactor     │
└──────────────┘   └──────────────────┘  └───────────────┘
        │                    │                    │
┌───────▼──────┐   ┌─────────▼────────┐  ┌──────▼────────┐
│Transactions  │   │     Blocks        │  │  Housekeeping │
│   Reactor    │   │    Reactor        │  │    Reactor    │
└──────────────┘   └──────────────────┘  └───────────────┘

```text

---

## Package-by-Package Analysis

### 1. pkg/abi - Application Binary Interface

**Location:** `/Volumes/Tendermint/stealth/blockberry/pkg/abi/`

**Purpose:** Defines the contract between the blockchain framework and application logic.

#### Key Components

##### Application Interface

```go
type Application interface {
    Info() ApplicationInfo
    InitChain(genesis *Genesis) error
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult
    BeginBlock(ctx context.Context, header *BlockHeader) error
    ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult
    EndBlock(ctx context.Context) *EndBlockResult
    Commit(ctx context.Context) *CommitResult
    Query(ctx context.Context, req *QueryRequest) *QueryResponse
}

```text

**Design Patterns:**

- **Block Execution Lifecycle**: Fixed order ensures deterministic execution
  1. `BeginBlock` - Initialize block processing
  2. `ExecuteTx` - Execute each transaction in order
  3. `EndBlock` - Finalize block (return validator updates)
  4. `Commit` - Persist state changes, return app hash

- **Fail-Closed Base Implementation**: `BaseApplication` rejects all operations by default

  ```go
  func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
      return &TxCheckResult{
          Code:  CodeNotAuthorized,
          Error: errors.New("CheckTx not implemented"),
      }
  }

  ```text

##### Transaction Processing
- **Transaction Structure**: Opaque `Data` field allows any encoding

  ```go
  type Transaction struct {
      Hash     []byte  // SHA-256 of Data
      Data     []byte  // Opaque to framework
      Sender   []byte  // Optional extracted sender
      Nonce    uint64  // Optional sequence number
      GasLimit uint64  // Optional computation limit
      GasPrice uint64  // Optional price per unit
      Priority int64   // Derived priority for ordering
  }

  ```text

- **Result Codes**: Framework codes (0-99), application codes (100+)
  - `CodeOK = 0` - Success
  - `CodeInvalidTx = 2` - Malformed transaction
  - `CodeNotAuthorized = 6` - Operation not permitted
  - `CodeAppErrorStart = 100` - Application-specific errors begin here

##### Validator Set Management
- **SimpleValidatorSet**: Basic round-robin proposer selection
- **Byzantine Fault Tolerance**: `F() = (n-1)/3`, `Quorum() = 2F+1`
- **Deep Copy Pattern**: All public methods return deep copies to prevent mutation

  ```go
  func (vs *SimpleValidatorSet) GetByIndex(index uint16) *Validator {
      if int(index) >= len(vs.validators) {
          return nil
      }
      v := vs.validators[index]
      return &Validator{
          Address:     append([]byte(nil), v.Address...),
          PublicKey:   append([]byte(nil), v.PublicKey...),
          VotingPower: v.VotingPower,
          Index:       v.Index,
      }
  }

  ```text

##### Extended Interfaces
- **SnapshotApplication**: Adds state sync capabilities
  - `ListSnapshots()` - Return available snapshots
  - `LoadSnapshotChunk()` - Serve chunks to syncing peers
  - `OfferSnapshot()` - Validate received snapshot offers
  - `ApplySnapshotChunk()` - Apply chunks during sync

- **Component Lifecycle**: Standard interface for all components

  ```go
  type Component interface {
      Start() error      // Idempotent start
      Stop() error       // Idempotent stop (safe even if not started)
      IsRunning() bool   // Current state
  }

  ```text

##### Resource Limits
- **Comprehensive Limit System**: Prevents resource exhaustion

  ```go
  type ResourceLimits struct {
      MaxTxSize          int64  // 1 MB default
      MaxBlockSize       int64  // 21 MB default
      MaxMsgSize         int64  // 10 MB default
      MaxPeers           int    // 50 default
      MaxSubscribers     int    // 1000 default
  }

  ```text

- **Eclipse Attack Mitigation**: Protects against network isolation attacks
  - Limits peers per subnet
  - Tracks peer source diversity
  - Requires minimum outbound connection percentage

#### Security Features

1. **Fail-Closed Defaults**: All operations rejected unless explicitly implemented
2. **Result Code System**: Separates framework errors from application errors
3. **Event System**: Type-safe event emission with indexed attributes
4. **Defensive Copying**: All byte slices are copied to prevent mutation
5. **Context Propagation**: All methods accept `context.Context` for cancellation

---

### 2. pkg/blockstore - Block Storage

**Location:** `/Volumes/Tendermint/stealth/blockberry/pkg/blockstore/`

**Purpose:** Pluggable block persistence with multiple backend implementations.

#### Interface Definition

```go
type BlockStore interface {
    SaveBlock(height int64, hash []byte, data []byte) error
    LoadBlock(height int64) (hash []byte, data []byte, err error)
    LoadBlockByHash(hash []byte) (height int64, data []byte, err error)
    HasBlock(height int64) bool
    Height() int64  // Latest height
    Base() int64    // Earliest available height (after pruning)
    Close() error
}

```text

#### Implementations

##### 1. LevelDBBlockStore

**Key Design Decisions:**

- **Dual Indexing**: Height→Hash and Hash→Data for efficient lookups
- **Metadata Caching**: In-memory height/base tracking with disk persistence
- **Atomic Writes**: Uses LevelDB batches for consistency
- **Key Encoding**: Big-endian encoding for lexicographic ordering

**Key Prefixes:**

```go
var (
    prefixHeight  = []byte("H:")  // H:<height> → hash
    prefixBlock   = []byte("B:")  // B:<hash> → block data
    keyMetaHeight = []byte("M:height")
    keyMetaBase   = []byte("M:base")
)

```text

**Certificate Storage Integration:**

```go
// For Looseberry DAG consensus
var (
    prefixCert       = []byte("C:")   // C:<digest> → Certificate
    prefixCertRound  = []byte("CR:")  // CR:<round>:<validator_idx> → digest
    prefixCertHeight = []byte("CH:")  // CH:<height>:<validator_idx> → digest
    prefixBatch      = []byte("CB:")  // CB:<digest> → Batch
)

```text

**Thread Safety:**

- `sync.RWMutex` protects all state
- Read operations use `RLock()` for concurrency
- Write operations use exclusive `Lock()`

**Pruning Strategy:**

```go
func (s *LevelDBBlockStore) Prune(beforeHeight int64) (*PruneResult, error) {
    // 1. Validate inputs
    // 2. Check not already pruning (prevents concurrent prune)
    // 3. Iterate blocks from base to beforeHeight
    // 4. Check ShouldKeep for each block (preserves checkpoints)
    // 5. Delete in batches (1000 blocks per batch)
    // 6. Update base metadata
    // 7. Return statistics (count, bytes freed, duration)
}

```text

##### 2. BadgerDBBlockStore

**Advantages over LevelDB:**

- Optimized for SSDs with LSM tree design
- Built-in compression (Snappy)
- Value log GC for space reclamation
- Better write performance for certain workloads

**Configuration Options:**

```go
type BadgerDBOptions struct {
    SyncWrites              bool   // true for durability
    Compression             bool   // true for Snappy compression
    ValueLogFileSize        int64  // 1GB default
    MemTableSize            int64  // 64MB default
    NumMemtables            int    // 5 default
    NumLevelZeroTables      int    // 5 default
    NumLevelZeroTablesStall int    // 15 default (triggers write stalling)
}

```text

**Garbage Collection:**

```go
func (s *BadgerDBBlockStore) Compact() error {
    // Run value log GC with 0.5 discard ratio
    // Rewrites files with 50%+ garbage
    for {
        err := s.db.RunValueLogGC(0.5)
        if err == badger.ErrNoRewrite {
            break  // No more GC needed
        }
        if err != nil {
            return err
        }
    }
    return nil
}

```text

##### 3. MemoryBlockStore

**Purpose:** Testing and development

**Features:**

- In-memory maps for fast access
- No persistence
- Full interface implementation including pruning
- Certificate storage support

**Deep Copy Pattern:**

```go
func (m *MemoryBlockStore) SaveBlock(height int64, hash, data []byte) error {
    // Store defensive copies to prevent external mutation
    m.blocks[height] = blockEntry{
        hash: append([]byte(nil), hash...),
        data: append([]byte(nil), data...),
    }
    return nil
}

```text

##### 4. Additional Implementations

- **NoopBlockStore**: Discards all blocks (for testing)
- **HeaderOnlyStore**: Stores only block headers (light clients)
- **PruningBlockStore**: Wrapper that adds automatic background pruning

#### Certificate Storage (Looseberry Integration)

**Purpose:** Store DAG certificates and transaction batches for Looseberry consensus.

**Interface:**

```go
type CertificateStore interface {
    SaveCertificate(cert *loosetypes.Certificate) error
    GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error)
    GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error)
    GetCertificatesForHeight(height int64) ([]*loosetypes.Certificate, error)
    SetCertificateBlockHeight(digest loosetypes.Hash, height int64) error
    SaveBatch(batch *loosetypes.Batch) error
    GetBatch(digest loosetypes.Hash) (*loosetypes.Batch, error)
}

```text

**Triple Indexing:**

1. **Primary:** `C:<digest>` → Certificate data (Cramberry serialized)
2. **Round Index:** `CR:<round>:<validator_idx>` → digest (for DAG operations)
3. **Height Index:** `CH:<height>:<validator_idx>` → digest (for block queries)

**Defensive Copying:**

```go
func (s *LevelDBBlockStore) SaveCertificate(cert *loosetypes.Certificate) error {
    // Create defensive copy of digest
    digest := cert.Digest()
    digestCopy := make([]byte, len(digest))
    copy(digestCopy, digest[:])

    // Marshal with cramberry
    data, err := cramberry.Marshal(cert)

    // Atomic batch write with all indexes
    batch.Put(certKey, data)
    batch.Put(roundKey, digestCopy)

    return s.db.Write(batch, &opt.WriteOptions{Sync: true})
}

```text

#### Pruning System

**Strategies:**

```go
const (
    PruneNothing    PruneStrategy = "nothing"     // Archive node
    PruneEverything PruneStrategy = "everything"  // Aggressive
    PruneDefault    PruneStrategy = "default"     // Balanced
    PruneCustom     PruneStrategy = "custom"      // User-defined
)

```text

**Configuration:**

```go
type PruneConfig struct {
    Strategy   PruneStrategy
    KeepRecent int64          // Always keep last N blocks
    KeepEvery  int64          // Keep every Nth block as checkpoint
    Interval   time.Duration  // Auto-prune frequency
}

```text

**Background Pruner:**

- Runs in separate goroutine
- Configurable interval
- Callbacks for success/error
- Calculates prune target based on current height

**Example:**

```go
cfg := &PruneConfig{
    Strategy:   PruneDefault,
    KeepRecent: 1000,    // Keep last 1000 blocks
    KeepEvery:  10000,   // Keep blocks 10000, 20000, 30000...
    Interval:   time.Hour,
}

pruner := NewBackgroundPruner(store, cfg)
pruner.SetOnPrune(func(result *PruneResult) {
    log.Info("Pruned %d blocks, freed %d bytes in %v",
        result.PrunedCount, result.BytesFreed, result.Duration)
})
pruner.Start()

```text

#### Concurrency Patterns

1. **Read-Write Mutex**: Allows multiple concurrent readers
2. **Atomic Batches**: Multiple operations succeed or fail together
3. **Defensive Copying**: All byte slices copied to prevent data races
4. **Prune Guard**: `pruning` flag prevents concurrent pruning operations

---

### 3. pkg/config - Configuration Management

**Location:** `/Volumes/Tendermint/stealth/blockberry/pkg/config/`

**Purpose:** TOML-based configuration with validation and defaults.

#### Configuration Structure

```go
type Config struct {
    Node         NodeConfig
    Network      NetworkConfig
    Role         NodeRole
    Handlers     HandlersConfig
    PEX          PEXConfig
    StateSync    StateSyncConfig
    Mempool      MempoolConfig
    BlockStore   BlockStoreConfig
    StateStore   StateStoreConfig
    Housekeeping HousekeepingConfig
    Metrics      MetricsConfig
    Logging      LoggingConfig
    Limits       LimitsConfig
}

```text

#### Key Design Patterns

##### 1. Duration Wrapper

```go
type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
    duration, err := time.ParseDuration(string(text))
    if err != nil {
        return err
    }
    *d = Duration(duration)
    return nil
}

```text

Enables TOML syntax: `timeout = "30s"` or `interval = "5m"`

##### 2. Cascading Validation

```go
func (c *Config) Validate() error {
    if err := c.Node.Validate(); err != nil {
        return fmt.Errorf("node config: %w", err)
    }
    if err := c.Network.Validate(); err != nil {
        return fmt.Errorf("network config: %w", err)
    }
    // ... validate all subsections
    return nil
}

```text

Each subsection validates independently, errors include context path.

##### 3. Sensible Defaults

```go
func DefaultConfig() *Config {
    return &Config{
        Node: NodeConfig{
            ChainID:         "blockberry-testnet-1",
            ProtocolVersion: 1,
            PrivateKeyPath:  "node_key.json",
        },
        Network: NetworkConfig{
            ListenAddrs:      []string{"/ip4/0.0.0.0/tcp/26656"},
            MaxInboundPeers:  40,
            MaxOutboundPeers: 10,
            HandshakeTimeout: Duration(30 * time.Second),
            DialTimeout:      Duration(3 * time.Second),
        },
        // ... more defaults
    }
}

```text

##### 4. Sentinel Errors

```go
var (
    ErrEmptyChainID           = errors.New("chain_id cannot be empty")
    ErrInvalidProtocolVersion = errors.New("protocol_version must be positive")
    ErrNoListenAddrs          = errors.New("at least one listen address required")
    // ... 40+ specific error types
)

```text

Enables programmatic error handling with `errors.Is()`.

#### Configuration Sections

##### Node Configuration

```toml
[node]
chain_id = "blockberry-testnet-1"
protocol_version = 1
private_key_path = "node_key.json"

```text

##### Network Configuration

```toml
[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
dial_timeout = "3s"
address_book_path = "addrbook.json"

[network.seeds]
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/QmABC123...",
    "/ip4/seed2.example.com/tcp/26656/p2p/QmDEF456..."
]

```text

##### Mempool Configuration

```toml
[mempool]
type = "simple"  # "simple", "priority", "ttl", "looseberry"
max_txs = 5000
max_bytes = 1073741824  # 1GB
cache_size = 10000

# TTL mempool specific
ttl = "1h"
cleanup_interval = "5m"

```text

##### Handler Configuration

```toml
[handlers.transactions]
request_interval = "5s"
batch_size = 100
max_pending = 1000
max_pending_age = "60s"

[handlers.blocks]
max_block_size = 22020096  # ~21MB

[handlers.sync]
sync_interval = "5s"
batch_size = 100
max_pending_batches = 10

```text

##### PEX (Peer Exchange) Configuration

```toml
[pex]
enabled = true
request_interval = "30s"
max_addresses_per_response = 100
max_total_addresses = 1000

```text

##### State Sync Configuration

```toml
[statesync]
enabled = false
trust_height = 1000000
trust_hash = "ABC123DEF456..."
discovery_interval = "10s"
chunk_request_timeout = "30s"
max_chunk_retries = 3
snapshot_path = "data/snapshots"

```text

##### Metrics Configuration

```toml
[metrics]
enabled = false
namespace = "blockberry"
listen_addr = ":9090"

```text

##### Logging Configuration

```toml
[logging]
level = "info"    # "debug", "info", "warn", "error"
format = "text"   # "text" or "json"
output = "stderr" # "stdout", "stderr", or file path

```text

##### Resource Limits

```toml
[limits]
max_tx_size = 1048576        # 1 MB
max_block_size = 22020096    # 21 MB
max_block_txs = 10000
max_msg_size = 10485760      # 10 MB
max_subscribers = 1000
max_subscribers_per_query = 100

```text

#### Loading and Validation

```go
// Load config from file
cfg, err := config.LoadConfig("config.toml")
if err != nil {
    return err
}

// Ensure data directories exist
if err := cfg.EnsureDataDirs(); err != nil {
    return err
}

// Write config to file (e.g., for init command)
if err := config.WriteConfigFile("config.toml", cfg); err != nil {
    return err
}

```text

#### Node Roles

```go
type NodeRole string

const (
    RoleValidator NodeRole = "validator"  // Participates in consensus
    RoleFull      NodeRole = "full"       // Full node, no consensus
    RoleSeed      NodeRole = "seed"       // Peer discovery only
    RoleLight     NodeRole = "light"      // Light client
    RoleArchive   NodeRole = "archive"    // Full historical data
)

func (r NodeRole) IsValid() bool {
    switch r {
    case RoleValidator, RoleFull, RoleSeed, RoleLight, RoleArchive:
        return true
    default:
        return false
    }
}

```text

---

### 4. pkg/mempool - Transaction Mempool

**Location:** `/Volumes/Tendermint/stealth/blockberry/pkg/mempool/`

**Purpose:** Pluggable transaction pool with multiple implementations.

#### Interface Definition

```go
type Mempool interface {
    AddTx(tx []byte) error
    RemoveTxs(hashes [][]byte)
    ReapTxs(maxBytes int64) [][]byte
    GetTx(hash []byte) ([]byte, error)
    Size() int
    Flush()
}

```text

#### Implementations

##### 1. SimpleMempool

**Design:**

- Hash-based storage: `map[string][]byte`
- Insertion order preservation: `[][]byte` slice
- No prioritization

**Key Operations:**

```go
func (m *SimpleMempool) AddTx(tx []byte) error {
    // 1. Check size limits
    if m.maxTxSize > 0 && int64(len(tx)) > m.maxTxSize {
        return types.ErrTxTooLarge
    }

    // 2. Check for duplicates
    hash := types.HashTx(tx)
    if _, exists := m.txs[string(hash)]; exists {
        return types.ErrTxAlreadyExists
    }

    // 3. Validate transaction (fail-closed if no validator set)
    validator := m.validator
    if validator == nil {
        validator = DefaultTxValidator  // Rejects all
    }
    if err := validator(tx); err != nil {
        return err
    }

    // 4. Check capacity
    if len(m.txs) >= m.maxTxs || m.sizeBytes + txSize > m.maxBytes {
        return types.ErrMempoolFull
    }

    // 5. Store defensive copy
    txCopy := append([]byte(nil), tx...)
    m.txs[string(hash)] = txCopy
    m.order = append(m.order, hash)
    m.sizeBytes += txSize

    return nil
}

```text

**Reaping (for block building):**

```go
func (m *SimpleMempool) ReapTxs(maxBytes int64) [][]byte {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make([][]byte, 0)
    var totalBytes int64

    for _, hash := range m.order {
        tx := m.txs[string(hash)]
        txSize := int64(len(tx))

        if maxBytes > 0 && totalBytes + txSize > maxBytes {
            break
        }

        // Return defensive copy
        result = append(result, append([]byte(nil), tx...))
        totalBytes += txSize
    }

    return result
}

```text

##### 2. PriorityMempool

**Features:**

- Priority-based ordering
- Sender-based deduplication (keeps highest nonce per sender)
- Gas price sorting

**Priority Calculation:**

```go
func calculatePriority(tx *types.Transaction) int64 {
    // Priority = GasPrice * GasLimit
    // Higher priority = included first
    return int64(tx.GasPrice * tx.GasLimit)
}

```text

**Data Structures:**

```go
type PriorityMempool struct {
    txs          map[string]*types.Transaction  // hash → tx
    bySender     map[string][]*types.Transaction // sender → txs (sorted by nonce)
    priorityHeap *TxHeap                        // min-heap by priority

    maxTxs    int
    maxBytes  int64
    sizeBytes int64

    mu sync.RWMutex
}

```text

**Heap Operations:**

```go
type TxHeap []*types.Transaction

func (h TxHeap) Less(i, j int) bool {
    return h[i].Priority > h[j].Priority  // Max-heap (highest priority first)
}

func (m *PriorityMempool) ReapTxs(maxBytes int64) [][]byte {
    // Return transactions in priority order
    // Maintain nonce ordering for same-sender txs
}

```text

##### 3. TTLMempool

**Features:**

- Time-based expiration
- Automatic cleanup of old transactions
- Background cleanup goroutine

**Structure:**

```go
type TTLMempool struct {
    SimpleMempool

    ttl             time.Duration
    cleanupInterval time.Duration
    expiryTimes     map[string]time.Time  // hash → expiry time

    stopCh chan struct{}
    wg     sync.WaitGroup
}

```text

**Cleanup Loop:**

```go
func (m *TTLMempool) cleanupLoop() {
    ticker := time.NewTicker(m.cleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-m.stopCh:
            return
        case <-ticker.C:
            m.cleanupExpired()
        }
    }
}

func (m *TTLMempool) cleanupExpired() {
    now := time.Now()

    m.mu.Lock()
    defer m.mu.Unlock()

    var expiredHashes [][]byte
    for hashStr, expiryTime := range m.expiryTimes {
        if now.After(expiryTime) {
            expiredHashes = append(expiredHashes, []byte(hashStr))
            delete(m.expiryTimes, hashStr)
        }
    }

    if len(expiredHashes) > 0 {
        m.RemoveTxs(expiredHashes)
    }
}

```text

##### 4. Looseberry Mempool (DAG)

**Purpose:** Adapter for Looseberry DAG mempool.

**Features:**

- Integrates Looseberry's DAG-based mempool
- Supports high-throughput transaction ordering
- Certificate-based consensus integration

**Interface Mapping:**

```go
type LooseberryMempool struct {
    dag *looseberry.DAGMempool

    // Blockberry → Looseberry translation
}

func (m *LooseberryMempool) AddTx(tx []byte) error {
    // Convert to Looseberry transaction format
    looseTx := looseberry.NewTransaction(tx)
    return m.dag.AddTransaction(looseTx)
}

```text

#### Transaction Validation

**Default Validator (Fail-Closed):**

```go
var DefaultTxValidator TxValidator = func(tx []byte) error {
    return errors.New("no transaction validator configured")
}

```text

**Custom Validator:**

```go
type TxValidator func(tx []byte) error

mempool.SetValidator(func(tx []byte) error {
    // Parse transaction
    parsed, err := ParseTx(tx)
    if err != nil {
        return err
    }

    // Validate signature
    if !parsed.VerifySignature() {
        return errors.New("invalid signature")
    }

    // Check balance
    if balance < parsed.Amount {
        return errors.New("insufficient funds")
    }

    return nil
})

```text

#### Concurrency Patterns

1. **Read-Write Mutex**: `sync.RWMutex` for all operations
2. **Defensive Copying**: All transactions copied on entry/exit
3. **Atomic Operations**: No partial state updates
4. **Lock-Free Reads**: Multiple concurrent readers allowed

#### Cache Management

**Recent Transaction Cache:**

```go
type recentTxCache struct {
    hashes map[string]struct{}
    order  []string
    maxSize int
    mu     sync.RWMutex
}

func (c *recentTxCache) Add(hash []byte) {
    c.mu.Lock()
    defer c.mu.Unlock()

    hashStr := string(hash)
    if _, exists := c.hashes[hashStr]; exists {
        return
    }

    c.hashes[hashStr] = struct{}{}
    c.order = append(c.order, hashStr)

    // Evict oldest if over capacity
    if len(c.order) > c.maxSize {
        oldest := c.order[0]
        delete(c.hashes, oldest)
        c.order = c.order[1:]
    }
}

```text

Purpose: Prevents re-processing of recently seen transactions.

---

### 5. pkg/node - Node Lifecycle Management

**Location:** `/Volumes/Tendermint/stealth/blockberry/pkg/node/`

**Purpose:** Main coordinator for all node components and lifecycle management.

#### Node Structure

```go
type Node struct {
    // Configuration
    cfg        *config.Config
    privateKey ed25519.PrivateKey
    nodeID     string
    role       types.NodeRole

    // Core network
    glueNode *glueberry.Node
    network  *p2p.Network

    // Stores
    blockStore blockstore.BlockStore
    mempool    mempool.Mempool

    // Handlers (Reactors)
    handshakeHandler    *handlers.HandshakeHandler
    transactionsReactor *handlers.TransactionsReactor
    blocksReactor       *handlers.BlockReactor
    consensusReactor    *handlers.ConsensusReactor
    housekeepingReactor *handlers.HousekeepingReactor
    pexReactor          *pex.Reactor
    syncReactor         *bsync.SyncReactor

    // Callbacks
    callbacks *types.NodeCallbacks

    // Lifecycle
    started  bool
    stopping atomic.Bool
    stopCh   chan struct{}
    wg       sync.WaitGroup
    mu       sync.RWMutex
}

```text

#### Initialization Pattern

**Builder Pattern with Functional Options:**

```go
type Option func(*Node)

func WithMempool(mp mempool.Mempool) Option {
    return func(n *Node) {
        n.mempool = mp
    }
}

func WithBlockStore(bs blockstore.BlockStore) Option {
    return func(n *Node) {
        n.blockStore = bs
    }
}

func WithConsensusHandler(ch handlers.ConsensusHandler) Option {
    return func(n *Node) {
        if n.consensusReactor != nil {
            n.consensusReactor.SetHandler(ch)
        }
    }
}

// Usage:
node, err := node.NewNode(cfg,
    node.WithMempool(customMempool),
    node.WithBlockStore(customStore),
    node.WithConsensusHandler(consensusHandler),
    node.WithBlockValidator(validator),
)

```text

**Advantages:**

- Flexible configuration
- Optional components
- Clear dependencies
- Type-safe

#### Lifecycle Management

**Start Sequence:**

```go
func (n *Node) Start() error {
    n.mu.Lock()
    defer n.mu.Unlock()

    if n.started {
        return errors.New("node already started")
    }

    // 1. Start glueberry network
    if err := n.glueNode.Start(); err != nil {
        return fmt.Errorf("starting network: %w", err)
    }

    // 2. Start all reactors in dependency order
    components := []struct {
        name string
        comp interface{ Start() error }
    }{
        {"handshake", n.handshakeHandler},
        {"pex", n.pexReactor},
        {"housekeeping", n.housekeepingReactor},
        {"transactions", n.transactionsReactor},
        {"blocks", n.blocksReactor},
        {"consensus", n.consensusReactor},
        {"sync", n.syncReactor},
    }

    for _, c := range components {
        if c.comp != nil {
            if err := c.comp.Start(); err != nil {
                // Cleanup: stop already-started components
                n.stopComponents()
                return fmt.Errorf("starting %s: %w", c.name, err)
            }
        }
    }

    // 3. Start event loop
    n.wg.Add(1)
    go n.eventLoop()

    n.started = true
    return nil
}

```text

**Stop Sequence:**

```go
func (n *Node) Stop() error {
    n.mu.Lock()
    if !n.started {
        n.mu.Unlock()
        return nil
    }

    // Signal shutdown
    n.stopping.Store(true)
    close(n.stopCh)
    n.mu.Unlock()

    // Wait for event loop with timeout
    done := make(chan struct{})
    go func() {
        n.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // Clean shutdown
    case <-time.After(DefaultShutdownTimeout):
        // Timeout - force stop
        log.Warn("Event loop shutdown timeout, forcing stop")
    }

    // Stop all components
    n.stopComponents()

    // Close stores
    if n.blockStore != nil {
        n.blockStore.Close()
    }
    if n.mempool != nil {
        n.mempool.Flush()
    }

    // Stop network last
    if n.glueNode != nil {
        n.glueNode.Stop()
    }

    n.mu.Lock()
    n.started = false
    n.mu.Unlock()

    return nil
}

```text

**Graceful Shutdown:**

- Signal all goroutines via `stopCh`
- Wait for event loop with timeout
- Stop components in reverse dependency order
- Close resources (stores, network)

#### Event Loop

**Purpose:** Central message routing from glueberry to handlers.

```go
func (n *Node) eventLoop() {
    defer n.wg.Done()

    // Get event channels from glueberry
    events := n.glueNode.Events()
    messages := n.glueNode.Messages()

    for {
        select {
        case <-n.stopCh:
            return

        case event := <-events:
            n.handleConnectionEvent(event)

        case msg := <-messages:
            n.routeMessage(msg)
        }
    }
}

```text

**Connection Event Handling:**

```go
func (n *Node) handleConnectionEvent(event *glueberry.Event) {
    switch event.State {
    case glueberry.StateConnected:
        // New connection established (pre-handshake)
        n.handshakeHandler.OnConnected(event.PeerID)

    case glueberry.StateEstablished:
        // Handshake complete, encrypted streams ready
        n.peerManager.AddPeer(event.PeerID, event.IsOutbound)
        if n.callbacks != nil && n.callbacks.OnPeerConnected != nil {
            n.callbacks.OnPeerConnected(event.PeerID)
        }

    case glueberry.StateDisconnected:
        // Connection lost
        n.peerManager.RemovePeer(event.PeerID)
        if n.callbacks != nil && n.callbacks.OnPeerDisconnected != nil {
            n.callbacks.OnPeerDisconnected(event.PeerID)
        }
    }
}

```text

**Message Routing:**

```go
func (n *Node) routeMessage(msg *glueberry.Message) {
    switch msg.StreamName {
    case "handshake":
        n.handshakeHandler.HandleMessage(msg)

    case "pex":
        if n.pexReactor != nil {
            n.pexReactor.HandleMessage(msg)
        }

    case "transactions":
        if n.transactionsReactor != nil {
            n.transactionsReactor.HandleMessage(msg)
        }

    case "blocks":
        if n.blocksReactor != nil {
            n.blocksReactor.HandleMessage(msg)
        }

    case "blocksync":
        if n.syncReactor != nil {
            n.syncReactor.HandleMessage(msg)
        }

    case "consensus":
        if n.consensusReactor != nil {
            n.consensusReactor.HandleMessage(msg)
        }

    case "housekeeping":
        if n.housekeepingReactor != nil {
            n.housekeepingReactor.HandleMessage(msg)
        }

    default:
        log.Warn("Unknown stream: %s", msg.StreamName)
    }
}

```text

#### Dependency Injection

**Container Pattern:**

```go
type Container struct {
    components map[string]interface{}
    mu         sync.RWMutex
}

func (c *Container) Register(name string, component interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.components[name] = component
}

func (c *Container) Get(name string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    comp, ok := c.components[name]
    return comp, ok
}

```text

**Usage in Node:**

```go
// Create container
container := container.NewContainer()

// Register components
container.Register(ComponentNetwork, network)
container.Register(ComponentHandshake, handshakeHandler)
container.Register(ComponentPEX, pexReactor)

// Retrieve for use
if comp, ok := container.Get(ComponentNetwork); ok {
    network := comp.(*p2p.Network)
    // use network
}

```text

#### Callbacks System

**Type-Safe Event Notifications:**

```go
type NodeCallbacks struct {
    // Connection events
    OnPeerConnected    func(peerID peer.ID)
    OnPeerDisconnected func(peerID peer.ID)

    // Transaction events
    OnTxReceived func(tx []byte)
    OnTxAdded    func(tx []byte)
    OnTxRejected func(tx []byte, err error)

    // Block events
    OnBlockReceived func(height int64, hash []byte)
    OnBlockApplied  func(height int64, hash []byte)
    OnBlockRejected func(height int64, err error)

    // Sync events
    OnSyncStarted   func(startHeight, targetHeight int64)
    OnSyncProgress  func(currentHeight, targetHeight int64)
    OnSyncCompleted func(finalHeight int64)
}

```text

**Usage:**

```go
node, err := node.NewNode(cfg,
    node.WithCallbacks(&types.NodeCallbacks{
        OnPeerConnected: func(peerID peer.ID) {
            log.Info("Peer connected: %s", peerID)
            metrics.IncrementPeerCount()
        },
        OnBlockApplied: func(height int64, hash []byte) {
            log.Info("Block %d applied: %x", height, hash)
            updateUI(height)
        },
    }),
)

```text

#### Component Dependencies

**Startup Order (matters for dependencies):**

1. Handshake Handler (establishes connections)
2. PEX Reactor (discovers peers)
3. Housekeeping Reactor (maintains connections)
4. Transactions Reactor (depends on mempool)
5. Blocks Reactor (depends on block store)
6. Consensus Reactor (depends on all above)
7. Sync Reactor (depends on block store + validator)

**Shutdown Order (reverse):**

- Stop in reverse order to prevent incomplete operations

---

### 6. internal/handlers - P2P Message Handlers

**Location:** `/Volumes/Tendermint/stealth/blockberry/internal/handlers/`

**Purpose:** Stream-specific message handling for different P2P protocols.

#### Handler Architecture

Each handler (called a "Reactor" in the code) manages one or more P2P streams:

```text
Handler/Reactor
    ├── HandleMessage()  - Process incoming messages
    ├── Start()          - Initialize (implement Component)
    ├── Stop()           - Cleanup
    └── Background loops - Periodic tasks (optional)

```text

#### 1. HandshakeHandler

**Purpose:** Two-phase handshake with peer authentication.

**Protocol Flow:**

```text
Initiator                          Responder
    |                                   |
    |--- HelloRequest ----------------→|
    |    (NodeID, Version, ChainID)     |
    |                                   |
    |←-- HelloResponse ----------------|
    |    (NodeID, PublicKey)            |
    |                                   |
    |--- HelloFinalize ---------------→|
    |                                   |
    |←-- HelloFinalize ----------------|
    |                                   |
    [PrepareStreams with PublicKey]
    [StateEstablished - encrypted streams active]

```text

**State Tracking:**

```go
type PeerHandshakeState struct {
    State       HandshakeState
    StartedAt   time.Time
    PeerPubKey  []byte
    PeerNodeID  string
    PeerHeight  int64
    PeerVersion int32
    PeerChainID string

    // Flags for order-independent progress tracking
    SentRequest      bool
    ReceivedRequest  bool
    SentResponse     bool
    ReceivedResponse bool
    StreamsPrepared  bool
    SentFinalize     bool
    ReceivedFinalize bool
}

```text

**Why Order-Independent Flags?**

- Both peers can initiate simultaneously
- Messages may arrive out-of-order
- No linear state machine required

**Validation:**

```go
func (h *HandshakeHandler) validateHelloRequest(req *schema.HelloRequest) error {
    // Check chain ID
    if req.ChainId != h.chainID {
        h.network.Ban(peerID, TempBanDurationChainMismatch)
        return errors.New("chain ID mismatch")
    }

    // Check protocol version
    if req.Version != h.protocolVersion {
        h.network.Ban(peerID, TempBanDurationVersionMismatch)
        return errors.New("version mismatch")
    }

    return nil
}

```text

**Timeout Handling:**

```go
func (h *HandshakeHandler) timeoutLoop() {
    ticker := time.NewTicker(h.checkInterval)  // 5s default
    defer ticker.Stop()

    for {
        select {
        case <-h.stopCh:
            return
        case <-ticker.C:
            h.cleanupStaleHandshakes()
        }
    }
}

func (h *HandshakeHandler) cleanupStaleHandshakes() {
    now := time.Now()

    h.mu.Lock()
    defer h.mu.Unlock()

    for peerID, state := range h.states {
        if state.State != StateComplete &&
           now.Sub(state.StartedAt) > h.timeout {  // 30s default
            delete(h.states, peerID)
            go h.network.Disconnect(peerID)
        }
    }
}

```text

#### 2. TransactionsReactor

**Purpose:** Transaction gossiping and mempool synchronization.

**Protocol Messages:**

```go
// schema/messages.cram
message TxAnnounce {
    hashes: []bytes  // Transaction hashes we have
}

message TxRequest {
    hashes: []bytes  // Request full transactions
}

message TxResponse {
    txs: []bytes     // Full transaction data
}

```text

**Gossiping Strategy:**

```go
func (r *TransactionsReactor) gossipTx(tx []byte) {
    hash := types.HashTx(tx)

    // Find peers that should receive this tx
    peers := r.peerManager.PeersToSendTx(hash)

    // Announce to peers
    announce := &schema.TxAnnounce{
        Hashes: [][]byte{hash},
    }

    data, _ := cramberry.Marshal(announce)

    for _, peerID := range peers {
        r.network.Send(peerID, "transactions", data)
        r.peerManager.MarkTxSent(peerID, hash)
    }
}

```text

**Request Handling:**

```go
func (r *TransactionsReactor) handleTxRequest(peerID peer.ID, req *schema.TxRequest) {
    var txs [][]byte

    for _, hash := range req.Hashes {
        tx, err := r.mempool.GetTx(hash)
        if err == nil {
            txs = append(txs, tx)
        }
    }

    if len(txs) > 0 {
        resp := &schema.TxResponse{Txs: txs}
        data, _ := cramberry.Marshal(resp)
        r.network.Send(peerID, "transactions", data)
    }
}

```text

**Rate Limiting:**

```go
type TxRateLimiter struct {
    requests  map[peer.ID]*rateLimitState
    maxPerMin int
    mu        sync.RWMutex
}

func (l *TxRateLimiter) Allow(peerID peer.ID) bool {
    l.mu.Lock()
    defer l.mu.Unlock()

    state := l.requests[peerID]
    if state == nil {
        state = &rateLimitState{
            count:      0,
            windowStart: time.Now(),
        }
        l.requests[peerID] = state
    }

    // Reset window if expired
    if time.Since(state.windowStart) > time.Minute {
        state.count = 0
        state.windowStart = time.Now()
    }

    if state.count >= l.maxPerMin {
        return false  // Rate limited
    }

    state.count++
    return true
}

```text

#### 3. BlockReactor

**Purpose:** Real-time block propagation.

**Protocol:**

```go
message BlockAnnounce {
    height: int64
    hash:   bytes
}

message BlockRequest {
    height: int64
}

message BlockResponse {
    height: int64
    hash:   bytes
    data:   bytes    // Full block data
}

```text

**Propagation:**

```go
func (r *BlockReactor) propagateBlock(height int64, hash, data []byte) {
    // Find peers that need this block
    peers := r.peerManager.PeersToSendBlock(height)

    for _, peerID := range peers {
        // Send announce first
        announce := &schema.BlockAnnounce{
            Height: height,
            Hash:   hash,
        }

        announceData, _ := cramberry.Marshal(announce)
        r.network.Send(peerID, "blocks", announceData)

        r.peerManager.MarkBlockSent(peerID, height)
    }
}

```text

**Validation (with callback to application):**

```go
func (r *BlockReactor) handleBlockResponse(peerID peer.ID, resp *schema.BlockResponse) {
    // Check size limit
    if len(resp.Data) > r.maxBlockSize {
        r.peerManager.AddPenalty(peerID, PenaltyOversizedBlock)
        return
    }

    // Verify hash
    actualHash := types.Hash(resp.Data)
    if !bytes.Equal(actualHash, resp.Hash) {
        r.peerManager.AddPenalty(peerID, PenaltyInvalidHash)
        return
    }

    // Validate block (application-defined)
    if r.validator != nil {
        if err := r.validator(resp.Height, resp.Data); err != nil {
            log.Warn("Block validation failed: %v", err)
            r.peerManager.AddPenalty(peerID, PenaltyInvalidBlock)
            return
        }
    }

    // Store block
    if err := r.blockStore.SaveBlock(resp.Height, resp.Hash, resp.Data); err != nil {
        log.Error("Failed to save block: %v", err)
        return
    }

    // Notify callbacks
    if r.callbacks != nil && r.callbacks.OnBlockApplied != nil {
        r.callbacks.OnBlockApplied(resp.Height, resp.Hash)
    }
}

```text

#### 4. ConsensusReactor

**Purpose:** Forward raw consensus messages to application.

**Design Philosophy:**

- Blockberry is NOT a consensus engine
- Provides raw byte transport for application-defined consensus
- No message parsing/interpretation

**Interface:**

```go
type ConsensusHandler interface {
    HandleMessage(peerID peer.ID, data []byte) error
}

```text

**Implementation:**

```go
func (r *ConsensusReactor) HandleMessage(msg *glueberry.Message) {
    if r.handler == nil {
        // No consensus handler set - ignore
        return
    }

    // Forward raw bytes directly to application
    if err := r.handler.HandleMessage(msg.PeerID, msg.Data); err != nil {
        log.Warn("Consensus handler error: %v", err)
    }
}

```text

**Usage (Tendermint example):**

```go
type TendermintConsensus struct {
    // ... consensus state
}

func (tc *TendermintConsensus) HandleMessage(peerID peer.ID, data []byte) error {
    // Parse Tendermint message
    msg, err := tendermint.DecodeMessage(data)
    if err != nil {
        return err
    }

    // Process according to Tendermint protocol
    switch m := msg.(type) {
    case *tendermint.Proposal:
        return tc.handleProposal(peerID, m)
    case *tendermint.Vote:
        return tc.handleVote(peerID, m)
    case *tendermint.BlockPart:
        return tc.handleBlockPart(peerID, m)
    default:
        return errors.New("unknown message type")
    }
}

```text

#### 5. HousekeepingReactor

**Purpose:** Connection quality monitoring and maintenance.

**Features:**

- Latency probes
- Firewall detection
- Keep-alive

**Protocol:**

```go
message LatencyProbe {
    timestamp: int64  // Unix nanos
    nonce:     int64  // Random nonce
}

message LatencyProbeResponse {
    nonce: int64      // Echo nonce from request
}

```text

**Latency Measurement:**

```go
func (r *HousekeepingReactor) sendLatencyProbe(peerID peer.ID) {
    nonce := rand.Int63()

    probe := &schema.LatencyProbe{
        Timestamp: time.Now().UnixNano(),
        Nonce:     nonce,
    }

    data, _ := cramberry.Marshal(probe)

    // Track probe
    r.probes[peerID] = &probeState{
        nonce:   nonce,
        sentAt:  time.Now(),
    }

    r.network.Send(peerID, "housekeeping", data)
}

func (r *HousekeepingReactor) handleLatencyProbeResponse(peerID peer.ID, resp *schema.LatencyProbeResponse) {
    probe, ok := r.probes[peerID]
    if !ok || probe.nonce != resp.Nonce {
        return  // Unknown or mismatched probe
    }

    latency := time.Since(probe.sentAt)
    delete(r.probes, peerID)

    // Update peer latency
    r.peerManager.UpdateLatency(peerID, latency)
}

```text

**Periodic Probing:**

```go
func (r *HousekeepingReactor) probeLoop() {
    ticker := time.NewTicker(r.interval)  // 60s default
    defer ticker.Stop()

    for {
        select {
        case <-r.stopCh:
            return
        case <-ticker.C:
            r.probeAllPeers()
        }
    }
}

func (r *HousekeepingReactor) probeAllPeers() {
    peers := r.peerManager.AllPeerIDs()

    for _, peerID := range peers {
        r.sendLatencyProbe(peerID)
    }
}

```text

#### Concurrency Patterns in Handlers

**1. Message Processing (per-handler):**

```go
func (r *Reactor) HandleMessage(msg *glueberry.Message) {
    // Quick dispatch - don't block event loop
    go r.processMessage(msg)
}

func (r *Reactor) processMessage(msg *glueberry.Message) {
    // Unmarshal
    var envelope schema.MessageEnvelope
    if err := cramberry.UnmarshalInterface(msg.Data, &envelope); err != nil {
        log.Warn("Failed to unmarshal: %v", err)
        return
    }

    // Type switch
    switch m := envelope.(type) {
    case *schema.TypeA:
        r.handleTypeA(msg.PeerID, m)
    case *schema.TypeB:
        r.handleTypeB(msg.PeerID, m)
    }
}

```text

**2. Rate Limiting (per-peer, per-stream):**

```go
type streamRateLimiter struct {
    limiters map[peer.ID]*rateLimiter
    mu       sync.RWMutex
}

func (l *streamRateLimiter) Allow(peerID peer.ID) bool {
    l.mu.RLock()
    limiter := l.limiters[peerID]
    l.mu.RUnlock()

    if limiter == nil {
        l.mu.Lock()
        limiter = newRateLimiter(rate.Limit(100), 200)  // 100/s, burst 200
        l.limiters[peerID] = limiter
        l.mu.Unlock()
    }

    return limiter.Allow()
}

```text

**3. Pending Request Tracking:**

```go
type pendingRequests struct {
    requests map[string]*pendingRequest  // requestID → request
    mu       sync.RWMutex
}

type pendingRequest struct {
    peerID    peer.ID
    createdAt time.Time
    responseCh chan *Response
}

func (p *pendingRequests) Add(reqID string, req *pendingRequest) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.requests[reqID] = req
}

func (p *pendingRequests) Complete(reqID string, resp *Response) {
    p.mu.Lock()
    req, ok := p.requests[reqID]
    delete(p.requests, reqID)
    p.mu.Unlock()

    if ok {
        select {
        case req.responseCh <- resp:
        default:
            // Response channel closed or full
        }
    }
}

func (p *pendingRequests) Cleanup(maxAge time.Duration) {
    now := time.Now()

    p.mu.Lock()
    defer p.mu.Unlock()

    for id, req := range p.requests {
        if now.Sub(req.createdAt) > maxAge {
            close(req.responseCh)
            delete(p.requests, id)
        }
    }
}

```text

---

### 7. internal/p2p - Peer Management

**Location:** `/Volumes/Tendermint/stealth/blockberry/internal/p2p/`

**Purpose:** Manage peer connections, track per-peer state, enforce limits.

#### Network Wrapper

**Purpose:** Wrap glueberry with Blockberry-specific abstractions.

```go
type Network struct {
    glueNode    *glueberry.Node
    peerManager *PeerManager

    // Stream registry maps stream names to handlers
    streamRegistry *StreamRegistry
}

func NewNetwork(glueNode *glueberry.Node) *Network {
    return &Network{
        glueNode:       glueNode,
        peerManager:    NewPeerManager(),
        streamRegistry: NewStreamRegistry(),
    }
}

```text

**Stream Registry:**

```go
type StreamRegistry struct {
    streams map[string]StreamConfig
    mu      sync.RWMutex
}

type StreamConfig struct {
    Name        string
    MaxMsgSize  int64
    RateLimit   int  // Messages per second
    RequiresAuth bool
}

func (r *StreamRegistry) Register(name string, cfg StreamConfig) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.streams[name] = cfg
}

func (r *StreamRegistry) Validate(name string, msgSize int) error {
    r.mu.RLock()
    cfg, ok := r.streams[name]
    r.mu.RUnlock()

    if !ok {
        return errors.New("unknown stream")
    }

    if cfg.MaxMsgSize > 0 && int64(msgSize) > cfg.MaxMsgSize {
        return errors.New("message too large")
    }

    return nil
}

```text

#### PeerManager

**Purpose:** Central registry of all connected peers.

```go
type PeerManager struct {
    peers map[peer.ID]*PeerState
    mu    sync.RWMutex
}

func (pm *PeerManager) AddPeer(peerID peer.ID, isOutbound bool) *PeerState {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    if state, exists := pm.peers[peerID]; exists {
        return state
    }

    state := NewPeerState(peerID, isOutbound)
    pm.peers[peerID] = state
    return state
}

func (pm *PeerManager) GetPeer(peerID peer.ID) *PeerState {
    pm.mu.RLock()
    defer pm.mu.RUnlock()
    return pm.peers[peerID]
}

```text

**Bulk Operations:**

```go
func (pm *PeerManager) PeersToSendTx(txHash []byte) []peer.ID {
    pm.mu.RLock()
    defer pm.mu.RUnlock()

    var result []peer.ID
    for id, state := range pm.peers {
        if state.ShouldSendTx(txHash) {
            result = append(result, id)
        }
    }
    return result
}

func (pm *PeerManager) PeersToSendBlock(height int64) []peer.ID {
    pm.mu.RLock()
    defer pm.mu.RUnlock()

    var result []peer.ID
    for id, state := range pm.peers {
        if state.ShouldSendBlock(height) {
            result = append(result, id)
        }
    }
    return result
}

```text

#### PeerState

**Purpose:** Track per-peer data exchange and reputation.

```go
type PeerState struct {
    peerID     peer.ID
    isOutbound bool
    connectedAt time.Time

    // Transaction tracking (prevent redundant sends)
    txsSent     *lru.Cache  // LRU cache of sent tx hashes
    txsReceived *lru.Cache  // LRU cache of received tx hashes

    // Block tracking
    blocksSent     *lru.Cache
    blocksReceived *lru.Cache

    // Latency
    latency       time.Duration
    latencyUpdatedAt time.Time

    // Reputation
    score       int64
    penalties   []penalty
    isBanned    bool
    bannedUntil time.Time

    mu sync.RWMutex
}

```text

**Transaction Deduplication:**

```go
func (ps *PeerState) ShouldSendTx(txHash []byte) bool {
    ps.mu.RLock()
    defer ps.mu.RUnlock()

    hashKey := string(txHash)

    // Don't send if already sent
    if ps.txsSent.Contains(hashKey) {
        return false
    }

    // Don't send if they sent it to us
    if ps.txsReceived.Contains(hashKey) {
        return false
    }

    return true
}

func (ps *PeerState) MarkTxSent(txHash []byte) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    ps.txsSent.Add(string(txHash), true)
}

func (ps *PeerState) MarkTxReceived(txHash []byte) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    ps.txsReceived.Add(string(txHash), true)
}

```text

**Same pattern for blocks:**

```go
func (ps *PeerState) ShouldSendBlock(height int64) bool {
    ps.mu.RLock()
    defer ps.mu.RUnlock()

    heightKey := fmt.Sprintf("%d", height)

    if ps.blocksSent.Contains(heightKey) {
        return false
    }

    if ps.blocksReceived.Contains(heightKey) {
        return false
    }

    return true
}

```text

#### Scoring System

**Penalty Types:**

```go
type PenaltyType int

const (
    PenaltyInvalidMessage PenaltyType = iota
    PenaltyInvalidTx
    PenaltyInvalidBlock
    PenaltyOversizedMessage
    PenaltyRateLimitExceeded
    PenaltyHandshakeTimeout
    PenaltyChainMismatch
    PenaltyVersionMismatch
)

var penaltyWeights = map[PenaltyType]int64{
    PenaltyInvalidMessage:    10,
    PenaltyInvalidTx:         5,
    PenaltyInvalidBlock:      100,  // Severe
    PenaltyOversizedMessage:  20,
    PenaltyRateLimitExceeded: 5,
    PenaltyHandshakeTimeout:  50,
    PenaltyChainMismatch:     1000, // Permanent ban
    PenaltyVersionMismatch:   500,
}

```text

**Scoring Logic:**

```go
type penalty struct {
    penaltyType PenaltyType
    timestamp   time.Time
    weight      int64
}

func (ps *PeerState) AddPenalty(pt PenaltyType) {
    ps.mu.Lock()
    defer ps.mu.Unlock()

    weight := penaltyWeights[pt]

    p := penalty{
        penaltyType: pt,
        timestamp:   time.Now(),
        weight:      weight,
    }

    ps.penalties = append(ps.penalties, p)
    ps.score -= weight

    // Check for ban thresholds
    if ps.score <= -1000 {
        ps.isBanned = true
        ps.bannedUntil = time.Now().Add(24 * time.Hour)
    }
}

func (ps *PeerState) CalculateScore() int64 {
    ps.mu.RLock()
    defer ps.mu.RUnlock()

    // Decay old penalties (forgiveness over time)
    now := time.Now()
    score := int64(0)

    for _, p := range ps.penalties {
        age := now.Sub(p.timestamp)
        if age < 24*time.Hour {
            // Recent penalty - full weight
            score -= p.weight
        } else if age < 7*24*time.Hour {
            // Older penalty - half weight
            score -= p.weight / 2
        }
        // Very old penalties (>7 days) are forgiven
    }

    return score
}

```text

#### Rate Limiting (per-peer)

```go
type PeerRateLimiter struct {
    limiters map[peer.ID]*tokenBucket
    mu       sync.RWMutex
}

type tokenBucket struct {
    rate       float64  // Tokens per second
    capacity   int64    // Maximum tokens
    tokens     float64  // Current tokens
    lastUpdate time.Time
    mu         sync.Mutex
}

func (tb *tokenBucket) Allow(n int) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    now := time.Now()
    elapsed := now.Sub(tb.lastUpdate).Seconds()

    // Add tokens based on elapsed time
    tb.tokens += elapsed * tb.rate
    if tb.tokens > float64(tb.capacity) {
        tb.tokens = float64(tb.capacity)
    }

    tb.lastUpdate = now

    // Check if we have enough tokens
    if tb.tokens >= float64(n) {
        tb.tokens -= float64(n)
        return true
    }

    return false
}

```text

**Usage:**

```go
func (r *Reactor) HandleMessage(msg *glueberry.Message) {
    if !r.rateLimiter.Allow(msg.PeerID, 1) {
        r.peerManager.AddPenalty(msg.PeerID, PenaltyRateLimitExceeded)
        return
    }

    // Process message
    r.processMessage(msg)
}

```text

---

## Code Patterns and Conventions

### 1. Defensive Programming

**Deep Copying for Safety:**

```go
// Always copy byte slices before storing
func (m *Mempool) AddTx(tx []byte) error {
    txCopy := append([]byte(nil), tx...)
    m.txs[hash] = txCopy
    return nil
}

// Always copy byte slices when returning
func (m *Mempool) GetTx(hash []byte) ([]byte, error) {
    tx := m.txs[hash]
    return append([]byte(nil), tx...), nil
}

```text

**Rationale:** Prevents caller from modifying internal state.

### 2. Fail-Closed Security

**Default to Rejection:**

```go
var DefaultTxValidator TxValidator = func(tx []byte) error {
    return errors.New("no validator configured")
}

func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("not implemented"),
    }
}

```text

**Rationale:** System is secure by default; features must be explicitly enabled.

### 3. Thread Safety

**Read-Write Mutexes:**

```go
type Store struct {
    data map[string][]byte
    mu   sync.RWMutex
}

func (s *Store) Get(key string) ([]byte, bool) {
    s.mu.RLock()  // Multiple concurrent readers allowed
    defer s.mu.RUnlock()
    val, ok := s.data[key]
    return val, ok
}

func (s *Store) Put(key string, value []byte) {
    s.mu.Lock()  // Exclusive access for writes
    defer s.mu.Unlock()
    s.data[key] = value
}

```text

### 4. Error Handling

**Error Wrapping with Context:**

```go
func (s *Store) SaveBlock(height int64, data []byte) error {
    if err := s.validate(data); err != nil {
        return fmt.Errorf("validating block at height %d: %w", height, err)
    }

    if err := s.db.Put(key, data); err != nil {
        return fmt.Errorf("writing to database: %w", err)
    }

    return nil
}

```text

**Sentinel Errors:**

```go
var (
    ErrBlockNotFound = errors.New("block not found")
    ErrMempoolFull   = errors.New("mempool is full")
    ErrTxTooLarge    = errors.New("transaction exceeds size limit")
)

// Usage with errors.Is()
if errors.Is(err, ErrBlockNotFound) {
    // Handle missing block
}

```text

### 5. Interface-Based Design

**Pluggable Components:**

```go
type BlockStore interface {
    SaveBlock(height int64, hash, data []byte) error
    LoadBlock(height int64) ([]byte, []byte, error)
    // ...
}

// Multiple implementations
var _ BlockStore = (*LevelDBBlockStore)(nil)
var _ BlockStore = (*BadgerDBBlockStore)(nil)
var _ BlockStore = (*MemoryBlockStore)(nil)

```text

**Benefit:** Easy testing, swappable implementations, clear contracts.

### 6. Functional Options Pattern

**Flexible Configuration:**

```go
type Node struct {
    // ...
}

type Option func(*Node)

func WithMempool(mp mempool.Mempool) Option {
    return func(n *Node) {
        n.mempool = mp
    }
}

func WithBlockStore(bs blockstore.BlockStore) Option {
    return func(n *Node) {
        n.blockStore = bs
    }
}

// Usage
node, err := NewNode(cfg,
    WithMempool(customMempool),
    WithBlockStore(customStore),
)

```text

**Benefits:**

- Optional parameters
- Backwards compatible
- Self-documenting
- Type-safe

### 7. Context Propagation

**Cancellation and Timeouts:**

```go
func (app *Application) ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return &TxExecResult{
            Code:  CodeTimeout,
            Error: ctx.Err(),
        }
    default:
    }

    // Execute with timeout
    result, err := app.executeWithTimeout(ctx, tx)
    return result
}

```text

### 8. Resource Cleanup

**Deferred Cleanup:**

```go
func (s *Store) LoadBlock(height int64) ([]byte, []byte, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()  // Always unlock, even on error

    // ... load block

    return hash, data, nil
}

```text

**Graceful Shutdown:**

```go
func (n *Node) Stop() error {
    // Signal shutdown
    close(n.stopCh)

    // Wait with timeout
    done := make(chan struct{})
    go func() {
        n.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        // Clean shutdown
    case <-time.After(5 * time.Second):
        // Force shutdown
        log.Warn("Shutdown timeout, forcing stop")
    }

    // Cleanup resources
    n.cleanup()

    return nil
}

```text

### 9. Atomic Operations

**Lock-Free State Flags:**

```go
type Node struct {
    stopping atomic.Bool
    // ...
}

func (n *Node) Stop() error {
    // Set flag without locking
    n.stopping.Store(true)

    // Other goroutines can check:
    if n.stopping.Load() {
        return  // Exit early
    }
}

```text

### 10. LRU Caches

**Bounded Memory Usage:**

```go
import "github.com/hashicorp/golang-lru"

type PeerState struct {
    txsSent *lru.Cache  // Bounded size
}

func NewPeerState(size int) *PeerState {
    cache, _ := lru.New(size)
    return &PeerState{
        txsSent: cache,
    }
}

func (ps *PeerState) MarkTxSent(hash []byte) {
    ps.txsSent.Add(string(hash), true)  // Automatically evicts oldest
}

```text

---

## Security Considerations

### 1. Fail-Closed Defaults

**Principle:** System denies all operations by default. Features must be explicitly enabled.

**Examples:**

- `BaseApplication` rejects all transactions
- Default mempool validator rejects all
- No block validator = blocks rejected
- Unknown stream names = messages dropped

### 2. Input Validation

**Message Size Limits:**

```go
func (n *Network) Send(peerID peer.ID, stream string, data []byte) error {
    cfg := n.streamRegistry.Get(stream)

    if cfg.MaxMsgSize > 0 && len(data) > cfg.MaxMsgSize {
        return errors.New("message exceeds size limit")
    }

    return n.glueNode.Send(peerID, stream, data)
}

```text

**Block Size Limits:**

```go
func (r *BlockReactor) validateBlock(data []byte) error {
    if len(data) > r.maxBlockSize {
        return errors.New("block too large")
    }
    return nil
}

```text

### 3. Rate Limiting

**Per-Peer Message Limits:**

- Token bucket algorithm
- Configurable rates per stream
- Penalty system for violations

**Per-Peer Bandwidth Limits:**

- Bytes per second tracking
- Burst allowance
- Backpressure on violation

### 4. Eclipse Attack Mitigation

**Diversity Requirements:**

```go
type EclipseMitigation struct {
    maxPeersPerSubnet      int
    maxPeersFromSameSource int
    requireOutboundPercent int
}

func (em *EclipseMitigation) ShouldAcceptPeer(peerID peer.ID, addr string, isInbound bool) bool {
    // Check subnet diversity
    subnet := extractSubnet(addr)
    if em.countPeersInSubnet(subnet) >= em.maxPeersPerSubnet {
        return false
    }

    // Require minimum outbound connections
    if isInbound {
        outboundPct := em.outboundCount * 100 / em.totalCount
        if outboundPct < em.requireOutboundPercent {
            return false  // Need more outbound connections first
        }
    }

    return true
}

```text

### 5. Peer Reputation System

**Penalty-Based Scoring:**

- Track misbehavior (invalid messages, protocol violations)
- Accumulate penalty points
- Auto-ban at threshold
- Time-based decay (forgiveness)

**Permanent Blacklisting:**

- Chain ID mismatch
- Repeated severe violations
- Cryptographic fraud

### 6. Defensive Copying

**Prevent Mutation Bugs:**

```go
// WRONG - caller can modify internal state
func (s *Store) GetData() []byte {
    return s.data  // Returns reference to internal slice
}

// CORRECT - return defensive copy
func (s *Store) GetData() []byte {
    return append([]byte(nil), s.data...)  // Returns independent copy
}

```text

**Applies to:**

- Transaction data
- Block data
- Validator sets
- Peer addresses

### 7. Timing Attack Prevention

**Constant-Time Comparisons:**

```go
import "crypto/subtle"

func verifyHash(expected, actual []byte) bool {
    // WRONG - timing leak
    // return bytes.Equal(expected, actual)

    // CORRECT - constant time
    return subtle.ConstantTimeCompare(expected, actual) == 1
}

```text

### 8. Resource Exhaustion Prevention

**Limits on All Resources:**

- Transaction count in mempool
- Transaction size
- Block size
- Message size
- Peer connections
- Pending requests
- Subscription count

**Example:**

```go
type Limits struct {
    MaxTxs          int
    MaxTxSize       int64
    MaxBlockSize    int64
    MaxMsgSize      int64
    MaxPeers        int
    MaxSubscribers  int
}

func (l *Limits) Validate() error {
    if l.MaxTxSize <= 0 || l.MaxBlockSize <= 0 {
        return errors.New("invalid limits")
    }
    return nil
}

```text

---

## Performance Optimizations

### 1. Memory Pools

**sync.Pool for Buffer Reuse:**

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func processMessage(data []byte) error {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()

    // Use buffer
    buf.Write(data)
    // ...

    return nil
}

```text

**Benefits:**

- Reduces GC pressure
- Reuses allocated memory
- Faster than repeated allocations

### 2. LRU Caches

**Bounded In-Memory Storage:**

```go
type PeerState struct {
    txsSent *lru.Cache  // Fixed size, auto-evicts oldest
}

func NewPeerState(cacheSize int) *PeerState {
    cache, _ := lru.New(cacheSize)
    return &PeerState{
        txsSent: cache,
    }
}

```text

**Benefits:**

- O(1) lookup
- O(1) insertion
- Bounded memory
- Automatic eviction

### 3. Batch Operations

**LevelDB Batching:**

```go
func (s *Store) SaveBlocks(blocks []Block) error {
    batch := new(leveldb.Batch)

    for _, block := range blocks {
        key := makeKey(block.Height)
        batch.Put(key, block.Data)
    }

    // Single atomic write
    return s.db.Write(batch, &opt.WriteOptions{Sync: true})
}

```text

**Benefits:**

- Fewer disk I/O operations
- Atomic multi-key updates
- Better throughput

### 4. Goroutine Pooling

**Worker Pool Pattern:**

```go
type WorkerPool struct {
    workers  int
    taskCh   chan Task
    resultCh chan Result
    wg       sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
    wp := &WorkerPool{
        workers:  workers,
        taskCh:   make(chan Task, workers*10),
        resultCh: make(chan Result, workers*10),
    }

    for i := 0; i < workers; i++ {
        wp.wg.Add(1)
        go wp.worker()
    }

    return wp
}

func (wp *WorkerPool) worker() {
    defer wp.wg.Done()

    for task := range wp.taskCh {
        result := task.Process()
        wp.resultCh <- result
    }
}

```text

**Benefits:**

- Bounded goroutine count
- Prevents goroutine explosion
- Better CPU cache utilization

### 5. Read-Write Locks

**Concurrent Reads:**

```go
type Store struct {
    data map[string][]byte
    mu   sync.RWMutex  // Not sync.Mutex
}

func (s *Store) Get(key string) []byte {
    s.mu.RLock()  // Multiple readers can proceed simultaneously
    defer s.mu.RUnlock()
    return s.data[key]
}

```text

**Benefits:**

- Multiple concurrent readers
- No contention for read-only operations
- Better throughput for read-heavy workloads

### 6. Zero-Copy Techniques

**Minimize Allocations:**

```go
// SLOW - multiple allocations
func slowConcat(a, b, c []byte) []byte {
    result := append(a, b...)
    result = append(result, c...)
    return result
}

// FAST - single allocation
func fastConcat(a, b, c []byte) []byte {
    result := make([]byte, 0, len(a)+len(b)+len(c))
    result = append(result, a...)
    result = append(result, b...)
    result = append(result, c...)
    return result
}

```text

### 7. Efficient Serialization

**Cramberry Benefits:**

- Binary format (smaller than JSON)
- Schema-based (no field names in data)
- Fast encoding/decoding
- Polymorphic message support

**Example:**

```go
// ~10x faster than JSON
data, err := cramberry.Marshal(message)

var msg schema.BlockResponse
err := cramberry.Unmarshal(data, &msg)

```text

### 8. Database Compaction

**LevelDB:**

```go
func (s *LevelDBBlockStore) Compact() error {
    return s.db.CompactRange(util.Range{})
}

```text

**BadgerDB:**

```go
func (s *BadgerDBBlockStore) Compact() error {
    for {
        err := s.db.RunValueLogGC(0.5)  // Discard ratio 50%
        if err == badger.ErrNoRewrite {
            break
        }
        if err != nil {
            return err
        }
    }
    return nil
}

```text

**Benefits:**

- Reclaim disk space
- Improve read performance
- Reduce amplification

---

## Testing Strategy

### 1. Unit Tests

**Table-Driven Tests:**

```go
func TestMempool_AddTx(t *testing.T) {
    tests := []struct {
        name    string
        tx      []byte
        wantErr error
    }{
        {
            name:    "valid transaction",
            tx:      []byte("valid tx"),
            wantErr: nil,
        },
        {
            name:    "empty transaction",
            tx:      nil,
            wantErr: types.ErrInvalidTx,
        },
        {
            name:    "duplicate transaction",
            tx:      []byte("dup tx"),
            wantErr: types.ErrTxAlreadyExists,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mp := mempool.NewSimpleMempool(100, 1024)

            err := mp.AddTx(tt.tx)

            if !errors.Is(err, tt.wantErr) {
                t.Errorf("AddTx() error = %v, want %v", err, tt.wantErr)
            }
        })
    }
}

```text

### 2. Mocking

**Interface Mocking:**

```go
type MockBlockStore struct {
    blocks map[int64][]byte
}

func (m *MockBlockStore) SaveBlock(height int64, hash, data []byte) error {
    m.blocks[height] = data
    return nil
}

func (m *MockBlockStore) LoadBlock(height int64) ([]byte, []byte, error) {
    data, ok := m.blocks[height]
    if !ok {
        return nil, nil, types.ErrBlockNotFound
    }
    return nil, data, nil
}

// Use in tests
func TestReactor(t *testing.T) {
    mockStore := &MockBlockStore{blocks: make(map[int64][]byte)}
    reactor := NewBlockReactor(mockStore)
    // ... test reactor
}

```text

### 3. Integration Tests

**Location:** `/test/integration_test.go`

**Multi-Node Tests:**

```go
func TestTwoNodeConsensus(t *testing.T) {
    // Start two nodes
    node1, err := test.StartTestNode(t, "node1")
    require.NoError(t, err)
    defer node1.Stop()

    node2, err := test.StartTestNode(t, "node2")
    require.NoError(t, err)
    defer node2.Stop()

    // Connect nodes
    node1.ConnectTo(node2.Address())

    // Wait for handshake
    time.Sleep(2 * time.Second)

    // Send transaction
    tx := []byte("test tx")
    err = node1.BroadcastTx(tx)
    require.NoError(t, err)

    // Verify node2 received
    assert.Eventually(t, func() bool {
        _, err := node2.Mempool().GetTx(types.HashTx(tx))
        return err == nil
    }, 5*time.Second, 100*time.Millisecond)
}

```text

### 4. Race Detection

**Always Test with Race Detector:**

```bash
go test -race ./...

```text

**Example Race Bug:**

```go
// BUGGY CODE - data race
type Counter struct {
    count int  // NOT protected
}

func (c *Counter) Increment() {
    c.count++  // Race condition!
}

// FIXED CODE
type Counter struct {
    count int
    mu    sync.Mutex
}

func (c *Counter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}

```text

### 5. Benchmarks

**Performance Testing:**

```go
func BenchmarkMempool_AddTx(b *testing.B) {
    mp := mempool.NewSimpleMempool(10000, 10*1024*1024)

    txs := make([][]byte, b.N)
    for i := 0; i < b.N; i++ {
        txs[i] = make([]byte, 256)
        rand.Read(txs[i])
    }

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        if err := mp.AddTx(txs[i]); err != nil {
            b.Fatal(err)
        }
    }
}

```text

**Run Benchmarks:**

```bash
go test -bench=. -benchmem ./pkg/mempool

```text

### 6. Test Helpers

**Location:** `/test/helpers.go`

**Reusable Test Utilities:**

```go
package test

func StartTestNode(t *testing.T, name string) (*node.Node, error) {
    cfg := config.DefaultConfig()
    cfg.Node.ChainID = "test-chain"
    cfg.Network.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

    // Create temp directories
    tmpDir := t.TempDir()
    cfg.BlockStore.Path = filepath.Join(tmpDir, "blocks")
    cfg.StateStore.Path = filepath.Join(tmpDir, "state")

    node, err := node.NewNode(cfg)
    if err != nil {
        return nil, err
    }

    if err := node.Start(); err != nil {
        return nil, err
    }

    return node, nil
}

```text

---

## Dependencies and Integration

### 1. Glueberry Integration

**Purpose:** Secure P2P networking with encrypted streams.

**Key Features Used:**

- Libp2p-based networking
- Ed25519 key management
- Stream multiplexing
- Noise protocol encryption

**Integration Points:**

```go
// Create glueberry node
glueCfg := glueberry.NewConfig(
    privateKey,
    addressBookPath,
    listenAddrs,
    glueberry.WithHandshakeTimeout(30 * time.Second),
)

glueNode, err := glueberry.New(glueCfg)
if err != nil {
    return err
}

// Start network
glueNode.Start()

// Handle events
for event := range glueNode.Events() {
    switch event.State {
    case glueberry.StateConnected:
        // Handle new connection
    case glueberry.StateEstablished:
        // Encrypted streams ready
    case glueberry.StateDisconnected:
        // Handle disconnect
    }
}

// Handle messages
for msg := range glueNode.Messages() {
    // Route to appropriate handler
    routeMessage(msg.StreamName, msg.PeerID, msg.Data)
}

// Send message
glueNode.Send(peerID, streamName, data)

```text

**Two-Phase Handshake:**

```go
// Phase 1: On StateConnected
node.Send(peerID, "handshake", helloRequestBytes)

// Phase 2: On receiving HelloResponse
node.PrepareStreams(peerID, peerPublicKey, []string{
    "pex", "transactions", "blocksync", "blocks", "consensus", "housekeeping",
})
node.Send(peerID, "handshake", helloFinalizeBytes)

// Phase 3: On receiving HelloFinalize
node.FinalizeHandshake(peerID)
// Now StateEstablished - encrypted streams active

```text

### 2. Cramberry Integration

**Purpose:** High-performance binary serialization.

**Schema Definition** (`blockberry.cram`):

```cramberry
// Handshake messages
message HelloRequest {
    node_id: bytes
    version: int32
    chain_id: string
    timestamp: int64
    height: int64
}

message HelloResponse {
    node_id: bytes
    public_key: bytes
    height: int64
}

message HelloFinalize {
    // Empty message signals completion
}

// Transaction messages
message TxAnnounce {
    hashes: []bytes
}

message TxRequest {
    hashes: []bytes
}

message TxResponse {
    txs: []bytes
}

// Block messages
message BlockAnnounce {
    height: int64
    hash: bytes
}

message BlockRequest {
    height: int64
}

message BlockResponse {
    height: int64
    hash: bytes
    data: bytes
}

```text

**Generated Code** (`schema/`):

```go
// Auto-generated by cramberry
package schema

type HelloRequest struct {
    NodeId    *[]byte
    Version   *int32
    ChainId   *string
    Timestamp *int64
    Height    *int64
}

func (m *HelloRequest) MarshalCramberry() ([]byte, error) {
    // Generated serialization code
}

func (m *HelloRequest) UnmarshalCramberry(data []byte) error {
    // Generated deserialization code
}

```text

**Usage in Code:**

```go
import (
    "github.com/blockberries/cramberry/pkg/cramberry"
    "github.com/blockberries/blockberry/schema"
)

// Encode
req := &schema.HelloRequest{
    NodeId:    &nodeID,
    Version:   &version,
    ChainId:   &chainID,
    Timestamp: &timestamp,
    Height:    &height,
}
data, err := cramberry.Marshal(req)

// Decode
var req schema.HelloRequest
err := cramberry.Unmarshal(data, &req)

// Polymorphic messages
var msg schema.PexMessage  // Interface type
err := cramberry.UnmarshalInterface(data, &msg)

switch m := msg.(type) {
case *schema.AddressRequest:
    handleAddressRequest(m)
case *schema.AddressResponse:
    handleAddressResponse(m)
}

```text

### 3. Looseberry Integration (Optional)

**Purpose:** DAG-based mempool and consensus (optional extension).

**Components:**

- **DAG Mempool**: High-throughput transaction ordering
- **Certificates**: DAG consensus votes
- **Batches**: Transaction batches with causal ordering

**Integration:**

```go
import (
    "github.com/blockberries/blockberry/pkg/mempool/looseberry"
    loosetypes "github.com/blockberries/looseberry/types"
)

// Create Looseberry mempool
looseMp, err := looseberry.NewLooseberryMempool(cfg)

// Use as standard mempool
node, err := node.NewNode(cfg,
    node.WithMempool(looseMp),
)

// Certificate storage
certStore := blockStore.(blockstore.CertificateStore)

cert := &loosetypes.Certificate{
    // ... certificate data
}

err := certStore.SaveCertificate(cert)

// Query certificates by round
certs, err := certStore.GetCertificatesForRound(round)

// Query certificates by block height
certs, err := certStore.GetCertificatesForHeight(height)

```text

### 4. Cosmos IAVL Integration

**Purpose:** Merkleized key-value store for application state.

**Usage:**

```go
import "github.com/cosmos/iavl"

// Create IAVL tree
tree, err := iavl.NewMutableTree(db, cacheSize)

// Set key-value
tree.Set([]byte("key"), []byte("value"))

// Get value
value, err := tree.Get([]byte("key"))

// Commit (get root hash)
hash, version, err := tree.SaveVersion()

// Load previous version
tree, err = iavl.NewMutableTree(db, cacheSize)
tree.LoadVersion(version)

// Merkle proof
proof, err := tree.GetVersionedProof([]byte("key"), version)

```text

### 5. External Dependencies

**Key Third-Party Libraries:**

```go
// Networking
"github.com/libp2p/go-libp2p/core/peer"
"github.com/multiformats/go-multiaddr"

// Storage
"github.com/syndtr/goleveldb/leveldb"
"github.com/dgraph-io/badger/v4"
"github.com/cosmos/iavl"

// Utilities
"github.com/BurntSushi/toml"  // Config parsing
"github.com/hashicorp/golang-lru"  // LRU caches

// Serialization
"github.com/blockberries/cramberry/pkg/cramberry"

// Networking
"github.com/blockberries/glueberry"

```text

**Dependency Management:**

```bash

# View dependencies
go list -m all

# Update dependencies
go get -u ./...

# Tidy dependencies
go mod tidy

```text

---

## Conclusion

Blockberry is a well-architected blockchain framework that prioritizes:

1. **Security**: Fail-closed defaults, input validation, rate limiting
2. **Modularity**: Pluggable components via interfaces
3. **Performance**: Efficient serialization, batching, caching
4. **Reliability**: Defensive programming, thread safety, graceful shutdown
5. **Flexibility**: Not a consensus engine - provides building blocks

The codebase demonstrates professional Go development practices and is suitable for production use with appropriate application-layer implementation.

### Key Strengths

- Clear separation of concerns (public API vs internal)
- Comprehensive configuration system
- Multiple storage backend support
- Robust P2P networking via Glueberry
- Efficient binary serialization via Cramberry
- Strong security posture with fail-closed defaults
- Excellent test coverage

### Integration Considerations

Applications building on Blockberry must implement:

1. `Application` interface for blockchain logic
2. Transaction validation logic
3. Block validation logic
4. Consensus protocol (or use provided integrations)
5. State management (using provided IAVL store)

The framework handles all infrastructure concerns (networking, storage, mempool), allowing applications to focus on business logic.
