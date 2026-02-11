# Blockberry API Reference

**Version:** 1.0
**Last Updated:** 2026-02-02

## Table of Contents

- [Package Index](#package-index)
- [Package Documentation](#package-documentation)
  - [bapi (Application Lifecycle)](#bapi-application-lifecycle)
  - [pkg/blockstore](#pkgblockstore)
  - [pkg/config](#pkgconfig)
  - [pkg/consensus](#pkgconsensus)
  - [pkg/mempool](#pkgmempool)
  - [pkg/node](#pkgnode)
  - [pkg/statestore](#pkgstatestore)
  - [pkg/types](#pkgtypes)
- [Message Protocol Reference](#message-protocol-reference)
- [Error Reference](#error-reference)
- [Configuration Reference](#configuration-reference)

---

## Package Index

| Package | Purpose |
|---------|---------|
| [bapi](#bapi-application-lifecycle) | Application Lifecycle Interface - main contract between framework and application (external module) |
| [pkg/blockstore](#pkgblockstore) | Block and certificate storage interfaces and implementations |
| [pkg/config](#pkgconfig) | Configuration structures and validation |
| [pkg/consensus](#pkgconsensus) | Pluggable consensus engine interfaces |
| [pkg/events](#pkgevents) | Event bus for publish-subscribe messaging |
| [pkg/indexer](#pkgindexer) | Transaction indexing for queries |
| [pkg/logging](#pkglogging) | Structured logging utilities |
| [pkg/mempool](#pkgmempool) | Transaction mempool interfaces and implementations |
| [pkg/metrics](#pkgmetrics) | Prometheus metrics collection |
| [pkg/node](#pkgnode) | Main Node type and lifecycle management |
| [pkg/rpc](#pkgrpc) | RPC servers (gRPC, JSON-RPC, WebSocket) |
| [pkg/statestore](#pkgstatestore) | Merkleized state storage with IAVL trees |
| [pkg/tracing](#pkgtracing) | OpenTelemetry distributed tracing |
| [pkg/types](#pkgtypes) | Common types and interfaces |

---

## Package Documentation

### bapi (Application Lifecycle)

Package `bapi` (`github.com/blockberries/bapi`) provides the Application Lifecycle Interface for Blockberry applications. Types are in `github.com/blockberries/bapi/types` (aliased as `bapitypes`).

#### Package Overview

The application lifecycle interface is the main contract between the blockchain framework and application logic. Applications implement the `Lifecycle` interface to define their state machine behavior. The framework calls these methods during block execution in a well-defined order:

1. **Handshake** - Called once at genesis
2. **CheckTx** - Called to validate transactions before mempool inclusion (concurrent)
3. **ExecuteBlock** - Called to execute all transactions in a block (sequential)
4. **Commit** - Called to finalize and persist state
5. **Query** - Called to read state (concurrent, anytime)

#### Interfaces

##### Lifecycle

```go
type Lifecycle interface {
    Info() ApplicationInfo
    Handshake(genesis *Genesis) error
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult
    ExecuteBlock(ctx context.Context, block *Block) *BlockResult
    Commit(ctx context.Context) *CommitResult
    Query(ctx context.Context, req *QueryRequest) *QueryResponse
}

```text

The main interface for blockchain applications. All methods are called by the framework.

**Thread Safety:** `CheckTx` and `Query` may be called concurrently from multiple goroutines. Implementations must be thread-safe for these methods. All other methods are called sequentially on a single goroutine.

**Method Details:**

- **Info()** - Returns metadata about the application (name, version, app hash, last height). Called on startup to verify application state matches blockchain state.

- **Handshake(genesis)** - Initializes the application at genesis. The genesis parameter contains initial validators and app-specific genesis state. Called exactly once when starting from height 0. Returns an error if initialization fails.

- **CheckTx(ctx, tx)** - Validates a transaction for mempool inclusion. This is a lightweight pre-execution check. Must not modify state. Returns `TxCheckResult` indicating accept/reject. May be called concurrently.

- **ExecuteBlock(ctx, block)** - Executes all transactions in a block and updates application state. Called once per block with all transactions. Returns `BlockResult` with execution status and events. Must be deterministic.

- **Commit(ctx)** - Finalizes the block and persists all state changes. Returns `CommitResult` with the new application state hash. After Commit, state changes are permanent.

- **Query(ctx, req)** - Reads application state without modifying it. Can be called at any time. Returns `QueryResponse` with query results. May be called concurrently.

**Example:**

```go
type MyApp struct {
    bapi.BaseApplication  // Embed for safe defaults
    state *AppState
}

func (app *MyApp) CheckTx(ctx context.Context, tx *bapitypes.Transaction) *bapitypes.TxCheckResult {
    // Validate transaction format
    if len(tx.Data) == 0 {
        return &bapitypes.TxCheckResult{
            Code:  bapitypes.CodeInvalidTx,
            Error: errors.New("empty transaction"),
        }
    }

    // Check balance, signature, nonce, etc.
    if !app.state.HasSufficientBalance(tx) {
        return &bapitypes.TxCheckResult{
            Code:  bapitypes.CodeInsufficientFunds,
            Error: errors.New("insufficient balance"),
        }
    }

    return &bapitypes.TxCheckResult{Code: bapitypes.CodeOK}
}

func (app *MyApp) ExecuteBlock(ctx context.Context, block *bapitypes.Block) *bapitypes.BlockResult {
    // Execute all transactions and update state
    events, err := app.state.ApplyBlock(block)
    if err != nil {
        return &bapitypes.BlockResult{
            Code:  bapitypes.CodeExecutionFailed,
            Error: err,
        }
    }

    return &bapitypes.BlockResult{
        Code:   bapitypes.CodeOK,
        Events: events,
    }
}

func (app *MyApp) Commit(ctx context.Context) *bapitypes.CommitResult {
    hash, err := app.state.Commit()
    if err != nil {
        return &bapitypes.CommitResult{Error: err}
    }

    return &bapitypes.CommitResult{
        AppHash: hash,
    }
}

```text

##### Validator

```go
type Validator interface {
    GetAddress() []byte
    GetPubKey() []byte
    GetVotingPower() int64
}

```text

Represents a consensus validator. Used in validator set updates returned by `ExecuteBlock`.

**Method Details:**

- **GetAddress()** - Returns the validator's address (typically derived from public key)
- **GetPubKey()** - Returns the validator's public key for signature verification
- **GetVotingPower()** - Returns the validator's voting power (weight in consensus)

#### Types

##### ApplicationInfo

```go
type ApplicationInfo struct {
    Name       string
    Version    string
    AppHash    []byte
    LastHeight int64
}

```text

Metadata about the application returned by `Info()`.

**Fields:**

- `Name` - Application name (e.g., "my-blockchain")
- `Version` - Application version string (e.g., "1.0.0")
- `AppHash` - Last committed application state hash
- `LastHeight` - Last committed block height

##### Transaction

```go
type Transaction struct {
    Data []byte
}

```text

Opaque transaction wrapper. The framework treats transactions as raw bytes; the application defines their structure.

##### TxCheckResult

```go
type TxCheckResult struct {
    Code      uint32
    Error     error
    GasWanted int64
    Priority  int64
}

```text

Result of CheckTx validation.

**Fields:**

- `Code` - Result code (CodeOK = 0 for success, non-zero for rejection)
- `Error` - Error message if rejected
- `GasWanted` - Estimated gas for execution
- `Priority` - Priority for mempool ordering (higher = more urgent)

##### TxExecResult

```go
type TxExecResult struct {
    Code    uint32
    Error   error
    Data    []byte
    Events  []Event
    GasUsed int64
}

```text

Result of ExecuteTx execution.

**Fields:**

- `Code` - Result code (CodeOK = 0 for success)
- `Error` - Error message if execution failed
- `Data` - Arbitrary result data
- `Events` - Events emitted during execution (for indexing)
- `GasUsed` - Actual gas consumed

##### BlockHeader

```go
type BlockHeader struct {
    Height    int64
    Time      int64
    Proposer  []byte
    Evidence  []Evidence
}

```text

Block metadata included in the Block passed to ExecuteBlock.

**Fields:**

- `Height` - Block height
- `Time` - Block timestamp (Unix nanoseconds)
- `Proposer` - Proposer's validator address
- `Evidence` - Evidence of validator misbehavior

##### BlockResult

```go
type BlockResult struct {
    Code             uint32
    Error            error
    Events           []Event
    ValidatorUpdates []ValidatorUpdate
    ConsensusParams  *ConsensusParams
}

```text

Result of ExecuteBlock processing.

**Fields:**

- `Code` - Result code (CodeOK = 0 for success)
- `Error` - Error message if execution failed
- `Events` - Events emitted during execution (for indexing)
- `ValidatorUpdates` - Validator set changes (additions, removals, power updates)
- `ConsensusParams` - Updated consensus parameters (optional)

##### CommitResult

```go
type CommitResult struct {
    AppHash []byte
    Error   error
}

```text

Result of Commit finalization.

**Fields:**

- `AppHash` - New application state hash (must be deterministic)
- `Error` - Error if commit failed

##### QueryRequest

```go
type QueryRequest struct {
    Path   string
    Data   []byte
    Height int64
    Prove  bool
}

```text

Query request for reading application state.

**Fields:**

- `Path` - Query path (e.g., "/account/balance")
- `Data` - Query data (e.g., account address)
- `Height` - Block height to query (0 = latest)
- `Prove` - Whether to include merkle proof

##### QueryResponse

```go
type QueryResponse struct {
    Code   uint32
    Error  error
    Data   []byte
    Height int64
    Proof  *Proof
}

```text

Query response with results.

**Fields:**

- `Code` - Result code (CodeOK = 0 for success)
- `Error` - Error message if query failed
- `Data` - Query result data
- `Height` - Height at which query was executed
- `Proof` - Merkle proof (if requested)

##### Genesis

```go
type Genesis struct {
    ChainID       string
    InitialHeight int64
    Validators    []GenesisValidator
    AppState      []byte
}

```text

Genesis state for Handshake.

**Fields:**

- `ChainID` - Blockchain identifier
- `InitialHeight` - Starting block height (usually 1)
- `Validators` - Initial validator set
- `AppState` - Application-specific genesis data

#### Functions

##### NewBaseApplication

```go
func NewBaseApplication() *BaseApplication

```text

Creates a new BaseApplication with safe fail-closed defaults.

**Returns:** A BaseApplication instance that rejects all operations by default.

**Usage:** Embed BaseApplication in your application struct to inherit safe defaults:

```go
type MyApp struct {
    *bapi.BaseApplication
    // Add your fields
}

func NewMyApp() *MyApp {
    return &MyApp{
        BaseApplication: bapi.NewBaseApplication(),
    }
}

```text

#### Result Codes

```go
const (
    CodeOK                  uint32 = 0
    CodeInvalidTx           uint32 = 1
    CodeInsufficientFunds   uint32 = 2
    CodeNotAuthorized       uint32 = 3
    CodeExecutionFailed     uint32 = 4
    CodeInvalidNonce        uint32 = 5
    CodeInvalidSignature    uint32 = 6
)

```text

Standard result codes for transaction validation and execution.

---

### pkg/blockstore

Package `blockstore` provides block storage interfaces and implementations.

#### Package Overview

BlockStore implementations persist blocks to disk and provide retrieval by height or hash. All implementations must be safe for concurrent use. The package supports:

- **LevelDB** - Production-ready, proven storage backend
- **BadgerDB** - High-performance alternative with better compression
- **In-Memory** - For testing only
- **Certificate Storage** - Extensions for DAG mempool (Looseberry)

#### Interfaces

##### BlockStore

```go
type BlockStore interface {
    SaveBlock(height int64, hash []byte, data []byte) error
    LoadBlock(height int64) (hash []byte, data []byte, err error)
    LoadBlockByHash(hash []byte) (height int64, data []byte, err error)
    HasBlock(height int64) bool
    Height() int64
    Base() int64
    Close() error
}

```text

Interface for block persistence.

**Thread Safety:** All methods must be safe for concurrent use from multiple goroutines.

**Method Details:**

- **SaveBlock(height, hash, data)** - Persists a block at the given height. Returns `ErrBlockAlreadyExists` if a block already exists at that height. The hash and data are stored atomically.

- **LoadBlock(height)** - Retrieves a block by height. Returns the block hash and data. Returns `ErrBlockNotFound` if no block exists at that height.

- **LoadBlockByHash(hash)** - Retrieves a block by its hash. Returns the block height and data. Returns `ErrBlockNotFound` if no block with that hash exists.

- **HasBlock(height)** - Checks if a block exists at the given height without loading it. Faster than LoadBlock for existence checks.

- **Height()** - Returns the latest (highest) block height stored. Returns 0 if no blocks have been stored.

- **Base()** - Returns the earliest available block height. Returns 0 if no blocks exist. For pruned stores, this may be greater than 1.

- **Close()** - Closes the store and releases all resources. Must be called before process termination. After Close, all other methods return errors.

**Example:**

```go
// Create a LevelDB block store
store, err := blockstore.NewLevelDBBlockStore("./data/blocks")
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Save a block
blockData := []byte("block contents")
blockHash := sha256.Sum256(blockData)
err = store.SaveBlock(1, blockHash[:], blockData)

// Load a block
hash, data, err := store.LoadBlock(1)
if err == types.ErrBlockNotFound {
    log.Println("Block not found")
}

// Check current height
height := store.Height()
log.Printf("Latest block: %d", height)

```text

##### CertificateStore

```go
type CertificateStore interface {
    SaveCertificate(cert *loosetypes.Certificate) error
    GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error)
    GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error)
    GetCertificatesForHeight(height int64) ([]*loosetypes.Certificate, error)
    SaveBatch(batch *loosetypes.Batch) error
    GetBatch(digest loosetypes.Hash) (*loosetypes.Batch, error)
    SetCertificateBlockHeight(digest loosetypes.Hash, height int64) error
}

```text

Interface for DAG certificate and batch storage. Used by validators running Looseberry mempool.

**Thread Safety:** All methods must be safe for concurrent use.

**Method Details:**

- **SaveCertificate(cert)** - Persists a DAG certificate indexed by digest, round, and optionally block height. Returns `ErrCertificateAlreadyExists` if the certificate already exists.

- **GetCertificate(digest)** - Retrieves a certificate by its digest. Returns `ErrCertificateNotFound` if not found.

- **GetCertificatesForRound(round)** - Retrieves all certificates for a given DAG round. Returns an empty slice if none exist. Certificates are returned sorted by validator index.

- **GetCertificatesForHeight(height)** - Retrieves all certificates committed at a given block height. Returns an empty slice if none exist.

- **SaveBatch(batch)** - Persists a transaction batch indexed by digest. Returns `ErrBatchAlreadyExists` if already exists.

- **GetBatch(digest)** - Retrieves a batch by its digest. Returns `ErrBatchNotFound` if not found.

- **SetCertificateBlockHeight(digest, height)** - Updates the block height index for a certificate. Called when a certificate is committed in a block.

##### CertificateBlockStore

```go
type CertificateBlockStore interface {
    BlockStore
    CertificateStore
}

```text

Combined interface for stores supporting both blocks and certificates. This is the primary interface for validators running Looseberry.

#### Functions

##### NewLevelDBBlockStore

```go
func NewLevelDBBlockStore(path string) (BlockStore, error)

```text

Creates a new LevelDB-backed block store.

**Parameters:**

- `path` - Directory path for LevelDB storage

**Returns:** A BlockStore implementation or an error if the database cannot be opened.

**Error Conditions:**

- Path is not writable
- Database is corrupted
- Insufficient disk space

**Example:**

```go
store, err := blockstore.NewLevelDBBlockStore("./data/blocks")
if err != nil {
    log.Fatalf("Failed to open block store: %v", err)
}
defer store.Close()

```text

##### NewBadgerDBBlockStore

```go
func NewBadgerDBBlockStore(path string) (BlockStore, error)

```text

Creates a new BadgerDB-backed block store.

**Parameters:**

- `path` - Directory path for BadgerDB storage

**Returns:** A BlockStore implementation or an error if the database cannot be opened.

**Notes:** BadgerDB provides better compression and performance but uses more memory. Recommended for high-throughput chains.

##### NewInMemoryBlockStore

```go
func NewInMemoryBlockStore() BlockStore

```text

Creates a new in-memory block store for testing.

**Returns:** A BlockStore that stores blocks in memory.

**Warning:** All data is lost when the process terminates. Only use for testing.

---

### pkg/config

Package `config` provides configuration structures and validation.

#### Package Overview

Configuration is loaded from TOML files using the `LoadConfig` function. The package provides sensible defaults via `DefaultConfig()`. All configuration structs include validation methods to catch errors early.

#### Types

##### Config

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

Main configuration structure containing all node settings.

##### NodeConfig

```go
type NodeConfig struct {
    ChainID         string
    ProtocolVersion int32
    PrivateKeyPath  string
}

```text

Node identity and chain configuration.

**Fields:**

- `ChainID` - Unique blockchain network identifier (e.g., "my-chain-1")
- `ProtocolVersion` - Protocol version (must match peers)
- `PrivateKeyPath` - Path to Ed25519 private key file

##### NetworkConfig

```go
type NetworkConfig struct {
    ListenAddrs      []string
    MaxInboundPeers  int
    MaxOutboundPeers int
    HandshakeTimeout Duration
    DialTimeout      Duration
    AddressBookPath  string
    Seeds            SeedsConfig
}

```text

P2P networking configuration.

**Fields:**

- `ListenAddrs` - Multiaddrs to listen on (e.g., "/ip4/0.0.0.0/tcp/26656")
- `MaxInboundPeers` - Maximum incoming connections (default: 40)
- `MaxOutboundPeers` - Maximum outgoing connections (default: 10)
- `HandshakeTimeout` - Max time for handshake completion (default: 30s)
- `DialTimeout` - Max time for connection dialing (default: 3s)
- `AddressBookPath` - Path to persist peer addresses
- `Seeds` - Seed node configuration

##### MempoolConfig

```go
type MempoolConfig struct {
    Type            string
    MaxTxs          int
    MaxBytes        int64
    CacheSize       int
    TTL             Duration
    CleanupInterval Duration
}

```text

Transaction mempool configuration.

**Fields:**

- `Type` - Mempool type: "simple", "priority", "ttl", "looseberry" (default: "simple")
- `MaxTxs` - Maximum number of transactions (default: 5000)
- `MaxBytes` - Maximum total size in bytes (default: 1GB)
- `CacheSize` - Recent transaction hash cache size (default: 10000)
- `TTL` - Transaction time-to-live for "ttl" mempool (default: 1h)
- `CleanupInterval` - Cleanup check interval for "ttl" mempool (default: 5m)

##### BlockStoreConfig

```go
type BlockStoreConfig struct {
    Backend string
    Path    string
}

```text

Block storage configuration.

**Fields:**

- `Backend` - Storage backend: "leveldb" or "badgerdb" (default: "leveldb")
- `Path` - Directory path for storage (default: "data/blockstore")

##### StateStoreConfig

```go
type StateStoreConfig struct {
    Path      string
    CacheSize int
}

```text

State storage configuration.

**Fields:**

- `Path` - Directory path for IAVL state (default: "data/state")
- `CacheSize` - IAVL node cache size (default: 10000)

##### LimitsConfig

```go
type LimitsConfig struct {
    MaxTxSize              int64
    MaxBlockSize           int64
    MaxBlockTxs            int
    MaxMsgSize             int64
    MaxSubscribers         int
    MaxSubscribersPerQuery int
}

```text

Resource limits to prevent DoS attacks.

**Fields:**

- `MaxTxSize` - Max transaction size in bytes (default: 1MB)
- `MaxBlockSize` - Max block size in bytes (default: 21MB)
- `MaxBlockTxs` - Max transactions per block (default: 10000)
- `MaxMsgSize` - Max P2P message size (default: 10MB)
- `MaxSubscribers` - Max event subscribers (default: 1000)
- `MaxSubscribersPerQuery` - Max subscribers per query (default: 100)

#### Functions

##### DefaultConfig

```go
func DefaultConfig() *Config

```text

Returns a Config with sensible default values.

**Example:**

```go
cfg := config.DefaultConfig()
cfg.Node.ChainID = "my-chain"
cfg.Network.ListenAddrs = []string{"/ip4/0.0.0.0/tcp/26656"}

```text

##### LoadConfig

```go
func LoadConfig(path string) (*Config, error)

```text

Loads configuration from a TOML file. Missing values are filled with defaults.

**Parameters:**

- `path` - Path to TOML configuration file

**Returns:** A validated Config or an error.

**Error Conditions:**

- File not found or not readable
- Invalid TOML syntax
- Validation errors (invalid values)

**Example:**

```go
cfg, err := config.LoadConfig("config.toml")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}

```text

##### WriteConfigFile

```go
func WriteConfigFile(path string, cfg *Config) error

```text

Writes a Config to a TOML file.

**Example:**

```go
cfg := config.DefaultConfig()
err := config.WriteConfigFile("config.toml", cfg)

```text

#### Example Configuration

```toml
[node]
chain_id = "my-blockchain-1"
protocol_version = 1
private_key_path = "node_key.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
dial_timeout = "3s"
address_book_path = "addrbook.json"

[network.seeds]
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/12D3KooW...",
    "/ip4/seed2.example.com/tcp/26656/p2p/12D3KooW..."
]

[mempool]
type = "simple"
max_txs = 5000
max_bytes = 1073741824  # 1GB
cache_size = 10000

[blockstore]
backend = "leveldb"
path = "data/blockstore"

[statestore]
path = "data/state"
cache_size = 10000

[logging]
level = "info"
format = "text"
output = "stderr"

[limits]
max_tx_size = 1048576      # 1MB
max_block_size = 22020096  # 21MB
max_block_txs = 10000

```text

---

### pkg/consensus

Package `consensus` provides pluggable consensus engine interfaces.

#### Package Overview

Blockberry supports multiple consensus paradigms through a flexible interface system:

- **ConsensusEngine** - Base interface for all consensus implementations
- **BFTConsensus** - Extended interface for BFT-style consensus (proposals, votes, commits)
- **StreamAwareConsensus** - For engines needing custom network protocols

The framework handles networking, storage, and mempool; consensus engines focus on block ordering and validation.

#### Interfaces

##### ConsensusEngine

```go
type ConsensusEngine interface {
    types.Component
    types.Named

    Initialize(deps ConsensusDependencies) error
    ProcessBlock(block *Block) error
    GetHeight() int64
    GetRound() int32
    IsValidator() bool
    ValidatorSet() ValidatorSet
}

```text

Main interface for consensus implementations.

**Thread Safety:** Methods are called sequentially on a single goroutine. Implementations do not need internal synchronization unless they spawn additional goroutines.

**Method Details:**

- **Initialize(deps)** - Sets up the engine with dependencies (network, storage, mempool, application). Called after construction but before Start(). Store references to needed dependencies.

- **ProcessBlock(block)** - Called when a new block is received from a peer. The engine validates the block according to its consensus rules and potentially commits it. Returns an error if the block is invalid.

- **GetHeight()** - Returns the current consensus height (last committed block).

- **GetRound()** - Returns the current consensus round. For non-BFT engines, always return 0.

- **IsValidator()** - Returns true if this node is a validator. Non-validators only receive and validate blocks.

- **ValidatorSet()** - Returns the current validator set. Returns nil if not using validators.

**Example:**

```go
type MyConsensus struct {
    deps    consensus.ConsensusDependencies
    height  int64
    running atomic.Bool
}

func (c *MyConsensus) Initialize(deps consensus.ConsensusDependencies) error {
    c.deps = deps
    c.height = deps.BlockStore.Height()
    return nil
}

func (c *MyConsensus) ProcessBlock(block *consensus.Block) error {
    // Validate block
    if err := c.validateBlock(block); err != nil {
        return err
    }

    // Execute block through application
    blockResult := c.deps.Application.ExecuteBlock(ctx, block)
    if blockResult.Code != bapitypes.CodeOK {
        return blockResult.Error
    }

    commitResult := c.deps.Application.Commit(ctx)

    // Save block
    c.deps.BlockStore.SaveBlock(block.Height, block.Hash, block.Data)
    c.height = block.Height

    // Notify node
    c.deps.Callbacks.InvokeOnBlockCommitted(block.Height, block.Hash)

    return nil
}

```text

##### BFTConsensus

```go
type BFTConsensus interface {
    ConsensusEngine
    BlockProducer

    HandleProposal(proposal *Proposal) error
    HandleVote(vote *Vote) error
    HandleCommit(commit *Commit) error
    OnTimeout(height int64, round int32, step TimeoutStep) error
}

```text

Extended interface for BFT-style consensus engines.

**Additional Methods:**

- **HandleProposal(proposal)** - Processes a block proposal from the proposer for the current height/round.

- **HandleVote(vote)** - Processes a consensus vote (prevote or precommit) from a validator.

- **HandleCommit(commit)** - Processes a commit message with aggregated signatures.

- **OnTimeout(height, round, step)** - Handles consensus timeouts for propose, prevote, or precommit steps.

##### StreamAwareConsensus

```go
type StreamAwareConsensus interface {
    ConsensusEngine

    StreamConfigs() []StreamConfig
    HandleStreamMessage(stream string, peerID peer.ID, data []byte) error
}

```text

For consensus engines needing custom network streams.

**Additional Methods:**

- **StreamConfigs()** - Returns stream configurations the engine needs. The node registers these streams and routes messages to HandleStreamMessage.

- **HandleStreamMessage(stream, peerID, data)** - Handles messages on custom streams. The stream parameter identifies which custom stream received the message.

#### Types

##### ConsensusDependencies

```go
type ConsensusDependencies struct {
    Network     Network
    BlockStore  blockstore.BlockStore
    StateStore  *statestore.StateStore
    Mempool     mempool.Mempool
    Application bapi.Lifecycle
    Callbacks   *ConsensusCallbacks
    Config      *ConsensusConfig
}

```text

Dependencies provided to consensus engines during initialization.

##### Block

```go
type Block struct {
    Height       int64
    Round        int32
    Hash         []byte
    ParentHash   []byte
    Proposer     uint16
    Timestamp    int64
    Transactions [][]byte
    Data         []byte
    LastCommit   *Commit
}

```text

Consensus block structure.

##### ValidatorSet

```go
type ValidatorSet interface {
    Count() int
    GetByIndex(index uint16) *Validator
    GetByAddress(address []byte) *Validator
    Contains(address []byte) bool
    GetProposer(height int64, round int32) *Validator
    TotalVotingPower() int64
    Quorum() int64
    F() int
    Validators() []*Validator
    VerifyCommit(height int64, blockHash []byte, commit *Commit) error
}

```text

Interface for validator sets.

**Method Details:**

- **Count()** - Returns the number of validators.
- **GetByIndex(index)** - Returns validator by index.
- **GetByAddress(address)** - Returns validator by address.
- **Contains(address)** - Checks if address is a validator.
- **GetProposer(height, round)** - Returns the proposer for given height/round.
- **TotalVotingPower()** - Returns total voting power.
- **Quorum()** - Returns quorum size (2f+1 for BFT).
- **F()** - Returns max Byzantine validators tolerated.
- **VerifyCommit(height, blockHash, commit)** - Verifies commit signatures.

---

### pkg/mempool

Package `mempool` provides transaction mempool interfaces and implementations.

#### Package Overview

Mempools store pending transactions before block inclusion. Blockberry provides multiple implementations:

- **SimpleMempool** - Basic hash-based storage (FIFO)
- **PriorityMempool** - Priority queue ordering
- **TTLMempool** - Time-based expiration
- **Looseberry** - DAG-based mempool for high throughput

All mempools use defensive copying to prevent external mutation and fail-closed validation by default.

#### Interfaces

##### Mempool

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
    TxHashes() [][]byte
    SetTxValidator(validator TxValidator)
}

```text

Main mempool interface.

**Thread Safety:** All methods must be safe for concurrent use.

**Method Details:**

- **AddTx(tx)** - Adds a transaction to the mempool. Validates using the configured validator (or default fail-closed validator if none set). Returns `ErrTxAlreadyExists`, `ErrTxTooLarge`, or `ErrMempoolFull` on rejection.

- **RemoveTxs(hashes)** - Removes transactions by their hashes. Typically called after transactions are included in a block. Silently ignores non-existent hashes.

- **ReapTxs(maxBytes)** - Returns up to maxBytes worth of transactions for block proposal. Returns defensive copies. Transactions are ordered by mempool policy (FIFO, priority, etc.).

- **HasTx(hash)** - Checks if a transaction exists without loading it. Faster than GetTx for existence checks.

- **GetTx(hash)** - Retrieves a transaction by hash. Returns defensive copy. Returns `ErrTxNotFound` if not found.

- **Size()** - Returns the number of transactions in the mempool.

- **SizeBytes()** - Returns total size in bytes of all transactions.

- **Flush()** - Removes all transactions from the mempool. Used for reset or emergency cleanup.

- **TxHashes()** - Returns all transaction hashes. Returns defensive copies.

- **SetTxValidator(validator)** - Sets the validation function for AddTx. Must be set before accepting transactions (fail-closed by default).

**Example:**

```go
// Create mempool
mp := mempool.NewSimpleMempool(5000, 1024*1024*1024)

// Set validator
mp.SetTxValidator(func(tx []byte) error {
    if len(tx) == 0 {
        return types.ErrInvalidTx
    }
    // Perform validation
    return nil
})

// Add transaction
tx := []byte("transaction data")
if err := mp.AddTx(tx); err != nil {
    log.Printf("Rejected: %v", err)
}

// Reap for block proposal
txs := mp.ReapTxs(1024 * 1024) // 1MB max
log.Printf("Reaped %d transactions", len(txs))

// Remove after commit
hashes := [][]byte{types.HashTx(tx)}
mp.RemoveTxs(hashes)

```text

##### TxValidator

```go
type TxValidator func(tx []byte) error

```text

Function type for validating transactions before mempool inclusion.

**Parameters:**

- `tx` - Transaction data to validate

**Returns:** `nil` if valid, error if invalid.

**Example:**

```go
func myValidator(tx []byte) error {
    // Check basic format
    if len(tx) == 0 || len(tx) > 1024*1024 {
        return errors.New("invalid transaction size")
    }

    // Parse and validate
    parsed, err := ParseTx(tx)
    if err != nil {
        return err
    }

    // Check signature
    if !VerifySignature(parsed) {
        return errors.New("invalid signature")
    }

    return nil
}

mp.SetTxValidator(myValidator)

```text

#### Functions

##### NewSimpleMempool

```go
func NewSimpleMempool(maxTxs int, maxBytes int64) *SimpleMempool

```text

Creates a simple FIFO mempool.

**Parameters:**

- `maxTxs` - Maximum number of transactions
- `maxBytes` - Maximum total size in bytes

**Returns:** A SimpleMempool instance.

**Example:**

```go
mp := mempool.NewSimpleMempool(5000, 1024*1024*1024) // 5000 txs, 1GB

```text

##### NewPriorityMempool

```go
func NewPriorityMempool(maxTxs int, maxBytes int64) *PriorityMempool

```text

Creates a priority-ordered mempool.

**Parameters:**

- `maxTxs` - Maximum number of transactions
- `maxBytes` - Maximum total size in bytes

**Returns:** A PriorityMempool instance that orders transactions by priority.

**Note:** Use `AddTxWithPriority` to specify priority values.

##### NewTTLMempool

```go
func NewTTLMempool(maxTxs int, maxBytes int64, ttl time.Duration) *TTLMempool

```text

Creates a TTL-based mempool with automatic expiration.

**Parameters:**

- `maxTxs` - Maximum number of transactions
- `maxBytes` - Maximum total size in bytes
- `ttl` - Time-to-live for transactions

**Returns:** A TTLMempool that automatically removes expired transactions.

---

### pkg/node

Package `node` provides the main Node type and lifecycle management.

#### Package Overview

The Node is the central coordinator that aggregates all components (network, mempool, storage, handlers) and manages their lifecycle. It wraps a Glueberry node for P2P networking and routes messages to appropriate handlers.

#### Types

##### Node

```go
type Node struct {
    // Exported fields for inspection (read-only)
}

```text

Main coordinator for a blockberry node.

**Thread Safety:** All public methods are safe for concurrent use.

#### Functions

##### NewNode

```go
func NewNode(cfg *config.Config, opts ...Option) (*Node, error)

```text

Creates a new blockberry node with the given configuration.

**Parameters:**

- `cfg` - Node configuration
- `opts` - Functional options for customization

**Returns:** A Node instance or an error.

**Error Conditions:**

- Invalid configuration
- Cannot load/generate private key
- Cannot create storage backends
- Network initialization failure

**Example:**

```go
// Load configuration
cfg, err := config.LoadConfig("config.toml")
if err != nil {
    log.Fatal(err)
}

// Create application
app := NewMyApplication()

// Create node with options
node, err := node.NewNode(cfg,
    node.WithBlockValidator(app.ValidateBlock),
    node.WithCallbacks(&types.NodeCallbacks{
        OnPeerConnected: func(id peer.ID, outbound bool) {
            log.Printf("Peer connected: %s", id)
        },
    }),
)
if err != nil {
    log.Fatal(err)
}

// Start node
if err := node.Start(); err != nil {
    log.Fatal(err)
}

// Wait for shutdown signal
<-sigterm

// Stop node
if err := node.Stop(); err != nil {
    log.Fatal(err)
}

```text

##### Node.Start

```go
func (n *Node) Start() error

```text

Starts the node and all its components.

**Returns:** Error if startup fails.

**Error Conditions:**

- Node already started
- Component initialization failure
- Network bind failure

**Startup Sequence:**

1. Start network layer
2. Start handshake handler
3. Start reactors (PEX, housekeeping, transactions, sync)
4. Start event loop
5. Connect to seed nodes

##### Node.Stop

```go
func (n *Node) Stop() error

```text

Stops the node and all its components gracefully.

**Returns:** Error if shutdown fails.

**Shutdown Sequence:**

1. Signal shutdown in progress
2. Stop accepting new messages
3. Wait for event loop to drain (with timeout)
4. Stop all reactors
5. Stop network layer
6. Close storage backends

**Timeout:** Default shutdown timeout is 5 seconds to prevent indefinite hangs.

##### Node.IsRunning

```go
func (n *Node) IsRunning() bool

```text

Returns whether the node is currently running.

##### Node.PeerID

```go
func (n *Node) PeerID() peer.ID

```text

Returns the node's libp2p peer ID.

##### Node.NodeID

```go
func (n *Node) NodeID() string

```text

Returns the node's hex-encoded public key ID.

##### Node.Role

```go
func (n *Node) Role() types.NodeRole

```text

Returns the node's role (validator, full, seed, light, or archive).

##### Node.Network

```go
func (n *Node) Network() *p2p.Network

```text

Returns the network layer for advanced use cases.

##### Node.BlockStore

```go
func (n *Node) BlockStore() blockstore.BlockStore

```text

Returns the block store.

##### Node.Mempool

```go
func (n *Node) Mempool() mempool.Mempool

```text

Returns the mempool.

##### Node.PeerCount

```go
func (n *Node) PeerCount() int

```text

Returns the number of connected peers.

#### Options

##### WithMempool

```go
func WithMempool(mp mempool.Mempool) Option

```text

Sets a custom mempool implementation.

##### WithBlockStore

```go
func WithBlockStore(bs blockstore.BlockStore) Option

```text

Sets a custom block store implementation.

##### WithBlockValidator

```go
func WithBlockValidator(v types.BlockValidator) Option

```text

Sets the block validation function. **REQUIRED** for the node to accept blocks (fail-closed behavior).

##### WithCallbacks

```go
func WithCallbacks(cb *types.NodeCallbacks) Option

```text

Sets event notification callbacks.

**Example:**

```go
callbacks := &types.NodeCallbacks{
    OnPeerConnected: func(id peer.ID, outbound bool) {
        log.Printf("Peer connected: %s (outbound=%v)", id, outbound)
    },
    OnPeerDisconnected: func(id peer.ID) {
        log.Printf("Peer disconnected: %s", id)
    },
    OnBlockCommitted: func(height int64, hash []byte) {
        log.Printf("Block committed: height=%d hash=%x", height, hash)
    },
}

node, err := node.NewNode(cfg, node.WithCallbacks(callbacks))

```text

---

### pkg/statestore

Package `statestore` provides merkleized state storage with IAVL trees.

#### Package Overview

StateStore implements a versioned, merkleized key-value store using IAVL trees. Each version represents a committed block height. Merkle proofs allow light clients to verify state without downloading the entire tree.

#### Interfaces

##### StateStore

```go
type StateStore interface {
    Get(key []byte) ([]byte, error)
    Has(key []byte) (bool, error)
    Set(key []byte, value []byte) error
    Delete(key []byte) error
    Commit() (hash []byte, version int64, err error)
    RootHash() []byte
    Version() int64
    LoadVersion(version int64) error
    GetProof(key []byte) (*Proof, error)
    Close() error
}

```text

Interface for merkleized key-value state storage.

**Thread Safety:** Methods must be called sequentially on a single goroutine. IAVL trees are not thread-safe internally.

**Method Details:**

- **Get(key)** - Retrieves the value for a key. Returns `nil, nil` if the key does not exist.

- **Has(key)** - Checks if a key exists without loading the value. Faster than Get for existence checks.

- **Set(key, value)** - Stores a key-value pair in the working tree. Changes are not persisted until Commit is called. Can be called multiple times before Commit.

- **Delete(key)** - Removes a key from the working tree. Change is not persisted until Commit.

- **Commit()** - Saves the current working tree as a new version. Returns the root hash and version number. After Commit, the working tree is reset to the new version.

- **RootHash()** - Returns the root hash of the current tree. For the working tree, this reflects uncommitted changes.

- **Version()** - Returns the latest committed version number. Returns 0 if no versions have been committed.

- **LoadVersion(version)** - Loads a specific version of the tree. All subsequent operations are based on this version. Used for historical queries.

- **GetProof(key)** - Returns a merkle proof for a key. The proof can verify either the existence or non-existence of the key at the current version.

- **Close()** - Closes the store and releases resources. Must be called before process termination.

**Example:**

```go
// Create state store
store, err := statestore.NewIAVLStore("./data/state", 10000)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

// Set values
store.Set([]byte("account:alice"), []byte("balance:1000"))
store.Set([]byte("account:bob"), []byte("balance:500"))

// Commit to persist
hash, version, err := store.Commit()
log.Printf("Committed version %d with hash %x", version, hash)

// Query
value, err := store.Get([]byte("account:alice"))
log.Printf("Alice's balance: %s", value)

// Generate proof
proof, err := store.GetProof([]byte("account:alice"))
valid, err := proof.Verify(hash)
log.Printf("Proof valid: %v", valid)

// Load historical version
store.LoadVersion(version - 1)

```text

#### Types

##### Proof

```go
type Proof struct {
    Key        []byte
    Value      []byte
    Exists     bool
    RootHash   []byte
    Version    int64
    ProofBytes []byte
}

```text

Merkle proof for a key in the state store.

**Fields:**

- `Key` - The key this proof is for
- `Value` - The value if the key exists, nil otherwise
- `Exists` - Whether the key exists in the tree
- `RootHash` - Root hash the proof was generated from
- `Version` - Version the proof was generated from
- `ProofBytes` - Serialized ICS23 commitment proof

##### Proof.Verify

```go
func (p *Proof) Verify(rootHash []byte) (bool, error)

```text

Verifies the proof against the given root hash.

**Parameters:**

- `rootHash` - Root hash to verify against

**Returns:** `true` if valid, `false` if invalid, or an error.

**Example:**

```go
proof, err := store.GetProof([]byte("key"))
valid, err := proof.Verify(store.RootHash())
if !valid {
    log.Fatal("Invalid proof")
}

```text

#### Functions

##### NewIAVLStore

```go
func NewIAVLStore(path string, cacheSize int) (*IAVLStore, error)

```text

Creates a new IAVL-backed state store.

**Parameters:**

- `path` - Directory path for LevelDB storage
- `cacheSize` - Number of nodes to cache in memory

**Returns:** An IAVLStore implementation or an error.

**Recommended Cache Size:** 10,000 for typical workloads, higher for large state trees.

---

### pkg/types

Package `types` provides common types and interfaces.

#### Package Overview

This package defines core types used throughout Blockberry: Height, Hash, Transaction, Block, and component lifecycle interfaces.

#### Types

##### Height

```go
type Height int64

```text

Represents a block height in the blockchain.

**Methods:**

- `String()` - Returns height as string
- `Int64()` - Returns height as int64

##### Hash

```go
type Hash []byte

```text

Represents a cryptographic hash (typically SHA-256, 32 bytes).

**Methods:**

- `String()` - Returns hex-encoded string
- `Bytes()` - Returns raw bytes
- `IsEmpty()` - Returns true if nil or empty
- `Equal(other Hash)` - Compares equality

**Example:**

```go
h := types.Hash(sha256.Sum256(data))
fmt.Println(h.String()) // hex output

```text

##### Tx

```go
type Tx []byte

```text

Represents an opaque transaction as raw bytes. Transaction structure is defined by the application.

**Methods:**

- `String()` - Returns truncated hex string (first 32 bytes)
- `Bytes()` - Returns raw bytes
- `Size()` - Returns size in bytes

##### Block

```go
type Block []byte

```text

Represents an opaque block as raw bytes. Block structure is defined by the application.

**Methods:**

- `String()` - Returns truncated hex string
- `Bytes()` - Returns raw bytes
- `Size()` - Returns size in bytes

##### PeerID

```go
type PeerID string

```text

Type alias for peer identification (libp2p peer ID string).

**Methods:**

- `String()` - Returns peer ID as string
- `IsEmpty()` - Returns true if empty

#### Interfaces

##### Component

```go
type Component interface {
    Start() error
    Stop() error
    IsRunning() bool
}

```text

Base interface for all pluggable components.

**Contract:**

- `Start()` must be idempotent (safe to call multiple times)
- `Stop()` must be idempotent and safe to call even if not started
- `IsRunning()` returns current state

**Example Implementation:**

```go
type MyComponent struct {
    running atomic.Bool
    stopCh  chan struct{}
}

func (c *MyComponent) Start() error {
    if c.running.Load() {
        return nil // Already started
    }
    c.stopCh = make(chan struct{})
    c.running.Store(true)
    go c.run()
    return nil
}

func (c *MyComponent) Stop() error {
    if !c.running.Load() {
        return nil // Already stopped
    }
    close(c.stopCh)
    c.running.Store(false)
    return nil
}

func (c *MyComponent) IsRunning() bool {
    return c.running.Load()
}

func (c *MyComponent) run() {
    for {
        select {
        case <-c.stopCh:
            return
        // ... component logic
        }
    }
}

```text

##### Named

```go
type Named interface {
    Name() string
}

```text

Provides a name for logging and debugging.

##### BlockValidator

```go
type BlockValidator func(height int64, hash []byte, data []byte) error

```text

Function type for validating blocks during sync.

**Parameters:**

- `height` - Block height
- `hash` - Block hash
- `data` - Block data

**Returns:** `nil` if valid, error if invalid.

**Important:** Must be set via `WithBlockValidator` option or node will reject all blocks (fail-closed).

---

## Message Protocol Reference

### Handshake Protocol

The handshake protocol establishes encrypted peer connections using a three-phase process.

#### Messages

##### HelloRequest

```go
type HelloRequest struct {
    NodeId       *string  // Hex-encoded public key
    Version      *int32   // Protocol version
    InboundUrl   string   // Optional inbound URL
    InboundPort  int32    // Optional inbound port
    ChainId      *string  // Blockchain ID
    Timestamp    *int64   // Unix timestamp
    LatestHeight *int64   // Current block height
}

```text

Initial handshake message sent when connection is established.

**Phase:** 1 (StateConnected)
**Direction:** Both peers send simultaneously
**Validation:**

- Chain ID must match or connection is rejected
- Protocol version must be compatible
- Timestamp must be within acceptable range

##### HelloResponse

```go
type HelloResponse struct {
    Accepted  *bool   // Whether connection is accepted
    PublicKey []byte  // Ed25519 public key for encryption
}

```text

Response to HelloRequest indicating acceptance.

**Phase:** 2 (after receiving HelloRequest)
**Direction:** Both peers respond
**Action:** If Accepted=true, prepare encrypted streams using PublicKey

##### HelloFinalize

```go
type HelloFinalize struct {
    Success *bool  // Finalization confirmation
}

```text

Final handshake message to activate encrypted streams.

**Phase:** 3 (after preparing streams)
**Direction:** Both peers send
**Action:** After receiving, transition to StateEstablished

#### Protocol Flow

```text
Peer A                                 Peer B
   |                                      |
   |---- HelloRequest ------------------>|
   |<--- HelloRequest --------------------|
   |                                      |
   |---- HelloResponse ------------------>|
   |<--- HelloResponse -------------------|
   |                                      |
   | [Prepare encrypted streams]          |
   |                                      |
   |---- HelloFinalize ------------------>|
   |<--- HelloFinalize -------------------|
   |                                      |
   | [StateEstablished]                   |

```text

### Peer Exchange (PEX) Protocol

Used for peer discovery and address gossiping.

#### Messages

##### AddressRequest

```go
type AddressRequest struct {
    LastSeen *int64  // Timestamp of last address update
}

```text

Requests peer addresses from a peer.

##### AddressResponse

```go
type AddressResponse struct {
    Addresses []PeerAddress  // List of known peer addresses
}

type PeerAddress struct {
    Addr     *string  // Multiaddr
    LastSeen *int64   // Last seen timestamp
}

```text

Returns known peer addresses.

### Transaction Gossip Protocol

Disseminates transactions across the network.

#### Messages

##### TransactionBroadcast

```go
type TransactionBroadcast struct {
    Transactions [][]byte  // Transaction data
}

```text

Broadcasts new transactions to peers.

##### TransactionRequest

```go
type TransactionRequest struct {
    Hashes [][]byte  // Requested transaction hashes
}

```text

Requests specific transactions by hash.

##### TransactionResponse

```go
type TransactionResponse struct {
    Transactions [][]byte  // Requested transaction data
}

```text

Returns requested transactions.

### Block Sync Protocol

Synchronizes historical blocks for catching up.

#### Messages

##### BlockRequest

```go
type BlockRequest struct {
    StartHeight *int64  // Starting block height
    EndHeight   *int64  // Ending block height (inclusive)
}

```text

Requests a range of blocks.

##### BlockResponse

```go
type BlockResponse struct {
    Blocks []BlockData  // Block data
}

type BlockData struct {
    Height *int64   // Block height
    Hash   []byte   // Block hash
    Data   []byte   // Block data
}

```text

Returns requested blocks.

### Block Propagation Protocol

Real-time block announcements for validators.

#### Messages

##### BlockAnnouncement

```go
type BlockAnnouncement struct {
    Height *int64   // Block height
    Hash   []byte   // Block hash
}

```text

Announces a new block without data.

##### NewBlock

```go
type NewBlock struct {
    Height *int64   // Block height
    Hash   []byte   // Block hash
    Data   []byte   // Block data
}

```text

Broadcasts a new block with full data.

### Housekeeping Protocol

Network health monitoring and diagnostics.

#### Messages

##### LatencyProbe

```go
type LatencyProbe struct {
    Timestamp *int64   // Send timestamp
    Nonce     *uint64  // Unique nonce
}

```text

Measures round-trip latency.

##### LatencyResponse

```go
type LatencyResponse struct {
    Nonce *uint64  // Matching nonce from probe
}

```text

Response to latency probe.

---

## Error Reference

### Common Errors

#### Peer Errors

```go
var (
    ErrPeerNotFound         = errors.New("peer not found")
    ErrPeerBlacklisted      = errors.New("peer is blacklisted")
    ErrPeerAlreadyConnected = errors.New("peer already connected")
    ErrMaxPeersReached      = errors.New("maximum peers reached")
)

```text

#### Block Errors

```go
var (
    ErrBlockNotFound       = errors.New("block not found")
    ErrBlockAlreadyExists  = errors.New("block already exists")
    ErrInvalidBlock        = errors.New("invalid block")
    ErrInvalidBlockHeight  = errors.New("invalid block height")
    ErrInvalidBlockHash    = errors.New("invalid block hash")
)

```text

#### Transaction Errors

```go
var (
    ErrTxNotFound      = errors.New("transaction not found")
    ErrTxAlreadyExists = errors.New("transaction already exists")
    ErrInvalidTx       = errors.New("invalid transaction")
    ErrTxTooLarge      = errors.New("transaction too large")
)

```text

#### Mempool Errors

```go
var (
    ErrMempoolFull   = errors.New("mempool is full")
    ErrMempoolClosed = errors.New("mempool is closed")
)

```text

#### Connection Errors

```go
var (
    ErrChainIDMismatch   = errors.New("chain ID mismatch")
    ErrVersionMismatch   = errors.New("protocol version mismatch")
    ErrHandshakeFailed   = errors.New("handshake failed")
    ErrHandshakeTimeout  = errors.New("handshake timeout")
    ErrConnectionClosed  = errors.New("connection closed")
    ErrNotConnected      = errors.New("not connected")
)

```text

#### State Errors

```go
var (
    ErrKeyNotFound   = errors.New("key not found")
    ErrStoreClosed   = errors.New("store is closed")
    ErrInvalidProof  = errors.New("invalid proof")
)

```text

#### Sync Errors

```go
var (
    ErrAlreadySyncing       = errors.New("already syncing")
    ErrNotSyncing           = errors.New("not syncing")
    ErrSyncFailed           = errors.New("sync failed")
    ErrNoBlockValidator     = errors.New("block validator is required")
    ErrNonContiguousBlock   = errors.New("non-contiguous block")
)

```text

### ABI Result Codes

```go
const (
    CodeOK                uint32 = 0  // Success
    CodeInvalidTx         uint32 = 1  // Transaction format invalid
    CodeInsufficientFunds uint32 = 2  // Insufficient balance
    CodeNotAuthorized     uint32 = 3  // Not authorized
    CodeExecutionFailed   uint32 = 4  // Execution failed
    CodeInvalidNonce      uint32 = 5  // Invalid nonce
    CodeInvalidSignature  uint32 = 6  // Invalid signature
)

```text

---

## Configuration Reference

### Complete Example

```toml
[node]
chain_id = "blockberry-mainnet-1"
protocol_version = 1
private_key_path = "config/node_key.json"

[network]
listen_addrs = [
    "/ip4/0.0.0.0/tcp/26656",
    "/ip6/::/tcp/26656"
]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
dial_timeout = "3s"
address_book_path = "data/addrbook.json"

[network.seeds]
addrs = [
    "/dns4/seed1.blockberry.io/tcp/26656/p2p/12D3KooWPeerID1",
    "/dns4/seed2.blockberry.io/tcp/26656/p2p/12D3KooWPeerID2"
]

# Node role: validator, full, seed, light, or archive
role = "full"

[handlers.transactions]
request_interval = "5s"
batch_size = 100
max_pending = 1000
max_pending_age = "60s"

[handlers.blocks]
max_block_size = 22020096  # 21 MB

[handlers.sync]
sync_interval = "5s"
batch_size = 100
max_pending_batches = 10

[pex]
enabled = true
request_interval = "30s"
max_addresses_per_response = 100
max_total_addresses = 1000

[statesync]
enabled = false
trust_height = 0
trust_hash = ""
discovery_interval = "10s"
chunk_request_timeout = "30s"
max_chunk_retries = 3
snapshot_path = "data/snapshots"

[mempool]
type = "simple"  # simple, priority, ttl, looseberry
max_txs = 5000
max_bytes = 1073741824  # 1 GB
cache_size = 10000
ttl = "1h"              # For ttl mempool
cleanup_interval = "5m" # For ttl mempool

[blockstore]
backend = "leveldb"  # leveldb or badgerdb
path = "data/blockstore"

[statestore]
path = "data/state"
cache_size = 10000

[housekeeping]
latency_probe_interval = "60s"

[metrics]
enabled = false
namespace = "blockberry"
listen_addr = ":9090"

[logging]
level = "info"      # debug, info, warn, error
format = "text"     # text or json
output = "stderr"   # stdout, stderr, or file path

[limits]
max_tx_size = 1048576        # 1 MB
max_block_size = 22020096    # 21 MB
max_block_txs = 10000
max_msg_size = 10485760      # 10 MB
max_subscribers = 1000
max_subscribers_per_query = 100

```text

### Configuration Sections

#### Node Configuration

- **chain_id** (required) - Unique blockchain identifier. Must match across all nodes.
- **protocol_version** (required) - Protocol version for compatibility checks. Peers with different versions are rejected.
- **private_key_path** (required) - Path to Ed25519 private key. Generated automatically if missing.

#### Network Configuration

- **listen_addrs** (required) - Multiaddrs for incoming connections. Supports IPv4, IPv6, TCP, QUIC.
- **max_inbound_peers** (default: 40) - Maximum incoming connections.
- **max_outbound_peers** (default: 10) - Maximum outgoing connections.
- **handshake_timeout** (default: 30s) - Handshake completion deadline.
- **dial_timeout** (default: 3s) - Connection dialing deadline.
- **address_book_path** (default: addrbook.json) - Peer address persistence.

#### Mempool Configuration

- **type** (default: simple) - Mempool implementation: simple, priority, ttl, looseberry
- **max_txs** (default: 5000) - Maximum transactions
- **max_bytes** (default: 1GB) - Maximum total size
- **cache_size** (default: 10000) - Recent transaction cache
- **ttl** (default: 1h) - TTL mempool expiration
- **cleanup_interval** (default: 5m) - TTL mempool cleanup frequency

#### Storage Configuration

- **blockstore.backend** (default: leveldb) - leveldb or badgerdb
- **blockstore.path** (default: data/blockstore) - Block storage directory
- **statestore.path** (default: data/state) - State storage directory
- **statestore.cache_size** (default: 10000) - IAVL cache size

#### Limits Configuration

Resource limits prevent denial of service attacks:

- **max_tx_size** (default: 1MB) - Maximum single transaction size
- **max_block_size** (default: 21MB) - Maximum block size
- **max_block_txs** (default: 10000) - Maximum transactions per block
- **max_msg_size** (default: 10MB) - Maximum P2P message size
- **max_subscribers** (default: 1000) - Maximum event subscribers
- **max_subscribers_per_query** (default: 100) - Maximum subscribers per query

---

## Usage Examples

### Basic Node Setup

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/blockberries/blockberry/pkg/config"
    "github.com/blockberries/blockberry/pkg/node"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("config.toml")
    if err != nil {
        log.Fatalf("Config error: %v", err)
    }

    // Create and start node
    n, err := node.NewNode(cfg)
    if err != nil {
        log.Fatalf("Node creation failed: %v", err)
    }

    if err := n.Start(); err != nil {
        log.Fatalf("Node start failed: %v", err)
    }

    log.Printf("Node started with ID: %s", n.NodeID())

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    // Graceful shutdown
    log.Println("Shutting down...")
    if err := n.Stop(); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}

```text

### Custom Application

```go
package main

import (
    "context"
    "github.com/blockberries/bapi"
    bapitypes "github.com/blockberries/bapi/types"
)

type MyApp struct {
    *bapi.BaseApplication
    state map[string]string
}

func NewMyApp() *MyApp {
    return &MyApp{
        BaseApplication: &bapi.BaseApplication{},
        state:          make(map[string]string),
    }
}

func (app *MyApp) Info() bapitypes.ApplicationInfo {
    return bapitypes.ApplicationInfo{
        Name:    "my-app",
        Version: "1.0.0",
    }
}

func (app *MyApp) CheckTx(ctx context.Context, tx *bapitypes.Transaction) *bapitypes.TxCheckResult {
    // Validate transaction
    if len(tx.Data) == 0 {
        return &bapitypes.TxCheckResult{
            Code:  bapitypes.CodeInvalidTx,
            Error: errors.New("empty transaction"),
        }
    }
    return &bapitypes.TxCheckResult{Code: bapitypes.CodeOK}
}

func (app *MyApp) ExecuteBlock(ctx context.Context, block *bapitypes.Block) *bapitypes.BlockResult {
    // Parse and execute all transactions in block
    for _, txData := range block.Txs {
        key, value := parseTx(txData)
        app.state[key] = value
    }

    return &bapitypes.BlockResult{
        Code: bapitypes.CodeOK,
        Events: []bapitypes.Event{
            {Type: "block.executed"},
        },
    }
}

func (app *MyApp) Commit(ctx context.Context) *bapitypes.CommitResult {
    // Compute state hash
    hash := computeStateHash(app.state)
    return &bapitypes.CommitResult{AppHash: hash}
}

func (app *MyApp) Query(ctx context.Context, req *bapitypes.QueryRequest) *bapitypes.QueryResponse {
    value, exists := app.state[string(req.Data)]
    if !exists {
        return &bapitypes.QueryResponse{
            Code:  bapitypes.CodeNotFound,
            Error: errors.New("key not found"),
        }
    }
    return &bapitypes.QueryResponse{
        Code: bapitypes.CodeOK,
        Data: []byte(value),
    }
}

```text

---

**Documentation Generated:** 2026-02-02
**Blockberry Version:** 1.0
**Framework:** github.com/blockberries/blockberry

For more information, visit: https://github.com/blockberries/blockberry
