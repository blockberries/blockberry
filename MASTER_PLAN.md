# Blockberry Master Plan

This document supersedes `ROADMAP.md` and provides a comprehensive plan to transform blockberry into a fully pluggable blockchain node framework built around the **Application Blockchain Interface (ABI) v2.0**. The ABI is the central contract between the blockchain framework and application logic, providing clean separation of concerns and maximum flexibility.

See **ABI_DESIGN.md** for the complete ABI v2.0 specification.

---

## Vision

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         Blockchain Application                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                     APPLICATION INTERFACE (ABI)                         ││
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐   ││
│  │  │ CheckTx     │ │ BeginBlock  │ │ ExecuteTx   │ │ Query           │   ││
│  │  │ Commit      │ │ EndBlock    │ │ InitChain   │ │ Info            │   ││
│  │  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────────┘   ││
│  │  ┌─────────────────────────────────────────────────────────────────┐   ││
│  │  │              Extended Interfaces (Optional)                      │   ││
│  │  │  Snapshot · Proposer · Finality · VoteExtension                  │   ││
│  │  └─────────────────────────────────────────────────────────────────┘   ││
│  └─────────────────────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                         BLOCKBERRY CORE                                 ││
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────────┐   ││
│  │  │ MempoolEngine │  │ConsensusEngine│  │ Component Lifecycle       │   ││
│  │  │ (Pluggable)   │  │ (Pluggable)   │  │ (Start/Stop/IsRunning)    │   ││
│  │  └───────────────┘  └───────────────┘  └───────────────────────────┘   ││
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────────┐   ││
│  │  │ BlockStore    │  │ StateStore    │  │ EventBus                  │   ││
│  │  │ (Pluggable)   │  │ (IAVL)        │  │ (Pub/Sub Events)          │   ││
│  │  └───────────────┘  └───────────────┘  └───────────────────────────┘   ││
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────────────────┐   ││
│  │  │ Network       │  │ PeerScorer    │  │ CallbackRegistry          │   ││
│  │  │ (Glueberry)   │  │ (Reputation)  │  │ (Extension Points)        │   ││
│  │  └───────────────┘  └───────────────┘  └───────────────────────────┘   ││
│  └─────────────────────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                         GLUEBERRY (P2P Layer)                           ││
│  │              Encrypted Streams · Peer Discovery · Handshake             ││
│  └─────────────────────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                         CRAMBERRY (Serialization)                       ││
│  │              Binary Protocol · Polymorphic Messages · Code Gen          ││
│  └─────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## ABI-Centric Architecture

The ABI (Application Blockchain Interface) v2.0 is the **central contract** between the blockchain framework and applications. All major components implement ABI interfaces.

**Important: ABI is the ONLY interface.** There is no backward compatibility with legacy interfaces. Applications MUST implement `abi.Application` directly. This simplifies the codebase, eliminates adapter complexity, and ensures all applications benefit from ABI v2.0's improved design.

| Layer | ABI Interface | Purpose |
|-------|---------------|---------|
| Application | `Application` | Core app logic (CheckTx, ExecuteTx, Commit, Query) |
| Application | `SnapshotApplication` | State sync via snapshots |
| Application | `ProposerApplication` | Block proposal customization |
| Application | `FinalityApplication` | Vote extensions and finality |
| Mempool | `MempoolEngine` | Transaction pool management |
| Mempool | `DAGMempoolEngine` | DAG-based ordering (Looseberry) |
| Consensus | `ConsensusEngine` | Consensus algorithm plugin |
| Consensus | `BFTConsensusEngine` | BFT-specific consensus |
| Storage | `BlockStore` | Block persistence |
| Storage | `StateStore` | Application state |
| Events | `EventBus` | Pub/sub event system |
| Lifecycle | `Component` | Start/Stop/IsRunning lifecycle |

---

## Table of Contents

1. [Critical Fixes (Phase 0)](#phase-0-critical-fixes)
2. [ABI Core Types (Phase 1)](#phase-1-abi-core-types)
3. [Application Interface (Phase 2)](#phase-2-application-interface)
4. [Pluggable Architecture (Phase 3)](#phase-3-pluggable-architecture)
5. [MempoolEngine System (Phase 4)](#phase-4-mempoolengine-system)
6. [ConsensusEngine Framework (Phase 5)](#phase-5-consensusengine-framework)
7. [Node Role System (Phase 6)](#phase-6-node-role-system)
8. [Dynamic Stream Registry (Phase 7)](#phase-7-dynamic-stream-registry)
9. [Storage Enhancements (Phase 8)](#phase-8-storage-enhancements)
10. [Event System (Phase 9)](#phase-9-event-system)
11. [Extended Application Interfaces (Phase 10)](#phase-10-extended-application-interfaces)
12. [Developer Experience (Phase 11)](#phase-11-developer-experience)
13. [Security Hardening (Phase 12)](#phase-12-security-hardening)
14. [Performance & Observability (Phase 13)](#phase-13-performance--observability)
15. [Ecosystem Integration (Phase 14)](#phase-14-ecosystem-integration)
16. [Version Milestones](#version-milestones)

---

## Phase 0: Critical Fixes

**Priority: CRITICAL**
**Status: ✅ COMPLETE (January 27, 2026)**

These issues must be fixed before any other work. They represent security vulnerabilities, reliability issues, or design flaws that could cause production failures.

### Progress Summary - ALL ITEMS COMPLETE

| Item | Status | Verification |
|------|--------|--------------|
| 0.1 Block Validation | ✅ **COMPLETE** | `sync/reactor.go:41-43, 125-149` - Fail-closed DefaultBlockValidator |
| 0.2 Transaction Validation | ✅ **COMPLETE** | `mempool/ttl_mempool.go:198-205` - Fail-closed TxValidator |
| 0.3 Block Height Continuity | ✅ **COMPLETE** | `sync/reactor.go` - Contiguity validation with peer penalties |
| 0.4 Pending Requests Cleanup | ✅ **COMPLETE** | `handlers/transactions.go:167-170` - Timeout cleanup |
| 0.5 Node Shutdown Race | ✅ **COMPLETE** | `node/node.go` - Atomic stopping flag + shutdown timeout |
| 0.6 Handshake Timeout | ✅ **COMPLETE** | `handlers/handshake.go:155-191` - Timeout loop |
| 0.7 Constant-Time Hash | ✅ **COMPLETE** | `types/hash.go:55-60` - `crypto/subtle.ConstantTimeCompare()` |
| 0.8 Penalty Persistence | ✅ **COMPLETE** | `p2p/scoring.go:63-70, 238-250` - Wall-clock decay |

### Additional Critical Fixes from Code Review - ALL FIXED

| Issue | Status | Notes |
|-------|--------|-------|
| O(n²) bubble sort in ReapTxs | ✅ **FIXED** | `mempool/ttl_mempool.go:293-313` - Heap-based sorting |
| Handshake state race condition | ✅ **FIXED** | `handlers/handshake.go:77-78` - Mutex protection |
| Block hash collision detection | ✅ **FIXED** | `blockstore/leveldb.go:84-92` - Collision check |

### Remaining Operational Improvements (MEDIUM Priority)

These are NOT blocking issues - they are operational conveniences:

| Issue | Priority | Notes |
|-------|----------|-------|
| PEX total address limit | MEDIUM | Has `MaxAddressesPerResponse` but no total limit. Low risk. |
| Softer blacklist policy | MEDIUM | Uses permanent blacklist for chain/version mismatch. Operational issue only. |

---

## Phase 1: ABI Core Types

**Priority: CRITICAL**
**Status: ✅ COMPLETE (January 2026)**

Implement all core ABI types that form the foundation of the interface contract. These types are used throughout the system by all components.

### 1.1 Result Codes and Errors

```go
// abi/codes.go

// ResultCode for transaction processing.
type ResultCode uint32

const (
    CodeOK                  ResultCode = 0
    CodeUnknownError        ResultCode = 1
    CodeInvalidTx           ResultCode = 2
    CodeInsufficientFunds   ResultCode = 3
    CodeInvalidNonce        ResultCode = 4
    CodeInsufficientGas     ResultCode = 5
    CodeNotAuthorized       ResultCode = 6
    CodeInvalidState        ResultCode = 7
    // 100-999: App-specific codes
)

func (c ResultCode) IsOK() bool { return c == CodeOK }
func (c ResultCode) String() string
```

**Tasks:**
- [x] Create `abi/codes.go` with ResultCode enum
- [x] Add helper methods (IsOK, String)
- [x] Document error code ranges for applications

### 1.2 Transaction Types

```go
// abi/transaction.go

// Transaction represents a blockchain transaction.
type Transaction struct {
    Hash      []byte            // SHA-256 hash
    Data      []byte            // Raw transaction bytes (opaque to framework)
    Sender    []byte            // Optional: extracted sender (for dedup)
    Nonce     uint64            // Optional: sequence number (for ordering)
    GasLimit  uint64            // Optional: max computation units
    GasPrice  uint64            // Optional: price per unit
    Priority  int64             // Derived priority (higher = more important)
    Metadata  map[string]any    // App-populated metadata
}

// CheckTxMode indicates the context of transaction validation.
type CheckTxMode int

const (
    CheckTxNew       CheckTxMode = iota  // New transaction from client/peer
    CheckTxRecheck                        // Re-validate after block commit
    CheckTxRecovery                       // Recovery from crash/restart
)

// TxCheckResult is returned from CheckTx.
type TxCheckResult struct {
    Code         ResultCode
    Error        error
    GasWanted    uint64
    Priority     int64
    Sender       []byte
    Nonce        uint64
    Data         []byte           // Optional: modified transaction
    RecheckAfter uint64           // Suggest re-check after this height
    ExpireAfter  time.Duration    // Suggest expiration time
}

// TxExecResult is returned from ExecuteTx during block execution.
type TxExecResult struct {
    Code         ResultCode
    Error        error
    GasUsed      uint64
    Events       []Event
    Data         []byte           // Return data
    StateChanges []StateChange    // For indexing
}
```

**Tasks:**
- [x] Create `abi/transaction.go`
- [x] Implement Transaction struct with all fields
- [x] Implement TxCheckResult and TxExecResult
- [x] Add validation methods

### 1.3 Block Types

```go
// abi/block.go

// BlockHeader contains block metadata passed to BeginBlock.
type BlockHeader struct {
    Height          uint64
    Time            time.Time
    PrevHash        []byte
    ProposerAddress []byte
    ConsensusData   []byte      // Opaque to application
    Evidence        []Evidence  // Misbehavior evidence
    LastValidators  []Validator
}

// EndBlockResult contains application responses to block finalization.
type EndBlockResult struct {
    ValidatorUpdates []ValidatorUpdate
    ConsensusParams  *ConsensusParams
    Events           []Event
}

// CommitResult contains the result of committing state.
type CommitResult struct {
    AppHash      []byte
    RetainHeight uint64   // Suggested minimum height to retain
}
```

**Tasks:**
- [x] Create `abi/block.go`
- [x] Implement BlockHeader, EndBlockResult, CommitResult
- [x] Add Evidence and ValidatorUpdate types

### 1.4 Event Types

```go
// abi/event.go

// Event represents an application event.
type Event struct {
    Type       string
    Attributes []Attribute
}

type Attribute struct {
    Key   string
    Value []byte
    Index bool   // Should this be indexed?
}

// Common event types
const (
    EventNewBlock             = "NewBlock"
    EventNewBlockHeader       = "NewBlockHeader"
    EventTx                   = "Tx"
    EventVote                 = "Vote"
    EventCommit               = "Commit"
    EventValidatorSetUpdates  = "ValidatorSetUpdates"
    EventEvidence             = "Evidence"
    EventTxAdded              = "TxAdded"
    EventTxRemoved            = "TxRemoved"
    EventTxRejected           = "TxRejected"
    EventPeerConnected        = "PeerConnected"
    EventPeerDisconnected     = "PeerDisconnected"
    EventSyncStarted          = "SyncStarted"
    EventSyncProgress         = "SyncProgress"
    EventSyncCompleted        = "SyncCompleted"
)
```

**Tasks:**
- [x] Create `abi/event.go`
- [x] Define Event and Attribute types
- [x] Define standard event type constants

### 1.5 Query Types

```go
// abi/query.go

// QueryRequest for reading application state.
type QueryRequest struct {
    Path    string    // Query path (e.g., "/accounts/{address}")
    Data    []byte    // Query-specific data
    Height  uint64    // Historical height (0 = latest)
    Prove   bool      // Include Merkle proof
}

// QueryResponse from Query.
type QueryResponse struct {
    Code    ResultCode
    Error   error
    Key     []byte
    Value   []byte
    Proof   *Proof    // Merkle proof (if requested)
    Height  uint64    // Height at which query was executed
}

// Proof represents a Merkle proof.
type Proof struct {
    Ops []ProofOp
}

type ProofOp struct {
    Type string
    Key  []byte
    Data []byte
}
```

**Tasks:**
- [x] Create `abi/query.go`
- [x] Implement QueryRequest and QueryResponse
- [x] Implement Proof types for Merkle proofs

### 1.6 Genesis and Initialization Types

```go
// abi/genesis.go

// Genesis contains chain initialization data.
type Genesis struct {
    ChainID         string
    GenesisTime     time.Time
    ConsensusParams *ConsensusParams
    Validators      []Validator
    AppState        []byte   // App-specific genesis state (opaque)
}

// ApplicationInfo provides metadata about the application.
type ApplicationInfo struct {
    Name     string
    Version  string
    AppHash  []byte
    Height   uint64
    Metadata map[string]any
}
```

**Tasks:**
- [x] Create `abi/genesis.go`
- [x] Implement Genesis and ApplicationInfo types

### 1.7 Validator Types

```go
// abi/validator.go

// Validator represents a consensus validator.
type Validator struct {
    Address     []byte
    PublicKey   []byte
    VotingPower int64
    Index       uint16
}

// ValidatorUpdate represents a change to the validator set.
type ValidatorUpdate struct {
    PublicKey   []byte
    Power       int64   // 0 = remove validator
}

// ValidatorSet represents the set of validators.
type ValidatorSet interface {
    Validators() []*Validator
    GetByIndex(index uint16) *Validator
    GetByAddress(addr []byte) *Validator
    Count() int
    TotalVotingPower() int64
    Proposer(height uint64, round uint32) *Validator
    VerifySignature(validatorIdx uint16, digest, sig []byte) bool
    Quorum() int64   // 2f+1
    F() int          // Max Byzantine validators
    Epoch() uint64
    Copy() ValidatorSet
}
```

**Tasks:**
- [x] Create `abi/validator.go`
- [x] Implement Validator and ValidatorUpdate types
- [x] Define ValidatorSet interface

---

## Phase 2: Application Interface

**Priority: CRITICAL**
**Status: ✅ COMPLETE (January 2026)**

Implement the core Application interface - the primary contract between the blockchain framework and application logic.

### 2.1 Core Application Interface

```go
// abi/application.go

// Application is the main interface applications must implement.
// All methods are invoked by the framework; applications respond.
type Application interface {
    // Lifecycle
    Info() ApplicationInfo
    InitChain(genesis *Genesis) error

    // Transaction Processing
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult

    // Block Execution (strict ordering)
    BeginBlock(ctx context.Context, block *BlockHeader) error
    ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult
    EndBlock(ctx context.Context) *EndBlockResult
    Commit(ctx context.Context) *CommitResult

    // State Access
    Query(ctx context.Context, req *QueryRequest) *QueryResponse
}
```

**Tasks:**
- [x] Create `abi/application.go`
- [x] Define Application interface
- [x] Document method contracts and ordering guarantees

### 2.2 Default Application Implementation

```go
// abi/base_application.go

// BaseApplication provides default implementations that reject all operations.
// This follows the fail-closed security principle.
type BaseApplication struct{}

func (app *BaseApplication) Info() ApplicationInfo {
    return ApplicationInfo{Name: "base", Version: "0.0.0"}
}

func (app *BaseApplication) InitChain(genesis *Genesis) error {
    return nil
}

func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("CheckTx not implemented"),
    }
}

// ... other methods with fail-closed defaults
```

**Tasks:**
- [x] Create `abi/base_application.go`
- [x] Implement fail-closed defaults for all methods
- [x] Add documentation about fail-closed principle

### 2.3 Application Wrapper

```go
// abi/wrapper.go

// ApplicationWrapper adds metrics, logging, and validation to any Application.
type ApplicationWrapper struct {
    app     Application
    metrics Metrics
    logger  Logger
}

func WrapApplication(app Application, opts ...WrapperOption) *ApplicationWrapper

func (w *ApplicationWrapper) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    start := time.Now()
    defer func() {
        w.metrics.AppCheckTx(time.Since(start), result.Code == CodeOK)
    }()

    // Validate transaction basic structure
    if err := tx.ValidateBasic(); err != nil {
        return &TxCheckResult{Code: CodeInvalidTx, Error: err}
    }

    return w.app.CheckTx(ctx, tx)
}
```

**Tasks:**
- [x] Create `abi/wrapper.go`
- [x] Implement ApplicationWrapper with metrics
- [x] Add logging for all methods
- [x] Add basic validation before delegation

### 2.4 Node-Application Integration

```go
// node/app_executor.go

// AppExecutor manages the application lifecycle and execution.
type AppExecutor struct {
    app         Application
    blockStore  BlockStore
    stateStore  StateStore
    eventBus    EventBus
    mu          sync.RWMutex
}

// ExecuteBlock runs the full block execution cycle.
func (e *AppExecutor) ExecuteBlock(ctx context.Context, block *Block) (*BlockResult, error) {
    e.mu.Lock()
    defer e.mu.Unlock()

    // BeginBlock
    if err := e.app.BeginBlock(ctx, &block.Header); err != nil {
        return nil, fmt.Errorf("BeginBlock failed: %w", err)
    }

    // Execute transactions
    var txResults []*TxExecResult
    for _, tx := range block.Txs {
        result := e.app.ExecuteTx(ctx, tx)
        txResults = append(txResults, result)
        e.eventBus.Publish(ctx, Event{Type: EventTx, Attributes: txToAttrs(tx, result)})
    }

    // EndBlock
    endResult := e.app.EndBlock(ctx)

    // Commit
    commitResult := e.app.Commit(ctx)

    return &BlockResult{
        TxResults:        txResults,
        ValidatorUpdates: endResult.ValidatorUpdates,
        AppHash:          commitResult.AppHash,
    }, nil
}
```

**Tasks:**
- [x] Create `node/app_executor.go`
- [x] Implement ExecuteBlock with proper ordering
- [x] Add event publishing for all operations
- [x] Add crash recovery via WAL

---

## Phase 3: Pluggable Architecture

**Priority: HIGH**
**Status: ✅ COMPLETE (January 2026)**

Establish the foundational patterns for pluggable components using ABI's Component interface.

### 3.1 Component Interface (from ABI)

```go
// abi/component.go

// Component is the base interface for all manageable components.
type Component interface {
    Start() error
    Stop() error
    IsRunning() bool
}

// Named components can report their name.
type Named interface {
    Name() string
}

// HealthChecker components can report health status.
type HealthChecker interface {
    Health() HealthStatus
}

type HealthStatus struct {
    Status    Health
    Message   string
    Details   map[string]any
    Timestamp time.Time
}

type Health int
const (
    HealthUnknown Health = iota
    HealthHealthy
    HealthDegraded
    HealthUnhealthy
)

// Dependent components declare their dependencies.
type Dependent interface {
    Dependencies() []string
}

// LifecycleAware components receive lifecycle notifications.
type LifecycleAware interface {
    OnStart() error
    OnStop() error
}

// Resettable components can be reset to initial state.
type Resettable interface {
    Reset() error
}
```

**Tasks:**
- [x] Create `abi/component.go` with ABI component interfaces
- [x] Refactor all reactors to implement `Component`
- [x] Add lifecycle hooks for startup/shutdown coordination

### 3.2 Service Registry (from ABI)

```go
// abi/registry.go

// ServiceRegistry manages component lifecycle and dependencies.
type ServiceRegistry interface {
    Register(name string, component Component) error
    Get(name string) (Component, error)
    MustGet(name string) Component
    StartAll() error
    StopAll() error
    Health() map[string]HealthStatus
}
```

**Tasks:**
- [x] Create `abi/registry.go` with ServiceRegistry interface
- [x] Implement topological sort for dependency ordering
- [x] Add circular dependency detection
- [x] Integrate with Node startup/shutdown

### 3.3 Callback Registry (from ABI)

```go
// abi/callbacks.go

// CallbackRegistry manages application callbacks.
type CallbackRegistry struct {
    // Transaction callbacks
    OnTxReceived   func(peerID []byte, tx *Transaction)
    OnTxValidated  func(tx *Transaction, result *TxCheckResult)
    OnTxAdded      func(tx *Transaction)
    OnTxRemoved    func(hash []byte, reason TxRemovalReason)
    OnTxBroadcast  func(tx *Transaction, peerCount int)

    // Block callbacks
    OnBlockProposed    func(height uint64, hash []byte, txCount int)
    OnBlockReceived    func(peerID []byte, height uint64, hash []byte)
    OnBlockValidated   func(height uint64, hash []byte, err error)
    OnBlockCommitted   func(height uint64, hash []byte, appHash []byte)

    // Consensus callbacks
    OnConsensusStateChange func(oldState, newState ConsensusState)
    OnRoundChange          func(height uint64, round uint32)
    OnVoteReceived         func(vote *Vote)
    OnProposalReceived     func(proposal *Proposal)

    // Validator callbacks
    OnValidatorSetChange func(height uint64, validators []Validator)
    OnSlashingEvent      func(validator []byte, reason string, amount uint64)

    // Network callbacks
    OnPeerConnected    func(peerID []byte, isOutbound bool)
    OnPeerDisconnected func(peerID []byte)
    OnPeerScoreChanged func(peerID []byte, oldScore, newScore int)
}

type TxRemovalReason int
const (
    TxRemovalCommitted TxRemovalReason = iota
    TxRemovalExpired
    TxRemovalReplaced
    TxRemovalInvalid
    TxRemovalEvicted
)
```

**Tasks:**
- [x] Create `abi/callbacks.go`
- [x] Implement CallbackRegistry with all callback types
- [x] Add nil-check wrappers for safe invocation
- [x] Replace direct handler calls with callback invocations

### 3.4 Configuration Overhaul

```go
// config/config.go

type Config struct {
    // Core configuration
    Node     NodeConfig     `toml:"node"`
    Network  NetworkConfig  `toml:"network"`

    // Role-specific configuration
    Role     NodeRole       `toml:"role"`  // validator, full, seed, light

    // Pluggable component configuration (ABI-aligned)
    Mempool   MempoolEngineConfig   `toml:"mempool"`
    Consensus ConsensusEngineConfig `toml:"consensus"`

    // Storage configuration
    BlockStore BlockStoreConfig `toml:"blockstore"`
    StateStore StateStoreConfig `toml:"statestore"`

    // Stream configuration (for plugins that add streams)
    Streams  []StreamConfig  `toml:"streams"`
}

type MempoolEngineConfig struct {
    Type string `toml:"type"`  // "simple", "priority", "ttl", "dag", "custom"
    // Type-specific configuration
    Simple     *SimpleMempoolConfig     `toml:"simple,omitempty"`
    Priority   *PriorityMempoolConfig   `toml:"priority,omitempty"`
    TTL        *TTLMempoolConfig        `toml:"ttl,omitempty"`
    DAG        *DAGMempoolConfig        `toml:"dag,omitempty"`
}

type ConsensusEngineConfig struct {
    Type string `toml:"type"`  // "none", "bft", "dag", "custom"
    BFT  *BFTConsensusConfig  `toml:"bft,omitempty"`
    DAG  *DAGConsensusConfig  `toml:"dag,omitempty"`
}
```

**Tasks:**
- [x] Restructure `config/config.go` with ABI-aligned schema
- [x] Add `NodeRole` enum with validation
- [x] Add per-plugin configuration sections
- [x] Update all examples with new configuration

---

## Phase 4: MempoolEngine System

**Priority: HIGH**
**Status: ✅ COMPLETE (January 2026)**

Implement ABI's MempoolEngine interface system for pluggable transaction pools.

### 4.1 MempoolEngine Interface (from ABI)

```go
// abi/mempool.go

// MempoolEngine is the interface for transaction pool implementations.
type MempoolEngine interface {
    Component

    // AddTx adds a transaction to the mempool.
    AddTx(tx *Transaction, callback TxCallback) error

    // RemoveTxs removes transactions (after commit).
    RemoveTxs(hashes [][]byte)

    // ReapTxs returns transactions for block proposal.
    ReapTxs(opts ReapOptions) []*Transaction

    // Size returns current mempool state.
    Size() MempoolSize

    // HasTx checks if transaction exists.
    HasTx(hash []byte) bool

    // Flush removes all transactions.
    Flush()

    // Update is called after a block is committed.
    Update(height uint64, txs []*Transaction) error

    // SetTxValidator sets the transaction validation function.
    SetTxValidator(validator TxValidator)
}

type TxCallback func(result *TxCheckResult)

type TxValidator func(ctx context.Context, tx *Transaction) *TxCheckResult

type ReapOptions struct {
    MaxBytes    int64
    MaxTxs      int
    MinPriority int64
}

type MempoolSize struct {
    TxCount   int
    ByteCount int64
}
```

**Tasks:**
- [x] Create `abi/mempool.go` with MempoolEngine interface
- [x] Add TxCallback, TxValidator types
- [x] Add ReapOptions and MempoolSize types

### 4.2 DAGMempoolEngine Interface (from ABI)

```go
// abi/mempool_dag.go

// DAGMempoolEngine extends MempoolEngine for DAG-based ordering (Looseberry).
type DAGMempoolEngine interface {
    MempoolEngine

    // ReapCertifiedBatches returns ordered, certified transaction batches.
    ReapCertifiedBatches(maxBytes int64) []*CertifiedBatch

    // NotifyCommitted informs the DAG of committed rounds.
    NotifyCommitted(round uint64)

    // UpdateValidatorSet updates the set of validators for certification.
    UpdateValidatorSet(validators []Validator)

    // CurrentRound returns the current DAG round.
    CurrentRound() uint64

    // DAGMetrics returns DAG-specific metrics.
    DAGMetrics() *DAGMetrics
}

// CertifiedBatch represents an ordered batch with quorum certification.
type CertifiedBatch struct {
    Round       uint64
    Validator   []byte
    Txs         []*Transaction
    Certificate *Certificate
}

// Certificate proves batch ordering via quorum signatures.
type Certificate struct {
    Round      uint64
    BatchHash  []byte
    Signatures []ValidatorSig
}

type DAGMetrics struct {
    CurrentRound     uint64
    CommittedRound   uint64
    PendingBatches   int
    CertifiedBatches int
    ValidatorCount   int
}
```

**Tasks:**
- [x] Create `abi/mempool_dag.go` with DAGMempoolEngine interface
- [x] Add CertifiedBatch, Certificate types
- [x] Add DAGMetrics type

### 4.3 NetworkAwareMempoolEngine Interface (from ABI)

```go
// abi/mempool_network.go

// NetworkAwareMempoolEngine can declare custom network streams.
type NetworkAwareMempoolEngine interface {
    MempoolEngine

    // StreamConfigs returns stream configurations for the network layer.
    StreamConfigs() []StreamConfig

    // SetNetwork provides the network interface.
    SetNetwork(network MempoolNetwork)

    // HandleStreamMessage processes messages from custom streams.
    HandleStreamMessage(stream string, peerID []byte, data []byte) error
}

// MempoolNetwork provides network operations to the mempool.
type MempoolNetwork interface {
    Broadcast(stream string, data []byte) error
    Send(peerID []byte, stream string, data []byte) error
    PeerCount() int
    ValidatorPeers() [][]byte
}
```

**Tasks:**
- [x] Create `abi/mempool_network.go`
- [x] Implement NetworkAwareMempoolEngine interface
- [x] Implement MempoolNetwork interface

### 4.4 Mempool Factory

```go
// mempool/factory.go

type MempoolFactory struct {
    registry map[string]MempoolConstructor
}

type MempoolConstructor func(cfg *MempoolEngineConfig) (MempoolEngine, error)

func NewMempoolFactory() *MempoolFactory {
    f := &MempoolFactory{registry: make(map[string]MempoolConstructor)}
    f.Register("simple", NewSimpleMempoolEngine)
    f.Register("priority", NewPriorityMempoolEngine)
    f.Register("ttl", NewTTLMempoolEngine)
    return f
}

func (f *MempoolFactory) Register(name string, constructor MempoolConstructor)
func (f *MempoolFactory) Create(cfg *MempoolEngineConfig) (MempoolEngine, error)
```

**Tasks:**
- [x] Create `mempool/factory.go`
- [x] Register all built-in mempools
- [x] Add external mempool registration API
- [x] Integrate factory with Node builder

### 4.5 Looseberry Adapter

```go
// mempool/looseberry/adapter.go

// LooseberryAdapter adapts looseberry to ABI's DAGMempoolEngine interface.
type LooseberryAdapter struct {
    lb          *looseberry.Looseberry
    network     MempoolNetwork
    txValidator TxValidator
    running     atomic.Bool
}

func (a *LooseberryAdapter) AddTx(tx *Transaction, callback TxCallback) error
func (a *LooseberryAdapter) ReapCertifiedBatches(maxBytes int64) []*CertifiedBatch
func (a *LooseberryAdapter) NotifyCommitted(round uint64)
func (a *LooseberryAdapter) UpdateValidatorSet(validators []Validator)
func (a *LooseberryAdapter) StreamConfigs() []StreamConfig
func (a *LooseberryAdapter) SetNetwork(network MempoolNetwork)
```

**Tasks:**
- [x] Create `mempool/looseberry/adapter.go`
- [x] Implement all ABI mempool interfaces
- [x] Create `LooseberryNetworkAdapter` for glueberry
- [x] Add looseberry stream registration

---

## Phase 5: ConsensusEngine Framework

**Priority: HIGH**
**Status: ✅ COMPLETE (January 2026)**

Implement ABI's ConsensusEngine interface system for pluggable consensus algorithms.

### 5.1 ConsensusEngine Interface (from ABI)

```go
// abi/consensus.go

// ConsensusEngine is the interface for consensus implementations.
type ConsensusEngine interface {
    Component
    Named

    // Initialize sets up the engine with its dependencies.
    Initialize(deps ConsensusDependencies) error

    // State queries
    Height() uint64
    Round() uint32
    State() ConsensusState

    // Validator info
    IsValidator() bool
    ValidatorSet() ValidatorSet

    // Message handling
    HandleMessage(peerID []byte, msgType ConsensusMessageType, data []byte) error
}

type ConsensusDependencies struct {
    Network     ConsensusNetwork
    BlockStore  BlockStore
    StateStore  StateStore
    Mempool     MempoolEngine
    Application Application
    Signer      Signer
    Config      *ConsensusConfig
    Logger      Logger
    Metrics     Metrics
}

type ConsensusState int
const (
    ConsensusStateIdle ConsensusState = iota
    ConsensusStateProposing
    ConsensusStatePrevoting
    ConsensusStatePrecommitting
    ConsensusStateCommitting
    ConsensusStateCatchup
)

type ConsensusMessageType int
const (
    MsgTypeProposal ConsensusMessageType = iota
    MsgTypePrevote
    MsgTypePrecommit
    MsgTypeCommit
    MsgTypeBlock
    MsgTypeBlockPart
    MsgTypeVoteSetBits
    MsgTypeNewRoundStep
    MsgTypeHasVote
)
```

**Tasks:**
- [x] Create `abi/consensus.go` with ConsensusEngine interface
- [x] Define ConsensusDependencies struct
- [x] Define ConsensusState enum
- [x] Define ConsensusMessageType enum

### 5.2 BFTConsensusEngine Interface (from ABI)

```go
// abi/consensus_bft.go

// BFTConsensusEngine extends ConsensusEngine for BFT protocols.
type BFTConsensusEngine interface {
    ConsensusEngine

    // Block production
    ProposeBlock(ctx context.Context) (*BlockProposal, error)

    // Message handling
    HandleProposal(proposal *Proposal) error
    HandlePrevote(vote *Vote) error
    HandlePrecommit(vote *Vote) error
    HandleCommit(commit *Commit) error

    // Timeout handling
    OnTimeout(height uint64, round uint32, step TimeoutStep) error

    // Evidence
    ReportMisbehavior(evidence Evidence) error
}

type BlockProposal struct {
    Height    uint64
    Round     uint32
    Block     *Block
    POLRound  int32   // Proof-of-Lock round (-1 if none)
    Signature []byte
}

type Proposal struct {
    Type      ConsensusMessageType
    Height    uint64
    Round     uint32
    BlockHash []byte
    POLRound  int32
    Timestamp time.Time
    Signature []byte
}

type Vote struct {
    Type             ConsensusMessageType  // Prevote or Precommit
    Height           uint64
    Round            uint32
    BlockHash        []byte     // nil = nil vote
    Timestamp        time.Time
    ValidatorAddress []byte
    ValidatorIndex   int32
    Signature        []byte
    Extension        []byte     // Vote extension (optional)
    ExtensionSig     []byte
}

type Commit struct {
    Height     uint64
    Round      uint32
    BlockHash  []byte
    Signatures []CommitSig
}

type TimeoutStep int
const (
    TimeoutPropose TimeoutStep = iota
    TimeoutPrevote
    TimeoutPrecommit
)
```

**Tasks:**
- [x] Create `abi/consensus_bft.go`
- [x] Implement BFTConsensusEngine interface
- [x] Define Proposal, Vote, Commit types
- [x] Define timeout types

### 5.3 DAGConsensusEngine Interface (from ABI)

```go
// abi/consensus_dag.go

// DAGConsensusEngine extends ConsensusEngine for DAG-based protocols.
type DAGConsensusEngine interface {
    ConsensusEngine

    // DAG operations
    AddVertex(vertex *DAGVertex) error
    GetVertex(hash []byte) (*DAGVertex, error)
    GetVerticesByRound(round uint64) []*DAGVertex

    // Certificate operations
    HandleCertificate(cert *Certificate) error
    GetCertificate(round uint64, validator []byte) (*Certificate, error)

    // Ordering
    GetOrderedVertices(fromRound, toRound uint64) []*DAGVertex
    CommittedRound() uint64

    // Wave/Leader operations (for Tusk/Bullshark)
    GetWaveLeader(wave uint64) ([]byte, error)
    IsLeaderVertex(vertex *DAGVertex) bool
}

type DAGVertex struct {
    Round     uint64
    Source    []byte           // Validator that created this
    Parents   [][]byte         // Hashes of parent vertices
    Payload   []byte           // Transaction batch or other data
    Timestamp time.Time
    Signature []byte
    Hash      []byte           // Computed hash
}
```

**Tasks:**
- [x] Create `abi/consensus_dag.go`
- [x] Implement DAGConsensusEngine interface
- [x] Define DAGVertex type

### 5.4 StreamAwareConsensusEngine (from ABI)

```go
// abi/consensus_stream.go

// StreamAwareConsensusEngine can declare custom network streams.
type StreamAwareConsensusEngine interface {
    ConsensusEngine

    // StreamConfigs returns stream configurations.
    StreamConfigs() []StreamConfig

    // HandleStreamMessage processes messages from custom streams.
    HandleStreamMessage(stream string, peerID []byte, data []byte) error
}
```

**Tasks:**
- [x] Create `abi/consensus_stream.go`
- [x] Implement StreamAwareConsensusEngine interface

### 5.5 Consensus Factory

```go
// consensus/factory.go

type ConsensusFactory struct {
    registry map[string]ConsensusConstructor
}

type ConsensusConstructor func(cfg *ConsensusEngineConfig) (ConsensusEngine, error)

func NewConsensusFactory() *ConsensusFactory {
    f := &ConsensusFactory{registry: make(map[string]ConsensusConstructor)}
    f.Register("none", NewNullConsensusEngine)
    return f
}
```

**Tasks:**
- [x] Create `consensus/factory.go`
- [x] Create `NullConsensusEngine` (no-op for full nodes)
- [x] Add external engine registration API

### 5.6 Reference BFT Implementation Skeleton

```go
// consensus/bft/tendermint.go

// TendermintBFT is a reference Tendermint-style BFT implementation.
type TendermintBFT struct {
    cfg           *BFTConsensusConfig
    height        uint64
    round         uint32
    step          RoundStep
    lockedValue   *Block
    lockedRound   int32
    validValue    *Block
    validRound    int32
    validatorSet  ValidatorSet
    privValidator PrivValidator
    prevotes      *HeightVoteSet
    precommits    *HeightVoteSet
    deps          ConsensusDependencies
    wal           *WAL
    // Timeouts and lifecycle...
}
```

**Tasks:**
- [x] Create `consensus/bft/tendermint.go` skeleton
- [x] Define round step state machine
- [x] Define vote tracking structures
- [x] Document as reference for custom implementations

---

## Phase 6: Node Role System

**Priority: HIGH**
**Status: ✅ COMPLETE (January 2026)**

Implement a comprehensive node role system using ABI patterns.

### 6.1 Node Role Definitions

```go
// abi/role.go

// NodeRole defines the role of a node in the network.
type NodeRole string

const (
    RoleValidator NodeRole = "validator"  // Consensus-participating validator
    RoleFull      NodeRole = "full"       // Full node (stores all data)
    RoleSeed      NodeRole = "seed"       // Seed node for peer discovery
    RoleLight     NodeRole = "light"      // Light client (headers only)
    RoleArchive   NodeRole = "archive"    // Full node that never prunes
)

// RoleCapabilities defines what each role can do.
type RoleCapabilities struct {
    CanPropose                bool
    CanVote                   bool
    StoresFullBlocks          bool
    StoresState               bool
    ParticipatesInPEX         bool
    GossipsTransactions       bool
    AcceptsInboundConnections bool
    Prunes                    bool
}

var roleCapabilities = map[NodeRole]RoleCapabilities{
    RoleValidator: {
        CanPropose: true, CanVote: true,
        StoresFullBlocks: true, StoresState: true,
        ParticipatesInPEX: true, GossipsTransactions: false,  // Uses DAG
        AcceptsInboundConnections: true, Prunes: true,
    },
    RoleFull: {
        CanPropose: false, CanVote: false,
        StoresFullBlocks: true, StoresState: true,
        ParticipatesInPEX: true, GossipsTransactions: true,
        AcceptsInboundConnections: true, Prunes: true,
    },
    // ... other roles
}
```

**Tasks:**
- [x] Create `abi/role.go` with role definitions
- [x] Define capabilities per role
- [x] Add role validation

### 6.2 Role-Based Component Selection

```go
// node/role_builder.go

type RoleBasedBuilder struct {
    cfg        *Config
    role       NodeRole
    components map[string]Component
}

func (b *RoleBasedBuilder) Build() (*Node, error) {
    caps := roleCapabilities[b.role]

    // Create mempool based on role
    var mempool MempoolEngine
    if caps.GossipsTransactions {
        mempool = b.createSimpleMempoolEngine()
    } else if caps.CanPropose {
        mempool = b.createDAGMempoolEngine()
    }

    // Create consensus engine based on role
    var consensus ConsensusEngine
    if caps.CanVote {
        consensus = b.createConsensusEngine()
    } else {
        consensus = NewNullConsensusEngine()
    }

    // ... create other components based on role
}
```

**Tasks:**
- [x] Create `node/role_builder.go`
- [x] Implement role-based component selection
- [x] Add role-specific startup/shutdown logic

---

## Phase 7: Dynamic Stream Registry

**Priority: HIGH**
**Status: ✅ COMPLETE (January 2026)**

Create a dynamic stream registry for plugin stream registration using ABI patterns.

### 7.1 Stream Configuration (from ABI)

```go
// abi/stream.go

type StreamConfig struct {
    Name           string
    Direction      StreamDirection
    Priority       int
    BufferSize     int
    RateLimit      int   // Messages per second
    MaxMessageSize int
}

type StreamDirection int
const (
    StreamBidirectional StreamDirection = iota
    StreamInbound
    StreamOutbound
)
```

**Tasks:**
- [x] Create `abi/stream.go` with StreamConfig
- [x] Implement stream registry
- [x] Add glueberry integration

---

## Phase 8: Storage Enhancements

**Priority: MEDIUM**
**Status: ✅ COMPLETE (January 2026)**

Enhance storage capabilities using ABI's storage interfaces.

### 8.1 BlockStore Interface (from ABI)

```go
// abi/storage.go

// BlockStore persists blocks and provides retrieval.
type BlockStore interface {
    Component

    // Save a block and its metadata.
    SaveBlock(block *Block, parts *PartSet, commit *Commit) error

    // Load operations
    LoadBlock(height uint64) (*Block, error)
    LoadBlockMeta(height uint64) (*BlockMeta, error)
    LoadBlockPart(height uint64, index int) (*Part, error)
    LoadBlockCommit(height uint64) (*Commit, error)

    // Height queries
    Base() uint64    // Lowest available height
    Height() uint64  // Highest stored height

    // Pruning
    PruneBlocks(retainHeight uint64) (uint64, error)
}
```

**Tasks:**
- [x] Block pruning (`blockstore/pruning.go`)
- [x] State pruning (`statestore/pruning.go`)
- [x] State snapshots (`statestore/snapshot.go`)
- [x] BadgerDB backend (`blockstore/badgerdb.go`)

### 8.2 StateStore Interface (from ABI)

```go
// abi/storage.go

// StateStore manages consensus and application state.
type StateStore interface {
    Component

    // Consensus state
    LoadState() (*ConsensusState, error)
    SaveState(state *ConsensusState) error

    // Validator sets
    LoadValidators(height uint64) (*ValidatorSet, error)
    SaveValidators(height uint64, validators *ValidatorSet) error

    // Consensus params
    LoadConsensusParams(height uint64) (*ConsensusParams, error)
    SaveConsensusParams(height uint64, params *ConsensusParams) error

    // Pruning
    PruneStates(retainHeight uint64) error
}
```

**Tasks:**
- [x] Implement StateStore interface
- [x] Add IAVL pruning
- [x] Add background pruning

### 8.3 EvidenceStore Interface (from ABI)

```go
// abi/storage.go

// EvidenceStore tracks misbehavior evidence.
type EvidenceStore interface {
    Component

    AddEvidence(evidence Evidence) error
    HasEvidence(evidence Evidence) bool
    PendingEvidence(maxBytes int64) []Evidence
    MarkEvidenceCommitted(evidence Evidence) error
    IsExpired(evidence Evidence, currentHeight uint64, currentTime time.Time) bool
}
```

**Tasks:**
- [x] Create EvidenceStore interface
- [x] Implement LevelDB-based evidence store

---

## Phase 9: Event System

**Priority: MEDIUM**
**Status: ✅ COMPLETE (January 2026)**

Implement ABI's EventBus for pub/sub event handling.

### 9.1 EventBus Interface (from ABI)

```go
// abi/eventbus.go

// EventBus provides pub/sub for system events.
type EventBus interface {
    Component

    // Subscribe to events matching the query.
    Subscribe(ctx context.Context, subscriber string, query Query) (<-chan Event, error)

    // Unsubscribe from events.
    Unsubscribe(ctx context.Context, subscriber string, query Query) error

    // UnsubscribeAll removes all subscriptions for a subscriber.
    UnsubscribeAll(ctx context.Context, subscriber string) error

    // Publish sends an event to all matching subscribers.
    Publish(ctx context.Context, event Event) error

    // PublishWithTimeout publishes with a timeout for slow subscribers.
    PublishWithTimeout(ctx context.Context, event Event, timeout time.Duration) error
}

// Query for filtering events.
type Query interface {
    Matches(event Event) bool
    String() string
}
```

**Tasks:**
- [x] Create `abi/eventbus.go` with EventBus interface
- [x] Implement in-memory EventBus (`events/bus.go`)
- [x] Add query language for filtering (QueryAll, QueryEventType, QueryEventTypes, QueryAttribute, QueryAttributeExists, QueryAnd, QueryOr, QueryFunc)
- [x] Add WebSocket transport for external subscribers (rpc/websocket package)

---

## Phase 10: Extended Application Interfaces

**Priority: MEDIUM**
**Status: ✅ COMPLETE (January 2026)**

Implement ABI's extended application interfaces for advanced functionality.

### 10.1 SnapshotApplication Interface (from ABI)

```go
// abi/application_snapshot.go

// SnapshotApplication supports state sync via snapshots.
type SnapshotApplication interface {
    Application

    ListSnapshots() []*Snapshot
    LoadSnapshotChunk(height uint64, format uint32, chunk uint32) ([]byte, error)
    OfferSnapshot(snapshot *Snapshot) OfferResult
    ApplySnapshotChunk(chunk []byte, index uint32) ApplyResult
}

type Snapshot struct {
    Height   uint64
    Format   uint32
    Chunks   uint32
    Hash     []byte
    Metadata []byte
}

type OfferResult uint32
const (
    OfferAccept OfferResult = iota
    OfferAbort
    OfferReject
    OfferRejectFormat
    OfferRejectSender
)

type ApplyResult uint32
const (
    ApplyAccept ApplyResult = iota
    ApplyAbort
    ApplyRetry
    ApplyRetrySnapshot
    ApplyRejectSnapshot
)
```

**Tasks:**
- [x] Create `abi/application_snapshot.go`
- [x] Implement SnapshotApplication interface
- [x] Add snapshot sync reactor

### 10.2 ProposerApplication Interface (from ABI)

```go
// abi/application_proposer.go

// ProposerApplication supports custom block proposal logic.
type ProposerApplication interface {
    Application

    // PrepareProposal allows the app to modify a proposed block.
    PrepareProposal(ctx context.Context, req *PrepareRequest) *PrepareResponse

    // ProcessProposal allows the app to accept/reject a received proposal.
    ProcessProposal(ctx context.Context, req *ProcessRequest) *ProcessResponse
}

type PrepareRequest struct {
    MaxTxBytes      int64
    Txs             [][]byte
    LocalLastCommit CommitInfo
    Misbehavior     []Misbehavior
    Height          uint64
    Time            time.Time
    ProposerAddress []byte
}

type PrepareResponse struct {
    Txs [][]byte
}

type ProcessRequest struct {
    Txs                [][]byte
    ProposedLastCommit CommitInfo
    Misbehavior        []Misbehavior
    Hash               []byte
    Height             uint64
    Time               time.Time
    ProposerAddress    []byte
}

type ProcessResponse struct {
    Status ProcessStatus
}

type ProcessStatus uint32
const (
    ProcessAccept ProcessStatus = iota
    ProcessReject
)
```

**Tasks:**
- [x] Create `abi/application_proposer.go`
- [x] Implement ProposerApplication interface
- [ ] Integrate with consensus engine (deferred to consensus implementation)

### 10.3 FinalityApplication Interface (from ABI)

```go
// abi/application_finality.go

// FinalityApplication supports extended finality information.
type FinalityApplication interface {
    Application

    ExtendVote(ctx context.Context, req *ExtendVoteRequest) *ExtendVoteResponse
    VerifyVoteExtension(ctx context.Context, req *VerifyVoteExtRequest) *VerifyVoteExtResponse
    FinalizeBlock(ctx context.Context, req *FinalizeBlockRequest) *FinalizeBlockResponse
}

type ExtendVoteRequest struct {
    Hash   []byte
    Height uint64
    Time   time.Time
}

type ExtendVoteResponse struct {
    VoteExtension []byte
}

type VerifyVoteExtRequest struct {
    Hash             []byte
    ValidatorAddress []byte
    Height           uint64
    VoteExtension    []byte
}

type VerifyVoteExtResponse struct {
    Status VerifyStatus
}

type VerifyStatus uint32
const (
    VerifyAccept VerifyStatus = iota
    VerifyReject
)

type FinalizeBlockRequest struct {
    Txs             [][]byte
    DecidedLastCommit CommitInfo
    Misbehavior     []Misbehavior
    Hash            []byte
    Height          uint64
    Time            time.Time
    ProposerAddress []byte
}

type FinalizeBlockResponse struct {
    Events           []Event
    TxResults        []TxExecResult
    ValidatorUpdates []ValidatorUpdate
    ConsensusParams  *ConsensusParams
    AppHash          []byte
}
```

**Tasks:**
- [x] Create `abi/application_finality.go`
- [x] Implement FinalityApplication interface
- [ ] Add vote extension support to consensus engine (deferred to consensus implementation)

---

## Phase 11: Developer Experience

**Priority: MEDIUM**
**Status: 🔄 IN PROGRESS**

Improve developer experience with tooling, documentation, and APIs.

### 11.1 RPC API (using ABI types)

```go
// rpc/server.go

type RPCServer interface {
    Component

    Health() (*HealthStatus, error)
    Status() (*NodeStatus, error)
    BroadcastTx(tx []byte, mode BroadcastMode) (*BroadcastResult, error)
    Query(path string, data []byte, height uint64) (*QueryResponse, error)  // ABI type
    Block(height uint64) (*Block, error)
    Tx(hash []byte) (*TxExecResult, error)  // ABI type
    Peers() ([]*PeerInfo, error)
    Subscribe(query string) (<-chan Event, error)  // ABI Event type
}
```

**Tasks:**
- [x] Define RPC interface using ABI types (`rpc/server.go`, `rpc/types.go`)
- [x] Implement JSON-RPC 2.0 transport (`rpc/jsonrpc/server.go`, `rpc/jsonrpc/types.go`)
- [ ] Implement gRPC transport (deferred)
- [ ] Add authentication and rate limiting (deferred)

### 11.2 CLI Tooling ✅ COMPLETE

```bash
blockberry init --chain-id mychain --moniker mynode
blockberry start --config config.toml
blockberry status
blockberry keys generate
blockberry keys show <key-file>
```

**Tasks:**
- [x] Create CLI framework using cobra (`cmd/blockberry/`)
- [x] Implement `init` command - Initialize new node with config and keys
- [x] Implement `start` command - Start node with configuration
- [x] Implement `status` command - Query running node status via RPC
- [x] Implement `keys` commands - Key generation and display
- [x] Implement `version` command - Display version information
- [ ] Implement transaction commands (future enhancement)
- [ ] Implement query commands (future enhancement)

### 11.3 Transaction Indexing

```go
// abi/indexer.go

type TxIndexer interface {
    Index(tx *TxExecResult) error      // ABI type
    Get(hash []byte) (*TxExecResult, error)
    Search(query string) ([]*TxExecResult, error)
    Delete(hash []byte) error
}
```

**Tasks:**
- [x] Define TxIndexer interface using ABI types (`abi/indexer.go`)
- [x] Implement NullTxIndexer no-op implementation
- [x] Implement LevelDB-based indexer (`indexer/kv/indexer.go`)
- [x] Add search query language (height queries, event queries with pagination)

---

## Phase 12: Security Hardening

**Priority: MEDIUM**
**Status: 🔄 IN PROGRESS**

Address security issues using ABI's security patterns.

### 12.1 Fail-Closed Defaults (from ABI)

```go
// All validators and handlers default to rejecting (ABI principle).

// DefaultTxValidator rejects all transactions
func DefaultTxValidator(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("no validator configured"),
    }
}
```

**Tasks:**
- [x] Audit all validation points for fail-closed behavior (completed in Phase 0)
- [x] Add eclipse attack mitigation (`security/eclipse.go`)
- [x] Implement enhanced rate limiting (`security/ratelimit.go`)
- [ ] Add private key encryption (deferred)

### 12.2 Resource Limits (from ABI)

```go
// abi/limits.go

type ResourceLimits struct {
    MaxTxSize       int64
    MaxBlockSize    int64
    MaxEvidenceSize int64
    MaxMsgSize      int64
    MaxPeers        int
    MaxSubscribers  int
}
```

**Tasks:**
- [x] Implement configurable resource limits (`abi/limits.go`)
- [ ] Add limit enforcement throughout (partial - interfaces defined)

---

## Phase 13: Performance & Observability

**Priority: MEDIUM**
**Status: 🔄 IN PROGRESS (January 27, 2026)**

Optimize performance using ABI's observability interfaces.

### 13.1 Metrics Interface (from ABI) ✅ COMPLETE

The `abi.Metrics` interface has been implemented in `abi/metrics.go` with:
- Consensus metrics (height, round, step, block committed, block size)
- Mempool metrics (size, tx added/removed/rejected)
- Application metrics (BeginBlock, ExecuteTx, EndBlock, Commit, Query, CheckTx)
- Network metrics (peers, bytes sent/received, messages, errors)
- Storage metrics (block store height/size, state store operations)
- Sync metrics (progress, blocks received, duration)

The `metrics.ABIMetricsAdapter` in `metrics/abi_adapter.go` bridges the existing Prometheus implementation to the new ABI interface.

**Tasks:**
- [x] Implement ABI Metrics interface (`abi/metrics.go`)
- [x] Add NullMetrics no-op implementation
- [x] Add Prometheus exporter adapter (`metrics/abi_adapter.go`)
- [ ] Add parallel block sync
- [ ] Add memory optimization

### 13.2 Tracer Interface (from ABI) ✅ COMPLETE

The `abi.Tracer` interface has been implemented in `abi/tracer.go` with:
- `Tracer` interface for distributed tracing
- `Span` interface for individual trace spans
- `SpanContext` for trace propagation
- `Carrier` interface for context injection/extraction
- `NullTracer` no-op implementation
- Standard span names and attribute keys

**Tasks:**
- [x] Implement ABI Tracer interface (`abi/tracer.go`)
- [x] Add NullTracer no-op implementation
- [x] Add SpanAttribute helpers
- [x] Add MapCarrier for context propagation
- [x] Add OpenTelemetry integration (tracing/otel package)
- [ ] Add Jaeger/Zipkin export

---

## Phase 14: Ecosystem Integration

**Priority: LOW**
**Status: ⬜ PENDING**

Integrate with the broader blockchain ecosystem.

### 14.1 ABCI Interoperability (Optional)

For applications that want to expose an ABCI-compatible interface to external tools (e.g., Tendermint tooling, ABCI-aware clients), an optional ABCI server can wrap the ABI Application.

**Note:** This is for EXTERNAL interoperability only. Within blockberry, all applications implement `abi.Application` directly - there is no legacy support.

| ABCI v1 | ABI v2 |
|---------|--------|
| `CheckTx(RequestCheckTx)` | `CheckTx(ctx, *Transaction)` |
| `BeginBlock(RequestBeginBlock)` | `BeginBlock(ctx, *BlockHeader)` |
| `DeliverTx(RequestDeliverTx)` | `ExecuteTx(ctx, *Transaction)` |
| `EndBlock(RequestEndBlock)` | `EndBlock(ctx)` |
| `Commit()` | `Commit(ctx)` |
| `Query(RequestQuery)` | `Query(ctx, *QueryRequest)` |

**Tasks:**
- [ ] Implement optional ABCI server for external tool compatibility
- [ ] Add IBC compatibility
- [ ] Add light client protocol

---

## Version Milestones

### v1.1.0 - Critical Fixes & Stability ✅ COMPLETE
**Status: ✅ COMPLETE (January 2026)**

All Phase 0 critical fixes completed.

### v2.0.0 - ABI Foundation ✅ COMPLETE
**Status: ✅ COMPLETE (January 2026)**

- ✅ ABI Core Types (Phase 1)
- ✅ Application Interface (Phase 2)
- ✅ Pluggable Architecture (Phase 3)

### v2.1.0 - Pluggable Engines ✅ COMPLETE
**Status: ✅ COMPLETE (January 2026)**

- ✅ MempoolEngine System (Phase 4)
- ✅ ConsensusEngine Framework (Phase 5)
- ✅ Node Role System (Phase 6)

### v2.2.0 - Infrastructure ✅ COMPLETE
**Status: ✅ COMPLETE (January 2026)**

- ✅ Dynamic Stream Registry (Phase 7)
- ✅ Storage Enhancements (Phase 8)

### v3.0.0 - Events & Extended Interfaces
**Status: ✅ COMPLETE (January 2026)**

- ✅ Event System (Phase 9)
- ✅ Extended Application Interfaces (Phase 10)
  - SnapshotApplication
  - ProposerApplication
  - FinalityApplication

### v3.1.0 - Developer Experience
**Status: ✅ COMPLETE (January 2026)**

- ✅ RPC Server interface (ABI types)
- ✅ JSON-RPC 2.0 transport
- ✅ TxIndexer interface with NullTxIndexer
- ✅ LevelDB-based KV indexer (indexer/kv package)
- ✅ CLI tooling (cmd/blockberry - init, start, status, keys, version)
- ⬜ gRPC transport (not implemented - user opted out of protobuf)

### v3.2.0 - Security & Performance
**Status: 🔄 IN PROGRESS (January 2026)**

- ✅ ResourceLimits types and configuration
- ✅ Rate limiting (TokenBucket + SlidingWindow)
- ✅ Connection limiting
- ✅ Eclipse attack mitigation
- ✅ Observability - ABI Metrics interface + Prometheus adapter
- ✅ Observability - ABI Tracer interface + NullTracer
- ✅ OpenTelemetry integration (tracing/otel package)
- ⬜ Performance optimization (deferred)

### v4.0.0 - Ecosystem Integration
**Status: ⬜ PENDING**

- ABCI compatibility
- IBC support
- Light client protocol

---

## Example: Building a Blockchain with ABI

```go
package main

import (
    "context"

    "github.com/blockberries/blockberry"
    "github.com/blockberries/blockberry/abi"
    "github.com/blockberries/blockberry/consensus"
    "github.com/blockberries/blockberry/mempool"
    "github.com/blockberries/looseberry"
)

// MyApp implements the ABI Application interface.
type MyApp struct {
    abi.BaseApplication  // Provides fail-closed defaults
    state   map[string][]byte
    height  uint64
    appHash []byte
}

func (app *MyApp) Info() abi.ApplicationInfo {
    return abi.ApplicationInfo{
        Name:    "myapp",
        Version: "1.0.0",
        AppHash: app.appHash,
        Height:  app.height,
    }
}

func (app *MyApp) CheckTx(ctx context.Context, tx *abi.Transaction) *abi.TxCheckResult {
    // Validate transaction
    if len(tx.Data) == 0 {
        return &abi.TxCheckResult{Code: abi.CodeInvalidTx}
    }
    return &abi.TxCheckResult{Code: abi.CodeOK, Priority: 1}
}

func (app *MyApp) ExecuteTx(ctx context.Context, tx *abi.Transaction) *abi.TxExecResult {
    // Execute transaction
    return &abi.TxExecResult{Code: abi.CodeOK}
}

func (app *MyApp) Commit(ctx context.Context) *abi.CommitResult {
    app.height++
    app.appHash = computeHash(app.state)
    return &abi.CommitResult{AppHash: app.appHash}
}

func main() {
    cfg, _ := blockberry.LoadConfig("config.toml")

    builder := blockberry.NewNodeBuilder(cfg)

    // Configure based on role
    if cfg.Role == abi.RoleValidator {
        // Validators use DAGMempoolEngine (Looseberry)
        lb, _ := looseberry.New(&looseberry.Config{
            ValidatorIndex: cfg.DAG.ValidatorIndex,
            Signer:         loadSigner(cfg),
        })
        builder.WithMempool(mempool.NewLooseberryAdapter(lb))

        // Validators use BFTConsensusEngine
        bft := consensus.NewTendermintBFT(&cfg.Consensus.BFT)
        builder.WithConsensus(bft)
    } else {
        // Full nodes use simple MempoolEngine with gossip
        builder.WithMempool(mempool.NewSimpleMempoolEngine(&cfg.Mempool.Simple))

        // Full nodes use NullConsensusEngine (follow only)
        builder.WithConsensus(consensus.NewNullConsensusEngine())
    }

    // Set ABI Application
    builder.WithApplication(&MyApp{})

    // Build and start
    node, _ := builder.Build()
    node.Start()

    // Wait for shutdown
    <-signalChan
    node.Stop()
}
```

---

## Contributing

When working on this plan:

1. **Reference ABI_DESIGN.md** - All interfaces must align with ABI v2.0 specification
2. **Reference this document** in commits and PRs (e.g., "Phase 1.2: ABI Transaction Types")
3. **Follow CLAUDE.md guidelines** for implementation workflow
4. **Update PROGRESS_REPORT.md** after completing each item
5. **Use ABI types throughout** - Never create parallel types when ABI types exist
6. **Maintain fail-closed defaults** - All validators reject by default
7. **No backward compatibility** - ABI is the only interface; do not create legacy adapters or shims

---

*Last Updated: January 27, 2026 - Comprehensive ABI-centric restructuring*
