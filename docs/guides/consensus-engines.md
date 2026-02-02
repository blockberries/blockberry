# Consensus Engines Guide

Guide to implementing consensus protocols with Blockberry.

## Overview

Blockberry provides pluggable consensus through the `ConsensusEngine` interface. This guide covers implementing BFT, DAG, and custom consensus protocols.

## ConsensusEngine Interface

See [API Reference](../../API_REFERENCE.md#pkgconsensus) for interface details.

## Implementation Steps

1. Define consensus state machine
2. Implement ConsensusEngine interface
3. Handle block production and validation
4. Manage validator sets
5. Implement timeout logic
6. Test with multiple nodes

## Example: Simple PoA Consensus

See [Consensus Integration Tutorial](../tutorials/consensus-integration.md).

## Next Steps

- [Consensus Concepts](../concepts/consensus.md)
- [Consensus Integration](../tutorials/consensus-integration.md)
