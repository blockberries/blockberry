// Package rpc provides the RPC server interface and types for blockberry nodes.
package rpc

import (
	"time"

	bapitypes "github.com/blockberries/bapi/types"
)

// NodeStatus contains status information about a node.
type NodeStatus struct {
	// NodeInfo contains node metadata.
	NodeInfo NodeInfo

	// SyncInfo contains synchronization status.
	SyncInfo SyncInfo

	// ValidatorInfo contains validator information (if this node is a validator).
	ValidatorInfo *ValidatorInfo
}

// NodeInfo contains metadata about a node.
type NodeInfo struct {
	// ID is the node's peer ID.
	ID string

	// Moniker is the node's human-readable name.
	Moniker string

	// Network is the chain ID.
	Network string

	// Version is the software version.
	Version string

	// ListenAddr is the address the node listens on.
	ListenAddr string

	// Other contains additional node-specific information.
	Other map[string]string
}

// SyncInfo contains synchronization status.
type SyncInfo struct {
	// LatestBlockHash is the hash of the latest block.
	LatestBlockHash []byte

	// LatestAppHash is the latest application hash.
	LatestAppHash []byte

	// LatestBlockHeight is the height of the latest block.
	LatestBlockHeight int64

	// LatestBlockTime is the timestamp of the latest block.
	LatestBlockTime time.Time

	// EarliestBlockHeight is the earliest available block height.
	EarliestBlockHeight int64

	// EarliestBlockTime is the timestamp of the earliest block.
	EarliestBlockTime time.Time

	// CatchingUp indicates if the node is still syncing.
	CatchingUp bool
}

// ValidatorInfo contains information about this node's validator status.
type ValidatorInfo struct {
	// Address is the validator's address.
	Address []byte

	// PublicKey is the validator's public key.
	PublicKey []byte

	// VotingPower is the validator's current voting power.
	VotingPower int64
}

// HealthStatus represents the health of the RPC server.
type HealthStatus struct {
	// Status is the overall health status.
	Status string

	// Checks contains individual health check results.
	Checks map[string]HealthCheck
}

// HealthCheck represents a single health check result.
type HealthCheck struct {
	// Status is the check status ("pass", "warn", "fail").
	Status string

	// Message provides additional context.
	Message string

	// Time is when the check was performed.
	Time time.Time
}

// BroadcastMode indicates how to broadcast a transaction.
type BroadcastMode int

const (
	// BroadcastSync waits for the transaction to pass CheckTx.
	BroadcastSync BroadcastMode = iota

	// BroadcastAsync returns immediately after queueing.
	BroadcastAsync

	// BroadcastCommit waits for the transaction to be committed (slow).
	BroadcastCommit
)

// String returns a string representation of the broadcast mode.
func (m BroadcastMode) String() string {
	switch m {
	case BroadcastSync:
		return "sync"
	case BroadcastAsync:
		return "async"
	case BroadcastCommit:
		return "commit"
	default:
		return "unknown"
	}
}

// BroadcastResult contains the result of broadcasting a transaction.
type BroadcastResult struct {
	// Code is the result code from CheckTx (0 = success).
	Code uint32

	// Hash is the transaction hash.
	Hash []byte

	// Log contains the CheckTx log.
	Log string

	// Data contains any data from CheckTx.
	Data []byte

	// Height is the block height (only set for BroadcastCommit).
	Height int64
}

// PeerInfo contains information about a connected peer.
type PeerInfo struct {
	// ID is the peer's ID.
	ID string

	// Address is the peer's address.
	Address string

	// IsOutbound indicates if this is an outbound connection.
	IsOutbound bool

	// ConnectionStatus describes the connection state.
	ConnectionStatus string

	// NodeInfo contains the peer's node information.
	NodeInfo *NodeInfo
}

// BlockResult contains a block and its metadata.
type BlockResult struct {
	// Block is the block data.
	Block *Block

	// BlockID contains the block hash.
	BlockID BlockID
}

// Block represents a block in the RPC layer.
type Block struct {
	// Header contains block metadata.
	Header BlockHeader

	// Txs contains the raw transactions in this block.
	Txs [][]byte
}

// BlockHeader contains block metadata for the RPC layer.
type BlockHeader struct {
	// Height is the block height.
	Height int64

	// Time is the block timestamp.
	Time time.Time

	// LastBlockHash is the hash of the previous block.
	LastBlockHash []byte

	// AppHash is the application state root.
	AppHash []byte

	// ValidatorsHash is the merkle root of the validator set.
	ValidatorsHash []byte
}

// BlockID identifies a block.
type BlockID struct {
	// Hash is the block hash.
	Hash []byte

	// PartSetHeader contains part set information.
	PartSetHeader PartSetHeader
}

// PartSetHeader contains part set metadata.
type PartSetHeader struct {
	// Total is the total number of parts.
	Total uint32

	// Hash is the hash of all parts.
	Hash []byte
}

// TxResult contains a transaction and its execution result.
type TxResult struct {
	// Hash is the transaction hash.
	Hash []byte

	// Height is the block height containing this transaction.
	Height int64

	// Index is the transaction's index in the block.
	Index uint32

	// Result is the execution result.
	Result *bapitypes.TxOutcome
}

// ConsensusState contains the current consensus state.
type ConsensusState struct {
	// Height is the current height.
	Height int64

	// Round is the current round.
	Round uint32

	// Step is the current step.
	Step string

	// StartTime is when the current round started.
	StartTime time.Time

	// Validators contains validator information.
	Validators []ValidatorInfo
}

// NetInfo contains network information.
type NetInfo struct {
	// Listening indicates if the node is listening for connections.
	Listening bool

	// Listeners contains the listen addresses.
	Listeners []string

	// NumPeers is the number of connected peers.
	NumPeers int

	// Peers contains peer information.
	Peers []PeerInfo
}
