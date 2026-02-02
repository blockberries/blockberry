# Transaction Lifecycle Deep Dive

Complete walkthrough of a transaction's journey through Blockberry.

## Overview

Understand every step of transaction processing from submission to finality.

## Transaction Stages

1. **Submission** - Client submits via RPC
2. **Validation** - CheckTx in mempool
3. **Gossiping** - P2P propagation
4. **Selection** - Inclusion in block proposal
5. **Execution** - ExecuteTx in block
6. **Commitment** - State finalization
7. **Indexing** - Event indexing

## Detailed Flow

See [Application Development](../guides/application-development.md) for implementation details.

## Next Steps

- [ABI Concepts](../concepts/abi.md)
- [P2P Networking](../concepts/p2p-networking.md)
