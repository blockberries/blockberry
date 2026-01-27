# Blockberry Master Plan

This document supersedes `ROADMAP.md` and provides a comprehensive plan to transform blockberry into a fully pluggable blockchain node framework. The goal is to enable combining **cramberry** (serialization), **glueberry** (networking), **looseberry** (DAG mempool), **blockberry** (node framework), and a **pluggable consensus engine** to build production blockchain applications.

---

## Vision

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Blockchain Application                           │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │
│  │ Pluggable       │  │ Pluggable       │  │ Application Logic       │  │
│  │ Consensus       │  │ Mempool         │  │ (State Machine)         │  │
│  │ (BFT, PoS, etc) │  │ (Looseberry,    │  │                         │  │
│  │                 │  │  Simple, etc)   │  │                         │  │
│  └────────┬────────┘  └────────┬────────┘  └────────────┬────────────┘  │
│           │                    │                        │               │
│  ┌────────┴────────────────────┴────────────────────────┴────────────┐  │
│  │                         BLOCKBERRY                                 │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │  │
│  │  │ P2P      │  │ Block    │  │ State    │  │ Stream Registry  │   │  │
│  │  │ Manager  │  │ Store    │  │ Store    │  │ (Dynamic Streams)│   │  │
│  │  └────┬─────┘  └──────────┘  └──────────┘  └──────────────────┘   │  │
│  └───────┼───────────────────────────────────────────────────────────┘  │
│          │                                                               │
│  ┌───────┴───────────────────────────────────────────────────────────┐  │
│  │                         GLUEBERRY                                  │  │
│  │              (Encrypted P2P with Dynamic Streams)                  │  │
│  └───────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                         CRAMBERRY                                  │  │
│  │              (Binary Serialization with Polymorphism)              │  │
│  └───────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Table of Contents

1. [Critical Fixes (Phase 0)](#phase-0-critical-fixes)
2. [Pluggable Architecture Foundation (Phase 1)](#phase-1-pluggable-architecture-foundation)
3. [Mempool Plugin System (Phase 2)](#phase-2-mempool-plugin-system)
4. [Consensus Engine Framework (Phase 3)](#phase-3-consensus-engine-framework)
5. [Node Role System (Phase 4)](#phase-4-node-role-system)
6. [Dynamic Stream Registry (Phase 5)](#phase-5-dynamic-stream-registry)
7. [Storage Enhancements (Phase 6)](#phase-6-storage-enhancements)
8. [Developer Experience (Phase 7)](#phase-7-developer-experience)
9. [Security Hardening (Phase 8)](#phase-8-security-hardening)
10. [Performance & Observability (Phase 9)](#phase-9-performance--observability)
11. [Ecosystem Integration (Phase 10)](#phase-10-ecosystem-integration)
12. [Version Milestones](#version-milestones)

---

## Phase 0: Critical Fixes

**Priority: CRITICAL**
**Effort: 1-2 weeks**

These issues must be fixed before any other work. They represent security vulnerabilities, reliability issues, or design flaws that could cause production failures.

### 0.1 Mandatory Block Validation

**Issue:** Blocks are accepted without validation unless an external `BlockValidator` is set (sync/reactor.go:443). This is fail-open, not fail-closed.

**Fix:**
```go
// BlockValidator MUST be set, or use DefaultBlockValidator that rejects all
type SyncReactorConfig struct {
    BlockValidator BlockValidator // Required, no nil default
}

// DefaultBlockValidator rejects all blocks (fail-closed)
var DefaultBlockValidator = func(height int64, hash, data []byte) error {
    return errors.New("no block validator configured")
}
```

**Tasks:**
- [ ] Make `BlockValidator` a required field in `SyncReactorConfig`
- [ ] Add `DefaultBlockValidator` that rejects all blocks
- [ ] Panic on `Start()` if validator is nil
- [ ] Update all examples and tests to explicitly set validators

### 0.2 Mandatory Transaction Validation

**Issue:** Mempool accepts transactions without semantic validation. The `Application.CheckTx()` interface exists but is never called.

**Fix:**
```go
// Mempool requires a TxValidator
type Mempool interface {
    // SetTxValidator MUST be called before AddTx
    SetTxValidator(validator TxValidator)
    AddTx(tx []byte) error
    // ...
}

type TxValidator func(tx []byte) error
```

**Tasks:**
- [ ] Add `SetTxValidator()` to `Mempool` interface
- [ ] Call validator in `AddTx()` before accepting
- [ ] Default to reject-all validator if not set
- [ ] Add `CheckTx` routing from `TransactionsReactor` through validator

### 0.3 Block Height Continuity Check

**Issue:** Blocks are stored at arbitrary heights without checking continuity (sync/reactor.go:424-427).

**Fix:**
```go
func (r *SyncReactor) handleBlocksResponse(...) error {
    // Blocks must be in order and contiguous
    expectedHeight := r.blockStore.Height() + 1
    for _, block := range resp.Blocks {
        if *block.Height != expectedHeight {
            return fmt.Errorf("non-contiguous block: expected %d, got %d",
                expectedHeight, *block.Height)
        }
        // ... validate and store
        expectedHeight++
    }
}
```

**Tasks:**
- [ ] Add contiguity validation in `handleBlocksResponse`
- [ ] Penalize peers who send non-contiguous blocks
- [ ] Add gap detection and repair logic

### 0.4 Unbounded Pending Requests Cleanup

**Issue:** The `pendingRequests` map in `TransactionsReactor` grows unbounded (handlers/transactions.go:37).

**Fix:**
```go
type TransactionsReactor struct {
    pendingRequests map[peer.ID]map[string]time.Time
    maxPendingAge   time.Duration // Default 60s
}

// In gossipLoop, periodically clean stale requests
func (r *TransactionsReactor) cleanupStaleRequests() {
    now := time.Now()
    r.mu.Lock()
    defer r.mu.Unlock()

    for peerID, pending := range r.pendingRequests {
        for txHash, requestTime := range pending {
            if now.Sub(requestTime) > r.maxPendingAge {
                delete(pending, txHash)
            }
        }
        if len(pending) == 0 {
            delete(r.pendingRequests, peerID)
        }
    }
}
```

**Tasks:**
- [ ] Add `maxPendingAge` configuration
- [ ] Add periodic cleanup in gossip loop
- [ ] Add metric for stale request count

### 0.5 Race Condition in Node Shutdown

**Issue:** The event loop may be processing messages when reactors are stopped (node/node.go:281-302).

**Fix:**
```go
func (n *Node) Stop() error {
    // 1. Stop accepting new connections
    n.network.StopAcceptingConnections()

    // 2. Signal event loop to stop
    close(n.stopCh)
    n.wg.Wait()  // Wait for event loop to drain

    // 3. Now safe to stop reactors
    _ = n.syncReactor.Stop()
    // ...
}
```

**Tasks:**
- [ ] Add `StopAcceptingConnections()` to network
- [ ] Ensure event loop drains before reactor shutdown
- [ ] Add shutdown timeout to prevent hangs

### 0.6 Handshake Timeout Enforcement

**Issue:** `PeerHandshakeState.StartedAt` is never used for timeout cleanup (handlers/handshake.go).

**Fix:**
```go
// Add handshake timeout goroutine
func (h *HandshakeHandler) Start() error {
    go h.timeoutLoop()
    return nil
}

func (h *HandshakeHandler) timeoutLoop() {
    ticker := time.NewTicker(5 * time.Second)
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
    h.mu.Lock()
    defer h.mu.Unlock()

    now := time.Now()
    for peerID, state := range h.states {
        if state.State != StateComplete && now.Sub(state.StartedAt) > h.timeout {
            delete(h.states, peerID)
            _ = h.network.Disconnect(peerID)
        }
    }
}
```

**Tasks:**
- [ ] Add `Start()`/`Stop()` lifecycle to `HandshakeHandler`
- [ ] Implement timeout cleanup loop
- [ ] Disconnect peers with stale handshakes

### 0.7 Constant-Time Hash Comparison

**Issue:** Hash comparisons use string conversion which is timing-vulnerable (sync/reactor.go:431).

**Fix:**
```go
import "crypto/subtle"

// Replace:
// if string(computedHash) != string(block.Hash) {

// With:
if subtle.ConstantTimeCompare(computedHash, block.Hash) != 1 {
    // Hash mismatch
}
```

**Tasks:**
- [ ] Replace all `string(hash)` comparisons with `subtle.ConstantTimeCompare`
- [ ] Add `types.HashEqual()` helper function
- [ ] Audit all hash comparison sites

### 0.8 Penalty Persistence and Decay Fix

**Issue:** Penalties only decay for connected peers; disconnected peers never decay (p2p/scoring.go:162).

**Fix:**
```go
type PeerScorer struct {
    // Persist penalty history
    penaltyHistory map[peer.ID]PenaltyRecord
    persistPath    string
}

type PenaltyRecord struct {
    Points       int64
    LastDecay    time.Time
    BanCount     int
    LastBanEnd   time.Time
}

func (ps *PeerScorer) GetPenaltyPoints(peerID peer.ID) int64 {
    // Calculate decayed points based on time since last decay
    record := ps.penaltyHistory[peerID]
    elapsed := time.Since(record.LastDecay)
    decayedPoints := record.Points - int64(elapsed.Hours()) * PenaltyDecayRate
    if decayedPoints < 0 {
        decayedPoints = 0
    }
    return decayedPoints
}
```

**Tasks:**
- [ ] Persist penalty history to address book
- [ ] Calculate decay based on wall-clock time
- [ ] Load penalty history on startup
- [ ] Add configurable decay rate

---

## Phase 1: Pluggable Architecture Foundation

**Priority: HIGH**
**Effort: 2-3 weeks**

Establish the foundational patterns for pluggable components. This phase sets up the dependency injection framework and lifecycle management that all plugins will use.

### 1.1 Component Interface Pattern

Define the standard interface pattern all pluggable components must follow:

```go
// Component is the base interface for all pluggable components.
type Component interface {
    // Start initializes and starts the component.
    // Must be idempotent (safe to call multiple times).
    Start() error

    // Stop gracefully shuts down the component.
    // Must be idempotent and safe to call even if not started.
    Stop() error

    // IsRunning returns true if the component is running.
    IsRunning() bool
}

// ConfigurableComponent can be configured before starting.
type ConfigurableComponent interface {
    Component

    // Validate checks the component's configuration.
    Validate() error
}

// LifecycleAware components receive lifecycle events.
type LifecycleAware interface {
    // OnStart is called after all components are created but before Start().
    OnStart() error

    // OnStop is called before Stop() to prepare for shutdown.
    OnStop() error
}
```

**Tasks:**
- [ ] Create `types/component.go` with base interfaces
- [ ] Refactor all reactors to implement `Component`
- [ ] Add lifecycle hooks for startup/shutdown coordination
- [ ] Add component dependency graph for ordered startup

### 1.2 Dependency Injection Container

Create a simple DI container for component wiring:

```go
// Container manages component lifecycle and dependencies.
type Container struct {
    components map[string]Component
    order      []string  // Startup order
    started    bool
    mu         sync.RWMutex
}

// Register adds a component with its dependencies.
func (c *Container) Register(name string, component Component, deps ...string) error

// Get retrieves a component by name.
func (c *Container) Get(name string) (Component, error)

// StartAll starts all components in dependency order.
func (c *Container) StartAll() error

// StopAll stops all components in reverse dependency order.
func (c *Container) StopAll() error
```

**Tasks:**
- [ ] Create `container/container.go`
- [ ] Implement topological sort for dependency ordering
- [ ] Add circular dependency detection
- [ ] Integrate with Node startup/shutdown

### 1.3 Callback-Based Extensibility

Adopt looseberry's callback pattern for component communication:

```go
// Callbacks allow components to communicate without tight coupling.
type NodeCallbacks struct {
    // Transaction callbacks
    OnTxReceived     func(peerID peer.ID, tx []byte)
    OnTxValidated    func(tx []byte, err error)
    OnTxBroadcast    func(tx []byte, peers []peer.ID)

    // Block callbacks
    OnBlockReceived  func(peerID peer.ID, height int64, hash, data []byte)
    OnBlockValidated func(height int64, hash []byte, err error)
    OnBlockCommitted func(height int64, hash []byte)

    // Peer callbacks
    OnPeerConnected    func(peerID peer.ID, isOutbound bool)
    OnPeerHandshaked   func(peerID peer.ID, info *PeerInfo)
    OnPeerDisconnected func(peerID peer.ID)

    // Consensus callbacks (set by consensus engine)
    OnConsensusMessage func(peerID peer.ID, data []byte)
    OnProposalReady    func(height int64) ([]byte, error)
    OnVoteReady        func(height int64, round int32, voteType VoteType) ([]byte, error)
}
```

**Tasks:**
- [ ] Define callback types in `types/callbacks.go`
- [ ] Add `SetCallbacks()` method to Node
- [ ] Replace direct handler calls with callback invocations
- [ ] Add callback nil-checks with no-op defaults

### 1.4 Configuration Overhaul

Restructure configuration for pluggable components:

```go
type Config struct {
    // Core configuration
    Node     NodeConfig     `toml:"node"`
    Network  NetworkConfig  `toml:"network"`

    // Role-specific configuration
    Role     NodeRole       `toml:"role"`  // validator, full, seed, light

    // Pluggable component configuration
    Mempool   MempoolConfig   `toml:"mempool"`
    Consensus ConsensusConfig `toml:"consensus"`

    // Storage configuration
    BlockStore BlockStoreConfig `toml:"blockstore"`
    StateStore StateStoreConfig `toml:"statestore"`

    // Stream configuration (for plugins that add streams)
    Streams  []StreamConfig  `toml:"streams"`
}

type MempoolConfig struct {
    // Type selects the mempool implementation
    Type string `toml:"type"`  // "simple", "priority", "ttl", "looseberry", "custom"

    // Type-specific configuration (parsed based on Type)
    Simple      *SimpleMempoolConfig      `toml:"simple,omitempty"`
    Priority    *PriorityMempoolConfig    `toml:"priority,omitempty"`
    TTL         *TTLMempoolConfig         `toml:"ttl,omitempty"`
    Looseberry  *LooseberryConfig         `toml:"looseberry,omitempty"`
}

type ConsensusConfig struct {
    // Type selects the consensus engine
    Type string `toml:"type"`  // "none", "bft", "raft", "poa", "custom"

    // Type-specific configuration
    BFT  *BFTConsensusConfig  `toml:"bft,omitempty"`
    Raft *RaftConsensusConfig `toml:"raft,omitempty"`
    PoA  *PoAConsensusConfig  `toml:"poa,omitempty"`
}
```

**Tasks:**
- [ ] Restructure `config/config.go` with new schema
- [ ] Add `NodeRole` enum with validation
- [ ] Add per-plugin configuration sections
- [ ] Add `StreamConfig` for dynamic stream registration
- [ ] Update all examples with new configuration

### 1.5 Remove Hardcoded Values

**Issue:** Critical timing values are hardcoded (node/node.go:171-172).

**Fix:**
```go
type HandlersConfig struct {
    Transactions TransactionsConfig `toml:"transactions"`
    Blocks       BlocksConfig       `toml:"blocks"`
    Sync         SyncConfig         `toml:"sync"`
}

type TransactionsConfig struct {
    RequestInterval time.Duration `toml:"request_interval"`
    BatchSize       int32         `toml:"batch_size"`
    MaxPending      int           `toml:"max_pending"`
}
```

**Tasks:**
- [ ] Move all hardcoded values to configuration
- [ ] Add sensible defaults for all configurable values
- [ ] Document tuning guidelines

### 1.6 Fix Options Pattern

**Issue:** Options are applied twice (node/node.go:143-146, 211-214).

**Fix:**
```go
// Use builder pattern instead of double-apply
type NodeBuilder struct {
    cfg         *Config
    mempool     Mempool
    blockStore  BlockStore
    consensus   ConsensusEngine
    // ...
}

func NewNodeBuilder(cfg *Config) *NodeBuilder {
    return &NodeBuilder{cfg: cfg}
}

func (b *NodeBuilder) WithMempool(mp Mempool) *NodeBuilder {
    b.mempool = mp
    return b
}

func (b *NodeBuilder) Build() (*Node, error) {
    // Create node with all options set
    // Reactors can reference components set in builder
}
```

**Tasks:**
- [ ] Replace `Option` pattern with `NodeBuilder`
- [ ] Ensure single-pass component initialization
- [ ] Add validation in `Build()` before creation

---

## Phase 2: Mempool Plugin System

**Priority: HIGH**
**Effort: 2-3 weeks**

Create a fully pluggable mempool system that supports simple mempools, looseberry DAG mempools, and custom implementations.

### 2.1 Extended Mempool Interface

```go
// Mempool is the base interface for transaction storage.
type Mempool interface {
    Component

    // Core operations
    AddTx(tx []byte) error
    RemoveTxs(hashes [][]byte)
    HasTx(hash []byte) bool
    GetTx(hash []byte) ([]byte, error)

    // Size information
    Size() int
    SizeBytes() int64

    // Reaping for block production
    ReapTxs(maxBytes int64) [][]byte

    // Maintenance
    Flush()
    TxHashes() [][]byte
}

// ValidatingMempool adds transaction validation.
type ValidatingMempool interface {
    Mempool

    // SetTxValidator sets the transaction validation function.
    SetTxValidator(validator TxValidator)
}

// DAGMempool is for DAG-based mempools like looseberry.
type DAGMempool interface {
    ValidatingMempool

    // ReapCertifiedBatches returns certified transaction batches.
    ReapCertifiedBatches(maxBytes int64) []CertifiedBatch

    // NotifyCommitted notifies that consensus has committed a round.
    NotifyCommitted(round uint64)

    // UpdateValidatorSet updates the validator set on epoch change.
    UpdateValidatorSet(validators ValidatorSet)

    // CurrentRound returns the current DAG round.
    CurrentRound() uint64

    // Metrics returns DAG-specific metrics.
    DAGMetrics() *DAGMempoolMetrics
}

// NetworkAwareMempool can register additional streams.
type NetworkAwareMempool interface {
    Mempool

    // StreamConfigs returns stream configurations this mempool needs.
    StreamConfigs() []StreamConfig

    // SetNetwork provides the network layer for custom protocols.
    SetNetwork(network Network)
}

// CertifiedBatch represents a batch that has been certified.
type CertifiedBatch struct {
    Batch       []byte
    Certificate []byte
    Round       uint64
    Validator   uint16
}
```

**Tasks:**
- [ ] Create `mempool/interface.go` with extended interfaces
- [ ] Add `DAGMempool` interface for looseberry compatibility
- [ ] Add `NetworkAwareMempool` for mempools with custom streams
- [ ] Update existing mempools to implement `ValidatingMempool`

### 2.2 Mempool Factory

```go
// MempoolFactory creates mempool instances from configuration.
type MempoolFactory struct {
    registry map[string]MempoolConstructor
}

type MempoolConstructor func(cfg *MempoolConfig) (Mempool, error)

func NewMempoolFactory() *MempoolFactory {
    f := &MempoolFactory{
        registry: make(map[string]MempoolConstructor),
    }

    // Register built-in mempools
    f.Register("simple", NewSimpleMempoolFromConfig)
    f.Register("priority", NewPriorityMempoolFromConfig)
    f.Register("ttl", NewTTLMempoolFromConfig)

    return f
}

func (f *MempoolFactory) Register(name string, constructor MempoolConstructor) {
    f.registry[name] = constructor
}

func (f *MempoolFactory) Create(cfg *MempoolConfig) (Mempool, error) {
    constructor, ok := f.registry[cfg.Type]
    if !ok {
        return nil, fmt.Errorf("unknown mempool type: %s", cfg.Type)
    }
    return constructor(cfg)
}
```

**Tasks:**
- [ ] Create `mempool/factory.go`
- [ ] Register all built-in mempools
- [ ] Add external mempool registration API
- [ ] Integrate factory with Node builder

### 2.3 Looseberry Integration

Create the adapter layer for looseberry integration:

```go
// LooseberryAdapter adapts looseberry to blockberry's mempool interface.
type LooseberryAdapter struct {
    lb          *looseberry.Looseberry
    network     *LooseberryNetworkAdapter
    txValidator TxValidator
    running     atomic.Bool
}

func NewLooseberryAdapter(cfg *LooseberryConfig) (*LooseberryAdapter, error) {
    lbCfg := looseberry.Config{
        ValidatorIndex: cfg.ValidatorIndex,
        Signer:         cfg.Signer,
        Worker:         convertWorkerConfig(cfg.Worker),
        Primary:        convertPrimaryConfig(cfg.Primary),
        // ...
    }

    lb, err := looseberry.New(&lbCfg)
    if err != nil {
        return nil, err
    }

    return &LooseberryAdapter{lb: lb}, nil
}

// Implement Mempool interface by delegating to looseberry
func (a *LooseberryAdapter) AddTx(tx []byte) error {
    return a.lb.AddTx(tx)
}

func (a *LooseberryAdapter) ReapTxs(maxBytes int64) [][]byte {
    // For non-DAG consumers, extract transactions from certified batches
    batches := a.lb.ReapCertifiedBatches(maxBytes)
    var txs [][]byte
    for _, batch := range batches {
        txs = append(txs, batch.Transactions()...)
    }
    return txs
}

// Implement DAGMempool interface
func (a *LooseberryAdapter) ReapCertifiedBatches(maxBytes int64) []CertifiedBatch {
    return a.lb.ReapCertifiedBatches(maxBytes)
}

// Implement NetworkAwareMempool interface
func (a *LooseberryAdapter) StreamConfigs() []StreamConfig {
    return []StreamConfig{
        {Name: "looseberry-batches", Encrypted: true},
        {Name: "looseberry-headers", Encrypted: true},
        {Name: "looseberry-sync", Encrypted: true},
    }
}

func (a *LooseberryAdapter) SetNetwork(network Network) {
    a.network = NewLooseberryNetworkAdapter(network)
    a.lb.SetNetwork(a.network)
}
```

**Tasks:**
- [ ] Create `mempool/looseberry/adapter.go`
- [ ] Implement all mempool interfaces
- [ ] Create `LooseberryNetworkAdapter` for glueberry
- [ ] Add looseberry stream registration
- [ ] Register with mempool factory

### 2.4 TransactionsReactor Passive Mode

Support passive mode for validators using DAG mempools:

```go
type TransactionsReactor struct {
    mempool     Mempool
    dagMempool  DAGMempool  // nil for simple mempools
    passiveMode bool        // true for validators with DAG mempool
    // ...
}

func (r *TransactionsReactor) Start() error {
    // Determine mode based on mempool type
    if dagMP, ok := r.mempool.(DAGMempool); ok {
        r.dagMempool = dagMP
        r.passiveMode = true  // Validators using DAG don't gossip
    }

    if !r.passiveMode {
        r.wg.Add(1)
        go r.gossipLoop()
    }

    return nil
}

func (r *TransactionsReactor) HandleMessage(peerID peer.ID, data []byte) error {
    // In passive mode, only receive and route to mempool
    // In active mode, also gossip to other peers

    // ... parse message ...

    if r.passiveMode {
        // Just add to mempool, no further gossiping
        return r.mempool.AddTx(txData)
    }

    // Active mode: add to mempool and schedule gossip
    if err := r.mempool.AddTx(txData); err != nil {
        return err
    }
    r.scheduleGossip(txData)
    return nil
}
```

**Tasks:**
- [ ] Add `passiveMode` field to `TransactionsReactor`
- [ ] Auto-detect mode based on mempool type
- [ ] Skip gossip loop in passive mode
- [ ] Add mode to metrics

---

## Phase 3: Consensus Engine Framework

**Priority: HIGH**
**Effort: 3-4 weeks**

Create a pluggable consensus engine framework that supports multiple consensus algorithms.

### 3.1 Consensus Engine Interface

```go
// ConsensusEngine is the main interface for consensus implementations.
type ConsensusEngine interface {
    Component

    // Initialize sets up the consensus engine with its dependencies.
    Initialize(deps ConsensusDependencies) error

    // ProcessBlock is called when a new block is received.
    ProcessBlock(block *Block) error

    // GetHeight returns the current consensus height.
    GetHeight() int64

    // GetRound returns the current consensus round (if applicable).
    GetRound() int32

    // IsValidator returns true if this node is a validator.
    IsValidator() bool

    // ValidatorSet returns the current validator set.
    ValidatorSet() ValidatorSet
}

// ConsensusDependencies provides access to node components.
type ConsensusDependencies struct {
    Network      Network
    BlockStore   BlockStore
    StateStore   StateStore
    Mempool      Mempool
    Application  Application
    Callbacks    *ConsensusCallbacks
}

// ConsensusCallbacks allows consensus to notify the node of events.
type ConsensusCallbacks struct {
    // OnBlockProduced is called when consensus produces a new block.
    OnBlockProduced func(block *Block) error

    // OnBlockCommitted is called when a block is finalized.
    OnBlockCommitted func(height int64, hash []byte) error

    // OnValidatorSetChanged is called when validators change.
    OnValidatorSetChanged func(validators ValidatorSet) error

    // OnStateSync is called when state sync should begin.
    OnStateSync func(targetHeight int64) error
}

// BlockProducer is implemented by engines that can create blocks.
type BlockProducer interface {
    // ProduceBlock creates a new block proposal.
    ProduceBlock(height int64) (*Block, error)

    // ShouldPropose returns true if this node should propose for the round.
    ShouldPropose(height int64, round int32) bool
}

// BFTConsensus is the interface for BFT-style consensus engines.
type BFTConsensus interface {
    ConsensusEngine
    BlockProducer

    // HandleProposal processes a block proposal.
    HandleProposal(proposal *Proposal) error

    // HandleVote processes a consensus vote.
    HandleVote(vote *Vote) error

    // HandleCommit processes a commit message.
    HandleCommit(commit *Commit) error

    // OnTimeout handles consensus timeouts.
    OnTimeout(height int64, round int32, step TimeoutStep) error
}

// StreamAwareConsensus can register additional streams.
type StreamAwareConsensus interface {
    ConsensusEngine

    // StreamConfigs returns stream configurations this engine needs.
    StreamConfigs() []StreamConfig

    // HandleStreamMessage handles messages on custom streams.
    HandleStreamMessage(stream string, peerID peer.ID, data []byte) error
}
```

**Tasks:**
- [ ] Create `consensus/interface.go` with core interfaces
- [ ] Define `ConsensusDependencies` struct
- [ ] Define `ConsensusCallbacks` struct
- [ ] Add `BFTConsensus` interface for BFT engines
- [ ] Add `StreamAwareConsensus` for custom protocols

### 3.2 Consensus Factory

```go
// ConsensusFactory creates consensus engine instances.
type ConsensusFactory struct {
    registry map[string]ConsensusConstructor
}

type ConsensusConstructor func(cfg *ConsensusConfig) (ConsensusEngine, error)

func NewConsensusFactory() *ConsensusFactory {
    f := &ConsensusFactory{
        registry: make(map[string]ConsensusConstructor),
    }

    // Register built-in engines
    f.Register("none", NewNullConsensus)

    return f
}

func (f *ConsensusFactory) Register(name string, constructor ConsensusConstructor) {
    f.registry[name] = constructor
}
```

**Tasks:**
- [ ] Create `consensus/factory.go`
- [ ] Create `NullConsensus` (no-op for full nodes)
- [ ] Add external engine registration API

### 3.3 Consensus Reactor Refactoring

Refactor the existing consensus reactor to use the new interface:

```go
// ConsensusReactor routes messages to the consensus engine.
type ConsensusReactor struct {
    engine        ConsensusEngine
    network       *p2p.Network
    peerManager   *p2p.PeerManager

    // Custom streams registered by the engine
    customStreams map[string]bool

    running bool
    stopCh  chan struct{}
    wg      sync.WaitGroup
    mu      sync.RWMutex
}

func NewConsensusReactor(engine ConsensusEngine, network *p2p.Network) *ConsensusReactor {
    r := &ConsensusReactor{
        engine:        engine,
        network:       network,
        customStreams: make(map[string]bool),
    }

    // Register custom streams if engine supports them
    if streamAware, ok := engine.(StreamAwareConsensus); ok {
        for _, cfg := range streamAware.StreamConfigs() {
            r.customStreams[cfg.Name] = true
        }
    }

    return r
}

func (r *ConsensusReactor) HandleMessage(peerID peer.ID, data []byte) error {
    // Route to BFT handler or custom stream handler
    if bft, ok := r.engine.(BFTConsensus); ok {
        return r.handleBFTMessage(bft, peerID, data)
    }

    // For non-BFT engines, just pass raw data
    return r.engine.ProcessBlock(&Block{Data: data})
}
```

**Tasks:**
- [ ] Refactor `ConsensusReactor` to use `ConsensusEngine` interface
- [ ] Add custom stream routing
- [ ] Add BFT message parsing and routing
- [ ] Integrate with node callbacks

### 3.4 Reference BFT Implementation Skeleton

Create a skeleton for a reference BFT implementation:

```go
// TendermintBFT is a reference Tendermint-style BFT implementation.
type TendermintBFT struct {
    // Configuration
    cfg *BFTConsensusConfig

    // State
    height    int64
    round     int32
    step      RoundStep
    lockedValue *Block
    lockedRound int32
    validValue  *Block
    validRound  int32

    // Validator info
    validatorSet ValidatorSet
    privValidator PrivValidator

    // Vote tracking
    prevotes    *HeightVoteSet
    precommits  *HeightVoteSet

    // Dependencies
    deps ConsensusDependencies

    // WAL for crash recovery
    wal *WAL

    // Timeouts
    timeoutPropose   time.Duration
    timeoutPrevote   time.Duration
    timeoutPrecommit time.Duration
    timeoutCommit    time.Duration

    // Lifecycle
    running atomic.Bool
    stopCh  chan struct{}
    wg      sync.WaitGroup
    mu      sync.RWMutex
}
```

**Tasks:**
- [ ] Create `consensus/bft/tendermint.go` skeleton
- [ ] Define round step state machine
- [ ] Define vote tracking structures
- [ ] Define WAL interface
- [ ] Document as reference for custom implementations

### 3.5 Validator Set Management

```go
// ValidatorSet represents the set of validators.
type ValidatorSet interface {
    // Validators returns all validators.
    Validators() []*Validator

    // GetByIndex returns a validator by index.
    GetByIndex(index uint16) *Validator

    // GetByAddress returns a validator by address.
    GetByAddress(addr Address) *Validator

    // Count returns the number of validators.
    Count() int

    // TotalVotingPower returns the sum of all voting power.
    TotalVotingPower() int64

    // Proposer returns the proposer for the given height and round.
    Proposer(height int64, round int32) *Validator

    // VerifySignature verifies a signature from a validator.
    VerifySignature(validatorIdx uint16, digest Hash, sig Signature) bool

    // Quorum returns the minimum voting power for consensus (2f+1).
    Quorum() int64

    // F returns the maximum Byzantine validators tolerated.
    F() int

    // Epoch returns the epoch number for this validator set.
    Epoch() uint64

    // Copy returns a deep copy of the validator set.
    Copy() ValidatorSet
}

// Validator represents a single validator.
type Validator struct {
    Address     Address
    PublicKey   PublicKey
    VotingPower int64
    Index       uint16
}

// ValidatorSetStore persists validator sets.
type ValidatorSetStore interface {
    // Save persists a validator set at a height.
    Save(height int64, valSet ValidatorSet) error

    // Load retrieves the validator set at a height.
    Load(height int64) (ValidatorSet, error)

    // Latest returns the most recent validator set.
    Latest() (ValidatorSet, error)
}
```

**Tasks:**
- [ ] Create `types/validator.go` with interfaces
- [ ] Implement `SimpleValidatorSet`
- [ ] Implement `WeightedValidatorSet` for PoS
- [ ] Create `ValidatorSetStore` interface
- [ ] Add IAVL-based implementation

---

## Phase 4: Node Role System

**Priority: HIGH**
**Effort: 1-2 weeks**

Implement a comprehensive node role system that determines which components are active based on the node's role in the network.

### 4.1 Node Role Definitions

```go
// NodeRole defines the role of a node in the network.
type NodeRole string

const (
    // RoleValidator is a consensus-participating validator node.
    RoleValidator NodeRole = "validator"

    // RoleFull is a full node that stores all data but doesn't validate.
    RoleFull NodeRole = "full"

    // RoleSeed is a seed node for peer discovery.
    RoleSeed NodeRole = "seed"

    // RoleLight is a light client that only stores headers.
    RoleLight NodeRole = "light"

    // RoleArchive is a full node that never prunes.
    RoleArchive NodeRole = "archive"
)

// RoleCapabilities defines what each role can do.
type RoleCapabilities struct {
    // CanPropose indicates if this role can propose blocks.
    CanPropose bool

    // CanVote indicates if this role can vote in consensus.
    CanVote bool

    // StoresFullBlocks indicates if this role stores full block data.
    StoresFullBlocks bool

    // StoresState indicates if this role stores application state.
    StoresState bool

    // ParticipatesInPEX indicates if this role exchanges peer addresses.
    ParticipatesInPEX bool

    // GossipsTransactions indicates if this role gossips transactions.
    GossipsTransactions bool

    // AcceptsInboundConnections indicates if this role accepts connections.
    AcceptsInboundConnections bool

    // Prunes indicates if this role prunes old data.
    Prunes bool
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
- [ ] Create `types/role.go` with role definitions
- [ ] Define capabilities per role
- [ ] Add role to configuration
- [ ] Add role validation

### 4.2 Role-Based Component Selection

```go
// RoleBasedBuilder creates components based on node role.
type RoleBasedBuilder struct {
    cfg        *Config
    role       NodeRole
    components map[string]Component
}

func (b *RoleBasedBuilder) Build() (*Node, error) {
    caps := roleCapabilities[b.role]

    // Create mempool based on role
    var mempool Mempool
    if caps.GossipsTransactions {
        // Full nodes use simple mempool with gossip
        mempool = b.createSimpleMempool()
    } else if caps.CanPropose {
        // Validators use DAG mempool
        mempool = b.createDAGMempool()
    }

    // Create consensus engine based on role
    var consensus ConsensusEngine
    if caps.CanVote {
        consensus = b.createConsensusEngine()
    } else {
        consensus = NewNullConsensus()
    }

    // Create stores based on role
    var blockStore BlockStore
    if caps.StoresFullBlocks {
        blockStore = b.createBlockStore()
    } else {
        blockStore = NewHeaderOnlyBlockStore()
    }

    // ... create other components based on role

    return b.assembleNode()
}
```

**Tasks:**
- [ ] Create `node/role_builder.go`
- [ ] Implement role-based component selection
- [ ] Add role-specific startup logic
- [ ] Add role-specific shutdown logic

### 4.3 Validator Detection

```go
// ValidatorDetector determines if this node is a validator.
type ValidatorDetector interface {
    // IsValidator returns true if this node is in the validator set.
    IsValidator(valSet ValidatorSet) bool

    // ValidatorIndex returns this node's validator index, or -1 if not a validator.
    ValidatorIndex(valSet ValidatorSet) int
}

// KeyBasedDetector uses the node's key to detect validator status.
type KeyBasedDetector struct {
    publicKey PublicKey
}

func (d *KeyBasedDetector) IsValidator(valSet ValidatorSet) bool {
    for _, val := range valSet.Validators() {
        if bytes.Equal(val.PublicKey, d.publicKey) {
            return true
        }
    }
    return false
}
```

**Tasks:**
- [ ] Create `ValidatorDetector` interface
- [ ] Implement key-based detection
- [ ] Add validator set change handling
- [ ] Add dynamic role switching (e.g., becoming a validator)

---

## Phase 5: Dynamic Stream Registry

**Priority: HIGH**
**Effort: 2-3 weeks**

Create a dynamic stream registry that allows plugins to register additional P2P streams at runtime.

### 5.1 Stream Registry Interface

```go
// StreamRegistry manages dynamic stream registration.
type StreamRegistry interface {
    // Register adds a new stream configuration.
    Register(cfg StreamConfig) error

    // Unregister removes a stream.
    Unregister(name string) error

    // Get returns a stream configuration by name.
    Get(name string) (*StreamConfig, error)

    // All returns all registered streams.
    All() []StreamConfig

    // RegisterHandler sets the message handler for a stream.
    RegisterHandler(name string, handler StreamHandler) error
}

// StreamConfig defines a P2P stream.
type StreamConfig struct {
    // Name is the unique stream identifier.
    Name string `toml:"name"`

    // Encrypted indicates if the stream uses encryption.
    Encrypted bool `toml:"encrypted"`

    // MessageTypes lists the message type IDs for this stream.
    MessageTypes []uint16 `toml:"message_types"`

    // RateLimit is the maximum messages per second.
    RateLimit int `toml:"rate_limit"`

    // MaxMessageSize is the maximum message size in bytes.
    MaxMessageSize int `toml:"max_message_size"`

    // Owner identifies which component owns this stream.
    Owner string `toml:"owner"`
}

// StreamHandler processes messages on a stream.
type StreamHandler func(peerID peer.ID, data []byte) error
```

**Tasks:**
- [ ] Create `p2p/stream_registry.go`
- [ ] Implement in-memory registry
- [ ] Add handler registration
- [ ] Add rate limit enforcement per stream

### 5.2 Integration with Glueberry

```go
// GlueberryStreamAdapter integrates the registry with glueberry.
type GlueberryStreamAdapter struct {
    registry StreamRegistry
    node     *glueberry.Node
    handlers map[string]StreamHandler
    mu       sync.RWMutex
}

func (a *GlueberryStreamAdapter) OnStreamRegistered(cfg StreamConfig) error {
    // Add stream to glueberry
    if cfg.Encrypted {
        return a.node.AddEncryptedStream(cfg.Name)
    }
    return a.node.AddStream(cfg.Name)
}

func (a *GlueberryStreamAdapter) RouteMessage(msg streams.IncomingMessage) error {
    a.mu.RLock()
    handler, ok := a.handlers[msg.StreamName]
    a.mu.RUnlock()

    if !ok {
        return fmt.Errorf("no handler for stream: %s", msg.StreamName)
    }

    return handler(msg.PeerID, msg.Data)
}
```

**Tasks:**
- [ ] Create stream adapter for glueberry
- [ ] Add dynamic stream creation
- [ ] Add message routing
- [ ] Handle stream removal

### 5.3 Plugin Stream Registration

```go
// Network provides stream registration to plugins.
type Network interface {
    // ... existing methods ...

    // RegisterStream registers a new stream with the network.
    RegisterStream(cfg StreamConfig, handler StreamHandler) error

    // UnregisterStream removes a stream.
    UnregisterStream(name string) error
}

// Example: Looseberry registering its streams
func (a *LooseberryAdapter) SetNetwork(network Network) {
    // Register looseberry-specific streams
    network.RegisterStream(StreamConfig{
        Name:           "looseberry-batches",
        Encrypted:      true,
        RateLimit:      1000,
        MaxMessageSize: 10 * 1024 * 1024,
        Owner:          "looseberry",
    }, a.handleBatchMessage)

    network.RegisterStream(StreamConfig{
        Name:           "looseberry-headers",
        Encrypted:      true,
        RateLimit:      100,
        MaxMessageSize: 1024 * 1024,
        Owner:          "looseberry",
    }, a.handleHeaderMessage)
}
```

**Tasks:**
- [ ] Add `RegisterStream` to Network interface
- [ ] Update looseberry adapter to use registration
- [ ] Update consensus reactor to use registration
- [ ] Add stream ownership tracking

---

## Phase 6: Storage Enhancements

**Priority: MEDIUM**
**Effort: 3-4 weeks**

Enhance storage capabilities with pruning, snapshots, and additional backends.

### 6.1 Block Pruning

**Issue:** No pruning mechanism - blocks accumulate forever.

```go
type BlockStore interface {
    // ... existing methods ...

    // Prune removes blocks before the given height.
    Prune(beforeHeight int64) error

    // PruneConfig returns the pruning configuration.
    PruneConfig() *PruneConfig
}

type PruneConfig struct {
    // Strategy is the pruning strategy.
    Strategy PruneStrategy `toml:"strategy"`

    // KeepRecent is the number of recent blocks to keep.
    KeepRecent int64 `toml:"keep_recent"`

    // KeepEvery is the interval for keeping pruning checkpoints.
    KeepEvery int64 `toml:"keep_every"`

    // Interval is how often to run pruning.
    Interval time.Duration `toml:"interval"`
}

type PruneStrategy string

const (
    PruneNothing    PruneStrategy = "nothing"    // Archive node
    PruneEverything PruneStrategy = "everything" // Aggressive pruning
    PruneDefault    PruneStrategy = "default"    // Keep recent + checkpoints
)
```

**Tasks:**
- [ ] Add `Prune()` to BlockStore interface
- [ ] Implement pruning in LevelDBBlockStore
- [ ] Add background pruning goroutine
- [ ] Add pruning metrics

### 6.2 State Pruning

```go
type StateStore interface {
    // ... existing methods ...

    // PruneVersions removes old versions of the state.
    PruneVersions(keepRecent int64, keepEvery int64) error

    // AvailableVersions returns the range of available versions.
    AvailableVersions() (oldest, newest int64)
}
```

**Tasks:**
- [ ] Add `PruneVersions()` to StateStore interface
- [ ] Implement IAVL pruning
- [ ] Add pruning configuration
- [ ] Add background pruning

### 6.3 State Snapshots

```go
// SnapshotStore manages state snapshots for fast sync.
type SnapshotStore interface {
    // Create creates a snapshot at the given height.
    Create(height int64) (*Snapshot, error)

    // List returns available snapshots.
    List() ([]*SnapshotInfo, error)

    // Load loads a snapshot by hash.
    Load(hash []byte) (*Snapshot, error)

    // Delete removes a snapshot.
    Delete(hash []byte) error

    // Prune removes old snapshots.
    Prune(keepRecent int) error
}

type Snapshot struct {
    Height    int64
    Hash      []byte
    ChunkSize int
    Chunks    int
    Metadata  []byte
}

type SnapshotChunk struct {
    Index int
    Data  []byte
}
```

**Tasks:**
- [ ] Create `SnapshotStore` interface
- [ ] Implement IAVL export/import
- [ ] Add chunked streaming
- [ ] Add snapshot discovery via PEX

### 6.4 BadgerDB Backend

**Issue:** Configuration supports "badgerdb" but only LevelDB is implemented.

**Tasks:**
- [ ] Implement `BadgerDBBlockStore`
- [ ] Add BadgerDB-specific configuration
- [ ] Benchmark against LevelDB
- [ ] Add migration tooling

---

## Phase 7: Developer Experience

**Priority: MEDIUM**
**Effort: 4-6 weeks**

Improve the developer experience with tooling, documentation, and APIs.

### 7.1 RPC API

```go
// RPCServer provides the node RPC interface.
type RPCServer interface {
    Component

    // Health returns the node health status.
    Health() (*HealthStatus, error)

    // Status returns the node status.
    Status() (*NodeStatus, error)

    // BroadcastTx broadcasts a transaction.
    BroadcastTx(tx []byte, mode BroadcastMode) (*BroadcastResult, error)

    // Query queries the application state.
    Query(path string, data []byte, height int64) (*QueryResult, error)

    // Block returns a block by height or hash.
    Block(height int64) (*Block, error)

    // Tx returns a transaction by hash.
    Tx(hash []byte) (*TxResult, error)

    // Peers returns connected peers.
    Peers() ([]*PeerInfo, error)

    // Subscribe subscribes to events.
    Subscribe(query string) (<-chan Event, error)
}
```

**Tasks:**
- [ ] Define RPC interface
- [ ] Implement JSON-RPC 2.0 transport
- [ ] Implement gRPC transport
- [ ] Add authentication and rate limiting
- [ ] Generate OpenAPI documentation

### 7.2 CLI Tooling

```bash
# Node operations
blockberry init --chain-id mychain --moniker mynode
blockberry start --config config.toml
blockberry status
blockberry peers [list|connect|disconnect|ban]

# Transaction operations
blockberry tx broadcast <tx-file>
blockberry tx status <hash>

# Query operations
blockberry query state <path>
blockberry query block <height>
blockberry query validator-set

# Key management
blockberry keys generate
blockberry keys import <file>
blockberry keys export
blockberry keys show

# Debug operations
blockberry debug dump-state <height>
blockberry debug profile [cpu|heap|goroutine]
```

**Tasks:**
- [ ] Create CLI framework using cobra
- [ ] Implement node commands
- [ ] Implement transaction commands
- [ ] Implement query commands
- [ ] Implement key management

### 7.3 Event Subscription System

```go
// EventBus provides pub/sub for blockchain events.
type EventBus interface {
    // Publish publishes an event.
    Publish(event Event) error

    // Subscribe creates a subscription for the given query.
    Subscribe(query string) (Subscription, error)

    // Unsubscribe removes a subscription.
    Unsubscribe(sub Subscription) error
}

type Event interface {
    Type() string
    Data() interface{}
    Height() int64
    Time() time.Time
}

type Subscription interface {
    Events() <-chan Event
    Close() error
}
```

**Tasks:**
- [ ] Define event types
- [ ] Implement event bus
- [ ] Add query language for filtering
- [ ] Add WebSocket transport

### 7.4 Transaction Indexing

```go
// TxIndexer indexes transactions for querying.
type TxIndexer interface {
    // Index indexes a transaction.
    Index(tx *TxResult) error

    // Get retrieves a transaction by hash.
    Get(hash []byte) (*TxResult, error)

    // Search searches for transactions matching the query.
    Search(query string) ([]*TxResult, error)

    // Delete removes a transaction from the index.
    Delete(hash []byte) error
}
```

**Tasks:**
- [ ] Define TxIndexer interface
- [ ] Implement LevelDB-based indexer
- [ ] Add indexing by hash, height, sender
- [ ] Add search query language

---

## Phase 8: Security Hardening

**Priority: MEDIUM**
**Effort: 2-3 weeks**

Address security issues and harden the system against attacks.

### 8.1 Eclipse Attack Mitigation

**Issue:** PEX accepts addresses without validation (pex/reactor.go:165-189).

```go
// AddressTrustScore tracks address reputation.
type AddressTrustScore struct {
    Address     string
    Source      peer.ID
    FirstSeen   time.Time
    LastSuccess time.Time
    FailCount   int
    SuccessCount int
    Trust       float64  // 0.0 to 1.0
}

// AddressValidator validates received addresses.
type AddressValidator interface {
    // Validate checks if an address should be added.
    Validate(addr string, source peer.ID) error

    // UpdateScore updates the trust score for an address.
    UpdateScore(addr string, connected bool)
}
```

**Tasks:**
- [ ] Add address trust scoring
- [ ] Validate address reachability before adding
- [ ] Limit addresses per source peer
- [ ] Add diversity requirements (different subnets/ASNs)

### 8.2 Rate Limiting Enhancements

```go
// StreamRateLimiter applies per-stream rate limits.
type StreamRateLimiter struct {
    limits   map[string]*RateLimit
    counters map[peer.ID]map[string]*Counter
    mu       sync.RWMutex
}

type RateLimit struct {
    MessagesPerSecond int
    BytesPerSecond    int64
    BurstSize         int
}
```

**Tasks:**
- [ ] Add per-stream rate limits
- [ ] Add per-peer rate limits
- [ ] Add adaptive rate limiting
- [ ] Add rate limit metrics

### 8.3 Private Key Encryption

**Issue:** Private key stored as raw bytes (node/node.go:499).

```go
// EncryptedKeyStore stores encrypted private keys.
type EncryptedKeyStore interface {
    // Load loads and decrypts a private key.
    Load(path string, password []byte) (ed25519.PrivateKey, error)

    // Save encrypts and saves a private key.
    Save(path string, key ed25519.PrivateKey, password []byte) error
}

// Use argon2 for key derivation, AES-GCM for encryption
```

**Tasks:**
- [ ] Add encrypted key storage
- [ ] Add password prompt on startup
- [ ] Support environment variable for password
- [ ] Add key rotation support

### 8.4 Security Audit Preparation

**Tasks:**
- [ ] Document all security assumptions
- [ ] Add fuzz testing for message parsers
- [ ] Add static analysis CI checks
- [ ] Create threat model document

---

## Phase 9: Performance & Observability

**Priority: MEDIUM**
**Effort: 2-3 weeks**

Optimize performance and enhance observability.

### 9.1 Parallel Block Sync

**Issue:** Block sync only requests from one peer (sync/reactor.go:239).

```go
// ParallelSyncManager downloads blocks from multiple peers.
type ParallelSyncManager struct {
    blockStore  BlockStore
    peerManager *PeerManager
    network     Network

    workers   int
    batchSize int

    pending   map[int64]*syncRequest
    inFlight  map[peer.ID]int64  // peerID -> height being fetched
    mu        sync.Mutex
}

func (m *ParallelSyncManager) Sync(targetHeight int64) error {
    // Divide work across multiple peers
    // Use work-stealing for load balancing
    // Validate and store blocks in order
}
```

**Tasks:**
- [ ] Implement parallel sync manager
- [ ] Add work-stealing for load balancing
- [ ] Add adaptive worker count
- [ ] Add sync progress metrics

### 9.2 Memory Optimization

```go
// Use object pools for frequently allocated types
var messagePool = sync.Pool{
    New: func() interface{} {
        return &Message{}
    },
}

// Use buffer pools for serialization
var bufferPool = cramberry.NewBufferPool()
```

**Tasks:**
- [ ] Add object pooling for messages
- [ ] Add buffer pooling for serialization
- [ ] Profile memory hotspots
- [ ] Reduce allocations in hot paths

### 9.3 Distributed Tracing

```go
// TracingMiddleware adds OpenTelemetry tracing.
type TracingMiddleware struct {
    tracer trace.Tracer
}

func (m *TracingMiddleware) WrapHandler(name string, handler StreamHandler) StreamHandler {
    return func(peerID peer.ID, data []byte) error {
        ctx, span := m.tracer.Start(context.Background(), name)
        defer span.End()

        span.SetAttributes(
            attribute.String("peer_id", peerID.String()),
            attribute.Int("message_size", len(data)),
        )

        err := handler(peerID, data)
        if err != nil {
            span.RecordError(err)
        }
        return err
    }
}
```

**Tasks:**
- [ ] Add OpenTelemetry integration
- [ ] Add trace context to messages
- [ ] Add span creation for key operations
- [ ] Add Jaeger/Zipkin export

### 9.4 Health Check Endpoints

```go
// HealthServer provides health check endpoints.
type HealthServer struct {
    node *Node
}

// Kubernetes-compatible health checks
// GET /health/live - Node is running
// GET /health/ready - Node is synced and accepting requests
// GET /health/startup - Node has completed initialization
```

**Tasks:**
- [ ] Implement health check endpoints
- [ ] Add Kubernetes probe compatibility
- [ ] Add detailed health status
- [ ] Add health check configuration

---

## Phase 10: Ecosystem Integration

**Priority: LOW**
**Effort: 4-8 weeks**

Integrate with the broader blockchain ecosystem.

### 10.1 ABCI Compatibility Layer

```go
// ABCIApplication wraps a blockberry application as ABCI.
type ABCIApplication struct {
    app Application
}

func (a *ABCIApplication) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
    err := a.app.CheckTx(req.Tx)
    // Convert to ABCI response
}
```

**Tasks:**
- [ ] Implement ABCI server
- [ ] Create application wrapper
- [ ] Add Tendermint RPC compatibility
- [ ] Create migration guide

### 10.2 IBC Compatibility

**Tasks:**
- [ ] Implement IBC client interface
- [ ] Add connection/channel handshakes
- [ ] Implement packet relay
- [ ] Add ICS-20 token transfer

### 10.3 Light Client Protocol

```go
// LightClient verifies headers without full blocks.
type LightClient interface {
    // Sync syncs headers from trusted height to target.
    Sync(trustedHeight int64, trustedHash []byte, targetHeight int64) error

    // VerifyHeader verifies a header against the trusted set.
    VerifyHeader(header *Header) error

    // VerifyProof verifies a merkle proof against a root.
    VerifyProof(key, value, proof []byte, root []byte) error
}
```

**Tasks:**
- [ ] Implement header-only sync
- [ ] Add bisection algorithm for verification
- [ ] Add merkle proof verification
- [ ] Create light client library

---

## Version Milestones

### v1.1.0 - Critical Fixes & Stability
**Target: 2-3 weeks**
- All Phase 0 critical fixes
- Basic block/transaction validation
- Handshake timeout enforcement
- Penalty persistence

### v1.2.0 - Pluggable Foundation
**Target: 4-6 weeks**
- Component interface pattern
- Dependency injection container
- Callback-based extensibility
- Configuration overhaul

### v1.3.0 - Mempool Plugin System
**Target: 3-4 weeks**
- Extended mempool interfaces
- Mempool factory
- Looseberry integration
- Passive mode for TransactionsReactor

### v2.0.0 - Consensus Engine Framework
**Target: 6-8 weeks**
- Consensus engine interfaces
- Consensus factory
- Reference BFT skeleton
- Validator set management
- Node role system

### v2.1.0 - Dynamic Streams
**Target: 3-4 weeks**
- Stream registry
- Glueberry integration
- Plugin stream registration

### v2.2.0 - Storage Enhancements
**Target: 4-5 weeks**
- Block pruning
- State pruning
- State snapshots
- BadgerDB backend

### v3.0.0 - Developer Experience
**Target: 6-8 weeks**
- RPC API (JSON-RPC + gRPC)
- CLI tooling
- Event subscription
- Transaction indexing

### v3.1.0 - Security Hardening
**Target: 3-4 weeks**
- Eclipse attack mitigation
- Enhanced rate limiting
- Private key encryption
- Security documentation

### v3.2.0 - Performance & Observability
**Target: 3-4 weeks**
- Parallel block sync
- Memory optimization
- Distributed tracing
- Health check endpoints

### v4.0.0 - Ecosystem Integration
**Target: 8-12 weeks**
- ABCI compatibility
- IBC support
- Light client protocol

---

## Example: Building a Blockchain with the Berry Stack

```go
package main

import (
    "github.com/blockberries/blockberry"
    "github.com/blockberries/blockberry/consensus"
    "github.com/blockberries/blockberry/mempool"
    "github.com/blockberries/looseberry"
)

func main() {
    // Load configuration
    cfg, _ := blockberry.LoadConfig("config.toml")

    // Create node builder
    builder := blockberry.NewNodeBuilder(cfg)

    // Configure based on role
    if cfg.Role == blockberry.RoleValidator {
        // Validators use looseberry DAG mempool
        lb, _ := looseberry.New(&looseberry.Config{
            ValidatorIndex: cfg.Looseberry.ValidatorIndex,
            Signer:         loadSigner(cfg),
        })
        builder.WithMempool(mempool.NewLooseberryAdapter(lb))

        // Validators use BFT consensus
        bft := consensus.NewTendermintBFT(&cfg.Consensus.BFT)
        builder.WithConsensus(bft)
    } else {
        // Full nodes use simple mempool with gossip
        builder.WithMempool(mempool.NewSimpleMempool(&cfg.Mempool.Simple))

        // Full nodes use null consensus (just follow)
        builder.WithConsensus(consensus.NewNullConsensus())
    }

    // Set application logic
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

1. **Reference this document** in commits and PRs (e.g., "Phase 0.1: Mandatory Block Validation")
2. **Follow CLAUDE.md guidelines** for implementation workflow
3. **Update PROGRESS_REPORT.md** after completing each item
4. **Open issues** for blockers or design questions
5. **Write tests first** for all new functionality

---

*Last Updated: January 2025*
