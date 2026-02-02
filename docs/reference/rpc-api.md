# RPC API Reference

Complete reference for Blockberry's RPC APIs.

## Supported Protocols

- gRPC
- JSON-RPC 2.0
- WebSocket

## gRPC Methods

### Health
Check node health status.

### Status
Get current node status.

### BroadcastTx
Submit a transaction.

### Query
Query application state.

### Subscribe
Subscribe to events (WebSocket).

## JSON-RPC Methods

All gRPC methods available via JSON-RPC at `/rpc` endpoint.

## WebSocket

Subscribe to real-time events at `/websocket`.

## Authentication

Configure API authentication in config.toml.

## Next Steps

- [Application Development](../guides/application-development.md)
- [Configuration](configuration.md)
