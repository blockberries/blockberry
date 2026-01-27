package p2p

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/glueberry"
	"github.com/blockberries/glueberry/pkg/streams"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/blockberries/blockberry/types"
)

// Stream name constants for blockberry protocols.
const (
	StreamHandshake    = "handshake"
	StreamPEX          = "pex"
	StreamTransactions = "transactions"
	StreamBlockSync    = "blocksync"
	StreamStateSync    = "statesync"
	StreamBlocks       = "blocks"
	StreamConsensus    = "consensus"
	StreamHousekeeping = "housekeeping"
)

// AllStreams returns all encrypted stream names used by blockberry.
func AllStreams() []string {
	return []string{
		StreamPEX,
		StreamTransactions,
		StreamBlockSync,
		StreamStateSync,
		StreamBlocks,
		StreamConsensus,
		StreamHousekeeping,
	}
}

// TempBanEntry represents a temporary ban on a peer.
type TempBanEntry struct {
	ExpiresAt time.Time
	Reason    string
}

// Network wraps glueberry.Node with blockberry-specific logic.
// It provides a higher-level API for blockchain networking.
type Network struct {
	node        *glueberry.Node
	peerManager *PeerManager
	scorer      *PeerScorer

	// Stream management
	streamAdapter *GlueberryStreamAdapter

	// Channels for incoming data
	messages <-chan streams.IncomingMessage
	events   <-chan glueberry.ConnectionEvent

	// Temporary bans (softer than permanent blacklist)
	tempBans   map[peer.ID]*TempBanEntry
	tempBansMu sync.RWMutex

	// Lifecycle
	started bool
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// NewNetwork creates a new network instance wrapping a glueberry node.
func NewNetwork(node *glueberry.Node) *Network {
	pm := NewPeerManager()
	registry := NewStreamRegistry()
	return &Network{
		node:          node,
		peerManager:   pm,
		scorer:        NewPeerScorer(pm),
		streamAdapter: NewGlueberryStreamAdapter(registry),
		tempBans:      make(map[peer.ID]*TempBanEntry),
		stopCh:        make(chan struct{}),
	}
}

// NewNetworkWithRegistry creates a new network instance with a custom stream registry.
func NewNetworkWithRegistry(node *glueberry.Node, registry StreamRegistry) *Network {
	pm := NewPeerManager()
	return &Network{
		node:          node,
		peerManager:   pm,
		scorer:        NewPeerScorer(pm),
		streamAdapter: NewGlueberryStreamAdapter(registry),
		tempBans:      make(map[peer.ID]*TempBanEntry),
		stopCh:        make(chan struct{}),
	}
}

// Start starts the network and begins processing events.
func (n *Network) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.started {
		return types.ErrNodeAlreadyStarted
	}

	// Start the underlying glueberry node
	if err := n.node.Start(); err != nil {
		return fmt.Errorf("starting glueberry node: %w", err)
	}

	// Get channels
	n.messages = n.node.Messages()
	n.events = n.node.Events()

	// Start penalty decay loop
	n.scorer.StartDecayLoop(n.stopCh)

	n.started = true
	return nil
}

// Stop stops the network and releases resources.
func (n *Network) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return types.ErrNodeNotStarted
	}

	// Signal stop to background goroutines
	close(n.stopCh)

	// Stop the underlying node
	if err := n.node.Stop(); err != nil {
		return fmt.Errorf("stopping glueberry node: %w", err)
	}

	n.started = false
	return nil
}

// Name returns the component name for identification.
func (n *Network) Name() string {
	return "network"
}

// IsRunning returns whether the network is running.
func (n *Network) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.started
}

// PeerID returns the local peer ID.
func (n *Network) PeerID() peer.ID {
	return n.node.PeerID()
}

// PublicKey returns the local Ed25519 public key.
func (n *Network) PublicKey() ed25519.PublicKey {
	return n.node.PublicKey()
}

// PeerManager returns the peer manager.
func (n *Network) PeerManager() *PeerManager {
	return n.peerManager
}

// Scorer returns the peer scorer.
func (n *Network) Scorer() *PeerScorer {
	return n.scorer
}

// Messages returns the channel for incoming messages.
func (n *Network) Messages() <-chan streams.IncomingMessage {
	return n.messages
}

// Events returns the channel for connection events.
func (n *Network) Events() <-chan glueberry.ConnectionEvent {
	return n.events
}

// Connect initiates a connection to a peer.
func (n *Network) Connect(peerID peer.ID) error {
	return n.node.Connect(peerID)
}

// ConnectMultiaddr initiates a connection to a peer using a multiaddr string.
func (n *Network) ConnectMultiaddr(addrStr string) error {
	maddr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return fmt.Errorf("parsing multiaddr: %w", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("extracting peer info from multiaddr: %w", err)
	}

	if err := n.node.AddPeer(addrInfo.ID, addrInfo.Addrs, nil); err != nil {
		return fmt.Errorf("adding peer: %w", err)
	}

	return nil
}

// Disconnect closes the connection to a peer.
func (n *Network) Disconnect(peerID peer.ID) error {
	return n.node.Disconnect(peerID)
}

// Send sends data to a specific peer on a specific stream.
func (n *Network) Send(peerID peer.ID, streamName string, data []byte) error {
	if err := n.node.Send(peerID, streamName, data); err != nil {
		return fmt.Errorf("sending to %s on %s: %w", peerID.String()[:8], streamName, err)
	}
	return nil
}

// Broadcast sends data to all connected peers on a specific stream.
func (n *Network) Broadcast(streamName string, data []byte) []error {
	peers := n.peerManager.AllPeerIDs()
	var errors []error

	for _, peerID := range peers {
		if err := n.Send(peerID, streamName, data); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// BroadcastTx broadcasts a transaction to peers that haven't seen it.
func (n *Network) BroadcastTx(txHash []byte, data []byte) []error {
	peers := n.peerManager.PeersToSendTx(txHash)
	var errors []error

	for _, peerID := range peers {
		if err := n.Send(peerID, StreamTransactions, data); err != nil {
			errors = append(errors, err)
		} else {
			// Peer is known to exist since PeersToSendTx returned it
			_ = n.peerManager.MarkTxSent(peerID, txHash)
		}
	}
	return errors
}

// BroadcastBlock broadcasts a block to peers that haven't seen it.
func (n *Network) BroadcastBlock(height int64, data []byte) []error {
	peers := n.peerManager.PeersToSendBlock(height)
	var errors []error

	for _, peerID := range peers {
		if err := n.Send(peerID, StreamBlocks, data); err != nil {
			errors = append(errors, err)
		} else {
			// Peer is known to exist since PeersToSendBlock returned it
			_ = n.peerManager.MarkBlockSent(peerID, height)
		}
	}
	return errors
}

// PrepareStreams prepares encrypted streams for a peer after receiving their public key.
// Uses streams from the stream registry if configured, otherwise uses default streams.
func (n *Network) PrepareStreams(peerID peer.ID, peerPubKey ed25519.PublicKey) error {
	streamNames := n.getStreamNames()
	return n.node.PrepareStreams(peerID, peerPubKey, streamNames)
}

// FinalizeHandshake completes the handshake and transitions to established state.
func (n *Network) FinalizeHandshake(peerID peer.ID) error {
	return n.node.FinalizeHandshake(peerID)
}

// CompleteHandshake performs PrepareStreams and FinalizeHandshake in one step.
// Uses streams from the stream registry if configured, otherwise uses default streams.
func (n *Network) CompleteHandshake(peerID peer.ID, peerPubKey ed25519.PublicKey) error {
	streamNames := n.getStreamNames()
	return n.node.CompleteHandshake(peerID, peerPubKey, streamNames)
}

// getStreamNames returns stream names to use for handshake.
// If the registry has streams registered, uses those. Otherwise uses defaults.
func (n *Network) getStreamNames() []string {
	if n.streamAdapter != nil {
		names := n.streamAdapter.GetEncryptedStreamNames()
		if len(names) > 0 {
			return names
		}
	}
	return AllStreams()
}

// BlacklistPeer blacklists a peer permanently and disconnects them.
// Use TempBanPeer for temporary bans (e.g., chain/version mismatch).
func (n *Network) BlacklistPeer(peerID peer.ID) error {
	// Remove from peer manager
	n.peerManager.RemovePeer(peerID)

	// Record the ban
	n.scorer.RecordBan(peerID)

	// Blacklist in glueberry
	return n.node.BlacklistPeer(peerID)
}

// TempBanPeer temporarily bans a peer for a specified duration.
// Unlike BlacklistPeer, this is not permanent and the peer can reconnect after the duration.
// Use this for recoverable issues like chain/version mismatch.
func (n *Network) TempBanPeer(peerID peer.ID, duration time.Duration, reason string) error {
	n.tempBansMu.Lock()
	n.tempBans[peerID] = &TempBanEntry{
		ExpiresAt: time.Now().Add(duration),
		Reason:    reason,
	}
	n.tempBansMu.Unlock()

	// Remove from peer manager
	n.peerManager.RemovePeer(peerID)

	// Record the ban for scoring purposes
	n.scorer.RecordBan(peerID)

	// Disconnect the peer (but don't permanently blacklist)
	return n.node.Disconnect(peerID)
}

// IsTempBanned checks if a peer is currently temporarily banned.
func (n *Network) IsTempBanned(peerID peer.ID) bool {
	n.tempBansMu.RLock()
	defer n.tempBansMu.RUnlock()

	entry, ok := n.tempBans[peerID]
	if !ok {
		return false
	}
	return time.Now().Before(entry.ExpiresAt)
}

// GetTempBanReason returns the reason for a temporary ban, or empty string if not banned.
func (n *Network) GetTempBanReason(peerID peer.ID) string {
	n.tempBansMu.RLock()
	defer n.tempBansMu.RUnlock()

	entry, ok := n.tempBans[peerID]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return ""
	}
	return entry.Reason
}

// CleanupExpiredTempBans removes expired temporary bans.
func (n *Network) CleanupExpiredTempBans() int {
	n.tempBansMu.Lock()
	defer n.tempBansMu.Unlock()

	now := time.Now()
	removed := 0
	for peerID, entry := range n.tempBans {
		if now.After(entry.ExpiresAt) {
			delete(n.tempBans, peerID)
			removed++
		}
	}
	return removed
}

// AddPenalty adds penalty points to a peer and blacklists if threshold exceeded.
func (n *Network) AddPenalty(peerID peer.ID, points int64, reason PenaltyReason, message string) error {
	n.scorer.AddPenalty(peerID, points, reason, message)

	if n.scorer.ShouldBan(peerID) {
		return n.BlacklistPeer(peerID)
	}
	return nil
}

// OnPeerConnected should be called when a peer connects.
func (n *Network) OnPeerConnected(peerID peer.ID, isOutbound bool) {
	n.peerManager.AddPeer(peerID, isOutbound)
}

// OnPeerDisconnected should be called when a peer disconnects.
func (n *Network) OnPeerDisconnected(peerID peer.ID) {
	n.peerManager.RemovePeer(peerID)
}

// OnTxReceived should be called when a transaction is received from a peer.
func (n *Network) OnTxReceived(peerID peer.ID, txHash []byte) {
	// Ignore errors - peer may have disconnected
	_ = n.peerManager.MarkTxReceived(peerID, txHash)
	n.peerManager.UpdateLastSeen(peerID)
}

// OnBlockReceived should be called when a block is received from a peer.
func (n *Network) OnBlockReceived(peerID peer.ID, height int64) {
	// Ignore errors - peer may have disconnected
	_ = n.peerManager.MarkBlockReceived(peerID, height)
	n.peerManager.UpdateLastSeen(peerID)
}

// ConnectionState returns the connection state for a peer.
func (n *Network) ConnectionState(peerID peer.ID) glueberry.ConnectionState {
	return n.node.ConnectionState(peerID)
}

// PeerCount returns the number of connected peers.
func (n *Network) PeerCount() int {
	return n.peerManager.PeerCount()
}

// StreamRegistry returns the stream registry.
func (n *Network) StreamRegistry() StreamRegistry {
	if n.streamAdapter != nil {
		return n.streamAdapter.Registry()
	}
	return nil
}

// StreamAdapter returns the stream adapter for direct access.
func (n *Network) StreamAdapter() *GlueberryStreamAdapter {
	return n.streamAdapter
}

// RegisterStream registers a new stream with the network.
// The stream will be included in future handshakes with new peers.
// For existing peers, use RefreshPeerStreams to update their streams.
// Returns ErrStreamAlreadyRegistered if the stream is already registered.
func (n *Network) RegisterStream(cfg StreamConfig, handler StreamHandler) error {
	if n.streamAdapter == nil {
		return fmt.Errorf("stream adapter not initialized")
	}

	// Register the stream configuration
	if err := n.streamAdapter.RegisterStream(cfg); err != nil {
		return err
	}

	// Register the handler if provided
	if handler != nil {
		if err := n.streamAdapter.SetHandler(cfg.Name, handler); err != nil {
			// Rollback stream registration on handler error
			_ = n.streamAdapter.UnregisterStream(cfg.Name)
			return err
		}
	}

	return nil
}

// UnregisterStream removes a stream from the network.
// Returns ErrStreamNotFound if the stream is not registered.
// Returns ErrStreamInUse if the stream has an active handler.
func (n *Network) UnregisterStream(name string) error {
	if n.streamAdapter == nil {
		return fmt.Errorf("stream adapter not initialized")
	}

	// Clear the handler first
	if err := n.streamAdapter.SetHandler(name, nil); err != nil {
		return err
	}

	// Then unregister the stream
	return n.streamAdapter.UnregisterStream(name)
}

// SetStreamHandler sets the message handler for a registered stream.
// Returns ErrStreamNotFound if the stream is not registered.
func (n *Network) SetStreamHandler(name string, handler StreamHandler) error {
	if n.streamAdapter == nil {
		return fmt.Errorf("stream adapter not initialized")
	}
	return n.streamAdapter.SetHandler(name, handler)
}

// GetStreamHandler returns the handler for a stream.
// Returns nil if no handler is registered.
func (n *Network) GetStreamHandler(name string) StreamHandler {
	if n.streamAdapter == nil {
		return nil
	}
	return n.streamAdapter.Registry().GetHandler(name)
}

// HasStream returns true if the stream is registered.
func (n *Network) HasStream(name string) bool {
	if n.streamAdapter == nil {
		return false
	}
	return n.streamAdapter.HasStream(name)
}

// GetStreamConfig returns the configuration for a stream.
func (n *Network) GetStreamConfig(name string) *StreamConfig {
	if n.streamAdapter == nil {
		return nil
	}
	return n.streamAdapter.GetStreamConfig(name)
}

// RegisteredStreams returns all registered stream names.
func (n *Network) RegisteredStreams() []string {
	if n.streamAdapter == nil {
		return AllStreams()
	}
	names := n.streamAdapter.GetAllStreamNames()
	if len(names) == 0 {
		return AllStreams()
	}
	return names
}

// RouteMessage routes an incoming message to its registered handler.
// Returns an error if no handler is registered for the stream.
func (n *Network) RouteMessage(msg streams.IncomingMessage) error {
	if n.streamAdapter == nil {
		return fmt.Errorf("stream adapter not initialized")
	}
	return n.streamAdapter.RouteMessage(msg)
}

// RegisterBuiltinStreams registers the core blockberry streams with handlers.
// This is typically called during node initialization.
func (n *Network) RegisterBuiltinStreams() error {
	if n.streamAdapter == nil {
		return fmt.Errorf("stream adapter not initialized")
	}
	return RegisterBuiltinStreams(n.streamAdapter.Registry())
}

// UnregisterStreamsByOwner removes all streams owned by the given owner.
// Returns the number of streams unregistered.
func (n *Network) UnregisterStreamsByOwner(owner string) int {
	if n.streamAdapter == nil {
		return 0
	}
	return n.streamAdapter.UnregisterByOwner(owner)
}

// ConnectedPeers returns the list of connected peer IDs.
// This implements the MempoolNetwork interface.
func (n *Network) ConnectedPeers() []peer.ID {
	return n.peerManager.AllPeerIDs()
}
