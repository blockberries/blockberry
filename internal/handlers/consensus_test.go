package handlers

import (
	"encoding/binary"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/consensus"
	"github.com/blockberries/blockberry/pkg/types"
)

// mockConsensusHandler implements ConsensusHandler for testing.
type mockConsensusHandler struct {
	messages []mockConsensusMessage
}

type mockConsensusMessage struct {
	peerID peer.ID
	data   []byte
}

func (h *mockConsensusHandler) HandleConsensusMessage(peerID peer.ID, data []byte) error {
	h.messages = append(h.messages, mockConsensusMessage{
		peerID: peerID,
		data:   data,
	})
	return nil
}

type errorConsensusHandler struct{}

func (h *errorConsensusHandler) HandleConsensusMessage(peerID peer.ID, data []byte) error {
	return types.ErrInvalidMessage
}

func TestNewConsensusReactor(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	require.NotNil(t, reactor)
	require.Nil(t, reactor.GetHandler())
}

func TestConsensusReactor_SetHandler(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)
	handler := &mockConsensusHandler{}

	reactor.SetHandler(handler)
	require.Equal(t, handler, reactor.GetHandler())

	reactor.SetHandler(nil)
	require.Nil(t, reactor.GetHandler())
}

func TestConsensusReactor_HandleMessageEmpty(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	err := reactor.HandleMessage(peer.ID("peer1"), nil)
	require.ErrorIs(t, err, types.ErrInvalidMessage)

	err = reactor.HandleMessage(peer.ID("peer1"), []byte{})
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestConsensusReactor_HandleMessageNoHandler(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	// Should not error when no handler is set
	err := reactor.HandleMessage(peer.ID("peer1"), []byte("consensus data"))
	require.NoError(t, err)
}

func TestConsensusReactor_HandleMessageWithHandler(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)
	handler := &mockConsensusHandler{}
	reactor.SetHandler(handler)

	peerID := peer.ID("peer1")
	data := []byte("consensus message data")

	err := reactor.HandleMessage(peerID, data)
	require.NoError(t, err)

	// Handler should have received the message
	require.Len(t, handler.messages, 1)
	require.Equal(t, peerID, handler.messages[0].peerID)
	require.Equal(t, data, handler.messages[0].data)
}

func TestConsensusReactor_HandleMessageMultiple(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)
	handler := &mockConsensusHandler{}
	reactor.SetHandler(handler)

	// Send multiple messages
	for i := 0; i < 5; i++ {
		peerID := peer.ID("peer1")
		data := []byte{byte(i)}
		err := reactor.HandleMessage(peerID, data)
		require.NoError(t, err)
	}

	require.Len(t, handler.messages, 5)
}

func TestConsensusReactor_HandleMessageHandlerError(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)
	handler := &errorConsensusHandler{}
	reactor.SetHandler(handler)

	err := reactor.HandleMessage(peer.ID("peer1"), []byte("data"))
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestConsensusReactor_SendConsensusMessageNilNetwork(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	// Should not error with nil network
	err := reactor.SendConsensusMessage(peer.ID("peer1"), []byte("data"))
	require.NoError(t, err)
}

func TestConsensusReactor_BroadcastConsensusMessageNilDependencies(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	// Should not error with nil dependencies
	err := reactor.BroadcastConsensusMessage([]byte("data"))
	require.NoError(t, err)
}

func TestConsensusReactor_OnPeerDisconnected(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	// Should not panic
	reactor.OnPeerDisconnected(peer.ID("peer1"))
}

// mockConsensusEngine implements consensus.ConsensusEngine for testing.
type mockConsensusEngine struct {
	blocksProcessed []*consensus.Block
	running         bool
}

func (e *mockConsensusEngine) Name() string                         { return "mock-engine" }
func (e *mockConsensusEngine) IsRunning() bool                      { return e.running }
func (e *mockConsensusEngine) Start() error                         { e.running = true; return nil }
func (e *mockConsensusEngine) Stop() error                          { e.running = false; return nil }
func (e *mockConsensusEngine) GetHeight() int64                     { return 0 }
func (e *mockConsensusEngine) GetRound() int32                      { return 0 }
func (e *mockConsensusEngine) IsValidator() bool                    { return false }
func (e *mockConsensusEngine) ValidatorSet() consensus.ValidatorSet { return nil }

func (e *mockConsensusEngine) Initialize(deps consensus.ConsensusDependencies) error {
	return nil
}

func (e *mockConsensusEngine) ProcessBlock(block *consensus.Block) error {
	e.blocksProcessed = append(e.blocksProcessed, block)
	return nil
}

// mockBFTConsensusEngine implements consensus.BFTConsensus for testing.
type mockBFTConsensusEngine struct {
	mockConsensusEngine
	proposals []*consensus.Proposal
	votes     []*consensus.Vote
	commits   []*consensus.Commit
}

func (e *mockBFTConsensusEngine) HandleProposal(proposal *consensus.Proposal) error {
	e.proposals = append(e.proposals, proposal)
	return nil
}

func (e *mockBFTConsensusEngine) HandleVote(vote *consensus.Vote) error {
	e.votes = append(e.votes, vote)
	return nil
}

func (e *mockBFTConsensusEngine) HandleCommit(commit *consensus.Commit) error {
	e.commits = append(e.commits, commit)
	return nil
}

func (e *mockBFTConsensusEngine) OnTimeout(height int64, round int32, step consensus.TimeoutStep) error {
	return nil
}

func (e *mockBFTConsensusEngine) ProduceBlock(height int64) (*consensus.Block, error) {
	return &consensus.Block{Height: height}, nil
}

func (e *mockBFTConsensusEngine) ShouldPropose(height int64, round int32) bool {
	return false
}

// mockStreamAwareEngine implements consensus.StreamAwareConsensus for testing.
type mockStreamAwareEngine struct {
	mockConsensusEngine
	streams  []consensus.StreamConfig
	messages []struct {
		stream string
		peerID peer.ID
		data   []byte
	}
}

func (e *mockStreamAwareEngine) StreamConfigs() []consensus.StreamConfig {
	return e.streams
}

func (e *mockStreamAwareEngine) HandleStreamMessage(stream string, peerID peer.ID, data []byte) error {
	e.messages = append(e.messages, struct {
		stream string
		peerID peer.ID
		data   []byte
	}{stream, peerID, data})
	return nil
}

func TestNewConsensusReactorWithEngine(t *testing.T) {
	engine := &mockConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	require.NotNil(t, reactor)
	assert.Equal(t, engine, reactor.GetEngine())
}

func TestNewConsensusReactorWithEngine_StreamAware(t *testing.T) {
	engine := &mockStreamAwareEngine{
		streams: []consensus.StreamConfig{
			{Name: "custom-stream-1"},
			{Name: "custom-stream-2"},
		},
	}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	require.NotNil(t, reactor)
	assert.True(t, reactor.HasCustomStream("custom-stream-1"))
	assert.True(t, reactor.HasCustomStream("custom-stream-2"))
	assert.False(t, reactor.HasCustomStream("unknown-stream"))

	streams := reactor.CustomStreams()
	assert.Len(t, streams, 2)
}

func TestConsensusReactor_SetEngine(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	engine := &mockConsensusEngine{}
	reactor.SetEngine(engine)
	assert.Equal(t, engine, reactor.GetEngine())

	reactor.SetEngine(nil)
	assert.Nil(t, reactor.GetEngine())
}

func TestConsensusReactor_SetEngine_StreamAware(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	engine := &mockStreamAwareEngine{
		streams: []consensus.StreamConfig{
			{Name: "my-stream"},
		},
	}
	reactor.SetEngine(engine)

	assert.True(t, reactor.HasCustomStream("my-stream"))

	// Setting a new engine should clear old streams
	engine2 := &mockStreamAwareEngine{
		streams: []consensus.StreamConfig{
			{Name: "other-stream"},
		},
	}
	reactor.SetEngine(engine2)

	assert.False(t, reactor.HasCustomStream("my-stream"))
	assert.True(t, reactor.HasCustomStream("other-stream"))
}

func TestConsensusReactor_HandleMessageWithEngine(t *testing.T) {
	engine := &mockConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	// Non-BFT engine: message should be passed as raw block data
	data := []byte("raw block data from peer")
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	require.Len(t, engine.blocksProcessed, 1)
	// For non-BFT engines, the raw data is stored in Block.Data
	assert.Equal(t, data, engine.blocksProcessed[0].Data)
}

func TestConsensusReactor_HandleMessageEngineOverHandler(t *testing.T) {
	// Both engine and handler set - engine should take priority
	engine := &mockConsensusEngine{}
	handler := &mockConsensusHandler{}

	reactor := NewConsensusReactor(nil, nil)
	reactor.SetHandler(handler)
	reactor.SetEngine(engine)

	data := []byte("test data")
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Engine should have processed the block
	require.Len(t, engine.blocksProcessed, 1)
	assert.Equal(t, data, engine.blocksProcessed[0].Data)
	// Handler should not have been called
	require.Len(t, handler.messages, 0)
}

func TestConsensusReactor_HandleMessageBFTProposal(t *testing.T) {
	engine := &mockBFTConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	data := makeProposalData(100, 2, 1)
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	require.Len(t, engine.proposals, 1)
	assert.Equal(t, int64(100), engine.proposals[0].Height)
	assert.Equal(t, int32(2), engine.proposals[0].Round)
}

func TestConsensusReactor_HandleMessageBFTVote(t *testing.T) {
	engine := &mockBFTConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	data := makeVoteData(consensus.VoteTypePrevote, 100, 2, 5, []byte{1, 2, 3})
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	require.Len(t, engine.votes, 1)
	assert.Equal(t, consensus.VoteTypePrevote, engine.votes[0].Type)
	assert.Equal(t, int64(100), engine.votes[0].Height)
	assert.Equal(t, int32(2), engine.votes[0].Round)
	assert.Equal(t, uint16(5), engine.votes[0].ValidatorIndex)
}

func TestConsensusReactor_HandleMessageBFTCommit(t *testing.T) {
	engine := &mockBFTConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	data := makeCommitData(100, 2, []byte{1, 2, 3, 4})
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	require.Len(t, engine.commits, 1)
	assert.Equal(t, int64(100), engine.commits[0].Height)
	assert.Equal(t, int32(2), engine.commits[0].Round)
}

func TestConsensusReactor_HandleMessageBFTBlock(t *testing.T) {
	engine := &mockBFTConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	data := makeBFTBlockData(100, 2)
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.NoError(t, err)

	require.Len(t, engine.blocksProcessed, 1)
	assert.Equal(t, int64(100), engine.blocksProcessed[0].Height)
	assert.Equal(t, int32(2), engine.blocksProcessed[0].Round)
}

func TestConsensusReactor_HandleMessageBFTUnknownType(t *testing.T) {
	engine := &mockBFTConsensusEngine{}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	// Unknown message type (99)
	data := []byte{99, 0, 0, 0, 0, 0, 0, 0, 0}
	err := reactor.HandleMessage(peer.ID("peer1"), data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown BFT message type")
}

func TestConsensusReactor_HandleCustomStreamMessage(t *testing.T) {
	engine := &mockStreamAwareEngine{
		streams: []consensus.StreamConfig{
			{Name: "my-custom-stream"},
		},
	}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	peerID := peer.ID("peer1")
	data := []byte("custom stream data")
	err := reactor.HandleCustomStreamMessage("my-custom-stream", peerID, data)
	require.NoError(t, err)

	require.Len(t, engine.messages, 1)
	assert.Equal(t, "my-custom-stream", engine.messages[0].stream)
	assert.Equal(t, peerID, engine.messages[0].peerID)
	assert.Equal(t, data, engine.messages[0].data)
}

func TestConsensusReactor_HandleCustomStreamMessage_UnknownStream(t *testing.T) {
	engine := &mockStreamAwareEngine{
		streams: []consensus.StreamConfig{
			{Name: "known-stream"},
		},
	}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	err := reactor.HandleCustomStreamMessage("unknown-stream", peer.ID("peer1"), []byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown custom stream")
}

func TestConsensusReactor_HandleCustomStreamMessage_Empty(t *testing.T) {
	engine := &mockStreamAwareEngine{
		streams: []consensus.StreamConfig{
			{Name: "my-stream"},
		},
	}
	reactor := NewConsensusReactorWithEngine(engine, nil, nil)

	err := reactor.HandleCustomStreamMessage("my-stream", peer.ID("peer1"), nil)
	require.ErrorIs(t, err, types.ErrInvalidMessage)

	err = reactor.HandleCustomStreamMessage("my-stream", peer.ID("peer1"), []byte{})
	require.ErrorIs(t, err, types.ErrInvalidMessage)
}

func TestConsensusReactor_StartStop(t *testing.T) {
	reactor := NewConsensusReactor(nil, nil)

	err := reactor.Start()
	require.NoError(t, err)
	assert.True(t, reactor.IsRunning())

	// Double start should fail
	err = reactor.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	err = reactor.Stop()
	require.NoError(t, err)
	assert.False(t, reactor.IsRunning())

	// Double stop should be safe
	err = reactor.Stop()
	require.NoError(t, err)
}

// Helper functions to create test message data

func makeProposalData(height int64, round int32, polRound int32) []byte {
	data := make([]byte, 37)
	data[0] = MsgTypeProposal
	binary.BigEndian.PutUint64(data[1:9], uint64(height))
	binary.BigEndian.PutUint32(data[9:13], uint32(round))
	binary.BigEndian.PutUint32(data[13:17], uint32(polRound))
	// Add minimal block data (8 bytes height + 4 bytes round + 8 bytes timestamp = 20 bytes)
	binary.BigEndian.PutUint64(data[17:25], uint64(height))
	binary.BigEndian.PutUint32(data[25:29], uint32(round))
	binary.BigEndian.PutUint64(data[29:37], 0) // timestamp (8 bytes)
	return data
}

func makeVoteData(voteType consensus.VoteType, height int64, round int32, validatorIndex uint16, blockHash []byte) []byte {
	data := make([]byte, 17+len(blockHash))
	data[0] = MsgTypeVote
	data[1] = byte(voteType)
	binary.BigEndian.PutUint64(data[2:10], uint64(height))
	binary.BigEndian.PutUint32(data[10:14], uint32(round))
	binary.BigEndian.PutUint16(data[14:16], validatorIndex)
	data[16] = byte(len(blockHash))
	copy(data[17:], blockHash)
	return data
}

func makeCommitData(height int64, round int32, blockHash []byte) []byte {
	data := make([]byte, 14+len(blockHash))
	data[0] = MsgTypeCommit
	binary.BigEndian.PutUint64(data[1:9], uint64(height))
	binary.BigEndian.PutUint32(data[9:13], uint32(round))
	data[13] = byte(len(blockHash))
	copy(data[14:], blockHash)
	return data
}

func makeBFTBlockData(height int64, round int32) []byte {
	data := make([]byte, 21)
	data[0] = MsgTypeBlock
	binary.BigEndian.PutUint64(data[1:9], uint64(height))
	binary.BigEndian.PutUint32(data[9:13], uint32(round))
	binary.BigEndian.PutUint64(data[13:21], 0) // timestamp (8 bytes)
	return data
}
