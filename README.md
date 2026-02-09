# Blockberry

**A modular blockchain node framework for Go**

[![Go Version](https://img.shields.io/badge/Go-1.25.6+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen?style=flat)](#)
[![Go Report Card](https://img.shields.io/badge/go%20report-A+-brightgreen?style=flat)](#)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/badge/release-v0.1.7-blue?style=flat)](#)

---

## Overview

Blockberry is a **generic blockchain node framework** that provides production-ready infrastructure for building custom blockchain networks. It is **not** a consensus engine or smart contract platform—instead, it offers a composable architecture where applications implement their own state machines and consensus algorithms while leveraging battle-tested networking, storage, and transaction pooling.

### What Blockberry Is

- **Modular Framework**: Pluggable consensus engines, mempool implementations, and storage backends
- **P2P Infrastructure**: Secure networking via [Glueberry](https://github.com/blockberries/glueberry) with encrypted streams
- **Production-Ready**: Comprehensive observability, security hardening, and failure handling
- **High-Performance**: Binary serialization via [Cramberry](https://github.com/blockberries/cramberry), buffer pooling, and zero-copy optimizations

### What Blockberry Is NOT

- **Not a Consensus Engine**: You implement your own consensus (BFT, DAG, PoS, etc.)
- **Not a Smart Contract Platform**: No built-in VM—applications define their own execution logic
- **Not Application-Specific**: Generic framework suitable for any blockchain type

---

## Key Features

### Core Features

- **Pluggable Consensus Engines**: Implement BFT, DAG, PoS, or custom consensus algorithms
- **Flexible Mempool Implementations**:
  - `SimpleMempool`: FIFO ordering for straightforward transaction processing
  - `PriorityMempool`: Fee-based or priority-based ordering
  - `TTLMempool`: Automatic transaction expiration
  - `Looseberry`: DAG-based mempool with certified batch ordering (optional)
- **Multiple Storage Backends**:
  - LevelDB (production-ready, proven)
  - BadgerDB (high-performance alternative)
  - In-memory (testing only)
- **Node Roles**: Validator, Full Node, Seed Node, Light Client, Archive Node
- **Application Binary Interface (ABI v2.0)**: Clean contract between framework and application logic

### Networking

- **Secure P2P**: Encrypted streams via Glueberry with Ed25519 key exchange
- **Peer Exchange (PEX)**: Automatic peer discovery and address book management
- **Rate Limiting**: Per-peer, per-stream rate limits with token bucket algorithm
- **Reputation Scoring**: Progressive penalty system with automatic bans
- **Eclipse Attack Protection**: Connection diversity enforcement

### Observability

- **Prometheus Metrics**: 50+ metrics covering peers, blocks, transactions, sync, and consensus
- **OpenTelemetry Tracing**: Distributed tracing across network boundaries
- **Structured Logging**: Production-ready logging via Go's `slog` package
- **Health Checks**: Liveness and readiness probes for orchestration platforms

### Security

- **Fail-Closed by Default**: All validators reject unless explicitly enabled
- **Input Validation**: Comprehensive validation at all protocol boundaries
- **Progressive Penalty System**: Automatic detection and penalization of misbehavior
- **Timing Attack Prevention**: Constant-time comparisons for cryptographic operations
- **Resource Limits**: Configurable limits prevent memory exhaustion and DoS attacks

---

## Architecture

```text
┌─────────────────────────────────────────┐
│     Application Layer (Your Code)       │
│  - State machine                         │
│  - Transaction validation                │
│  - Query handling                        │
├─────────────────────────────────────────┤
│     Consensus Layer (Pluggable)         │
│  - BFT / DAG / PoS / Custom             │
│  - Block production & validation         │
├─────────────────────────────────────────┤
│     Mempool Layer (Pluggable)           │
│  - Simple / Priority / TTL / DAG        │
│  - Transaction ordering & gossiping      │
├─────────────────────────────────────────┤
│     Network Layer (Glueberry)           │
│  - Encrypted P2P streams                │
│  - Peer discovery & management           │
├─────────────────────────────────────────┤
│  Storage (BlockStore + StateStore)      │
│  - LevelDB / BadgerDB / In-memory       │
│  - AVL+ merkle trees for state           │
└─────────────────────────────────────────┘

```text

For detailed architecture documentation, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Documentation](#documentation)
- [Examples](#examples)
- [Configuration](#configuration)
- [Building Applications](#building-applications)
- [Development](#development)
- [Project Structure](#project-structure)
- [Dependencies](#dependencies)
- [Contributing](#contributing)
- [Versioning](#versioning)
- [License](#license)
- [Acknowledgments](#acknowledgments)
- [Support](#support)
- [Roadmap](#roadmap)

---

## Installation

### Prerequisites

- **Go 1.25.6 or higher**
- **Git**

### Install as a Library

```bash
go get github.com/blockberries/blockberry@latest

```text

### Build from Source

```bash
git clone https://github.com/blockberries/blockberry.git
cd blockberry
make build

```text

### Install CLI Tool

```bash
go install github.com/blockberries/blockberry/cmd/blockberry@latest

```text

Verify installation:

```bash
blockberry version

```text

---

## Quick Start

### Initialize a New Node

```bash
blockberry init --chain-id my-chain

```text

This creates a default configuration file (`config.toml`) and generates node keys.

### Start the Node

```bash
blockberry start --config config.toml

```text

### Programmatic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/blockberries/blockberry/pkg/abi"
    "github.com/blockberries/blockberry/pkg/config"
    "github.com/blockberries/blockberry/pkg/node"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("config.toml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create custom application
    app := &MyApplication{}

    // Create node
    n, err := node.NewNode(cfg,
        node.WithApplication(app),
    )
    if err != nil {
        log.Fatalf("Failed to create node: %v", err)
    }

    // Start node
    if err := n.Start(); err != nil {
        log.Fatalf("Failed to start node: %v", err)
    }
    defer n.Stop()

    // Wait for shutdown signal
    select {}
}

// MyApplication implements the ABI interface
type MyApplication struct {
    abi.BaseApplication
}

func (app *MyApplication) CheckTx(ctx context.Context, tx *abi.Transaction) *abi.TxCheckResult {
    // Validate transaction
    if len(tx.Data) == 0 {
        return &abi.TxCheckResult{
            Code:  abi.CodeInvalidTx,
            Error: errors.New("empty transaction"),
        }
    }
    return &abi.TxCheckResult{Code: abi.CodeOK}
}

func (app *MyApplication) ExecuteTx(ctx context.Context, tx *abi.Transaction) *abi.TxExecResult {
    // Execute transaction and update state
    return &abi.TxExecResult{
        Code:    abi.CodeOK,
        GasUsed: 1000,
    }
}

func (app *MyApplication) Commit(ctx context.Context) *abi.CommitResult {
    // Persist state changes
    hash := computeStateHash()
    return &abi.CommitResult{AppHash: hash}
}

```text

---

## Documentation

### Comprehensive Guides

- [**Architecture Overview**](ARCHITECTURE.md) - System architecture and design patterns
- [**API Reference**](API_REFERENCE.md) - Complete package and interface documentation
- [**ABI Design**](ABI_DESIGN.md) - Application Binary Interface v2.0 specification
- [**Development Guidelines**](CLAUDE.md) - Developer workflow and conventions
- [**Changelog**](CHANGELOG.md) - Version history and release notes

### Package Documentation

- [pkg.go.dev/github.com/blockberries/blockberry](https://pkg.go.dev/github.com/blockberries/blockberry)

### Extended Documentation

The `docs/` directory contains tutorials, guides, and additional documentation.

---

## Examples

Blockberry includes several example applications demonstrating key features:

### Available Examples

- [**Simple Node**](examples/simple_node/) - Basic node setup with minimal configuration
- [**Custom Mempool**](examples/custom_mempool/) - Implementing a custom mempool strategy
- [**Mock Consensus**](examples/mock_consensus/) - Integrating a consensus engine

### Running Examples

```bash

# Build all examples
make examples

# Run specific example
make example-simple
make example-mempool
make example-consensus

```text

---

## Configuration

Blockberry uses TOML configuration files. Example configuration:

```toml
[node]
chain_id = "my-chain"
protocol_version = 1
private_key_path = "node_key.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10

[network.seeds]
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/12D3KooW..."
]

[mempool]
type = "simple"  # simple, priority, ttl, looseberry
max_txs = 5000
max_bytes = 1073741824  # 1GB

[blockstore]
backend = "leveldb"  # leveldb or badgerdb
path = "data/blockstore"

[statestore]
path = "data/state"
cache_size = 10000

[logging]
level = "info"
format = "text"
output = "stderr"

[limits]
max_tx_size = 1048576        # 1MB
max_block_size = 22020096    # 21MB
max_block_txs = 10000

```text

For complete configuration options, see [API_REFERENCE.md - Configuration Reference](API_REFERENCE.md#configuration-reference).

---

## Building Applications

### Implement the Application Interface

Applications must implement the `abi.Application` interface:

```go
type Application interface {
    // Lifecycle
    Info() ApplicationInfo
    InitChain(genesis *Genesis) error

    // Transaction validation (concurrent-safe)
    CheckTx(ctx context.Context, tx *Transaction) *TxCheckResult

    // Block execution (sequential)
    BeginBlock(ctx context.Context, header *BlockHeader) error
    ExecuteTx(ctx context.Context, tx *Transaction) *TxExecResult
    EndBlock(ctx context.Context) *EndBlockResult
    Commit(ctx context.Context) *CommitResult

    // State queries (concurrent-safe)
    Query(ctx context.Context, req *QueryRequest) *QueryResponse
}

```text

### Use BaseApplication for Defaults

The `BaseApplication` provides fail-closed defaults:

```go
type MyApp struct {
    *abi.BaseApplication
    state *AppState
}

func NewMyApp() *MyApp {
    return &MyApp{
        BaseApplication: abi.NewBaseApplication(),
        state:          NewAppState(),
    }
}

// Override only the methods you need
func (app *MyApp) CheckTx(ctx context.Context, tx *abi.Transaction) *abi.TxCheckResult {
    // Your validation logic
    return &abi.TxCheckResult{Code: abi.CodeOK}
}

func (app *MyApp) ExecuteTx(ctx context.Context, tx *abi.Transaction) *abi.TxExecResult {
    // Your execution logic
    result := app.state.Apply(tx)
    return &abi.TxExecResult{Code: abi.CodeOK, Events: result.Events}
}

func (app *MyApp) Commit(ctx context.Context) *abi.CommitResult {
    hash, err := app.state.Commit()
    if err != nil {
        return &abi.CommitResult{Error: err}
    }
    return &abi.CommitResult{AppHash: hash}
}

```text

For complete examples, see [ABI_DESIGN.md](ABI_DESIGN.md#part-12-integration-patterns).

---

## Development

### Run Tests

```bash

# Run all unit tests
make test

# Run tests with race detection
make test-race

# Run integration tests
make test-integration

# Run all tests
make test-all

```text

### Lint Code

```bash
make lint

```text

### Run Benchmarks

```bash
make bench

```text

### Generate Coverage Report

```bash
make coverage

```text

### Code Quality Checks

```bash

# Run all checks (format, lint, test)
make check

```text

---

## Project Structure

```text
blockberry/
├── cmd/                  # CLI applications
│   └── blockberry/       # Main CLI tool
├── pkg/                  # Public API (safe to import)
│   ├── abi/              # Application Binary Interface
│   ├── blockstore/       # Block storage
│   ├── config/           # Configuration
│   ├── consensus/        # Consensus interfaces
│   ├── events/           # Event bus
│   ├── indexer/          # Transaction indexing
│   ├── logging/          # Structured logging
│   ├── mempool/          # Transaction mempool
│   ├── metrics/          # Prometheus metrics
│   ├── node/             # Node lifecycle
│   ├── rpc/              # RPC servers
│   ├── statestore/       # State storage (AVL+)
│   ├── tracing/          # OpenTelemetry tracing
│   └── types/            # Common types
├── internal/             # Private implementation (not importable)
│   ├── container/        # Dependency injection
│   ├── handlers/         # P2P message handlers
│   ├── memory/           # Buffer pools
│   ├── p2p/              # Peer management
│   ├── pex/              # Peer exchange
│   ├── security/         # Security utilities
│   └── sync/             # Block synchronization
├── schema/               # Generated code (DO NOT EDIT)
├── examples/             # Example applications
├── test/                 # Integration tests
└── docs/                 # Extended documentation

```text

**Important**: Only packages under `pkg/` are stable public APIs. Code in `internal/` is subject to change without notice.

---

## Dependencies

### Core Dependencies

- [**Glueberry**](https://github.com/blockberries/glueberry) v1.2.10 - P2P networking with encrypted streams
- [**Cramberry**](https://github.com/blockberries/cramberry) v1.5.5 - High-performance binary serialization
- [**Looseberry**](https://github.com/blockberries/looseberry) (optional) - DAG mempool with certified batching
- [**avlberry**](https://github.com/blockberries/avlberry) - AVL+ tree for merkleized state

### Storage

- [**LevelDB**](https://github.com/syndtr/goleveldb) - Production key-value store
- [**BadgerDB**](https://github.com/dgraph-io/badger) v4.9.0 - High-performance embedded database

### Observability

- [**Prometheus**](https://github.com/prometheus/client_golang) v1.22.0 - Metrics collection
- [**OpenTelemetry**](https://go.opentelemetry.io/otel) v1.39.0 - Distributed tracing
- Native Go `slog` - Structured logging

### RPC

- [**gRPC**](https://google.golang.org/grpc) v1.78.0 - High-performance RPC framework
- [**WebSocket**](https://github.com/gobwas/ws) v1.4.0 - Low-latency WebSocket support

For a complete dependency list, see [go.mod](go.mod).

---

## Contributing

We welcome contributions! Please follow these guidelines:

### Contributing Process

1. **Fork the repository**
2. **Create a feature branch** (`git checkout -b feature/amazing-feature`)
3. **Follow [development guidelines](CLAUDE.md)**
4. **Write comprehensive tests** for new functionality
5. **Run tests and linting** (`make check`)
6. **Commit your changes** (`git commit -m 'Add amazing feature'`)
7. **Push to the branch** (`git push origin feature/amazing-feature`)
8. **Open a Pull Request**

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Run `make fmt` before committing
- Ensure `make lint` passes with no warnings
- Write godoc-compatible comments for all exported symbols

### Testing Requirements

- Write table-driven tests where applicable
- Ensure race detection passes (`make test-race`)
- Add integration tests for new features
- Maintain or improve code coverage (aim for 80%+)

### Code Review

- All submissions require code review
- Changes must pass CI checks
- Two approvals required for core packages
- Be responsive to feedback

---

## Versioning

Blockberry uses [Semantic Versioning](https://semver.org/):

- **MAJOR**: Incompatible API changes
- **MINOR**: Backwards-compatible functionality additions
- **PATCH**: Backwards-compatible bug fixes

See [CHANGELOG.md](CHANGELOG.md) for version history.

**Latest Release**: v0.1.7 (2026-01-29)

---

## License

This project is licensed under the **Apache License 2.0** - see the [LICENSE](LICENSE) file for details.

```text
Copyright 2026 Blockberries Project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

```text

---

## Acknowledgments

- [**Tendermint**](https://tendermint.com/) - Inspiration for BFT consensus interfaces
- [**Cosmos SDK**](https://cosmos.network/) - Inspiration for merkleized state patterns
- **The Go Community** - Excellent tooling and libraries

---

## Support

### Documentation

- [Architecture Guide](ARCHITECTURE.md) - System design and patterns
- [API Reference](API_REFERENCE.md) - Complete API documentation
- [Examples](examples/) - Working code samples

### Community

- **GitHub Issues**: [Report bugs or request features](https://github.com/blockberries/blockberry/issues)
- **GitHub Discussions**: [Ask questions and share ideas](https://github.com/blockberries/blockberry/discussions)

### Professional Support

For commercial support, training, or consulting, please contact the Blockberries team.

---

## Roadmap

### Current Focus (v0.2.0)

- Enhanced state sync protocol with chunk verification
- Additional consensus engine implementations (Tendermint BFT, DAG consensus)
- Improved Prometheus dashboards and alerting
- Performance optimizations for high-throughput chains

### Future Plans (v0.3.0+)

- Light client protocol improvements
- Advanced mempool strategies (fee markets, MEV protection)
- Cross-chain communication (IBC integration)
- WebAssembly application support
- Optimistic rollup support

For detailed roadmap, see [GitHub Projects](https://github.com/blockberries/blockberry/projects).

---

## Why Blockberry?

### Key Differentiators

1. **Truly Modular**: Not just pluggable consensus—pluggable everything (mempool, storage, networking)
2. **Production-Ready**: Battle-tested components with comprehensive observability
3. **Secure by Default**: Fail-closed design prevents accidental vulnerabilities
4. **High Performance**: Optimized for blockchain workloads with zero-copy operations
5. **Well-Documented**: Extensive documentation with architecture analysis and design rationale

### When to Use Blockberry

Use Blockberry when you need to:

- Build a custom blockchain with specific consensus requirements
- Experiment with novel consensus algorithms (DAG, PoS variants, hybrid approaches)
- Create application-specific blockchains with optimized transaction ordering
- Deploy permissioned blockchains with custom validator sets
- Build blockchain infrastructure that integrates with existing systems

### When NOT to Use Blockberry

Don't use Blockberry if:

- You need an off-the-shelf blockchain (consider Ethereum, Cosmos SDK, etc.)
- You want a smart contract platform without implementing execution logic
- You need full Ethereum compatibility (use go-ethereum instead)
- You're building a simple database application (not a blockchain use case)

---

**Built with ❤️ by the Blockberries Team**

*Blockberry: The foundation for your blockchain innovation*
