# CLI Reference

Complete reference for the `blockberry` command-line tool.

## Commands

### blockberry init

Initialize a new node with default configuration.

```bash
blockberry init [flags]

```text

**Flags:**

- `--chain-id <string>`: Chain identifier (required)
- `--config <path>`: Config file path (default: ./config.toml)
- `--home <path>`: Node home directory (default: ./data)

**Example:**

```bash
blockberry init --chain-id my-testnet

```text

### blockberry start

Start the node.

```bash
blockberry start [flags]

```text

**Flags:**

- `--config <path>`: Config file path (default: ./config.toml)

**Example:**

```bash
blockberry start --config config.toml

```text

### blockberry status

Check node status.

```bash
blockberry status [flags]

```text

### blockberry version

Show version information.

```bash
blockberry version

```text

## Global Flags

- `--verbose`: Enable verbose logging
- `--log-level <level>`: Set log level (debug/info/warn/error)

## Environment Variables

- `BLOCKBERRY_CONFIG`: Default config path
- `BLOCKBERRY_HOME`: Default home directory
- `BLOCKBERRY_LOG_LEVEL`: Default log level

## Next Steps

- [Configuration Reference](configuration.md)
- [Getting Started](../getting-started/quickstart.md)
