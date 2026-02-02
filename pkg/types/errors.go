package types

import (
	"errors"
	"fmt"
)

// WrapMessageError wraps an error with message context (stream name and message type).
func WrapMessageError(err error, stream string, msgType string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s/%s: %w", stream, msgType, err)
}

// WrapUnmarshalError wraps an unmarshal error with message type context.
func WrapUnmarshalError(err error, msgType string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("unmarshal %s: %w", msgType, ErrInvalidMessage)
}

// WrapValidationError wraps a validation error with field context.
func WrapValidationError(err error, field string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("invalid %s: %w", field, err)
}

// Peer-related errors.
var (
	// ErrPeerNotFound is returned when a peer cannot be found.
	ErrPeerNotFound = errors.New("peer not found")

	// ErrPeerBlacklisted is returned when attempting to connect to a blacklisted peer.
	ErrPeerBlacklisted = errors.New("peer is blacklisted")

	// ErrPeerAlreadyConnected is returned when attempting to connect to an already connected peer.
	ErrPeerAlreadyConnected = errors.New("peer already connected")

	// ErrMaxPeersReached is returned when the maximum number of peers has been reached.
	ErrMaxPeersReached = errors.New("maximum peers reached")
)

// Block-related errors.
var (
	// ErrBlockNotFound is returned when a block cannot be found.
	ErrBlockNotFound = errors.New("block not found")

	// ErrBlockAlreadyExists is returned when attempting to store a block that already exists.
	ErrBlockAlreadyExists = errors.New("block already exists")

	// ErrBlockExists is an alias for ErrBlockAlreadyExists.
	ErrBlockExists = ErrBlockAlreadyExists

	// ErrInvalidBlock is returned when a block fails validation.
	ErrInvalidBlock = errors.New("invalid block")

	// ErrInvalidBlockHeight is returned when a block height is invalid.
	ErrInvalidBlockHeight = errors.New("invalid block height")

	// ErrInvalidBlockHash is returned when a block hash is invalid.
	ErrInvalidBlockHash = errors.New("invalid block hash")
)

// Transaction-related errors.
var (
	// ErrTxNotFound is returned when a transaction cannot be found.
	ErrTxNotFound = errors.New("transaction not found")

	// ErrTxAlreadyExists is returned when a transaction already exists in the mempool.
	ErrTxAlreadyExists = errors.New("transaction already exists")

	// ErrInvalidTx is returned when a transaction is invalid.
	ErrInvalidTx = errors.New("invalid transaction")

	// ErrTxTooLarge is returned when a transaction exceeds size limits.
	ErrTxTooLarge = errors.New("transaction too large")
)

// Mempool-related errors.
var (
	// ErrMempoolFull is returned when the mempool has reached capacity.
	ErrMempoolFull = errors.New("mempool is full")

	// ErrMempoolClosed is returned when operations are attempted on a closed mempool.
	ErrMempoolClosed = errors.New("mempool is closed")
)

// Connection and handshake errors.
var (
	// ErrChainIDMismatch is returned when a peer has a different chain ID.
	ErrChainIDMismatch = errors.New("chain ID mismatch")

	// ErrVersionMismatch is returned when a peer has an incompatible protocol version.
	ErrVersionMismatch = errors.New("protocol version mismatch")

	// ErrHandshakeFailed is returned when the handshake process fails.
	ErrHandshakeFailed = errors.New("handshake failed")

	// ErrHandshakeTimeout is returned when the handshake times out.
	ErrHandshakeTimeout = errors.New("handshake timeout")

	// ErrConnectionClosed is returned when a connection has been closed.
	ErrConnectionClosed = errors.New("connection closed")

	// ErrNotConnected is returned when an operation requires a connection that doesn't exist.
	ErrNotConnected = errors.New("not connected")
)

// Message-related errors.
var (
	// ErrInvalidMessage is returned when a message is malformed or invalid.
	ErrInvalidMessage = errors.New("invalid message format")

	// ErrUnknownMessageType is returned when a message type is not recognized.
	ErrUnknownMessageType = errors.New("unknown message type")

	// ErrMessageTooLarge is returned when a message exceeds size limits.
	ErrMessageTooLarge = errors.New("message too large")
)

// State-related errors.
var (
	// ErrKeyNotFound is returned when a key cannot be found in the state store.
	ErrKeyNotFound = errors.New("key not found")

	// ErrStoreClosed is returned when operations are attempted on a closed store.
	ErrStoreClosed = errors.New("store is closed")

	// ErrInvalidProof is returned when a merkle proof is invalid.
	ErrInvalidProof = errors.New("invalid proof")
)

// Sync-related errors.
var (
	// ErrAlreadySyncing is returned when sync is already in progress.
	ErrAlreadySyncing = errors.New("already syncing")

	// ErrNotSyncing is returned when an operation requires sync to be in progress.
	ErrNotSyncing = errors.New("not syncing")

	// ErrSyncFailed is returned when synchronization fails.
	ErrSyncFailed = errors.New("sync failed")

	// ErrNoBlockValidator is returned when attempting to start sync without a block validator.
	// This is a critical safety measure to prevent accepting unvalidated blocks.
	ErrNoBlockValidator = errors.New("block validator is required")

	// ErrNoTxValidator is returned when attempting to add transactions without a validator.
	// This is a critical safety measure to prevent accepting unvalidated transactions.
	ErrNoTxValidator = errors.New("transaction validator is required")

	// ErrNonContiguousBlock is returned when received blocks are not contiguous.
	// This prevents gaps in the block chain and detects misbehaving peers.
	ErrNonContiguousBlock = errors.New("non-contiguous block")
)

// Node lifecycle errors.
var (
	// ErrNodeNotStarted is returned when operations are attempted before the node starts.
	ErrNodeNotStarted = errors.New("node not started")

	// ErrNodeAlreadyStarted is returned when attempting to start an already running node.
	ErrNodeAlreadyStarted = errors.New("node already started")

	// ErrNodeStopped is returned when operations are attempted after the node stops.
	ErrNodeStopped = errors.New("node stopped")
)

// Certificate-related errors (DAG mempool).
var (
	// ErrCertificateNotFound is returned when a certificate cannot be found.
	ErrCertificateNotFound = errors.New("certificate not found")

	// ErrCertificateAlreadyExists is returned when attempting to store a certificate that already exists.
	ErrCertificateAlreadyExists = errors.New("certificate already exists")

	// ErrInvalidCertificate is returned when a certificate fails validation.
	ErrInvalidCertificate = errors.New("invalid certificate")
)

// Batch-related errors (DAG mempool).
var (
	// ErrBatchNotFound is returned when a batch cannot be found.
	ErrBatchNotFound = errors.New("batch not found")

	// ErrBatchAlreadyExists is returned when attempting to store a batch that already exists.
	ErrBatchAlreadyExists = errors.New("batch already exists")

	// ErrInvalidBatch is returned when a batch fails validation.
	ErrInvalidBatch = errors.New("invalid batch")
)
