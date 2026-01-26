# Integration Guide

This guide explains how to integrate blockberry into your blockchain application.

## Overview

Blockberry provides the networking, storage, and peer management layers. Your application provides:

1. **Consensus Algorithm**: How blocks are proposed and agreed upon
2. **Transaction Validation**: What makes a transaction valid
3. **State Execution**: How transactions modify application state
4. **RPC Interface**: How users interact with your chain (optional)

## Application Interface

The full `Application` interface is defined in `types/application.go`:

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

### CheckTx

Called when a transaction is submitted to the mempool. Return an error to reject the transaction.

```go
func (app *MyApp) CheckTx(tx []byte) error {
    // Parse and validate transaction
    parsedTx, err := parseTx(tx)
    if err != nil {
        return fmt.Errorf("invalid tx format: %w", err)
    }

    // Check signature
    if !parsedTx.VerifySignature() {
        return errors.New("invalid signature")
    }

    // Check balance (optional, for fee-based chains)
    if !app.hasBalance(parsedTx.Sender, parsedTx.Fee) {
        return errors.New("insufficient balance")
    }

    return nil
}
```

### Block Execution Methods

Called during block commit:

```go
func (app *MyApp) BeginBlock(height int64, hash []byte) error {
    // Initialize block execution context
    app.currentHeight = height
    app.currentHash = hash
    app.pendingStateChanges = make(map[string][]byte)
    return nil
}

func (app *MyApp) DeliverTx(tx []byte) error {
    // Execute transaction and accumulate state changes
    parsedTx, _ := parseTx(tx)

    // Apply transaction logic
    if err := app.applyTx(parsedTx); err != nil {
        return err
    }

    return nil
}

func (app *MyApp) EndBlock() error {
    // Finalize block (validator updates, rewards, etc.)
    return nil
}

func (app *MyApp) Commit() ([]byte, error) {
    // Persist all state changes
    for key, value := range app.pendingStateChanges {
        app.stateStore.Set([]byte(key), value)
    }

    // Commit to IAVL and get app hash
    hash, _, err := app.stateStore.Commit()
    return hash, err
}
```

### Consensus Handler

Handles consensus protocol messages:

```go
func (app *MyApp) HandleConsensusMessage(peerID peer.ID, data []byte) error {
    // Parse consensus message
    msg, err := parseConsensusMessage(data)
    if err != nil {
        return err
    }

    switch m := msg.(type) {
    case *Proposal:
        return app.handleProposal(peerID, m)
    case *Vote:
        return app.handleVote(peerID, m)
    case *Commit:
        return app.handleCommit(peerID, m)
    }

    return nil
}
```

## Building a Complete Application

### Step 1: Define Your Transaction Format

```go
type Transaction struct {
    Type      string
    Sender    []byte
    Recipient []byte
    Amount    uint64
    Nonce     uint64
    Signature []byte
}

func (tx *Transaction) Marshal() ([]byte, error) {
    // Serialize transaction
}

func ParseTransaction(data []byte) (*Transaction, error) {
    // Deserialize transaction
}
```

### Step 2: Implement Application Logic

```go
type MyApp struct {
    node       *node.Node
    stateStore statestore.StateStore

    currentHeight int64
    currentHash   []byte
}

func NewMyApp(n *node.Node, ss statestore.StateStore) *MyApp {
    return &MyApp{
        node:       n,
        stateStore: ss,
    }
}

// Implement all Application interface methods...
```

### Step 3: Implement Consensus

For a simple round-robin leader selection:

```go
type SimpleConsensus struct {
    app        *MyApp
    validators [][]byte
    round      int
}

func (c *SimpleConsensus) IsLeader() bool {
    leaderIdx := c.round % len(c.validators)
    return bytes.Equal(c.validators[leaderIdx], c.app.node.PublicKey())
}

func (c *SimpleConsensus) ProposeBlock() error {
    if !c.IsLeader() {
        return nil
    }

    // Reap transactions from mempool
    txs := c.app.node.Mempool().ReapTxs(1_000_000) // 1MB max

    // Create block
    block := &Block{
        Height: c.app.currentHeight + 1,
        Txs:    txs,
    }

    // Broadcast proposal
    data, _ := block.Marshal()
    return c.app.node.Network().Broadcast(p2p.StreamConsensus, data)
}
```

### Step 4: Wire Everything Together

```go
func main() {
    cfg := config.DefaultConfig()
    cfg.Node.ChainID = "mychain-1"

    // Create application with consensus handler
    app := &MyApp{}

    n, err := node.NewNode(cfg, node.WithConsensusHandler(app))
    if err != nil {
        log.Fatal(err)
    }

    app.node = n
    app.stateStore = statestore.NewIAVLStore("data/state", 10000)

    if err := n.Start(); err != nil {
        log.Fatal(err)
    }

    // Start consensus loop
    go app.consensusLoop()

    // Wait for shutdown
    select {}
}
```

## Mempool Options

Blockberry provides three mempool implementations out of the box:

### Simple Mempool (Default)

Basic hash-based mempool with FIFO ordering:

```go
mp := mempool.NewSimpleMempool(mempool.SimpleMempoolConfig{
    MaxTxs:   5000,
    MaxBytes: 1024 * 1024 * 1024, // 1GB
})
n, err := node.NewNode(cfg, node.WithMempool(mp))
```

### Priority Mempool

Priority-based ordering with configurable priority function:

```go
// Custom priority function (e.g., by gas price)
priorityFunc := func(tx []byte) int64 {
    parsed, _ := ParseTransaction(tx)
    return int64(parsed.GasPrice)
}

mp := mempool.NewPriorityMempool(mempool.PriorityMempoolConfig{
    MaxTxs:       5000,
    MaxBytes:     1024 * 1024 * 1024,
    PriorityFunc: priorityFunc,
})
n, err := node.NewNode(cfg, node.WithMempool(mp))
```

Features:
- Max-heap ordering for O(1) highest priority access
- Automatic eviction of lowest priority when full
- Built-in functions: `DefaultPriorityFunc`, `SizePriorityFunc`

### TTL Mempool

Priority mempool with automatic transaction expiration:

```go
mp := mempool.NewTTLMempool(mempool.TTLMempoolConfig{
    MaxTxs:          5000,
    MaxBytes:        1024 * 1024 * 1024,
    TTL:             30 * time.Minute,     // Default TTL
    CleanupInterval: time.Minute,           // Cleanup frequency
    PriorityFunc:    priorityFunc,
})
defer mp.Stop() // Stop cleanup goroutine on shutdown

n, err := node.NewNode(cfg, node.WithMempool(mp))
```

Additional methods:
```go
// Add with custom TTL
mp.AddTxWithTTL(tx, 5*time.Minute)

// Check/modify TTL
remaining := mp.GetTTL(hash)
mp.ExtendTTL(hash, 10*time.Minute)
mp.SetTTL(hash, time.Now().Add(time.Hour))

// Count non-expired transactions
activeCount := mp.SizeActive()
```

### Custom Mempool

Implement `mempool.Mempool` for fully custom behavior:

```go
type MyMempool struct {
    // your fields
}

func (m *MyMempool) AddTx(tx []byte) error { /* ... */ }
func (m *MyMempool) RemoveTxs(hashes [][]byte) { /* ... */ }
func (m *MyMempool) ReapTxs(maxBytes int64) [][]byte { /* ... */ }
func (m *MyMempool) HasTx(hash []byte) bool { /* ... */ }
func (m *MyMempool) GetTx(hash []byte) ([]byte, error) { /* ... */ }
func (m *MyMempool) Size() int { /* ... */ }
func (m *MyMempool) SizeBytes() int64 { /* ... */ }
func (m *MyMempool) Flush() { /* ... */ }
func (m *MyMempool) TxHashes() [][]byte { /* ... */ }

mp := &MyMempool{}
n, err := node.NewNode(cfg, node.WithMempool(mp))
```

## Sending Consensus Messages

```go
// Send to specific peer
err := n.Network().Send(peerID, p2p.StreamConsensus, data)

// Broadcast to all peers
err := n.Network().Broadcast(p2p.StreamConsensus, data)
```

## Accessing Peer Information

```go
// Get connected peers
peers := n.Network().PeerManager().GetConnectedPeers()

// Get peer count
count := n.PeerCount()

// Check peer latency
state := n.Network().PeerManager().GetPeer(peerID)
if state != nil {
    latency := state.Latency
}
```

## Block Storage

```go
// Save a committed block
err := n.BlockStore().SaveBlock(height, hash, data)

// Load block
hash, data, err := n.BlockStore().LoadBlock(height)

// Check latest height
height := n.BlockStore().Height()
```

## State Storage

Use the IAVL store for merkleized state:

```go
import "github.com/blockberries/blockberry/statestore"

store, err := statestore.NewIAVLStore("data/state", 10000)

// Set values
store.Set([]byte("account:alice"), balanceBytes)

// Get values
value, err := store.Get([]byte("account:alice"))

// Commit changes
hash, version, err := store.Commit()

// Generate proofs
proof, err := store.GetProof([]byte("account:alice"))
```

## Error Handling

Common errors to handle:

```go
import "github.com/blockberries/blockberry/types"

switch {
case errors.Is(err, types.ErrMempoolFull):
    // Mempool is at capacity
case errors.Is(err, types.ErrInvalidTx):
    // Transaction validation failed
case errors.Is(err, types.ErrBlockNotFound):
    // Requested block doesn't exist
case errors.Is(err, types.ErrPeerNotFound):
    // Peer not connected
}
```

## Observability

### Metrics

Enable Prometheus metrics for production monitoring:

```go
import "github.com/blockberries/blockberry/metrics"

// Create metrics
m := metrics.NewPrometheusMetrics("mychain")

// Use with node
n, err := node.NewNode(cfg, node.WithMetrics(m))

// Expose /metrics endpoint (typically on a separate port)
http.Handle("/metrics", m.Handler().(http.Handler))
go http.ListenAndServe(":9090", nil)
```

Available metrics include:
- Peer counts by direction (inbound/outbound)
- Block height and sync progress
- Mempool size and bytes
- Transaction counts (received, rejected, evicted)
- Message counts by stream
- Latency histograms

### Logging

Use structured logging for production:

```go
import "github.com/blockberries/blockberry/logging"

// Production: JSON format, INFO level
logger := logging.NewProductionLogger()

// Development: Text format, DEBUG level
logger := logging.NewDevelopmentLogger()

// Custom configuration
logger := logging.NewJSONLogger(os.Stdout, slog.LevelInfo)

// Use with node
n, err := node.NewNode(cfg, node.WithLogger(logger))
```

Logging with context:

```go
logger.Info("block received",
    logging.Height(block.Height),
    logging.Hash(block.Hash()),
    logging.PeerID(peerID),
    logging.Size(len(block.Data)),
)

// Create component-specific logger
txLogger := logger.WithComponent("transactions")
txLogger.Debug("processing transaction", logging.TxHash(hash))
```

## Testing

Use the testing helpers for integration tests:

```go
import "github.com/blockberries/blockberry/testing"

func TestMyApp(t *testing.T) {
    // Create test node with ephemeral ports
    cfg := testing.DefaultTestNodeConfig()
    node1, err := testing.NewTestNode(cfg)
    require.NoError(t, err)
    defer node1.Stop()

    require.NoError(t, node1.Start())

    // Connect two nodes
    cfg2 := testing.DefaultTestNodeConfig()
    node2, err := testing.NewTestNode(cfg2)
    require.NoError(t, err)
    defer node2.Stop()

    require.NoError(t, node2.Start())

    // Connect node2 to node1
    addr, _ := node1.Multiaddr()
    err = node2.Connect(addr)
    require.NoError(t, err)

    // Wait for connection
    testing.WaitForPeers(t, node1, 1, 5*time.Second)
}
```
