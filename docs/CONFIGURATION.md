# Configuration Reference

This document describes all configuration options for blockberry nodes.

## Configuration File Format

Blockberry uses TOML format for configuration files. The configuration is organized into sections.

## Sections

### [node]

Core node settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `chain_id` | string | `"blockberry-1"` | Unique identifier for the blockchain network. Nodes with different chain IDs will reject each other. |
| `protocol_version` | int | `1` | Protocol version. Must match between peers. |
| `private_key_path` | string | `"node_key.json"` | Path to the node's private key file. If the file doesn't exist, a new Ed25519 key will be generated. |

### [network]

Network and peer settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `listen_addrs` | []string | `["/ip4/0.0.0.0/tcp/26656"]` | Multiaddrs to listen on for incoming connections. |
| `max_inbound_peers` | int | `40` | Maximum number of inbound peer connections. |
| `max_outbound_peers` | int | `10` | Maximum number of outbound peer connections. |
| `handshake_timeout` | duration | `"30s"` | Timeout for completing the handshake with a peer. |
| `dial_timeout` | duration | `"3s"` | Timeout for dialing a peer. |
| `address_book_path` | string | `"addrbook.json"` | Path to the peer address book file. |

### [network.seeds]

Seed node configuration for bootstrap.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `addrs` | []string | `[]` | List of seed node multiaddrs. Format: `/ip4/host/tcp/port/p2p/peerID` |

### [pex]

Peer Exchange protocol settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable/disable PEX. |
| `request_interval` | duration | `"30s"` | How often to request new peer addresses from connected peers. |
| `max_addresses_per_response` | int | `100` | Maximum addresses to include in a PEX response. |

### [mempool]

Transaction mempool settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `max_txs` | int | `5000` | Maximum number of transactions in the mempool. |
| `max_bytes` | int | `1073741824` | Maximum total size of transactions in bytes (default 1GB). |
| `cache_size` | int | `10000` | Size of the recent transaction cache (for duplicate detection). |

### [blockstore]

Block storage settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `"data/blocks"` | Directory path for block storage. |

### [statestore]

State storage settings (IAVL tree).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `path` | string | `"data/state"` | Directory path for state storage. |
| `cache_size` | int | `10000` | IAVL node cache size. |

### [housekeeping]

Housekeeping and health check settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `latency_probe_interval` | duration | `"60s"` | How often to probe peer latency. |

## Duration Format

Duration values use Go's duration string format:
- `"30s"` - 30 seconds
- `"5m"` - 5 minutes
- `"1h"` - 1 hour
- `"100ms"` - 100 milliseconds

## Example Configuration

```toml
# Blockberry Node Configuration

[node]
chain_id = "mychain-1"
protocol_version = 1
private_key_path = "node_key.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/26656"]
max_inbound_peers = 40
max_outbound_peers = 10
handshake_timeout = "30s"
dial_timeout = "3s"
address_book_path = "addrbook.json"

[network.seeds]
addrs = [
    "/ip4/seed1.example.com/tcp/26656/p2p/12D3KooWExample1...",
    "/ip4/seed2.example.com/tcp/26656/p2p/12D3KooWExample2...",
]

[pex]
enabled = true
request_interval = "30s"
max_addresses_per_response = 100

[mempool]
max_txs = 5000
max_bytes = 1073741824
cache_size = 10000

[blockstore]
path = "data/blocks"

[statestore]
path = "data/state"
cache_size = 10000

[housekeeping]
latency_probe_interval = "60s"
```

## Programmatic Configuration

You can also configure blockberry programmatically:

```go
import (
    "github.com/blockberries/blockberry/config"
)

func createConfig() *config.Config {
    cfg := config.DefaultConfig()

    // Modify settings
    cfg.Node.ChainID = "mychain-1"
    cfg.Network.MaxInboundPeers = 100
    cfg.Mempool.MaxTxs = 10000

    // Validate
    if err := cfg.Validate(); err != nil {
        log.Fatalf("Invalid config: %v", err)
    }

    return cfg
}
```

## Writing Configuration to File

```go
cfg := config.DefaultConfig()
cfg.Node.ChainID = "mychain-1"

if err := config.WriteConfigFile("config.toml", cfg); err != nil {
    log.Fatalf("Failed to write config: %v", err)
}
```

## Multiaddr Format

Multiaddrs follow the [multiaddr specification](https://multiformats.io/multiaddr/):

```
/ip4/<ip>/tcp/<port>                    # IPv4 TCP
/ip6/<ip>/tcp/<port>                    # IPv6 TCP
/ip4/<ip>/tcp/<port>/p2p/<peerID>       # With peer ID (for seeds)
/dns4/<hostname>/tcp/<port>             # DNS hostname
```

Examples:
- `/ip4/0.0.0.0/tcp/26656` - Listen on all interfaces
- `/ip4/127.0.0.1/tcp/26656` - Localhost only
- `/ip4/192.168.1.100/tcp/26656/p2p/12D3KooWExample...` - Full peer address
