# API Reference

This document provides a detailed API reference for blockberry's public interfaces.

## Package: node

### Type: Node

The main coordinator for a blockberry node.

```go
type Node struct {
    // unexported fields
}
```

#### func NewNode

```go
func NewNode(cfg *config.Config, opts ...Option) (*Node, error)
```

Creates a new blockberry node with the given configuration. The node is not started until `Start()` is called.

**Parameters:**
- `cfg`: Node configuration
- `opts`: Optional functional options

**Returns:**
- `*Node`: The created node
- `error`: Error if creation fails

#### func (*Node) Start

```go
func (n *Node) Start() error
```

Starts the node and all its components including network, reactors, and background goroutines.

#### func (*Node) Stop

```go
func (n *Node) Stop() error
```

Stops the node and all its components gracefully.

#### func (*Node) IsRunning

```go
func (n *Node) IsRunning() bool
```

Returns whether the node is currently running.

#### func (*Node) PeerID

```go
func (n *Node) PeerID() peer.ID
```

Returns the node's libp2p peer ID.

#### func (*Node) NodeID

```go
func (n *Node) NodeID() string
```

Returns the node's hex-encoded Ed25519 public key.

#### func (*Node) Network

```go
func (n *Node) Network() *p2p.Network
```

Returns the P2P network layer.

#### func (*Node) BlockStore

```go
func (n *Node) BlockStore() blockstore.BlockStore
```

Returns the block storage.

#### func (*Node) Mempool

```go
func (n *Node) Mempool() mempool.Mempool
```

Returns the transaction mempool.

#### func (*Node) PeerCount

```go
func (n *Node) PeerCount() int
```

Returns the number of connected peers.

### Options

#### func WithMempool

```go
func WithMempool(mp mempool.Mempool) Option
```

Sets a custom mempool implementation.

#### func WithBlockStore

```go
func WithBlockStore(bs blockstore.BlockStore) Option
```

Sets a custom block store implementation.

#### func WithConsensusHandler

```go
func WithConsensusHandler(ch handlers.ConsensusHandler) Option
```

Sets the consensus message handler.

---

## Package: mempool

### Interface: Mempool

```go
type Mempool interface {
    AddTx(tx []byte) error
    RemoveTxs(hashes [][]byte)
    ReapTxs(maxBytes int64) [][]byte
    HasTx(hash []byte) bool
    GetTx(hash []byte) ([]byte, error)
    Size() int
    SizeBytes() int64
    Flush()
    TxHashes() [][]byte
}
```

#### AddTx

```go
AddTx(tx []byte) error
```

Adds a transaction to the mempool. Returns `types.ErrMempoolFull` if at capacity or `types.ErrDuplicateTx` if already exists.

#### RemoveTxs

```go
RemoveTxs(hashes [][]byte)
```

Removes transactions by their hashes. Used after block commit.

#### ReapTxs

```go
ReapTxs(maxBytes int64) [][]byte
```

Returns transactions for block proposal up to `maxBytes`. Transactions remain in mempool until explicitly removed.

#### HasTx

```go
HasTx(hash []byte) bool
```

Returns true if transaction exists in mempool.

#### GetTx

```go
GetTx(hash []byte) ([]byte, error)
```

Returns transaction data by hash.

#### Size

```go
Size() int
```

Returns number of transactions in mempool.

#### SizeBytes

```go
SizeBytes() int64
```

Returns total size of all transactions in bytes.

#### Flush

```go
Flush()
```

Removes all transactions from mempool.

#### TxHashes

```go
TxHashes() [][]byte
```

Returns hashes of all transactions in mempool.

---

## Package: blockstore

### Interface: BlockStore

```go
type BlockStore interface {
    SaveBlock(height int64, hash []byte, data []byte) error
    LoadBlock(height int64) (hash []byte, data []byte, error)
    LoadBlockByHash(hash []byte) (height int64, data []byte, error)
    HasBlock(height int64) bool
    Height() int64
    Base() int64
    Close() error
}
```

#### SaveBlock

```go
SaveBlock(height int64, hash []byte, data []byte) error
```

Persists a block. Returns `types.ErrBlockExists` if block already exists at height.

#### LoadBlock

```go
LoadBlock(height int64) (hash []byte, data []byte, error)
```

Loads a block by height. Returns `types.ErrBlockNotFound` if not found.

#### LoadBlockByHash

```go
LoadBlockByHash(hash []byte) (height int64, data []byte, error)
```

Loads a block by its hash.

#### HasBlock

```go
HasBlock(height int64) bool
```

Returns true if block exists at height.

#### Height

```go
Height() int64
```

Returns the latest block height. Returns 0 for empty store.

#### Base

```go
Base() int64
```

Returns the earliest available block height (for pruned stores).

---

## Package: statestore

### Interface: StateStore

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

#### Get

```go
Get(key []byte) ([]byte, error)
```

Retrieves a value by key. Returns nil if key doesn't exist.

#### Has

```go
Has(key []byte) (bool, error)
```

Returns true if key exists.

#### Set

```go
Set(key []byte, value []byte) error
```

Stores a key-value pair. Changes are visible immediately but not persisted until Commit.

#### Delete

```go
Delete(key []byte) error
```

Removes a key.

#### Commit

```go
Commit() (hash []byte, version int64, error)
```

Persists all pending changes and returns the new merkle root hash and version.

#### RootHash

```go
RootHash() []byte
```

Returns the current merkle root hash (includes uncommitted changes).

#### Version

```go
Version() int64
```

Returns the current committed version.

#### LoadVersion

```go
LoadVersion(version int64) error
```

Loads a historical version of the state.

#### GetProof

```go
GetProof(key []byte) (*Proof, error)
```

Generates a merkle proof for a key.

### Type: Proof

```go
type Proof struct {
    Key        []byte
    Value      []byte
    Exists     bool
    RootHash   []byte
    Version    int64
    ProofBytes []byte  // Serialized ICS23 proof
}
```

---

## Package: p2p

### Type: Network

```go
type Network struct {
    // unexported fields
}
```

#### func (*Network) Send

```go
func (n *Network) Send(peerID peer.ID, stream string, data []byte) error
```

Sends data to a specific peer on the named stream.

#### func (*Network) Broadcast

```go
func (n *Network) Broadcast(stream string, data []byte) error
```

Broadcasts data to all connected peers on the named stream.

#### func (*Network) PeerManager

```go
func (n *Network) PeerManager() *PeerManager
```

Returns the peer manager.

#### func (*Network) ConnectMultiaddr

```go
func (n *Network) ConnectMultiaddr(addr string) error
```

Connects to a peer at the given multiaddr.

#### func (*Network) Disconnect

```go
func (n *Network) Disconnect(peerID peer.ID) error
```

Disconnects from a peer.

#### func (*Network) BlacklistPeer

```go
func (n *Network) BlacklistPeer(peerID peer.ID) error
```

Blacklists a peer permanently.

### Stream Constants

```go
const (
    StreamHandshake    = "handshake"
    StreamPEX          = "pex"
    StreamTransactions = "transactions"
    StreamBlockSync    = "blocksync"
    StreamBlocks       = "blocks"
    StreamConsensus    = "consensus"
    StreamHousekeeping = "housekeeping"
)
```

---

## Package: handlers

### Interface: ConsensusHandler

```go
type ConsensusHandler interface {
    HandleConsensusMessage(peerID peer.ID, data []byte) error
}
```

Implement this interface to handle consensus protocol messages.

---

## Package: types

### Type Aliases

```go
type Height int64
type Hash []byte
type Tx []byte
type Block []byte
```

### Hash Functions

```go
func HashTx(tx Tx) Hash
func HashBlock(block Block) Hash
func HashBytes(data []byte) Hash
```

### Error Types

```go
var (
    // Peer errors
    ErrPeerNotFound      = errors.New("peer not found")
    ErrPeerBlacklisted   = errors.New("peer is blacklisted")

    // Block errors
    ErrBlockNotFound     = errors.New("block not found")
    ErrBlockExists       = errors.New("block already exists")
    ErrInvalidBlock      = errors.New("invalid block")

    // Transaction errors
    ErrTxNotFound        = errors.New("transaction not found")
    ErrDuplicateTx       = errors.New("duplicate transaction")
    ErrMempoolFull       = errors.New("mempool is full")
    ErrInvalidTx         = errors.New("invalid transaction")

    // Protocol errors
    ErrChainIDMismatch   = errors.New("chain ID mismatch")
    ErrVersionMismatch   = errors.New("protocol version mismatch")
    ErrInvalidMessage    = errors.New("invalid message format")
    ErrHandshakeFailed   = errors.New("handshake failed")

    // Connection errors
    ErrConnectionClosed  = errors.New("connection closed")
    ErrConnectionTimeout = errors.New("connection timeout")

    // Node lifecycle errors
    ErrNodeAlreadyStarted = errors.New("node already started")
    ErrNodeNotStarted     = errors.New("node not started")
)
```

---

## Package: config

### Type: Config

```go
type Config struct {
    Node         NodeConfig
    Network      NetworkConfig
    PEX          PEXConfig
    Mempool      MempoolConfig
    BlockStore   BlockStoreConfig
    StateStore   StateStoreConfig
    Housekeeping HousekeepingConfig
}
```

#### func DefaultConfig

```go
func DefaultConfig() *Config
```

Returns a configuration with sensible defaults.

#### func LoadConfig

```go
func LoadConfig(path string) (*Config, error)
```

Loads configuration from a TOML file.

#### func (*Config) Validate

```go
func (c *Config) Validate() error
```

Validates the configuration.

#### func WriteConfigFile

```go
func WriteConfigFile(path string, cfg *Config) error
```

Writes configuration to a TOML file.
