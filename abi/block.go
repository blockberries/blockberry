package abi

import "time"

// BlockHeader contains block metadata passed to BeginBlock.
// This is the information the application receives at the start of block processing.
type BlockHeader struct {
	// Height is the block height (1-indexed).
	Height uint64

	// Time is the block timestamp.
	Time time.Time

	// PrevHash is the hash of the previous block.
	PrevHash []byte

	// ProposerAddress is the address of the block proposer.
	ProposerAddress []byte

	// ConsensusData contains consensus-specific data (opaque to application).
	ConsensusData []byte

	// Evidence contains evidence of validator misbehavior.
	Evidence []Evidence

	// LastValidators contains the validator set from the previous block.
	LastValidators []Validator
}

// Evidence represents evidence of validator misbehavior.
type Evidence struct {
	// Type indicates the type of misbehavior.
	Type EvidenceType

	// Height is the height at which the misbehavior occurred.
	Height uint64

	// Time is when the misbehavior occurred.
	Time time.Time

	// Validator is the misbehaving validator.
	Validator Validator

	// TotalVotingPower is the total voting power at the time of misbehavior.
	TotalVotingPower int64

	// Data contains type-specific evidence data.
	Data []byte
}

// EvidenceType indicates the type of misbehavior.
type EvidenceType int

const (
	// EvidenceUnknown is the default unknown type.
	EvidenceUnknown EvidenceType = iota

	// EvidenceDuplicateVote indicates the validator voted twice in the same round.
	EvidenceDuplicateVote

	// EvidenceLightClientAttack indicates a light client attack.
	EvidenceLightClientAttack
)

// String returns a human-readable description of the evidence type.
func (t EvidenceType) String() string {
	switch t {
	case EvidenceDuplicateVote:
		return "DuplicateVote"
	case EvidenceLightClientAttack:
		return "LightClientAttack"
	default:
		return "Unknown"
	}
}

// EndBlockResult contains the application's response to EndBlock.
type EndBlockResult struct {
	// ValidatorUpdates contains changes to the validator set.
	// Power = 0 means remove the validator.
	ValidatorUpdates []ValidatorUpdate

	// ConsensusParams contains changes to consensus parameters (rare).
	ConsensusParams *ConsensusParams

	// Events are block-level events emitted during EndBlock.
	Events []Event
}

// CommitResult contains the result of committing application state.
type CommitResult struct {
	// AppHash is the new application state root hash.
	AppHash []byte

	// RetainHeight is the suggested minimum height to retain for queries.
	// Heights below this may be pruned.
	RetainHeight uint64
}

// ConsensusParams define consensus rules that can be updated by the application.
type ConsensusParams struct {
	// Block contains block-related parameters.
	Block BlockParams

	// Evidence contains evidence-related parameters.
	Evidence EvidenceParams

	// Validator contains validator-related parameters.
	Validator ValidatorParams
}

// BlockParams define block size limits.
type BlockParams struct {
	// MaxBytes is the maximum block size in bytes.
	MaxBytes int64

	// MaxGas is the maximum gas per block (-1 for unlimited).
	MaxGas int64
}

// EvidenceParams define evidence handling rules.
type EvidenceParams struct {
	// MaxAgeNumBlocks is the maximum age of evidence in blocks.
	MaxAgeNumBlocks int64

	// MaxAgeDuration is the maximum age of evidence in time.
	MaxAgeDuration time.Duration

	// MaxBytes is the maximum total size of evidence in a block.
	MaxBytes int64
}

// ValidatorParams define validator rules.
type ValidatorParams struct {
	// PubKeyTypes are the allowed public key types for validators.
	PubKeyTypes []string
}

// Block represents a complete block with header, transactions, and commit.
type Block struct {
	// Header contains block metadata.
	Header BlockHeader

	// Txs contains the transactions in this block.
	Txs []*Transaction

	// LastCommit contains the commit from the previous block.
	LastCommit *Commit
}

// Commit contains validator signatures for a block.
type Commit struct {
	// Height is the block height.
	Height uint64

	// Round is the consensus round.
	Round uint32

	// BlockHash is the hash of the committed block.
	BlockHash []byte

	// Signatures contains the validator signatures.
	Signatures []CommitSig
}

// CommitSig contains a single validator's commit signature.
type CommitSig struct {
	// Flag indicates the type of signature.
	Flag BlockIDFlag

	// ValidatorAddress is the validator's address.
	ValidatorAddress []byte

	// Timestamp is when the signature was created.
	Timestamp time.Time

	// Signature is the cryptographic signature.
	Signature []byte
}

// BlockIDFlag indicates the type of commit signature.
type BlockIDFlag uint8

const (
	// BlockIDFlagAbsent indicates the validator did not sign.
	BlockIDFlagAbsent BlockIDFlag = iota

	// BlockIDFlagCommit indicates the validator signed the block.
	BlockIDFlagCommit

	// BlockIDFlagNil indicates the validator signed a nil vote.
	BlockIDFlagNil
)

// String returns a human-readable description of the flag.
func (f BlockIDFlag) String() string {
	switch f {
	case BlockIDFlagAbsent:
		return "Absent"
	case BlockIDFlagCommit:
		return "Commit"
	case BlockIDFlagNil:
		return "Nil"
	default:
		return "Unknown"
	}
}
