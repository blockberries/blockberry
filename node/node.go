package node

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"github.com/blockberries/glueberry"
	"github.com/blockberries/glueberry/pkg/streams"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/blockberries/blockberry/blockstore"
	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/handlers"
	"github.com/blockberries/blockberry/mempool"
	"github.com/blockberries/blockberry/p2p"
	"github.com/blockberries/blockberry/pex"
	bsync "github.com/blockberries/blockberry/sync"
	"github.com/blockberries/blockberry/types"
)

// Node is the main coordinator for a blockberry node.
// It aggregates all components and manages their lifecycle.
type Node struct {
	// Configuration
	cfg        *config.Config
	privateKey ed25519.PrivateKey
	nodeID     string

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

	// Lifecycle
	started bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
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
		5000, // request interval ms - could be configurable
		100,  // batch size
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
		5000, // sync interval ms - could be configurable
		100,  // batch size
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

	// Start reactors
	if err := n.pexReactor.Start(); err != nil {
		_ = n.network.Stop()
		return fmt.Errorf("starting PEX reactor: %w", err)
	}

	if err := n.housekeepingReactor.Start(); err != nil {
		_ = n.pexReactor.Stop()
		_ = n.network.Stop()
		return fmt.Errorf("starting housekeeping reactor: %w", err)
	}

	if err := n.transactionsReactor.Start(); err != nil {
		_ = n.housekeepingReactor.Stop()
		_ = n.pexReactor.Stop()
		_ = n.network.Stop()
		return fmt.Errorf("starting transactions reactor: %w", err)
	}

	if err := n.syncReactor.Start(); err != nil {
		_ = n.transactionsReactor.Stop()
		_ = n.housekeepingReactor.Stop()
		_ = n.pexReactor.Stop()
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
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return types.ErrNodeNotStarted
	}

	// Signal event loop to stop
	close(n.stopCh)
	n.wg.Wait()

	// Stop reactors in reverse order
	_ = n.syncReactor.Stop()
	_ = n.transactionsReactor.Stop()
	_ = n.housekeepingReactor.Stop()
	_ = n.pexReactor.Stop()

	// Stop network
	if err := n.network.Stop(); err != nil {
		return fmt.Errorf("stopping network: %w", err)
	}

	// Close stores
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
	peerID := event.PeerID

	switch event.State {
	case glueberry.StateConnected:
		// Initiate handshake
		n.network.OnPeerConnected(peerID, true) // TODO: determine if outbound
		_ = n.handshakeHandler.OnPeerConnected(peerID, true)

	case glueberry.StateEstablished:
		// Peer is fully connected, notify reactors
		info := n.handshakeHandler.GetPeerInfo(peerID)
		if info != nil {
			n.syncReactor.OnPeerConnected(peerID, info.PeerHeight)
			n.pexReactor.OnPeerConnected(peerID, "", true) // TODO: get multiaddr
		}

	case glueberry.StateDisconnected:
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
	peerID := msg.PeerID
	data := msg.Data
	stream := msg.StreamName

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
		err = n.consensusReactor.HandleMessage(peerID, data)
	case p2p.StreamHousekeeping:
		err = n.housekeepingReactor.HandleMessage(peerID, data)
	}

	if err != nil {
		// Add penalty for invalid messages
		_ = n.network.AddPenalty(peerID, p2p.PenaltyInvalidMessage, p2p.ReasonInvalidMessage,
			fmt.Sprintf("error handling %s message: %v", stream, err))
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
