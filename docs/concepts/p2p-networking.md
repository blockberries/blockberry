# P2P Networking

Understanding Blockberry's secure P2P networking layer built on Glueberry.

## Network Architecture

### Stream-Based Communication

Blockberry uses 7 encrypted streams:

- handshake: Connection setup
- pex: Peer exchange
- transactions: Transaction gossip
- blocksync: Historical sync
- blocks: Block propagation
- consensus: Consensus messages
- housekeeping: Health monitoring

### Two-Phase Handshake

1. HelloRequest (unencrypted)
2. HelloResponse with public key
3. Stream preparation (encrypted setup)
4. HelloFinalize (activation)

## Security Features

- Ed25519 key exchange
- Encrypted streams (after handshake)
- Peer reputation scoring
- Rate limiting
- Eclipse attack protection

## Next Steps

- [Network Configuration](../getting-started/configuration.md#network-configuration)
- [Message Protocol](../reference/message-protocol.md)
