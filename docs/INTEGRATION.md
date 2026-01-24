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

## Custom Mempool

Implement `mempool.Mempool` for custom transaction ordering:

```go
type PriorityMempool struct {
    txs      map[string]*TxWithPriority
    mu       sync.RWMutex
}

type TxWithPriority struct {
    Data     []byte
    Fee      uint64
    Priority int
}

func (m *PriorityMempool) AddTx(tx []byte) error {
    // Parse and extract priority
    parsed, _ := ParseTransaction(tx)
    hash := types.HashTx(tx)

    m.mu.Lock()
    m.txs[string(hash)] = &TxWithPriority{
        Data:     tx,
        Fee:      parsed.Fee,
        Priority: calculatePriority(parsed),
    }
    m.mu.Unlock()
    return nil
}

func (m *PriorityMempool) ReapTxs(maxBytes int64) [][]byte {
    m.mu.RLock()
    defer m.mu.RUnlock()

    // Sort by priority and return highest priority transactions
    sorted := m.sortByPriority()

    var result [][]byte
    var totalSize int64
    for _, tx := range sorted {
        if totalSize+int64(len(tx.Data)) > maxBytes {
            break
        }
        result = append(result, tx.Data)
        totalSize += int64(len(tx.Data))
    }
    return result
}

// Implement remaining interface methods...
```

Use with node:

```go
mp := NewPriorityMempool()
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
