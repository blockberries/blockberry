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

### Type: PriorityMempool

Priority-based mempool with configurable ordering.

```go
type PriorityMempool struct {
    // unexported fields
}
```

#### func NewPriorityMempool

```go
func NewPriorityMempool(cfg PriorityMempoolConfig) *PriorityMempool
```

Creates a new priority-based mempool.

#### func (*PriorityMempool) GetPriority

```go
func (m *PriorityMempool) GetPriority(hash []byte) int64
```

Returns the priority of a transaction (0 if not found).

#### func (*PriorityMempool) HighestPriority

```go
func (m *PriorityMempool) HighestPriority() int64
```

Returns the highest priority in the mempool.

#### func (*PriorityMempool) SetPriorityFunc

```go
func (m *PriorityMempool) SetPriorityFunc(fn PriorityFunc)
```

Updates the priority calculation function.

### Type: TTLMempool

Mempool with automatic transaction expiration.

```go
type TTLMempool struct {
    // unexported fields
}
```

#### func NewTTLMempool

```go
func NewTTLMempool(cfg TTLMempoolConfig) *TTLMempool
```

Creates a new TTL-based mempool.

#### func (*TTLMempool) AddTxWithTTL

```go
func (m *TTLMempool) AddTxWithTTL(tx []byte, ttl time.Duration) error
```

Adds a transaction with a custom TTL.

#### func (*TTLMempool) GetTTL

```go
func (m *TTLMempool) GetTTL(hash []byte) time.Duration
```

Returns remaining TTL for a transaction (0 if not found or expired).

#### func (*TTLMempool) ExtendTTL

```go
func (m *TTLMempool) ExtendTTL(hash []byte, extension time.Duration) bool
```

Extends the TTL for a transaction. Returns false if not found.

#### func (*TTLMempool) SetTTL

```go
func (m *TTLMempool) SetTTL(hash []byte, expiresAt time.Time) bool
```

Sets a specific expiration time. Returns false if not found.

#### func (*TTLMempool) SizeActive

```go
func (m *TTLMempool) SizeActive() int
```

Returns count of non-expired transactions.

#### func (*TTLMempool) Stop

```go
func (m *TTLMempool) Stop()
```

Stops the background cleanup goroutine.

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
    ProofBytes []byte  // Serialized ICS23 commitment proof
    Version    int64
}
```

#### func (*Proof) Verify

```go
func (p *Proof) Verify(rootHash []byte) (bool, error)
```

Verifies the proof against a root hash using ICS23 verification. Supports both existence proofs (key-value exists) and non-existence proofs (key does not exist).

Returns `(true, nil)` if valid, `(false, nil)` if invalid, or `(false, error)` if verification fails.

#### func (*Proof) VerifyConsistent

```go
func (p *Proof) VerifyConsistent(rootHash []byte) error
```

Helper that returns an error if the proof is invalid.

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

### Type: RateLimiter

Per-peer, per-stream rate limiting.

```go
type RateLimiter struct {
    // unexported fields
}
```

#### func NewRateLimiter

```go
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter
```

Creates a new rate limiter with the given configuration.

#### func (*RateLimiter) Allow

```go
func (rl *RateLimiter) Allow(peerID peer.ID, stream string, messageSize int) bool
```

Checks if a message from a peer should be allowed. Returns true if within limits.

#### func (*RateLimiter) AllowN

```go
func (rl *RateLimiter) AllowN(peerID peer.ID, stream string, n int, totalSize int) bool
```

Checks if n messages with total size should be allowed.

#### func (*RateLimiter) ResetPeer

```go
func (rl *RateLimiter) ResetPeer(peerID peer.ID)
```

Resets rate limit state for a peer (allows them to send again after being throttled).

#### func (*RateLimiter) RemovePeer

```go
func (rl *RateLimiter) RemovePeer(peerID peer.ID)
```

Removes all rate limit state for a peer.

#### func (*RateLimiter) SetLimits

```go
func (rl *RateLimiter) SetLimits(limits RateLimits)
```

Updates rate limits (affects new peer limiters only).

#### func (*RateLimiter) Stop

```go
func (rl *RateLimiter) Stop()
```

Stops the background cleanup goroutine.

### Type: RateLimits

```go
type RateLimits struct {
    MessagesPerSecond map[string]float64  // Per-stream message limits
    BytesPerSecond    int64               // Overall bandwidth limit (0 = unlimited)
    BurstSize         int                 // Token bucket burst size
}
```

#### func DefaultRateLimits

```go
func DefaultRateLimits() RateLimits
```

Returns sensible default rate limits for all stream types.

---

## Package: metrics

### Interface: Metrics

```go
type Metrics interface {
    // Peer metrics
    SetPeersTotal(direction string, count int)
    IncPeerConnections(result string)
    IncPeerDisconnections(reason string)

    // Block metrics
    SetBlockHeight(height int64)
    IncBlocksReceived()
    IncBlocksProposed()
    ObserveBlockLatency(seconds float64)
    ObserveBlockSize(bytes int)

    // Transaction metrics
    SetMempoolSize(count int)
    SetMempoolBytes(bytes int64)
    IncTxsReceived()
    IncTxsProposed()
    IncTxsRejected(reason string)
    IncTxsEvicted()

    // Sync metrics
    SetSyncState(state string)
    SetSyncProgress(progress float64)
    SetSyncPeerHeight(height int64)
    ObserveSyncDuration(seconds float64)

    // Message metrics
    IncMessagesReceived(stream string)
    IncMessagesSent(stream string)
    IncMessageErrors(stream, errorType string)
    ObserveMessageSize(stream string, bytes int)

    // Latency metrics
    ObservePeerLatency(peerID string, seconds float64)

    // State store metrics
    SetStateVersion(version int64)
    SetStateSize(bytes int64)

    // HTTP handler
    Handler() any
}
```

#### func NewPrometheusMetrics

```go
func NewPrometheusMetrics(namespace string) *PrometheusMetrics
```

Creates Prometheus metrics with the given namespace.

#### func NewNopMetrics

```go
func NewNopMetrics() *NopMetrics
```

Creates a no-op metrics implementation (zero overhead when disabled).

---

## Package: logging

### Type: Logger

```go
type Logger struct {
    *slog.Logger
}
```

Wraps `slog.Logger` with convenience methods for blockchain-specific logging.

#### Factory Functions

```go
func NewTextLogger(w io.Writer, level slog.Level) *Logger
func NewJSONLogger(w io.Writer, level slog.Level) *Logger
func NewProductionLogger() *Logger   // JSON to stdout, INFO level
func NewDevelopmentLogger() *Logger  // Text to stdout, DEBUG level
func NewNopLogger() *Logger          // Discards all logs
```

#### Builder Methods

```go
func (l *Logger) With(args ...any) *Logger
func (l *Logger) WithComponent(name string) *Logger
func (l *Logger) WithPeer(id peer.ID) *Logger
func (l *Logger) WithStream(name string) *Logger
```

#### Attribute Constructors

```go
func Component(name string) slog.Attr      // "component" field
func PeerID(id peer.ID) slog.Attr          // "peer_id" field
func PeerIDStr(id string) slog.Attr        // "peer_id" field (string)
func Height(h int64) slog.Attr             // "height" field
func Version(v int64) slog.Attr            // "version" field
func Hash(h []byte) slog.Attr              // "hash" field (hex)
func TxHash(h []byte) slog.Attr            // "tx_hash" field (hex)
func BlockHash(h []byte) slog.Attr         // "block_hash" field (hex)
func Stream(name string) slog.Attr         // "stream" field
func MsgType(t string) slog.Attr           // "msg_type" field
func Duration(d time.Duration) slog.Attr   // "duration" field
func Latency(d time.Duration) slog.Attr    // "latency_ms" field
func Count(n int) slog.Attr                // "count" field
func Size(n int) slog.Attr                 // "size" field
func ChainID(id string) slog.Attr          // "chain_id" field
func NodeID(id string) slog.Attr           // "node_id" field
func Direction(isOutbound bool) slog.Attr  // "direction" field
func Error(err error) slog.Attr            // "error" field
func Reason(r string) slog.Attr            // "reason" field
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

### Validation Functions

```go
func ValidateHeight(h int64) error
func ValidateHeightRange(from, to int64) error
func ValidateBatchSize(size, maxSize int32) error
func ValidateMessageSize(size int) error
func ValidateTransactionSize(size int) error
func ValidateKeySize(key []byte) error
func ValidateValueSize(value []byte) error
func ValidateHash(hash []byte, expectedLen int) error
func ValidateBlockData(height *int64, hash, data []byte) error
func ClampBatchSize(size, maxSize int32) int32
func MustNotBeNil(ptr any, name string) error
```

### Validation Constants

```go
const (
    MaxBatchSize       = 1000
    MaxBlockHeight     = 1<<62 - 1
    MaxMessageSize     = 10 * 1024 * 1024  // 10 MB
    MaxTransactionSize = 1 * 1024 * 1024   // 1 MB
    MaxKeySize         = 1024              // 1 KB
    MaxValueSize       = 10 * 1024 * 1024  // 10 MB
)
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
    Metrics      MetricsConfig
    Logging      LoggingConfig
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
