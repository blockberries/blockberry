package abi

import (
	"context"
	"time"
)

// ProposerApplication extends Application with custom block proposal logic.
// Applications implementing this interface can control what transactions
// are included in proposed blocks and can validate received proposals.
//
// This enables:
// - Transaction reordering, filtering, or injection
// - MEV (Maximal Extractable Value) strategies
// - Application-specific proposal validation
type ProposerApplication interface {
	Application

	// PrepareProposal is called when this validator is the proposer.
	// The application can modify the proposed block by reordering,
	// adding, or removing transactions.
	PrepareProposal(ctx context.Context, req *PrepareRequest) *PrepareResponse

	// ProcessProposal is called when receiving a proposal from another validator.
	// The application should validate the proposal and return Accept or Reject.
	ProcessProposal(ctx context.Context, req *ProcessRequest) *ProcessResponse
}

// PrepareRequest contains information for preparing a block proposal.
type PrepareRequest struct {
	// MaxTxBytes is the maximum total size of transactions in bytes.
	MaxTxBytes int64

	// Txs contains the transactions from the mempool for inclusion.
	// The application may reorder, filter, or add to this list.
	Txs [][]byte

	// LocalLastCommit contains the commit info from the last block.
	LocalLastCommit CommitInfo

	// Misbehavior contains evidence of validator misbehavior to include.
	Misbehavior []Misbehavior

	// Height is the block height being proposed.
	Height int64

	// Time is the proposed block timestamp.
	Time time.Time

	// ProposerAddress is the address of the proposer (this validator).
	ProposerAddress []byte
}

// PrepareResponse contains the application's response to PrepareProposal.
type PrepareResponse struct {
	// Txs is the modified list of transactions to include in the block.
	// The total size must not exceed MaxTxBytes from the request.
	Txs [][]byte
}

// ProcessRequest contains information for validating a received proposal.
type ProcessRequest struct {
	// Txs contains the transactions in the proposed block.
	Txs [][]byte

	// ProposedLastCommit contains the commit info from the previous block.
	ProposedLastCommit CommitInfo

	// Misbehavior contains evidence of validator misbehavior in the proposal.
	Misbehavior []Misbehavior

	// Hash is the proposed block hash.
	Hash []byte

	// Height is the proposed block height.
	Height int64

	// Time is the proposed block timestamp.
	Time time.Time

	// ProposerAddress is the address of the validator who created this proposal.
	ProposerAddress []byte
}

// ProcessResponse contains the application's response to ProcessProposal.
type ProcessResponse struct {
	// Status indicates whether the proposal is accepted or rejected.
	Status ProcessStatus
}

// ProcessStatus indicates the result of proposal processing.
type ProcessStatus uint32

const (
	// ProcessAccept indicates the proposal is valid and should be voted on.
	ProcessAccept ProcessStatus = iota

	// ProcessReject indicates the proposal is invalid and should not be voted on.
	ProcessReject
)

// String returns a human-readable description of the process status.
func (s ProcessStatus) String() string {
	switch s {
	case ProcessAccept:
		return "Accept"
	case ProcessReject:
		return "Reject"
	default:
		return "Unknown"
	}
}

// CommitInfo contains information about validator commit signatures.
// This is used in PrepareProposal and ProcessProposal to provide
// information about the previous block's commit.
type CommitInfo struct {
	// Round is the consensus round of the commit.
	Round uint32

	// Votes contains vote information for each validator.
	Votes []VoteInfo
}

// VoteInfo contains information about a validator's vote.
type VoteInfo struct {
	// Validator identifies the validator.
	Validator Validator

	// BlockIDFlag indicates whether and how the validator voted.
	BlockIDFlag BlockIDFlag
}

// Misbehavior represents evidence of validator misbehavior.
// This is passed to PrepareProposal and ProcessProposal so the application
// can slash misbehaving validators.
type Misbehavior struct {
	// Type indicates the type of misbehavior.
	Type MisbehaviorType

	// Validator is the misbehaving validator.
	Validator Validator

	// Height is the height at which the misbehavior occurred.
	Height int64

	// Time is when the misbehavior was detected.
	Time time.Time

	// TotalVotingPower is the total voting power at the time of misbehavior.
	TotalVotingPower int64
}

// MisbehaviorType indicates the type of validator misbehavior.
type MisbehaviorType int

const (
	// MisbehaviorUnknown is the default unknown type.
	MisbehaviorUnknown MisbehaviorType = iota

	// MisbehaviorDuplicateVote indicates the validator voted twice in the same round.
	MisbehaviorDuplicateVote

	// MisbehaviorLightClientAttack indicates a light client attack.
	MisbehaviorLightClientAttack
)

// String returns a human-readable description of the misbehavior type.
func (t MisbehaviorType) String() string {
	switch t {
	case MisbehaviorDuplicateVote:
		return "DuplicateVote"
	case MisbehaviorLightClientAttack:
		return "LightClientAttack"
	default:
		return "Unknown"
	}
}

// BaseProposerApplication provides default implementations for ProposerApplication.
// It passes through transactions unchanged and accepts all proposals.
// Applications should embed this and override methods as needed.
type BaseProposerApplication struct {
	BaseApplication
}

// PrepareProposal returns transactions unchanged by default.
func (app *BaseProposerApplication) PrepareProposal(ctx context.Context, req *PrepareRequest) *PrepareResponse {
	return &PrepareResponse{Txs: req.Txs}
}

// ProcessProposal accepts all proposals by default.
func (app *BaseProposerApplication) ProcessProposal(ctx context.Context, req *ProcessRequest) *ProcessResponse {
	return &ProcessResponse{Status: ProcessAccept}
}

// Ensure BaseProposerApplication implements ProposerApplication.
var _ ProposerApplication = (*BaseProposerApplication)(nil)
