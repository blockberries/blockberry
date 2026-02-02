# Mempool Implementations

Understanding transaction mempool strategies and implementations.

## Mempool Interface

Defines standard operations for transaction pooling.

## Implementations

### SimpleMempool
- FIFO ordering
- Hash-based storage
- Basic deduplication

### PriorityMempool
- Priority queue (heap)
- Gas price ordering
- Sender-based tracking

### TTLMempool
- Time-based expiration
- Background cleanup
- Automatic eviction

### Looseberry
- DAG-based ordering
- High throughput
- Certified batches

## Design Considerations

- Thread safety
- Memory bounds
- Eviction policies
- Transaction validation

## Next Steps

- [Custom Mempool Tutorial](../tutorials/custom-mempool.md)
- [Transaction Lifecycle](../tutorials/transaction-lifecycle.md)
