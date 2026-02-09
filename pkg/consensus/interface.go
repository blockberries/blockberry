// Package consensus provides pluggable consensus engine interfaces for blockberry.
//
// Consensus engines implement these interfaces to integrate with blockberry's
// node framework. The framework supports multiple consensus paradigms:
//
//   - Simple engines that only receive and process blocks (ConsensusEngine)
//   - BFT-style engines with proposals, votes, and commits (BFTConsensus)
//   - Custom protocol engines that need additional network streams (StreamAwareConsensus)
//
// Example usage for a simple consensus engine:
//
//	type MyConsensus struct {
//	    deps ConsensusDependencies
//	    running atomic.Bool
//	}
//
//	func (c *MyConsensus) Initialize(deps ConsensusDependencies) error {
//	    c.deps = deps
//	    return nil
//	}
//
//	func (c *MyConsensus) ProcessBlock(block *Block) error {
//	    // Validate and commit block
//	    return nil
//	}
package consensus

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/pkg/abi"
	"github.com/blockberries/blockberry/pkg/blockstore"
	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/blockberry/pkg/statestore"
	"github.com/blockberries/blockberry/pkg/types"
)

// ConsensusHandler defines the interface for handling raw consensus messages.
// Implement this interface to receive consensus messages from peers via the
// consensus stream. For richer integration, implement ConsensusEngine instead.
type ConsensusHandler interface {
	// HandleConsensusMessage processes an incoming consensus message from a peer.
	HandleConsensusMessage(peerID peer.ID, data []byte) error
}

// ConsensusEngine is the main interface for consensus implementations.
// All consensus engines must implement this interface to integrate with blockberry.
// The engine is responsible for validating and ordering blocks according to its
// consensus algorithm.
type ConsensusEngine interface {
	types.Component
	types.Named

	// Initialize sets up the consensus engine with its dependencies.
	// This is called after construction but before Start().
	// The engine should store references to needed dependencies.
	Initialize(deps ConsensusDependencies) error

	// ProcessBlock is called when a new block is received from a peer.
	// The engine should validate the block according to its rules and
	// potentially commit it to the chain.
	ProcessBlock(block *Block) error

	// GetHeight returns the current consensus height.
	// This is the height of the last committed block.
	GetHeight() int64

	// GetRound returns the current consensus round (if applicable).
	// For non-BFT engines, this may always return 0.
	GetRound() int32

	// IsValidator returns true if this node is a validator.
	// Non-validators only receive and validate blocks, they don't produce them.
	IsValidator() bool

	// ValidatorSet returns the current validator set.
	// Returns nil if the consensus algorithm doesn't use validators.
	ValidatorSet() ValidatorSet
}

// ConsensusDependencies provides access to node components needed by consensus.
// This struct is passed to Initialize() and should be stored by the engine.
type ConsensusDependencies struct {
	// Network provides P2P messaging capabilities.
	Network Network

	// BlockStore provides block storage and retrieval.
	BlockStore blockstore.BlockStore

	// StateStore provides merkleized state storage.
	StateStore *statestore.StateStore

	// Mempool provides transaction management.
	Mempool mempool.Mempool

	// Application is the application-specific logic (ABI v2.0).
	Application abi.Application

	// Callbacks allow consensus to notify the node of events.
	Callbacks *ConsensusCallbacks

	// Config contains consensus-specific configuration.
	Config *ConsensusConfig
}

// ConsensusConfig contains configuration for consensus engines.
type ConsensusConfig struct {
	// Type is the consensus engine type (e.g., "none", "bft", "poa").
	Type string

	// ValidatorKey is the path to the validator's private key.
	ValidatorKey string

	// TimeoutPropose is the timeout for proposing a block.
	TimeoutPropose int64 // milliseconds

	// TimeoutPrevote is the timeout for prevote.
	TimeoutPrevote int64 // milliseconds

	// TimeoutPrecommit is the timeout for precommit.
	TimeoutPrecommit int64 // milliseconds

	// TimeoutCommit is the timeout for commit.
	TimeoutCommit int64 // milliseconds

	// Custom contains engine-specific configuration.
	Custom map[string]any
}

// ConsensusCallbacks allows consensus to notify the node of events.
// These callbacks are invoked by the consensus engine at appropriate times.
type ConsensusCallbacks struct {
	// OnBlockProposed is called when a new block is proposed.
	// Only called for validators that are proposers.
	OnBlockProposed func(block *Block) error

	// OnBlockCommitted is called when a block is finalized and committed.
	// This is the main notification that a new block is ready.
	OnBlockCommitted func(height int64, hash []byte) error

	// OnValidatorSetChanged is called when the validator set changes.
	// This typically happens at epoch boundaries.
	OnValidatorSetChanged func(validators ValidatorSet) error

	// OnStateSync is called when state sync should begin.
	// This is called when the node is too far behind to catch up normally.
	OnStateSync func(targetHeight int64) error

	// OnConsensusError is called when a consensus error occurs.
	// This allows the node to handle errors appropriately.
	OnConsensusError func(err error)
}

// Network provides P2P messaging capabilities for consensus.
// This is a simplified interface focused on what consensus needs.
type Network interface {
	// Send sends a message to a specific peer.
	Send(peerID peer.ID, stream string, data []byte) error

	// Broadcast sends a message to all connected peers.
	Broadcast(stream string, data []byte) error
}

// BlockProducer is implemented by engines that can create blocks.
// This is typically implemented by validators in a BFT system.
type BlockProducer interface {
	// ProduceBlock creates a new block proposal for the given height.
	// The block should include transactions from the mempool.
	ProduceBlock(height int64) (*Block, error)

	// ShouldPropose returns true if this node should propose for the round.
	// This is determined by the consensus algorithm's leader election.
	ShouldPropose(height int64, round int32) bool
}

// BFTConsensus is the interface for BFT-style consensus engines.
// This extends ConsensusEngine with BFT-specific message handling.
type BFTConsensus interface {
	ConsensusEngine
	BlockProducer

	// HandleProposal processes a block proposal from a proposer.
	HandleProposal(proposal *Proposal) error

	// HandleVote processes a consensus vote (prevote or precommit).
	HandleVote(vote *Vote) error

	// HandleCommit processes a commit message with aggregated votes.
	HandleCommit(commit *Commit) error

	// OnTimeout handles consensus timeouts.
	// Different timeout steps trigger different behaviors.
	OnTimeout(height int64, round int32, step TimeoutStep) error
}

// StreamAwareConsensus can register additional network streams.
// Use this when the consensus algorithm needs custom network protocols.
type StreamAwareConsensus interface {
	ConsensusEngine

	// StreamConfigs returns stream configurations this engine needs.
	// The node will register these streams and route messages to HandleStreamMessage.
	StreamConfigs() []StreamConfig

	// HandleStreamMessage handles messages on custom streams.
	// The stream parameter identifies which custom stream the message came from.
	HandleStreamMessage(stream string, peerID peer.ID, data []byte) error
}

// Block represents a consensus block.
// The structure is simplified from the application's block format.
type Block struct {
	// Height is the block height.
	Height int64

	// Round is the consensus round (for BFT).
	Round int32

	// Hash is the block hash.
	Hash []byte

	// ParentHash is the hash of the parent block.
	ParentHash []byte

	// Proposer is the validator index of the proposer.
	Proposer uint16

	// Timestamp is the block timestamp (Unix nanoseconds).
	Timestamp int64

	// Transactions are the transactions in this block.
	Transactions [][]byte

	// Data is the raw block data (application-specific).
	Data []byte

	// LastCommit is the commit for the previous block.
	LastCommit *Commit
}

// Proposal represents a block proposal in BFT consensus.
type Proposal struct {
	// Height is the block height.
	Height int64

	// Round is the consensus round.
	Round int32

	// Block is the proposed block.
	Block *Block

	// POLRound is the proof-of-lock round (-1 if none).
	POLRound int32

	// Signature is the proposer's signature.
	Signature []byte
}

// Vote represents a consensus vote in BFT consensus.
type Vote struct {
	// Type is the vote type (prevote or precommit).
	Type VoteType

	// Height is the block height.
	Height int64

	// Round is the consensus round.
	Round int32

	// BlockHash is the hash of the block being voted on.
	// Empty for nil votes.
	BlockHash []byte

	// ValidatorIndex is the index of the voting validator.
	ValidatorIndex uint16

	// Signature is the validator's signature.
	Signature []byte
}

// VoteType represents the type of consensus vote.
type VoteType int

// Vote type constants.
const (
	VoteTypePrevote   VoteType = 1
	VoteTypePrecommit VoteType = 2
)

// Commit represents aggregated votes for a committed block.
type Commit struct {
	// Height is the block height.
	Height int64

	// Round is the consensus round.
	Round int32

	// BlockHash is the hash of the committed block.
	BlockHash []byte

	// Signatures are the precommit signatures.
	Signatures []CommitSig
}

// CommitSig represents a validator's signature in a commit.
type CommitSig struct {
	// ValidatorIndex is the index of the validator.
	ValidatorIndex uint16

	// Signature is the validator's signature.
	// Empty if the validator didn't sign.
	Signature []byte

	// Timestamp is when the validator signed.
	Timestamp int64
}

// TimeoutStep represents the step in consensus that timed out.
type TimeoutStep int

// Timeout step constants.
const (
	TimeoutStepPropose   TimeoutStep = 1
	TimeoutStepPrevote   TimeoutStep = 2
	TimeoutStepPrecommit TimeoutStep = 3
)

// ValidatorSet represents the set of validators.
type ValidatorSet interface {
	// Count returns the number of validators.
	Count() int

	// GetByIndex returns the validator at the given index.
	GetByIndex(index uint16) *Validator

	// GetByAddress returns the validator with the given address.
	GetByAddress(address []byte) *Validator

	// Contains returns true if the address is a validator.
	Contains(address []byte) bool

	// GetProposer returns the proposer for the given height and round.
	GetProposer(height int64, round int32) *Validator

	// TotalVotingPower returns the total voting power.
	TotalVotingPower() int64

	// Quorum returns the quorum size (2f+1 for BFT).
	Quorum() int64

	// F returns the maximum number of Byzantine validators tolerated.
	F() int

	// Validators returns all validators in the set.
	Validators() []*Validator

	// VerifyCommit verifies a commit against this validator set.
	VerifyCommit(height int64, blockHash []byte, commit *Commit) error
}

// Validator represents a consensus validator.
type Validator struct {
	// Index is the validator's position in the set.
	Index uint16

	// Address is the validator's address (typically derived from public key).
	Address []byte

	// PublicKey is the validator's public key.
	PublicKey []byte

	// VotingPower is the validator's voting power.
	VotingPower int64

	// ProposerPriority is used for round-robin proposer selection.
	ProposerPriority int64
}

// StreamConfig describes a custom network stream for consensus.
type StreamConfig struct {
	// Name is the stream identifier.
	Name string

	// Encrypted indicates whether the stream should be encrypted.
	Encrypted bool

	// RateLimit is the maximum messages per second.
	RateLimit int

	// MaxMessageSize is the maximum message size in bytes.
	MaxMessageSize int
}

// InvokeOnBlockProposed safely invokes the OnBlockProposed callback.
func (c *ConsensusCallbacks) InvokeOnBlockProposed(block *Block) error {
	if c != nil && c.OnBlockProposed != nil {
		return c.OnBlockProposed(block)
	}
	return nil
}

// InvokeOnBlockCommitted safely invokes the OnBlockCommitted callback.
func (c *ConsensusCallbacks) InvokeOnBlockCommitted(height int64, hash []byte) error {
	if c != nil && c.OnBlockCommitted != nil {
		return c.OnBlockCommitted(height, hash)
	}
	return nil
}

// InvokeOnValidatorSetChanged safely invokes the OnValidatorSetChanged callback.
func (c *ConsensusCallbacks) InvokeOnValidatorSetChanged(validators ValidatorSet) error {
	if c != nil && c.OnValidatorSetChanged != nil {
		return c.OnValidatorSetChanged(validators)
	}
	return nil
}

// InvokeOnStateSync safely invokes the OnStateSync callback.
func (c *ConsensusCallbacks) InvokeOnStateSync(targetHeight int64) error {
	if c != nil && c.OnStateSync != nil {
		return c.OnStateSync(targetHeight)
	}
	return nil
}

// InvokeOnConsensusError safely invokes the OnConsensusError callback.
func (c *ConsensusCallbacks) InvokeOnConsensusError(err error) {
	if c != nil && c.OnConsensusError != nil {
		c.OnConsensusError(err)
	}
}
