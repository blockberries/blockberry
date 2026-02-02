# Application Binary Interface (ABI)

Deep dive into Blockberry's ABI v2.0 design and philosophy.

## Design Philosophy

ABI v2.0 uses callback inversion where the framework calls your application at defined lifecycle points.

## Core Concepts

### Lifecycle Methods

- InitChain: Genesis initialization
- CheckTx: Mempool validation
- BeginBlock: Block start
- ExecuteTx: Transaction execution
- EndBlock: Block finalization
- Commit: State persistence

### Thread Safety

- CheckTx and Query: Concurrent-safe required
- Lifecycle methods: Sequential execution guaranteed

### Fail-Closed Security

BaseApplication rejects all operations by default, requiring explicit implementation.

## Next Steps

- [Application Development Guide](../guides/application-development.md)
- [First Application](../getting-started/first-application.md)
