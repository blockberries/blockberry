# Storage Layer

Understanding Blockberry's storage architecture.

## Storage Components

### BlockStore
Persists blocks with multiple backend options:

- LevelDB: Proven, stable
- BadgerDB: High performance
- Memory: Testing only

### CertificateStore
DAG certificate storage for Looseberry integration.

### StateStore
IAVL merkleized state with:

- Versioned state
- Merkle proofs
- Pruning support

## Design Considerations

- Durability: Atomic writes
- Performance: Batching, caching
- Pruning: Space management
- Concurrency: Thread-safe operations

## Next Steps

- [Custom Storage Backend](../tutorials/storage-backend.md)
- [State Management](state-management.md)
