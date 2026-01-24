package p2p

import (
	"crypto/ed25519"
	"fmt"
	"sync"

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
		StreamBlocks,
		StreamConsensus,
		StreamHousekeeping,
	}
}

// Network wraps glueberry.Node with blockberry-specific logic.
// It provides a higher-level API for blockchain networking.
type Network struct {
	node        *glueberry.Node
	peerManager *PeerManager
	scorer      *PeerScorer

	// Channels for incoming data
	messages <-chan streams.IncomingMessage
	events   <-chan glueberry.ConnectionEvent

	// Lifecycle
	started bool
	stopCh  chan struct{}
	mu      sync.RWMutex
}

// NewNetwork creates a new network instance wrapping a glueberry node.
func NewNetwork(node *glueberry.Node) *Network {
	pm := NewPeerManager()
	return &Network{
		node:        node,
		peerManager: pm,
		scorer:      NewPeerScorer(pm),
		stopCh:      make(chan struct{}),
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
func (n *Network) PrepareStreams(peerID peer.ID, peerPubKey ed25519.PublicKey) error {
	return n.node.PrepareStreams(peerID, peerPubKey, AllStreams())
}

// FinalizeHandshake completes the handshake and transitions to established state.
func (n *Network) FinalizeHandshake(peerID peer.ID) error {
	return n.node.FinalizeHandshake(peerID)
}

// CompleteHandshake performs PrepareStreams and FinalizeHandshake in one step.
func (n *Network) CompleteHandshake(peerID peer.ID, peerPubKey ed25519.PublicKey) error {
	return n.node.CompleteHandshake(peerID, peerPubKey, AllStreams())
}

// BlacklistPeer blacklists a peer and disconnects them.
func (n *Network) BlacklistPeer(peerID peer.ID) error {
	// Remove from peer manager
	n.peerManager.RemovePeer(peerID)

	// Record the ban
	n.scorer.RecordBan(peerID)

	// Blacklist in glueberry
	return n.node.BlacklistPeer(peerID)
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
