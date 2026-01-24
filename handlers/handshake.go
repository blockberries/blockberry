package handlers

import (
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

// Handshake message type IDs from schema.
const (
	TypeIDHelloRequest  cramberry.TypeID = 128
	TypeIDHelloResponse cramberry.TypeID = 129
	TypeIDHelloFinalize cramberry.TypeID = 130
)

// HandshakeState represents the state of a handshake with a peer.
type HandshakeState int

const (
	// StateInit is the initial state before any messages are exchanged.
	StateInit HandshakeState = iota
	// StateHelloSent indicates we've sent our HelloRequest.
	StateHelloSent
	// StateHelloReceived indicates we've received and validated the peer's HelloRequest.
	StateHelloReceived
	// StateResponseSent indicates we've sent our HelloResponse.
	StateResponseSent
	// StateResponseReceived indicates we've received the peer's HelloResponse.
	StateResponseReceived
	// StateFinalizeSent indicates we've sent HelloFinalize.
	StateFinalizeSent
	// StateComplete indicates the handshake is complete.
	StateComplete
)

// PeerHandshakeState tracks the handshake state for a single peer.
type PeerHandshakeState struct {
	State       HandshakeState
	PeerPubKey  []byte
	PeerNodeID  string
	PeerHeight  int64
	PeerVersion int32
	PeerChainID string
	StartedAt   time.Time
}

// HandshakeHandler manages the handshake protocol for peer connections.
type HandshakeHandler struct {
	// Configuration
	chainID         string
	protocolVersion int32
	nodeID          string
	publicKey       []byte

	// Dependencies
	network     *p2p.Network
	peerManager *p2p.PeerManager
	getHeight   func() int64 // Function to get current block height

	// Per-peer handshake state
	states map[peer.ID]*PeerHandshakeState
	mu     sync.RWMutex
}

// NewHandshakeHandler creates a new handshake handler.
func NewHandshakeHandler(
	chainID string,
	protocolVersion int32,
	nodeID string,
	publicKey []byte,
	network *p2p.Network,
	peerManager *p2p.PeerManager,
	getHeight func() int64,
) *HandshakeHandler {
	return &HandshakeHandler{
		chainID:         chainID,
		protocolVersion: protocolVersion,
		nodeID:          nodeID,
		publicKey:       publicKey,
		network:         network,
		peerManager:     peerManager,
		getHeight:       getHeight,
		states:          make(map[peer.ID]*PeerHandshakeState),
	}
}

// OnPeerConnected should be called when a new peer connection is established.
// It initiates the handshake by sending a HelloRequest.
func (h *HandshakeHandler) OnPeerConnected(peerID peer.ID, isOutbound bool) error {
	h.mu.Lock()
	// Initialize handshake state for this peer
	h.states[peerID] = &PeerHandshakeState{
		State:     StateInit,
		StartedAt: time.Now(),
	}
	h.mu.Unlock()

	// Send HelloRequest
	return h.sendHelloRequest(peerID)
}

// OnPeerDisconnected cleans up handshake state when a peer disconnects.
func (h *HandshakeHandler) OnPeerDisconnected(peerID peer.ID) {
	h.mu.Lock()
	delete(h.states, peerID)
	h.mu.Unlock()
}

// HandleMessage processes an incoming handshake message.
func (h *HandshakeHandler) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	// Decode the message type ID (varint at start)
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	if r.Err() != nil {
		return fmt.Errorf("reading type ID: %w", r.Err())
	}

	// Get the message payload (remaining data after type ID)
	payload := r.Remaining()

	switch typeID {
	case TypeIDHelloRequest:
		return h.handleHelloRequest(peerID, payload)
	case TypeIDHelloResponse:
		return h.handleHelloResponse(peerID, payload)
	case TypeIDHelloFinalize:
		return h.handleHelloFinalize(peerID, payload)
	default:
		return fmt.Errorf("%w: unknown handshake message type %d", types.ErrInvalidMessage, typeID)
	}
}

// sendHelloRequest sends a HelloRequest to the peer.
func (h *HandshakeHandler) sendHelloRequest(peerID peer.ID) error {
	height := h.getHeight()
	timestamp := time.Now().UnixNano()

	req := &schema.HelloRequest{
		NodeId:       &h.nodeID,
		Version:      &h.protocolVersion,
		ChainId:      &h.chainID,
		Timestamp:    &timestamp,
		LatestHeight: &height,
	}

	data, err := h.encodeHandshakeMessage(TypeIDHelloRequest, req)
	if err != nil {
		return fmt.Errorf("encoding HelloRequest: %w", err)
	}

	if h.network != nil {
		if err := h.network.Send(peerID, p2p.StreamHandshake, data); err != nil {
			return fmt.Errorf("sending HelloRequest: %w", err)
		}
	}

	h.mu.Lock()
	if state, ok := h.states[peerID]; ok {
		state.State = StateHelloSent
	}
	h.mu.Unlock()

	return nil
}

// handleHelloRequest processes an incoming HelloRequest.
func (h *HandshakeHandler) handleHelloRequest(peerID peer.ID, data []byte) error {
	var req schema.HelloRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return fmt.Errorf("decoding HelloRequest: %w", err)
	}

	if err := req.Validate(); err != nil {
		return fmt.Errorf("validating HelloRequest: %w", err)
	}

	// Validate chain ID
	if *req.ChainId != h.chainID {
		// Blacklist peer for chain ID mismatch
		if h.network != nil {
			_ = h.network.BlacklistPeer(peerID)
		}
		return fmt.Errorf("%w: expected %s, got %s", types.ErrChainIDMismatch, h.chainID, *req.ChainId)
	}

	// Validate protocol version
	if *req.Version != h.protocolVersion {
		// Blacklist peer for version mismatch
		if h.network != nil {
			_ = h.network.BlacklistPeer(peerID)
		}
		return fmt.Errorf("%w: expected %d, got %d", types.ErrVersionMismatch, h.protocolVersion, *req.Version)
	}

	// Update state with peer info
	h.mu.Lock()
	state, ok := h.states[peerID]
	if !ok {
		state = &PeerHandshakeState{StartedAt: time.Now()}
		h.states[peerID] = state
	}
	state.PeerNodeID = *req.NodeId
	state.PeerVersion = *req.Version
	state.PeerChainID = *req.ChainId
	state.PeerHeight = *req.LatestHeight
	state.State = StateHelloReceived
	h.mu.Unlock()

	// Send HelloResponse with our public key
	return h.sendHelloResponse(peerID, true)
}

// sendHelloResponse sends a HelloResponse to the peer.
func (h *HandshakeHandler) sendHelloResponse(peerID peer.ID, accepted bool) error {
	resp := &schema.HelloResponse{
		Accepted:  &accepted,
		PublicKey: h.publicKey,
	}

	data, err := h.encodeHandshakeMessage(TypeIDHelloResponse, resp)
	if err != nil {
		return fmt.Errorf("encoding HelloResponse: %w", err)
	}

	if h.network != nil {
		if err := h.network.Send(peerID, p2p.StreamHandshake, data); err != nil {
			return fmt.Errorf("sending HelloResponse: %w", err)
		}
	}

	h.mu.Lock()
	if state, ok := h.states[peerID]; ok {
		state.State = StateResponseSent
	}
	h.mu.Unlock()

	return nil
}

// handleHelloResponse processes an incoming HelloResponse.
func (h *HandshakeHandler) handleHelloResponse(peerID peer.ID, data []byte) error {
	var resp schema.HelloResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return fmt.Errorf("decoding HelloResponse: %w", err)
	}

	if err := resp.Validate(); err != nil {
		return fmt.Errorf("validating HelloResponse: %w", err)
	}

	// Check if peer accepted our handshake
	if !*resp.Accepted {
		if h.network != nil {
			_ = h.network.Disconnect(peerID)
		}
		return fmt.Errorf("%w: peer rejected handshake", types.ErrHandshakeFailed)
	}

	// Store peer's public key
	h.mu.Lock()
	state, ok := h.states[peerID]
	if !ok {
		h.mu.Unlock()
		return fmt.Errorf("%w: no handshake state for peer", types.ErrInvalidMessage)
	}
	state.PeerPubKey = resp.PublicKey
	state.State = StateResponseReceived
	h.mu.Unlock()

	// Prepare encrypted streams using peer's public key
	if h.network != nil {
		if err := h.network.PrepareStreams(peerID, resp.PublicKey); err != nil {
			return fmt.Errorf("preparing streams: %w", err)
		}
	}

	// Send HelloFinalize
	return h.sendHelloFinalize(peerID, true)
}

// sendHelloFinalize sends a HelloFinalize to the peer.
func (h *HandshakeHandler) sendHelloFinalize(peerID peer.ID, success bool) error {
	fin := &schema.HelloFinalize{
		Success: &success,
	}

	data, err := h.encodeHandshakeMessage(TypeIDHelloFinalize, fin)
	if err != nil {
		return fmt.Errorf("encoding HelloFinalize: %w", err)
	}

	if h.network != nil {
		if err := h.network.Send(peerID, p2p.StreamHandshake, data); err != nil {
			return fmt.Errorf("sending HelloFinalize: %w", err)
		}
	}

	h.mu.Lock()
	if state, ok := h.states[peerID]; ok {
		state.State = StateFinalizeSent
	}
	h.mu.Unlock()

	return nil
}

// handleHelloFinalize processes an incoming HelloFinalize.
func (h *HandshakeHandler) handleHelloFinalize(peerID peer.ID, data []byte) error {
	var fin schema.HelloFinalize
	if err := fin.UnmarshalCramberry(data); err != nil {
		return fmt.Errorf("decoding HelloFinalize: %w", err)
	}

	if err := fin.Validate(); err != nil {
		return fmt.Errorf("validating HelloFinalize: %w", err)
	}

	// Check if peer indicates success
	if !*fin.Success {
		if h.network != nil {
			_ = h.network.Disconnect(peerID)
		}
		return fmt.Errorf("%w: peer indicated failure", types.ErrHandshakeFailed)
	}

	h.mu.Lock()
	state, ok := h.states[peerID]
	if !ok {
		h.mu.Unlock()
		return fmt.Errorf("%w: no handshake state for peer", types.ErrInvalidMessage)
	}

	// Check if we're ready to finalize
	// We need to have sent our finalize (meaning we received their response and prepared streams)
	if state.State < StateFinalizeSent {
		// We haven't sent finalize yet, just mark that we received theirs
		h.mu.Unlock()
		return nil
	}

	state.State = StateComplete
	pubKey := state.PeerPubKey
	h.mu.Unlock()

	// Finalize the handshake - this transitions to StateEstablished in glueberry
	if h.network != nil {
		if err := h.network.FinalizeHandshake(peerID); err != nil {
			return fmt.Errorf("finalizing handshake: %w", err)
		}
	}

	// Store peer's public key in peer manager
	if h.peerManager != nil {
		_ = h.peerManager.SetPublicKey(peerID, pubKey)
	}

	return nil
}

// encodeHandshakeMessage encodes a handshake message with its type ID prefix.
func (h *HandshakeHandler) encodeHandshakeMessage(typeID cramberry.TypeID, msg interface {
	MarshalCramberry() ([]byte, error)
}) ([]byte, error) {
	// Encode the message
	msgData, err := msg.MarshalCramberry()
	if err != nil {
		return nil, err
	}

	// Write type ID prefix + message data
	w := cramberry.GetWriter()
	defer cramberry.PutWriter(w)

	w.WriteTypeID(typeID)
	w.WriteRawBytes(msgData)

	if w.Err() != nil {
		return nil, w.Err()
	}

	return w.BytesCopy(), nil
}

// GetPeerState returns the handshake state for a peer.
func (h *HandshakeHandler) GetPeerState(peerID peer.ID) (HandshakeState, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	state, ok := h.states[peerID]
	if !ok {
		return StateInit, false
	}
	return state.State, true
}

// IsHandshakeComplete returns true if the handshake with the peer is complete.
func (h *HandshakeHandler) IsHandshakeComplete(peerID peer.ID) bool {
	state, ok := h.GetPeerState(peerID)
	return ok && state == StateComplete
}

// PeerCount returns the number of peers with active handshakes.
func (h *HandshakeHandler) PeerCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.states)
}

// GetPeerInfo returns the full handshake info for a peer.
func (h *HandshakeHandler) GetPeerInfo(peerID peer.ID) *PeerHandshakeState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	state, ok := h.states[peerID]
	if !ok {
		return nil
	}
	// Return a copy
	copy := *state
	return &copy
}
