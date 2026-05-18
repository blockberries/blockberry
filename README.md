# Blockberry

A modular blockchain node framework for Go. Provides composable
infrastructure — mempool, block store, state store, P2P reactors, RPC
transports — without committing to a specific consensus algorithm or
application.

Used by [raspberry](../raspberry/) as its node substrate.

## Scope

Blockberry is **not** a consensus engine. It defines the
`ConsensusEngine` interface and ships a `NullConsensus` for full nodes;
real engines (e.g. [leaderberry](../leaderberry/)) are plugged in by the
consumer.

It does not ship a concrete RPC server either — only the JSON-RPC, gRPC,
and WebSocket *transports*. The consumer (raspberry) provides the
backing `rpc.Server` implementation.

## What it does provide

- `Node` and `NodeBuilder` with role-aware wiring (`validator`, `full`,
  `seed`, `light`, `archive`).
- Mempool variants: `Simple`, `Priority`, `TTL`, `NoOp`, plus a
  Looseberry adapter.
- BlockStore variants: `LevelDB`, `BadgerDB`, `Memory`, `HeaderOnly`,
  `Pruning` wrapper, `NoOp`. Optional `CertificateStore` extension.
- `StateStore` interface with `IAVLStore` impl backed by [avlberry](../avlberry/).
- Seven internal P2P reactors: `handshake`, `pex`, `transactions`,
  `blocksync`, `blocks`, `consensus`, `housekeeping`.
- RPC transports: HTTP JSON-RPC, gRPC (cramberry-encoded), WebSocket.
- Events bus with non-blocking pub/sub.
- Eclipse-attack mitigations and per-peer rate limiting (libraries; not
  yet wired into Node by default).

## Usage

```go
import (
    "github.com/blockberries/blockberry/pkg/node"
    "github.com/blockberries/blockberry/pkg/types"
)

n, err := node.NewBuilder().
    WithRole(types.RoleFull).
    WithChainID("mainnet-1").
    WithDataDir("/var/lib/mychain").
    WithMempool(simpleMempool).
    WithBlockStore(blockStore).
    WithBlockValidator(myValidator).
    Build()

err = n.Start()
defer n.Stop()
```

## Layout

```
blockberry/
├── cmd/blockberry/        CLI: init, keys, start, status
├── pkg/
│   ├── types/             Hash, NodeRole, Component, NodeCallbacks, etc.
│   ├── mempool/           Simple, Priority, TTL, NoOp, looseberry adapter
│   ├── blockstore/        LevelDB, BadgerDB, Memory, HeaderOnly, Pruning, NoOp + Cert variants
│   ├── statestore/        IAVLStore (avlberry-backed), pruning, snapshots
│   ├── consensus/         ConsensusEngine, BFTConsensus interfaces; NullConsensus impl
│   │   └── bft/           Tendermint-style skeleton (NOT a working engine)
│   ├── node/              Node, NodeBuilder, RoleBasedBuilder, NetworkBridge
│   ├── rpc/               Server interface; jsonrpc/, grpc/, websocket/ transports
│   ├── events/            Pub/sub event bus
│   ├── indexer/           TxIndexer / BlockIndexer interfaces
│   └── config/ logging/ metrics/ tracing/
├── internal/
│   ├── handlers/          7 P2P reactors (handshake, transactions, blocks, consensus, housekeeping)
│   ├── p2p/               Network (glueberry wrapper), PeerManager, scoring
│   ├── pex/               Peer exchange + address book
│   ├── security/          EclipseProtector, RateLimit
│   ├── sync/              Block sync, state sync reactors
│   └── container/ memory/  DI container, buffer pools
├── schema/blockberry.cram .cram schemas + generated companion code
└── examples/              simple_node, custom_mempool, mock_consensus
```

## Application interface

Blockberry uses the [bapi](../bapi/) `Lifecycle` interface (5 methods).
The `pkg/types/null_app.go::NullApplication` is a no-op stub for
testing. Real applications (e.g. punnet-sdk) plug in via `bapi`.

## Status

The framework, network reactors, mempools, block stores, state store,
events bus, and RPC transports are real and tested. Phase A–C of
[`/Volumes/Tendermint/stealth/PLAN.md`](../PLAN.md) green; the
four-validator testnet produces blocks end-to-end.

- T2-7 (consensus reactor decoders) is mooted — the BFTConsensus path is
  unreachable from raspberry's setup.
- T2-8 (eclipse protection wiring) resolved: `EclipseProtector` is now
  installed unconditionally in `NodeBuilder.Build` with a bootstrap-window
  exemption (`OutboundCheckMinPeers` default 3) that prevents the
  4-validator localhost wedge.
- T2-9 (looseberry bridge encoders) resolved: bridge now uses Cramberry,
  matching raspberry's side.
- T2-10 (BFT skeleton) resolved: `pkg/consensus/bft/` deleted (D6).

Versioning: workspace uses `replace` directives so `require` versions in
`go.mod` are documentation only. CHANGELOG.md tracks user-visible
behavior changes.

## Development

See [`CLAUDE.md`](./CLAUDE.md) for development guidelines.
[`ARCHITECTURE.md`](./ARCHITECTURE.md) for design details.
[`CHANGELOG.md`](./CHANGELOG.md) for release history.

## License

Apache-2.0.
