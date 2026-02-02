// Package blockstore provides block storage interface and implementations.
package blockstore

import (
	"github.com/blockberries/blockberry/pkg/types"
	loosetypes "github.com/blockberries/looseberry/types"
)

// BlockStore defines the interface for block persistence.
// Implementations must be safe for concurrent use.
type BlockStore interface {
	// SaveBlock persists a block at the given height with its hash and data.
	// Returns an error if the block already exists at that height.
	SaveBlock(height int64, hash []byte, data []byte) error

	// LoadBlock retrieves a block by height.
	// Returns the block hash, data, and any error.
	// Returns types.ErrBlockNotFound if the block does not exist.
	LoadBlock(height int64) (hash []byte, data []byte, err error)

	// LoadBlockByHash retrieves a block by its hash.
	// Returns the block height, data, and any error.
	// Returns types.ErrBlockNotFound if the block does not exist.
	LoadBlockByHash(hash []byte) (height int64, data []byte, err error)

	// HasBlock checks if a block exists at the given height.
	HasBlock(height int64) bool

	// Height returns the latest block height.
	// Returns 0 if no blocks have been stored.
	Height() int64

	// Base returns the earliest available block height.
	// Returns 0 if no blocks have been stored.
	// For pruned stores, this may be greater than 1.
	Base() int64

	// Close closes the store and releases resources.
	Close() error
}

// CertificateStore defines the interface for DAG certificate and batch persistence.
// This interface is used by Looseberry-integrated BlockStores to store
// certificates and batches from the DAG mempool.
// Implementations must be safe for concurrent use.
type CertificateStore interface {
	// SaveCertificate persists a DAG certificate.
	// The certificate is indexed by its digest, round, and optionally block height.
	// Returns an error if the certificate already exists.
	SaveCertificate(cert *loosetypes.Certificate) error

	// GetCertificate retrieves a certificate by its digest.
	// Returns types.ErrCertificateNotFound if the certificate does not exist.
	GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error)

	// GetCertificatesForRound retrieves all certificates for a given DAG round.
	// Returns an empty slice if no certificates exist for the round.
	// Certificates are returned in validator index order.
	GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error)

	// GetCertificatesForHeight retrieves all certificates committed at a given block height.
	// Returns an empty slice if no certificates exist for the height.
	// Certificates are returned in validator index order.
	GetCertificatesForHeight(height int64) ([]*loosetypes.Certificate, error)

	// SaveBatch persists a transaction batch.
	// The batch is indexed by its digest.
	// Returns an error if the batch already exists.
	SaveBatch(batch *loosetypes.Batch) error

	// GetBatch retrieves a batch by its digest.
	// Returns types.ErrBatchNotFound if the batch does not exist.
	GetBatch(digest loosetypes.Hash) (*loosetypes.Batch, error)

	// SetCertificateBlockHeight updates the block height index for a certificate.
	// This is called when a certificate is committed in a block.
	SetCertificateBlockHeight(digest loosetypes.Hash, height int64) error
}

// CertificateBlockStore combines BlockStore and CertificateStore interfaces.
// This is the primary interface for stores that support both block and
// certificate storage, used by validators running Looseberry.
type CertificateBlockStore interface {
	BlockStore
	CertificateStore
}

// BlockInfo contains metadata about a stored block.
type BlockInfo struct {
	Height int64
	Hash   types.Hash
	Size   int
}
