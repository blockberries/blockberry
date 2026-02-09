# Blockberry Architecture

**Version:** 1.0
**Date:** 2026-02-02
**Go Version:** 1.25.6
**Module:** github.com/blockberries/blockberry

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Architecture Principles](#2-architecture-principles)
3. [System Architecture](#3-system-architecture)
4. [Go Package Architecture](#4-go-package-architecture)
5. [Core Components](#5-core-components)
6. [Concurrency Architecture](#6-concurrency-architecture)
7. [Network Architecture](#7-network-architecture)
8. [Storage Architecture](#8-storage-architecture)
9. [API Design](#9-api-design)
10. [Configuration](#10-configuration)
11. [Testing Strategy](#11-testing-strategy)
12. [Performance Considerations](#12-performance-considerations)
13. [Error Handling](#13-error-handling)
14. [Security Architecture](#14-security-architecture)
15. [Deployment Architecture](#15-deployment-architecture)
16. [Dependency Management](#16-dependency-management)
17. [Extension Points](#17-extension-points)
18. [Development Workflow](#18-development-workflow)
19. [Future Roadmap](#19-future-roadmap)
20. [Glossary](#20-glossary)

---

## 1. Project Overview

### 1.1 Description

Blockberry is a **generic blockchain node framework** written in Go that provides the fundamental building blocks for constructing blockchain networks. It is **not** a consensus engine but rather a composable framework that allows applications to implement their own consensus algorithms while leveraging production-ready networking, storage, and transaction pooling infrastructure.

### 1.2 Purpose and Scope

**Purpose:**

- Provide a modular, extensible framework for building blockchain nodes
- Abstract away P2P networking complexity via Glueberry integration
- Offer pluggable consensus, mempool, and storage implementations
- Enable developers to focus on application-specific logic rather than infrastructure

**Scope:**

- P2P networking and peer management
- Transaction mempool with multiple strategies
- Block and state storage
- Message serialization and validation
- Event system and metrics collection
- RPC servers (gRPC, JSON-RPC, WebSocket)

**Non-Goals:**

- Built-in consensus algorithm (applications provide their own)
- Virtual machine or smart contract execution (application layer)
- Wallet functionality (separate concern)

### 1.3 Key Dependencies

| Dependency | Version/Location | Purpose |
|------------|------------------|---------|
| **Glueberry** | `../glueberry` (v1.2.10) | Secure P2P networking with encrypted streams |
| **Cramberry** | `../cramberry` (v1.5.5) | High-performance binary serialization with polymorphic support |
| **Looseberry** | `../looseberry` (local) | DAG-based mempool with certified batch ordering |
| **avlberry** | `../avlberry` (local) | AVL+ tree for merkleized state storage |
| **LevelDB** | syndtr/goleveldb | Persistent key-value store for blocks |
| **BadgerDB** | dgraph-io/badger v4.9.0 | Alternative high-performance storage backend |
| **Prometheus** | client_golang v1.22.0 | Metrics collection and exposition |
| **OpenTelemetry** | v1.39.0 | Distributed tracing instrumentation |
| **gRPC** | google.golang.org/grpc v1.78.0 | High-performance RPC framework |

### 1.4 Project Goals

1. **Modularity**: All major components are pluggable via interfaces
2. **Safety**: Fail-closed defaults, defensive programming, comprehensive validation
3. **Performance**: Optimized for blockchain workloads (batching, pooling, zero-copy)
4. **Observability**: Rich metrics, tracing, and structured logging throughout
5. **Correctness**: Deterministic execution, race-free concurrency
6. **Extensibility**: Clear extension points for custom implementations

### 1.5 Non-Goals

- **Not a consensus engine**: Applications must implement their own consensus
- **Not a smart contract platform**: No built-in VM or contract execution
- **Not application-specific**: Framework is generic for any blockchain type
- **Not a wallet**: Key management and transaction signing are application concerns

---

## 2. Architecture Principles

### 2.1 Design Philosophy

Blockberry's architecture is built on several core principles derived from distributed systems best practices and blockchain-specific requirements.

#### Layered Independence

Each architectural layer operates independently with well-defined interfaces:

```text
┌─────────────────────────────────────────┐
│        Application Layer (ABI)          │  ← Custom blockchain logic
├─────────────────────────────────────────┤
│       Consensus Layer (Pluggable)       │  ← BFT, DAG, PoS, etc.
├─────────────────────────────────────────┤
│       Mempool Layer (Pluggable)         │  ← FIFO, Priority, DAG
├─────────────────────────────────────────┤
│     Network Layer (Glueberry P2P)       │  ← Encrypted streams
├─────────────────────────────────────────┤
│  Storage Layer (BlockStore, StateStore) │  ← Persistent storage
└─────────────────────────────────────────┘

```text

**Benefits:**

- Each layer can be developed, tested, and deployed independently
- Layers communicate only through defined interfaces
- Easy to swap implementations (e.g., different mempool strategies)
- Clear separation of concerns reduces coupling

#### Callback Inversion

Rather than applications calling into the framework, the framework calls registered application handlers:

```go
// Framework calls application
type Application interface {
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult
    ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult
    Commit(ctx context.Context) *CommitResult
}

// Application registers with framework
node.SetApplication(myApp)

```text

**Benefits:**

- Framework controls execution flow and timing
- Applications respond to well-defined lifecycle events
- Easier to reason about concurrent execution
- Framework can optimize batching and ordering

#### Async by Default

All I/O operations are asynchronous with explicit synchronization points:

```go
// Non-blocking send
node.Send(peerID, "transactions", txData)

// Explicit synchronization
result := app.Commit(ctx)  // Blocks until state committed

```text

**Benefits:**

- High throughput for network operations
- Framework can batch and pipeline operations
- Responsive to multiple concurrent events
- Clear points where ordering matters (Commit)

#### Fail-Closed Security

Default behaviors deny/reject; applications must explicitly enable acceptance:

```go
// Default validator rejects all transactions
var DefaultTxValidator = func(tx []byte) error {
    return fmt.Errorf("%w: no validator configured", ErrNoTxValidator)
}

// BaseApplication rejects all operations
type BaseApplication struct{}

func (a *BaseApplication) CheckTx(...) *TxCheckResult {
    return &TxCheckResult{Code: CodeNotAuthorized}
}

```text

**Benefits:**

- Secure by default, even if misconfigured
- Forces developers to think about validation
- Prevents accidental acceptance of invalid data
- Reduces attack surface

### 2.2 Core Principles from ABI Design

From ABI_DESIGN.md, the following principles guide the entire architecture:

1. **Deterministic Execution**: Same inputs produce same outputs across all nodes
2. **Zero-Copy Where Possible**: Minimize data copying across boundaries using slices and pooling
3. **Composability**: Mix and match components (mempools, consensus, storage)
4. **Testability**: Each component can be unit tested in isolation
5. **Observability**: Rich metrics and tracing at every boundary

### 2.3 Go Idioms Applied

Blockberry leverages Go's strengths:

**Interface-Driven Design:**

```go
type Mempool interface {
    AddTx(tx []byte) error
    ReapTxs(maxBytes int64) [][]byte
}

```text

**Struct Embedding for Composition:**

```go
type MyApp struct {
    *abi.BaseApplication  // Inherit safe defaults
    customState *State
}

```text

**Goroutine Safety:**

- All public APIs protected by mutexes
- Atomic operations for flags
- Channel-based communication
- Defensive copying on data exposure

**Error Wrapping:**

```go
return fmt.Errorf("loading block at height %d: %w", height, err)

```text

### 2.4 SOLID Principles in Practice

**Single Responsibility:**

- Each component has one well-defined purpose
- `BlockStore` only handles block persistence
- `Mempool` only manages pending transactions

**Open/Closed:**

- Open for extension via interfaces
- Closed for modification (stable public API)
- New consensus engines via `ConsensusEngine` interface

**Liskov Substitution:**

- Any `BlockStore` implementation is interchangeable
- Framework works with any `Application` implementation

**Interface Segregation:**

- Small, focused interfaces (`Component`, `Named`, `HealthChecker`)
- Extended interfaces for optional capabilities (`DAGMempool`, `PrioritizedMempool`)

**Dependency Inversion:**

- High-level components depend on abstractions, not concrete implementations
- Dependency injection via constructors and configuration

### 2.5 Fail-Closed Security Model

Every entry point defaults to rejection:

1. **Transaction Validation**: Default validator rejects all
2. **Block Validation**: Must explicitly set validator
3. **Application Methods**: `BaseApplication` rejects all operations
4. **Peer Connections**: Chain ID and version mismatch → permanent ban
5. **Message Handling**: Invalid format → disconnect + temporary ban

### 2.6 Plugin Architecture Philosophy

Blockberry is designed as a framework, not a monolith:

- **Consensus**: Register custom engines via factory
- **Mempool**: Swap strategies (Simple, Priority, TTL, DAG)
- **Storage**: Choose backend (LevelDB, BadgerDB, custom)
- **Network Streams**: Add custom protocols
- **Event Handlers**: Subscribe to system events

---

## 3. System Architecture

### 3.1 High-Level Architecture Diagram

```text
┌──────────────────────────────────────────────────────────────────────────┐
│                          BLOCKBERRY NODE                                  │
│                                                                           │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                     Application Layer (ABI v2.0)                    │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │  │
│  │  │  CheckTx     │  │  ExecuteTx   │  │  Commit      │             │  │
│  │  │  (Validate)  │  │  (Apply)     │  │  (Finalize)  │             │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘             │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                   ▲                                       │
│                                   │ Lifecycle Calls                       │
│  ┌────────────────────────────────┴───────────────────────────────────┐  │
│  │                    Consensus Layer (Pluggable)                      │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │  │
│  │  │  BFT Engine  │  │  Null Engine │  │  Custom...   │             │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘             │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                   ▲                                       │
│                                   │ Block Proposals                       │
│  ┌────────────────────────────────┴───────────────────────────────────┐  │
│  │                     Mempool Layer (Pluggable)                       │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │  │
│  │  │ Simple FIFO  │  │   Priority   │  │ DAG (Loose)  │             │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘             │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                   ▲                                       │
│                                   │ Transaction Gossip                    │
│  ┌────────────────────────────────┴───────────────────────────────────┐  │
│  │               Network Layer (Glueberry P2P)                         │  │
│  │  ┌──────────────────────────────────────────────────────────────┐  │  │
│  │  │  Encrypted Streams: handshake, pex, tx, blocks, consensus   │  │  │
│  │  └──────────────────────────────────────────────────────────────┘  │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │  │
│  │  │ Peer Manager │  │  Discovery   │  │ Rate Limiter │             │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘             │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                   ▲                                       │
│                                   │ Persist/Retrieve                      │
│  ┌────────────────────────────────┴───────────────────────────────────┐  │
│  │                     Storage Layer                                   │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐             │  │
│  │  │  BlockStore  │  │  StateStore  │  │  CertStore   │             │  │
│  │  │  (LevelDB)   │  │  (AVL+)      │  │  (Loosebrry) │             │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘             │  │
│  └────────────────────────────────────────────────────────────────────┘  │
│                                                                           │
│  ┌────────────────────────────────────────────────────────────────────┐  │
│  │                   Cross-Cutting Concerns                            │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │  │
│  │  │ Metrics  │  │  Tracing │  │  Logging │  │  Events  │           │  │
│  │  │(Promeths)│  │  (OTEL)  │  │  (slog)  │  │  (Bus)   │           │  │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────┘           │  │
│  └────────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────────┘

```text

### 3.2 Layer Breakdown

#### Application Layer (ABI v2.0)

**Responsibilities:**

- Define state machine logic (transaction validation, execution)
- Manage application-specific state
- Emit events for indexing
- Update validator sets (if applicable)

**Key Interfaces:**

- `Application`: Core lifecycle methods
- `Validator`: Validator representation
- `TxCheckResult`, `TxExecResult`: Standardized responses

**Implementation Pattern:**

```go
type MyApp struct {
    *abi.BaseApplication  // Safe defaults
    state map[string][]byte
}

func (app *MyApp) ExecuteTx(ctx context.Context, tx *abi.Transaction) *abi.TxExecResult {
    // Parse, validate, apply transaction
    // Update app.state
    // Return result with events
}

```text

#### Consensus Layer (Pluggable Engines)

**Responsibilities:**

- Order transactions into blocks
- Achieve agreement on block validity
- Drive application lifecycle (BeginBlock, ExecuteTx, EndBlock, Commit)
- Handle validator set changes

**Key Interfaces:**

- `ConsensusEngine`: Base interface
- `BFTConsensus`: Extended for BFT protocols
- `StreamAwareConsensus`: Custom network protocols

**Implementations:**

- `NullConsensus`: No-op for full nodes
- Custom engines registered via factory pattern

#### Mempool Layer (Pluggable Implementations)

**Responsibilities:**

- Store pending transactions
- Validate transactions before inclusion
- Provide transactions for block proposals
- Handle transaction removal after commit
- Track recent transactions (deduplication)

**Key Interfaces:**

- `Mempool`: Core interface
- `PrioritizedMempool`: Priority ordering
- `ExpirableMempool`: TTL-based expiration
- `DAGMempool`: DAG-based ordering (Looseberry)

**Implementations:**

- `SimpleMempool`: Hash-based FIFO storage
- `PriorityMempool`: Heap-based priority queue
- `TTLMempool`: Time-expiring transactions
- Looseberry adapter: DAG with certified batches

#### Network Layer (Glueberry P2P)

**Responsibilities:**

- Establish encrypted peer connections
- Multiplex streams (handshake, pex, tx, blocks, consensus, etc.)
- Peer discovery via PEX
- Rate limiting and peer scoring
- Message routing to handlers

**Key Components:**

- `Network`: P2P interface wrapper
- `PeerManager`: Per-peer state tracking
- `HandshakeHandler`: Two-phase handshake
- Stream handlers: PEX, Transactions, Blocks, BlockSync, Consensus, Housekeeping

**Stream Types:**
| Stream | Encrypted | Purpose |
|--------|-----------|---------|
| `handshake` | No | Initial connection setup |
| `pex` | Yes | Peer exchange |
| `transactions` | Yes | Transaction gossip |
| `blocks` | Yes | Real-time block propagation |
| `blocksync` | Yes | Batch historical sync |
| `consensus` | Yes | Consensus messages (raw bytes) |
| `housekeeping` | Yes | Latency probes |
| `statesync` | Yes | Snapshot sync |

#### Storage Layer

**Responsibilities:**

- Persist blocks and metadata
- Store application state (merkleized)
- Support historical queries
- Provide pruning strategies
- Certificate storage for DAG mempool

**Key Interfaces:**

- `BlockStore`: Block persistence
- `StateStore`: Merkleized key-value store
- `CertificateStore`: DAG certificate storage

**Implementations:**

- `LevelDBBlockStore`: Production-ready block storage
- `BadgerDBBlockStore`: High-performance alternative
- `IAVLStateStore`: avlberry AVL+ merkle tree wrapper
- `MemoryBlockStore`: In-memory for testing

### 3.3 Component Interaction Diagram

```text
┌─────────────┐
│   Client    │ Submits Tx
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────────────────────────┐
│                         Node                                 │
│                                                              │
│  ┌──────────────┐     ┌──────────────┐     ┌────────────┐  │
│  │   Network    │────▶│   Mempool    │────▶│ Consensus  │  │
│  │  (receives)  │     │ (validates)  │     │ (orders)   │  │
│  └──────────────┘     └──────────────┘     └─────┬──────┘  │
│                                                   │          │
│                                                   ▼          │
│                                         ┌──────────────────┐│
│                                         │   Application    ││
│                                         │ (BeginBlock,     ││
│                                         │  ExecuteTx,      ││
│                                         │  EndBlock,       ││
│                                         │  Commit)         ││
│                                         └─────┬────────────┘│
│                                               │              │
│                                               ▼              │
│                          ┌──────────────────────────────┐   │
│                          │  BlockStore   │  StateStore  │   │
│                          │  (persist)    │  (commit)    │   │
│                          └──────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘

```text

### 3.4 Data Flow

#### Transaction Lifecycle

1. **Receive**: Peer broadcasts transaction
2. **Validate**: `Mempool.AddTx` → `Application.CheckTx`
3. **Store**: Add to mempool if valid
4. **Gossip**: Broadcast to other peers
5. **Propose**: Consensus reaps transactions
6. **Execute**: `Application.ExecuteTx` for each tx
7. **Commit**: `Application.Commit` finalizes state
8. **Remove**: Mempool removes committed transactions

```text
Peer → Network → Mempool.AddTx → App.CheckTx → Mempool (stored)
                                                     │
                                                     ▼
                                               Gossip to peers
                                                     │
                                                     ▼
Consensus.ReapTxs ← Mempool ← Block Proposal
       │
       ▼
App.BeginBlock → App.ExecuteTx (loop) → App.EndBlock → App.Commit
       │                                                      │
       ▼                                                      ▼
BlockStore.SaveBlock                              StateStore.Commit
       │                                                      │
       └──────────────────────────────────────────────────────┘
                              │
                              ▼
                    Mempool.RemoveTxs

```text

#### Block Lifecycle

1. **Propose**: Proposer creates block from mempool
2. **Broadcast**: Send to validators
3. **Validate**: Each validator checks block
4. **Vote**: Validators vote on validity
5. **Commit**: 2f+1 votes → commit
6. **Execute**: Call application lifecycle
7. **Store**: Persist block and state
8. **Announce**: Broadcast to full nodes

```text
Proposer                          Validators                    Full Nodes
    │                                 │                             │
    │ 1. Reap Mempool                 │                             │
    ▼                                 │                             │
CreateBlock                           │                             │
    │                                 │                             │
    │ 2. Broadcast Proposal           │                             │
    ├────────────────────────────────▶│                             │
    │                                 │                             │
    │                                 ▼                             │
    │                          Validate Block                       │
    │                                 │                             │
    │                                 ▼                             │
    │                           Vote (Prevote)                      │
    │                                 │                             │
    │ 3. Collect Votes                │                             │
    │◀────────────────────────────────┤                             │
    │                                 │                             │
    ▼                                 ▼                             │
+2/3 Prevotes                   +2/3 Prevotes                       │
    │                                 │                             │
    ▼                                 ▼                             │
Vote (Precommit)              Vote (Precommit)                      │
    │                                 │                             │
    │ 4. Collect Precommits           │                             │
    │◀────────────────────────────────┤                             │
    │                                 │                             │
    ▼                                 ▼                             │
+2/3 Precommits               +2/3 Precommits                       │
    │                                 │                             │
    ▼                                 ▼                             │
Commit Block                   Commit Block                         │
    │                                 │                             │
    │ 5. Announce Block               │                             │
    ├─────────────────────────────────┴────────────────────────────▶│
    │                                                                ▼
    │                                                         Sync & Apply
    ▼                                                                │
BeginBlock → ExecuteTx → EndBlock → Commit                          │
    │                                                                │
    ▼                                                                ▼
SaveBlock + SaveState                                   SaveBlock + SaveState

```text

### 3.5 Request/Response Lifecycle

#### RPC Query

```text
Client                      Node                    Application
   │                          │                           │
   │  HTTP/gRPC Query         │                           │
   ├─────────────────────────▶│                           │
   │                          │  Query(path, data)        │
   │                          ├──────────────────────────▶│
   │                          │                           │
   │                          │  QueryResponse            │
   │                          │◀──────────────────────────┤
   │  Response                │                           │
   │◀─────────────────────────┤                           │

```text

#### Transaction Submission

```text
Client              RPC Server           Mempool            Network
   │                    │                   │                  │
   │  SubmitTx          │                   │                  │
   ├───────────────────▶│                   │                  │
   │                    │  AddTx            │                  │
   │                    ├──────────────────▶│                  │
   │                    │                   │  Validate        │
   │                    │                   │  (CheckTx)       │
   │                    │                   ▼                  │
   │                    │                 Accept               │
   │                    │                   │  Broadcast       │
   │                    │                   ├─────────────────▶│
   │                    │  TxHash           │                  │
   │  Response          │◀──────────────────┤                  │
   │◀───────────────────┤                   │                  │

```text

---

## 4. Go Package Architecture

### 4.1 Package Organization Philosophy

Blockberry follows Go's standard project layout with clear separation between public API (`pkg/`) and private implementation (`internal/`).

```text
blockberry/
├── cmd/                    # Command-line applications
│   └── blockberry/        # Main CLI entry point
│
├── pkg/                   # Public API (stable, importable)
│   ├── abi/              # Application Binary Interface v2.0
│   ├── blockstore/       # Block storage interfaces and implementations
│   ├── config/           # Configuration structures and loading
│   ├── consensus/        # Consensus engine interfaces and factory
│   ├── events/           # Event bus for pub/sub
│   ├── indexer/          # Transaction indexing
│   ├── logging/          # Structured logging wrappers
│   ├── mempool/          # Transaction pool interfaces and implementations
│   ├── metrics/          # Prometheus metrics collection
│   ├── node/             # Main Node type and lifecycle
│   ├── rpc/              # RPC servers (gRPC, JSON-RPC, WebSocket)
│   ├── statestore/       # AVL+ merkleized state storage (avlberry)
│   ├── tracing/          # OpenTelemetry distributed tracing
│   └── types/            # Common types and interfaces
│
├── internal/              # Private implementation (not importable)
│   ├── container/        # Dependency injection container
│   ├── handlers/         # Message handlers for P2P streams
│   ├── memory/           # Buffer pools and memory management
│   ├── p2p/              # Peer management internals
│   ├── pex/              # Peer exchange reactor
│   ├── security/         # Security utilities
│   └── sync/             # Block synchronization
│
├── schema/                # Generated cramberry code (DO NOT EDIT)
│   └── blockberry.go     # Generated from blockberry.cram
│
├── examples/              # Example applications
│   ├── simple/           # Minimal example
│   └── config.example.toml
│
├── test/                  # Integration tests
│   └── integration/
│
└── docs/                  # Documentation

```text

**Rationale:**

- `pkg/`: Stable public API, semantic versioning applies
- `internal/`: Implementation details, can change without notice
- `schema/`: Generated code, never manually edited
- `cmd/`: Binary entry points
- `examples/`: Demonstrate usage patterns
- `test/`: End-to-end integration tests

### 4.2 Module Structure

```go
module github.com/blockberries/blockberry

go 1.25.6

require (
    github.com/blockberries/cramberry v1.5.5
    github.com/blockberries/glueberry v1.2.10
    github.com/blockberries/looseberry v0.0.0  // Local replace
    github.com/blockberries/avlberry v0.0.0  // Local replace
    // ... more dependencies
)

replace github.com/blockberries/looseberry => ../looseberry
replace github.com/blockberries/avlberry => ../avlberry

```text

### 4.3 Dependency Graph

```text
cmd/blockberry
    └─▶ pkg/node
            ├─▶ pkg/abi
            ├─▶ pkg/mempool
            │       └─▶ pkg/types
            ├─▶ pkg/consensus
            │       ├─▶ pkg/types
            │       └─▶ pkg/abi
            ├─▶ pkg/blockstore
            │       └─▶ pkg/types
            ├─▶ pkg/statestore
            │       └─▶ pkg/types
            ├─▶ pkg/rpc
            │       └─▶ pkg/types
            ├─▶ pkg/events
            ├─▶ pkg/metrics
            ├─▶ pkg/logging
            ├─▶ pkg/tracing
            └─▶ internal/
                    ├─▶ internal/p2p
                    ├─▶ internal/handlers
                    ├─▶ internal/pex
                    ├─▶ internal/sync
                    ├─▶ internal/container
                    └─▶ schema/

```text

**Dependency Rules:**

1. `pkg/` packages can import other `pkg/` packages
2. `internal/` packages can import `pkg/` packages
3. `pkg/` packages CANNOT import `internal/` packages
4. `schema/` is generated, imports nothing from Blockberry
5. No circular dependencies between packages

### 4.4 Export Policy

#### pkg/ (Public API)

**Exported:**

- Interfaces for all major components
- Configuration structures
- Result and error types
- Factory functions (`NewNode`, `NewMempool`, etc.)

**Not Exported:**

- Internal state structures
- Low-level implementation details
- Helper functions not meant for external use

**Example:**

```go
// pkg/mempool/mempool.go

// Exported interface
type Mempool interface {
    AddTx(tx []byte) error
    ReapTxs(maxBytes int64) [][]byte
}

// Exported factory
func NewSimpleMempool(maxTxs int, maxBytes int64) *SimpleMempool

// Exported implementation (fields unexported)
type SimpleMempool struct {
    mu       sync.RWMutex          // Unexported
    txs      map[string]*txEntry   // Unexported
    maxTxs   int                   // Unexported
}

// Exported methods on exported type
func (m *SimpleMempool) AddTx(tx []byte) error

// Unexported helper
func (m *SimpleMempool) hasCapacity() bool  // Unexported

```text

#### internal/ (Private Implementation)

Everything in `internal/` is unexported to external packages by Go's visibility rules. Only Blockberry itself can import from `internal/`.

### 4.5 Interface-Driven Design

Every major component is defined as an interface first:

```go
// pkg/blockstore/store.go
type BlockStore interface {
    SaveBlock(height int64, hash []byte, data []byte) error
    LoadBlock(height int64) (hash []byte, data []byte, err error)
    HasBlock(height int64) bool
    Height() int64
    Close() error
}

```text

**Benefits:**

- Easy to mock for testing
- Swap implementations without changing code
- Clear contracts between components
- Supports dependency injection

### 4.6 Code Generation (Cramberry Schema)

Message definitions live in `blockberry.cram`:

```cramberry
message HelloRequest {
  required string node_id = 1;
  required int32 version = 2;
  required string chain_id = 5;
}

interface HandshakeMessage {
  128 = HelloRequest;
  129 = HelloResponse;
  130 = HelloFinalize;
}

```text

Generated Go code in `schema/blockberry.go`:

```go
// Generated by cramberry - DO NOT EDIT

type HelloRequest struct {
    NodeId  *string
    Version *int32
    ChainId *string
}

func (m *HelloRequest) MarshalCramberry() ([]byte, error)
func (m *HelloRequest) UnmarshalCramberry(data []byte) error

// Type ID for polymorphic dispatch
const TypeIDHelloRequest cramberry.TypeID = 128

```text

**Generation Command:**

```bash
cramberry generate -lang go -out ./schema ./blockberry.cram

```text

---

## 5. Core Components

This section documents each major component in detail, including its responsibilities, interfaces, implementation patterns, and usage examples.

### 5.1 Node (pkg/node/)

#### Overview

The **Node** is the central coordinator that aggregates all components (network, mempool, storage, handlers) and manages their lifecycle. It wraps a Glueberry node for P2P networking and routes messages to appropriate handlers.

#### File: pkg/node/node.go

**Primary Structure:**

```go
type Node struct {
    cfg              *config.Config
    glueNode         *glueberry.Node
    network          *p2p.Network
    mempool          mempool.Mempool
    blockStore       blockstore.BlockStore
    stateStore       *statestore.StateStore
    consensus        consensus.ConsensusEngine

    // Handlers
    handshakeHandler *handlers.HandshakeHandler
    pexReactor       *pex.Reactor
    txReactor        *handlers.TransactionsHandler
    blockReactor     *handlers.BlocksHandler
    syncReactor      *sync.Reactor

    // Lifecycle
    stopping         atomic.Bool
    stopCh           chan struct{}
    wg               sync.WaitGroup

    // Callbacks
    callbacks        *types.NodeCallbacks
    mu               sync.RWMutex
}

```text

#### Lifecycle Management

**Node Builder Pattern:**

```go
func (b *Builder) AsValidator() *Builder
func (b *Builder) AsFullNode() *Builder
func (b *Builder) AsSeedNode() *Builder
func (b *Builder) AsLightClient() *Builder
func (b *Builder) AsArchiveNode() *Builder

```text

**Functional Options Pattern:**

```go
func WithMempool(mp mempool.Mempool) Option
func WithBlockStore(bs blockstore.BlockStore) Option
func WithBlockValidator(v types.BlockValidator) Option
func WithCallbacks(cb *types.NodeCallbacks) Option

```text

**Example Usage:**

```go
cfg, _ := config.LoadConfig("config.toml")

node, err := node.NewNode(cfg,
    node.WithBlockValidator(myApp.ValidateBlock),
    node.WithCallbacks(&types.NodeCallbacks{
        OnPeerConnected: func(id peer.ID, outbound bool) {
            log.Printf("Peer connected: %s", id)
        },
    }),
)

if err := node.Start(); err != nil {
    log.Fatal(err)
}
defer node.Stop()

```text

#### Role-Based Configuration

| Role | Validates Blocks | Participates in Consensus | Stores Full History | Serves Queries |
|------|------------------|---------------------------|---------------------|----------------|
| **Validator** | ✓ | ✓ | ✓ | ✓ |
| **Full Node** | ✓ | ✗ | ✓ | ✓ |
| **Seed Node** | ✗ | ✗ | ✗ | ✗ |
| **Light Client** | ✓ (headers) | ✗ | ✗ (headers only) | Limited |
| **Archive Node** | ✓ | ✗ | ✓ (no pruning) | ✓ |

**Configuration:**

```toml
role = "full"  # validator, full, seed, light, archive

```text

#### Component Coordination

The Node uses a dependency injection container to manage component startup order:

```go
func (n *Node) ComponentContainer() (*container.Container, error) {
    c := container.New()

    // Register components with dependencies
    c.Register(ComponentNetwork, n.network)
    c.Register(ComponentHandshake, n.handshakeHandler, ComponentNetwork)
    c.Register(ComponentPEX, n.pexReactor, ComponentNetwork, ComponentHandshake)
    c.Register(ComponentTransactions, n.txReactor, ComponentNetwork)
    c.Register(ComponentBlocks, n.blockReactor, ComponentNetwork)
    c.Register(ComponentSync, n.syncReactor, ComponentNetwork, ComponentBlocks)

    return c, nil
}

```text

**Startup Sequence:**

1. Start network layer (Glueberry)
2. Start handshake handler
3. Start PEX reactor (depends on handshake)
4. Start transaction, block, sync reactors
5. Start event loop
6. Connect to seed nodes

**Shutdown Sequence:**

1. Signal shutdown (`stopping.Store(true)`)
2. Stop event loop (`close(stopCh)`)
3. Wait for goroutines with timeout
4. Stop reactors in reverse order
5. Close storage backends

#### Event Loop Pattern

```go
func (n *Node) eventLoop() {
    defer n.wg.Done()

    for {
        select {
        case <-n.stopCh:
            return

        case event := <-n.glueNode.Events():
            n.handleConnectionEvent(event)

        case msg := <-n.glueNode.Messages():
            n.handleMessage(msg)
        }
    }
}

```text

---

### 5.2 Application Binary Interface (pkg/abi/)

#### Overview

The **ABI** defines the contract between the blockchain framework and application-specific logic. Applications implement the `Application` interface to define their state machine behavior.

#### File: pkg/abi/application.go

**Core Interface:**

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

#### Lifecycle Methods

**1. InitChain** (called once at genesis)

```go
func (app *MyApp) InitChain(genesis *Genesis) error {
    // Initialize state from genesis
    // Set initial validators
    // Return error if initialization fails
}

```text

**2. CheckTx** (concurrent validation)

```go
func (app *MyApp) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    // Lightweight validation for mempool
    // Check signature, nonce, balance
    // Must NOT modify state
    // Must be thread-safe

    return &TxCheckResult{
        Code:      CodeOK,
        GasWanted: 1000,
        Priority:  calculatePriority(tx),
    }
}

```text

**3. BeginBlock** (start of block execution)

```go
func (app *MyApp) BeginBlock(ctx context.Context, header *BlockHeader) error {
    // Prepare for block execution
    // Process evidence (slash validators)
    // Update block-level state
}

```text

**4. ExecuteTx** (execute single transaction)

```go
func (app *MyApp) ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult {
    // Parse transaction
    // Apply state changes
    // Emit events
    // Must be deterministic

    return &TxExecResult{
        Code:    CodeOK,
        Events:  events,
        GasUsed: 850,
    }
}

```text

**5. EndBlock** (finalize block)

```go
func (app *MyApp) EndBlock(ctx context.Context) *EndBlockResult {
    // Return validator updates
    // Return consensus parameter changes

    return &EndBlockResult{
        ValidatorUpdates: validatorUpdates,
    }
}

```text

**6. Commit** (persist state)

```go
func (app *MyApp) Commit(ctx context.Context) *CommitResult {
    // Compute state hash
    // Persist state to disk
    // Must be deterministic across all nodes

    return &CommitResult{
        AppHash: stateHash,
    }
}

```text

**7. Query** (read state, concurrent)

```go
func (app *MyApp) Query(ctx context.Context, req *QueryRequest) *QueryResponse {
    // Read application state
    // Support historical queries (req.Height)
    // Optionally generate merkle proof
    // Must be thread-safe

    return &QueryResponse{
        Code:   CodeOK,
        Value:  value,
        Height: app.height,
    }
}

```text

#### BaseApplication Implementation

Provides fail-closed defaults (rejects all operations):

```go
type BaseApplication struct{}

func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("CheckTx not implemented"),
    }
}

```text

**Usage Pattern:**

```go
type MyApp struct {
    *abi.BaseApplication  // Inherit safe defaults
    state *AppState
}

// Override only the methods you need
func (app *MyApp) CheckTx(...) *TxCheckResult {
    // Custom implementation
}

```text

#### Callback Inversion Pattern

The framework drives execution; applications respond to callbacks:

```text
Framework                     Application
    │                              │
    │  1. BeginBlock(header)       │
    ├─────────────────────────────▶│
    │                              │
    │  2. ExecuteTx(tx1)           │
    ├─────────────────────────────▶│
    │                              │
    │  3. ExecuteTx(tx2)           │
    ├─────────────────────────────▶│
    │                              │
    │  4. EndBlock()               │
    ├─────────────────────────────▶│
    │                              │
    │  5. Commit()                 │
    ├─────────────────────────────▶│
    │                              │
    │  6. Query(req)               │
    ├─────────────────────────────▶│

```text

#### Event System

Applications emit events during execution:

```go
type Event struct {
    Type       string
    Attributes []Attribute
}

type Attribute struct {
    Key   string
    Value []byte
    Index bool  // Should this be indexed?
}

// Example
result := &TxExecResult{
    Code: CodeOK,
    Events: []Event{
        {
            Type: "transfer",
            Attributes: []Attribute{
                {Key: "from", Value: []byte("alice"), Index: true},
                {Key: "to", Value: []byte("bob"), Index: true},
                {Key: "amount", Value: []byte("100"), Index: false},
            },
        },
    },
}

```text

#### Metrics and Tracing Integration

The framework automatically instruments application calls:

```go
// Framework code
func (n *Node) executeTransaction(ctx context.Context, tx *Transaction) *TxExecResult {
    span := trace.StartSpan(ctx, "ExecuteTx")
    defer span.End()

    start := time.Now()
    result := n.app.ExecuteTx(ctx, tx)

    n.metrics.AppExecuteTx(time.Since(start), result.Code == CodeOK)

    return result
}

```text

#### Fail-Closed Defaults

Every method on `BaseApplication` returns a rejection:

```go
func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("CheckTx not implemented"),
    }
}

```text

**Rationale:**

- Secure by default
- Forces explicit implementation
- Prevents accidental acceptance of invalid data

---

### 5.3 Mempool (pkg/mempool/)

#### Overview

The **Mempool** stores pending transactions before block inclusion. Blockberry provides multiple implementations with different ordering strategies.

#### File: pkg/mempool/interface.go

**Core Interface:**

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

#### Simple Mempool (FIFO)

**File:** pkg/mempool/simple.go

**Structure:**

```go
type SimpleMempool struct {
    mu         sync.RWMutex
    txs        map[string]*txEntry  // Hash → entry
    queue      []string             // FIFO queue of hashes
    cache      *lru.Cache[string, struct{}]  // Recent tx cache
    maxTxs     int
    maxBytes   int64
    sizeBytes  int64
    validator  TxValidator
}

type txEntry struct {
    hash      []byte
    tx        []byte
    timestamp time.Time
}

```text

**Add Transaction:**

```go
func (m *SimpleMempool) AddTx(tx []byte) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Check capacity
    if len(m.txs) >= m.maxTxs {
        return types.ErrMempoolFull
    }

    // Hash transaction
    hash := sha256.Sum256(tx)
    hashStr := string(hash[:])

    // Check duplicate
    if _, exists := m.txs[hashStr]; exists {
        return types.ErrTxAlreadyExists
    }

    // Check recent cache
    if m.cache.Contains(hashStr) {
        return types.ErrTxAlreadyExists
    }

    // Validate
    if err := m.validator(tx); err != nil {
        return err
    }

    // Deep copy for safety
    txCopy := make([]byte, len(tx))
    copy(txCopy, tx)

    // Add to mempool
    m.txs[hashStr] = &txEntry{
        hash:      hash[:],
        tx:        txCopy,
        timestamp: time.Now(),
    }
    m.queue = append(m.queue, hashStr)
    m.sizeBytes += int64(len(tx))

    return nil
}

```text

**Reap Transactions:**

```go
func (m *SimpleMempool) ReapTxs(maxBytes int64) [][]byte {
    m.mu.RLock()
    defer m.mu.RUnlock()

    result := make([][]byte, 0, len(m.queue))
    var totalBytes int64

    for _, hashStr := range m.queue {
        entry := m.txs[hashStr]
        if totalBytes+int64(len(entry.tx)) > maxBytes {
            break
        }

        // Defensive copy
        txCopy := make([]byte, len(entry.tx))
        copy(txCopy, entry.tx)
        result = append(result, txCopy)

        totalBytes += int64(len(entry.tx))
    }

    return result
}

```text

#### Priority Mempool

**File:** pkg/mempool/priority.go

Uses a heap for priority-based ordering:

```go
type PriorityMempool struct {
    mu        sync.RWMutex
    txs       map[string]*priorityEntry
    heap      *txHeap
    maxTxs    int
    maxBytes  int64
    sizeBytes int64
    validator TxValidator
}

type priorityEntry struct {
    hash      []byte
    tx        []byte
    priority  int64  // Higher = more urgent
    timestamp time.Time
}

type txHeap []*priorityEntry

func (h txHeap) Less(i, j int) bool {
    // Higher priority comes first
    if h[i].priority != h[j].priority {
        return h[i].priority > h[j].priority
    }
    // Tie-breaker: older first
    return h[i].timestamp.Before(h[j].timestamp)
}

```text

#### TTL Mempool

**File:** pkg/mempool/ttl.go

Automatically expires transactions after TTL:

```go
type TTLMempool struct {
    SimpleMempool
    ttl             time.Duration
    cleanupInterval time.Duration
    stopCh          chan struct{}
    wg              sync.WaitGroup
}

func (m *TTLMempool) Start() error {
    m.wg.Add(1)
    go m.cleanupLoop()
    return nil
}

func (m *TTLMempool) cleanupLoop() {
    defer m.wg.Done()

    ticker := time.NewTicker(m.cleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-m.stopCh:
            return
        case <-ticker.C:
            m.removeExpired()
        }
    }
}

func (m *TTLMempool) removeExpired() {
    m.mu.Lock()
    defer m.mu.Unlock()

    now := time.Now()
    expiredHashes := make([]string, 0)

    for hashStr, entry := range m.txs {
        if now.Sub(entry.timestamp) > m.ttl {
            expiredHashes = append(expiredHashes, hashStr)
        }
    }

    for _, hashStr := range expiredHashes {
        m.removeUnsafe(hashStr)
    }
}

```text

#### Looseberry DAG Mempool

**File:** pkg/mempool/dag.go (adapter)

Integrates Looseberry for DAG-based ordering:

```go
type DAGMempool struct {
    looseberry *looseberry.Looseberry
    mu         sync.RWMutex
}

func (m *DAGMempool) ReapCertifiedBatches(maxBytes int64) []*CertifiedBatch {
    batches := m.looseberry.ReapCertifiedBatches(maxBytes)

    result := make([]*CertifiedBatch, len(batches))
    for i, b := range batches {
        result[i] = &CertifiedBatch{
            Round:       b.Certificate.Round,
            Validator:   b.Batch.Validator,
            Txs:         convertTxs(b.Batch.Txs),
            Certificate: convertCert(b.Certificate),
        }
    }
    return result
}

func (m *DAGMempool) UpdateValidatorSet(validators []Validator) {
    m.looseberry.SetValidatorSet(convertValidators(validators))
}

```text

#### Transaction Validation

**Validator Function:**

```go
type TxValidator func(tx []byte) error

// Default: Fail-closed
var DefaultTxValidator TxValidator = func(tx []byte) error {
    return fmt.Errorf("%w: no transaction validator configured", types.ErrNoTxValidator)
}

// Application-provided
func (app *MyApp) ValidateTx(tx []byte) error {
    // Parse transaction
    // Check signature
    // Check nonce
    // Check balance
    return nil
}

// Set on mempool
mempool.SetTxValidator(app.ValidateTx)

```text

#### Deduplication Strategy

Three-level deduplication:

1. **Mempool Map**: Check `txs[hashStr]`
2. **LRU Cache**: Check recently seen transactions
3. **Gossip Tracking**: Per-peer tracking (in handlers)

```go
// Level 1: Mempool
if _, exists := m.txs[hashStr]; exists {
    return ErrTxAlreadyExists
}

// Level 2: LRU cache
if m.cache.Contains(hashStr) {
    return ErrTxAlreadyExists
}

// Level 3: Per-peer tracking (in TransactionsHandler)
if peerState.HasSentTx(hash) {
    // Don't send again to this peer
    continue
}

```text

---

I'll continue with the remaining sections in the next response due to length constraints.

### 5.4 BlockStore (pkg/blockstore/)

#### Overview

The **BlockStore** persists blocks to disk and provides retrieval by height or hash. All implementations are thread-safe and support concurrent reads.

#### File: pkg/blockstore/store.go

**Core Interface:**

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

#### LevelDB Implementation

**File:** pkg/blockstore/leveldb.go

```go
type LevelDBBlockStore struct {
    db         *leveldb.DB
    mu         sync.RWMutex
    height     int64
    base       int64
}

// Key schema
const (
    keyPrefixHeight     = "h:"  // h:{height} → hash
    keyPrefixHash       = "H:"  // H:{hash} → height
    keyPrefixBlock      = "b:"  // b:{height} → block data
    keyMetaHeight       = "meta:height"
    keyMetaBase         = "meta:base"
)

func (s *LevelDBBlockStore) SaveBlock(height int64, hash []byte, data []byte) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if block already exists
    if s.hasBlockUnsafe(height) {
        return types.ErrBlockAlreadyExists
    }

    // Atomic batch write
    batch := new(leveldb.Batch)
    
    heightKey := makeHeightKey(height)
    hashKey := makeHashKey(hash)
    blockKey := makeBlockKey(height)

    batch.Put(heightKey, hash)
    batch.Put(hashKey, encodeInt64(height))
    batch.Put(blockKey, data)
    batch.Put([]byte(keyMetaHeight), encodeInt64(height))

    if s.base == 0 {
        batch.Put([]byte(keyMetaBase), encodeInt64(height))
    }

    // Sync write for durability
    if err := s.db.Write(batch, &opt.WriteOptions{Sync: true}); err != nil {
        return fmt.Errorf("writing block: %w", err)
    }

    // Update in-memory metadata
    if height > s.height {
        s.height = height
    }
    if s.base == 0 || height < s.base {
        s.base = height
    }

    return nil
}

```text

#### Certificate Storage Extension

**File:** pkg/blockstore/certificate.go

Extends BlockStore for DAG mempool (Looseberry) integration:

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

type CertificateBlockStore interface {
    BlockStore
    CertificateStore
}

```text

**Implementation in LevelDB:**

```go
// Key schema for certificates
const (
    keyPrefixCert          = "c:"   // c:{digest} → certificate
    keyPrefixCertRound     = "cr:"  // cr:{round}:{validator} → digest
    keyPrefixCertHeight    = "ch:"  // ch:{height}:{validator} → digest
    keyPrefixBatch         = "bat:" // bat:{digest} → batch
)

func (s *LevelDBBlockStore) SaveCertificate(cert *loosetypes.Certificate) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    digest := cert.Digest()
    
    // Serialize certificate
    data, err := cert.MarshalCramberry()
    if err != nil {
        return err
    }

    // Defensive copy of digest
    digestCopy := make([]byte, len(digest))
    copy(digestCopy, digest[:])

    batch := new(leveldb.Batch)
    
    // c:{digest} → certificate
    certKey := makeCertKey(digestCopy)
    batch.Put(certKey, data)
    
    // cr:{round}:{validator} → digest
    roundKey := makeCertRoundKey(cert.Round(), cert.Validator())
    batch.Put(roundKey, digestCopy)

    if err := s.db.Write(batch, nil); err != nil {
        return fmt.Errorf("saving certificate: %w", err)
    }

    return nil
}

```text

#### Pruning Wrapper (Decorator Pattern)

**File:** pkg/blockstore/pruning.go

Adds automatic pruning to any BlockStore:

```go
type PruningWrapper struct {
    store    BlockStore
    pruner   *BackgroundPruner
    mu       sync.Mutex
}

type PruneConfig struct {
    Strategy   PruneStrategy
    KeepRecent int64          // Always keep last N blocks
    KeepEvery  int64          // Keep checkpoint every N blocks
    Interval   time.Duration  // Pruning interval
}

type PruneStrategy string

const (
    PruneNothing    PruneStrategy = "nothing"
    PruneEverything PruneStrategy = "everything"
    PruneDefault    PruneStrategy = "default"
)

func (w *PruningWrapper) SaveBlock(height int64, hash []byte, data []byte) error {
    if err := w.store.SaveBlock(height, hash, data); err != nil {
        return err
    }

    // Trigger pruning asynchronously
    w.pruner.NotifyNewBlock(height)

    return nil
}

type BackgroundPruner struct {
    store     BlockStore
    config    PruneConfig
    notifyCh  chan int64
    stopCh    chan struct{}
    wg        sync.WaitGroup
}

func (p *BackgroundPruner) pruneLoop() {
    defer p.wg.Done()

    ticker := time.NewTicker(p.config.Interval)
    defer ticker.Stop()

    for {
        select {
        case <-p.stopCh:
            return
        case height := <-p.notifyCh:
            p.prune(height)
        case <-ticker.C:
            p.prune(p.store.Height())
        }
    }
}

func (p *BackgroundPruner) prune(currentHeight int64) {
    if p.config.Strategy == PruneNothing {
        return
    }

    retainHeight := currentHeight - p.config.KeepRecent
    if retainHeight <= 0 {
        return
    }

    base := p.store.Base()
    
    for h := base; h < retainHeight; h++ {
        // Keep checkpoints
        if p.config.KeepEvery > 0 && h%p.config.KeepEvery == 0 {
            continue
        }

        // Delete block (implementation-specific)
        // This requires extending BlockStore with Delete method
    }
}

```text

---

### 5.5 StateStore (pkg/statestore/)

#### Overview

The **StateStore** manages merkleized application state using AVL+ trees from avlberry. Each version represents a committed block height.

#### File: pkg/statestore/store.go

**Core Interface:**

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

#### AVL+ Implementation (avlberry)

**File:** pkg/statestore/iavl.go

```go
type IAVLStateStore struct {
    tree      *iavl.MutableTree
    db        dbm.DB
    cacheSize int
    mu        sync.Mutex  // avlberry not thread-safe
}

func NewIAVLStore(path string, cacheSize int) (*IAVLStateStore, error) {
    db, err := dbm.NewGoLevelDB("state", path)
    if err != nil {
        return nil, err
    }

    tree, err := iavl.NewMutableTree(db, cacheSize, false)
    if err != nil {
        return nil, err
    }

    // Load latest version
    if _, err := tree.Load(); err != nil {
        return nil, err
    }

    return &IAVLStateStore{
        tree:      tree,
        db:        db,
        cacheSize: cacheSize,
    }, nil
}

func (s *IAVLStateStore) Set(key []byte, value []byte) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    _, err := s.tree.Set(key, value)
    return err
}

func (s *IAVLStateStore) Commit() ([]byte, int64, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    hash, version, err := s.tree.SaveVersion()
    if err != nil {
        return nil, 0, fmt.Errorf("saving AVL+ version: %w", err)
    }

    return hash, version, nil
}

```text

#### Merkle Proof Generation

```go
func (s *IAVLStateStore) GetProof(key []byte) (*Proof, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    value, proof, err := s.tree.GetWithProof(key)
    if err != nil {
        return nil, err
    }

    // Convert to ICS23 format (Cosmos standard)
    ics23Proof := convertToICS23(proof)

    return &Proof{
        Key:        key,
        Value:      value,
        Exists:     value != nil,
        RootHash:   s.tree.Hash(),
        Version:    s.tree.Version(),
        ProofBytes: ics23Proof,
    }, nil
}

func (p *Proof) Verify(rootHash []byte) (bool, error) {
    // Verify using ICS23 proof verification
    return ics23.VerifyMembership(
        p.ProofBytes,
        rootHash,
        p.Key,
        p.Value,
    )
}

```text

#### Versioned State Access

```go
func (s *IAVLStateStore) LoadVersion(version int64) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if _, err := s.tree.LoadVersion(version); err != nil {
        return fmt.Errorf("loading version %d: %w", version, err)
    }

    return nil
}

// Query historical state
func (app *MyApp) Query(ctx context.Context, req *QueryRequest) *QueryResponse {
    if req.Height > 0 && req.Height < app.stateStore.Version() {
        // Load historical version
        if err := app.stateStore.LoadVersion(req.Height); err != nil {
            return &QueryResponse{Code: CodeExecutionFailed, Error: err}
        }
        defer app.stateStore.LoadVersion(app.stateStore.Version()) // Restore latest
    }

    value, err := app.stateStore.Get(req.Data)
    if err != nil {
        return &QueryResponse{Code: CodeExecutionFailed, Error: err}
    }

    return &QueryResponse{
        Code:   CodeOK,
        Value:  value,
        Height: app.stateStore.Version(),
    }
}

```text

---

### 5.6 Consensus (pkg/consensus/)

#### Overview

The **Consensus** layer is pluggable. Applications register custom consensus engines via a factory pattern.

#### File: pkg/consensus/interface.go

**Core Interface:**

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

#### Consensus Dependencies

```go
type ConsensusDependencies struct {
    Network     Network
    BlockStore  blockstore.BlockStore
    StateStore  *statestore.StateStore
    Mempool     mempool.Mempool
    Application abi.Application
    Callbacks   *ConsensusCallbacks
    Config      *ConsensusConfig
}

```text

#### Factory Pattern

**File:** pkg/consensus/factory.go

```go
type Factory struct {
    registry map[ConsensusType]ConsensusConstructor
    mu       sync.RWMutex
}

type ConsensusConstructor func(cfg *ConsensusConfig) (ConsensusEngine, error)

var DefaultFactory = NewFactory()

func (f *Factory) Register(consensusType ConsensusType, constructor ConsensusConstructor) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.registry[consensusType] = constructor
}

func (f *Factory) Create(cfg *ConsensusConfig) (ConsensusEngine, error) {
    f.mu.RLock()
    constructor, exists := f.registry[cfg.Type]
    f.mu.RUnlock()

    if !exists {
        return nil, fmt.Errorf("unknown consensus type: %s", cfg.Type)
    }

    return constructor(cfg)
}

// Global registration
func RegisterConsensus(consensusType ConsensusType, constructor ConsensusConstructor) {
    DefaultFactory.Register(consensusType, constructor)
}

```text

#### Null Consensus Implementation

**File:** pkg/consensus/null.go

For full nodes that don't participate in consensus:

```go
type NullConsensus struct {
    deps    ConsensusDependencies
    height  int64
    running atomic.Bool
}

func (c *NullConsensus) Name() string {
    return "null-consensus"
}

func (c *NullConsensus) Initialize(deps ConsensusDependencies) error {
    c.deps = deps
    c.height = deps.BlockStore.Height()
    return nil
}

func (c *NullConsensus) ProcessBlock(block *Block) error {
    // Validate block
    if err := c.validateBlock(block); err != nil {
        return err
    }

    // Execute through application
    ctx := context.Background()

    if err := c.deps.Application.BeginBlock(ctx, &block.Header); err != nil {
        return err
    }

    for _, tx := range block.Transactions {
        result := c.deps.Application.ExecuteTx(ctx, &abi.Transaction{Data: tx})
        if result.Code != abi.CodeOK {
            return result.Error
        }
    }

    c.deps.Application.EndBlock(ctx)
    commitResult := c.deps.Application.Commit(ctx)

    // Save block
    if err := c.deps.BlockStore.SaveBlock(block.Height, block.Hash, block.Data); err != nil {
        return err
    }

    c.height = block.Height

    // Notify
    c.deps.Callbacks.InvokeOnBlockCommitted(block.Height, block.Hash)

    return nil
}

```text

---

## 6. Concurrency Architecture

### 6.1 Goroutine Usage Patterns

Blockberry employs several well-defined goroutine patterns for safe concurrent execution.

#### Event Loop Pattern

**File:** pkg/node/node.go

```go
func (n *Node) eventLoop() {
    defer n.wg.Done()

    for {
        select {
        case <-n.stopCh:
            return

        case event := <-n.glueNode.Events():
            n.handleConnectionEvent(event)

        case msg := <-n.glueNode.Messages():
            n.handleMessage(msg)
        }
    }
}

```text

**Characteristics:**

- Single goroutine for message/event processing
- Non-blocking select
- Graceful shutdown via stop channel
- WaitGroup tracking for cleanup

#### Background Worker Pattern

**File:** internal/handlers/handshake.go

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

**Use Cases:**

- Periodic cleanup tasks
- Heartbeat/health checks
- Background pruning
- Metric collection

#### One-Shot Async Pattern

**File:** internal/handlers/handshake.go

```go
go func(pid peer.ID) {
    _ = h.network.Disconnect(pid)
}(peerID)

```text

**Use Cases:**

- Fire-and-forget operations
- Non-blocking disconnections
- Async notifications

### 6.2 Synchronization Primitives

#### Mutex Usage Patterns

**Read-Heavy Pattern (RWMutex):**

```go
type Network struct {
    tempBans   map[peer.ID]*TempBanEntry
    tempBansMu sync.RWMutex  // Read-heavy access
}

func (n *Network) IsTempBanned(peerID peer.ID) bool {
    n.tempBansMu.RLock()
    defer n.tempBansMu.RUnlock()

    entry, exists := n.tempBans[peerID]
    if !exists {
        return false
    }

    return time.Now().Before(entry.UntilTime)
}

```text

**Write-Heavy Pattern (Mutex):**

```go
type Container struct {
    components map[string]*componentEntry
    mu         sync.RWMutex
}

func (c *Container) Register(name string, component Component) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if _, exists := c.components[name]; exists {
        return fmt.Errorf("component %s already registered", name)
    }

    c.components[name] = &componentEntry{
        component: component,
    }

    return nil
}

```text

**Lock Ordering:**

To prevent deadlocks, always acquire locks in consistent order:

1. Network-level locks first
2. Peer-specific locks second
3. Resource-specific locks third

```go
// Good: Consistent ordering
func (n *Network) UpdatePeerScore(peerID peer.ID, delta int) {
    n.peersMu.Lock()         // 1. Network lock
    defer n.peersMu.Unlock()

    peer := n.peers[peerID]
    peer.mu.Lock()           // 2. Peer lock
    defer peer.mu.Unlock()

    peer.score += delta
}

// Bad: Inconsistent ordering (potential deadlock)
func (n *Network) BadMethod(peerID peer.ID) {
    peer.mu.Lock()           // Wrong order!
    n.peersMu.Lock()
    // ...
}

```text

#### Atomic Operations

**File:** pkg/node/node.go

```go
type Node struct {
    stopping atomic.Bool  // Lock-free shutdown flag
}

func (n *Node) Stop() error {
    if !n.stopping.CompareAndSwap(false, true) {
        return nil  // Already stopping
    }

    close(n.stopCh)
    // ...
}

func (n *Node) eventLoop() {
    for {
        if n.stopping.Load() {
            return
        }
        // Process messages
    }
}

```text

**Use Cases:**

- Boolean flags (running, stopped, cancelled)
- Counters (atomic.Int64)
- Lock-free status checks

#### Channel Patterns

**Buffered Channels (Non-Blocking):**

```go
ch := make(chan abi.Event, b.config.BufferSize)

// Non-blocking send with timeout
timer := time.NewTimer(timeout)
select {
case sub.ch <- event:
    if !timer.Stop() {
        <-timer.C  // Drain timer to prevent leak
    }
case <-timer.C:
    // Timeout - slow subscriber
}

```text

**Stop Channel Pattern:**

```go
type Reactor struct {
    stopCh chan struct{}
}

func (r *Reactor) Stop() error {
    close(r.stopCh)  // Broadcast to all listeners
    r.wg.Wait()
    return nil
}

func (r *Reactor) worker() {
    for {
        select {
        case <-r.stopCh:
            return
        case msg := <-r.msgCh:
            r.process(msg)
        }
    }
}

```text

### 6.3 Graceful Shutdown Pattern

**Multi-Phase Shutdown:**

```go
func (n *Node) Stop() error {
    // Phase 1: Signal shutdown
    if !n.stopping.CompareAndSwap(false, true) {
        return nil
    }

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
        // Timeout - log warning but continue
        n.logger.Warn("Shutdown timeout exceeded, forcing shutdown")
    }

    // Phase 4: Stop components in reverse order
    _ = n.syncReactor.Stop()
    _ = n.blockReactor.Stop()
    _ = n.txReactor.Stop()
    _ = n.pexReactor.Stop()
    _ = n.handshakeHandler.Stop()

    // Phase 5: Close resources
    _ = n.blockStore.Close()
    _ = n.stateStore.Close()

    return nil
}

```text

### 6.4 Race Condition Prevention

#### Deep Copy Pattern

**File:** internal/handlers/handshake.go

```go
func (h *HandshakeHandler) GetPeerInfo(peerID peer.ID) *PeerHandshakeState {
    h.mu.RLock()
    defer h.mu.RUnlock()

    state, exists := h.peers[peerID]
    if !exists {
        return nil
    }

    // Return deep copy to prevent mutation
    return &PeerHandshakeState{
        PeerPubKey:  append([]byte(nil), state.PeerPubKey...),
        ChainID:     state.ChainID,
        Version:     state.Version,
        LatestHeight: state.LatestHeight,
        SentRequest: state.SentRequest,
        // ... copy all fields
    }
}

```text

#### Lock-Then-Check Pattern

```go
h.mu.Lock()
if state != nil && state.SentRequest {
    h.mu.Unlock()
    return nil  // Already sent
}
state.SentRequest = true
h.mu.Unlock()

```text

#### Snapshot-Then-Release Pattern

```go
n.mu.RLock()
cb := n.callbacks  // Snapshot under lock
n.mu.RUnlock()

// Use cb outside lock to avoid holding lock during callback
if cb != nil {
    cb.InvokePeerConnected(peerID, isOutbound)
}

```text

### 6.5 Thread Safety Contracts

#### Application Methods

From `pkg/abi/application.go`:

**Thread-Safe (Concurrent):**

- `CheckTx(ctx, tx)` - Called from multiple goroutines
- `Query(ctx, req)` - Called from RPC handlers

**Single-Threaded (Sequential):**

- `BeginBlock(ctx, header)`
- `ExecuteTx(ctx, tx)`
- `EndBlock(ctx)`
- `Commit(ctx)`

**Contract:**

```go
// Applications must ensure CheckTx and Query are thread-safe
type Application interface {
    // Thread-safe: may be called concurrently
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult
    Query(ctx context.Context, req *QueryRequest) *QueryResponse

    // Sequential: called from single goroutine
    BeginBlock(ctx context.Context, header *BlockHeader) error
    ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult
    EndBlock(ctx context.Context) *EndBlockResult
    Commit(ctx context.Context) *CommitResult
}

```text

#### Mempool Methods

All Mempool methods must be thread-safe:

```go
type Mempool interface {
    AddTx(tx []byte) error           // Thread-safe
    RemoveTxs(hashes [][]byte)       // Thread-safe
    ReapTxs(maxBytes int64) [][]byte // Thread-safe
    // ... all methods thread-safe
}

```text

---

## 7. Network Architecture

### 7.1 P2P Protocol

Blockberry uses **Glueberry** for P2P networking, providing:

- Encrypted streams after handshake
- Multiplexed connections
- libp2p-based peer discovery
- NAT traversal
- Connection management

#### Stream-Based Communication

Each logical protocol runs on a separate stream:

| Stream | Encrypted | Direction | Purpose |
|--------|-----------|-----------|---------|
| `handshake` | No | Bidirectional | Initial connection setup |
| `pex` | Yes | Bidirectional | Peer address exchange |
| `transactions` | Yes | Bidirectional | Transaction gossip |
| `blocks` | Yes | Bidirectional | Block propagation |
| `blocksync` | Yes | Request/Response | Historical block sync |
| `consensus` | Yes | Bidirectional | Consensus messages (raw bytes) |
| `housekeeping` | Yes | Bidirectional | Latency probes, diagnostics |
| `statesync` | Yes | Request/Response | State snapshot sync |

#### Two-Phase Handshake Protocol

**Phase 1: Initial Connection (Unencrypted)**

```text
Peer A                                 Peer B
   │                                      │
   │ ── TCP Connection Established ────▶ │
   │                                      │
   │ ── HelloRequest ───────────────────▶│
   │    {node_id, version, chain_id}     │
   │                                      │
   │◀── HelloRequest ───────────────────│
   │    {node_id, version, chain_id}     │

```text

**HelloRequest Validation:**

```go
// Chain ID must match
if *req.ChainId != h.chainID {
    _ = h.network.TempBanPeer(peerID, TempBanDurationChainMismatch, reason)
    return ErrChainIDMismatch
}

// Protocol version must match
if *req.Version != h.protocolVersion {
    _ = h.network.TempBanPeer(peerID, TempBanDurationVersionMismatch, reason)
    return ErrVersionMismatch
}

```text

**Phase 2: Key Exchange**

```text
Peer A                                 Peer B
   │                                      │
   │ ── HelloResponse ──────────────────▶│
   │    {accepted=true, public_key}      │
   │                                      │
   │◀── HelloResponse ──────────────────│
   │    {accepted=true, public_key}      │
   │                                      │
   │ [Prepare Encrypted Streams]         │
   │                                      │
   │ ── HelloFinalize ──────────────────▶│
   │    {success=true}                   │
   │                                      │
   │◀── HelloFinalize ──────────────────│
   │    {success=true}                   │
   │                                      │
   │ [StateEstablished - Encrypted]      │

```text

**Stream Preparation:**

```go
streamNames := []string{
    "pex",
    "transactions",
    "blocksync",
    "blocks",
    "consensus",
    "housekeeping",
    "statesync",
}

err := node.PrepareStreams(peerID, peerPubKey, streamNames)

```text

**After Finalization:**

```go
// Transition to StateEstablished
node.FinalizeHandshake(peerID)

// All subsequent messages are encrypted
node.Send(peerID, "transactions", txData)  // Encrypted

```text

### 7.2 Node Roles

Blockberry supports multiple node roles with different capabilities:

#### Validator Node

**Configuration:**

```toml
role = "validator"

```text

**Characteristics:**

- Validates all blocks
- Participates in consensus
- Stores full history
- Serves RPC queries
- Requires staking/authorization

**Components Enabled:**

- All stream handlers
- Consensus engine (BFT, custom)
- Full mempool
- Complete block and state storage

#### Full Node

**Configuration:**

```toml
role = "full"

```text

**Characteristics:**

- Validates all blocks
- Does NOT participate in consensus
- Stores full history
- Serves RPC queries

**Components Enabled:**

- All stream handlers except consensus
- NullConsensus engine
- Full mempool
- Complete block and state storage

#### Seed Node

**Configuration:**

```toml
role = "seed"

```text

**Characteristics:**

- Does NOT validate blocks
- Does NOT participate in consensus
- Does NOT store blocks/state
- Only provides peer discovery (PEX)

**Components Enabled:**

- Handshake handler
- PEX reactor only
- No mempool
- No block/state storage

#### Light Client

**Configuration:**

```toml
role = "light"

```text

**Characteristics:**

- Validates block headers only
- Does NOT participate in consensus
- Stores headers only (pruned)
- Limited query support

**Components Enabled:**

- Handshake, PEX
- Block header sync
- Header-only storage
- Merkle proof verification

#### Archive Node

**Configuration:**

```toml
role = "archive"

```text

**Characteristics:**

- Validates all blocks
- Does NOT participate in consensus
- Stores FULL history (no pruning)
- Serves historical queries

**Components Enabled:**

- All stream handlers except consensus
- NullConsensus
- Full mempool
- No pruning on storage

### 7.3 Peer Management

#### Peer State Tracking

**File:** internal/p2p/peer_manager.go

```go
type PeerManager struct {
    peers   map[peer.ID]*PeerState
    mu      sync.RWMutex
    scorer  *PeerScorer
}

type PeerState struct {
    ID              peer.ID
    PublicKey       []byte
    IsOutbound      bool
    IsValidator     bool
    Score           int
    Latency         time.Duration
    BytesSent       uint64
    BytesRecv       uint64
    ConnectedAt     time.Time

    // Transaction tracking
    knownTxs        *lru.Cache[string, struct{}]
    sentTxs         *lru.Cache[string, struct{}]

    // Block tracking
    knownBlocks     *lru.Cache[int64, struct{}]
    sentBlocks      *lru.Cache[int64, struct{}]

    mu              sync.RWMutex
}

```text

#### Eclipse Attack Protection

**Strategies:**

1. **Peer Diversity**: Limit connections per IP subnet
2. **Outbound Preference**: Maintain minimum outbound connections
3. **Connection Limits**: Max inbound and outbound peers
4. **Address Book**: Persistent peer discovery
5. **Seed Nodes**: Hardcoded trusted peers

```go
func (n *Network) ShouldAcceptInbound(addr multiaddr.Multiaddr) bool {
    // Check total peer count
    if n.PeerCount() >= n.cfg.MaxInboundPeers {
        return false
    }

    // Extract IP
    ip := extractIP(addr)

    // Limit connections per /24 subnet
    subnet := ip.Mask(net.CIDRMask(24, 32))
    count := n.countPeersInSubnet(subnet)
    if count >= MaxPeersPerSubnet {
        return false
    }

    return true
}

```text

#### Rate Limiting

**Per-Peer Rate Limits:**

```go
type PeerRateLimiter struct {
    limits map[string]*rate.Limiter
    mu     sync.RWMutex
}

func (r *PeerRateLimiter) Allow(peerID peer.ID, stream string) bool {
    r.mu.RLock()
    limiter, exists := r.limits[stream]
    r.mu.RUnlock()

    if !exists {
        return true
    }

    return limiter.Allow()
}

```text

**Stream-Specific Limits:**

| Stream | Rate Limit | Burst |
|--------|------------|-------|
| `transactions` | 100 msg/s | 200 |
| `blocks` | 10 msg/s | 20 |
| `blocksync` | 50 msg/s | 100 |
| `pex` | 1 msg/s | 5 |

#### Peer Scoring and Reputation

**File:** internal/p2p/scorer.go

```go
type PeerScorer struct {
    scores        map[peer.ID]*ScoreEntry
    mu            sync.RWMutex
    decayInterval time.Duration
    decayRate     float64
}

type ScoreEntry struct {
    Score         int
    PenaltyPoints int64
    LastDecay     time.Time
}

// Penalty points for violations
const (
    PenaltyInvalidMessage      int64 = 10
    PenaltyProtocolViolation   int64 = 50
    PenaltyDuplicateMessage    int64 = 5
    PenaltySlowResponse        int64 = 2
    PenaltyChainIDMismatch     int64 = 1000  // Permanent ban
    PenaltyVersionMismatch     int64 = 1000  // Permanent ban
)

// Penalty thresholds
const (
    BanThresholdTemporary int64 = 100   // 1 hour ban
    BanThresholdPermanent int64 = 1000  // Permanent ban
)

func (s *PeerScorer) AddPenalty(peerID peer.ID, points int64, reason string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    entry := s.getOrCreateEntry(peerID)
    entry.PenaltyPoints += points

    if entry.PenaltyPoints >= BanThresholdPermanent {
        s.banPermanent(peerID, reason)
    } else if entry.PenaltyPoints >= BanThresholdTemporary {
        s.banTemporary(peerID, 1*time.Hour, reason)
    }
}

// Decay penalties over time
func (s *PeerScorer) decayPenalties() {
    s.mu.Lock()
    defer s.mu.Unlock()

    now := time.Now()

    for peerID, entry := range s.scores {
        if now.Sub(entry.LastDecay) > s.decayInterval {
            entry.PenaltyPoints = int64(float64(entry.PenaltyPoints) * s.decayRate)
            entry.LastDecay = now
        }
    }
}

```text

#### Temporary vs Permanent Bans

**Temporary Bans:**

```go
type TempBanEntry struct {
    PeerID    peer.ID
    UntilTime time.Time
    Reason    string
}

func (n *Network) TempBanPeer(peerID peer.ID, duration time.Duration, reason string) error {
    n.tempBansMu.Lock()
    defer n.tempBansMu.Unlock()

    n.tempBans[peerID] = &TempBanEntry{
        PeerID:    peerID,
        UntilTime: time.Now().Add(duration),
        Reason:    reason,
    }

    return n.Disconnect(peerID)
}

```text

**Permanent Blacklist:**

```go
func (n *Network) BlacklistPeer(peerID peer.ID, reason string) error {
    n.blacklistMu.Lock()
    defer n.blacklistMu.Unlock()

    n.blacklist[peerID] = reason

    return n.Disconnect(peerID)
}

```text

**Ban Durations:**

| Violation | Ban Duration |
|-----------|--------------|
| Chain ID mismatch | Permanent |
| Version mismatch | Permanent |
| Invalid message format | 1 hour |
| Protocol violation | 24 hours |
| Repeated bad behavior | Progressive (1h → 24h → permanent) |

---


## 8. Storage Architecture

### 8.1 Block Storage

#### Pluggable Backends

Blockberry supports multiple storage backends via the `BlockStore` interface:

**LevelDB (Default):**

- Proven stability
- Good read/write performance
- Efficient compression
- Production-ready

**BadgerDB:**

- Higher throughput
- Better compression ratios
- More memory usage
- Concurrent-friendly

**In-Memory:**

- Testing only
- No persistence
- Fast access

#### Key Schema Design

**LevelDB Key Layout:**

```text
Prefixes:
  h:{height}       → block_hash        (height → hash mapping)
  H:{hash}         → height            (hash → height mapping)
  b:{height}       → block_data        (actual block data)
  c:{digest}       → certificate       (DAG certificate)
  cr:{round}:{val} → cert_digest       (round → certificate)
  ch:{height}:{val}→ cert_digest       (height → certificate)
  bat:{digest}     → batch             (transaction batch)
  meta:height      → latest_height
  meta:base        → earliest_height

Example:
  h:00000001       → 0x1a2b3c4d...
  H:0x1a2b3c4d...  → 1
  b:00000001       → <block bytes>

```text

**Benefits:**

- Prefix-based iteration
- Efficient range queries
- Separate hash and data lookup
- Metadata tracking

#### Batch Writes for Atomicity

```go
func (s *LevelDBBlockStore) SaveBlock(height int64, hash []byte, data []byte) error {
    batch := new(leveldb.Batch)

    // All writes in single atomic batch
    batch.Put(makeHeightKey(height), hash)
    batch.Put(makeHashKey(hash), encodeInt64(height))
    batch.Put(makeBlockKey(height), data)
    batch.Put([]byte(keyMetaHeight), encodeInt64(height))

    // Sync write for durability
    opts := &opt.WriteOptions{Sync: true}
    return s.db.Write(batch, opts)
}

```text

#### Pruning Strategies

**PruneNothing:**

```toml
[pruning]
strategy = "nothing"

```text

- Keep all blocks forever
- Archive node configuration
- Highest disk usage

**PruneDefault:**

```toml
[pruning]
strategy = "default"
keep_recent = 100000
keep_every = 10000

```text

- Keep last 100k blocks
- Keep checkpoint every 10k blocks
- Balanced storage/query capability

**PruneEverything:**

```toml
[pruning]
strategy = "everything"
keep_recent = 1000

```text

- Keep only last 1000 blocks
- Minimal disk usage
- Light node configuration

### 8.2 State Storage

#### AVL+ Merkle Tree

Blockberry uses **avlberry** (AVL+ tree) for state storage:

**Properties:**

- Versioned (one version per block height)
- Merkleized (cryptographic proof of state)
- Self-balancing (AVL tree)
- Copy-on-write (immutability)

**Structure:**

```text
                Root Hash (v3)
                /           \
           Node A          Node B
          /     \         /     \
       Leaf1  Leaf2   Leaf3   Leaf4
       (k1,v1)(k2,v2)(k3,v3)(k4,v4)

Versions:
  v1: Initial state
  v2: After block 2
  v3: After block 3 (current)

Each version shares unchanged nodes (copy-on-write)

```text

#### Versioned Queries

```go
// Query current state
value, _ := store.Get([]byte("account:alice"))

// Query historical state
store.LoadVersion(100)
value, _ := store.Get([]byte("account:alice"))  // State at height 100
store.LoadVersion(store.Version())  // Restore latest

```text

#### ICS23 Proof Generation

Compatible with Cosmos IBC:

```go
func (s *IAVLStateStore) GetProof(key []byte) (*Proof, error) {
    value, avlProof, err := s.tree.GetWithProof(key)
    if err != nil {
        return nil, err
    }

    // Convert to ICS23 format
    ics23Proof := convertToICS23(avlProof)

    return &Proof{
        Key:        key,
        Value:      value,
        Exists:     value != nil,
        RootHash:   s.tree.Hash(),
        Version:    s.tree.Version(),
        ProofBytes: ics23Proof,
    }, nil
}

```text

**Proof Verification:**

```go
valid, err := ics23.VerifyMembership(
    proof.ProofBytes,
    proof.RootHash,
    proof.Key,
    proof.Value,
)

```text

#### Snapshot Support

For state sync:

```go
func (s *IAVLStateStore) ExportSnapshot(height int64) (*Snapshot, error) {
    if err := s.LoadVersion(height); err != nil {
        return nil, err
    }

    exporter := s.tree.Export()
    defer exporter.Close()

    chunks := make([][]byte, 0)
    for {
        chunk := exporter.Next()
        if chunk == nil {
            break
        }
        chunks = append(chunks, chunk)
    }

    return &Snapshot{
        Height: height,
        Hash:   s.tree.Hash(),
        Chunks: chunks,
    }, nil
}

```text

#### Pruning Strategies

**State Pruning:**

```toml
[statestore]
pruning = "default"
keep_recent = 100

```text

**Strategies:**

- **nothing**: Keep all versions
- **default**: Keep configurable recent versions
- **everything**: Keep only latest version

---

## 9. API Design

### 9.1 Application Interface (ABI v2.0)

The ABI is the primary contract between framework and application. See [Section 5.2](#52-application-binary-interface-pkgabi) for full details.

**Key Design Decisions:**

1. **Callback Inversion**: Framework drives execution
2. **Fail-Closed Defaults**: BaseApplication rejects all
3. **Context-Aware**: All methods receive `context.Context`
4. **Structured Results**: Typed result structures, not raw bytes
5. **Event Emission**: Events part of results

**Lifecycle Flow:**

```text
InitChain (once) → [CheckTx (concurrent)] → BeginBlock → ExecuteTx (loop) → EndBlock → Commit → [Query (concurrent)]

```text

### 9.2 RPC APIs

#### gRPC Service

**File:** pkg/rpc/grpc/server.go

```go
service BlockberryNode {
    // Transactions
    rpc BroadcastTx(BroadcastTxRequest) returns (BroadcastTxResponse);
    rpc GetTx(GetTxRequest) returns (GetTxResponse);

    // Blocks
    rpc GetBlock(GetBlockRequest) returns (GetBlockResponse);
    rpc GetBlockHeader(GetBlockHeaderRequest) returns (GetBlockHeaderResponse);

    // State
    rpc Query(QueryRequest) returns (QueryResponse);
    rpc GetValidators(GetValidatorsRequest) returns (GetValidatorsResponse);

    // Network
    rpc GetNetworkInfo(GetNetworkInfoRequest) returns (GetNetworkInfoResponse);
    rpc GetPeers(GetPeersRequest) returns (GetPeersResponse);

    // Subscriptions
    rpc Subscribe(SubscribeRequest) returns (stream Event);
}

```text

**Authentication:**

```go
func (s *GRPCServer) authInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing metadata")
    }

    tokens := md.Get("authorization")
    if len(tokens) == 0 {
        return nil, status.Error(codes.Unauthenticated, "missing authorization")
    }

    if !s.validateToken(tokens[0]) {
        return nil, status.Error(codes.PermissionDenied, "invalid token")
    }

    return handler(ctx, req)
}

```text

#### JSON-RPC 2.0 Server

**File:** pkg/rpc/jsonrpc/server.go

**Methods:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "broadcast_tx",
  "params": {
    "tx": "0x1a2b3c..."
  }
}

```text

**Supported Methods:**

- `broadcast_tx`: Submit transaction
- `get_tx`: Query transaction by hash
- `get_block`: Query block by height
- `get_block_header`: Query block header
- `query`: Application state query
- `get_validators`: Current validator set
- `get_network_info`: Network status
- `subscribe`: Subscribe to events

**Rate Limiting:**

```go
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
}

func (r *RateLimiter) Allow(clientID string, method string) bool {
    key := clientID + ":" + method

    r.mu.RLock()
    limiter, exists := r.limiters[key]
    r.mu.RUnlock()

    if !exists {
        r.mu.Lock()
        limiter = rate.NewLimiter(rate.Limit(10), 20)  // 10 req/s, burst 20
        r.limiters[key] = limiter
        r.mu.Unlock()
    }

    return limiter.Allow()
}

```text

#### WebSocket Server

**File:** pkg/rpc/websocket/server.go

**Event Subscription:**

```javascript
// Client connects
ws = new WebSocket("ws://localhost:26657/websocket")

// Subscribe to new blocks
ws.send(JSON.stringify({
  jsonrpc: "2.0",
  method: "subscribe",
  params: {
    query: "tm.event='NewBlock'"
  },
  id: 1
}))

// Receive events
ws.onmessage = (event) => {
  const data = JSON.parse(event.data)
  if (data.result && data.result.data) {
    console.log("New block:", data.result.data)
  }
}

```text

**Server Implementation:**

```go
func (s *WebSocketServer) handleSubscribe(conn *websocket.Conn, req *SubscribeRequest) {
    query, err := parseQuery(req.Query)
    if err != nil {
        s.sendError(conn, err)
        return
    }

    events, err := s.eventBus.Subscribe(context.Background(), conn.ID(), query)
    if err != nil {
        s.sendError(conn, err)
        return
    }

    // Stream events to client
    go func() {
        for event := range events {
            data, _ := json.Marshal(event)
            conn.WriteMessage(websocket.TextMessage, data)
        }
    }()
}

```text

### 9.3 Message Protocol (Cramberry)

#### Schema Definition

**File:** blockberry.cram

```cramberry
message HelloRequest {
  required string node_id = 1;
  required int32 version = 2;
  optional string inbound_url = 3;
  optional int32 inbound_port = 4;
  required string chain_id = 5;
  required int64 timestamp = 6;
  required int64 latest_height = 7;
}

message HelloResponse {
  required bool accepted = 1;
  required bytes public_key = 2;
}

interface HandshakeMessage {
  128 = HelloRequest;
  129 = HelloResponse;
  130 = HelloFinalize;
}

```text

#### Type ID Registry

| Type ID Range | Message Category |
|---------------|------------------|
| 128-130 | Handshake |
| 131-132 | PEX |
| 133-136 | Transactions |
| 137-138 | Block Sync |
| 139 | Blocks |
| 140-143 | Housekeeping |
| 144-147 | State Sync |
| 200-299 | RPC (reserved) |

#### Polymorphic Dispatch

```go
// Decode with type ID
var msg schema.HandshakeMessage
if err := cramberry.UnmarshalInterface(data, &msg); err != nil {
    return err
}

// Type switch
switch m := msg.(type) {
case *schema.HelloRequest:
    handleHelloRequest(m)
case *schema.HelloResponse:
    handleHelloResponse(m)
case *schema.HelloFinalize:
    handleHelloFinalize(m)
}

```text

#### V2 Encoding Optimizations

Cramberry v2 provides:

- **Zero-copy reading**: Direct slice references
- **Buffer pooling**: Reusable writer buffers
- **Efficient varint encoding**: Compact integers
- **Optional fields**: Skip encoding if nil

```go
// Use pooled writer
w := cramberry.GetWriter()
defer cramberry.PutWriter(w)

w.WriteTypeID(typeID)
w.WriteBytes(data)
result := w.BytesCopy()

```text

---

## 10. Configuration

### 10.1 Configuration Structure

**File:** pkg/config/config.go

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

### 10.2 Node Configuration

```go
type NodeConfig struct {
    ChainID         string  // Unique blockchain identifier
    ProtocolVersion int32   // Protocol version (must match peers)
    PrivateKeyPath  string  // Path to Ed25519 private key
}

```text

### 10.3 Network Configuration

```go
type NetworkConfig struct {
    ListenAddrs      []string       // Multiaddrs to listen on
    MaxInboundPeers  int           // Max incoming connections
    MaxOutboundPeers int           // Max outgoing connections
    HandshakeTimeout Duration      // Handshake completion deadline
    DialTimeout      Duration      // Connection dialing deadline
    AddressBookPath  string        // Peer address persistence
    Seeds            SeedsConfig   // Seed node configuration
}

```text

### 10.4 Role Configuration

```go
type NodeRole string

const (
    RoleValidator NodeRole = "validator"
    RoleFull      NodeRole = "full"
    RoleSeed      NodeRole = "seed"
    RoleLight     NodeRole = "light"
    RoleArchive   NodeRole = "archive"
)

```text

### 10.5 Example Configuration

**File:** examples/config.example.toml

```toml

# Blockberry Node Configuration

[node]
chain_id = "my-chain-1"
protocol_version = 1
private_key_path = "data/node_key.bin"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
dial_timeout = "10s"
address_book_path = "data/addrbook.json"

[network.seeds]
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/12D3KooW...",
    "/ip4/seed2.example.com/tcp/26656/p2p/12D3KooW..."
]

role = "full"

[pex]
enabled = true
request_interval = "30s"
max_addresses_per_response = 100

[statesync]
enabled = false
trust_height = 0
trust_hash = ""
discovery_interval = "10s"
chunk_request_timeout = "30s"

[mempool]
max_txs = 5000
max_bytes = 104857600  # 100MB
cache_size = 10000

[blockstore]
backend = "leveldb"
path = "data/blockstore"

[statestore]
path = "data/statestore"
cache_size = 10000

[housekeeping]
latency_probe_interval = "60s"

[metrics]
enabled = false
namespace = "blockberry"
listen_addr = ":9090"

[logging]
level = "info"
format = "text"
output = "stderr"

[limits]
max_tx_size = 1048576        # 1MB
max_block_size = 22020096    # 21MB
max_block_txs = 10000
max_msg_size = 10485760      # 10MB
max_subscribers = 1000
max_subscribers_per_query = 100

```text

---

## 11. Testing Strategy

### 11.1 Test Organization

```text
blockberry/
├── pkg/
│   ├── abi/
│   │   └── application_test.go
│   ├── mempool/
│   │   ├── simple_test.go
│   │   ├── priority_test.go
│   │   └── ttl_test.go
│   └── blockstore/
│       ├── leveldb_test.go
│       └── memory_test.go
└── test/
    └── integration/
        ├── basic_test.go
        └── sync_test.go

```text

### 11.2 Table-Driven Tests

```go
func TestMempool_AddTx(t *testing.T) {
    tests := []struct {
        name    string
        tx      []byte
        wantErr error
    }{
        {
            name:    "valid transaction",
            tx:      []byte("valid tx data"),
            wantErr: nil,
        },
        {
            name:    "empty transaction",
            tx:      []byte{},
            wantErr: types.ErrInvalidTx,
        },
        {
            name:    "duplicate transaction",
            tx:      []byte("duplicate tx"),
            wantErr: types.ErrTxAlreadyExists,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mp := mempool.NewSimpleMempool(100, 1024*1024)
            mp.SetTxValidator(func(tx []byte) error {
                if len(tx) == 0 {
                    return types.ErrInvalidTx
                }
                return nil
            })

            err := mp.AddTx(tt.tx)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("AddTx() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

```text

### 11.3 Mock Implementations

**Null Objects for Testing:**

```go
// NoOpBlockStore: Rejects all operations
type NoOpBlockStore struct{}

func (s *NoOpBlockStore) SaveBlock(...) error {
    return types.ErrStoreClosed
}

// AcceptAllApplication: Accepts everything
type AcceptAllApplication struct {
    *abi.BaseApplication
}

func (app *AcceptAllApplication) CheckTx(ctx context.Context, tx *abi.Transaction) *abi.TxCheckResult {
    return &abi.TxCheckResult{Code: abi.CodeOK}
}

// NullConsensus: No-op consensus
type NullConsensus struct{}

func (c *NullConsensus) ProcessBlock(block *Block) error {
    return nil
}

```text

### 11.4 Test Helpers

```go
// Test helper functions
func makeTestNode(t *testing.T) *Node {
    cfg := config.DefaultConfig()
    cfg.Network.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}

    node, err := node.NewNode(cfg,
        node.WithBlockStore(blockstore.NewInMemoryBlockStore()),
    )
    require.NoError(t, err)

    return node
}

func makeTestTransaction(t *testing.T) *abi.Transaction {
    data := []byte("test transaction")
    return &abi.Transaction{Data: data}
}

```text

### 11.5 Race Detection

All tests run with race detector:

```bash
make test-race

# or
go test -race ./...

```text

**CI Configuration (.github/workflows/test.yml):**

```yaml

- name: Test with race detector
  run: go test -race -v ./...

```text

### 11.6 Integration Tests

**File:** test/integration/basic_test.go

```go
func TestNodeLifecycle(t *testing.T) {
    // Create node
    node := makeTestNode(t)

    // Start
    require.NoError(t, node.Start())
    require.True(t, node.IsRunning())

    // Wait for ready
    time.Sleep(100 * time.Millisecond)

    // Stop
    require.NoError(t, node.Stop())
    require.False(t, node.IsRunning())
}

func TestTransactionFlow(t *testing.T) {
    // Setup two nodes
    node1 := makeTestNode(t)
    node2 := makeTestNode(t)

    require.NoError(t, node1.Start())
    require.NoError(t, node2.Start())

    defer node1.Stop()
    defer node2.Stop()

    // Connect nodes
    // ...

    // Submit transaction to node1
    tx := makeTestTransaction(t)
    err := node1.Mempool().AddTx(tx.Data)
    require.NoError(t, err)

    // Verify propagation to node2
    time.Sleep(500 * time.Millisecond)
    require.True(t, node2.Mempool().HasTx(types.HashTx(tx.Data)))
}

```text

---

## 12. Performance Considerations

### 12.1 Optimization Techniques

#### Buffer Pooling

**Cramberry Writer Pool:**

```go
// Reuse writers to reduce allocations
w := cramberry.GetWriter()
defer cramberry.PutWriter(w)

w.WriteBytes(data)
result := w.BytesCopy()

```text

**Benefits:**

- Reduces GC pressure
- Amortizes allocation cost
- Thread-safe pool (sync.Pool)

#### Batch Operations

**Database Writes:**

```go
batch := new(leveldb.Batch)

for _, op := range operations {
    batch.Put(op.key, op.value)
}

// Single atomic write
db.Write(batch, &opt.WriteOptions{Sync: true})

```text

**Benefits:**

- Fewer I/O operations
- Atomic guarantees
- Better throughput

#### Zero-Copy Operations

**Cramberry Reader:**

```go
r := cramberry.NewReader(data)
payload := r.Remaining()  // Returns slice, no copy

```text

**Read-only Slices:**

```go
// Don't copy if only reading
func (m *Mempool) TxHashes() [][]byte {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Return copy of slice of pointers (not deep copy of data)
    result := make([][]byte, 0, len(m.txs))
    for _, entry := range m.txs {
        result = append(result, entry.hash)  // Share hash bytes
    }
    return result
}

```text

#### LRU Caches with Bounded Memory

```go
import "github.com/hashicorp/golang-lru/v2"

cache, _ := lru.New[string, struct{}](10000)  // Max 10k entries

// Automatic eviction when full
cache.Add(txHash, struct{}{})

```text

**Use Cases:**

- Recent transaction hashes
- Block height → hash mapping
- Peer connection cache

### 12.2 Lock Contention Reduction

#### Read-Write Locks

```go
type Mempool struct {
    mu   sync.RWMutex  // Allows concurrent reads
    txs  map[string]*txEntry
}

// Concurrent reads
func (m *Mempool) HasTx(hash []byte) bool {
    m.mu.RLock()  // Multiple readers allowed
    defer m.mu.RUnlock()

    _, exists := m.txs[string(hash)]
    return exists
}

// Exclusive writes
func (m *Mempool) AddTx(tx []byte) error {
    m.mu.Lock()  // Exclusive access
    defer m.mu.Unlock()

    // ...
}

```text

#### Snapshot-Then-Release Pattern

```go
// Bad: Hold lock during callback
n.mu.RLock()
cb := n.callbacks
cb.OnPeerConnected(peerID)  // Holding lock!
n.mu.RUnlock()

// Good: Release before callback
n.mu.RLock()
cb := n.callbacks
n.mu.RUnlock()

if cb != nil {
    cb.OnPeerConnected(peerID)  // Lock released
}

```text

### 12.3 Scalability

#### Per-Peer Resource Limits

```toml
[limits]
max_tx_size = 1048576         # 1MB per tx
max_block_size = 22020096     # 21MB per block
max_msg_size = 10485760       # 10MB per message

# Per-peer limits
max_pending_txs_per_peer = 100
max_pending_blocks_per_peer = 10

```text

#### Global Resource Quotas

```toml
[mempool]
max_txs = 5000               # Total mempool size
max_bytes = 104857600        # 100MB total

[network]
max_inbound_peers = 40
max_outbound_peers = 10

[limits]
max_subscribers = 1000       # Event bus
max_subscribers_per_query = 100

```text

#### Connection Limits

```go
func (n *Network) ShouldAcceptConnection(peerID peer.ID) bool {
    if n.InboundPeerCount() >= n.cfg.MaxInboundPeers {
        return false
    }

    if n.PeerCount() >= n.cfg.MaxInboundPeers + n.cfg.MaxOutboundPeers {
        return false
    }

    return true
}

```text

### 12.4 Profiling Integration

**CPU Profiling:**

```go
import "runtime/pprof"

// Start CPU profile
f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

```text

**Memory Profiling:**

```go
// Write heap profile
f, _ := os.Create("heap.prof")
pprof.WriteHeapProfile(f)
f.Close()

```text

**HTTP Profiling Endpoint:**

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()

// Access at http://localhost:6060/debug/pprof/

```text

---

## 13. Error Handling

### 13.1 Sentinel Errors

**File:** pkg/types/errors.go

```go
var (
    // Block errors
    ErrBlockNotFound      = errors.New("block not found")
    ErrBlockAlreadyExists = errors.New("block already exists")
    ErrInvalidBlock       = errors.New("invalid block")

    // Transaction errors
    ErrTxNotFound         = errors.New("transaction not found")
    ErrTxAlreadyExists    = errors.New("transaction already exists")
    ErrInvalidTx          = errors.New("invalid transaction")
    ErrTxTooLarge         = errors.New("transaction too large")

    // Mempool errors
    ErrMempoolFull        = errors.New("mempool is full")
    ErrNoTxValidator      = errors.New("transaction validator is required")

    // Connection errors
    ErrChainIDMismatch    = errors.New("chain ID mismatch")
    ErrVersionMismatch    = errors.New("protocol version mismatch")
    ErrHandshakeFailed    = errors.New("handshake failed")
    ErrHandshakeTimeout   = errors.New("handshake timeout")

    // State errors
    ErrStoreClosed        = errors.New("store is closed")
    ErrInvalidProof       = errors.New("invalid proof")
)

```text

**Usage:**

```go
if errors.Is(err, types.ErrBlockNotFound) {
    // Handle specifically
}

```text

### 13.2 Error Wrapping

**Context-Rich Errors:**

```go
return fmt.Errorf("loading block at height %d: %w", height, err)

```text

**Structured Wrapping:**

```go
func WrapMessageError(err error, stream string, msgType string) error {
    return fmt.Errorf("%s/%s: %w", stream, msgType, err)
}

```text

**Chain Preservation:**

```go
// Wrap
err1 := errors.New("base error")
err2 := fmt.Errorf("context: %w", err1)
err3 := fmt.Errorf("more context: %w", err2)

// Unwrap
errors.Is(err3, err1)  // true

```text

### 13.3 Error Checking Patterns

**Sentinel Check:**

```go
if errors.Is(err, types.ErrBlockNotFound) {
    // Handle not found
}

```text

**Type Assertion:**

```go
var rpcErr *rpc.RPCError
if errors.As(err, &rpcErr) {
    log.Printf("RPC error code: %d", rpcErr.Code)
}

```text

### 13.4 ABI Result Codes

```go
const (
    CodeOK                uint32 = 0
    CodeInvalidTx         uint32 = 1
    CodeInsufficientFunds uint32 = 2
    CodeNotAuthorized     uint32 = 3
    CodeExecutionFailed   uint32 = 4
    CodeInvalidNonce      uint32 = 5
    CodeInvalidSignature  uint32 = 6
    // 100-999: App-specific codes
)

```text

---

## 14. Security Architecture

### 14.1 Fail-Closed Security Model

Every entry point defaults to rejection:

**Transaction Validation:**

```go
var DefaultTxValidator = func(tx []byte) error {
    return fmt.Errorf("%w: no validator configured", types.ErrNoTxValidator)
}

```text

**Block Validation:**

```go
var DefaultBlockValidator = func(height int64, hash []byte, data []byte) error {
    return fmt.Errorf("%w: no block validator configured", types.ErrNoBlockValidator)
}

```text

**Application Methods:**

```go
func (app *BaseApplication) CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult {
    return &TxCheckResult{
        Code:  CodeNotAuthorized,
        Error: errors.New("CheckTx not implemented"),
    }
}

```text

### 14.2 Input Validation

**Handshake Validation:**

```go
// Chain ID must match
if *req.ChainId != h.chainID {
    _ = h.network.TempBanPeer(peerID, TempBanDurationChainMismatch, reason)
    return ErrChainIDMismatch
}

// Protocol version must be compatible
if *req.Version != h.protocolVersion {
    _ = h.network.TempBanPeer(peerID, TempBanDurationVersionMismatch, reason)
    return ErrVersionMismatch
}

// Public key length
if len(resp.PublicKey) != ed25519.PublicKeySize {
    _ = h.network.Disconnect(peerID)
    return ErrHandshakeFailed
}

```text

**Message Size Limits:**

```go
if len(msgData) > MaxMessageSize {
    return ErrMessageTooLarge
}

```text

### 14.3 Attack Mitigation

#### Eclipse Attack Protection

**Strategies:**

1. Peer diversity (subnet limits)
2. Outbound preference (maintain minimum outbound)
3. Connection limits
4. Persistent address book
5. Hardcoded seed nodes

```go
// Limit peers per /24 subnet
subnet := ip.Mask(net.CIDRMask(24, 32))
if n.countPeersInSubnet(subnet) >= MaxPeersPerSubnet {
    return false
}

```text

#### Sybil Attack Resistance

**Measures:**

1. Peer scoring and reputation
2. Connection limits per IP
3. Resource-based penalties
4. Validator staking (application layer)

#### Resource Exhaustion Protection

**Limits:**

```toml
[limits]
max_tx_size = 1048576
max_block_size = 22020096
max_msg_size = 10485760
max_subscribers = 1000

```text

**Rate Limiting:**

- Per-peer message rate limits
- Global resource quotas
- LRU cache bounded memory

#### Timing Attack Prevention

**Constant-Time Comparisons:**

```go
import "crypto/subtle"

// Compare public keys
if subtle.ConstantTimeCompare(key1, key2) != 1 {
    return errors.New("key mismatch")
}

```text

### 14.4 Penalty System

| Violation | Penalty Points | Action |
|-----------|----------------|--------|
| Chain ID mismatch | 1000 | Permanent ban |
| Version mismatch | 1000 | Permanent ban |
| Invalid message | 10 | Accumulate |
| Protocol violation | 50 | Accumulate |
| Duplicate message | 5 | Accumulate |

**Thresholds:**

- 100 points: 1 hour temporary ban
- 1000 points: Permanent blacklist

**Decay:**
Penalty points decay over time (e.g., 50% every 24 hours)

### 14.5 Defensive Deep Copying

**File:** internal/handlers/handshake.go

```go
func (h *HandshakeHandler) GetPeerInfo(peerID peer.ID) *PeerHandshakeState {
    h.mu.RLock()
    defer h.mu.RUnlock()

    state, exists := h.peers[peerID]
    if !exists {
        return nil
    }

    // Deep copy to prevent mutation
    return &PeerHandshakeState{
        PeerPubKey:   append([]byte(nil), state.PeerPubKey...),
        ChainID:      state.ChainID,
        Version:      state.Version,
        LatestHeight: state.LatestHeight,
    }
}

```text

**Rationale:**

- Prevents external mutation of internal state
- Avoids race conditions
- Security hardening

---

## 15. Deployment Architecture

### 15.1 Build Process

**Makefile:**

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS = -X github.com/blockberries/blockberry/pkg/version.Version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/blockberry ./cmd/blockberry

test:
	go test -race -v ./...

test-integration:
	go test -v ./test/integration/...

lint:
	golangci-lint run

clean:
	rm -rf bin/

```text

**Version Injection:**

```go
package version

var Version = "dev"  // Overridden at build time

func GetVersion() string {
    return Version
}

```text

### 15.2 Configuration Management

**Loading Priority:**

1. Command-line flags (highest)
2. Environment variables
3. Config file (TOML)
4. Defaults (lowest)

**Example:**

```go
cfg := config.DefaultConfig()

// Load from file
if configPath != "" {
    fileCfg, err := config.LoadConfig(configPath)
    if err != nil {
        log.Fatal(err)
    }
    cfg = fileCfg
}

// Override with env vars
if chainID := os.Getenv("BLOCKBERRY_CHAIN_ID"); chainID != "" {
    cfg.Node.ChainID = chainID
}

// Override with flags
if *flagChainID != "" {
    cfg.Node.ChainID = *flagChainID
}

```text

### 15.3 Monitoring

#### Prometheus Metrics

**Exposed Metrics:**

```text

# Consensus
blockberry_consensus_height
blockberry_consensus_round
blockberry_consensus_proposals_received_total
blockberry_consensus_votes_received_total

# Mempool
blockberry_mempool_size
blockberry_mempool_size_bytes
blockberry_mempool_tx_added_total
blockberry_mempool_tx_removed_total

# Network
blockberry_network_peers
blockberry_network_bytes_sent_total
blockberry_network_bytes_received_total

# Application
blockberry_app_begin_block_duration_seconds
blockberry_app_execute_tx_duration_seconds
blockberry_app_commit_duration_seconds

```text

**Endpoint:**

```text
http://localhost:9090/metrics

```text

#### OpenTelemetry Tracing

**Trace Example:**

```text
Trace: Block Execution
  Span: BeginBlock (50ms)
  Span: ExecuteTx #1 (10ms)
  Span: ExecuteTx #2 (12ms)
  Span: ExecuteTx #3 (15ms)
  Span: EndBlock (20ms)
  Span: Commit (30ms)
Total: 137ms

```text

**Exporters:**

- OTLP (gRPC/HTTP)
- Zipkin
- Jaeger
- Stdout (dev)

#### Structured Logging

**File:** pkg/logging/logger.go

```go
logger.Info("Block committed",
    "height", height,
    "hash", hex.EncodeToString(hash),
    "txs", len(txs),
    "duration_ms", duration.Milliseconds(),
)

```text

**Output:**

```json
{
  "time": "2026-02-02T10:15:30Z",
  "level": "INFO",
  "msg": "Block committed",
  "height": 1000,
  "hash": "1a2b3c4d...",
  "txs": 42,
  "duration_ms": 137
}

```text

### 15.4 Operations

#### Graceful Shutdown

**Signal Handling:**

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

<-sigCh
log.Println("Shutting down...")

if err := node.Stop(); err != nil {
    log.Printf("Shutdown error: %v", err)
}

```text

#### Data Directory Layout

```text
data/
├── blockstore/
│   ├── CURRENT
│   ├── LOCK
│   ├── LOG
│   └── *.ldb
├── statestore/
│   ├── CURRENT
│   ├── LOCK
│   └── *.ldb
├── addrbook.json
└── node_key.bin

```text

#### Backup Considerations

**What to Backup:**

- `blockstore/`: Full block history
- `statestore/`: Application state
- `node_key.bin`: Node private key
- `config.toml`: Configuration

**What NOT to Backup:**

- `addrbook.json`: Regenerates
- `*.log`: Transient

---

## 16. Dependency Management

### 16.1 Module Dependencies

**File:** go.mod

```go
module github.com/blockberries/blockberry

go 1.25.6

require (
    github.com/blockberries/cramberry v1.5.5
    github.com/blockberries/glueberry v1.2.10
    github.com/blockberries/looseberry v0.0.0
    github.com/blockberries/avlberry v0.0.0
    github.com/cosmos/ics23/go v0.10.0
    github.com/syndtr/goleveldb v1.0.1-0.20210819022825-2ae1ddf74ef7
    github.com/dgraph-io/badger/v4 v4.9.0
    github.com/prometheus/client_golang v1.22.0
    go.opentelemetry.io/otel v1.39.0
    google.golang.org/grpc v1.78.0
)

replace github.com/blockberries/looseberry => ../looseberry
replace github.com/blockberries/avlberry => ../avlberry

```text

### 16.2 Key External Dependencies

| Dependency | Purpose | License |
|------------|---------|---------|
| **Glueberry** | P2P networking | MIT |
| **Cramberry** | Binary serialization | MIT |
| **Looseberry** | DAG mempool | MIT |
| **avlberry** | Merkle tree | MIT |
| **LevelDB** | Key-value store | BSD-3 |
| **BadgerDB** | Key-value store | Apache 2.0 |
| **Prometheus** | Metrics | Apache 2.0 |
| **OpenTelemetry** | Tracing | Apache 2.0 |
| **gRPC** | RPC framework | Apache 2.0 |

### 16.3 Version Strategy

**Semantic Versioning:**

- Major: Breaking changes
- Minor: New features (backward compatible)
- Patch: Bug fixes

**Dependency Updates:**

```bash

# Update dependencies
go get -u ./...

# Tidy
go mod tidy

# Verify
go mod verify

```text

---

## 17. Extension Points

### 17.1 Pluggable Components

#### Custom Consensus Engine

```go
// Implement ConsensusEngine interface
type MyConsensus struct {
    deps consensus.ConsensusDependencies
}

func (c *MyConsensus) Initialize(deps consensus.ConsensusDependencies) error {
    c.deps = deps
    return nil
}

// Register with factory
func init() {
    consensus.RegisterConsensus("my-consensus", func(cfg *consensus.ConsensusConfig) (consensus.ConsensusEngine, error) {
        return &MyConsensus{}, nil
    })
}

// Use in config
[consensus]
type = "my-consensus"

```text

#### Custom Mempool

```go
type MyMempool struct {
    // Custom implementation
}

func (m *MyMempool) AddTx(tx []byte) error {
    // Custom logic
}

// Use in node
node, _ := node.NewNode(cfg,
    node.WithMempool(NewMyMempool()),
)

```text

#### Custom BlockStore

```go
type MyBlockStore struct {
    // Custom storage backend
}

func (s *MyBlockStore) SaveBlock(height int64, hash []byte, data []byte) error {
    // Custom persistence
}

// Use in node
node, _ := node.NewNode(cfg,
    node.WithBlockStore(NewMyBlockStore()),
)

```text

### 17.2 Customization Patterns

#### Factory Pattern

```go
type Factory struct {
    registry map[string]Constructor
}

func (f *Factory) Register(name string, ctor Constructor) {
    f.registry[name] = ctor
}

func (f *Factory) Create(name string, config any) (Component, error) {
    ctor, exists := f.registry[name]
    if !exists {
        return nil, fmt.Errorf("unknown type: %s", name)
    }
    return ctor(config)
}

```text

#### Functional Options

```go
type Option func(*Node) error

func WithMempool(mp mempool.Mempool) Option {
    return func(n *Node) error {
        n.mempool = mp
        return nil
    }
}

node, err := NewNode(cfg,
    WithMempool(myMempool),
    WithBlockStore(myBlockStore),
)

```text

#### Callbacks

```go
type NodeCallbacks struct {
    OnPeerConnected    func(peer.ID, bool) error
    OnPeerDisconnected func(peer.ID) error
    OnBlockCommitted   func(int64, []byte) error
}

node, _ := NewNode(cfg,
    node.WithCallbacks(&NodeCallbacks{
        OnBlockCommitted: func(height int64, hash []byte) error {
            log.Printf("Block %d committed", height)
            return nil
        },
    }),
)

```text

---

## 18. Development Workflow

### 18.1 Build Commands

```bash

# Build
make build

# Test
make test

# Test with race detection
make test-race

# Integration tests
make test-integration

# Lint
make lint

# Coverage
make coverage

# Clean
make clean

```text

### 18.2 Code Generation

**Cramberry Schema:**

```bash

# Generate from blockberry.cram
cramberry generate -lang go -out ./schema ./blockberry.cram

```text

**Regeneration triggers:**

- Modified `blockberry.cram`
- Upgraded `cramberry` version

### 18.3 Development Guidelines

From `CLAUDE.md`:

**Mandatory Workflow:**

1. Ultrathink about implementation decisions
2. Re-read CLAUDE.md and documentation
3. Implement feature
4. Write comprehensive tests
5. Run all tests (fix failures)
6. Run integration tests
7. Verify build (no errors/warnings)
8. Ensure no mention of AI tools
9. Append summary to PROGRESS_REPORT.md
10. Commit with comprehensive message (no AI co-authoring)

---

## 19. Future Roadmap

### 19.1 Potential Improvements

**Performance:**

- Lock-free data structures for hot paths
- SIMD optimizations for cryptography
- Memory-mapped block storage
- Parallel transaction execution

**Features:**

- Enhanced state sync protocol
- Advanced pruning strategies
- IBC (Inter-Blockchain Communication)
- Light client improvements
- Monitoring dashboards

**Scalability:**

- Sharding support
- Pipelined block processing
- Optimistic execution

### 19.2 Technical Debt

**Identified Areas:**

- Some components use `sync.Mutex` where `sync.RWMutex` would be better
- Message handling could benefit from priority queuing
- More comprehensive metrics coverage
- Additional integration tests

---

## 20. Glossary

### Key Terms

**ABI (Application Binary Interface)**
The contract between the blockchain framework and application logic. Defines lifecycle methods (CheckTx, ExecuteTx, Commit, etc.) that applications must implement.

**BFT (Byzantine Fault Tolerance)**
Consensus algorithm family that tolerates up to f Byzantine (malicious) failures in a system of 3f+1 nodes.

**Cramberry**
High-performance binary serialization library with polymorphic message support and zero-copy optimizations.

**DAG (Directed Acyclic Graph)**
Graph structure used by some consensus algorithms (e.g., Looseberry) for partial ordering of transactions.

**Eclipse Attack**
Attack where malicious nodes monopolize all connections to isolate a victim from the honest network.

**Fail-Closed**
Security principle where systems default to denial/rejection rather than acceptance. Requires explicit configuration to allow operations.

**Glueberry**
Secure P2P networking library providing encrypted streams, peer discovery, and connection management.

**AVL+ (avlberry)**
Self-balancing binary tree used for merkleized state storage. Provides cryptographic proofs of state.

**ICS23**
Cosmos standard for merkle proof format, enabling interoperability with IBC (Inter-Blockchain Communication).

**Looseberry**
DAG-based mempool implementation with certified batch ordering and high throughput.

**Mempool**
Transaction pool storing pending transactions before block inclusion.

**Merkle Proof**
Cryptographic proof that a key-value pair exists in a merkle tree, verifiable against the root hash.

**PEX (Peer Exchange)**
Protocol for peer discovery where nodes share known peer addresses.

**Quorum**
Minimum number of votes required for consensus decision. In BFT systems, typically 2f+1 out of 3f+1 validators.

**State Sync**
Protocol for rapidly syncing blockchain state by downloading snapshots rather than replaying all blocks.

**Block Sync (Fast Sync)**
Protocol for downloading batches of historical blocks to catch up to network tip.

**Stream**
Logical communication channel multiplexed over a single P2P connection. Each protocol (handshake, pex, transactions, blocks, consensus) uses its own stream.

**Validator**
Node authorized to participate in consensus and propose/vote on blocks.

**Zero-Copy**
Optimization technique that avoids copying data, instead sharing references to existing memory.

---

## Conclusion

Blockberry is a comprehensive, production-ready blockchain node framework built on solid architectural principles:

- **Modularity**: Every major component is pluggable
- **Safety**: Fail-closed defaults, defensive programming, comprehensive validation
- **Performance**: Optimized for blockchain workloads with batching, pooling, and zero-copy operations
- **Observability**: Rich metrics, tracing, and structured logging throughout
- **Correctness**: Deterministic execution, race-free concurrency, extensive testing
- **Extensibility**: Clear extension points for custom consensus, mempool, and storage implementations

The architecture balances flexibility with safety, performance with maintainability, and simplicity with power, providing a solid foundation for building production blockchain applications.

---

**Documentation Version:** 1.0
**Last Updated:** 2026-02-02
**Framework Version:** Based on commit 2368b5f
**Contact:** https://github.com/blockberries/blockberry

