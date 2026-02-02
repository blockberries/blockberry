# State Management

Deep dive into state management with IAVL merkle trees.

## IAVL Trees

Immutable AVL trees provide:

- Merkle proofs
- Versioned state
- Efficient lookups
- Proof of existence/non-existence

## State Operations

- Get/Set/Delete
- Commit (creates new version)
- LoadVersion (historical queries)
- GetProof (merkle proofs)

## Versioning

Each Commit creates a new state version corresponding to a block height.

## Pruning

Configure pruning to manage disk space while maintaining required state versions.

## Next Steps

- [Application Development](../guides/application-development.md)
- [Storage Concepts](storage.md)
