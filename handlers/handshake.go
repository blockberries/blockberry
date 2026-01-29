package handlers

import (
	"crypto/ed25519"
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

// DefaultHandshakeTimeout is the default maximum time allowed for a handshake to complete.
const DefaultHandshakeTimeout = 30 * time.Second

// DefaultHandshakeCheckInterval is the interval between handshake timeout checks.
const DefaultHandshakeCheckInterval = 5 * time.Second

// TempBanDurationChainMismatch is the temporary ban duration for chain ID mismatch.
// This is softer than permanent blacklisting since the peer might be on a different
// network temporarily (e.g., during upgrades) or might fix their configuration.
const TempBanDurationChainMismatch = 1 * time.Hour

// TempBanDurationVersionMismatch is the temporary ban duration for protocol version mismatch.
// This is softer than permanent blacklisting since the peer might upgrade their software.
const TempBanDurationVersionMismatch = 30 * time.Minute

// HandshakeState represents the state of a handshake with a peer.
type HandshakeState int

const (
	// StateInit is the initial state before any messages are exchanged.
	StateInit HandshakeState = iota
	// StateComplete indicates the handshake is complete.
	StateComplete
)

// PeerHandshakeState tracks the handshake state for a single peer.
// Uses independent flags rather than a linear state machine to handle
// simultaneous initiation and out-of-order message delivery.
type PeerHandshakeState struct {
	State       HandshakeState
	StartedAt   time.Time
	PeerPubKey  []byte
	PeerNodeID  string
	PeerHeight  int64
	PeerVersion int32
	PeerChainID string

	// Flags for handshake progress (order-independent)
	SentRequest      bool // We sent HelloRequest
	ReceivedRequest  bool // We received and validated their HelloRequest
	SentResponse     bool // We sent HelloResponse (with our public key)
	ReceivedResponse bool // We received their HelloResponse (with their public key)
	StreamsPrepared  bool // We called PrepareStreams with their public key
	SentFinalize     bool // We sent HelloFinalize
	ReceivedFinalize bool // We received their HelloFinalize
}

// HandshakeHandler manages the handshake protocol for peer connections.
type HandshakeHandler struct {
	// Configuration
	chainID         string
	protocolVersion int32
	nodeID          string
	publicKey       []byte
	timeout         time.Duration // Maximum time allowed for handshake completion
	checkInterval   time.Duration // Interval for checking stale handshakes

	// Dependencies
	network     *p2p.Network
	peerManager *p2p.PeerManager
	getHeight   func() int64 // Function to get current block height

	// Per-peer handshake state
	states map[peer.ID]*PeerHandshakeState
	mu     sync.RWMutex

	// Lifecycle
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
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
		timeout:         DefaultHandshakeTimeout,
		checkInterval:   DefaultHandshakeCheckInterval,
		network:         network,
		peerManager:     peerManager,
		getHeight:       getHeight,
		states:          make(map[peer.ID]*PeerHandshakeState),
		stopCh:          make(chan struct{}),
	}
}

// Name returns the component name for identification.
func (h *HandshakeHandler) Name() string {
	return "handshake-handler"
}

// Start begins the handshake timeout monitoring loop.
func (h *HandshakeHandler) Start() error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = true
	h.stopCh = make(chan struct{})
	h.mu.Unlock()

	h.wg.Add(1)
	go h.timeoutLoop()

	return nil
}

// Stop halts the handshake timeout monitoring loop.
func (h *HandshakeHandler) Stop() error {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return nil
	}
	h.running = false
	close(h.stopCh)
	h.mu.Unlock()

	h.wg.Wait()
	return nil
}

// IsRunning returns whether the handshake handler is running.
func (h *HandshakeHandler) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// timeoutLoop periodically checks for and cleans up stale handshakes.
func (h *HandshakeHandler) timeoutLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.cleanupStaleHandshakes()
		}
	}
}

// cleanupStaleHandshakes disconnects peers with handshakes that have exceeded the timeout.
func (h *HandshakeHandler) cleanupStaleHandshakes() {
	now := time.Now()

	h.mu.Lock()
	defer h.mu.Unlock()

	for peerID, state := range h.states {
		// Only cleanup incomplete handshakes that have exceeded the timeout
		if state.State != StateComplete && now.Sub(state.StartedAt) > h.timeout {
			delete(h.states, peerID)
			// Disconnect peer asynchronously to avoid holding lock
			if h.network != nil {
				go func(pid peer.ID) {
					_ = h.network.Disconnect(pid)
				}(peerID)
			}
		}
	}
}

// OnPeerConnected should be called when a new peer connection is established.
// It initiates the handshake by sending a HelloRequest.
func (h *HandshakeHandler) OnPeerConnected(peerID peer.ID, isOutbound bool) error {
	h.mu.Lock()
	// Initialize handshake state for this peer only if it doesn't exist
	// (message handlers may have already created state if messages arrived before this event)
	if _, ok := h.states[peerID]; !ok {
		h.states[peerID] = &PeerHandshakeState{
			State:     StateInit,
			StartedAt: time.Now(),
		}
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

// getOrCreateState gets the state for a peer, creating it if needed.
func (h *HandshakeHandler) getOrCreateState(peerID peer.ID) *PeerHandshakeState {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.states[peerID]
	if !ok {
		state = &PeerHandshakeState{
			State:     StateInit,
			StartedAt: time.Now(),
		}
		h.states[peerID] = state
	}
	return state
}

// sendHelloRequest sends a HelloRequest to the peer.
func (h *HandshakeHandler) sendHelloRequest(peerID peer.ID) error {
	h.mu.Lock()
	state := h.states[peerID]
	if state != nil && state.SentRequest {
		h.mu.Unlock()
		return nil // Already sent
	}
	if state != nil {
		state.SentRequest = true
	}
	h.mu.Unlock()

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

	return nil
}

// handleHelloRequest processes an incoming HelloRequest.
// On receiving HelloRequest -> send HelloResponse (with our public key)
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
		if h.network != nil {
			// Use temporary ban instead of permanent blacklist - peer might be
			// on a different network temporarily or fix their configuration
			reason := fmt.Sprintf("chain ID mismatch: expected %s, got %s", h.chainID, *req.ChainId)
			_ = h.network.TempBanPeer(peerID, TempBanDurationChainMismatch, reason)
		}
		return fmt.Errorf("%w: expected %s, got %s", types.ErrChainIDMismatch, h.chainID, *req.ChainId)
	}

	// Validate protocol version
	if *req.Version != h.protocolVersion {
		if h.network != nil {
			// Use temporary ban instead of permanent blacklist - peer might upgrade
			reason := fmt.Sprintf("version mismatch: expected %d, got %d", h.protocolVersion, *req.Version)
			_ = h.network.TempBanPeer(peerID, TempBanDurationVersionMismatch, reason)
		}
		return fmt.Errorf("%w: expected %d, got %d", types.ErrVersionMismatch, h.protocolVersion, *req.Version)
	}

	// Get or create state, update with peer info
	state := h.getOrCreateState(peerID)

	h.mu.Lock()
	alreadyReceived := state.ReceivedRequest
	if !alreadyReceived {
		state.ReceivedRequest = true
		state.PeerNodeID = *req.NodeId
		state.PeerVersion = *req.Version
		state.PeerChainID = *req.ChainId
		state.PeerHeight = *req.LatestHeight
	}
	h.mu.Unlock()

	// Send HelloResponse if we haven't already
	if !alreadyReceived {
		return h.sendHelloResponse(peerID)
	}
	return nil
}

// sendHelloResponse sends a HelloResponse to the peer.
func (h *HandshakeHandler) sendHelloResponse(peerID peer.ID) error {
	h.mu.Lock()
	state := h.states[peerID]
	if state != nil && state.SentResponse {
		h.mu.Unlock()
		return nil // Already sent
	}
	if state != nil {
		state.SentResponse = true
	}
	h.mu.Unlock()

	accepted := true
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

	return nil
}

// handleHelloResponse processes an incoming HelloResponse.
// On receiving HelloResponse -> PrepareStreams + send HelloFinalize
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

	// Validate public key length (must be ed25519.PublicKeySize = 32 bytes)
	if len(resp.PublicKey) != ed25519.PublicKeySize {
		if h.network != nil {
			_ = h.network.Disconnect(peerID)
		}
		return fmt.Errorf("%w: invalid public key length %d (expected %d)",
			types.ErrHandshakeFailed, len(resp.PublicKey), ed25519.PublicKeySize)
	}

	// Get state
	state := h.getOrCreateState(peerID)

	h.mu.Lock()
	alreadyReceived := state.ReceivedResponse
	if !alreadyReceived {
		state.ReceivedResponse = true
		state.PeerPubKey = resp.PublicKey
	}
	h.mu.Unlock()

	if alreadyReceived {
		return nil // Already processed
	}

	// Prepare encrypted streams using peer's public key
	if h.network != nil {
		if err := h.network.PrepareStreams(peerID, resp.PublicKey); err != nil {
			return fmt.Errorf("preparing streams: %w", err)
		}
	}

	h.mu.Lock()
	state.StreamsPrepared = true
	h.mu.Unlock()

	// Send HelloFinalize
	if err := h.sendHelloFinalize(peerID); err != nil {
		return err
	}

	// Check if we can complete the handshake
	return h.tryComplete(peerID)
}

// sendHelloFinalize sends a HelloFinalize to the peer.
func (h *HandshakeHandler) sendHelloFinalize(peerID peer.ID) error {
	h.mu.Lock()
	state := h.states[peerID]
	if state != nil && state.SentFinalize {
		h.mu.Unlock()
		return nil // Already sent
	}
	if state != nil {
		state.SentFinalize = true
	}
	h.mu.Unlock()

	success := true
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

	// Mark that we received their finalize
	state := h.getOrCreateState(peerID)

	h.mu.Lock()
	state.ReceivedFinalize = true
	h.mu.Unlock()

	// Check if we can complete the handshake
	return h.tryComplete(peerID)
}

// tryComplete checks if all conditions are met to finalize the handshake.
// Conditions: StreamsPrepared AND ReceivedFinalize
func (h *HandshakeHandler) tryComplete(peerID peer.ID) error {
	h.mu.Lock()
	state, ok := h.states[peerID]
	if !ok {
		h.mu.Unlock()
		return nil
	}

	// Check completion conditions (order-independent)
	canComplete := state.StreamsPrepared && state.ReceivedFinalize && state.State != StateComplete
	if !canComplete {
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
// The returned struct is a deep copy; callers may safely modify it.
func (h *HandshakeHandler) GetPeerInfo(peerID peer.ID) *PeerHandshakeState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	state, ok := h.states[peerID]
	if !ok {
		return nil
	}
	// Return a deep copy to prevent callers from mutating internal state
	return &PeerHandshakeState{
		State:            state.State,
		StartedAt:        state.StartedAt,
		PeerPubKey:       append([]byte(nil), state.PeerPubKey...),
		PeerNodeID:       state.PeerNodeID,
		PeerHeight:       state.PeerHeight,
		PeerVersion:      state.PeerVersion,
		PeerChainID:      state.PeerChainID,
		SentRequest:      state.SentRequest,
		ReceivedRequest:  state.ReceivedRequest,
		SentResponse:     state.SentResponse,
		ReceivedResponse: state.ReceivedResponse,
		StreamsPrepared:  state.StreamsPrepared,
		SentFinalize:     state.SentFinalize,
		ReceivedFinalize: state.ReceivedFinalize,
	}
}
