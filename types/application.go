package types

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// Application is the interface that must be implemented by blockchain applications.
// It provides callbacks for transaction validation, block execution, and consensus.
type Application interface {
	// CheckTx validates a transaction before it is added to the mempool.
	// Returns nil if the transaction is valid, an error otherwise.
	CheckTx(tx []byte) error

	// BeginBlock is called at the start of processing a new block.
	// height is the block height, hash is the block hash.
	BeginBlock(height int64, hash []byte) error

	// DeliverTx is called for each transaction in the block.
	// Transactions are delivered in order.
	DeliverTx(tx []byte) error

	// EndBlock is called after all transactions have been delivered.
	EndBlock() error

	// Commit persists the application state and returns the app hash.
	// The app hash is included in the next block header.
	Commit() (appHash []byte, err error)

	// Query performs a read-only query on the application state.
	// path specifies the query type, data contains query parameters.
	Query(path string, data []byte) ([]byte, error)

	// HandleConsensusMessage processes an incoming consensus message from a peer.
	// This is embedded to make Application compatible with ConsensusHandler.
	HandleConsensusMessage(peerID peer.ID, data []byte) error
}

// TxValidator is a function that validates transactions.
// Used by the mempool to check transactions before accepting them.
type TxValidator func(tx []byte) error

// BlockValidator is a function that validates blocks.
// Used by sync and block reactors to verify blocks before storing.
type BlockValidator func(height int64, hash, data []byte) error
