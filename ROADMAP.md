# Blockberry Roadmap

This document outlines future improvements, enhancements, and features planned for blockberry. Items are organized by priority and category, with clear descriptions of scope and rationale.

---

## Table of Contents

1. [Current State Summary](#current-state-summary)
2. [Critical Fixes](#critical-fixes)
3. [High Priority Enhancements](#high-priority-enhancements)
4. [Medium Priority Improvements](#medium-priority-improvements)
5. [Future Features](#future-features)
6. [Technical Debt](#technical-debt)
7. [Documentation & Developer Experience](#documentation--developer-experience)
8. [Performance Optimizations](#performance-optimizations)
9. [Security Hardening](#security-hardening)
10. [Ecosystem Integration](#ecosystem-integration)

---

## Current State Summary

Blockberry is a **production-ready** modular blockchain node framework with:

| Component | Status | Notes |
|-----------|--------|-------|
| Core Types & Errors | ✅ Complete | 32 sentinel errors, hash functions |
| Configuration | ✅ Complete | TOML loading, validation, defaults |
| Block Storage | ✅ Complete | LevelDB + in-memory implementations |
| State Storage | ⚠️ 95% Complete | IAVL-based, proof verification stub |
| Mempool | ⚠️ Basic | FIFO only, no prioritization |
| P2P Network | ✅ Complete | Peer management, scoring, encryption |
| Peer Discovery | ✅ Complete | Full PEX protocol |
| Block Sync | ✅ Complete | Batch synchronization |
| Message Handlers | ✅ Complete | All streams implemented |
| Node Coordinator | ✅ Complete | Lifecycle, event routing |
| Integration Tests | ✅ Complete | Multi-node scenarios |

**Test Coverage**: 22 test files, ~6,000 lines of tests, 150+ test functions

---

## Critical Fixes

Issues that should be addressed before any production deployment.

### 1. Complete Proof Verification Implementation

**Location**: `statestore/store.go:70-77`

**Current State**: `Proof.Verify()` always returns `false`

**Required Work**:
- Implement ICS23 proof verification using cosmos/ics23/go
- Parse `ProofBytes` and verify against provided root hash
- Add comprehensive tests for existence and non-existence proofs
- Document proof format and verification requirements

**Impact**: Critical for light clients, cross-chain verification, and state proofs

```go
// Current (stub):
func (p *Proof) Verify(rootHash []byte) (bool, error) {
    return false, nil
}

// Needed:
func (p *Proof) Verify(rootHash []byte) (bool, error) {
    // Parse ICS23 proof from ProofBytes
    // Verify against rootHash using ics23.VerifyMembership or VerifyNonMembership
}
```

### 2. Fix Outbound Connection Detection

**Location**: `node/node.go:376`

**Current State**: `isOutbound` is hardcoded to `true`

**Required Work**:
- Query glueberry for actual connection direction
- Add `IsOutbound(peerID)` method to glueberry if not present
- Update PeerManager to accurately track connection direction
- Fix peer counts for inbound vs outbound limits

**Impact**: Affects peer limit enforcement and connection management

---

## High Priority Enhancements

Features that significantly improve functionality or user experience.

### 3. Priority-Based Mempool

**Current State**: FIFO ordering only via `SimpleMempool`

**Required Work**:
- Create `PriorityMempool` implementation
- Support configurable ordering strategies:
  - Fee-per-byte ordering
  - Gas price ordering
  - Custom priority functions
- Maintain backwards compatibility with `Mempool` interface
- Add eviction policies for low-priority transactions when full

**Interface Addition**:
```go
type PriorityMempool interface {
    Mempool
    SetPriorityFunc(func(tx []byte) int64)
    UpdatePriority(hash []byte, priority int64) error
}
```

**Impact**: Essential for fee-based blockchains and fair transaction ordering

### 4. Transaction Expiration/TTL

**Current State**: Transactions persist in mempool indefinitely

**Required Work**:
- Add `ttl` field to transaction metadata
- Background goroutine for periodic cleanup
- Configurable default TTL in `MempoolConfig`
- Track insertion timestamp per transaction

**Config Addition**:
```toml
[mempool]
default_ttl = "1h"      # Default transaction lifetime
cleanup_interval = "1m"  # How often to check for expired txs
```

**Impact**: Prevents mempool bloat with stale transactions

### 5. Rate Limiting for Message Receipt

**Current State**: No limits on incoming message rate

**Required Work**:
- Implement per-peer rate limiting
- Configurable limits per stream type
- Token bucket or leaky bucket algorithm
- Automatic penalty for rate limit violations

**Config Addition**:
```toml
[network.rate_limits]
transactions_per_second = 100
blocks_per_second = 10
pex_requests_per_minute = 10
```

**Impact**: Prevents DoS attacks and bandwidth exhaustion

### 6. Metrics and Observability

**Current State**: No metrics or structured logging

**Required Work**:
- Add Prometheus metrics for:
  - Peer counts (inbound, outbound)
  - Message counts per stream
  - Block heights (local, max peer)
  - Mempool size and bytes
  - Sync state and progress
  - Latency histograms
- Configurable metrics endpoint
- Add structured logging with zerolog or slog
- Trace ID propagation for debugging

**Config Addition**:
```toml
[metrics]
enabled = true
listen_addr = "127.0.0.1:9090"
```

**Metrics Examples**:
```
blockberry_peers_total{direction="inbound"} 25
blockberry_peers_total{direction="outbound"} 10
blockberry_mempool_size 1234
blockberry_mempool_bytes 5678901
blockberry_block_height 100000
blockberry_sync_state{state="synced"} 1
blockberry_messages_received_total{stream="transactions"} 50000
```

**Impact**: Essential for production operations and debugging

### 7. Firewall Detection Implementation

**Location**: `handlers/housekeeping.go:253-292`

**Current State**: Stub implementation that echoes endpoint back

**Required Work**:
- Implement actual reachability testing
- Node A asks Node B to connect to Node A's external address
- Node B attempts connection and reports success/failure
- Track reachability status per node
- Auto-detect NAT and report it

**Impact**: Helps nodes understand their network reachability

---

## Medium Priority Improvements

Features that improve robustness, efficiency, or developer experience.

### 8. State Pruning Strategy

**Current State**: All IAVL versions retained indefinitely

**Required Work**:
- Implement configurable pruning strategies:
  - `none`: Keep all versions
  - `recent`: Keep last N versions
  - `interval`: Keep every Nth version
- Background pruning goroutine
- Pruning during idle periods

**Config Addition**:
```toml
[statestore]
pruning_strategy = "recent"  # none, recent, interval
pruning_keep_recent = 100    # Number of recent versions to keep
pruning_interval = 10        # For interval strategy
```

**Impact**: Reduces disk usage for long-running nodes

### 9. Block Pruning

**Current State**: All blocks retained indefinitely

**Required Work**:
- Implement block pruning for archival nodes
- Keep headers for pruned blocks (for sync verification)
- Update `Base()` as blocks are pruned
- Configurable minimum blocks to retain

**Config Addition**:
```toml
[blockstore]
pruning_enabled = true
min_retain_blocks = 10000  # Minimum blocks to keep
```

**Impact**: Enables non-archival node operation

### 10. BadgerDB Block Store Implementation

**Current State**: Only LevelDB supported

**Required Work**:
- Create `BadgerBlockStore` implementing `BlockStore` interface
- Add backend selection in config
- Performance benchmarks vs LevelDB
- Migration tooling between backends

**Config Addition**:
```toml
[blockstore]
backend = "badgerdb"  # leveldb, badgerdb
```

**Impact**: Provides alternative with different performance characteristics

### 11. Connection Pooling and Reuse

**Current State**: No connection pooling for frequent operations

**Required Work**:
- Pool glueberry connections efficiently
- Reuse stream writers/readers
- Reduce allocation overhead for message encoding
- Buffer pool for cramberry serialization

**Impact**: Improved performance under high load

### 12. Enhanced Handshake Recovery

**Current State**: Lost HelloFinalize may cause connection hang

**Required Work**:
- Add handshake timeout per-peer
- Retry HelloFinalize on timeout
- Clean up stuck handshake states
- Configurable retry parameters

**Impact**: More robust connection establishment

### 13. Bandwidth Allocation Per Stream

**Current State**: All streams share bandwidth equally

**Required Work**:
- Priority queuing for outbound messages
- Configurable bandwidth weights per stream
- Fair queuing implementation
- Back-pressure signaling

**Config Addition**:
```toml
[network.bandwidth]
consensus_weight = 100   # Highest priority
blocks_weight = 80
transactions_weight = 50
pex_weight = 10          # Lowest priority
```

**Impact**: Ensures critical messages (consensus) aren't delayed

---

## Future Features

Longer-term enhancements for expanded functionality.

### 14. Light Client Support

**Current State**: Only full nodes supported

**Required Work**:
- Header-only sync mode
- State proof verification
- On-demand block/state fetching
- Compact block notifications
- Light client P2P protocol extensions

**New Message Types**:
```
HeadersRequest { start_height, count }
HeadersResponse { headers[] }
StateProofRequest { key, height }
StateProofResponse { proof, value }
```

**Impact**: Enables mobile and resource-constrained clients

### 15. Snapshot Sync

**Current State**: Must sync from genesis or latest known block

**Required Work**:
- State snapshot creation at configurable intervals
- Snapshot advertisement via PEX
- Snapshot download protocol
- State verification after snapshot application
- Catch-up sync from snapshot height

**New Message Types**:
```
SnapshotRequest { }
SnapshotResponse { height, hash, chunks_count }
SnapshotChunkRequest { height, chunk_index }
SnapshotChunkResponse { data }
```

**Impact**: Dramatically reduces sync time for new nodes

### 16. Validator Set Management

**Current State**: No built-in validator tracking

**Required Work**:
- Validator set interface and storage
- Validator updates via consensus
- Stake/power tracking
- Validator address book integration

**Impact**: Foundation for PoS consensus implementations

### 17. RPC Server

**Current State**: No RPC interface

**Required Work**:
- HTTP/JSON-RPC server
- WebSocket subscriptions
- gRPC support
- Standard endpoints:
  - `/status` - Node status
  - `/block` - Block queries
  - `/tx` - Transaction queries
  - `/state` - State queries
  - `/peers` - Peer information

**Config Addition**:
```toml
[rpc]
enabled = true
listen_addr = "127.0.0.1:26657"
max_connections = 100
```

**Impact**: Essential for applications and tooling

### 18. Transaction Indexing

**Current State**: No transaction indexing

**Required Work**:
- Configurable transaction indexing
- Index by hash, sender, receiver (application-defined)
- Query interface
- Index storage (separate from blocks)

**Config Addition**:
```toml
[indexing]
enabled = true
index_path = "data/txindex"
```

**Impact**: Enables transaction history queries

### 19. Event System

**Current State**: No application-level events

**Required Work**:
- Event emission from block execution
- Event subscription interface
- WebSocket event streaming
- Event filtering by type/attributes

**Impact**: Enables reactive applications and monitoring

### 20. Multi-Chain Support

**Current State**: Single chain per node

**Required Work**:
- Multiple chain ID support
- Isolated stores per chain
- Cross-chain message routing
- Shared peer management

**Impact**: Enables relay nodes and multi-chain applications

---

## Technical Debt

Code quality and maintainability improvements.

### 21. Improve Test Coverage

**Areas Needing More Tests**:
- `p2p/network.go` - Only 37 lines of tests
- `node/node.go` - Integration with all components
- Error path testing in handlers
- Concurrent access stress tests

**Target**: 80%+ coverage in all packages

### 22. Add Benchmarks

**Missing Benchmarks**:
- Message encoding/decoding
- Block store operations under load
- Mempool operations with large tx counts
- PEX address book operations
- Concurrent peer manager access

**Impact**: Enables performance regression detection

### 23. Refactor Duplicate Code

**Identified Duplications**:
- `encodeMessage()` repeated in each reactor
- Type ID prefix handling in all handlers
- Similar error handling patterns

**Solution**: Create shared message encoding utilities

### 24. Consistent Context Usage

**Current State**: Mixed context usage across components

**Required Work**:
- Add `context.Context` to all public APIs
- Proper cancellation propagation
- Timeout context support
- Trace context integration

---

## Documentation & Developer Experience

### 25. API Documentation

**Required Work**:
- Godoc comments for all public types/methods
- Package-level documentation
- Example code in `example_test.go` files
- Architecture decision records (ADRs)

### 26. Protocol Specification

**Required Work**:
- Formal specification of all message types
- State machine diagrams for handlers
- Handshake sequence diagrams
- PEX protocol specification

### 27. Tutorials and Guides

**Required Work**:
- "Build Your First Chain" tutorial
- Consensus implementation guide
- Custom mempool implementation guide
- Production deployment guide
- Troubleshooting guide

### 28. CLI Tool

**Required Work**:
- `blockberryd` binary for running nodes
- Command-line configuration
- Key generation utilities
- Debug commands

---

## Performance Optimizations

### 29. Memory Pool for Messages

**Current State**: Allocations for each message

**Required Work**:
- Implement `sync.Pool` for common message types
- Buffer pooling for cramberry encoding
- Reduce allocations in hot paths

**Impact**: Reduced GC pressure under load

### 30. Parallel Block Validation

**Current State**: Sequential block processing

**Required Work**:
- Parallel transaction validation
- Concurrent signature verification
- Pipeline block processing

**Impact**: Higher throughput for block processing

### 31. Optimized Hash Caching

**Current State**: Re-hashing on each access

**Required Work**:
- Cache transaction hashes
- Cache block hashes
- Lazy hash computation

**Impact**: Reduced CPU usage for repeated operations

---

## Security Hardening

### 32. Enhanced Peer Scoring

**Current State**: Basic penalty system

**Required Work**:
- Reputation decay over time
- Positive scoring for good behavior
- Peer recommendation based on reputation
- Eclipse attack detection

### 33. Message Size Limits

**Current State**: No explicit size limits

**Required Work**:
- Configurable max message size per stream
- Reject oversized messages early
- Log suspicious large messages

**Config Addition**:
```toml
[network.limits]
max_block_size = 10485760      # 10MB
max_transaction_size = 1048576  # 1MB
max_pex_response = 102400       # 100KB
```

### 34. DoS Protection

**Current State**: Limited protection

**Required Work**:
- Request rate limiting per peer
- Connection rate limiting
- Memory usage limits
- CPU usage monitoring
- Automatic peer banning for abuse

### 35. Audit and Security Review

**Required Work**:
- Third-party security audit
- Fuzzing for message parsers
- Static analysis with security linters
- Dependency vulnerability scanning

---

## Ecosystem Integration

### 36. Cosmos SDK Compatibility

**Required Work**:
- ABCI 2.0 compatible interface
- CometBFT-style state sync
- Cosmos IBC compatibility layer

### 37. Ethereum Compatibility Layer

**Required Work**:
- EVM state storage adapter
- Ethereum RPC compatibility
- Web3 provider support

### 38. Cross-Platform Builds

**Required Work**:
- Windows support testing
- ARM64 builds
- Docker images
- Reproducible builds

---

## Priority Matrix

| Priority | Item | Effort | Impact |
|----------|------|--------|--------|
| Critical | Proof Verification | Medium | High |
| Critical | Outbound Detection | Low | Medium |
| High | Priority Mempool | High | High |
| High | Transaction TTL | Medium | Medium |
| High | Rate Limiting | Medium | High |
| High | Metrics | High | High |
| Medium | State Pruning | Medium | Medium |
| Medium | Block Pruning | Medium | Medium |
| Medium | BadgerDB Backend | Medium | Low |
| Medium | Bandwidth Allocation | High | Medium |
| Future | Light Client | Very High | High |
| Future | Snapshot Sync | Very High | High |
| Future | RPC Server | High | High |

---

## Version Milestones

### v0.2.0 - Stability Release
- [ ] Complete proof verification
- [ ] Fix outbound connection detection
- [ ] Add rate limiting
- [ ] Basic metrics support

### v0.3.0 - Production Ready
- [ ] Priority mempool
- [ ] Transaction TTL
- [ ] State and block pruning
- [ ] Full metrics suite
- [ ] Firewall detection

### v0.4.0 - Feature Complete
- [ ] Light client support
- [ ] Snapshot sync
- [ ] RPC server
- [ ] Transaction indexing

### v1.0.0 - Stable Release
- [ ] Security audit completed
- [ ] Full documentation
- [ ] Production deployment guides
- [ ] Ecosystem integrations

---

## Contributing

When implementing roadmap items:

1. Create an issue referencing the roadmap item
2. Follow the implementation guidelines in CLAUDE.md
3. Add comprehensive tests
4. Update documentation
5. Update this roadmap with completion status

---

*Last updated: January 2026*
