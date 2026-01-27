package bft

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/consensus"
)

func TestDefaultBFTConfig(t *testing.T) {
	cfg := DefaultBFTConfig()
	require.NotNil(t, cfg)

	assert.Equal(t, 3000*time.Millisecond, cfg.TimeoutPropose)
	assert.Equal(t, 1000*time.Millisecond, cfg.TimeoutPrevote)
	assert.Equal(t, 1000*time.Millisecond, cfg.TimeoutPrecommit)
	assert.Equal(t, 1000*time.Millisecond, cfg.TimeoutCommit)
	assert.False(t, cfg.SkipTimeoutCommit)
	assert.True(t, cfg.CreateEmptyBlocks)
}

func TestNewTendermintBFT(t *testing.T) {
	bft := NewTendermintBFT(nil)
	require.NotNil(t, bft)

	assert.Equal(t, "tendermint-bft", bft.Name())
	assert.False(t, bft.IsRunning())
	assert.Equal(t, int64(0), bft.GetHeight())
	assert.Equal(t, int32(0), bft.GetRound())
	assert.False(t, bft.IsValidator())
	assert.Nil(t, bft.ValidatorSet())
}

func TestNewTendermintBFT_WithConfig(t *testing.T) {
	cfg := &BFTConfig{
		TimeoutPropose:   5000 * time.Millisecond,
		TimeoutPrevote:   2000 * time.Millisecond,
		TimeoutPrecommit: 2000 * time.Millisecond,
		TimeoutCommit:    2000 * time.Millisecond,
	}

	bft := NewTendermintBFT(cfg)
	require.NotNil(t, bft)
	assert.Equal(t, cfg.TimeoutPropose, bft.cfg.TimeoutPropose)
}

func TestTendermintBFT_StartStop(t *testing.T) {
	bft := NewTendermintBFT(nil)

	err := bft.Start()
	require.NoError(t, err)
	assert.True(t, bft.IsRunning())

	// Double start should fail
	err = bft.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	err = bft.Stop()
	require.NoError(t, err)
	assert.False(t, bft.IsRunning())

	// Double stop should be safe
	err = bft.Stop()
	require.NoError(t, err)
}

func TestTendermintBFT_Initialize(t *testing.T) {
	bft := NewTendermintBFT(nil)

	deps := consensus.ConsensusDependencies{
		Config: &consensus.ConsensusConfig{
			TimeoutPropose:   5000,
			TimeoutPrevote:   2000,
			TimeoutPrecommit: 2000,
			TimeoutCommit:    2000,
		},
	}

	err := bft.Initialize(deps)
	require.NoError(t, err)

	assert.Equal(t, 5000*time.Millisecond, bft.cfg.TimeoutPropose)
	assert.Equal(t, 2000*time.Millisecond, bft.cfg.TimeoutPrevote)
}

func TestTendermintBFT_ProcessBlock_NilBlock(t *testing.T) {
	bft := NewTendermintBFT(nil)

	err := bft.ProcessBlock(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil block")
}

func TestTendermintBFT_HandleProposal_NilProposal(t *testing.T) {
	bft := NewTendermintBFT(nil)

	err := bft.HandleProposal(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil proposal")
}

func TestTendermintBFT_HandleProposal_PastHeight(t *testing.T) {
	bft := NewTendermintBFT(nil)
	bft.height = 10
	bft.round = 0

	// Past height should be ignored
	proposal := &consensus.Proposal{
		Height: 5,
		Round:  0,
	}

	err := bft.HandleProposal(proposal)
	require.NoError(t, err)
	assert.Nil(t, bft.proposal) // Should not be stored
}

func TestTendermintBFT_HandleProposal_CurrentRound(t *testing.T) {
	bft := NewTendermintBFT(nil)
	bft.height = 10
	bft.round = 1
	bft.step = RoundStepPropose

	proposal := &consensus.Proposal{
		Height: 10,
		Round:  1,
		Block:  &consensus.Block{Height: 10},
	}

	err := bft.HandleProposal(proposal)
	require.NoError(t, err)
	assert.Equal(t, proposal, bft.proposal)
}

func TestTendermintBFT_HandleVote_NilVote(t *testing.T) {
	bft := NewTendermintBFT(nil)

	err := bft.HandleVote(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil vote")
}

func TestTendermintBFT_HandleCommit_NilCommit(t *testing.T) {
	bft := NewTendermintBFT(nil)

	err := bft.HandleCommit(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil commit")
}

func TestTendermintBFT_OnTimeout(t *testing.T) {
	bft := NewTendermintBFT(nil)
	bft.height = 10
	bft.round = 1

	// Stale timeout should be ignored
	err := bft.OnTimeout(5, 0, consensus.TimeoutStepPropose)
	require.NoError(t, err)

	// Current timeout should be processed
	bft.step = RoundStepPropose
	err = bft.OnTimeout(10, 1, consensus.TimeoutStepPropose)
	require.NoError(t, err)
	assert.Equal(t, RoundStepPrevote, bft.step)
}

func TestTendermintBFT_ProduceBlock(t *testing.T) {
	bft := NewTendermintBFT(nil)
	bft.height = 10
	bft.round = 1

	// Correct height
	block, err := bft.ProduceBlock(10)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Equal(t, int64(10), block.Height)
	assert.Equal(t, int32(1), block.Round)

	// Wrong height
	_, err = bft.ProduceBlock(5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot produce block")
}

func TestTendermintBFT_ShouldPropose_NoValidatorSet(t *testing.T) {
	bft := NewTendermintBFT(nil)

	assert.False(t, bft.ShouldPropose(10, 0))
}

func TestTendermintBFT_SetValidatorSet(t *testing.T) {
	bft := NewTendermintBFT(nil)

	valSet := &mockValidatorSet{count: 4}
	bft.SetValidatorSet(valSet)

	assert.Equal(t, valSet, bft.ValidatorSet())
}

func TestTendermintBFT_InterfaceCompliance(t *testing.T) {
	var _ consensus.ConsensusEngine = (*TendermintBFT)(nil)
	var _ consensus.BFTConsensus = (*TendermintBFT)(nil)
	var _ consensus.BlockProducer = (*TendermintBFT)(nil)
}

func TestRoundStep_String(t *testing.T) {
	tests := []struct {
		step     RoundStep
		expected string
	}{
		{RoundStepNewHeight, "NewHeight"},
		{RoundStepNewRound, "NewRound"},
		{RoundStepPropose, "Propose"},
		{RoundStepPrevote, "Prevote"},
		{RoundStepPrevoteWait, "PrevoteWait"},
		{RoundStepPrecommit, "Precommit"},
		{RoundStepPrecommitWait, "PrecommitWait"},
		{RoundStepCommit, "Commit"},
		{RoundStep(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.step.String())
	}
}

// mockValidatorSet for testing
type mockValidatorSet struct {
	count int
}

func (m *mockValidatorSet) Count() int { return m.count }

func (m *mockValidatorSet) GetByIndex(index uint16) *consensus.Validator {
	if int(index) >= m.count {
		return nil
	}
	return &consensus.Validator{Index: index, VotingPower: 100}
}

func (m *mockValidatorSet) GetByAddress(address []byte) *consensus.Validator {
	return nil
}

func (m *mockValidatorSet) Contains(address []byte) bool {
	return false
}

func (m *mockValidatorSet) GetProposer(height int64, round int32) *consensus.Validator {
	if m.count == 0 {
		return nil
	}
	idx := int((height + int64(round))) % m.count
	return &consensus.Validator{Index: uint16(idx), VotingPower: 100}
}

func (m *mockValidatorSet) TotalVotingPower() int64 {
	return int64(m.count * 100)
}

func (m *mockValidatorSet) Quorum() int64 {
	f := (m.count - 1) / 3
	return int64((2*f + 1) * 100)
}

func (m *mockValidatorSet) F() int {
	return (m.count - 1) / 3
}

func (m *mockValidatorSet) Validators() []*consensus.Validator {
	validators := make([]*consensus.Validator, m.count)
	for i := range m.count {
		validators[i] = &consensus.Validator{
			Index:       uint16(i),
			Address:     []byte{byte(i)},
			VotingPower: 100,
		}
	}
	return validators
}

func (m *mockValidatorSet) VerifyCommit(_ int64, _ []byte, _ *consensus.Commit) error {
	return nil
}

var _ consensus.ValidatorSet = (*mockValidatorSet)(nil)
