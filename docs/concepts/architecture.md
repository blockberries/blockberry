# Architecture Overview

Comprehensive overview of Blockberry's system architecture.

## System Design

Blockberry is built on a modular, layered architecture:

1. **Application Layer** - Your blockchain logic
2. **Consensus Layer** - Pluggable consensus engines
3. **Mempool Layer** - Transaction pooling and ordering
4. **Network Layer** - P2P networking (Glueberry)
5. **Storage Layer** - Blocks and state persistence

## Component Architecture

See [Architecture Analysis](../../ARCHITECTURE_ANALYSIS.md) for detailed analysis.

## Design Principles

- **Modularity**: Pluggable components
- **Security**: Fail-closed by default
- **Performance**: Optimized for blockchain workloads
- **Extensibility**: Clear interfaces for customization

## Key Abstractions

- **Application Interface**: ABI v2.0 contract
- **Storage Interfaces**: BlockStore, StateStore
- **Network Streams**: Encrypted P2P channels
- **Consensus Interface**: Pluggable algorithms

## Data Flow

1. Transaction submission
2. Mempool validation
3. P2P gossip
4. Block proposal
5. Consensus agreement
6. Block execution
7. State commitment

## Next Steps

- [ABI Concepts](abi.md)
- [P2P Networking](p2p-networking.md)
- [Storage Layer](storage.md)
