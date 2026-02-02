# Error Codes Reference

Complete reference for error codes and troubleshooting.

## ABI Result Codes

- CodeOK (0): Success
- CodeInvalidTx (1): Transaction format invalid
- CodeInsufficientFunds (2): Insufficient balance
- CodeNotAuthorized (3): Not authorized
- CodeExecutionFailed (4): Execution failed
- CodeInvalidNonce (5): Invalid nonce
- CodeInvalidSignature (6): Invalid signature

## Sentinel Errors

### Peer Errors
- ErrPeerNotFound
- ErrPeerBlacklisted
- ErrMaxPeersReached

### Block Errors
- ErrBlockNotFound
- ErrBlockAlreadyExists
- ErrInvalidBlock

### Transaction Errors
- ErrTxNotFound
- ErrTxAlreadyExists
- ErrTxTooLarge

### Mempool Errors
- ErrMempoolFull
- ErrMempoolClosed

## Troubleshooting Guide

See [API Reference](../../API_REFERENCE.md#error-reference) for complete error documentation.

## Next Steps

- [Application Development](../guides/application-development.md)
- [API Reference](../../API_REFERENCE.md)
