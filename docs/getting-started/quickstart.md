# Quickstart Guide

Get a Blockberry node running in 5 minutes with this quick start guide.

## Prerequisites

- Go 1.25.6 or higher installed
- Basic command-line familiarity
- 15 minutes of time

Haven't installed Blockberry yet? See the [installation guide](installation.md).

## Step 1: Create a New Project

```bash

# Create a new directory for your blockchain
mkdir my-blockchain
cd my-blockchain

# Initialize a Go module
go mod init github.com/yourusername/my-blockchain

# Add Blockberry as a dependency
go get github.com/blockberries/blockberry@latest

```text

## Step 2: Initialize Node Configuration

If you have the `blockberry` CLI installed:

```bash

# Initialize configuration
blockberry init --chain-id my-testnet

# This creates:
# - config.toml (configuration file)
# - node_key.json (node private key)
# - data/ directory (for blocks and state)

```text

Or create a minimal `config.toml` manually:

```toml
[node]
chain_id = "my-testnet"
protocol_version = 1
private_key_path = "node_key.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10

[mempool]
type = "simple"
max_txs = 5000
max_bytes = 1073741824  # 1GB

[blockstore]
backend = "leveldb"
path = "data/blockstore"

[statestore]
path = "data/state"
cache_size = 10000

[logging]
level = "info"
format = "text"
output = "stderr"

```text

## Step 3: Create Your Application

Create `main.go` with a simple application:

```go
package main

import (
    "context"
    "errors"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/blockberries/bapi"
    bapitypes "github.com/blockberries/bapi/types"
    "github.com/blockberries/blockberry/pkg/config"
    "github.com/blockberries/blockberry/pkg/node"
)

// SimpleApp implements the bapi.Lifecycle interface
type SimpleApp struct {
    bapi.BaseApplication
    txCount int
}

// Info returns application metadata
func (app *SimpleApp) Info() bapitypes.ApplicationInfo {
    return bapitypes.ApplicationInfo{
        Name:    "simple-blockchain",
        Version: "1.0.0",
    }
}

// CheckTx validates a transaction before adding to mempool
func (app *SimpleApp) CheckTx(ctx context.Context, tx *bapitypes.Transaction) *bapitypes.TxCheckResult {
    // Basic validation: transaction must not be empty
    if len(tx.Data) == 0 {
        return &bapitypes.TxCheckResult{
            Code:  bapitypes.CodeInvalidTx,
            Error: errors.New("empty transaction"),
        }
    }

    // Accept the transaction
    return &bapitypes.TxCheckResult{
        Code:      bapitypes.CodeOK,
        GasWanted: 1000,
    }
}

// ExecuteBlock processes all transactions in a block
func (app *SimpleApp) ExecuteBlock(ctx context.Context, block *bapitypes.Block) *bapitypes.BlockResult {
    app.txCount += len(block.Txs)

    // Emit an event for the block
    event := bapitypes.Event{
        Type: "block.executed",
        Attributes: map[string]string{
            "tx_count": string(rune(app.txCount)),
        },
    }

    return &bapitypes.BlockResult{
        Code:   bapitypes.CodeOK,
        Events: []bapitypes.Event{event},
    }
}

// Commit finalizes the block and returns the app hash
func (app *SimpleApp) Commit(ctx context.Context) *bapitypes.CommitResult {
    // In a real app, you would compute a proper state hash
    // For this example, we'll use a dummy hash
    dummyHash := []byte("simple-app-hash")

    return &bapitypes.CommitResult{
        AppHash: dummyHash,
    }
}

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("config.toml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create the application
    app := &SimpleApp{}

    // Create the node
    n, err := node.NewNode(cfg)
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    // Start the node
    log.Println("Starting Blockberry node...")
    if err := n.Start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }

    log.Printf("Node started successfully!")
    log.Printf("Node ID: %s", n.NodeID())
    log.Printf("Chain ID: %s", cfg.Node.ChainID)
    log.Printf("Listening on: %v", cfg.Network.ListenAddrs)

    // Wait for shutdown signal
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    // Graceful shutdown
    log.Println("Shutting down...")
    if err := n.Stop(); err != nil {
        log.Printf("Error during shutdown: %v", err)
    }
    log.Println("Shutdown complete")
}

```text

## Step 4: Run Your Node

```bash

# Ensure dependencies are downloaded
go mod tidy

# Run the node
go run main.go

```text

You should see output like:

```text
Starting Blockberry node...
Node started successfully!
Node ID: QmABC123...
Chain ID: my-testnet
Listening on: [/ip4/0.0.0.0/tcp/26656]

```text

Press `Ctrl+C` to stop the node gracefully.

## Step 5: Test Transaction Submission (Optional)

While your node is running, open another terminal and create a simple client:

```go
// client.go
package main

import (
    "fmt"
    "log"
)

func main() {
    // In a real application, you would:
    // 1. Connect to the node's RPC endpoint
    // 2. Create a transaction
    // 3. Submit it via BroadcastTx

    fmt.Println("Transaction submission requires RPC setup")
    fmt.Println("See the full application tutorial for details")
}

```text

## What Just Happened?

Congratulations! You just:

1. Created a new blockchain project
2. Defined a simple application that accepts transactions
3. Started a Blockberry node
4. The node is now:
   - Listening for peer connections
   - Ready to accept transactions
   - Processing blocks (when connected to other nodes)

## Understanding the Components

### The Application

The `SimpleApp` implements the `bapi.Lifecycle` interface, which defines:

- `Info()` - Returns app metadata
- `CheckTx()` - Validates transactions for the mempool
- `ExecuteBlock()` - Executes all transactions in a block
- `Commit()` - Finalizes the block and returns app state hash

### The Node

The `node.Node` coordinates all components:

- P2P networking (via Glueberry)
- Transaction mempool
- Block storage
- State storage (IAVL merkle tree)
- Message handlers (PEX, sync, etc.)

### The Configuration

`config.toml` controls all node behavior:

- Chain identity and protocol version
- Network settings (ports, peer limits)
- Mempool configuration
- Storage backends
- Logging and metrics

## Next Steps

### 1. Build a Real Application

The example above is minimal. For a production application:

- Implement proper state management (use `StateStore`)
- Add transaction validation logic
- Handle validator set updates
- Implement query handling
- Add event emission for indexing

See [Building Your First Application](first-application.md) for a complete guide.

### 2. Connect Multiple Nodes

To form a network:

```bash

# Node 1 (seed node)
blockberry init --chain-id my-testnet --node-id node1
blockberry start --config node1/config.toml

# Get the node1 address from logs:
# /ip4/127.0.0.1/tcp/26656/p2p/QmNode1ID...

# Node 2
blockberry init --chain-id my-testnet --node-id node2

# Edit node2/config.toml to add node1 as seed
blockberry start --config node2/config.toml

```text

### 3. Add Consensus

The minimal example doesn't include consensus. For block production:

- Implement a `ConsensusEngine` (see [Consensus Integration](../tutorials/consensus-integration.md))
- Or use a pre-built consensus like Tendermint BFT
- Configure validator set in genesis

### 4. Enable RPC

To allow external clients to interact with your blockchain:

```toml

# Add to config.toml
[rpc]
enabled = true
listen_addr = "127.0.0.1:26657"

```text

Then use the RPC API to submit transactions, query state, and subscribe to events.

### 5. Add Monitoring

Enable Prometheus metrics:

```toml
[metrics]
enabled = true
listen_addr = ":9090"

```text

Then view metrics at `http://localhost:9090/metrics`.

## Common Issues

### Port Already in Use

```text
Error: bind: address already in use

```text

Solution: Change the port in `config.toml`:

```toml
[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26657"]  # Use different port

```text

### Module Errors

```text
go: module requires Go 1.25.6

```text

Solution: Update Go to 1.25.6 or higher.

### Permission Denied

```text
Error: permission denied: data/blockstore

```text

Solution: Ensure data directories are writable:

```bash
mkdir -p data/{blockstore,state}
chmod -R 755 data/

```text

## Quick Reference

### Essential Commands

```bash

# Initialize node
blockberry init --chain-id <chain-id>

# Start node
blockberry start --config config.toml

# Check node status
blockberry status --config config.toml

# Show version
blockberry version

```text

### Essential Configuration

```toml
[node]
chain_id = "my-chain"          # Blockchain identifier
protocol_version = 1            # Protocol version

[network]
listen_addrs = [               # P2P listen addresses
    "/ip4/0.0.0.0/tcp/26656"
]

[mempool]
type = "simple"                 # Mempool type
max_txs = 5000                  # Max transactions

[logging]
level = "info"                  # Log level: debug/info/warn/error

```text

### Essential Application Methods

```go
// Minimal implementation
type App struct {
    bapi.BaseApplication
}

func (app *App) CheckTx(ctx context.Context, tx *bapitypes.Transaction) *bapitypes.TxCheckResult {
    // Validate transaction
    return &bapitypes.TxCheckResult{Code: bapitypes.CodeOK}
}

func (app *App) ExecuteBlock(ctx context.Context, block *bapitypes.Block) *bapitypes.BlockResult {
    // Execute block
    return &bapitypes.BlockResult{Code: bapitypes.CodeOK}
}

func (app *App) Commit(ctx context.Context) *bapitypes.CommitResult {
    // Return state hash
    return &bapitypes.CommitResult{AppHash: []byte("hash")}
}

```text

## Resources

- [First Application Tutorial](first-application.md) - Build a complete app
- [Configuration Guide](configuration.md) - All config options
- [Application Development](../guides/application-development.md) - In-depth ABI guide
- [API Reference](../../API_REFERENCE.md) - Complete API documentation
- [Examples](../../examples/) - More example applications

---

**Congratulations!** You now have a working Blockberry node. Next, learn to [build a complete application](first-application.md).
