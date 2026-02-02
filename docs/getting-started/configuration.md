# Configuration Guide

This guide explains all configuration options for Blockberry nodes.

## Configuration File

Blockberry uses TOML format for configuration. The default location is `config.toml` in the current directory.

## Loading Configuration

```go
import "github.com/blockberries/blockberry/pkg/config"

// Load from file
cfg, err := config.LoadConfig("config.toml")

// Or start with defaults
cfg := config.DefaultConfig()

// Validate configuration
if err := cfg.Validate(); err != nil {
    log.Fatal(err)
}

```text

## Complete Configuration Reference

### Node Configuration

Controls node identity and protocol settings.

```toml
[node]
chain_id = "my-blockchain-1"  # REQUIRED: Unique blockchain identifier
protocol_version = 1            # Protocol version (must match network)
private_key_path = "node_key.json"  # Ed25519 private key file

```text

**Fields:**

- `chain_id` (string, required): Unique identifier for the blockchain network. Nodes with different chain IDs will reject each other.
- `protocol_version` (int32, required): Protocol version number. Must match across all nodes in the network.
- `private_key_path` (string, required): Path to the node's Ed25519 private key file. Generated automatically if missing.

### Network Configuration

Controls P2P networking behavior.

```toml
[network]
listen_addrs = [
    "/ip4/0.0.0.0/tcp/26656",  # Listen on all IPv4 interfaces
    "/ip6/::/tcp/26656"         # Listen on all IPv6 interfaces
]
max_inbound_peers = 40          # Maximum incoming connections
max_outbound_peers = 10         # Maximum outgoing connections
handshake_timeout = "30s"       # Handshake completion deadline
dial_timeout = "3s"             # Connection attempt timeout
address_book_path = "addrbook.json"  # Peer address persistence

[network.seeds]
addrs = [
    "/dns4/seed1.example.com/tcp/26656/p2p/12D3KooW...",
    "/dns4/seed2.example.com/tcp/26656/p2p/12D3KooW..."
]

```text

**Fields:**

- `listen_addrs` ([]string, required): Multiaddr format listen addresses. Supports IPv4, IPv6, TCP, QUIC protocols.
- `max_inbound_peers` (int, default: 40): Maximum number of accepted incoming connections.
- `max_outbound_peers` (int, default: 10): Maximum number of outgoing connections to initiate.
- `handshake_timeout` (duration, default: 30s): Maximum time for handshake completion before disconnect.
- `dial_timeout` (duration, default: 3s): Maximum time to establish connection.
- `address_book_path` (string, default: "addrbook.json"): File for persisting known peer addresses.
- `seeds.addrs` ([]string, optional): Bootstrap seed node addresses.

### Node Role

Defines the node's role in the network.

```toml
role = "full"  # Options: validator, full, seed, light, archive

```text

**Options:**

- `validator`: Participates in consensus, stores full state
- `full`: Full node without consensus participation
- `seed`: Provides peer discovery only (no transaction/block processing)
- `light`: Light client with minimal storage
- `archive`: Full node with complete historical data (no pruning)

### Handler Configuration

Controls behavior of message handlers.

```toml
[handlers.transactions]
request_interval = "5s"         # How often to request missing transactions
batch_size = 100                # Transactions per request batch
max_pending = 1000              # Maximum pending transaction requests
max_pending_age = "60s"         # Maximum age for pending requests

[handlers.blocks]
max_block_size = 22020096       # 21 MB maximum block size

[handlers.sync]
sync_interval = "5s"            # Block sync check interval
batch_size = 100                # Blocks per sync batch
max_pending_batches = 10        # Maximum concurrent batch requests

```text

### PEX Configuration

Peer exchange protocol settings.

```toml
[pex]
enabled = true                  # Enable peer exchange
request_interval = "30s"        # How often to request new peers
max_addresses_per_response = 100  # Max addresses in single response
max_total_addresses = 1000      # Max addresses in address book

```text

### State Sync Configuration

Fast state synchronization settings.

```toml
[statesync]
enabled = false                 # Enable state sync
trust_height = 1000000          # Trusted snapshot height
trust_hash = "ABC123..."        # Trusted state hash
discovery_interval = "10s"      # Snapshot discovery interval
chunk_request_timeout = "30s"   # Chunk request timeout
max_chunk_retries = 3           # Maximum chunk request retries
snapshot_path = "data/snapshots"  # Snapshot storage directory

```text

**State Sync:** Allows new nodes to quickly sync by downloading state snapshots instead of replaying all blocks.

### Mempool Configuration

Transaction mempool settings.

```toml
[mempool]
type = "simple"                 # Options: simple, priority, ttl, looseberry
max_txs = 5000                  # Maximum transaction count
max_bytes = 1073741824          # 1 GB maximum total size
cache_size = 10000              # Recent transaction cache size

# TTL mempool specific
ttl = "1h"                      # Transaction time-to-live
cleanup_interval = "5m"         # Cleanup check interval

```text

**Mempool Types:**

- `simple`: Basic FIFO mempool
- `priority`: Priority-based ordering (uses GasPrice * GasLimit)
- `ttl`: Time-based expiration
- `looseberry`: DAG-based high-throughput mempool

### Block Store Configuration

Block storage settings.

```toml
[blockstore]
backend = "leveldb"             # Options: leveldb, badgerdb
path = "data/blockstore"        # Storage directory

# Optional: Pruning configuration
[blockstore.pruning]
strategy = "default"            # Options: nothing, everything, default, custom
keep_recent = 1000              # Always keep last N blocks
keep_every = 10000              # Keep every Nth block as checkpoint
interval = "1h"                 # Auto-prune interval

```text

**Backend Options:**

- `leveldb`: Proven, stable storage (requires CGO)
- `badgerdb`: High-performance alternative (pure Go)

### State Store Configuration

Application state storage settings.

```toml
[statestore]
path = "data/state"             # IAVL tree storage directory
cache_size = 10000              # Node cache size (higher = faster, more RAM)

# Optional: Pruning configuration
[statestore.pruning]
keep_recent = 100               # Keep last N versions
prune_interval = "10"           # Prune every N blocks

```text

### Housekeeping Configuration

Network health monitoring settings.

```toml
[housekeeping]
latency_probe_interval = "60s"  # Latency measurement interval

```text

### Metrics Configuration

Prometheus metrics settings.

```toml
[metrics]
enabled = false                 # Enable Prometheus metrics
namespace = "blockberry"        # Metrics namespace
listen_addr = ":9090"           # Metrics HTTP endpoint

```text

**Usage:** When enabled, metrics are exposed at `http://localhost:9090/metrics` for Prometheus scraping.

### Logging Configuration

Structured logging settings.

```toml
[logging]
level = "info"                  # Options: debug, info, warn, error
format = "text"                 # Options: text, json
output = "stderr"               # Options: stdout, stderr, file path

```text

**Log Levels:**

- `debug`: Verbose debugging information
- `info`: Normal operational information
- `warn`: Warning messages (recoverable issues)
- `error`: Error messages (failures)

### Resource Limits

Prevent denial of service attacks.

```toml
[limits]
max_tx_size = 1048576           # 1 MB per transaction
max_block_size = 22020096       # 21 MB per block
max_block_txs = 10000           # Maximum transactions per block
max_msg_size = 10485760         # 10 MB P2P message size
max_subscribers = 1000          # Maximum event subscribers
max_subscribers_per_query = 100 # Max subscribers per query filter

```text

## Environment Variables

Override configuration via environment variables:

```bash

# Set chain ID
export BLOCKBERRY_NODE_CHAIN_ID="my-chain"

# Set log level
export BLOCKBERRY_LOGGING_LEVEL="debug"

# Set listen address
export BLOCKBERRY_NETWORK_LISTEN_ADDRS="/ip4/0.0.0.0/tcp/26656"

```text

Environment variables follow the pattern: `BLOCKBERRY_<SECTION>_<FIELD>` (uppercase, underscores).

## Validation

Configuration is validated when loaded:

```go
cfg, err := config.LoadConfig("config.toml")
if err != nil {
    // Handle validation errors
    switch {
    case errors.Is(err, config.ErrEmptyChainID):
        log.Fatal("Chain ID is required")
    case errors.Is(err, config.ErrNoListenAddrs):
        log.Fatal("At least one listen address required")
    default:
        log.Fatalf("Config error: %v", err)
    }
}

```text

## Common Configurations

### Validator Node

```toml
[node]
chain_id = "mainnet-1"
protocol_version = 1

role = "validator"

[network]
max_inbound_peers = 50          # Higher for validators
max_outbound_peers = 20

[mempool]
type = "priority"               # Priority-based for fee market
max_txs = 10000                 # Larger mempool

[blockstore]
backend = "badgerdb"            # Higher performance

[metrics]
enabled = true                  # Monitor validator health
listen_addr = ":9090"

[logging]
level = "info"
format = "json"                 # Structured logs

```text

### Full Node

```toml
[node]
chain_id = "mainnet-1"
protocol_version = 1

role = "full"

[network]
max_inbound_peers = 40
max_outbound_peers = 10

[blockstore]
backend = "leveldb"

[blockstore.pruning]
strategy = "default"            # Prune old blocks
keep_recent = 1000
interval = "6h"

```text

### Seed Node

```toml
[node]
chain_id = "mainnet-1"
protocol_version = 1

role = "seed"

[network]
max_inbound_peers = 100         # Many connections for discovery
max_outbound_peers = 50

[pex]
enabled = true                  # Peer exchange is primary function
request_interval = "10s"        # Frequent updates
max_total_addresses = 10000     # Large address book

```text

### Archive Node

```toml
[node]
chain_id = "mainnet-1"
protocol_version = 1

role = "archive"

[blockstore]
backend = "badgerdb"

[blockstore.pruning]
strategy = "nothing"            # Never prune (keep all blocks)

[statestore.pruning]
keep_recent = 1000000           # Keep many state versions

```text

## Security Considerations

1. **Private Keys**: Protect `node_key.json` with appropriate file permissions:

   ```bash
   chmod 600 node_key.json

   ```text

2. **Firewall**: Only expose P2P port publicly:

   ```bash

   # Allow P2P
   ufw allow 26656/tcp

   # Keep RPC/metrics private or use authentication
   ufw deny 26657/tcp
   ufw deny 9090/tcp

   ```text

3. **Resource Limits**: Set appropriate limits to prevent DoS attacks

4. **Monitoring**: Enable metrics and monitor for anomalies

## Troubleshooting

### Configuration Not Loading

**Problem:** `error loading config: no such file or directory`

**Solution:** Ensure config.toml exists in current directory or specify path:

```bash
blockberry start --config /path/to/config.toml

```text

### Validation Errors

**Problem:** `chain_id cannot be empty`

**Solution:** Ensure all required fields are set in [node] section.

### Port Already in Use

**Problem:** `bind: address already in use`

**Solution:** Change listen port or stop conflicting process:

```toml
[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26657"]  # Different port

```text

## Next Steps

- [Application Development](../guides/application-development.md) - Build your application
- [Deployment Guide](../guides/deployment.md) - Deploy to production
- [Monitoring Guide](../guides/monitoring.md) - Set up observability

---

For the complete configuration reference with all fields and defaults, see the [API Reference](../../API_REFERENCE.md#configuration-reference).
