# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-02-02

### Added

- Comprehensive documentation suite with over 20,000 lines across 41 markdown files:
  - **ARCHITECTURE.md**: Complete 4,681-line architecture guide covering all 20 major system aspects including design principles, component architecture, concurrency patterns, network protocols, security model, and deployment strategies
  - **API_REFERENCE.md**: Godoc-compatible API reference for all 14 public packages with complete method signatures, usage examples, and RPC API documentation for gRPC, JSON-RPC, and WebSocket endpoints
  - **README.md**: Professional GitHub homepage with installation instructions, quickstart guide, feature overview, and development workflow
  - **ARCHITECTURE_ANALYSIS.md**: Deep dive into Go architecture patterns including interface design, concurrency model, dependency injection, and design pattern usage across the codebase
  - **CODEBASE_ANALYSIS.md**: Detailed code analysis covering implementation patterns, security features, performance optimizations, and integration with Glueberry/Cramberry/Looseberry dependencies
  - **docs/getting-started/**: Installation guide, 5-minute quickstart, first application tutorial (KV-store blockchain), and comprehensive configuration reference
  - **docs/tutorials/**: Step-by-step guides for custom mempool implementation, consensus engine integration, storage backend development, and transaction lifecycle
  - **docs/guides/**: Developer guides covering application development (ABI v2.0), consensus engines, testing strategies, benchmarking, production deployment, monitoring, security hardening, and contributing guidelines
  - **docs/concepts/**: Core concept documentation for system architecture, ABI design philosophy, consensus layer, mempool strategies, P2P networking, storage layer, state management, and node roles
  - **docs/reference/**: Complete reference for CLI commands, configuration options, RPC APIs, message protocols, error codes, and terminology glossary
  - **docs/examples/**: Runnable code examples for simple node setup, custom mempool, consensus engine, and complete blockchain applications
- Python script for automated markdown linting fixes (`scripts/fix_markdown.py`) to ensure documentation quality and consistency

### Changed

- Improved markdown formatting in existing documentation files for better readability and linting compliance:
  - Added language specifiers to code fences for proper syntax highlighting
  - Added blank lines around lists and headings per markdown best practices
  - Fixed code fence closures in ABI_DESIGN.md and CODE_REVIEW.md

## [0.1.7] - 2026-01-29

### Security

- **consensus/validators.go**: Fixed `GetProposer()` methods in `SimpleValidatorSet` and `WeightedProposerSelection` returning direct references to internal validators, allowing callers to corrupt internal state.
- **handlers/consensus.go**: Fixed decode functions (`decodeVote`, `decodeCommit`, `decodeBlock`) storing slices pointing to input data buffer without defensive copies, preventing potential corruption if network layer reuses buffers.

### Fixed

- **consensus/validators.go**: Fixed `NewWeightedProposerSelection` storing external validator slice without defensive copy, preventing external mutation of internal state.

## [0.1.6] - 2026-01-29

### Security

- **node/node.go**: Fixed race condition in callback invocation by capturing callbacks under RLock before invoking.
- **consensus/validators.go**: Added `sync.RWMutex` to `WeightedProposerSelection` to prevent data races in concurrent access.
- **handlers/transactions.go**: Fixed unbounded memory growth by enforcing `maxPending` limit for pending transaction requests.

### Fixed

- **blockstore/memory.go**: Added defensive copies in `SaveBlock`, `LoadBlock`, and `LoadBlockByHash` to prevent external mutation of stored data.
- **mempool/looseberry/adapter.go**: Fixed `ReapTxs` returning shallow copies by adding defensive copies.
- **mempool/mempool.go**: Fixed `NewMempool` factory to respect `cfg.Type` configuration, now properly creates `PriorityMempool` and `TTLMempool` when configured.

## [0.1.5] - 2026-01-29

### Security

- **rpc/jsonrpc/server.go**: Fixed Slowloris attack vulnerability by adding `ReadHeaderTimeout` (10s) to HTTP server.
- **rpc/jsonrpc/server.go**: Fixed resource exhaustion vulnerability by adding `IdleTimeout` (120s) to HTTP server.
- **sync/statesync.go**: Fixed timing attack vulnerabilities by replacing `bytes.Equal()` with constant-time `types.HashEqual()` for all hash comparisons.

### Fixed

- **mempool/simple_mempool.go**: Fixed potential state corruption by making defensive copies in `AddTx`, `GetTx`, `ReapTxs`, and `TxHashes`.
- **mempool/priority_mempool.go**: Fixed potential state corruption by making defensive copies in `AddTx`, `GetTx`, `ReapTxs`, and `TxHashes`.
- **mempool/ttl_mempool.go**: Fixed potential state corruption by making defensive copies in `AddTx`, `GetTx`, `ReapTxs`, and `TxHashes`.
- **abi/validator.go**: Fixed shallow copy in `Validators()`, `GetByIndex()`, `GetByAddress()`, and `Proposer()` that exposed internal byte slices to callers.

## [0.1.4] - 2026-01-29

### Security

- **rpc/grpc/ratelimit.go**: Fixed race condition where exempt client/method maps were accessed without synchronization. Added `sync.RWMutex` protection.
- **handlers/handshake.go**: Added ed25519 public key length validation (must be 32 bytes) to prevent panics or undefined behavior in crypto operations.

### Fixed

- **rpc/grpc/server.go**: Fixed TCP listener resource leak when TLS credential loading fails during server startup.
- **events/bus.go**: Fixed timer leak in `PublishWithTimeout` by replacing `time.After()` with `time.NewTimer()` and proper cleanup.
- **handlers/handshake.go**: Fixed shallow copy in `GetPeerInfo` that exposed internal `PeerPubKey` slice to callers.
- **consensus/validators.go**: Fixed `GetByIndex` and `GetByAddress` returning internal pointers instead of deep copies, allowing callers to corrupt validator set state.
- **pex/address_book.go**: Fixed temp file not being cleaned up when atomic rename fails in `Save()`.
- **p2p/peer_state.go**: Fixed `SetPublicKey` storing external reference without defensive copy.

### Changed

- Updated dependency versions:
  - `github.com/blockberries/glueberry`: v1.2.9 → v1.2.10
  - `github.com/blockberries/cramberry`: v1.5.3 → v1.5.5

## [0.1.3] - 2026-01-29

### Fixed

- **sync/statesync.go**: Fixed critical deadlock in `requestMissingChunks` where calling `failWithError` attempted to acquire an already-held mutex. Changed to call `transitionToFailed` directly.
- **statestore/snapshot.go**: Fixed resource leak where avlberry importer was not closed if `Commit()` failed.

### Removed

- **sync/statesync.go**: Removed unused `failWithError` function (now using `transitionToFailed` directly).

## [0.1.2] - 2026-01-29

### Security

- **statestore/snapshot.go**: Fixed critical memory exhaustion vulnerabilities in snapshot deserialization:
  - `decodeSnapshotMetadata`: Added size limits for hash (64B), app hash (64B), metadata (1MB), and chunks (100K max)
  - `decodeExportNode`: Added size limits for AVL+ keys (4KB) and values (10MB)
  - These fixes prevent denial-of-service attacks where malicious peers send crafted snapshots with extremely large length fields

## [0.1.1] - 2026-01-29

### Security

- **handlers/consensus.go**: Fixed potential out-of-bounds slice access in `decodeVote` and `decodeCommit` functions. Hash length is now validated to be at most 64 bytes and data length is verified before slicing.

### Fixed

- **statestore/snapshot.go**: Fixed resource leak where gzReader was never closed in the `Import` function.

### Changed

- Updated ARCHITECTURE.md dependency versions to match actual versions (glueberry v1.2.9, cramberry v1.5.3)

## [0.1.0] - 2026-01-29

### Added

#### Core Components
- **Node Coordinator**: Full lifecycle management with graceful shutdown
- **Mempool**: Three implementations with configurable limits
  - `SimpleMempool`: Hash-based with FIFO ordering
  - `PriorityMempool`: Heap-based ordering with configurable priority function
  - `TTLMempool`: Automatic transaction expiration with background cleanup
- **Block Store**: LevelDB-based persistent storage with memory option for testing
- **State Store**: AVL+ merkleized storage with ICS23 proof generation and verification

#### P2P Networking
- Built on Glueberry with encrypted streams and automatic reconnection
- Two-phase handshake with chain/version validation
- Peer Exchange (PEX) protocol with address book persistence
- Transaction gossiping with deduplication
- Block synchronization (batch catch-up)
- Real-time block propagation
- Pluggable consensus message routing
- Per-peer rate limiting with token bucket algorithm
- LRU-bounded peer state tracking (prevents memory leaks)

#### Observability
- **Prometheus Metrics**: Comprehensive metrics for peers, blocks, transactions, sync, and messages
  - `PrometheusMetrics`: Full implementation with GaugeVec, CounterVec, Histograms
  - `NopMetrics`: Zero-overhead implementation when metrics disabled
- **Structured Logging**: Production-ready logging via Go's slog package
  - Factory functions: `NewTextLogger`, `NewJSONLogger`, `NewProductionLogger`, `NewDevelopmentLogger`
  - Blockchain-specific attribute constructors (Height, Hash, PeerID, etc.)
- Per-peer latency tracking

#### Safety Features
- Per-peer rate limiting (configurable messages/second per stream)
- Peer scoring with penalty decay
- Comprehensive input validation (heights, batch sizes, message sizes)
- ICS23 proof verification for state proofs

#### Configuration
- TOML-based configuration with comprehensive validation
- Sections: node, network, pex, mempool, blockstore, statestore, housekeeping, metrics, logging
- Sensible defaults for all options

#### Documentation
- Comprehensive README with quick start guide
- Architecture documentation (ARCHITECTURE.md)
- Development roadmap (ROADMAP.md)
- API reference (docs/API.md)
- Configuration reference (docs/CONFIGURATION.md)
- Integration guide (docs/INTEGRATION.md)
- Getting started guide (docs/GETTING_STARTED.md)
- Contributing guide (docs/CONTRIBUTING.md)
- Example implementations (simple_node, custom_mempool, mock_consensus)

### Security
- Chain ID and protocol version validation prevents cross-chain attacks
- Peer blacklisting for protocol violations
- Rate limiting prevents DoS attacks
- Input validation on all protocol messages
- Bounded memory usage with LRU caches

### Dependencies
- `github.com/blockberries/glueberry` v1.0.1 - P2P networking
- `github.com/blockberries/cramberry` v1.2.0 - Binary serialization
- `github.com/blockberries/avlberry` - Merkleized KV store
- `github.com/cosmos/ics23/go` v0.10.0 - Proof verification
- `github.com/prometheus/client_golang` v1.22.0 - Metrics
- `github.com/hashicorp/golang-lru/v2` v2.0.7 - LRU caches
- `github.com/syndtr/goleveldb` - Block storage

---

## Version History

### Pre-release Development

The following phases were completed during pre-release development:

#### Phase 1: Critical Fixes
- Implemented full ICS23 proof verification using `ics23.VerifyMembership()` and `ics23.VerifyNonMembership()`
- Fixed outbound connection detection using Glueberry's `IsOutbound` API
- Added peer limit checking before handshake completion

#### Phase 2: Memory & Safety
- Replaced unbounded maps with LRU caches in peer state tracking
  - `MaxKnownTxsPerPeer = 20000` entries per peer
  - `MaxKnownBlocksPerPeer = 2000` entries per peer
- Added comprehensive input validation functions
- Added error wrapping helpers for better context

#### Phase 3: Integration Hardening
- Pinned dependency versions (cramberry v1.2.0, glueberry v1.0.1)
- Added message validation after unmarshal in handlers
- Added contextual error wrapping for traceability

#### Phase 4: Observability
- Created metrics package with Prometheus implementation
- Created logging package with slog wrapper
- Added blockchain-specific attribute constructors

#### Phase 5: Features & Polish
- Implemented PriorityMempool with configurable ordering
- Implemented TTLMempool with automatic expiration
- Implemented per-peer, per-stream rate limiting

---

## Upgrade Guide

### From Development to 0.1.0

This is the first tagged release (pre-release). If upgrading from development versions:

1. **Configuration Changes**:
   - Add `[metrics]` section if using Prometheus
   - Add `[logging]` section for structured logging
   - Add `[ratelimit]` section if customizing rate limits

2. **API Changes**:
   - `Proof.Verify(rootHash)` now returns `(bool, error)` instead of `bool`
   - Peer state methods use LRU caches (behavior unchanged, but bounded memory)

3. **New Features Available**:
   - Use `PriorityMempool` for fee-based transaction ordering
   - Use `TTLMempool` for automatic transaction expiration
   - Enable metrics with `cfg.Metrics.Enabled = true`
