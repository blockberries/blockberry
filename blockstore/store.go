// Package blockstore provides block storage interface and implementations.
package blockstore

import (
	"github.com/blockberries/blockberry/types"
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

// BlockInfo contains metadata about a stored block.
type BlockInfo struct {
	Height int64
	Hash   types.Hash
	Size   int
}
