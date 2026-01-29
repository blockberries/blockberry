package handlers

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/consensus"
	"github.com/blockberries/blockberry/p2p"
	"github.com/blockberries/blockberry/types"
)

// ConsensusHandler defines the interface for handling consensus messages.
// Applications implement this interface to receive consensus messages from peers.
// Deprecated: Use consensus.ConsensusEngine instead for new implementations.
type ConsensusHandler interface {
	// HandleConsensusMessage processes an incoming consensus message from a peer.
	HandleConsensusMessage(peerID peer.ID, data []byte) error
}

// Consensus message type IDs for BFT message routing.
const (
	// MsgTypeProposal is a block proposal message.
	MsgTypeProposal uint8 = 1
	// MsgTypeVote is a prevote or precommit vote message.
	MsgTypeVote uint8 = 2
	// MsgTypeCommit is an aggregated commit message.
	MsgTypeCommit uint8 = 3
	// MsgTypeBlock is a raw block message.
	MsgTypeBlock uint8 = 4
)

// ConsensusReactor routes consensus messages between peers and the consensus engine.
// It supports both the legacy ConsensusHandler interface and the new ConsensusEngine interface.
// The reactor automatically detects BFT engines and custom stream-aware engines to route
// messages appropriately.
type ConsensusReactor struct {
	// Dependencies
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// Consensus engine (new approach)
	engine consensus.ConsensusEngine

	// Legacy handler (deprecated, for backward compatibility)
	handler ConsensusHandler

	// Custom streams registered by StreamAwareConsensus engines
	customStreams map[string]bool

	// Lifecycle
	running bool
	stopCh  chan struct{}

	mu sync.RWMutex
}

// NewConsensusReactor creates a new consensus reactor.
func NewConsensusReactor(
	network *p2p.Network,
	peerManager *p2p.PeerManager,
) *ConsensusReactor {
	return &ConsensusReactor{
		network:       network,
		peerManager:   peerManager,
		customStreams: make(map[string]bool),
	}
}

// NewConsensusReactorWithEngine creates a new consensus reactor with a consensus engine.
// This is the preferred constructor for new implementations.
func NewConsensusReactorWithEngine(
	engine consensus.ConsensusEngine,
	network *p2p.Network,
	peerManager *p2p.PeerManager,
) *ConsensusReactor {
	r := &ConsensusReactor{
		engine:        engine,
		network:       network,
		peerManager:   peerManager,
		customStreams: make(map[string]bool),
	}

	// Register custom streams if engine supports them
	if streamAware, ok := engine.(consensus.StreamAwareConsensus); ok {
		for _, cfg := range streamAware.StreamConfigs() {
			r.customStreams[cfg.Name] = true
		}
	}

	return r
}

// Name returns the component name for identification.
func (r *ConsensusReactor) Name() string {
	return "consensus-reactor"
}

// Start starts the consensus reactor.
func (r *ConsensusReactor) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("consensus reactor already running")
	}

	r.running = true
	r.stopCh = make(chan struct{})
	return nil
}

// Stop stops the consensus reactor.
func (r *ConsensusReactor) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	r.running = false
	if r.stopCh != nil {
		close(r.stopCh)
	}

	return nil
}

// IsRunning returns whether the reactor is running.
func (r *ConsensusReactor) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// SetHandler sets the consensus handler for incoming messages.
// Deprecated: Use SetEngine instead for new implementations.
func (r *ConsensusReactor) SetHandler(handler ConsensusHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
}

// GetHandler returns the current consensus handler.
// Deprecated: Use GetEngine instead for new implementations.
func (r *ConsensusReactor) GetHandler() ConsensusHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handler
}

// SetEngine sets the consensus engine for this reactor.
// This also registers any custom streams if the engine implements StreamAwareConsensus.
func (r *ConsensusReactor) SetEngine(engine consensus.ConsensusEngine) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.engine = engine
	r.customStreams = make(map[string]bool)

	// Register custom streams if engine supports them
	if streamAware, ok := engine.(consensus.StreamAwareConsensus); ok {
		for _, cfg := range streamAware.StreamConfigs() {
			r.customStreams[cfg.Name] = true
		}
	}
}

// GetEngine returns the current consensus engine.
func (r *ConsensusReactor) GetEngine() consensus.ConsensusEngine {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.engine
}

// HasCustomStream returns true if the given stream is registered as a custom stream.
func (r *ConsensusReactor) HasCustomStream(streamName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.customStreams[streamName]
}

// CustomStreams returns the names of all registered custom streams.
func (r *ConsensusReactor) CustomStreams() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	streams := make([]string, 0, len(r.customStreams))
	for name := range r.customStreams {
		streams = append(streams, name)
	}
	return streams
}

// HandleMessage processes an incoming consensus message on the main consensus stream.
// It routes messages to the consensus engine if set, otherwise falls back to the legacy handler.
func (r *ConsensusReactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	r.mu.RLock()
	engine := r.engine
	handler := r.handler
	r.mu.RUnlock()

	// Prefer engine over handler
	if engine != nil {
		return r.handleEngineMessage(engine, peerID, data)
	}

	// Fall back to legacy handler
	if handler != nil {
		return handler.HandleConsensusMessage(peerID, data)
	}

	// No handler or engine registered - silently ignore
	return nil
}

// HandleCustomStreamMessage processes a message on a custom stream registered by the engine.
func (r *ConsensusReactor) HandleCustomStreamMessage(streamName string, peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	r.mu.RLock()
	engine := r.engine
	hasStream := r.customStreams[streamName]
	r.mu.RUnlock()

	if !hasStream {
		return fmt.Errorf("unknown custom stream: %s", streamName)
	}

	streamAware, ok := engine.(consensus.StreamAwareConsensus)
	if !ok {
		return fmt.Errorf("engine does not support custom streams")
	}

	return streamAware.HandleStreamMessage(streamName, peerID, data)
}

// handleEngineMessage routes a message to the appropriate engine handler.
func (r *ConsensusReactor) handleEngineMessage(engine consensus.ConsensusEngine, peerID peer.ID, data []byte) error {
	// Check if this is a BFT engine
	bft, isBFT := engine.(consensus.BFTConsensus)

	if !isBFT {
		// For non-BFT engines, treat the message as a raw block
		return engine.ProcessBlock(&consensus.Block{Data: data})
	}

	// BFT engine - parse message type and route accordingly
	return r.handleBFTMessage(bft, peerID, data)
}

// handleBFTMessage parses and routes BFT consensus messages.
// The peerID is currently unused but will be used for message tracking in future.
func (r *ConsensusReactor) handleBFTMessage(bft consensus.BFTConsensus, _ peer.ID, data []byte) error {
	if len(data) < 1 {
		return types.ErrInvalidMessage
	}

	msgType := data[0]
	payload := data[1:]

	switch msgType {
	case MsgTypeProposal:
		proposal, err := r.decodeProposal(payload)
		if err != nil {
			return err
		}
		return bft.HandleProposal(proposal)

	case MsgTypeVote:
		vote, err := r.decodeVote(payload)
		if err != nil {
			return err
		}
		return bft.HandleVote(vote)

	case MsgTypeCommit:
		commit, err := r.decodeCommit(payload)
		if err != nil {
			return err
		}
		return bft.HandleCommit(commit)

	case MsgTypeBlock:
		block, err := r.decodeBlock(payload)
		if err != nil {
			return err
		}
		return bft.ProcessBlock(block)

	default:
		return fmt.Errorf("unknown BFT message type: %d", msgType)
	}
}

// decodeProposal decodes a proposal message from bytes.
// TODO: Implement proper cramberry deserialization.
func (r *ConsensusReactor) decodeProposal(data []byte) (*consensus.Proposal, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("proposal data too short")
	}

	//nolint:gosec // Safe conversion: protocol values are within int64/int32 range
	proposal := &consensus.Proposal{
		Height:   int64(binary.BigEndian.Uint64(data[0:8])),
		Round:    int32(binary.BigEndian.Uint32(data[8:12])),
		POLRound: int32(binary.BigEndian.Uint32(data[12:16])),
	}

	// Remaining data is the block
	if len(data) > 16 {
		block, err := r.decodeBlock(data[16:])
		if err != nil {
			return nil, err
		}
		proposal.Block = block
	}

	return proposal, nil
}

// decodeVote decodes a vote message from bytes.
// TODO: Implement proper cramberry deserialization.
func (r *ConsensusReactor) decodeVote(data []byte) (*consensus.Vote, error) {
	if len(data) < 15 {
		return nil, fmt.Errorf("vote data too short")
	}

	//nolint:gosec // Safe conversion: protocol values are within int64/int32 range
	vote := &consensus.Vote{
		Type:           consensus.VoteType(data[0]),
		Height:         int64(binary.BigEndian.Uint64(data[1:9])),
		Round:          int32(binary.BigEndian.Uint32(data[9:13])),
		ValidatorIndex: binary.BigEndian.Uint16(data[13:15]),
	}

	// Remaining data is block hash and signature
	if len(data) > 15 {
		hashLen := int(data[15])
		// Validate hashLen: must be reasonable (max 64 bytes for SHA-512)
		// and data must be long enough to contain the hash
		if hashLen > 64 {
			return nil, fmt.Errorf("invalid hash length: %d (max 64)", hashLen)
		}
		if len(data) < 16+hashLen {
			return nil, fmt.Errorf("vote data too short for hash length %d", hashLen)
		}
		vote.BlockHash = data[16 : 16+hashLen]
		if len(data) > 16+hashLen {
			vote.Signature = data[16+hashLen:]
		}
	}

	return vote, nil
}

// decodeCommit decodes a commit message from bytes.
// TODO: Implement proper cramberry deserialization.
func (r *ConsensusReactor) decodeCommit(data []byte) (*consensus.Commit, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("commit data too short")
	}

	//nolint:gosec // Safe conversion: protocol values are within int64/int32 range
	commit := &consensus.Commit{
		Height: int64(binary.BigEndian.Uint64(data[0:8])),
		Round:  int32(binary.BigEndian.Uint32(data[8:12])),
	}

	// Remaining data contains block hash and signatures
	if len(data) > 12 {
		hashLen := int(data[12])
		// Validate hashLen: must be reasonable (max 64 bytes for SHA-512)
		// and data must be long enough to contain the hash
		if hashLen > 64 {
			return nil, fmt.Errorf("invalid hash length: %d (max 64)", hashLen)
		}
		if len(data) < 13+hashLen {
			return nil, fmt.Errorf("commit data too short for hash length %d", hashLen)
		}
		commit.BlockHash = data[13 : 13+hashLen]
		// TODO: Parse signatures from data[13+hashLen:]
	}

	return commit, nil
}

// decodeBlock decodes a block from bytes.
// TODO: Implement proper cramberry deserialization.
func (r *ConsensusReactor) decodeBlock(data []byte) (*consensus.Block, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("block data too short")
	}

	//nolint:gosec // Safe conversion: protocol values are within int64/int32 range
	block := &consensus.Block{
		Height:    int64(binary.BigEndian.Uint64(data[0:8])),
		Round:     int32(binary.BigEndian.Uint32(data[8:12])),
		Timestamp: int64(binary.BigEndian.Uint64(data[12:20])),
		Data:      data[20:],
	}

	return block, nil
}

// SendConsensusMessage sends a consensus message to a specific peer.
func (r *ConsensusReactor) SendConsensusMessage(peerID peer.ID, data []byte) error {
	if r.network == nil {
		return nil
	}

	return r.network.Send(peerID, p2p.StreamConsensus, data)
}

// BroadcastConsensusMessage sends a consensus message to all connected peers.
func (r *ConsensusReactor) BroadcastConsensusMessage(data []byte) error {
	if r.network == nil || r.peerManager == nil {
		return nil
	}

	peers := r.peerManager.GetConnectedPeers()
	for _, peerID := range peers {
		if err := r.network.Send(peerID, p2p.StreamConsensus, data); err != nil {
			// Continue trying other peers
			continue
		}
	}

	return nil
}

// OnPeerDisconnected is called when a peer disconnects.
// ConsensusReactor doesn't maintain per-peer state, so this is a no-op.
func (r *ConsensusReactor) OnPeerDisconnected(peerID peer.ID) {
	// No per-peer state to clean up
}
