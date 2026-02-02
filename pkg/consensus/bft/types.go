package bft

import (
	"time"

	"github.com/blockberries/blockberry/pkg/consensus"
)

// PrivValidator is the interface for signing consensus messages.
// Implementations should protect the private key and implement double-signing prevention.
type PrivValidator interface {
	// GetAddress returns the validator's address.
	GetAddress() []byte

	// GetPublicKey returns the validator's public key.
	GetPublicKey() []byte

	// SignVote signs a vote.
	SignVote(vote *consensus.Vote) error

	// SignProposal signs a proposal.
	SignProposal(proposal *consensus.Proposal) error
}

// WAL is the write-ahead log interface for consensus crash recovery.
// The WAL records all consensus messages before they are processed,
// allowing replay after a crash.
type WAL interface {
	// Write writes a message to the WAL.
	Write(msg WALMessage) error

	// WriteSync writes a message and ensures it is synced to disk.
	WriteSync(msg WALMessage) error

	// SearchForEndHeight searches for the end height message.
	// Returns the group reader positioned at the end of that height.
	SearchForEndHeight(height int64) (WALReader, error)

	// Start starts the WAL.
	Start() error

	// Stop stops the WAL.
	Stop() error

	// Flush flushes the WAL to disk.
	Flush() error
}

// WALReader reads messages from the WAL.
type WALReader interface {
	// Decode reads the next message from the WAL.
	Decode() (*WALMessage, error)
}

// WALMessage is a message stored in the WAL.
type WALMessage struct {
	// Type identifies the message type.
	Type WALMessageType

	// Height is the consensus height.
	Height int64

	// Round is the consensus round.
	Round int32

	// Data contains the serialized message.
	Data []byte
}

// WALMessageType identifies the type of WAL message.
type WALMessageType int

// WAL message type constants.
const (
	WALMessageTypeProposal WALMessageType = iota + 1
	WALMessageTypeVote
	WALMessageTypeEndHeight
	WALMessageTypeState
)

// TimeoutTicker manages consensus timeouts.
type TimeoutTicker interface {
	// ScheduleTimeout schedules a timeout.
	ScheduleTimeout(duration time.Duration, height int64, round int32, step consensus.TimeoutStep)

	// Chan returns the channel that receives timeout events.
	Chan() <-chan TimeoutInfo

	// Start starts the timeout ticker.
	Start() error

	// Stop stops the timeout ticker.
	Stop() error
}

// TimeoutInfo describes a timeout event.
type TimeoutInfo struct {
	// Duration is how long the timeout was.
	Duration time.Duration

	// Height is the consensus height.
	Height int64

	// Round is the consensus round.
	Round int32

	// Step is the timeout step.
	Step consensus.TimeoutStep
}

// HeightVoteSet tracks votes for a specific height.
type HeightVoteSet struct {
	height       int64
	validatorSet consensus.ValidatorSet
	rounds       map[int32]*RoundVoteSet
}

// NewHeightVoteSet creates a new HeightVoteSet.
func NewHeightVoteSet(height int64, valSet consensus.ValidatorSet) *HeightVoteSet {
	return &HeightVoteSet{
		height:       height,
		validatorSet: valSet,
		rounds:       make(map[int32]*RoundVoteSet),
	}
}

// AddVote adds a vote to the set.
func (hvs *HeightVoteSet) AddVote(vote *consensus.Vote) (bool, error) {
	if vote.Height != hvs.height {
		return false, nil
	}

	rvs, ok := hvs.rounds[vote.Round]
	if !ok {
		rvs = NewRoundVoteSet(vote.Round, hvs.validatorSet, vote.Type)
		hvs.rounds[vote.Round] = rvs
	}

	return rvs.AddVote(vote)
}

// GetVotes returns all votes for a specific round.
func (hvs *HeightVoteSet) GetVotes(round int32) *RoundVoteSet {
	return hvs.rounds[round]
}

// HasTwoThirdsMajority returns true if there are 2f+1 votes for any block.
func (hvs *HeightVoteSet) HasTwoThirdsMajority(round int32) (bool, []byte) {
	rvs, ok := hvs.rounds[round]
	if !ok {
		return false, nil
	}
	return rvs.HasTwoThirdsMajority()
}

// RoundVoteSet tracks votes for a specific round.
type RoundVoteSet struct {
	round        int32
	validatorSet consensus.ValidatorSet
	voteType     consensus.VoteType

	// votes indexed by validator index
	votes []*consensus.Vote

	// vote counts by block hash
	voteCounts map[string]int64 // block hash -> voting power
}

// NewRoundVoteSet creates a new RoundVoteSet.
func NewRoundVoteSet(round int32, valSet consensus.ValidatorSet, voteType consensus.VoteType) *RoundVoteSet {
	count := 0
	if valSet != nil {
		count = valSet.Count()
	}

	return &RoundVoteSet{
		round:        round,
		validatorSet: valSet,
		voteType:     voteType,
		votes:        make([]*consensus.Vote, count),
		voteCounts:   make(map[string]int64),
	}
}

// AddVote adds a vote to the set.
func (rvs *RoundVoteSet) AddVote(vote *consensus.Vote) (bool, error) {
	if vote.Type != rvs.voteType || vote.Round != rvs.round {
		return false, nil
	}

	idx := int(vote.ValidatorIndex)
	if idx >= len(rvs.votes) {
		return false, nil
	}

	// Check for duplicate
	if rvs.votes[idx] != nil {
		// TODO: Check for equivocation (same validator, different vote)
		return false, nil
	}

	// Get voting power
	var votingPower int64
	if rvs.validatorSet != nil {
		if v := rvs.validatorSet.GetByIndex(vote.ValidatorIndex); v != nil {
			votingPower = v.VotingPower
		}
	}

	// Store vote
	rvs.votes[idx] = vote

	// Update vote count
	key := string(vote.BlockHash)
	rvs.voteCounts[key] += votingPower

	return true, nil
}

// HasTwoThirdsMajority returns true if any block has 2f+1 votes.
func (rvs *RoundVoteSet) HasTwoThirdsMajority() (bool, []byte) {
	if rvs.validatorSet == nil {
		return false, nil
	}

	quorum := rvs.validatorSet.Quorum()

	for hash, power := range rvs.voteCounts {
		if power >= quorum {
			return true, []byte(hash)
		}
	}

	return false, nil
}

// GetVotesByBlock returns all votes for a specific block hash.
func (rvs *RoundVoteSet) GetVotesByBlock(blockHash []byte) []*consensus.Vote {
	var result []*consensus.Vote
	key := string(blockHash)

	for _, vote := range rvs.votes {
		if vote != nil && string(vote.BlockHash) == key {
			result = append(result, vote)
		}
	}

	return result
}

// TotalVotingPower returns the total voting power that has voted.
func (rvs *RoundVoteSet) TotalVotingPower() int64 {
	var total int64
	for _, power := range rvs.voteCounts {
		total += power
	}
	return total
}
