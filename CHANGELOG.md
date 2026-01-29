# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-29

### Added

#### Core Components
- **Node Coordinator**: Full lifecycle management with graceful shutdown
- **Mempool**: Three implementations with configurable limits
  - `SimpleMempool`: Hash-based with FIFO ordering
  - `PriorityMempool`: Heap-based ordering with configurable priority function
  - `TTLMempool`: Automatic transaction expiration with background cleanup
- **Block Store**: LevelDB-based persistent storage with memory option for testing
- **State Store**: IAVL-based merkleized storage with ICS23 proof generation and verification

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
- `github.com/cosmos/iavl` v1.3.5 - Merkleized KV store
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
