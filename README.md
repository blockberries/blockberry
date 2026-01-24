# Blockberry

A modular blockchain node framework in Go. Blockberry provides the foundational infrastructure for building blockchain nodes while leaving consensus algorithm implementation to the integrating application.

## Features

- **Consensus Agnostic**: Plug in any consensus algorithm (PoW, PoS, BFT, etc.)
- **Modular Architecture**: Swappable components for mempool, block storage, and state management
- **P2P Networking**: Built on [glueberry](https://github.com/blockberries/glueberry) with encrypted streams and automatic reconnection
- **Efficient Serialization**: Uses [cramberry](https://github.com/blockberries/cramberry) for compact binary encoding
- **Merkle State Store**: IAVL-based state storage with historical queries and proofs
- **Peer Discovery**: PEX (Peer Exchange) protocol for decentralized peer discovery
- **Block Synchronization**: Efficient catch-up sync from peers

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

Transaction pool with configurable limits:

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
├── mempool/       # Transaction pool
├── node/          # Main Node coordinator
├── p2p/           # Peer management and scoring
├── pex/           # Peer exchange protocol
├── schema/        # Generated cramberry message types
├── statestore/    # IAVL state storage
├── sync/          # Block synchronization
├── testing/       # Integration test helpers
└── types/         # Common types and errors
```

## Dependencies

- [glueberry](https://github.com/blockberries/glueberry) - P2P networking layer
- [cramberry](https://github.com/blockberries/cramberry) - Binary serialization
- [cosmos/iavl](https://github.com/cosmos/iavl) - Merkleized key-value store
- [libp2p](https://github.com/libp2p/go-libp2p) - Peer-to-peer networking
- [goleveldb](https://github.com/syndtr/goleveldb) - Block storage backend

## License

MIT License - see LICENSE file for details.
