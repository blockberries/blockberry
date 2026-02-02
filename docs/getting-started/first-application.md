# Building Your First Application

This tutorial guides you through building a complete blockchain application with Blockberry. You'll create a simple key-value store blockchain with account balances.

## What You'll Build

A blockchain that:

- Stores account balances in merkleized state
- Validates transactions with signatures
- Enforces balance rules (no overdrafts)
- Emits events for indexing
- Supports queries

## Prerequisites

- Blockberry installed ([installation guide](installation.md))
- Basic Go knowledge
- Understanding of blockchain concepts
- Completed the [quickstart](quickstart.md)

## Project Setup

```bash
mkdir kvstore-blockchain
cd kvstore-blockchain
go mod init github.com/yourusername/kvstore-blockchain
go get github.com/blockberries/blockberry@latest

```text

## Step 1: Define Transaction Format

Create `types.go`:

```go
package main

import (
    "crypto/ed25519"
    "crypto/sha256"
    "encoding/binary"
    "fmt"
)

// Transaction represents a value transfer
type KVTransaction struct {
    From   [32]byte // Sender public key
    To     [32]byte // Recipient public key
    Amount uint64   // Amount to transfer
    Nonce  uint64   // Sender's transaction count
    Sig    [64]byte // Ed25519 signature
}

// Serialize converts transaction to bytes
func (tx *KVTransaction) Serialize() []byte {
    buf := make([]byte, 160)
    copy(buf[0:32], tx.From[:])
    copy(buf[32:64], tx.To[:])
    binary.BigEndian.PutUint64(buf[64:72], tx.Amount)
    binary.BigEndian.PutUint64(buf[72:80], tx.Nonce)
    copy(buf[80:144], tx.Sig[:])
    return buf
}

// Deserialize parses bytes into transaction
func DeserializeTransaction(data []byte) (*KVTransaction, error) {
    if len(data) != 160 {
        return nil, fmt.Errorf("invalid transaction size")
    }

    tx := &KVTransaction{
        Amount: binary.BigEndian.Uint64(data[64:72]),
        Nonce:  binary.BigEndian.Uint64(data[72:80]),
    }
    copy(tx.From[:], data[0:32])
    copy(tx.To[:], data[32:64])
    copy(tx.Sig[:], data[80:144])

    return tx, nil
}

// Hash returns transaction hash
func (tx *KVTransaction) Hash() []byte {
    h := sha256.Sum256(tx.Serialize())
    return h[:]
}

// SigningBytes returns bytes to sign
func (tx *KVTransaction) SigningBytes() []byte {
    buf := make([]byte, 80)
    copy(buf[0:32], tx.From[:])
    copy(buf[32:64], tx.To[:])
    binary.BigEndian.PutUint64(buf[64:72], tx.Amount)
    binary.BigEndian.PutUint64(buf[72:80], tx.Nonce)
    return buf
}

// Verify checks the signature
func (tx *KVTransaction) Verify() bool {
    pubKey := ed25519.PublicKey(tx.From[:])
    msg := tx.SigningBytes()
    return ed25519.Verify(pubKey, msg, tx.Sig[:])
}

```text

## Step 2: Implement the Application

Create `app.go`:

```go
package main

import (
    "context"
    "encoding/binary"
    "errors"
    "fmt"
    "sync"

    "github.com/blockberries/blockberry/pkg/abi"
    "github.com/blockberries/blockberry/pkg/statestore"
)

type KVStoreApp struct {
    abi.BaseApplication

    state *statestore.IAVLStore
    mu    sync.RWMutex

    // In-memory cache for current block
    pendingBalances map[[32]byte]uint64
    pendingNonces   map[[32]byte]uint64
}

func NewKVStoreApp(statePath string) (*KVStoreApp, error) {
    // Open state store
    state, err := statestore.NewIAVLStore(statePath, 10000)
    if err != nil {
        return nil, fmt.Errorf("failed to open state: %w", err)
    }

    return &KVStoreApp{
        state:           state,
        pendingBalances: make(map[[32]byte]uint64),
        pendingNonces:   make(map[[32]byte]uint64),
    }, nil
}

// Info returns application metadata
func (app *KVStoreApp) Info() abi.ApplicationInfo {
    return abi.ApplicationInfo{
        Name:       "kvstore-blockchain",
        Version:    "1.0.0",
        AppHash:    app.state.RootHash(),
        LastHeight: app.state.Version(),
    }
}

// InitChain initializes genesis state
func (app *KVStoreApp) InitChain(genesis *abi.Genesis) error {
    app.mu.Lock()
    defer app.mu.Unlock()

    // Parse genesis accounts from AppState
    // Format: <pubkey>:<balance>,<pubkey>:<balance>,...
    // For simplicity, we'll skip parsing and just return nil

    return nil
}

// CheckTx validates transaction for mempool
func (app *KVStoreApp) CheckTx(ctx context.Context, tx *abi.Transaction) *abi.TxCheckResult {
    app.mu.RLock()
    defer app.mu.RUnlock()

    // Parse transaction
    kvTx, err := DeserializeTransaction(tx.Data)
    if err != nil {
        return &abi.TxCheckResult{
            Code:  abi.CodeInvalidTx,
            Error: fmt.Errorf("invalid transaction format: %w", err),
        }
    }

    // Verify signature
    if !kvTx.Verify() {
        return &abi.TxCheckResult{
            Code:  abi.CodeInvalidSignature,
            Error: errors.New("invalid signature"),
        }
    }

    // Check balance
    balance := app.getBalance(kvTx.From)
    if balance < kvTx.Amount {
        return &abi.TxCheckResult{
            Code:  abi.CodeInsufficientFunds,
            Error: fmt.Errorf("insufficient balance: have %d, need %d", balance, kvTx.Amount),
        }
    }

    // Check nonce
    expectedNonce := app.getNonce(kvTx.From)
    if kvTx.Nonce != expectedNonce {
        return &abi.TxCheckResult{
            Code:  abi.CodeInvalidNonce,
            Error: fmt.Errorf("invalid nonce: have %d, expected %d", kvTx.Nonce, expectedNonce),
        }
    }

    return &abi.TxCheckResult{
        Code:      abi.CodeOK,
        GasWanted: 1000,
        Priority:  int64(kvTx.Amount), // Higher value = higher priority
    }
}

// BeginBlock starts block processing
func (app *KVStoreApp) BeginBlock(ctx context.Context, header *abi.BlockHeader) error {
    app.mu.Lock()
    defer app.mu.Unlock()

    // Reset pending changes for new block
    app.pendingBalances = make(map[[32]byte]uint64)
    app.pendingNonces = make(map[[32]byte]uint64)

    return nil
}

// ExecuteTx processes a transaction
func (app *KVStoreApp) ExecuteTx(ctx context.Context, tx *abi.Transaction) *abi.TxExecResult {
    app.mu.Lock()
    defer app.mu.Unlock()

    // Parse transaction
    kvTx, err := DeserializeTransaction(tx.Data)
    if err != nil {
        return &abi.TxExecResult{
            Code:  abi.CodeInvalidTx,
            Error: fmt.Errorf("invalid transaction: %w", err),
        }
    }

    // Get current balances
    fromBalance := app.getBalanceWithPending(kvTx.From)
    toBalance := app.getBalanceWithPending(kvTx.To)

    // Update balances
    fromBalance -= kvTx.Amount
    toBalance += kvTx.Amount

    // Store pending changes
    app.pendingBalances[kvTx.From] = fromBalance
    app.pendingBalances[kvTx.To] = toBalance

    // Update nonce
    nonce := app.getNonceWithPending(kvTx.From)
    app.pendingNonces[kvTx.From] = nonce + 1

    // Emit events
    events := []abi.Event{
        {
            Type: "transfer",
            Attributes: map[string]string{
                "from":   fmt.Sprintf("%x", kvTx.From[:]),
                "to":     fmt.Sprintf("%x", kvTx.To[:]),
                "amount": fmt.Sprintf("%d", kvTx.Amount),
            },
        },
    }

    return &abi.TxExecResult{
        Code:    abi.CodeOK,
        Events:  events,
        GasUsed: 1000,
    }
}

// EndBlock finalizes block
func (app *KVStoreApp) EndBlock(ctx context.Context) *abi.EndBlockResult {
    return &abi.EndBlockResult{}
}

// Commit persists state changes
func (app *KVStoreApp) Commit(ctx context.Context) *abi.CommitResult {
    app.mu.Lock()
    defer app.mu.Unlock()

    // Write pending balances to state
    for pubKey, balance := range app.pendingBalances {
        key := append([]byte("balance:"), pubKey[:]...)
        value := make([]byte, 8)
        binary.BigEndian.PutUint64(value, balance)
        if err := app.state.Set(key, value); err != nil {
            return &abi.CommitResult{
                Error: fmt.Errorf("failed to set balance: %w", err),
            }
        }
    }

    // Write pending nonces to state
    for pubKey, nonce := range app.pendingNonces {
        key := append([]byte("nonce:"), pubKey[:]...)
        value := make([]byte, 8)
        binary.BigEndian.PutUint64(value, nonce)
        if err := app.state.Set(key, value); err != nil {
            return &abi.CommitResult{
                Error: fmt.Errorf("failed to set nonce: %w", err),
            }
        }
    }

    // Commit to state store
    hash, version, err := app.state.Commit()
    if err != nil {
        return &abi.CommitResult{
            Error: fmt.Errorf("failed to commit state: %w", err),
        }
    }

    // Clear pending changes
    app.pendingBalances = make(map[[32]byte]uint64)
    app.pendingNonces = make(map[[32]byte]uint64)

    return &abi.CommitResult{
        AppHash: hash,
    }
}

// Query handles state queries
func (app *KVStoreApp) Query(ctx context.Context, req *abi.QueryRequest) *abi.QueryResponse {
    app.mu.RLock()
    defer app.mu.RUnlock()

    switch req.Path {
    case "/balance":
        var pubKey [32]byte
        if len(req.Data) != 32 {
            return &abi.QueryResponse{
                Code:  abi.CodeInvalidTx,
                Error: errors.New("invalid public key length"),
            }
        }
        copy(pubKey[:], req.Data)

        balance := app.getBalance(pubKey)
        value := make([]byte, 8)
        binary.BigEndian.PutUint64(value, balance)

        return &abi.QueryResponse{
            Code:   abi.CodeOK,
            Data:   value,
            Height: app.state.Version(),
        }

    case "/nonce":
        var pubKey [32]byte
        if len(req.Data) != 32 {
            return &abi.QueryResponse{
                Code:  abi.CodeInvalidTx,
                Error: errors.New("invalid public key length"),
            }
        }
        copy(pubKey[:], req.Data)

        nonce := app.getNonce(pubKey)
        value := make([]byte, 8)
        binary.BigEndian.PutUint64(value, nonce)

        return &abi.QueryResponse{
            Code:   abi.CodeOK,
            Data:   value,
            Height: app.state.Version(),
        }

    default:
        return &abi.QueryResponse{
            Code:  abi.CodeNotAuthorized,
            Error: fmt.Errorf("unknown query path: %s", req.Path),
        }
    }
}

// Helper methods

func (app *KVStoreApp) getBalance(pubKey [32]byte) uint64 {
    key := append([]byte("balance:"), pubKey[:]...)
    value, err := app.state.Get(key)
    if err != nil || value == nil {
        return 0
    }
    return binary.BigEndian.Uint64(value)
}

func (app *KVStoreApp) getBalanceWithPending(pubKey [32]byte) uint64 {
    if balance, ok := app.pendingBalances[pubKey]; ok {
        return balance
    }
    return app.getBalance(pubKey)
}

func (app *KVStoreApp) getNonce(pubKey [32]byte) uint64 {
    key := append([]byte("nonce:"), pubKey[:]...)
    value, err := app.state.Get(key)
    if err != nil || value == nil {
        return 0
    }
    return binary.BigEndian.Uint64(value)
}

func (app *KVStoreApp) getNonceWithPending(pubKey [32]byte) uint64 {
    if nonce, ok := app.pendingNonces[pubKey]; ok {
        return nonce
    }
    return app.getNonce(pubKey)
}

func (app *KVStoreApp) Close() error {
    return app.state.Close()
}

```text

## Step 3: Create the Main Program

Create `main.go`:

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/blockberries/blockberry/pkg/config"
    "github.com/blockberries/blockberry/pkg/node"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("config.toml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create application
    app, err := NewKVStoreApp(cfg.StateStore.Path)
    if err != nil {
        log.Fatalf("Failed to create application: %v", err)
    }
    defer app.Close()

    // Create node
    n, err := node.NewNode(cfg)
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    // Start node
    log.Println("Starting KVStore blockchain node...")
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

## Step 4: Configuration

Create `config.toml`:

```toml
[node]
chain_id = "kvstore-testnet-1"
protocol_version = 1
private_key_path = "node_key.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10

[mempool]
type = "priority"  # Use priority mempool
max_txs = 5000
max_bytes = 1073741824

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

## Step 5: Build and Run

```bash

# Initialize node
blockberry init --chain-id kvstore-testnet-1

# Build
go build -o kvstore .

# Run
./kvstore

```text

## Step 6: Create a Client (Optional)

Create `client.go` to interact with your blockchain:

```go
package main

import (
    "crypto/ed25519"
    "crypto/rand"
    "encoding/binary"
    "fmt"
)

func createTestTransaction() *KVTransaction {
    // Generate keypairs
    pubKey1, privKey1, _ := ed25519.GenerateKey(rand.Reader)
    pubKey2, _, _ := ed25519.GenerateKey(rand.Reader)

    // Create transaction
    tx := &KVTransaction{
        Amount: 100,
        Nonce:  0,
    }
    copy(tx.From[:], pubKey1)
    copy(tx.To[:], pubKey2)

    // Sign transaction
    sig := ed25519.Sign(privKey1, tx.SigningBytes())
    copy(tx.Sig[:], sig)

    return tx
}

func main() {
    tx := createTestTransaction()
    fmt.Printf("Created transaction: %x\n", tx.Hash())
    fmt.Printf("Serialized: %x\n", tx.Serialize())
}

```text

## What You've Built

Congratulations! You've built a complete blockchain application with:

- **Custom transaction format** with cryptographic signatures
- **State management** using IAVL merkle trees
- **Balance tracking** with overdraft prevention
- **Nonce-based replay protection**
- **Event emission** for transaction indexing
- **Query support** for balance and nonce lookups

## Next Steps

### Add Features

1. **Fee market**: Charge transaction fees
2. **Staking**: Implement proof-of-stake
3. **Smart contracts**: Add execution environment
4. **Multi-signature**: Support multi-sig accounts
5. **Token standards**: Implement fungible/non-fungible tokens

### Production Readiness

1. **Add tests**: Unit and integration tests
2. **Benchmarks**: Performance testing
3. **Monitoring**: Prometheus metrics
4. **RPC server**: Enable external clients
5. **Genesis configuration**: Proper genesis setup

### Deployment

1. **Multiple nodes**: Set up validator network
2. **Consensus**: Add consensus engine
3. **Persistence**: Database optimization
4. **Security**: Security audit and hardening

## Resources

- [Application Development Guide](../guides/application-development.md) - In-depth ABI guide
- [State Management](../concepts/state-management.md) - IAVL merkle trees
- [Testing Guide](../guides/testing.md) - Testing strategies
- [Deployment Guide](../guides/deployment.md) - Production deployment

---

**Well done!** You've built your first blockchain application. Continue with the [application development guide](../guides/application-development.md) for advanced topics.
