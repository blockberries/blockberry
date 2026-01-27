package mempool

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/types"
)

// ValidatingMempool extends Mempool with explicit validation control.
// This interface is satisfied by all standard mempool implementations.
type ValidatingMempool interface {
	Mempool

	// SetTxValidator sets the transaction validation function.
	// This method is already part of Mempool but is explicitly included here
	// to emphasize that validating mempools require a validator to be set.
	SetTxValidator(validator TxValidator)
}

// DAGMempool is the interface for DAG-based mempools like looseberry.
// DAG mempools organize transactions into a directed acyclic graph where
// certified batches from validators are linked together.
type DAGMempool interface {
	ValidatingMempool
	types.Component

	// ReapCertifiedBatches returns certified transaction batches ready for inclusion.
	// Unlike ReapTxs which returns raw transactions, this returns batches that have
	// been certified by validators in the DAG.
	// maxBytes limits the total size of returned batches.
	ReapCertifiedBatches(maxBytes int64) []CertifiedBatch

	// NotifyCommitted notifies the DAG that consensus has committed a round.
	// This allows the DAG to garbage collect batches that are no longer needed.
	NotifyCommitted(round uint64)

	// UpdateValidatorSet updates the validator set on epoch change.
	// DAG mempools need to know the validator set to verify batch certificates.
	UpdateValidatorSet(validators ValidatorSet)

	// CurrentRound returns the current DAG round.
	CurrentRound() uint64

	// DAGMetrics returns DAG-specific metrics for monitoring.
	DAGMetrics() *DAGMempoolMetrics
}

// NetworkAwareMempool is for mempools that need to register additional P2P streams.
// This is typically used by DAG mempools that have their own protocols for
// batch dissemination and certification.
type NetworkAwareMempool interface {
	Mempool

	// StreamConfigs returns the stream configurations this mempool needs.
	// The node will register these streams with the network layer.
	StreamConfigs() []StreamConfig

	// SetNetwork provides the network layer for custom protocols.
	// Called during node initialization after streams are registered.
	SetNetwork(network MempoolNetwork)
}

// MempoolNetwork is the network interface exposed to mempools.
// This is a subset of the full network functionality needed by mempools.
type MempoolNetwork interface {
	// Send sends data to a specific peer on a stream.
	Send(peerID peer.ID, stream string, data []byte) error

	// Broadcast sends data to all connected peers on a stream.
	Broadcast(stream string, data []byte) error

	// ConnectedPeers returns the list of connected peer IDs.
	ConnectedPeers() []peer.ID
}

// CertifiedBatch represents a batch of transactions that has been certified
// by a validator in a DAG mempool.
type CertifiedBatch struct {
	// Batch is the serialized batch data containing transactions.
	Batch []byte

	// Certificate is the cryptographic proof that this batch is valid.
	// The format depends on the DAG implementation.
	Certificate []byte

	// Round is the DAG round this batch belongs to.
	Round uint64

	// ValidatorIndex identifies which validator created this batch.
	ValidatorIndex uint16

	// Hash is the unique identifier for this batch.
	Hash []byte
}

// Transactions extracts individual transactions from the batch.
// This is a helper for consumers that don't understand the batch format.
func (b *CertifiedBatch) Transactions() [][]byte {
	// This is a placeholder - actual implementation depends on batch format.
	// DAG mempool implementations should override this or provide their own extraction.
	return nil
}

// DAGMempoolMetrics contains metrics specific to DAG mempools.
type DAGMempoolMetrics struct {
	// CurrentRound is the current DAG round.
	CurrentRound uint64

	// PendingBatches is the number of batches waiting to be certified.
	PendingBatches int

	// CertifiedBatches is the number of certified batches ready for inclusion.
	CertifiedBatches int

	// TotalTransactions is the total number of transactions in the DAG.
	TotalTransactions int

	// ValidatorCount is the number of validators in the current set.
	ValidatorCount int

	// BytesPending is the total bytes of pending transactions.
	BytesPending int64

	// BytesCertified is the total bytes of certified batches.
	BytesCertified int64
}

// StreamConfig defines a P2P stream configuration for dynamic registration.
type StreamConfig struct {
	// Name is the unique stream identifier (e.g., "looseberry-batches").
	Name string

	// Encrypted indicates if the stream uses encryption after handshake.
	Encrypted bool

	// RateLimit is the maximum messages per second (0 = unlimited).
	RateLimit int

	// MaxMessageSize is the maximum message size in bytes (0 = default).
	MaxMessageSize int

	// Owner identifies which component owns this stream.
	Owner string
}

// ValidatorSet represents the set of validators for DAG certification.
// This is a simplified interface that DAG mempools use to verify certificates.
type ValidatorSet interface {
	// Count returns the number of validators.
	Count() int

	// GetPublicKey returns the public key for a validator by index.
	// Returns nil if the index is out of range.
	GetPublicKey(index uint16) []byte

	// VerifySignature verifies a signature from a validator.
	// Returns true if the signature is valid.
	VerifySignature(validatorIndex uint16, message, signature []byte) bool

	// Quorum returns the minimum number of validators for consensus (2f+1).
	Quorum() int

	// F returns the maximum Byzantine validators tolerated.
	F() int
}

// StreamHandler is a callback for handling messages on a stream.
type StreamHandler func(peerID peer.ID, data []byte) error

// PrioritizedMempool is an optional interface for priority-based mempools.
// Mempools implementing this interface support transaction prioritization.
// Note: The concrete PriorityMempool type implements this interface.
type PrioritizedMempool interface {
	Mempool

	// AddTxWithPriority adds a transaction with explicit priority.
	// Higher priority transactions are reaped first.
	AddTxWithPriority(tx []byte, priority int64) error

	// GetPriority returns the priority of a transaction.
	// Returns 0 if the transaction is not found.
	GetPriority(hash []byte) int64
}

// ExpirableMempool is an optional interface for time-limited mempools.
// Mempools implementing this interface automatically expire old transactions.
// Note: The concrete TTLMempool type implements this interface.
type ExpirableMempool interface {
	Mempool

	// AddTxWithTTL adds a transaction that expires after the given duration.
	AddTxWithTTL(tx []byte, ttlSeconds int64) error

	// CleanExpired removes all expired transactions.
	// Returns the number of transactions removed.
	CleanExpired() int
}

// MempoolIterator provides iteration over mempool transactions.
type MempoolIterator interface {
	// Next advances to the next transaction.
	// Returns false when there are no more transactions.
	Next() bool

	// Tx returns the current transaction.
	Tx() []byte

	// Hash returns the current transaction hash.
	Hash() []byte

	// Close releases resources held by the iterator.
	Close()
}

// IterableMempool supports iteration over transactions.
type IterableMempool interface {
	Mempool

	// Iterator returns an iterator over all transactions.
	// The caller must call Close() when done.
	Iterator() MempoolIterator
}
