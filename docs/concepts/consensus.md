# Consensus Layer

Understanding Blockberry's pluggable consensus architecture.

## Consensus Abstraction

Blockberry provides interfaces for implementing any consensus protocol:

- BFT (Byzantine Fault Tolerant)
- DAG (Directed Acyclic Graph)
- PoS (Proof of Stake)
- PoA (Proof of Authority)
- Custom protocols

## ConsensusEngine Interface

See [API Reference](../../API_REFERENCE.md#pkgconsensus) for details.

## Integration Points

- Block production
- Block validation
- Validator set management
- Timeout handling
- Network communication

## Next Steps

- [Consensus Integration Tutorial](../tutorials/consensus-integration.md)
- [Consensus Engines Guide](../guides/consensus-engines.md)
