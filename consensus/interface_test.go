package consensus

import (
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConsensusEngine implements ConsensusEngine for testing.
type mockConsensusEngine struct {
	started     bool
	stopped     bool
	initialized bool
	deps        ConsensusDependencies
	height      int64
	round       int32
	isValidator bool
	valSet      ValidatorSet
}

func (m *mockConsensusEngine) Name() string               { return "mock-consensus" }
func (m *mockConsensusEngine) IsRunning() bool            { return m.started && !m.stopped }
func (m *mockConsensusEngine) Start() error               { m.started = true; return nil }
func (m *mockConsensusEngine) Stop() error                { m.stopped = true; return nil }
func (m *mockConsensusEngine) GetHeight() int64           { return m.height }
func (m *mockConsensusEngine) GetRound() int32            { return m.round }
func (m *mockConsensusEngine) IsValidator() bool          { return m.isValidator }
func (m *mockConsensusEngine) ValidatorSet() ValidatorSet { return m.valSet }

func (m *mockConsensusEngine) Initialize(deps ConsensusDependencies) error {
	m.deps = deps
	m.initialized = true
	return nil
}

func (m *mockConsensusEngine) ProcessBlock(block *Block) error {
	if block == nil {
		return errors.New("nil block")
	}
	m.height = block.Height
	return nil
}

// Verify mockConsensusEngine implements ConsensusEngine.
var _ ConsensusEngine = (*mockConsensusEngine)(nil)

// mockBFTConsensus implements BFTConsensus for testing.
type mockBFTConsensus struct {
	mockConsensusEngine
	proposalReceived *Proposal
	voteReceived     *Vote
	commitReceived   *Commit
	timeoutReceived  bool
	shouldPropose    bool
}

func (m *mockBFTConsensus) HandleProposal(proposal *Proposal) error {
	m.proposalReceived = proposal
	return nil
}

func (m *mockBFTConsensus) HandleVote(vote *Vote) error {
	m.voteReceived = vote
	return nil
}

func (m *mockBFTConsensus) HandleCommit(commit *Commit) error {
	m.commitReceived = commit
	return nil
}

func (m *mockBFTConsensus) OnTimeout(height int64, round int32, step TimeoutStep) error {
	m.timeoutReceived = true
	return nil
}

func (m *mockBFTConsensus) ProduceBlock(height int64) (*Block, error) {
	return &Block{Height: height}, nil
}

func (m *mockBFTConsensus) ShouldPropose(height int64, round int32) bool {
	return m.shouldPropose
}

// Verify mockBFTConsensus implements BFTConsensus.
var _ BFTConsensus = (*mockBFTConsensus)(nil)

// mockStreamAwareConsensus implements StreamAwareConsensus for testing.
type mockStreamAwareConsensus struct {
	mockConsensusEngine
	streamsConfigured []StreamConfig
	messagesReceived  []struct {
		stream string
		peerID peer.ID
		data   []byte
	}
}

func (m *mockStreamAwareConsensus) StreamConfigs() []StreamConfig {
	return m.streamsConfigured
}

func (m *mockStreamAwareConsensus) HandleStreamMessage(stream string, peerID peer.ID, data []byte) error {
	m.messagesReceived = append(m.messagesReceived, struct {
		stream string
		peerID peer.ID
		data   []byte
	}{stream, peerID, data})
	return nil
}

// Verify mockStreamAwareConsensus implements StreamAwareConsensus.
var _ StreamAwareConsensus = (*mockStreamAwareConsensus)(nil)

// mockValidatorSet implements ValidatorSet for testing.
type mockValidatorSet struct {
	validators []*Validator
}

func newMockValidatorSet(count int) *mockValidatorSet {
	validators := make([]*Validator, count)
	for i := range count {
		validators[i] = &Validator{
			Index:            uint16(i),
			Address:          []byte{byte(i)},
			PublicKey:        make([]byte, 32),
			VotingPower:      100,
			ProposerPriority: int64(i),
		}
	}
	return &mockValidatorSet{validators: validators}
}

func (m *mockValidatorSet) Count() int { return len(m.validators) }

func (m *mockValidatorSet) GetByIndex(index uint16) *Validator {
	if int(index) >= len(m.validators) {
		return nil
	}
	return m.validators[index]
}

func (m *mockValidatorSet) GetByAddress(address []byte) *Validator {
	for _, v := range m.validators {
		if len(v.Address) > 0 && len(address) > 0 && v.Address[0] == address[0] {
			return v
		}
	}
	return nil
}

func (m *mockValidatorSet) Contains(address []byte) bool {
	return m.GetByAddress(address) != nil
}

func (m *mockValidatorSet) GetProposer(height int64, round int32) *Validator {
	if len(m.validators) == 0 {
		return nil
	}
	index := int((height + int64(round))) % len(m.validators)
	return m.validators[index]
}

func (m *mockValidatorSet) TotalVotingPower() int64 {
	var total int64
	for _, v := range m.validators {
		total += v.VotingPower
	}
	return total
}

func (m *mockValidatorSet) Quorum() int64 {
	// 2f+1 for BFT
	f := (len(m.validators) - 1) / 3
	return int64(2*f + 1)
}

func (m *mockValidatorSet) VerifyCommit(height int64, blockHash []byte, commit *Commit) error {
	if commit == nil {
		return errors.New("nil commit")
	}
	if commit.Height != height {
		return errors.New("height mismatch")
	}
	return nil
}

// Verify mockValidatorSet implements ValidatorSet.
var _ ValidatorSet = (*mockValidatorSet)(nil)

func TestConsensusEngine_Interface(t *testing.T) {
	engine := &mockConsensusEngine{
		height:      100,
		round:       2,
		isValidator: true,
		valSet:      newMockValidatorSet(4),
	}

	assert.Equal(t, "mock-consensus", engine.Name())
	assert.False(t, engine.IsRunning())

	// Start
	err := engine.Start()
	require.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// Initialize
	deps := ConsensusDependencies{
		Config: &ConsensusConfig{
			Type:           "test",
			TimeoutPropose: 1000,
		},
	}
	err = engine.Initialize(deps)
	require.NoError(t, err)
	assert.True(t, engine.initialized)

	// State accessors
	assert.Equal(t, int64(100), engine.GetHeight())
	assert.Equal(t, int32(2), engine.GetRound())
	assert.True(t, engine.IsValidator())
	assert.NotNil(t, engine.ValidatorSet())

	// Process block
	block := &Block{Height: 101}
	err = engine.ProcessBlock(block)
	require.NoError(t, err)
	assert.Equal(t, int64(101), engine.GetHeight())

	// Stop
	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.IsRunning())
}

func TestConsensusEngine_ProcessBlock_NilBlock(t *testing.T) {
	engine := &mockConsensusEngine{}
	err := engine.ProcessBlock(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil block")
}

func TestBFTConsensus_Interface(t *testing.T) {
	engine := &mockBFTConsensus{
		shouldPropose: true,
	}

	// HandleProposal
	proposal := &Proposal{
		Height: 1,
		Round:  0,
		Block:  &Block{Height: 1},
	}
	err := engine.HandleProposal(proposal)
	require.NoError(t, err)
	assert.Equal(t, proposal, engine.proposalReceived)

	// HandleVote
	vote := &Vote{
		Type:           VoteTypePrevote,
		Height:         1,
		Round:          0,
		BlockHash:      []byte{1, 2, 3},
		ValidatorIndex: 0,
	}
	err = engine.HandleVote(vote)
	require.NoError(t, err)
	assert.Equal(t, vote, engine.voteReceived)

	// HandleCommit
	commit := &Commit{
		Height:    1,
		Round:     0,
		BlockHash: []byte{1, 2, 3},
	}
	err = engine.HandleCommit(commit)
	require.NoError(t, err)
	assert.Equal(t, commit, engine.commitReceived)

	// OnTimeout
	err = engine.OnTimeout(1, 0, TimeoutStepPropose)
	require.NoError(t, err)
	assert.True(t, engine.timeoutReceived)

	// ProduceBlock
	block, err := engine.ProduceBlock(5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), block.Height)

	// ShouldPropose
	assert.True(t, engine.ShouldPropose(1, 0))
}

func TestStreamAwareConsensus_Interface(t *testing.T) {
	engine := &mockStreamAwareConsensus{
		streamsConfigured: []StreamConfig{
			{Name: "test-stream", Encrypted: true, RateLimit: 100, MaxMessageSize: 1024},
		},
	}

	// StreamConfigs
	configs := engine.StreamConfigs()
	require.Len(t, configs, 1)
	assert.Equal(t, "test-stream", configs[0].Name)
	assert.True(t, configs[0].Encrypted)

	// HandleStreamMessage
	peerID := peer.ID("test-peer")
	data := []byte("test message")
	err := engine.HandleStreamMessage("test-stream", peerID, data)
	require.NoError(t, err)
	require.Len(t, engine.messagesReceived, 1)
	assert.Equal(t, "test-stream", engine.messagesReceived[0].stream)
	assert.Equal(t, peerID, engine.messagesReceived[0].peerID)
	assert.Equal(t, data, engine.messagesReceived[0].data)
}

func TestValidatorSet_Interface(t *testing.T) {
	valSet := newMockValidatorSet(4)

	assert.Equal(t, 4, valSet.Count())
	assert.Equal(t, int64(400), valSet.TotalVotingPower())
	assert.Equal(t, int64(3), valSet.Quorum()) // f=1, 2f+1=3 for 4 validators

	// GetByIndex
	val := valSet.GetByIndex(2)
	require.NotNil(t, val)
	assert.Equal(t, uint16(2), val.Index)

	// GetByIndex out of bounds
	val = valSet.GetByIndex(10)
	assert.Nil(t, val)

	// GetByAddress
	val = valSet.GetByAddress([]byte{1})
	require.NotNil(t, val)
	assert.Equal(t, uint16(1), val.Index)

	// GetByAddress not found
	val = valSet.GetByAddress([]byte{99})
	assert.Nil(t, val)

	// Contains
	assert.True(t, valSet.Contains([]byte{0}))
	assert.False(t, valSet.Contains([]byte{99}))

	// GetProposer - deterministic based on height + round
	proposer := valSet.GetProposer(0, 0)
	require.NotNil(t, proposer)
	assert.Equal(t, uint16(0), proposer.Index)

	proposer = valSet.GetProposer(1, 0)
	require.NotNil(t, proposer)
	assert.Equal(t, uint16(1), proposer.Index)

	proposer = valSet.GetProposer(0, 3)
	require.NotNil(t, proposer)
	assert.Equal(t, uint16(3), proposer.Index)

	// VerifyCommit
	err := valSet.VerifyCommit(1, []byte{1, 2, 3}, &Commit{Height: 1})
	assert.NoError(t, err)

	err = valSet.VerifyCommit(1, []byte{1, 2, 3}, nil)
	assert.Error(t, err)

	err = valSet.VerifyCommit(1, []byte{1, 2, 3}, &Commit{Height: 2})
	assert.Error(t, err)
}

func TestConsensusCallbacks_InvokeOnBlockProposed(t *testing.T) {
	var called bool
	var receivedBlock *Block

	callbacks := &ConsensusCallbacks{
		OnBlockProposed: func(block *Block) error {
			called = true
			receivedBlock = block
			return nil
		},
	}

	block := &Block{Height: 1}
	err := callbacks.InvokeOnBlockProposed(block)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, block, receivedBlock)
}

func TestConsensusCallbacks_InvokeOnBlockProposed_NilCallback(t *testing.T) {
	// Nil callbacks struct
	var callbacks *ConsensusCallbacks
	err := callbacks.InvokeOnBlockProposed(&Block{})
	assert.NoError(t, err)

	// Non-nil struct but nil function
	callbacks = &ConsensusCallbacks{}
	err = callbacks.InvokeOnBlockProposed(&Block{})
	assert.NoError(t, err)
}

func TestConsensusCallbacks_InvokeOnBlockProposed_Error(t *testing.T) {
	expectedErr := errors.New("test error")
	callbacks := &ConsensusCallbacks{
		OnBlockProposed: func(block *Block) error {
			return expectedErr
		},
	}

	err := callbacks.InvokeOnBlockProposed(&Block{})
	assert.Equal(t, expectedErr, err)
}

func TestConsensusCallbacks_InvokeOnBlockCommitted(t *testing.T) {
	var called bool
	var receivedHeight int64
	var receivedHash []byte

	callbacks := &ConsensusCallbacks{
		OnBlockCommitted: func(height int64, hash []byte) error {
			called = true
			receivedHeight = height
			receivedHash = hash
			return nil
		},
	}

	err := callbacks.InvokeOnBlockCommitted(100, []byte{1, 2, 3})
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, int64(100), receivedHeight)
	assert.Equal(t, []byte{1, 2, 3}, receivedHash)
}

func TestConsensusCallbacks_InvokeOnBlockCommitted_NilCallback(t *testing.T) {
	var callbacks *ConsensusCallbacks
	err := callbacks.InvokeOnBlockCommitted(100, []byte{1, 2, 3})
	assert.NoError(t, err)

	callbacks = &ConsensusCallbacks{}
	err = callbacks.InvokeOnBlockCommitted(100, []byte{1, 2, 3})
	assert.NoError(t, err)
}

func TestConsensusCallbacks_InvokeOnValidatorSetChanged(t *testing.T) {
	var called bool
	var receivedSet ValidatorSet

	callbacks := &ConsensusCallbacks{
		OnValidatorSetChanged: func(validators ValidatorSet) error {
			called = true
			receivedSet = validators
			return nil
		},
	}

	valSet := newMockValidatorSet(4)
	err := callbacks.InvokeOnValidatorSetChanged(valSet)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, valSet, receivedSet)
}

func TestConsensusCallbacks_InvokeOnValidatorSetChanged_NilCallback(t *testing.T) {
	var callbacks *ConsensusCallbacks
	err := callbacks.InvokeOnValidatorSetChanged(newMockValidatorSet(4))
	assert.NoError(t, err)

	callbacks = &ConsensusCallbacks{}
	err = callbacks.InvokeOnValidatorSetChanged(newMockValidatorSet(4))
	assert.NoError(t, err)
}

func TestConsensusCallbacks_InvokeOnStateSync(t *testing.T) {
	var called bool
	var receivedHeight int64

	callbacks := &ConsensusCallbacks{
		OnStateSync: func(targetHeight int64) error {
			called = true
			receivedHeight = targetHeight
			return nil
		},
	}

	err := callbacks.InvokeOnStateSync(1000)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, int64(1000), receivedHeight)
}

func TestConsensusCallbacks_InvokeOnStateSync_NilCallback(t *testing.T) {
	var callbacks *ConsensusCallbacks
	err := callbacks.InvokeOnStateSync(1000)
	assert.NoError(t, err)

	callbacks = &ConsensusCallbacks{}
	err = callbacks.InvokeOnStateSync(1000)
	assert.NoError(t, err)
}

func TestConsensusCallbacks_InvokeOnConsensusError(t *testing.T) {
	var called bool
	var receivedErr error

	callbacks := &ConsensusCallbacks{
		OnConsensusError: func(err error) {
			called = true
			receivedErr = err
		},
	}

	testErr := errors.New("consensus error")
	callbacks.InvokeOnConsensusError(testErr)
	assert.True(t, called)
	assert.Equal(t, testErr, receivedErr)
}

func TestConsensusCallbacks_InvokeOnConsensusError_NilCallback(t *testing.T) {
	// Should not panic
	var callbacks *ConsensusCallbacks
	callbacks.InvokeOnConsensusError(errors.New("test"))

	callbacks = &ConsensusCallbacks{}
	callbacks.InvokeOnConsensusError(errors.New("test"))
}

func TestVoteType_Constants(t *testing.T) {
	assert.Equal(t, VoteType(1), VoteTypePrevote)
	assert.Equal(t, VoteType(2), VoteTypePrecommit)
}

func TestTimeoutStep_Constants(t *testing.T) {
	assert.Equal(t, TimeoutStep(1), TimeoutStepPropose)
	assert.Equal(t, TimeoutStep(2), TimeoutStepPrevote)
	assert.Equal(t, TimeoutStep(3), TimeoutStepPrecommit)
}

func TestBlock_Structure(t *testing.T) {
	block := &Block{
		Height:       100,
		Round:        1,
		Hash:         []byte{1, 2, 3},
		ParentHash:   []byte{4, 5, 6},
		Proposer:     5,
		Timestamp:    1234567890,
		Transactions: [][]byte{{1}, {2}, {3}},
		Data:         []byte("app data"),
		LastCommit: &Commit{
			Height:    99,
			Round:     0,
			BlockHash: []byte{4, 5, 6},
		},
	}

	assert.Equal(t, int64(100), block.Height)
	assert.Equal(t, int32(1), block.Round)
	assert.Equal(t, []byte{1, 2, 3}, block.Hash)
	assert.Equal(t, []byte{4, 5, 6}, block.ParentHash)
	assert.Equal(t, uint16(5), block.Proposer)
	assert.Equal(t, int64(1234567890), block.Timestamp)
	assert.Len(t, block.Transactions, 3)
	assert.Equal(t, []byte("app data"), block.Data)
	require.NotNil(t, block.LastCommit)
	assert.Equal(t, int64(99), block.LastCommit.Height)
}

func TestProposal_Structure(t *testing.T) {
	proposal := &Proposal{
		Height:    100,
		Round:     1,
		Block:     &Block{Height: 100},
		POLRound:  -1,
		Signature: []byte{1, 2, 3, 4},
	}

	assert.Equal(t, int64(100), proposal.Height)
	assert.Equal(t, int32(1), proposal.Round)
	assert.NotNil(t, proposal.Block)
	assert.Equal(t, int32(-1), proposal.POLRound)
	assert.Equal(t, []byte{1, 2, 3, 4}, proposal.Signature)
}

func TestVote_Structure(t *testing.T) {
	vote := &Vote{
		Type:           VoteTypePrevote,
		Height:         100,
		Round:          1,
		BlockHash:      []byte{1, 2, 3},
		ValidatorIndex: 5,
		Signature:      []byte{4, 5, 6},
	}

	assert.Equal(t, VoteTypePrevote, vote.Type)
	assert.Equal(t, int64(100), vote.Height)
	assert.Equal(t, int32(1), vote.Round)
	assert.Equal(t, []byte{1, 2, 3}, vote.BlockHash)
	assert.Equal(t, uint16(5), vote.ValidatorIndex)
	assert.Equal(t, []byte{4, 5, 6}, vote.Signature)
}

func TestCommit_Structure(t *testing.T) {
	commit := &Commit{
		Height:    100,
		Round:     1,
		BlockHash: []byte{1, 2, 3},
		Signatures: []CommitSig{
			{ValidatorIndex: 0, Signature: []byte{1}, Timestamp: 1000},
			{ValidatorIndex: 1, Signature: []byte{2}, Timestamp: 1001},
		},
	}

	assert.Equal(t, int64(100), commit.Height)
	assert.Equal(t, int32(1), commit.Round)
	assert.Equal(t, []byte{1, 2, 3}, commit.BlockHash)
	require.Len(t, commit.Signatures, 2)
	assert.Equal(t, uint16(0), commit.Signatures[0].ValidatorIndex)
	assert.Equal(t, uint16(1), commit.Signatures[1].ValidatorIndex)
}

func TestCommitSig_Structure(t *testing.T) {
	sig := CommitSig{
		ValidatorIndex: 5,
		Signature:      []byte{1, 2, 3, 4},
		Timestamp:      1234567890,
	}

	assert.Equal(t, uint16(5), sig.ValidatorIndex)
	assert.Equal(t, []byte{1, 2, 3, 4}, sig.Signature)
	assert.Equal(t, int64(1234567890), sig.Timestamp)
}

func TestValidator_Structure(t *testing.T) {
	validator := &Validator{
		Index:            5,
		Address:          []byte{1, 2, 3},
		PublicKey:        []byte{4, 5, 6, 7, 8},
		VotingPower:      1000,
		ProposerPriority: -50,
	}

	assert.Equal(t, uint16(5), validator.Index)
	assert.Equal(t, []byte{1, 2, 3}, validator.Address)
	assert.Equal(t, []byte{4, 5, 6, 7, 8}, validator.PublicKey)
	assert.Equal(t, int64(1000), validator.VotingPower)
	assert.Equal(t, int64(-50), validator.ProposerPriority)
}

func TestStreamConfig_Structure(t *testing.T) {
	config := StreamConfig{
		Name:           "consensus-votes",
		Encrypted:      true,
		RateLimit:      100,
		MaxMessageSize: 65536,
	}

	assert.Equal(t, "consensus-votes", config.Name)
	assert.True(t, config.Encrypted)
	assert.Equal(t, 100, config.RateLimit)
	assert.Equal(t, 65536, config.MaxMessageSize)
}

func TestConsensusConfig_Structure(t *testing.T) {
	config := &ConsensusConfig{
		Type:             "bft",
		ValidatorKey:     "/path/to/key",
		TimeoutPropose:   3000,
		TimeoutPrevote:   1000,
		TimeoutPrecommit: 1000,
		TimeoutCommit:    1000,
		Custom: map[string]any{
			"custom_param": 42,
			"enabled":      true,
		},
	}

	assert.Equal(t, "bft", config.Type)
	assert.Equal(t, "/path/to/key", config.ValidatorKey)
	assert.Equal(t, int64(3000), config.TimeoutPropose)
	assert.Equal(t, int64(1000), config.TimeoutPrevote)
	assert.Equal(t, int64(1000), config.TimeoutPrecommit)
	assert.Equal(t, int64(1000), config.TimeoutCommit)
	assert.Equal(t, 42, config.Custom["custom_param"])
	assert.Equal(t, true, config.Custom["enabled"])
}

func TestConsensusDependencies_Structure(t *testing.T) {
	deps := ConsensusDependencies{
		Config: &ConsensusConfig{Type: "test"},
		Callbacks: &ConsensusCallbacks{
			OnBlockCommitted: func(height int64, hash []byte) error {
				return nil
			},
		},
	}

	assert.NotNil(t, deps.Config)
	assert.Equal(t, "test", deps.Config.Type)
	assert.NotNil(t, deps.Callbacks)
	assert.NotNil(t, deps.Callbacks.OnBlockCommitted)
}
