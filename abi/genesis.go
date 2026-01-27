package abi

import "time"

// Genesis contains chain initialization data passed to InitChain.
type Genesis struct {
	// ChainID is the unique identifier for this blockchain.
	ChainID string

	// GenesisTime is the official genesis timestamp.
	GenesisTime time.Time

	// ConsensusParams are the initial consensus parameters.
	ConsensusParams *ConsensusParams

	// Validators are the initial validators.
	Validators []Validator

	// AppState contains application-specific genesis state (opaque to framework).
	AppState []byte

	// InitialHeight is the initial block height (default 1).
	InitialHeight uint64
}

// ApplicationInfo provides metadata about the application.
// This is returned by Application.Info().
type ApplicationInfo struct {
	// Name is the application name.
	Name string

	// Version is the application version string.
	Version string

	// AppHash is the current application state hash.
	AppHash []byte

	// Height is the last committed block height.
	Height uint64

	// LastBlockTime is the timestamp of the last committed block.
	LastBlockTime time.Time

	// ProtocolVersion is the protocol version for compatibility checks.
	ProtocolVersion uint64
}
