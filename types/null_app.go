package types

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// NullApplication is a no-op implementation of the Application interface.
// It is useful for testing and as a starting point for new applications.
type NullApplication struct {
	// LastBlockHeight tracks the last committed block height.
	LastBlockHeight int64

	// LastBlockHash stores the last committed block hash.
	LastBlockHash []byte

	// AppHash is the current application hash (returned by Commit).
	AppHash []byte
}

// NewNullApplication creates a new NullApplication.
func NewNullApplication() *NullApplication {
	return &NullApplication{
		AppHash: make([]byte, 32), // Empty 32-byte hash
	}
}

// CheckTx validates a transaction. Always returns nil (accepts all transactions).
func (app *NullApplication) CheckTx(tx []byte) error {
	return nil
}

// BeginBlock starts processing a new block.
func (app *NullApplication) BeginBlock(height int64, hash []byte) error {
	app.LastBlockHeight = height
	app.LastBlockHash = hash
	return nil
}

// DeliverTx processes a transaction. Always returns nil.
func (app *NullApplication) DeliverTx(tx []byte) error {
	return nil
}

// EndBlock ends block processing. Always returns nil.
func (app *NullApplication) EndBlock() error {
	return nil
}

// Commit persists the application state and returns the app hash.
func (app *NullApplication) Commit() ([]byte, error) {
	// In a real application, this would compute a merkle root of state
	return app.AppHash, nil
}

// Query performs a read-only query. Always returns nil.
func (app *NullApplication) Query(path string, data []byte) ([]byte, error) {
	return nil, nil
}

// HandleConsensusMessage processes a consensus message. Always returns nil.
func (app *NullApplication) HandleConsensusMessage(peerID peer.ID, data []byte) error {
	return nil
}

// Ensure NullApplication implements Application.
var _ Application = (*NullApplication)(nil)
