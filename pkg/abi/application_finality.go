package abi

import (
	"context"
	"time"
)

// FinalityApplication extends Application with vote extension and finalization.
// Applications implementing this interface can:
// - Attach custom data to votes (vote extensions)
// - Verify vote extensions from other validators
// - Execute blocks with full finality information
//
// Vote extensions enable use cases like:
// - Oracle data inclusion
// - Threshold cryptography
// - Cross-chain communication
type FinalityApplication interface {
	Application

	// ExtendVote is called when this validator is about to precommit.
	// The application can attach arbitrary data (vote extension) to the vote.
	ExtendVote(ctx context.Context, req *ExtendVoteRequest) *ExtendVoteResponse

	// VerifyVoteExtension verifies a vote extension from another validator.
	// Invalid extensions cause the vote to be rejected.
	VerifyVoteExtension(ctx context.Context, req *VerifyVoteExtRequest) *VerifyVoteExtResponse

	// FinalizeBlock is called to execute a finalized block.
	// This combines BeginBlock + ExecuteTx (for all txs) + EndBlock into one call.
	// It receives full commit info including vote extensions.
	FinalizeBlock(ctx context.Context, req *FinalizeBlockRequest) *FinalizeBlockResponse
}

// ExtendVoteRequest contains information for creating a vote extension.
type ExtendVoteRequest struct {
	// Hash is the hash of the block being voted on.
	Hash []byte

	// Height is the block height.
	Height uint64

	// Time is the block timestamp.
	Time time.Time
}

// ExtendVoteResponse contains the vote extension.
type ExtendVoteResponse struct {
	// VoteExtension is the application-specific data to attach to the vote.
	// This data will be included in the commit and available in FinalizeBlock.
	VoteExtension []byte
}

// VerifyVoteExtRequest contains a vote extension to verify.
type VerifyVoteExtRequest struct {
	// Hash is the hash of the block being voted on.
	Hash []byte

	// ValidatorAddress is the address of the validator who created the extension.
	ValidatorAddress []byte

	// Height is the block height.
	Height uint64

	// VoteExtension is the extension data to verify.
	VoteExtension []byte
}

// VerifyVoteExtResponse contains the verification result.
type VerifyVoteExtResponse struct {
	// Status indicates whether the vote extension is valid.
	Status VerifyStatus
}

// VerifyStatus indicates the result of vote extension verification.
type VerifyStatus uint32

const (
	// VerifyAccept indicates the vote extension is valid.
	VerifyAccept VerifyStatus = iota

	// VerifyReject indicates the vote extension is invalid.
	// The vote will be rejected.
	VerifyReject
)

// String returns a human-readable description of the verify status.
func (s VerifyStatus) String() string {
	switch s {
	case VerifyAccept:
		return "Accept"
	case VerifyReject:
		return "Reject"
	default:
		return "Unknown"
	}
}

// FinalizeBlockRequest contains all information needed to finalize a block.
type FinalizeBlockRequest struct {
	// Txs contains the transactions in this block.
	Txs [][]byte

	// DecidedLastCommit contains full commit info with vote extensions.
	DecidedLastCommit ExtendedCommitInfo

	// Misbehavior contains evidence of validator misbehavior.
	Misbehavior []Misbehavior

	// Hash is the block hash.
	Hash []byte

	// Height is the block height.
	Height uint64

	// Time is the block timestamp.
	Time time.Time

	// ProposerAddress is the address of the block proposer.
	ProposerAddress []byte
}

// FinalizeBlockResponse contains the results of block finalization.
type FinalizeBlockResponse struct {
	// Events are the events emitted during block processing.
	Events []Event

	// TxResults contains the execution result for each transaction.
	TxResults []TxExecResult

	// ValidatorUpdates contains changes to the validator set.
	ValidatorUpdates []ValidatorUpdate

	// ConsensusParams contains changes to consensus parameters.
	ConsensusParams *ConsensusParams

	// AppHash is the new application state root hash.
	AppHash []byte
}

// ExtendedCommitInfo contains commit information including vote extensions.
type ExtendedCommitInfo struct {
	// Round is the consensus round of the commit.
	Round uint32

	// Votes contains extended vote information for each validator.
	Votes []ExtendedVoteInfo
}

// ExtendedVoteInfo contains vote information including the vote extension.
type ExtendedVoteInfo struct {
	// Validator identifies the validator.
	Validator Validator

	// BlockIDFlag indicates whether and how the validator voted.
	BlockIDFlag BlockIDFlag

	// VoteExtension is the application-specific extension data.
	// Empty if the validator did not include an extension.
	VoteExtension []byte

	// ExtensionSignature is the signature over the vote extension.
	// Used for verification.
	ExtensionSignature []byte
}

// BaseFinalityApplication provides default implementations for FinalityApplication.
// It returns empty vote extensions, accepts all extensions, and delegates to
// the standard block execution methods.
// Applications should embed this and override methods as needed.
type BaseFinalityApplication struct {
	BaseApplication
}

// ExtendVote returns an empty vote extension by default.
func (app *BaseFinalityApplication) ExtendVote(ctx context.Context, req *ExtendVoteRequest) *ExtendVoteResponse {
	return &ExtendVoteResponse{VoteExtension: nil}
}

// VerifyVoteExtension accepts all vote extensions by default.
func (app *BaseFinalityApplication) VerifyVoteExtension(ctx context.Context, req *VerifyVoteExtRequest) *VerifyVoteExtResponse {
	return &VerifyVoteExtResponse{Status: VerifyAccept}
}

// FinalizeBlock executes the block using standard methods.
// Override this for custom finalization logic.
func (app *BaseFinalityApplication) FinalizeBlock(ctx context.Context, req *FinalizeBlockRequest) *FinalizeBlockResponse {
	// This is a default implementation that returns empty results.
	// Real implementations should call BeginBlock, ExecuteTx for each tx, and EndBlock.
	return &FinalizeBlockResponse{
		TxResults: make([]TxExecResult, len(req.Txs)),
		AppHash:   nil,
	}
}

// Ensure BaseFinalityApplication implements FinalityApplication.
var _ FinalityApplication = (*BaseFinalityApplication)(nil)
