# Blockberry — Architecture

## Goals

1. **Modular** — every component is replaceable via interfaces.
2. **Consensus-agnostic** — the framework provides plumbing; engines
   are plugged in.
3. **Application-agnostic** — applications speak `bapi.Lifecycle`; the
   framework neither knows nor cares about state semantics.
4. **Role-aware** — validator vs full vs seed vs light vs archive nodes
   wire different sub-components.

## Component model

```go
type Component interface {
    Start(ctx context.Context) error
    Stop() error
    IsRunning() bool
}

type Named interface { Name() string }
```

All major pieces implement `Component`. `Node.Start` brings them up in
dependency order; `Node.Stop` brings them down in reverse.

## Roles

| Role       | Mempool                  | BlockStore     | StateStore | Consensus       |
|------------|--------------------------|----------------|------------|------------------|
| validator  | DAG (Looseberry adapter) | LevelDB/Badger | IAVL       | Plug-in (BFT)    |
| full       | Simple                   | LevelDB/Badger | IAVL       | NullConsensus    |
| seed       | NoOp                     | NoOp           | NoOp       | None             |
| light      | NoOp                     | HeaderOnly     | NoOp       | None             |
| archive    | NoOp                     | LevelDB (no prune) | IAVL   | None             |

`pkg/node/role_builder.go::validateRoleRequirements` enforces these.

## Mempool

`pkg/mempool/Mempool` interface:

```go
AddTx(tx Tx) error
RemoveTxs(hashes []Hash)
ReapTxs(maxBytes int) []Tx
HasTx(hash Hash) bool
GetTx(hash Hash) (Tx, bool)
Size() int
SizeBytes() int64
Flush()
TxHashes() []Hash
SetTxValidator(v TxValidator)
```

| Variant     | When to use                                                  |
|-------------|--------------------------------------------------------------|
| `SimpleMempool`   | Full nodes; FIFO; hash-indexed; byte/count caps         |
| `PriorityMempool` | Fee-based ordering; evicts lowest priority on overflow  |
| `TTLMempool`      | Per-tx expiration; background cleanup goroutine         |
| `NoopMempool`     | Seed/light nodes; rejects all                           |
| `looseberry/Adapter` | Validators; bridges Looseberry into the Mempool surface |

The DAG adapter has known incomplete methods (RemoveTxs no-op, GetTx not
supported, TxHashes nil, certificate transport stubbed). Transactions
reactor auto-detects DAG and disables active gossip.

## BlockStore

`pkg/blockstore/BlockStore`:

```go
SaveBlock(height int64, hash Hash, data []byte) error
LoadBlock(height int64) ([]byte, error)
LoadBlockByHash(hash Hash) ([]byte, error)
HasBlock(height int64) bool
Height() int64
Base() int64
Close() error
```

Plus optional extensions:

- `PrunableBlockStore` — `Prune(beforeHeight)`.
- `CertificateStore` — `SaveCertificate`, `GetCertificateForRound`,
  `SaveBatch`, `GetBatch`, etc.

| Variant         | Backing                              |
|-----------------|--------------------------------------|
| `LevelDBBlockStore`  | leveldb (atomic batched, sync writes)  |
| `BadgerDBBlockStore` | BadgerDB (multi-index for certificates) |
| `MemoryBlockStore`   | map (testing)                          |
| `HeaderOnlyBlockStore` | height ↔ hash only (light clients) |
| `PruningBlockStore`  | wraps any other with background pruning |
| `NoopBlockStore`     | seed nodes |

**Pruning correctness**: `PruneConfig.CalculatePruneTarget` does
`currentHeight - KeepRecent` with no concept of finality. Consumers
must ensure the target is below the finalized height.

**HeaderOnly safety**: `SaveBlock(height, hash, data)` discards `data`
without verifying it hashes to `hash`. Upstream (the consumer) must
verify before calling.

## StateStore

`pkg/statestore/StateStore`:

```go
Get(key []byte) ([]byte, error)
Set(key, value []byte) error
Delete(key []byte) error
Has(key []byte) (bool, error)
Commit() (Hash, int64, error)
RootHash() Hash
Version() int64
LoadVersion(version int64) error
GetProof(key []byte) (*Proof, error)
```

`IAVLStore` (`pkg/statestore/iavl.go`) is backed by [avlberry](../avlberry/).
Offers ICS-23 proofs via `Proof.Verify(rootHash)` calling
`ics23.VerifyMembership` / `VerifyNonMembership` with `IavlSpec`.

## Consensus engine plug-in

Three layers of interface:

```go
ConsensusEngine        // required: Start/Stop, ProcessBlock, GetHeight, ValidatorSet
└─ BFTConsensus        // adds: ProduceBlock, HandleProposal/Vote/Commit, OnTimeout
   └─ StreamAwareConsensus  // adds: StreamConfigs(), HandleStreamMessage(stream, peer, data)
```

Built-in: `NullConsensus` for full nodes (real, tracks height/round,
no proposals).

`pkg/consensus/bft/tendermint.go` is an explicit skeleton (not a working
engine). Raspberry plugs leaderberry in via `consensus.RegisterConsensus`.

`ConsensusDependencies` carries `Network`, `BlockStore`, `StateStore`,
`Mempool`, `Application bapi.Lifecycle`, `Callbacks`, `Config`. The
`StateStore` field is typed `*statestore.StateStore` (pointer to
interface — almost certainly a typo for `statestore.StateStore`).

## P2P reactors

Each reactor owns a stream name + a contiguous TypeID range:

| Reactor              | Streams                | TypeIDs   | Real or stub?                            |
|----------------------|------------------------|-----------|------------------------------------------|
| `HandshakeHandler`   | `handshake`            | 128–130   | Real, full 3-phase, chain-id check, version mismatch ban |
| `PexReactor`         | `pex`                  | 131–132   | Real, address-book backed                |
| `TransactionsReactor`| `transactions`         | 133–136   | Real, two-phase pull-based gossip; auto-disables on DAG mempool |
| `BlockReactor`       | `blocks`               | 139       | Real, real-time block propagation        |
| `BlockSyncReactor`   | `blocksync`            | 137–138   | Real, parallel requests; fail-closed without `BlockValidator` |
| `ConsensusReactor`   | `consensus`            | engine-defined | Routes to engine; **decoders hand-rolled big-endian** (PLAN T2-7) |
| `HousekeepingReactor`| `housekeeping`         | 140–143   | Latency probes real; firewall detection stubbed |
| `StateSyncReactor`   | (snapshot streams)     | 144–147   | Full state machine implemented           |

## RPC

Three transports for one server interface:

```go
type Server interface {
    Health(ctx) (HealthResponse, error)
    Status(ctx) (StatusResponse, error)
    NetInfo(ctx) (NetInfoResponse, error)
    BroadcastTxSync / Async / Commit (ctx, tx) → BroadcastResponse
    ABCIQuery(ctx, path, data, height, prove) → QueryResponse
    Block(ctx, height) → BlockResponse
    BlockByHash(ctx, hash) → BlockResponse
    Tx(ctx, hash, prove) → TxResponse
    TxSearch(ctx, query, ...) → TxSearchResponse
    Validators(ctx, height) → ValidatorsResponse
    ConsensusState(ctx) → ConsensusStateResponse
    Peers(ctx) → PeersResponse
    Subscribe / Unsubscribe / UnsubscribeAll (events)
}
```

Transports:

- `pkg/rpc/jsonrpc/` — HTTP JSON-RPC.
- `pkg/rpc/grpc/` — gRPC with `CramberryCodec` (replaces protobuf).
  Schemas in `schema/blockberry.cram`. Proof serialization stubbed.
- `pkg/rpc/websocket/` — WebSocket for `Subscribe`/`Unsubscribe`.

**Blockberry ships no concrete `Server` implementation.** Consumers
provide it (raspberry's `internal/rpc/`).

## Events

`pkg/events/Bus`:

```go
Subscribe(subscriberID, query) → (Subscription, error)
Publish(event)                  // non-blocking; drops on full
PublishWithTimeout(event, timeout) → bool
Unsubscribe(subscriberID, query)
UnsubscribeAll(subscriberID)
```

Per-subscription channel size 100, fan-out by query match. No automatic
slow-subscriber removal.

## Security libraries

- `internal/security/EclipseProtector` — tracks peers by subnet and
  source ID; enforces minimum outbound percent. **Not wired into Node**
  by default (PLAN T2-8).
- `internal/security/RateLimit` — per-peer/per-stream token bucket.
  Wired through `internal/p2p/rate_limiter.go`.

## Layout

```
blockberry/
├── cmd/blockberry/        CLI binary
├── pkg/
│   ├── types/             Hash, NodeRole, Component, NodeCallbacks, NullApplication
│   ├── mempool/           Mempool interface + 5 impls + looseberry/ adapter
│   ├── blockstore/        BlockStore interface + 6 impls + Cert variants
│   ├── statestore/        StateStore interface + IAVLStore (avlberry)
│   ├── consensus/         Engine interfaces + NullConsensus + bft/ skeleton
│   ├── node/              Node, NodeBuilder, RoleBasedBuilder, NetworkBridge
│   ├── rpc/               Server interface + jsonrpc, grpc, websocket transports
│   ├── events/            Bus + Query parsing
│   ├── indexer/           TxIndexer + BlockIndexer + kv/
│   ├── config/ logging/ metrics/ tracing/
├── internal/
│   ├── handlers/          7 P2P reactors
│   ├── p2p/               Network (glueberry wrapper), PeerManager
│   ├── pex/               Peer exchange + address book
│   ├── security/          EclipseProtector, RateLimit
│   ├── sync/              Block sync, state sync
│   ├── container/ memory/ DI, pools
├── schema/blockberry.cram + generated
└── examples/              simple_node, custom_mempool, mock_consensus
```
