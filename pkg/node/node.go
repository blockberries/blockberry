package node

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blockberries/glueberry"
	"github.com/blockberries/glueberry/pkg/streams"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/blockberries/blockberry/pkg/blockstore"
	"github.com/blockberries/blockberry/pkg/config"
	"github.com/blockberries/blockberry/internal/container"
	"github.com/blockberries/blockberry/internal/handlers"
	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/blockberry/internal/p2p"
	"github.com/blockberries/blockberry/internal/pex"
	bsync "github.com/blockberries/blockberry/internal/sync"
	"github.com/blockberries/blockberry/pkg/types"
)

// Component names for dependency injection container.
const (
	ComponentNetwork      = "network"
	ComponentHandshake    = "handshake-handler"
	ComponentPEX          = "pex-reactor"
	ComponentHousekeeping = "housekeeping-reactor"
	ComponentTransactions = "transactions-reactor"
	ComponentBlocks       = "block-reactor"
	ComponentConsensus    = "consensus-reactor"
	ComponentSync         = "sync-reactor"
)

// DefaultShutdownTimeout is the maximum time to wait for the event loop to drain during shutdown.
const DefaultShutdownTimeout = 5 * time.Second

// Node is the main coordinator for a blockberry node.
// It aggregates all components and manages their lifecycle.
type Node struct {
	// Configuration
	cfg        *config.Config
	privateKey ed25519.PrivateKey
	nodeID     string
	role       types.NodeRole

	// Core network
	glueNode *glueberry.Node
	network  *p2p.Network

	// Stores
	blockStore blockstore.BlockStore
	mempool    mempool.Mempool

	// Handlers
	handshakeHandler    *handlers.HandshakeHandler
	transactionsReactor *handlers.TransactionsReactor
	blocksReactor       *handlers.BlockReactor
	consensusReactor    *handlers.ConsensusReactor
	housekeepingReactor *handlers.HousekeepingReactor
	pexReactor          *pex.Reactor
	syncReactor         *bsync.SyncReactor

	// Callbacks for event notifications
	callbacks *types.NodeCallbacks

	// Lifecycle
	started  bool
	stopping atomic.Bool // true when shutdown is in progress
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// Option is a functional option for configuring a Node.
type Option func(*Node)

// WithMempool sets a custom mempool.
func WithMempool(mp mempool.Mempool) Option {
	return func(n *Node) {
		n.mempool = mp
	}
}

// WithBlockStore sets a custom block store.
func WithBlockStore(bs blockstore.BlockStore) Option {
	return func(n *Node) {
		n.blockStore = bs
	}
}

// WithConsensusHandler sets the consensus handler.
func WithConsensusHandler(ch handlers.ConsensusHandler) Option {
	return func(n *Node) {
		if n.consensusReactor != nil {
			n.consensusReactor.SetHandler(ch)
		}
	}
}

// WithBlockValidator sets the block validation function.
// This is REQUIRED for the node to start - blocks will not be accepted
// without a validator (fail-closed behavior).
func WithBlockValidator(v types.BlockValidator) Option {
	return func(n *Node) {
		if n.syncReactor != nil {
			n.syncReactor.SetValidator(v)
		}
	}
}

// WithCallbacks sets the node callbacks for event notifications.
// Callbacks allow external code to react to node events without
// tight coupling to internal components.
func WithCallbacks(cb *types.NodeCallbacks) Option {
	return func(n *Node) {
		n.callbacks = cb
	}
}

// NewNode creates a new blockberry node with the given configuration.
// The node is not started until Start() is called.
func NewNode(cfg *config.Config, opts ...Option) (*Node, error) {
	// Load or generate private key
	privateKey, err := loadOrGenerateKey(cfg.Node.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading private key: %w", err)
	}

	// Create node ID from public key
	publicKey, ok := privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("unexpected public key type")
	}
	nodeID := hex.EncodeToString(publicKey)

	// Parse listen addresses
	listenAddrs, err := parseMultiaddrs(cfg.Network.ListenAddrs)
	if err != nil {
		return nil, fmt.Errorf("parsing listen addresses: %w", err)
	}

	// Create glueberry config
	glueCfg := glueberry.NewConfig(
		privateKey,
		cfg.Network.AddressBookPath,
		listenAddrs,
		glueberry.WithHandshakeTimeout(cfg.Network.HandshakeTimeout.Duration()),
	)

	// Create glueberry node
	glueNode, err := glueberry.New(glueCfg)
	if err != nil {
		return nil, fmt.Errorf("creating glueberry node: %w", err)
	}

	// Create network wrapper
	network := p2p.NewNetwork(glueNode)

	// Create default stores
	blockStore, err := blockstore.NewLevelDBBlockStore(cfg.BlockStore.Path)
	if err != nil {
		return nil, fmt.Errorf("creating block store: %w", err)
	}

	mp := mempool.NewMempool(cfg.Mempool)

	// Create node
	n := &Node{
		cfg:        cfg,
		privateKey: privateKey,
		nodeID:     nodeID,
		glueNode:   glueNode,
		network:    network,
		blockStore: blockStore,
		mempool:    mp,
		stopCh:     make(chan struct{}),
	}

	// Apply options before creating reactors
	for _, opt := range opts {
		opt(n)
	}

	// Create address book for PEX
	addressBook := pex.NewAddressBook(cfg.Network.AddressBookPath)

	// Add seed nodes to address book
	addressBook.AddSeeds(cfg.Network.Seeds.Addrs)

	peerManager := network.PeerManager()

	// Create handlers
	n.handshakeHandler = handlers.NewHandshakeHandler(
		cfg.Node.ChainID,
		cfg.Node.ProtocolVersion,
		nodeID,
		publicKey,
		network,
		peerManager,
		func() int64 { return n.blockStore.Height() },
	)

	n.transactionsReactor = handlers.NewTransactionsReactor(
		n.mempool,
		network,
		peerManager,
		5*time.Second, // request interval
		100,           // batch size
	)

	n.blocksReactor = handlers.NewBlockReactor(
		n.blockStore,
		network,
		peerManager,
	)

	n.consensusReactor = handlers.NewConsensusReactor(
		network,
		peerManager,
	)

	n.housekeepingReactor = handlers.NewHousekeepingReactor(
		network,
		peerManager,
		cfg.Housekeeping.LatencyProbeInterval.Duration(),
	)

	n.pexReactor = pex.NewReactor(
		cfg.PEX.Enabled,
		cfg.PEX.RequestInterval.Duration(),
		cfg.PEX.MaxAddressesPerResponse,
		addressBook,
		network,
		peerManager,
		cfg.Network.MaxInboundPeers,
		cfg.Network.MaxOutboundPeers,
	)

	n.syncReactor = bsync.NewSyncReactor(
		n.blockStore,
		network,
		peerManager,
		5*time.Second, // sync interval
		100,           // batch size
	)

	// Re-apply options to set consensus handler after reactor is created
	for _, opt := range opts {
		opt(n)
	}

	return n, nil
}

// Start starts the node and all its components.
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.started {
		return types.ErrNodeAlreadyStarted
	}

	// Start network
	if err := n.network.Start(); err != nil {
		return fmt.Errorf("starting network: %w", err)
	}

	// Start handshake handler
	if err := n.handshakeHandler.Start(); err != nil {
		_ = n.network.Stop()
		return fmt.Errorf("starting handshake handler: %w", err)
	}

	// Start reactors
	if err := n.pexReactor.Start(); err != nil {
		_ = n.handshakeHandler.Stop()
		_ = n.network.Stop()
		return fmt.Errorf("starting PEX reactor: %w", err)
	}

	if err := n.housekeepingReactor.Start(); err != nil {
		_ = n.pexReactor.Stop()
		_ = n.handshakeHandler.Stop()
		_ = n.network.Stop()
		return fmt.Errorf("starting housekeeping reactor: %w", err)
	}

	if err := n.transactionsReactor.Start(); err != nil {
		_ = n.housekeepingReactor.Stop()
		_ = n.pexReactor.Stop()
		_ = n.handshakeHandler.Stop()
		_ = n.network.Stop()
		return fmt.Errorf("starting transactions reactor: %w", err)
	}

	if err := n.syncReactor.Start(); err != nil {
		_ = n.transactionsReactor.Stop()
		_ = n.housekeepingReactor.Stop()
		_ = n.pexReactor.Stop()
		_ = n.handshakeHandler.Stop()
		_ = n.network.Stop()
		return fmt.Errorf("starting sync reactor: %w", err)
	}

	// Start event loop
	n.stopCh = make(chan struct{})
	n.wg.Add(1)
	go n.eventLoop()

	// Connect to seed nodes
	n.connectToSeeds()

	n.started = true
	return nil
}

// Stop stops the node and all its components.
// It uses a proper shutdown sequence to prevent race conditions:
// 1. Signal shutdown in progress (prevents new message handling)
// 2. Signal event loop to stop and wait for it to drain (with timeout)
// 3. Stop all reactors
// 4. Stop network and close stores
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return types.ErrNodeNotStarted
	}

	// 1. Signal that shutdown is in progress - event loop will stop dispatching to reactors
	n.stopping.Store(true)

	// 2. Signal event loop to stop
	close(n.stopCh)

	// 3. Wait for event loop to drain with timeout to prevent hangs
	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
	defer cancel()

	select {
	case <-done:
		// Event loop drained cleanly
	case <-ctx.Done():
		// Timeout - proceed with shutdown anyway
		// This is a safety mechanism to prevent indefinite hangs
	}

	// 4. Now safe to stop reactors and handlers (event loop is either done or timed out)
	_ = n.syncReactor.Stop()
	_ = n.transactionsReactor.Stop()
	_ = n.housekeepingReactor.Stop()
	_ = n.pexReactor.Stop()
	_ = n.handshakeHandler.Stop()

	// 5. Stop network
	if err := n.network.Stop(); err != nil {
		return fmt.Errorf("stopping network: %w", err)
	}

	// 6. Close stores
	if err := n.blockStore.Close(); err != nil {
		return fmt.Errorf("closing block store: %w", err)
	}

	n.started = false
	return nil
}

// IsRunning returns whether the node is running.
func (n *Node) IsRunning() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.started
}

// PeerID returns the node's peer ID.
func (n *Node) PeerID() peer.ID {
	return n.network.PeerID()
}

// NodeID returns the node's hex-encoded public key ID.
func (n *Node) NodeID() string {
	return n.nodeID
}

// Role returns the node's role.
// Returns the default role (full) if no role was explicitly set.
func (n *Node) Role() types.NodeRole {
	if n.role == "" {
		return types.DefaultRole()
	}
	return n.role
}

// Network returns the network layer.
func (n *Node) Network() *p2p.Network {
	return n.network
}

// BlockStore returns the block store.
func (n *Node) BlockStore() blockstore.BlockStore {
	return n.blockStore
}

// Mempool returns the mempool.
func (n *Node) Mempool() mempool.Mempool {
	return n.mempool
}

// PeerCount returns the number of connected peers.
func (n *Node) PeerCount() int {
	return n.network.PeerCount()
}

// Callbacks returns the current node callbacks.
// Returns nil if no callbacks are set.
func (n *Node) Callbacks() *types.NodeCallbacks {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.callbacks
}

// SetCallbacks sets the node callbacks for event notifications.
// Can be called at any time, including after the node has started.
// Pass nil to remove callbacks.
func (n *Node) SetCallbacks(cb *types.NodeCallbacks) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.callbacks = cb
}

// eventLoop handles incoming events and messages.
func (n *Node) eventLoop() {
	defer n.wg.Done()

	events := n.network.Events()
	messages := n.network.Messages()

	for {
		select {
		case <-n.stopCh:
			return

		case event, ok := <-events:
			if !ok {
				return
			}
			n.handleConnectionEvent(event)

		case msg, ok := <-messages:
			if !ok {
				return
			}
			n.handleMessage(msg)
		}
	}
}

// handleConnectionEvent processes connection state changes.
func (n *Node) handleConnectionEvent(event glueberry.ConnectionEvent) {
	// Skip processing if shutdown is in progress
	if n.stopping.Load() {
		return
	}

	peerID := event.PeerID

	// Capture callbacks under lock to avoid race with SetCallbacks
	n.mu.RLock()
	cb := n.callbacks
	n.mu.RUnlock()

	switch event.State {
	case glueberry.StateConnected:
		// Query actual connection direction from Glueberry
		isOutbound, err := n.glueNode.IsOutbound(peerID)
		if err != nil {
			// If we can't determine direction, assume inbound for safety
			// (inbound connections are more restricted)
			isOutbound = false
		}

		// Check peer limits before proceeding with handshake
		if isOutbound {
			if n.network.PeerManager().OutboundPeerCount() >= n.cfg.Network.MaxOutboundPeers {
				_ = n.network.Disconnect(peerID)
				return
			}
		} else {
			if n.network.PeerManager().InboundPeerCount() >= n.cfg.Network.MaxInboundPeers {
				_ = n.network.Disconnect(peerID)
				return
			}
		}

		// Initiate handshake with correct direction
		n.network.OnPeerConnected(peerID, isOutbound)
		_ = n.handshakeHandler.OnPeerConnected(peerID, isOutbound)

		// Invoke callback
		cb.InvokePeerConnected(peerID, isOutbound)

	case glueberry.StateEstablished:
		// Peer is fully connected, notify reactors
		info := n.handshakeHandler.GetPeerInfo(peerID)
		if info != nil {
			n.syncReactor.OnPeerConnected(peerID, info.PeerHeight)

			// Get connection direction for PEX
			isOutbound, _ := n.glueNode.IsOutbound(peerID)

			// Get peer's multiaddr from libp2p peerstore for PEX
			// This works for both outbound (in address book) and inbound connections
			peerMultiaddr := ""
			if addrs := n.glueNode.PeerAddrs(peerID); len(addrs) > 0 {
				peerMultiaddr = fmt.Sprintf("%s/p2p/%s", addrs[0].String(), peerID.String())
			}
			n.pexReactor.OnPeerConnected(peerID, peerMultiaddr, isOutbound)

			// Invoke callback with peer info
			cb.InvokePeerHandshaked(peerID, &types.PeerInfo{
				NodeID:          info.PeerNodeID,
				ChainID:         info.PeerChainID,
				ProtocolVersion: info.PeerVersion,
				Height:          info.PeerHeight,
			})
		}

	case glueberry.StateDisconnected:
		// Invoke callback before cleanup
		cb.InvokePeerDisconnected(peerID)

		// Clean up peer state
		n.network.OnPeerDisconnected(peerID)
		n.handshakeHandler.OnPeerDisconnected(peerID)
		n.transactionsReactor.OnPeerDisconnected(peerID)
		n.blocksReactor.OnPeerDisconnected(peerID)
		n.consensusReactor.OnPeerDisconnected(peerID)
		n.housekeepingReactor.OnPeerDisconnected(peerID)
		n.pexReactor.OnPeerDisconnected(peerID)
		n.syncReactor.OnPeerDisconnected(peerID)
	}
}

// handleMessage routes messages to the appropriate handler.
func (n *Node) handleMessage(msg streams.IncomingMessage) {
	// Skip processing if shutdown is in progress
	if n.stopping.Load() {
		return
	}

	peerID := msg.PeerID
	data := msg.Data
	stream := msg.StreamName

	// Capture callbacks under lock to avoid race with SetCallbacks
	n.mu.RLock()
	cb := n.callbacks
	n.mu.RUnlock()

	var err error
	switch stream {
	case p2p.StreamHandshake:
		err = n.handshakeHandler.HandleMessage(peerID, data)
	case p2p.StreamPEX:
		err = n.pexReactor.HandleMessage(peerID, data)
	case p2p.StreamTransactions:
		err = n.transactionsReactor.HandleMessage(peerID, data)
	case p2p.StreamBlockSync:
		err = n.syncReactor.HandleMessage(peerID, data)
	case p2p.StreamBlocks:
		err = n.blocksReactor.HandleMessage(peerID, data)
	case p2p.StreamConsensus:
		// Invoke callback for consensus messages (allows pluggable consensus)
		cb.InvokeConsensusMessage(peerID, data)
		err = n.consensusReactor.HandleMessage(peerID, data)
	case p2p.StreamHousekeeping:
		err = n.housekeepingReactor.HandleMessage(peerID, data)
	}

	if err != nil {
		// Add penalty for invalid messages
		_ = n.network.AddPenalty(peerID, p2p.PenaltyInvalidMessage, p2p.ReasonInvalidMessage,
			fmt.Sprintf("error handling %s message: %v", stream, err))
		// Invoke penalty callback
		cb.InvokePeerPenalized(peerID, p2p.PenaltyInvalidMessage, string(p2p.ReasonInvalidMessage))
	}
}

// connectToSeeds connects to configured seed nodes.
func (n *Node) connectToSeeds() {
	for _, addr := range n.cfg.Network.Seeds.Addrs {
		if err := n.network.ConnectMultiaddr(addr); err != nil {
			// Log and continue - seed connection failure is not fatal
			continue
		}
	}
}

// loadOrGenerateKey loads a private key from file or generates a new one.
func loadOrGenerateKey(path string) (ed25519.PrivateKey, error) {
	// Try to load existing key
	data, err := os.ReadFile(path)
	if err == nil {
		if len(data) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(data), nil
		}
		// Try hex-encoded key
		decoded, decodeErr := hex.DecodeString(string(data))
		if decodeErr == nil && len(decoded) == ed25519.PrivateKeySize {
			return ed25519.PrivateKey(decoded), nil
		}
		return nil, fmt.Errorf("invalid key file: expected %d bytes", ed25519.PrivateKeySize)
	}

	// Generate new key
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	_, privateKey, genErr := ed25519.GenerateKey(nil)
	if genErr != nil {
		return nil, fmt.Errorf("generating key: %w", genErr)
	}

	// Save key
	if writeErr := os.WriteFile(path, []byte(privateKey), 0600); writeErr != nil {
		return nil, fmt.Errorf("saving key: %w", writeErr)
	}

	return privateKey, nil
}

// parseMultiaddrs parses a slice of multiaddr strings.
func parseMultiaddrs(addrs []string) ([]multiaddr.Multiaddr, error) {
	result := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("parsing multiaddr %q: %w", addr, err)
		}
		result = append(result, ma)
	}
	return result, nil
}

// ComponentContainer returns a dependency injection container with all
// node components registered. This allows users to access components
// by name and introspect the component graph.
//
// The container returned is for inspection purposes - the Node still
// manages its own lifecycle. To use container-managed lifecycle,
// create a custom Node setup using the container directly.
//
// Component dependencies:
//   - handshake-handler: depends on network
//   - pex-reactor: depends on network, handshake-handler
//   - housekeeping-reactor: depends on network
//   - transactions-reactor: depends on network
//   - block-reactor: depends on network
//   - consensus-reactor: depends on network
//   - sync-reactor: depends on network, block-reactor
func (n *Node) ComponentContainer() (*container.Container, error) {
	c := container.New()

	// Register network (no dependencies)
	if err := c.Register(ComponentNetwork, n.network); err != nil {
		return nil, fmt.Errorf("registering network: %w", err)
	}

	// Register handshake handler (depends on network)
	if err := c.Register(ComponentHandshake, n.handshakeHandler, ComponentNetwork); err != nil {
		return nil, fmt.Errorf("registering handshake handler: %w", err)
	}

	// Register PEX reactor (depends on network, handshake)
	if err := c.Register(ComponentPEX, n.pexReactor, ComponentNetwork, ComponentHandshake); err != nil {
		return nil, fmt.Errorf("registering PEX reactor: %w", err)
	}

	// Register housekeeping reactor (depends on network)
	if err := c.Register(ComponentHousekeeping, n.housekeepingReactor, ComponentNetwork); err != nil {
		return nil, fmt.Errorf("registering housekeeping reactor: %w", err)
	}

	// Register transactions reactor (depends on network)
	if err := c.Register(ComponentTransactions, n.transactionsReactor, ComponentNetwork); err != nil {
		return nil, fmt.Errorf("registering transactions reactor: %w", err)
	}

	// Register blocks reactor (depends on network)
	if err := c.Register(ComponentBlocks, n.blocksReactor, ComponentNetwork); err != nil {
		return nil, fmt.Errorf("registering blocks reactor: %w", err)
	}

	// Register consensus reactor (depends on network)
	if err := c.Register(ComponentConsensus, n.consensusReactor, ComponentNetwork); err != nil {
		return nil, fmt.Errorf("registering consensus reactor: %w", err)
	}

	// Register sync reactor (depends on network, blocks)
	if err := c.Register(ComponentSync, n.syncReactor, ComponentNetwork, ComponentBlocks); err != nil {
		return nil, fmt.Errorf("registering sync reactor: %w", err)
	}

	return c, nil
}

// GetComponent returns a component by name, allowing type-safe access
// to node components. Returns an error if the component doesn't exist.
func (n *Node) GetComponent(name string) (types.Component, error) {
	switch name {
	case ComponentNetwork:
		return n.network, nil
	case ComponentHandshake:
		return n.handshakeHandler, nil
	case ComponentPEX:
		return n.pexReactor, nil
	case ComponentHousekeeping:
		return n.housekeepingReactor, nil
	case ComponentTransactions:
		return n.transactionsReactor, nil
	case ComponentBlocks:
		return n.blocksReactor, nil
	case ComponentConsensus:
		return n.consensusReactor, nil
	case ComponentSync:
		return n.syncReactor, nil
	default:
		return nil, fmt.Errorf("%w: %s", container.ErrComponentNotFound, name)
	}
}

// ComponentNames returns the names of all components in the node.
func (n *Node) ComponentNames() []string {
	return []string{
		ComponentNetwork,
		ComponentHandshake,
		ComponentPEX,
		ComponentHousekeeping,
		ComponentTransactions,
		ComponentBlocks,
		ComponentConsensus,
		ComponentSync,
	}
}
