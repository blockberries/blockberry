package handlers

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
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
