# Getting Started with Blockberry

This guide walks you through setting up your first blockberry node.

## Prerequisites

- Go 1.21 or later
- A working knowledge of Go programming

## Installation

```bash
go get github.com/blockberries/blockberry
```

## Creating Your First Node

### Step 1: Create a Configuration

```go
package main

import (
    "github.com/blockberries/blockberry/config"
)

func main() {
    // Start with default configuration
    cfg := config.DefaultConfig()

    // Customize for your network
    cfg.Node.ChainID = "mychain-1"
    cfg.Node.ProtocolVersion = 1
    cfg.Node.PrivateKeyPath = "node_key.json"

    // Set network settings
    cfg.Network.ListenAddrs = []string{"/ip4/0.0.0.0/tcp/26656"}
    cfg.Network.MaxInboundPeers = 40
    cfg.Network.MaxOutboundPeers = 10

    // Add seed nodes (if any)
    cfg.Network.Seeds.Addrs = []string{
        "/ip4/seed.mychain.io/tcp/26656/p2p/12D3KooW...",
    }
}
```

Alternatively, load from a TOML file:

```go
cfg, err := config.LoadConfig("config.toml")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
```

### Step 2: Create and Start the Node

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
    cfg := config.DefaultConfig()
    cfg.Node.ChainID = "mychain-1"

    // Create the node
    n, err := node.NewNode(cfg)
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    // Start the node
    if err := n.Start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }

    log.Printf("Node started!")
    log.Printf("  Node ID: %s", n.NodeID())
    log.Printf("  Peer ID: %s", n.PeerID())

    // Handle shutdown gracefully
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    log.Println("Shutting down...")
    if err := n.Stop(); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }
}
```

### Step 3: Connecting to Peers

Nodes discover each other through:

1. **Seed Nodes**: Configure in `config.Network.Seeds.Addrs`
2. **PEX (Peer Exchange)**: Nodes share peer addresses automatically
3. **Manual Connection**: Use `n.Network().ConnectMultiaddr(addr)`

## Submitting Transactions

Once your node is running, submit transactions to the mempool:

```go
// Add a transaction
tx := []byte("my transaction data")
if err := n.Mempool().AddTx(tx); err != nil {
    log.Printf("Failed to add tx: %v", err)
}

// Transactions will be gossiped to peers automatically
```

## Accessing Blocks

Read blocks from the block store:

```go
// Get the latest height
height := n.BlockStore().Height()

// Load a block
hash, data, err := n.BlockStore().LoadBlock(height)
if err != nil {
    log.Printf("Block not found: %v", err)
}
```

## Implementing Consensus

Blockberry is consensus-agnostic. To implement your own consensus:

1. Create a type that implements `handlers.ConsensusHandler`
2. Pass it when creating the node

```go
type MyConsensus struct {
    node *node.Node
}

func (c *MyConsensus) HandleConsensusMessage(peerID peer.ID, data []byte) error {
    // Handle incoming consensus messages
    return nil
}

// When creating the node:
consensus := &MyConsensus{}
n, err := node.NewNode(cfg, node.WithConsensusHandler(consensus))
consensus.node = n
```

See `examples/mock_consensus/` for a complete example.

## Next Steps

- Read [CONFIGURATION.md](CONFIGURATION.md) for all configuration options
- Read [INTEGRATION.md](INTEGRATION.md) for building full applications
- Explore the `examples/` directory for working code
- Check [ARCHITECTURE.md](../ARCHITECTURE.md) for design details
