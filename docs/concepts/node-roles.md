# Node Roles

Understanding different node roles in Blockberry networks.

## Role Types

### Validator
- Participates in consensus
- Proposes and validates blocks
- Stores full state
- Highest resource requirements

### Full Node
- Validates all blocks
- Stores full state
- No consensus participation
- Serves queries

### Seed Node
- Provides peer discovery only
- No block processing
- Minimal storage
- High connection capacity

### Light Client
- Verifies with merkle proofs
- Minimal storage
- Trust assumptions
- Low resource requirements

### Archive Node
- Full node with complete history
- No pruning
- Maximum storage requirements
- Serves historical queries

## Configuration

Set role in config.toml:

```toml
role = "validator"  # or: full, seed, light, archive

```text

## Next Steps

- [Configuration Guide](../getting-started/configuration.md)
- [Deployment Guide](../guides/deployment.md)
