# Blockberry

A modular blockchain node framework in Go. Blockberry provides the foundational infrastructure for building blockchain nodes while leaving consensus algorithm implementation to the integrating application.

## Features

- **Consensus Agnostic**: Plug in any consensus algorithm (PoW, PoS, BFT, etc.)
- **Modular Architecture**: Swappable components for mempool, block storage, and state management
- **P2P Networking**: Built on [glueberry](https://github.com/blockberries/glueberry) with encrypted streams and automatic reconnection
- **Efficient Serialization**: Uses [cramberry](https://github.com/blockberries/cramberry) for compact binary encoding
- **Merkle State Store**: IAVL-based state storage with historical queries and ICS23 proofs
- **Peer Discovery**: PEX (Peer Exchange) protocol for decentralized peer discovery
- **Block Synchronization**: Efficient catch-up sync from peers
- **Priority Mempool**: Transaction ordering by configurable priority with automatic eviction
- **Transaction TTL**: Automatic expiration of stale transactions
- **Rate Limiting**: Per-peer, per-stream rate limiting with token bucket algorithm
- **Prometheus Metrics**: Comprehensive metrics for monitoring peers, blocks, transactions, and sync state
- **Structured Logging**: Production-ready logging with slog integration

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        APPLICATION LAYER                             │
│              (Consensus Engine, Business Logic, RPC)                 │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                            BLOCKBERRY                                │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────┐ │
│  │   Node    │  │  Mempool  │  │BlockStore │  │    StateStore     │ │
│  │Coordinator│  │ Interface │  │ Interface │  │   (IAVL-based)    │ │
│  └───────────┘  └───────────┘  └───────────┘  └───────────────────┘ │
│  ┌───────────┐  ┌───────────┐  ┌───────────┐  ┌───────────────────┐ │
│  │   PEX     │  │ BlockSync │  │ Consensus │  │    Housekeeping   │ │
│  │  Reactor  │  │  Reactor  │  │  Reactor  │  │      Reactor      │ │
│  └───────────┘  └───────────┘  └───────────┘  └───────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                            GLUEBERRY                                 │
│            (P2P Networking, Encrypted Streams, Reconnection)         │
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                              LIBP2P                                  │
│                (Transport, Multiplexing, NAT Traversal)              │
└─────────────────────────────────────────────────────────────────────┘
```

## Installation

```bash
go get github.com/blockberries/blockberry
```

## Quick Start

### Minimal Node Setup

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/blockberries/blockberry/config"
    "github.com/blockberries/blockberry/node"
)

func main() {
    // Load or create default configuration
    cfg := config.DefaultConfig()
    cfg.Node.ChainID = "mychain-1"
    cfg.Network.ListenAddrs = []string{"/ip4/0.0.0.0/tcp/26656"}

    // Create the node
    n, err := node.NewNode(cfg)
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    // Start the node
    if err := n.Start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }
    log.Printf("Node started with ID: %s", n.NodeID())

    // Wait for interrupt signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    // Graceful shutdown
    if err := n.Stop(); err != nil {
        log.Printf("Error stopping node: %v", err)
    }
}
```

### With Custom Consensus

```go
package main

import (
    "log"

    "github.com/libp2p/go-libp2p/core/peer"

    "github.com/blockberries/blockberry/config"
    "github.com/blockberries/blockberry/handlers"
    "github.com/blockberries/blockberry/node"
)

// MyConsensus implements the ConsensusHandler interface
type MyConsensus struct {
    node *node.Node
}

func (c *MyConsensus) HandleConsensusMessage(peerID peer.ID, data []byte) error {
    // Process incoming consensus messages (votes, proposals, etc.)
    log.Printf("Received consensus message from %s", peerID)
    return nil
}

func main() {
    cfg := config.DefaultConfig()
    cfg.Node.ChainID = "mychain-1"

    // Create node with consensus handler
    consensus := &MyConsensus{}

    n, err := node.NewNode(cfg, node.WithConsensusHandler(consensus))
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    consensus.node = n

    if err := n.Start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }

    // Your consensus logic can now send messages via:
    // n.Network().Send(peerID, p2p.StreamConsensus, data)
    // n.Network().Broadcast(p2p.StreamConsensus, data)
}
```

## Configuration

Configuration is loaded from a TOML file. See `examples/config.example.toml` for a complete reference.

### Key Configuration Options

```toml
[node]
chain_id = "mychain-1"          # Unique chain identifier
protocol_version = 1             # Protocol version (must match peers)
private_key_path = "node_key.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
address_book_path = "addrbook.json"

[network.seeds]
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/12D3KooW...",
]

[pex]
enabled = true
request_interval = "30s"
max_addresses_per_response = 100

[mempool]
max_txs = 5000
max_bytes = 1073741824  # 1GB

[blockstore]
path = "data/blocks"

[statestore]
path = "data/state"
cache_size = 10000

[metrics]
enabled = true
namespace = "blockberry"
listen_addr = ":9090"

[logging]
level = "info"       # debug, info, warn, error
format = "json"      # text or json
output = "stdout"    # stdout, stderr, or file path
```

## Core Components

### Node

The `Node` is the main coordinator that orchestrates all components:

```go
// Create with options
n, err := node.NewNode(cfg,
    node.WithMempool(customMempool),
    node.WithBlockStore(customBlockStore),
    node.WithConsensusHandler(myConsensus),
)

// Lifecycle
n.Start()
n.Stop()
n.IsRunning()

// Accessors
n.PeerID()     // libp2p peer ID
n.NodeID()     // hex-encoded public key
n.Network()    // P2P network layer
n.BlockStore() // Block storage
n.Mempool()    // Transaction pool
n.PeerCount()  // Connected peers
```

### Mempool

Transaction pool with configurable limits. Multiple implementations available:

```go
type Mempool interface {
    AddTx(tx []byte) error              // Add transaction
    RemoveTxs(hashes [][]byte)          // Remove by hash
    ReapTxs(maxBytes int64) [][]byte    // Get txs for block
    HasTx(hash []byte) bool             // Check existence
    GetTx(hash []byte) ([]byte, error)  // Retrieve by hash
    Size() int                          // Transaction count
    SizeBytes() int64                   // Total size
    Flush()                             // Clear all
}
```

**Implementations:**
- `SimpleMempool`: Basic hash-based mempool with FIFO ordering
- `PriorityMempool`: Priority-based ordering with configurable priority function and automatic eviction
- `TTLMempool`: Priority mempool with automatic transaction expiration

### Block Store

Persistent block storage:

```go
type BlockStore interface {
    SaveBlock(height int64, hash []byte, data []byte) error
    LoadBlock(height int64) (hash []byte, data []byte, error)
    LoadBlockByHash(hash []byte) (height int64, data []byte, error)
    HasBlock(height int64) bool
    Height() int64  // Latest height
    Base() int64    // Earliest height (for pruned stores)
    Close() error
}
```

### State Store

IAVL-based merkleized key-value store:

```go
type StateStore interface {
    Get(key []byte) ([]byte, error)
    Has(key []byte) (bool, error)
    Set(key []byte, value []byte) error
    Delete(key []byte) error
    Commit() (hash []byte, version int64, error)
    RootHash() []byte
    Version() int64
    LoadVersion(version int64) error
    GetProof(key []byte) (*Proof, error)
    Close() error
}
```

## Network Streams

Blockberry uses multiplexed encrypted streams for different message types:

| Stream | Purpose |
|--------|---------|
| `handshake` | Connection establishment |
| `pex` | Peer exchange |
| `transactions` | Transaction gossiping |
| `blocksync` | Historical block synchronization |
| `blocks` | Real-time block propagation |
| `consensus` | Application consensus messages |
| `housekeeping` | Latency probes |

## Application Interface

For full application integration, implement the `Application` interface:

```go
type Application interface {
    // Transaction validation for mempool
    CheckTx(tx []byte) error

    // Block execution
    BeginBlock(height int64, hash []byte) error
    DeliverTx(tx []byte) error
    EndBlock() error
    Commit() (appHash []byte, error)

    // State queries
    Query(path string, data []byte) ([]byte, error)

    // Consensus handling
    HandleConsensusMessage(peerID peer.ID, data []byte) error
}
```

## Examples

See the `examples/` directory for complete examples:

- **simple_node**: Minimal node setup
- **custom_mempool**: Priority mempool with fee ordering
- **mock_consensus**: Mock consensus handler for testing

## Development

### Building

```bash
make build
```

### Testing

```bash
# Unit tests (fast)
make test

# Integration tests
make test-integration

# With race detection
make test-race
```

### Linting

```bash
make lint
```

### Code Generation

```bash
make generate
```

## Package Structure

```
github.com/blockberries/blockberry/
├── blockstore/    # Block storage implementations
├── config/        # Configuration loading and validation
├── handlers/      # Message handlers (handshake, transactions, blocks, consensus)
├── logging/       # Structured logging with slog integration
├── mempool/       # Transaction pool (simple, priority, TTL implementations)
├── metrics/       # Prometheus metrics collection
├── node/          # Main Node coordinator
├── p2p/           # Peer management, scoring, and rate limiting
├── pex/           # Peer exchange protocol
├── schema/        # Generated cramberry message types
├── statestore/    # IAVL state storage with ICS23 proofs
├── sync/          # Block synchronization
├── testing/       # Integration test helpers
└── types/         # Common types, errors, and validation
```

## Dependencies

| Package | Version | Description |
|---------|---------|-------------|
| [glueberry](https://github.com/blockberries/glueberry) | v1.0.1 | P2P networking layer |
| [cramberry](https://github.com/blockberries/cramberry) | v1.2.0 | Binary serialization |
| [cosmos/iavl](https://github.com/cosmos/iavl) | v1.3.5 | Merkleized key-value store |
| [cosmos/ics23](https://github.com/cosmos/ics23) | v0.10.0 | ICS23 proof verification |
| [libp2p](https://github.com/libp2p/go-libp2p) | v0.46.0 | Peer-to-peer networking |
| [goleveldb](https://github.com/syndtr/goleveldb) | latest | Block storage backend |
| [prometheus/client_golang](https://github.com/prometheus/client_golang) | v1.22.0 | Metrics collection |
| [hashicorp/golang-lru](https://github.com/hashicorp/golang-lru) | v2.0.7 | LRU cache for peer state |

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the development roadmap including:

- **Phase 1**: Storage & Sync Enhancements (BadgerDB, state pruning, state sync)
- **Phase 2**: Consensus Framework (interface refinement, reference BFT implementation)
- **Phase 3**: Networking Improvements (firewall detection, peer reputation persistence)
- **Phase 4**: Developer Experience (transaction indexing, event subscriptions, RPC API, CLI)
- **Phase 5**: Light Client Support
- **Phase 6**: Performance Optimization
- **Phase 7**: Security Hardening
- **Phase 8**: Observability Enhancements
- **Phase 9**: Ecosystem Integration (IBC, ABCI compatibility)

## License

MIT License - see LICENSE file for details.
