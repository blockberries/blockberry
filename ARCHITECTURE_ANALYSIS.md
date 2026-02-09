# Blockberry Architecture Analysis

**Date**: 2026-02-02
**Version**: Based on commit 2368b5f

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Interface Architecture](#interface-architecture)
3. [Concurrency Model](#concurrency-model)
4. [Design Patterns](#design-patterns)
5. [Dependency Management](#dependency-management)
6. [Module Organization](#module-organization)
7. [Error Handling Architecture](#error-handling-architecture)
8. [Code Generation](#code-generation)
9. [Testing Architecture](#testing-architecture)
10. [Security Architecture](#security-architecture)
11. [Performance Optimizations](#performance-optimizations)
12. [Integration Patterns](#integration-patterns)

---

## Executive Summary

Blockberry is a well-architected Go blockchain framework that demonstrates sophisticated design patterns and architectural principles. The codebase exhibits:

- **Clean separation of concerns** through interface-driven design
- **Strong concurrency safety** using modern Go synchronization primitives
- **Fail-closed security** as a default principle
- **Pluggability** through factory patterns and dependency injection
- **Type safety** through cramberry code generation
- **Composability** through struct embedding and interface composition

The architecture prioritizes safety, extensibility, and maintainability while supporting high-performance blockchain operations.

---

## 1. Interface Architecture

### 1.1 Core Application Interfaces

#### `abi.Application` (pkg/abi/application.go)

**Contract**: Primary interface between blockchain framework and application logic.

```go
type Application interface {
    Info() ApplicationInfo
    InitChain(genesis *Genesis) error
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult      // Thread-safe
    BeginBlock(ctx context.Context, header *BlockHeader) error
    ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult
    EndBlock(ctx context.Context) *EndBlockResult
    Commit(ctx context.Context) *CommitResult
    Query(ctx context.Context, req *QueryRequest) *QueryResponse      // Thread-safe
}

```text

**Implementations**:

- `BaseApplication`: Fail-closed default implementation (rejects all operations)
- `AcceptAllApplication`: Testing implementation (accepts everything)

**Design Patterns**:

- **Template Method**: Structured lifecycle (BeginBlock → ExecuteTx → EndBlock → Commit)
- **Fail-Closed Security**: BaseApplication rejects by default, requiring explicit overrides
- **Thread-Safety Contract**: CheckTx and Query explicitly documented as concurrent-safe

**Goroutine Safety**: CheckTx and Query MUST be thread-safe per contract. Lifecycle methods (BeginBlock, ExecuteTx, EndBlock, Commit) are called sequentially by the framework.

---

#### `blockstore.BlockStore` (pkg/blockstore/store.go)

**Contract**: Block persistence with concurrent-safe operations.

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

**Implementations**:

- `LevelDBBlockStore`: Production implementation with pruning support
- `BadgerDBBlockStore`: Alternative high-performance backend
- `MemoryBlockStore`: In-memory for testing
- `HeaderOnlyStore`: Lightweight storage for light clients
- `NoOpBlockStore`: Null object for seed nodes
- `PruningWrapper`: Decorator adding automatic pruning

**Design Patterns**:

- **Strategy Pattern**: Pluggable storage backends
- **Decorator Pattern**: PruningWrapper adds functionality
- **Null Object Pattern**: NoOpBlockStore for nodes that don't need storage
- **Interface Composition**: `CertificateBlockStore` extends `BlockStore`

**Goroutine Safety**: All implementations use `sync.RWMutex` for concurrent access.

---

#### `mempool.Mempool` (pkg/mempool/interface.go)

**Contract**: Transaction pool management with validation.

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

**Extended Interfaces**:

- `ValidatingMempool`: Explicit validation control
- `DAGMempool`: DAG-based transaction ordering (looseberry integration)
- `NetworkAwareMempool`: Custom P2P stream registration
- `PrioritizedMempool`: Priority-based transaction ordering
- `ExpirableMempool`: Time-limited transaction storage
- `IterableMempool`: Iteration support

**Implementations**:

- `SimpleMempool`: Basic FIFO mempool
- `PriorityMempool`: Heap-based priority queue
- `TTLMempool`: Time-expiring transactions
- Looseberry adapter (external)

**Design Patterns**:

- **Interface Segregation**: Extended interfaces for optional capabilities
- **Factory Pattern**: `NewMempool()` creates appropriate implementation
- **Strategy Pattern**: Pluggable validators
- **Fail-Closed**: DefaultTxValidator rejects all transactions

**Goroutine Safety**: All implementations use mutexes. Validation callbacks must be thread-safe.

---

#### `consensus.ConsensusEngine` (pkg/consensus/interface.go)

**Contract**: Pluggable consensus algorithm interface.

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

**Extended Interfaces**:

- `BlockProducer`: Block creation capabilities
- `BFTConsensus`: BFT-specific message handling
- `StreamAwareConsensus`: Custom network stream registration

**Implementations**:

- `NullConsensus`: No-op for full nodes

**Design Patterns**:

- **Strategy Pattern**: Pluggable consensus algorithms
- **Factory Pattern**: `consensus.Factory` with type registry
- **Dependency Injection**: `ConsensusDependencies` struct
- **Callback Pattern**: `ConsensusCallbacks` for event notifications

**Goroutine Safety**: Implementations manage their own concurrency model.

---

#### `statestore.StateStore` (pkg/statestore/store.go)

**Contract**: Merkleized key-value state storage (AVL+ based, via avlberry).

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

**Implementation**: `IAVLStateStore` wrapping avlberry.

**Design Patterns**:

- **Adapter Pattern**: Wraps avlberry with Blockberry interface
- **Proof of Existence**: ICS23-compatible merkle proofs

**Goroutine Safety**: Thread-safe through avlberry's internal locking.

---

### 1.2 Component Lifecycle Interfaces

#### `types.Component` (pkg/types/component.go)

**Contract**: Unified lifecycle management for all components.

```go
type Component interface {
    Start() error
    Stop() error
    IsRunning() bool
}

```text

**Extended Interfaces**:

- `ConfigurableComponent`: Adds `Validate() error`
- `LifecycleAware`: Adds `OnStart()` and `OnStop()` hooks
- `Named`: Adds `Name() string` for identification
- `Dependent`: Declares dependencies for ordering
- `HealthChecker`: Provides `HealthCheck() error` for monitoring

**Design Patterns**:

- **Interface Segregation**: Small, focused interfaces
- **Lifecycle Management**: Consistent Start/Stop across all components
- **Dependency Declaration**: Explicit dependency tracking

---

### 1.3 Supporting Interfaces

#### RPC Server (`rpc.Server`)
- Transport-agnostic RPC interface
- JSON-RPC, gRPC, and WebSocket support
- Subscription-based event streaming

#### Metrics (`metrics.Metrics`)
- Prometheus-based metrics collection
- Thread-safe, non-blocking operations
- Standardized labels and naming

#### Logger (`logging.Logger`)
- Wraps `slog.Logger` with blockchain-specific helpers
- Structured logging with contextual attributes
- Type-safe attribute constructors

---

## 2. Concurrency Model

### 2.1 Goroutine Usage Patterns

#### Event Loop Pattern (pkg/node/node.go)

```go
func (n *Node) eventLoop() {
    defer n.wg.Done()

    for {
        select {
        case <-n.stopCh:
            return
        case event := <-events:
            n.handleConnectionEvent(event)
        case msg := <-messages:
            n.handleMessage(msg)
        }
    }
}

```text

**Characteristics**:

- Single goroutine per node for message/event processing
- Non-blocking select with stop channel
- WaitGroup for graceful shutdown

---

#### Background Worker Pattern (internal/handlers/handshake.go)

```go
func (h *HandshakeHandler) timeoutLoop() {
    defer h.wg.Done()

    ticker := time.NewTicker(h.checkInterval)
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

```text

**Characteristics**:

- Periodic background tasks with ticker
- Graceful cancellation via stop channel
- Proper cleanup with defer

---

#### One-Shot Async Pattern (internal/handlers/handshake.go)

```go
go func(pid peer.ID) {
    _ = h.network.Disconnect(pid)
}(peerID)

```text

**Use Case**: Fire-and-forget operations that shouldn't block current goroutine.

---

### 2.2 Synchronization Primitives

#### Mutex Usage Patterns

**Read-Heavy Pattern** (internal/p2p/network.go):

```go
type Network struct {
    tempBans   map[peer.ID]*TempBanEntry
    tempBansMu sync.RWMutex  // RWMutex for read-heavy access
}

func (n *Network) IsTempBanned(peerID peer.ID) bool {
    n.tempBansMu.RLock()
    defer n.tempBansMu.RUnlock()
    // ... read operation
}

```text

**Write-Heavy Pattern** (internal/container/container.go):

```go
type Container struct {
    components map[string]*componentEntry
    mu         sync.RWMutex
}

func (c *Container) Register(name string, component Component) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    // ... write operation
}

```text

**Lock Ordering**: Consistently acquire locks in the same order to prevent deadlocks:

1. Network-level locks first
2. Then peer-specific locks
3. Then resource-specific locks

---

#### Atomic Operations (pkg/node/node.go)

```go
type Node struct {
    stopping atomic.Bool  // Shutdown signal
}

// Check before processing
if n.stopping.Load() {
    return
}

```text

**Use Cases**:

- Boolean flags (running, stopped, cancelled)
- Counters that don't need mutex protection
- Lock-free status checks

---

#### Channel Patterns

**Buffered Channels** (pkg/events/bus.go):

```go
ch := make(chan abi.Event, b.config.BufferSize)  // Buffered for non-blocking

```text

**Stop Channel Pattern**:

```go
stopCh := make(chan struct{})  // Unbuffered, signal-only
close(stopCh)                  // Broadcast to all listeners

```text

**Timeout Pattern** (pkg/events/bus.go):

```go
timer := time.NewTimer(timeout)
select {
case sub.ch <- event:
    if !timer.Stop() {
        <-timer.C  // Drain timer to prevent leak
    }
case <-timer.C:
    // Timeout
}

```text

---

### 2.3 Graceful Shutdown Pattern

**Multi-Phase Shutdown** (pkg/node/node.go):

```go
func (n *Node) Stop() error {
    // Phase 1: Signal shutdown
    n.stopping.Store(true)

    // Phase 2: Stop event loop
    close(n.stopCh)

    // Phase 3: Wait with timeout
    done := make(chan struct{})
    go func() {
        n.wg.Wait()
        close(done)
    }()

    ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
    defer cancel()

    select {
    case <-done:
        // Clean shutdown
    case <-ctx.Done():
        // Timeout - proceed anyway
    }

    // Phase 4: Stop components
    _ = n.syncReactor.Stop()
    // ... other components

    return nil
}

```text

**Key Elements**:

1. Atomic flag to stop new work
2. Signal existing goroutines to stop
3. Wait for cleanup with timeout
4. Stop components in reverse order
5. Close resources (files, connections)

---

### 2.4 Race Condition Prevention

**Deep Copy Pattern** (internal/handlers/handshake.go):

```go
func (h *HandshakeHandler) GetPeerInfo(peerID peer.ID) *PeerHandshakeState {
    h.mu.RLock()
    defer h.mu.RUnlock()

    // Return deep copy to prevent mutation
    return &PeerHandshakeState{
        PeerPubKey: append([]byte(nil), state.PeerPubKey...),
        // ... other fields
    }
}

```text

**Lock-Then-Check Pattern**:

```go
h.mu.Lock()
if state != nil && state.SentRequest {
    h.mu.Unlock()
    return nil  // Already sent
}
state.SentRequest = true
h.mu.Unlock()

```text

**Snapshot-Then-Release Pattern** (pkg/node/node.go):

```go
n.mu.RLock()
cb := n.callbacks  // Snapshot under lock
n.mu.RUnlock()
// Use cb outside lock to avoid holding lock during callback
cb.InvokePeerConnected(peerID, isOutbound)

```text

---

## 3. Design Patterns

### 3.1 Creational Patterns

#### Factory Pattern

**Consensus Factory** (pkg/consensus/factory.go):

```go
type Factory struct {
    registry map[ConsensusType]ConsensusConstructor
    mu       sync.RWMutex
}

func (f *Factory) Register(consensusType ConsensusType, constructor ConsensusConstructor)
func (f *Factory) Create(cfg *ConsensusConfig) (ConsensusEngine, error)

```text

**Benefits**:

- Extensible: Users can register custom consensus engines
- Type-safe: Registry enforces constructor signatures
- Thread-safe: Protected by RWMutex

**Mempool Factory** (pkg/mempool/mempool.go):

```go
func NewMempool(cfg config.MempoolConfig) Mempool {
    switch cfg.Type {
    case MempoolTypePriority:
        return NewPriorityMempool(...)
    case MempoolTypeTTL:
        return NewTTLMempool(...)
    default:
        return NewSimpleMempool(...)
    }
}

```text

---

#### Builder Pattern

**Role-Based Node Configuration**:

```go
func (b *Builder) AsValidator() *Builder
func (b *Builder) AsFullNode() *Builder
func (b *Builder) AsSeedNode() *Builder

```text

**Benefits**:

- Fluent API for complex configuration
- Role-based defaults
- Validation before build

---

#### Singleton Pattern

**Global Factory** (pkg/consensus/factory.go):

```go
var DefaultFactory = NewFactory()

func RegisterConsensus(consensusType ConsensusType, constructor ConsensusConstructor) {
    DefaultFactory.Register(consensusType, constructor)
}

```text

**Usage**: Global registry for consensus types, allowing applications to register custom implementations.

---

### 3.2 Structural Patterns

#### Adapter Pattern

**Glueberry Stream Adapter** (internal/p2p/stream_adapter.go):

```go
type GlueberryStreamAdapter struct {
    registry StreamRegistry
}

func (a *GlueberryStreamAdapter) RouteMessage(msg streams.IncomingMessage) error

```text

**Purpose**: Adapts glueberry's stream API to Blockberry's handler interface.

**StateStore Adapter** (pkg/statestore/store.go):

- Adapts avlberry to Blockberry's StateStore interface
- Adds ICS23 proof generation

---

#### Decorator Pattern

**Pruning Wrapper** (pkg/blockstore/pruning.go):

```go
type PruningWrapper struct {
    store    BlockStore
    pruner   *BackgroundPruner
}

```text

**Purpose**: Adds automatic pruning to any BlockStore implementation.

**Benefits**:

- Transparent: Looks like normal BlockStore
- Composable: Can wrap any implementation
- Optional: Only used when pruning is needed

---

#### Facade Pattern

**Node Facade** (pkg/node/node.go):

```go
type Node struct {
    network  *p2p.Network
    mempool  mempool.Mempool
    blockStore blockstore.BlockStore
    // ... reactors
}

```text

**Purpose**: Single unified interface to entire blockchain node, hiding complexity of individual components.

---

#### Composite Pattern

**Component Container** (internal/container/container.go):

- Treats individual components and component groups uniformly
- Manages lifecycle as a tree

---

### 3.3 Behavioral Patterns

#### Strategy Pattern

**Mempool Strategies**:

- SimpleMempool: FIFO
- PriorityMempool: Heap-based
- TTLMempool: Time-expiring

**Consensus Strategies**:

- NullConsensus: No-op
- Custom engines registered via factory

**Pruning Strategies**:

```go
const (
    PruneNothing     PruneStrategy = "nothing"
    PruneEverything  PruneStrategy = "everything"
    PruneDefault     PruneStrategy = "default"
)

```text

---

#### Observer Pattern

**Event Bus** (pkg/events/bus.go):

```go
func (b *Bus) Subscribe(ctx context.Context, subscriber string, query abi.Query) (<-chan abi.Event, error)
func (b *Bus) Publish(ctx context.Context, event abi.Event) error

```text

**Characteristics**:

- Query-based filtering
- Buffered channels for non-blocking
- Context-aware subscription lifecycle
- Max subscriber limits for safety

---

#### Template Method Pattern

**Application Lifecycle** (pkg/abi/application.go):

```text
BeginBlock
  ↓
ExecuteTx (loop)
  ↓
EndBlock
  ↓
Commit

```text

**BaseApplication** provides default implementations; applications override specific steps.

---

#### Null Object Pattern

**NoOpBlockStore** (pkg/blockstore/noop.go):

```go
type NoOpBlockStore struct{}

func (s *NoOpBlockStore) SaveBlock(...) error {
    return types.ErrStoreClosed
}

```text

**Benefits**:

- Eliminates nil checks
- Safe default for nodes that don't need storage
- Clear error semantics

---

#### Callback Pattern

**Node Callbacks** (pkg/node/node.go):

```go
type NodeCallbacks struct {
    OnPeerConnected    func(peer.ID, bool) error
    OnPeerHandshaked   func(peer.ID, *PeerInfo) error
    OnPeerDisconnected func(peer.ID) error
    OnConsensusMessage func(peer.ID, []byte) error
    OnPeerPenalized    func(peer.ID, int64, string) error
}

```text

**Benefits**:

- Loose coupling between node and external code
- Safe invocation helpers prevent panics on nil callbacks

---

## 4. Dependency Management

### 4.1 Dependency Injection Container

**Container Implementation** (internal/container/container.go):

```go
type Container struct {
    components map[string]*componentEntry
    order      []string  // Topologically sorted
    started    bool
    mu         sync.RWMutex
}

func (c *Container) Register(name string, component Component, deps ...string) error
func (c *Container) StartAll() error  // Starts in dependency order
func (c *Container) StopAll() error   // Stops in reverse order

```text

**Features**:

- **Topological Sort**: Kahn's algorithm for dependency ordering
- **Circular Dependency Detection**: Fails fast on cycles
- **Lifecycle Hooks**: `LifecycleAware` interface for setup/teardown
- **Validation**: `ConfigurableComponent` validated before start

**Dependency Graph Example**:

```text
network
  ├── handshake-handler
  ├── pex-reactor (depends on handshake-handler)
  ├── transactions-reactor
  ├── block-reactor
  └── sync-reactor (depends on block-reactor)

```text

---

### 4.2 Dependency Declaration

**Explicit Dependencies** (pkg/node/node.go):

```go
func (n *Node) ComponentContainer() (*container.Container, error) {
    c := container.New()

    c.Register(ComponentNetwork, n.network)
    c.Register(ComponentHandshake, n.handshakeHandler, ComponentNetwork)
    c.Register(ComponentPEX, n.pexReactor, ComponentNetwork, ComponentHandshake)
    // ...
}

```text

**Interface-Based Dependencies** (types.Dependent):

```go
type Dependent interface {
    Dependencies() []string
}

```text

---

### 4.3 Constructor Injection

**ConsensusDependencies** (pkg/consensus/interface.go):

```go
type ConsensusDependencies struct {
    Network      Network
    BlockStore   blockstore.BlockStore
    StateStore   *statestore.StateStore
    Mempool      mempool.Mempool
    Application  abi.Application
    Callbacks    *ConsensusCallbacks
    Config       *ConsensusConfig
}

func (e *Engine) Initialize(deps ConsensusDependencies) error

```text

**Benefits**:

- Explicit dependencies
- Easy to test (mock injection)
- No hidden globals

---

### 4.4 Optional Dependencies

**Callbacks Pattern**:

```go
func (cb *ConsensusCallbacks) InvokeOnBlockCommitted(height int64, hash []byte) error {
    if cb != nil && cb.OnBlockCommitted != nil {
        return cb.OnBlockCommitted(height, hash)
    }
    return nil
}

```text

**Benefits**:

- Safe nil handling
- Optional features don't break core functionality

---

## 5. Module Organization

### 5.1 Package Structure

```text
blockberry/
├── pkg/              # Public API (importable)
│   ├── abi/          # Application Binary Interface (v2.0)
│   ├── blockstore/   # Block storage
│   ├── config/       # Configuration
│   ├── consensus/    # Consensus interfaces
│   ├── events/       # Event bus
│   ├── indexer/      # Transaction indexing
│   ├── logging/      # Structured logging
│   ├── mempool/      # Transaction pool
│   ├── metrics/      # Prometheus metrics
│   ├── node/         # Main Node type
│   ├── rpc/          # RPC servers
│   ├── statestore/   # State storage (AVL+)
│   ├── tracing/      # OpenTelemetry
│   └── types/        # Common types
├── internal/         # Private implementation (not importable)
│   ├── container/    # DI container
│   ├── handlers/     # Message handlers
│   ├── memory/       # Buffer pools
│   ├── p2p/          # Peer management
│   ├── pex/          # Peer exchange
│   ├── security/     # Security utilities
│   └── sync/         # Block synchronization
├── schema/           # Generated cramberry code
├── cmd/              # CLI applications
├── examples/         # Example applications
├── test/             # Integration tests
└── docs/             # Documentation

```text

---

### 5.2 Import Patterns

**Dependency Flow**:

```text
cmd → pkg → internal → schema
         ↓
    external deps

```text

**Rules**:

- `pkg/` packages can import each other
- `internal/` packages can import `pkg/` but not vice versa
- `schema/` is generated, imports nothing
- No circular dependencies between packages

---

### 5.3 Interface Segregation

**Small, Focused Interfaces**:

- `types.Component`: Just Start/Stop/IsRunning
- `types.Named`: Just Name()
- `types.HealthChecker`: Just HealthCheck()

**Composition Over Inheritance**:

```go
type ConsensusEngine interface {
    types.Component
    types.Named
    // ... consensus-specific methods
}

```text

---

### 5.4 Visibility Enforcement

**Public API** (`pkg/`):

- Stable interfaces
- Semantic versioning
- Documentation required

**Private Implementation** (`internal/`):

- Can change without notice
- Not importable by external code
- Implementation details

---

## 6. Error Handling Architecture

### 6.1 Sentinel Errors

**Package-Level Errors** (pkg/types/errors.go):

```go
var (
    ErrBlockNotFound      = errors.New("block not found")
    ErrTxNotFound         = errors.New("transaction not found")
    ErrMempoolFull        = errors.New("mempool is full")
    ErrChainIDMismatch    = errors.New("chain ID mismatch")
    ErrNoTxValidator      = errors.New("transaction validator is required")
)

```text

**Benefits**:

- Constant allocation (no runtime overhead)
- Identity comparison with `errors.Is()`
- Centralized error definitions

---

### 6.2 Error Wrapping

**Context-Rich Errors**:

```go
return fmt.Errorf("loading block at height %d: %w", height, err)

```text

**Structured Wrapping**:

```go
func WrapMessageError(err error, stream string, msgType string) error {
    return fmt.Errorf("%s/%s: %w", stream, msgType, err)
}

```text

**Benefits**:

- Preserves error chain with `%w`
- Adds context for debugging
- Compatible with `errors.Is()` and `errors.As()`

---

### 6.3 Error Checking Patterns

**Sentinel Check**:

```go
if errors.Is(err, types.ErrBlockNotFound) {
    // Handle specifically
}

```text

**Type Assertion**:

```go
var rpcErr *rpc.RPCError
if errors.As(err, &rpcErr) {
    // Handle RPC-specific error
}

```text

---

### 6.4 Fail-Closed Errors

**Default Validators Return Errors**:

```go
var DefaultTxValidator TxValidator = func(tx []byte) error {
    return fmt.Errorf("%w: no transaction validator configured", types.ErrNoTxValidator)
}

```text

**Base Application Rejects**:

```go
func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("CheckTx not implemented"),
    }
}

```text

**Benefits**:

- Secure by default
- Forces explicit implementation
- Prevents accidental acceptance

---

## 7. Code Generation

### 7.1 Cramberry Schema

**Schema Definition** (blockberry.cram):

```cramberry
message HelloRequest {
  required string node_id = 1;
  required int32 version = 2;
  required string chain_id = 5;
  required int64 timestamp = 6;
  required int64 latest_height = 7;
}

interface HandshakeMessage {
  128 = HelloRequest;
  129 = HelloResponse;
  130 = HelloFinalize;
}

```text

**Generated Code** (schema/blockberry.go):

- Marshal/Unmarshal methods
- Type ID constants
- Validation helpers
- Zero-copy optimizations

---

### 7.2 Message Type Registry

**Type IDs**:

- 128-130: Handshake messages
- 131-132: PEX messages
- 133-136: Transaction messages
- 137-138: Block sync messages
- 139: Block messages
- 140-143: Housekeeping messages
- 144-147: State sync messages
- 200-299: RPC messages (reserved)

**Polymorphic Interfaces**:

```go
var msg schema.HandshakeMessage
if err := cramberry.UnmarshalInterface(data, &msg); err != nil {
    return err
}

switch m := msg.(type) {
case *schema.HelloRequest:
    // Handle request
case *schema.HelloResponse:
    // Handle response
}

```text

---

### 7.3 Encoding Patterns

**Type-Prefixed Messages** (internal/handlers/handshake.go):

```go
func encodeHandshakeMessage(typeID cramberry.TypeID, msg Marshaler) ([]byte, error) {
    msgData, err := msg.MarshalCramberry()
    if err != nil {
        return nil, err
    }

    w := cramberry.GetWriter()
    defer cramberry.PutWriter(w)

    w.WriteTypeID(typeID)
    w.WriteRawBytes(msgData)

    return w.BytesCopy(), nil
}

```text

**Benefits**:

- Type-safe encoding/decoding
- Efficient binary format
- Built-in validation
- Zero-copy where possible

---

## 8. Testing Architecture

### 8.1 Test Organization

**Test Files**:

- `*_test.go`: Unit tests in same package
- `test/`: Integration tests
- `examples/`: Example applications (also act as integration tests)

---

### 8.2 Table-Driven Tests

**Pattern**:

```go
func TestBlockStore(t *testing.T) {
    tests := []struct {
        name    string
        height  int64
        hash    []byte
        data    []byte
        wantErr error
    }{
        {"valid block", 1, []byte("hash1"), []byte("data1"), nil},
        {"duplicate", 1, []byte("hash1"), []byte("data1"), types.ErrBlockAlreadyExists},
        // ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test logic
        })
    }
}

```text

**Benefits**:

- Comprehensive coverage
- Easy to add cases
- Clear test naming

---

### 8.3 Mock Implementations

**Null Objects for Testing**:

- `NoOpBlockStore`: Rejects all operations
- `AcceptAllApplication`: Accepts everything
- `NullConsensus`: No-op consensus
- `NewNopLogger()`: Discards all logs

**Benefits**:

- No external dependencies
- Fast test execution
- Deterministic behavior

---

### 8.4 Test Helpers

**Assertion Library**:

```go
import "github.com/stretchr/testify/require"

require.NoError(t, err)
require.Equal(t, expected, actual)
require.True(t, condition)

```text

---

### 8.5 Race Detection

**Build Tag**:

```bash
go test -race ./...

```text

**Continuous Integration**: All tests run with race detector enabled.

---

## 9. Security Architecture

### 9.1 Fail-Closed Defaults

**Transaction Validation**:

```go
var DefaultTxValidator TxValidator = func(tx []byte) error {
    return fmt.Errorf("%w: no transaction validator configured", types.ErrNoTxValidator)
}

```text

**Block Validation**:

- Sync reactor requires explicit validator before starting
- Returns `ErrNoBlockValidator` if missing

**Application Interface**:

- BaseApplication rejects all operations by default
- Forces explicit implementation

---

### 9.2 Input Validation

**Handshake Validation** (internal/handlers/handshake.go):

```go
// Validate chain ID
if *req.ChainId != h.chainID {
    _ = h.network.TempBanPeer(peerID, TempBanDurationChainMismatch, reason)
    return fmt.Errorf("%w: expected %s, got %s", types.ErrChainIDMismatch, ...)
}

// Validate protocol version
if *req.Version != h.protocolVersion {
    _ = h.network.TempBanPeer(peerID, TempBanDurationVersionMismatch, reason)
    return fmt.Errorf("%w: expected %d, got %d", types.ErrVersionMismatch, ...)
}

// Validate public key length
if len(resp.PublicKey) != ed25519.PublicKeySize {
    _ = h.network.Disconnect(peerID)
    return fmt.Errorf("%w: invalid public key length", types.ErrHandshakeFailed)
}

```text

---

### 9.3 Timing Attack Prevention

**Constant-Time Comparisons**:

- Used for cryptographic operations
- Public key validation
- Signature verification

---

### 9.4 Eclipse Attack Protection

**Peer Diversity**:

- Inbound/outbound peer limits
- Peer scoring and reputation
- Address book diversity

**Connection Limits**:

```go
if n.network.PeerManager().OutboundPeerCount() >= n.cfg.Network.MaxOutboundPeers {
    _ = n.network.Disconnect(peerID)
    return
}

```text

---

### 9.5 Rate Limiting

**Per-Peer Rate Limits**:

- Penalty system for misbehavior
- Progressive bans based on accumulated penalties
- Temporary bans vs permanent blacklist

**Penalty Decay**:

```go
func (s *PeerScorer) StartDecayLoop(stopCh chan struct{}) {
    go func() {
        ticker := time.NewTicker(s.decayInterval)
        defer ticker.Stop()

        for {
            select {
            case <-stopCh:
                return
            case <-ticker.C:
                s.decayPenalties()
            }
        }
    }()
}

```text

---

### 9.6 Shallow Copy Prevention

**Deep Copy Pattern** (fixed vulnerability):

```go
// Return deep copy to prevent mutation
return &PeerHandshakeState{
    PeerPubKey: append([]byte(nil), state.PeerPubKey...),
    // ... other fields
}

```text

**Defensive Copying in Storage** (pkg/blockstore/leveldb.go):

```go
// Create defensive copy of digest
digest := cert.Digest()
digestCopy := make([]byte, len(digest))
copy(digestCopy, digest[:])

```text

---

### 9.7 Resource Limits

**Mempool Limits**:

- Max transactions
- Max total bytes
- Max transaction size

**Subscription Limits**:

- Max subscribers
- Max subscribers per query
- Buffer size limits

---

## 10. Performance Optimizations

### 10.1 Buffer Pooling

**Cramberry Writer Pool**:

```go
w := cramberry.GetWriter()
defer cramberry.PutWriter(w)

```text

**Benefits**:

- Reduces GC pressure
- Amortizes allocation cost
- Thread-safe pool

---

### 10.2 Zero-Copy Operations

**Cramberry Reader**:

```go
r := cramberry.NewReader(data)
payload := r.Remaining()  // Returns slice, no copy

```text

---

### 10.3 Batch Operations

**LevelDB Batch Writes**:

```go
batch := new(leveldb.Batch)
batch.Put(heightKey, hash)
batch.Put(blockKey, blockValue)
batch.Put(keyMetaHeight, encodeInt64(height))
if err := s.db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
    return err
}

```text

**Benefits**:

- Atomic writes
- Reduced I/O
- Better throughput

---

### 10.4 Read-Write Lock Optimization

**Read-Heavy Structures**:

```go
type Network struct {
    tempBans   map[peer.ID]*TempBanEntry
    tempBansMu sync.RWMutex  // RWMutex for concurrent reads
}

```text

---

### 10.5 Lazy Initialization

**On-Demand Resource Creation**:

- Reactors start only when needed
- Streams created on first use
- Indexes built lazily

---

### 10.6 Pruning Strategies

**Configurable Retention**:

```go
type PruneConfig struct {
    Strategy   PruneStrategy
    KeepRecent int64  // Always keep last N blocks
    KeepEvery  int64  // Keep checkpoints every N blocks
    Interval   time.Duration
}

```text

**Background Pruning**:

- Periodic cleanup in separate goroutine
- Batched deletes to avoid blocking
- Compaction after pruning

---

## 11. Integration Patterns

### 11.1 Glueberry Integration

**Two-Phase Handshake**:

1. **StateConnected**: Send HelloRequest (unencrypted)
2. **Exchange Public Keys**: HelloResponse includes ed25519 public key
3. **PrepareStreams**: Setup encrypted streams with peer's public key
4. **StateEstablished**: Encrypted communication ready

**Stream Management**:

```go
streamNames := []string{
    "pex",
    "transactions",
    "blocksync",
    "blocks",
    "consensus",
    "housekeeping",
}

err := node.PrepareStreams(peerID, peerPubKey, streamNames)

```text

---

### 11.2 Cramberry Integration

**Schema-Driven Serialization**:

- Messages defined in `.cram` files
- Generate Go code with `cramberry generate`
- Type-safe marshal/unmarshal

**V2 Optimizations**:

- Polymorphic interfaces
- Type ID prefixing
- Zero-copy where possible

---

### 11.3 avlberry Integration

**Adapter Pattern**:

```go
type IAVLStateStore struct {
    tree *iavl.MutableTree
}

func (s *IAVLStateStore) Get(key []byte) ([]byte, error) {
    return s.tree.Get(key)
}

```text

**ICS23 Proofs**:

- ICS23-compatible merkle proofs
- Interoperability with IBC

---

### 11.4 Looseberry Integration

**DAG Mempool Adapter**:

- Implements `DAGMempool` interface
- Certificate and batch storage
- Validator set management

**Certificate Storage**:

```go
type CertificateStore interface {
    SaveCertificate(cert *loosetypes.Certificate) error
    GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error)
    GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error)
    // ...
}

```text

---

### 11.5 Prometheus Integration

**Metrics Collection**:

```go
type Metrics interface {
    SetBlockHeight(height int64)
    IncBlocksReceived()
    ObserveBlockLatency(latency time.Duration)
    // ... many more
    Handler() any  // Returns http.Handler
}

```text

**Standardized Labels**:

- `direction`: inbound/outbound
- `stream`: handshake/pex/transactions/blocks/consensus
- `reason`: disconnect/rejection reasons

---

### 11.6 OpenTelemetry Integration

**Tracing Support**:

- Span creation for operations
- Distributed tracing across nodes
- Multiple exporters (OTLP, Zipkin, stdout)

---

## 12. Key Architectural Decisions

### 12.1 Interface-Driven Design

**Decision**: All major components defined as interfaces.

**Rationale**:

- Testability through mocking
- Pluggability of implementations
- Clear contracts
- Decoupling

**Trade-off**: Slight indirection overhead (negligible in practice).

---

### 12.2 Fail-Closed Security

**Decision**: Default implementations reject all operations.

**Rationale**:

- Secure by default
- Forces explicit implementation
- Prevents accidental vulnerabilities

**Example**: `DefaultTxValidator` rejects all transactions.

---

### 12.3 Explicit Dependency Injection

**Decision**: Dependencies passed explicitly via structs.

**Rationale**:

- No hidden dependencies
- Easy to test
- Clear dependency graph
- No global state

**Example**: `ConsensusDependencies` struct.

---

### 12.4 Goroutine-Safe by Default

**Decision**: All public APIs must be thread-safe.

**Rationale**:

- Blockchain nodes are inherently concurrent
- Prevents race conditions
- Simplifies usage

**Implementation**: RWMutex for all shared state.

---

### 12.5 Pluggable Everything

**Decision**: Consensus, mempool, storage all pluggable.

**Rationale**:

- Different use cases need different strategies
- Experimentation with new algorithms
- Composability

**Implementation**: Factory pattern + interface registration.

---

### 12.6 Separation of Concerns

**Decision**: Clear boundaries between pkg/ and internal/.

**Rationale**:

- Stable public API
- Freedom to refactor internals
- Clear documentation requirements

---

### 12.7 Defensive Programming

**Decision**: Deep copies on data exposure, validation everywhere.

**Rationale**:

- Prevents mutation bugs
- Catches errors early
- Security hardening

**Example**: Deep copy in `GetPeerInfo()`.

---

## 13. Performance Characteristics

### 13.1 Lock Contention

**Analysis**:

- RWMutex used for read-heavy structures
- Lock granularity at component level
- Short critical sections

**Bottlenecks**:

- PeerManager lock during peer enumeration
- Container lock during component access

**Mitigation**:

- Snapshot-then-release pattern
- Lock-free atomics where possible

---

### 13.2 Memory Allocation

**Optimizations**:

- Buffer pooling (cramberry)
- Slice reuse
- Pre-allocation of known sizes

**GC Pressure**:

- Reduced through pooling
- Large objects pooled
- Short-lived objects in tight loops

---

### 13.3 I/O Performance

**Block Storage**:

- Batch writes to LevelDB
- Sync writes for durability
- Background compaction

**Network**:

- Buffered channels
- Non-blocking sends
- Stream multiplexing via glueberry

---

## 14. Extensibility Points

### 14.1 Custom Consensus Engines

**Registration**:

```go
consensus.RegisterConsensus("my-consensus", func(cfg *ConsensusConfig) (ConsensusEngine, error) {
    return NewMyConsensus(cfg), nil
})

```text

**Interfaces**:

- `ConsensusEngine`: Basic interface
- `BFTConsensus`: BFT-specific extensions
- `StreamAwareConsensus`: Custom network streams

---

### 14.2 Custom Mempool Implementations

**Factory Pattern**:

```go
func NewMempool(cfg config.MempoolConfig) Mempool {
    // Add custom type here
}

```text

**Extended Interfaces**:

- `DAGMempool`: For DAG-based ordering
- `NetworkAwareMempool`: For custom P2P protocols
- `PrioritizedMempool`: For priority-based ordering

---

### 14.3 Custom Storage Backends

**Implementation**:

```go
type MyBlockStore struct {
    // Custom storage
}

func (s *MyBlockStore) SaveBlock(height int64, hash []byte, data []byte) error {
    // Custom implementation
}

```text

**Optional Interfaces**:

- `PrunableBlockStore`: Add pruning support
- `CertificateStore`: Add DAG certificate storage

---

### 14.4 Custom Network Streams

**Registration**:

```go
network.RegisterStream(StreamConfig{
    Name:           "my-protocol",
    Encrypted:      true,
    RateLimit:      100,
    MaxMessageSize: 1024 * 1024,
}, handler)

```text

---

## 15. Future Architecture Considerations

### 15.1 Potential Improvements

**Performance**:

- Lock-free data structures for hot paths
- SIMD optimizations for cryptography
- Memory-mapped block storage

**Features**:

- State sync protocol
- Light client support
- Cross-chain communication (IBC)

**Scalability**:

- Sharding support
- Parallel transaction execution
- Pipelined block processing

---

### 15.2 Technical Debt

**Identified Areas**:

- Some components still use `sync.Mutex` where `sync.RWMutex` would be better
- Message handling could benefit from priority queuing
- More comprehensive metrics coverage

---

## 16. Conclusion

Blockberry demonstrates a mature, well-architected blockchain framework with:

- **Clean Architecture**: Interface-driven, layered design
- **Safety**: Fail-closed defaults, defensive programming
- **Concurrency**: Goroutine-safe, race-free
- **Extensibility**: Pluggable components via factories
- **Performance**: Optimized for blockchain workloads
- **Maintainability**: Clear separation of concerns
- **Security**: Defense in depth, input validation

The architecture balances flexibility with safety, performance with maintainability, and simplicity with power. It provides a solid foundation for building production blockchain applications.

---

## Appendix A: Key Files Reference

| Component | Primary Interface | Implementation(s) | Location |
|-----------|-------------------|-------------------|----------|
| Application | `abi.Application` | BaseApplication | pkg/abi/ |
| BlockStore | `blockstore.BlockStore` | LevelDB, BadgerDB, Memory | pkg/blockstore/ |
| Mempool | `mempool.Mempool` | Simple, Priority, TTL | pkg/mempool/ |
| Consensus | `consensus.ConsensusEngine` | NullConsensus | pkg/consensus/ |
| StateStore | `statestore.StateStore` | IAVLStateStore | pkg/statestore/ |
| Node | `node.Node` | Node | pkg/node/ |
| Network | `p2p.Network` | Network | internal/p2p/ |
| Container | `container.Container` | Container | internal/container/ |
| EventBus | `abi.EventBus` | Bus | pkg/events/ |

---

## Appendix B: Concurrency Patterns Summary

| Pattern | Use Case | Example |
|---------|----------|---------|
| Event Loop | Message processing | Node.eventLoop() |
| Background Worker | Periodic tasks | HandshakeHandler.timeoutLoop() |
| One-Shot Async | Fire-and-forget | Disconnect peer |
| Producer-Consumer | Channel communication | Events/Messages |
| Stop Channel | Graceful shutdown | Close(stopCh) |
| WaitGroup | Wait for goroutines | wg.Wait() |
| Atomic Bool | Lock-free flags | stopping.Load() |
| RWMutex | Read-heavy data | tempBansMu |
| Snapshot-Release | Reduce lock hold time | Snapshot callbacks |
| Deep Copy | Safe data exposure | GetPeerInfo() |

---

## Appendix C: Design Pattern Catalog

| Pattern | Implementation | Location |
|---------|----------------|----------|
| Factory | ConsensusFactory | pkg/consensus/factory.go |
| Builder | RoleBuilder | pkg/node/role_builder.go |
| Adapter | GlueberryStreamAdapter | internal/p2p/stream_adapter.go |
| Decorator | PruningWrapper | pkg/blockstore/pruning.go |
| Facade | Node | pkg/node/node.go |
| Strategy | Mempool types | pkg/mempool/ |
| Observer | EventBus | pkg/events/bus.go |
| Template Method | Application lifecycle | pkg/abi/application.go |
| Null Object | NoOpBlockStore | pkg/blockstore/noop.go |
| Singleton | DefaultFactory | pkg/consensus/factory.go |
| Dependency Injection | Container | internal/container/container.go |

---

**End of Architecture Analysis**
