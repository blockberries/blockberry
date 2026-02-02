package bft

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/consensus"
)

func TestNewHeightVoteSet(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	hvs := NewHeightVoteSet(10, valSet)

	require.NotNil(t, hvs)
	assert.Equal(t, int64(10), hvs.height)
}

func TestHeightVoteSet_AddVote_WrongHeight(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	hvs := NewHeightVoteSet(10, valSet)

	vote := &consensus.Vote{
		Height: 5, // Wrong height
		Round:  0,
		Type:   consensus.VoteTypePrevote,
	}

	added, err := hvs.AddVote(vote)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestHeightVoteSet_AddVote_CorrectHeight(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	hvs := NewHeightVoteSet(10, valSet)

	vote := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 0,
		BlockHash:      []byte{1, 2, 3},
	}

	added, err := hvs.AddVote(vote)
	require.NoError(t, err)
	assert.True(t, added)

	// Verify round was created
	rvs := hvs.GetVotes(0)
	require.NotNil(t, rvs)
}

func TestHeightVoteSet_HasTwoThirdsMajority(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	hvs := NewHeightVoteSet(10, valSet)

	blockHash := []byte{1, 2, 3}

	// Add 3 votes (2f+1 for 4 validators where f=1)
	for i := uint16(0); i < 3; i++ {
		vote := &consensus.Vote{
			Height:         10,
			Round:          0,
			Type:           consensus.VoteTypePrevote,
			ValidatorIndex: i,
			BlockHash:      blockHash,
		}
		_, err := hvs.AddVote(vote)
		require.NoError(t, err)
	}

	hasMajority, hash := hvs.HasTwoThirdsMajority(0)
	assert.True(t, hasMajority)
	assert.Equal(t, blockHash, hash)
}

func TestNewRoundVoteSet(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	require.NotNil(t, rvs)
	assert.Equal(t, int32(0), rvs.round)
	assert.Equal(t, consensus.VoteTypePrevote, rvs.voteType)
	assert.Len(t, rvs.votes, 4)
}

func TestRoundVoteSet_AddVote_WrongType(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	vote := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrecommit, // Wrong type
		ValidatorIndex: 0,
	}

	added, err := rvs.AddVote(vote)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestRoundVoteSet_AddVote_WrongRound(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	vote := &consensus.Vote{
		Height:         10,
		Round:          1, // Wrong round
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 0,
	}

	added, err := rvs.AddVote(vote)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestRoundVoteSet_AddVote_Duplicate(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	vote := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 0,
		BlockHash:      []byte{1, 2, 3},
	}

	// First add should succeed
	added, err := rvs.AddVote(vote)
	require.NoError(t, err)
	assert.True(t, added)

	// Duplicate should not be added
	added, err = rvs.AddVote(vote)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestRoundVoteSet_AddVote_InvalidIndex(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	vote := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 10, // Out of range
		BlockHash:      []byte{1, 2, 3},
	}

	added, err := rvs.AddVote(vote)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestRoundVoteSet_HasTwoThirdsMajority(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	blockHash := []byte{1, 2, 3}

	// Add 2 votes (not enough for 2f+1)
	for i := uint16(0); i < 2; i++ {
		vote := &consensus.Vote{
			Height:         10,
			Round:          0,
			Type:           consensus.VoteTypePrevote,
			ValidatorIndex: i,
			BlockHash:      blockHash,
		}
		_, err := rvs.AddVote(vote)
		require.NoError(t, err)
	}

	hasMajority, _ := rvs.HasTwoThirdsMajority()
	assert.False(t, hasMajority)

	// Add one more vote to reach 2f+1
	vote := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 2,
		BlockHash:      blockHash,
	}
	_, err := rvs.AddVote(vote)
	require.NoError(t, err)

	hasMajority, hash := rvs.HasTwoThirdsMajority()
	assert.True(t, hasMajority)
	assert.Equal(t, blockHash, hash)
}

func TestRoundVoteSet_GetVotesByBlock(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	blockHash1 := []byte{1, 2, 3}
	blockHash2 := []byte{4, 5, 6}

	// Add votes for different blocks
	vote1 := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 0,
		BlockHash:      blockHash1,
	}
	vote2 := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 1,
		BlockHash:      blockHash2,
	}
	vote3 := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 2,
		BlockHash:      blockHash1,
	}

	_, _ = rvs.AddVote(vote1)
	_, _ = rvs.AddVote(vote2)
	_, _ = rvs.AddVote(vote3)

	votes1 := rvs.GetVotesByBlock(blockHash1)
	assert.Len(t, votes1, 2)

	votes2 := rvs.GetVotesByBlock(blockHash2)
	assert.Len(t, votes2, 1)
}

func TestRoundVoteSet_TotalVotingPower(t *testing.T) {
	valSet := &mockValidatorSet{count: 4}
	rvs := NewRoundVoteSet(0, valSet, consensus.VoteTypePrevote)

	assert.Equal(t, int64(0), rvs.TotalVotingPower())

	vote := &consensus.Vote{
		Height:         10,
		Round:          0,
		Type:           consensus.VoteTypePrevote,
		ValidatorIndex: 0,
		BlockHash:      []byte{1, 2, 3},
	}
	_, _ = rvs.AddVote(vote)

	assert.Equal(t, int64(100), rvs.TotalVotingPower())
}

func TestWALMessageType_Constants(t *testing.T) {
	assert.Equal(t, WALMessageType(1), WALMessageTypeProposal)
	assert.Equal(t, WALMessageType(2), WALMessageTypeVote)
	assert.Equal(t, WALMessageType(3), WALMessageTypeEndHeight)
	assert.Equal(t, WALMessageType(4), WALMessageTypeState)
}
