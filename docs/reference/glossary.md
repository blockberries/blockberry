# Glossary

Definitions of technical terms used in Blockberry documentation.

## Terms

**ABI (Application Binary Interface)**: The contract between the blockchain framework and application logic.

**BadgerDB**: High-performance embedded key-value database alternative to LevelDB.

**Block**: Collection of transactions executed together atomically.

**BlockStore**: Interface for persistent block storage.

**Certificate**: In DAG consensus, a signed vote on transaction batches.

**Consensus**: Agreement protocol for block ordering and validation.

**Cramberry**: Binary serialization library used for P2P messages.

**DAG (Directed Acyclic Graph)**: Graph-based data structure for transaction ordering.

**Eclipse Attack**: Network attack that isolates a node from honest peers.

**Ed25519**: Elliptic curve signature algorithm used for node identity.

**Fail-Closed**: Security model where operations are rejected by default.

**Finality**: Point at which a block cannot be reverted.

**Genesis**: Initial state of the blockchain.

**Glueberry**: P2P networking library providing encrypted streams.

**Height**: Block number in the blockchain.

**IAVL**: Immutable AVL tree used for merkleized state storage.

**Looseberry**: DAG-based mempool library for high-throughput transaction ordering.

**Mempool**: Pool of pending transactions awaiting block inclusion.

**Merkle Proof**: Cryptographic proof of inclusion/exclusion in merkle tree.

**Nonce**: Transaction counter for replay protection.

**PEX (Peer Exchange)**: Protocol for peer discovery.

**Pruning**: Deletion of old blocks/state to save disk space.

**State Sync**: Fast synchronization using state snapshots.

**StateStore**: IAVL-based merkleized state storage.

**Sybil Attack**: Attack using many fake identities.

**Validator**: Node that participates in consensus.

## Next Steps

- [Architecture Overview](../concepts/architecture.md)
- [ABI Concepts](../concepts/abi.md)
