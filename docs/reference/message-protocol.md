# Message Protocol Reference

Complete specification of Blockberry's P2P message protocol.

## Message Format

All messages use Cramberry binary serialization with type-prefixed encoding.

## Message Types

### Handshake Messages (128-130)
- HelloRequest (128)
- HelloResponse (129)
- HelloFinalize (130)

### PEX Messages (131-132)
- AddressRequest (131)
- AddressResponse (132)

### Transaction Messages (133-136)
- TransactionBroadcast (133)
- TransactionRequest (134)
- TransactionResponse (135)

### Block Sync Messages (137-138)
- BlockRequest (137)
- BlockResponse (138)

### Block Messages (139)
- BlockAnnouncement / NewBlock (139)

### Housekeeping Messages (140-143)
- LatencyProbe (140)
- LatencyResponse (141)

### State Sync Messages (144-147)
- (Reserved for state sync protocol)

## Next Steps

- [P2P Networking Concepts](../concepts/p2p-networking.md)
- [API Reference](../../API_REFERENCE.md)
