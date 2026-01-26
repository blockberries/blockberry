# Blockberry Roadmap

This document outlines the development roadmap for Blockberry, organized by priority and phase. Items are based on a comprehensive codebase review and industry best practices.

---

## Current Status (v1.0.0)

Blockberry v1.0.0 is a production-ready blockchain node framework with the following capabilities:

### Core Components (Complete)
- **Node Coordinator**: Full lifecycle management with graceful shutdown
- **Mempool**: Three implementations (Simple, Priority, TTL) with configurable limits
- **Block Store**: LevelDB-based persistent storage with memory option for testing
- **State Store**: IAVL-based merkleized storage with ICS23 proof generation
- **P2P Networking**: Glueberry-based with encrypted streams, peer management, and rate limiting

### Protocol Handlers (Complete)
- Two-phase handshake with chain/version validation
- Peer Exchange (PEX) with address book persistence
- Transaction gossiping with deduplication
- Block synchronization (batch catch-up)
- Real-time block propagation
- Pluggable consensus message routing

### Observability (Complete)
- Prometheus metrics (peers, blocks, transactions, sync, messages)
- Structured logging via slog
- Per-peer latency tracking

### Safety Features (Complete)
- Per-peer rate limiting (token bucket algorithm)
- Peer scoring with penalty decay
- LRU-bounded peer state (prevents memory leaks)
- Comprehensive input validation
- ICS23 proof verification

---

## Phase 1: Storage & Sync Enhancements

### P1.1: BadgerDB Block Store
**Priority**: High
**Effort**: 1-2 days

The configuration supports "badgerdb" as a backend option, but only LevelDB is implemented.

**Tasks:**
- Implement `BadgerDBBlockStore` in `blockstore/badgerdb.go`
- Add configuration for BadgerDB-specific options (compression, value log)
- Benchmark against LevelDB for various workloads
- Add migration tooling between backends

### P1.2: State Pruning
**Priority**: High
**Effort**: 3-5 days

Currently, all IAVL versions are retained indefinitely, consuming unbounded disk space.

**Tasks:**
- Add `PruningConfig` with strategies: nothing, everything, custom
- Implement `PruneVersions(keepRecent, keepEvery)` in StateStore
- Background pruning goroutine with configurable intervals
- Metrics for pruned versions and disk space reclaimed

### P1.3: State Sync (Fast Sync)
**Priority**: High
**Effort**: 1-2 weeks

New nodes must replay all blocks from genesis. State sync allows bootstrapping from a recent state snapshot.

**Tasks:**
- Define snapshot format (IAVL export with metadata)
- Add `SnapshotStore` interface for snapshot management
- Implement snapshot creation at configurable intervals
- Add snapshot discovery via PEX extension
- Implement state sync reactor for streaming snapshots
- Verification of snapshot integrity via root hash

### P1.4: Block Pruning
**Priority**: Medium
**Effort**: 2-3 days

Archive nodes retain all blocks, but validators and full nodes may prune old blocks.

**Tasks:**
- Add `BlockRetention` config (number of blocks or duration)
- Implement `PruneBlocks(beforeHeight)` in BlockStore
- Track `Base()` height correctly after pruning
- Ensure sync reactor handles pruned peers gracefully

---

## Phase 2: Consensus Framework

### P2.1: Consensus Interface Refinement
**Priority**: High
**Effort**: 3-5 days

The current `ConsensusHandler` interface is minimal. A richer interface would support more consensus patterns.

**Tasks:**
- Define `ConsensusEngine` interface with lifecycle methods
- Add proposal generation hooks (`OnPropose(height, round)`)
- Add vote handling hooks (`OnVote(height, round, type, vote)`)
- Add commit handling (`OnCommit(height, block)`)
- Support for consensus timeouts and round changes

### P2.2: Reference BFT Implementation
**Priority**: Medium
**Effort**: 2-4 weeks

Provide a reference Tendermint-style BFT consensus implementation.

**Tasks:**
- Implement `TendermintConsensus` engine
- Proposal/prevote/precommit message types
- Validator set management
- Round-based state machine
- WAL (Write-Ahead Log) for crash recovery
- Evidence handling for double-signing

### P2.3: Validator Set Updates
**Priority**: Medium
**Effort**: 1 week

Support dynamic validator sets that change at block boundaries.

**Tasks:**
- `ValidatorSet` type with voting power tracking
- `EndBlock()` returns validator updates
- Validator set persistence in state store
- Light client validator set verification

---

## Phase 3: Networking Improvements

### P3.1: Firewall Detection
**Priority**: Medium
**Effort**: 2-3 days

The housekeeping reactor has placeholders for firewall detection.

**Tasks:**
- Implement `FirewallRequest`/`FirewallResponse` messages
- Add reachability probing via third-party peers
- Track and report NAT type (full cone, symmetric, etc.)
- Expose reachability status in metrics

### P3.2: Peer Reputation Persistence
**Priority**: Medium
**Effort**: 1-2 days

Peer scores and penalties are lost on restart.

**Tasks:**
- Serialize peer scores to address book
- Load historical scores on startup
- Gradual trust building for new peers
- Configurable score decay across restarts

### P3.3: Connection Diversity
**Priority**: Low
**Effort**: 2-3 days

Improve resilience by ensuring peer diversity.

**Tasks:**
- Track peer IP subnets and ASNs
- Limit connections per subnet/ASN
- Prefer geographically diverse peers
- Metrics for peer distribution

### P3.4: Protocol Upgrades
**Priority**: Low
**Effort**: 1 week

Support protocol version negotiation and upgrades.

**Tasks:**
- Feature flags in handshake
- Graceful degradation for older peers
- Coordinated upgrade signaling
- Protocol version metrics

---

## Phase 4: Developer Experience

### P4.1: Transaction Indexing
**Priority**: High
**Effort**: 1 week

Enable querying transactions by hash, sender, or other attributes.

**Tasks:**
- Define `TxIndexer` interface
- Implement LevelDB-based indexer
- Index by hash, height, sender (configurable)
- Query API for transaction lookup
- Reindexing tooling for existing chains

### P4.2: Event Subscription System
**Priority**: High
**Effort**: 1 week

Allow clients to subscribe to blockchain events.

**Tasks:**
- Define event types (NewBlock, NewTx, ValidatorSetUpdate, etc.)
- Implement pub/sub mechanism with filters
- WebSocket transport for subscriptions
- Event persistence for replay

### P4.3: RPC API
**Priority**: High
**Effort**: 2 weeks

Provide a standard RPC interface for node interaction.

**Tasks:**
- Define RPC methods (status, broadcast_tx, query, etc.)
- Implement JSON-RPC 2.0 transport
- Add gRPC transport option
- Authentication and rate limiting
- OpenAPI/Swagger documentation

### P4.4: CLI Tooling
**Priority**: Medium
**Effort**: 1 week

Command-line tools for node operation.

**Tasks:**
- `blockberry init` - Initialize node configuration
- `blockberry start` - Start the node
- `blockberry status` - Query node status
- `blockberry peers` - List connected peers
- `blockberry tx` - Submit transactions
- `blockberry query` - Query state

### P4.5: Plugin System
**Priority**: Low
**Effort**: 2 weeks

Allow extending blockberry without forking.

**Tasks:**
- Define plugin interfaces (Mempool, Store, Handler)
- Plugin discovery and loading
- Configuration for plugin-specific settings
- Example plugins

---

## Phase 5: Light Client Support

### P5.1: Light Client Protocol
**Priority**: Medium
**Effort**: 2 weeks

Enable trustless light clients that verify headers without full blocks.

**Tasks:**
- Header-only sync reactor
- Merkle proof verification for state queries
- Trusted height/hash bootstrap
- Bisection algorithm for header verification

### P5.2: Light Client Library
**Priority**: Medium
**Effort**: 1 week

Standalone library for building light clients.

**Tasks:**
- `LightClient` type with sync and query methods
- Multiple backend support (HTTP, WebSocket)
- Proof verification utilities
- Mobile-friendly (minimal dependencies)

---

## Phase 6: Performance Optimization

### P6.1: Parallel Transaction Execution
**Priority**: Medium
**Effort**: 2-3 weeks

Execute independent transactions in parallel during block production.

**Tasks:**
- Dependency analysis for transactions
- Worker pool for parallel execution
- Deterministic ordering of results
- Conflict detection and fallback to serial

### P6.2: Message Compression
**Priority**: Low
**Effort**: 2-3 days

Reduce bandwidth usage with message compression.

**Tasks:**
- Add compression negotiation in handshake
- Implement snappy/zstd compression for large messages
- Per-stream compression settings
- Metrics for compression ratios

### P6.3: Memory Pool Optimization
**Priority**: Low
**Effort**: 1 week

Reduce memory allocations in hot paths.

**Tasks:**
- Object pooling for frequently allocated types
- Buffer reuse for serialization
- Zero-copy message passing where possible
- Memory profiling and optimization

### P6.4: Batch Processing Improvements
**Priority**: Low
**Effort**: 3-5 days

Optimize batch operations for blocks and transactions.

**Tasks:**
- Adaptive batch sizing based on network conditions
- Pipelined block processing during sync
- Parallel block validation
- Metrics for batch efficiency

---

## Phase 7: Security Hardening

### P7.1: Enhanced DDoS Protection
**Priority**: Medium
**Effort**: 1 week

Additional protection against denial-of-service attacks.

**Tasks:**
- Connection rate limiting per IP
- Proof-of-work for new connections (optional)
- Automatic blacklisting of attack sources
- Metrics for attack detection

### P7.2: Sybil Attack Mitigation
**Priority**: Medium
**Effort**: 1 week

Prevent attackers from overwhelming the network with fake nodes.

**Tasks:**
- Peer identity verification options
- Stake-weighted peer selection (for PoS chains)
- IP diversity requirements
- Anomaly detection for peer behavior

### P7.3: Security Audit Preparation
**Priority**: High
**Effort**: 1-2 weeks

Prepare codebase for external security audit.

**Tasks:**
- Document security assumptions
- Review cryptographic usage
- Fuzz testing for parsers and handlers
- Static analysis tool integration

---

## Phase 8: Observability Enhancements

### P8.1: Distributed Tracing
**Priority**: Medium
**Effort**: 1 week

Add tracing support for debugging distributed issues.

**Tasks:**
- OpenTelemetry integration
- Trace context propagation in messages
- Span creation for key operations
- Jaeger/Zipkin export support

### P8.2: Health Check Endpoints
**Priority**: Medium
**Effort**: 2-3 days

Standardized health checks for orchestration systems.

**Tasks:**
- `/health/live` - Node is running
- `/health/ready` - Node is synced and accepting requests
- `/health/startup` - Node has completed initialization
- Kubernetes probe compatibility

### P8.3: Admin API
**Priority**: Low
**Effort**: 1 week

Protected API for node administration.

**Tasks:**
- Peer management (connect, disconnect, ban)
- Configuration hot-reload
- Log level adjustment
- Profiling endpoints (pprof)

---

## Phase 9: Ecosystem Integration

### P9.1: IBC Compatibility
**Priority**: Low
**Effort**: 4-6 weeks

Support Inter-Blockchain Communication protocol.

**Tasks:**
- IBC client interface implementation
- Connection and channel handshakes
- Packet relay support
- ICS-20 token transfer

### P9.2: ABCI Compatibility Layer
**Priority**: Low
**Effort**: 2-3 weeks

Compatibility with Cosmos SDK applications via ABCI.

**Tasks:**
- ABCI server implementation (socket/gRPC)
- Proxy application type
- Tendermint RPC compatibility mode
- Migration guide from Tendermint

---

## Version Milestones

### v1.1.0 - Storage & Performance
- BadgerDB block store (P1.1)
- State pruning (P1.2)
- Block pruning (P1.4)
- Transaction indexing (P4.1)

### v1.2.0 - Developer Experience
- RPC API (P4.3)
- Event subscription (P4.2)
- CLI tooling (P4.4)
- Health check endpoints (P8.2)

### v1.3.0 - Fast Sync
- State sync (P1.3)
- Light client protocol (P5.1)
- Light client library (P5.2)

### v2.0.0 - Consensus Framework
- Consensus interface refinement (P2.1)
- Reference BFT implementation (P2.2)
- Validator set updates (P2.3)

---

## Contributing

We welcome contributions to any roadmap item. Before starting work on a major feature:

1. Open an issue to discuss the approach
2. Reference the roadmap item (e.g., "P1.2: State Pruning")
3. Follow the contribution guidelines in CONTRIBUTING.md

### Priority Legend
- **High**: Critical for production use or highly requested
- **Medium**: Important for completeness but not blocking
- **Low**: Nice-to-have or specialized use cases

### Effort Estimates
Effort estimates assume a single experienced developer. Actual time may vary based on complexity discovered during implementation.

---

*Last Updated: January 2025*
