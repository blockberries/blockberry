// Package testing provides test utilities for blockberry integration tests.
package testing

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

// TestNode wraps a blockberry node with test utilities.
type TestNode struct {
	// Config
	cfg        *config.Config
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	nodeID     string
	dataDir    string

	// Core network
	GlueNode *glueberry.Node
	Network  *p2p.Network

	// Stores
	BlockStore blockstore.BlockStore
	Mempool    mempool.Mempool

	// Handlers
	HandshakeHandler    *handlers.HandshakeHandler
	TransactionsReactor *handlers.TransactionsReactor
	BlocksReactor       *handlers.BlockReactor
	ConsensusReactor    *handlers.ConsensusReactor
	HousekeepingReactor *handlers.HousekeepingReactor
	PexReactor          *pex.Reactor
	SyncReactor         *bsync.SyncReactor

	// Event notification for testing
	establishedNotify   map[peer.ID]chan struct{}
	establishedNotifyMu sync.Mutex

	// Lifecycle
	started bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mu      sync.RWMutex
}

// TestNodeConfig holds configuration options for creating a test node.
type TestNodeConfig struct {
	// ChainID is the chain identifier (default: "test-chain")
	ChainID string

	// ProtocolVersion is the protocol version (default: 1)
	ProtocolVersion int32

	// Seeds are seed node addresses to connect to
	Seeds []string
}

// DefaultTestNodeConfig returns a TestNodeConfig with default values.
func DefaultTestNodeConfig() *TestNodeConfig {
	return &TestNodeConfig{
		ChainID:         "test-chain",
		ProtocolVersion: 1,
		Seeds:           nil,
	}
}

// NewTestNode creates a new test node with random keys and ephemeral ports.
// The node is not started until Start() is called.
// Call Cleanup() when done to remove temporary files.
func NewTestNode(tc *TestNodeConfig) (*TestNode, error) {
	if tc == nil {
		tc = DefaultTestNodeConfig()
	}

	// Generate random keys
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating key: %w", err)
	}

	// Create temp directory
	dataDir, err := os.MkdirTemp("", "blockberry-test-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	// Write key to file
	keyPath := filepath.Join(dataDir, "node.key")
	if writeErr := os.WriteFile(keyPath, priv, 0600); writeErr != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("writing key file: %w", writeErr)
	}

	// Create config with ephemeral port
	cfg := config.DefaultConfig()
	cfg.Node.ChainID = tc.ChainID
	cfg.Node.ProtocolVersion = tc.ProtocolVersion
	cfg.Node.PrivateKeyPath = keyPath
	cfg.Network.ListenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
	cfg.Network.AddressBookPath = filepath.Join(dataDir, "addrbook.json")
	cfg.Network.HandshakeTimeout = config.Duration(30 * time.Second)
	cfg.BlockStore.Path = filepath.Join(dataDir, "blocks")
	cfg.StateStore.Path = filepath.Join(dataDir, "state")
	cfg.PEX.Enabled = true
	cfg.PEX.RequestInterval = config.Duration(5 * time.Second)
	cfg.Housekeeping.LatencyProbeInterval = config.Duration(10 * time.Second)

	if len(tc.Seeds) > 0 {
		cfg.Network.Seeds.Addrs = tc.Seeds
	}

	// Parse listen address
	listenAddr, err := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/0")
	if err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("parsing listen addr: %w", err)
	}

	// Create glueberry config
	glueCfg := glueberry.NewConfig(
		priv,
		cfg.Network.AddressBookPath,
		[]multiaddr.Multiaddr{listenAddr},
		glueberry.WithHandshakeTimeout(cfg.Network.HandshakeTimeout.Duration()),
	)

	// Create glueberry node
	glueNode, err := glueberry.New(glueCfg)
	if err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("creating glueberry node: %w", err)
	}

	// Create network wrapper
	network := p2p.NewNetwork(glueNode)

	// Create stores
	bs := blockstore.NewMemoryBlockStore()
	mp := mempool.NewSimpleMempool(cfg.Mempool.MaxTxs, cfg.Mempool.MaxBytes)

	// Create node
	nodeID := fmt.Sprintf("%x", pub)
	tn := &TestNode{
		cfg:               cfg,
		privateKey:        priv,
		publicKey:         pub,
		nodeID:            nodeID,
		dataDir:           dataDir,
		GlueNode:          glueNode,
		Network:           network,
		BlockStore:        bs,
		Mempool:           mp,
		establishedNotify: make(map[peer.ID]chan struct{}),
		stopCh:            make(chan struct{}),
	}

	// Create address book for PEX
	addressBook := pex.NewAddressBook(cfg.Network.AddressBookPath)
	if len(tc.Seeds) > 0 {
		addressBook.AddSeeds(tc.Seeds)
	}

	peerManager := network.PeerManager()

	// Create handlers
	tn.HandshakeHandler = handlers.NewHandshakeHandler(
		cfg.Node.ChainID,
		cfg.Node.ProtocolVersion,
		nodeID,
		pub,
		network,
		peerManager,
		func() int64 { return tn.BlockStore.Height() },
	)

	tn.TransactionsReactor = handlers.NewTransactionsReactor(
		mp,
		network,
		peerManager,
		1*time.Second, // request interval
		100,           // batch size
	)

	tn.BlocksReactor = handlers.NewBlockReactor(
		bs,
		network,
		peerManager,
	)

	tn.ConsensusReactor = handlers.NewConsensusReactor(
		network,
		peerManager,
	)

	tn.HousekeepingReactor = handlers.NewHousekeepingReactor(
		network,
		peerManager,
		cfg.Housekeeping.LatencyProbeInterval.Duration(),
	)

	tn.PexReactor = pex.NewReactor(
		cfg.PEX.Enabled,
		cfg.PEX.RequestInterval.Duration(),
		cfg.PEX.MaxAddressesPerResponse,
		addressBook,
		network,
		peerManager,
		cfg.Network.MaxInboundPeers,
		cfg.Network.MaxOutboundPeers,
	)

	tn.SyncReactor = bsync.NewSyncReactor(
		bs,
		network,
		peerManager,
		1*time.Second, // sync interval
		100,           // batch size
	)

	return tn, nil
}

// Start starts the test node and all its components.
func (tn *TestNode) Start() error {
	tn.mu.Lock()
	defer tn.mu.Unlock()

	if tn.started {
		return types.ErrNodeAlreadyStarted
	}

	// Start network
	if err := tn.Network.Start(); err != nil {
		return fmt.Errorf("starting network: %w", err)
	}

	// Start reactors
	if err := tn.PexReactor.Start(); err != nil {
		_ = tn.Network.Stop()
		return fmt.Errorf("starting PEX reactor: %w", err)
	}

	if err := tn.HousekeepingReactor.Start(); err != nil {
		_ = tn.PexReactor.Stop()
		_ = tn.Network.Stop()
		return fmt.Errorf("starting housekeeping reactor: %w", err)
	}

	if err := tn.TransactionsReactor.Start(); err != nil {
		_ = tn.HousekeepingReactor.Stop()
		_ = tn.PexReactor.Stop()
		_ = tn.Network.Stop()
		return fmt.Errorf("starting transactions reactor: %w", err)
	}

	if err := tn.SyncReactor.Start(); err != nil {
		_ = tn.TransactionsReactor.Stop()
		_ = tn.HousekeepingReactor.Stop()
		_ = tn.PexReactor.Stop()
		_ = tn.Network.Stop()
		return fmt.Errorf("starting sync reactor: %w", err)
	}

	// Start event loop
	tn.stopCh = make(chan struct{})
	tn.wg.Add(1)
	go tn.eventLoop()

	tn.started = true
	return nil
}

// Stop stops the test node and all its components.
func (tn *TestNode) Stop() error {
	tn.mu.Lock()
	defer tn.mu.Unlock()

	if !tn.started {
		return types.ErrNodeNotStarted
	}

	// Signal event loop to stop
	close(tn.stopCh)
	tn.wg.Wait()

	// Stop reactors in reverse order
	_ = tn.SyncReactor.Stop()
	_ = tn.TransactionsReactor.Stop()
	_ = tn.HousekeepingReactor.Stop()
	_ = tn.PexReactor.Stop()

	// Stop network
	if err := tn.Network.Stop(); err != nil {
		return fmt.Errorf("stopping network: %w", err)
	}

	// Close stores
	if err := tn.BlockStore.Close(); err != nil {
		return fmt.Errorf("closing block store: %w", err)
	}

	tn.started = false
	return nil
}

// Cleanup stops the node and removes temporary files.
func (tn *TestNode) Cleanup() error {
	if tn.IsRunning() {
		if err := tn.Stop(); err != nil {
			return err
		}
	}
	return os.RemoveAll(tn.dataDir)
}

// IsRunning returns whether the node is running.
func (tn *TestNode) IsRunning() bool {
	tn.mu.RLock()
	defer tn.mu.RUnlock()
	return tn.started
}

// PeerID returns the node's peer ID.
func (tn *TestNode) PeerID() peer.ID {
	return tn.Network.PeerID()
}

// NodeID returns the node's hex-encoded public key ID.
func (tn *TestNode) NodeID() string {
	return tn.nodeID
}

// PublicKey returns the node's Ed25519 public key.
func (tn *TestNode) PublicKey() ed25519.PublicKey {
	return tn.publicKey
}

// Multiaddr returns the node's listen multiaddr with peer ID.
func (tn *TestNode) Multiaddr() (string, error) {
	addrs := tn.GlueNode.Addrs()
	if len(addrs) == 0 {
		return "", fmt.Errorf("no listen addresses")
	}
	return fmt.Sprintf("%s/p2p/%s", addrs[0].String(), tn.PeerID().String()), nil
}

// ConnectTo connects this node to another test node.
func (tn *TestNode) ConnectTo(other *TestNode) error {
	addr, err := other.Multiaddr()
	if err != nil {
		return err
	}
	return tn.Network.ConnectMultiaddr(addr)
}

// RegisterForEstablished registers to be notified when StateEstablished is received for a peer.
// This should be called BEFORE initiating the connection to avoid missing the event.
// Returns a channel that will be closed when the connection reaches StateEstablished.
func (tn *TestNode) RegisterForEstablished(peerID peer.ID) <-chan struct{} {
	tn.establishedNotifyMu.Lock()
	defer tn.establishedNotifyMu.Unlock()

	// If already established, return a closed channel
	if tn.Network.ConnectionState(peerID) == glueberry.StateEstablished {
		ch := make(chan struct{})
		close(ch)
		return ch
	}

	ch := make(chan struct{})
	tn.establishedNotify[peerID] = ch
	return ch
}

// WaitForEstablished waits for a pre-registered channel to signal connection establishment.
// Use RegisterForEstablished to get the channel before connecting.
func (tn *TestNode) WaitForEstablished(ch <-chan struct{}, timeout time.Duration) error {
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("timeout waiting for connection")
	}
}

// UnregisterForEstablished removes the notification channel for a peer.
func (tn *TestNode) UnregisterForEstablished(peerID peer.ID) {
	tn.establishedNotifyMu.Lock()
	defer tn.establishedNotifyMu.Unlock()
	delete(tn.establishedNotify, peerID)
}

// ConnectAndWait connects to a peer and waits for the connection to be established.
// This properly registers for the established notification before connecting to avoid races.
func (tn *TestNode) ConnectAndWait(other *TestNode, timeout time.Duration) error {
	// Register for notification BEFORE connecting
	ch := tn.RegisterForEstablished(other.PeerID())

	// Connect
	if err := tn.ConnectTo(other); err != nil {
		return err
	}

	// Wait for established
	return tn.WaitForEstablished(ch, timeout)
}

// notifyEstablished notifies any waiters that a peer has reached StateEstablished.
func (tn *TestNode) notifyEstablished(peerID peer.ID) {
	tn.establishedNotifyMu.Lock()
	defer tn.establishedNotifyMu.Unlock()

	if ch, ok := tn.establishedNotify[peerID]; ok {
		close(ch)
		delete(tn.establishedNotify, peerID)
	}
}

// WaitForPeerCount waits for the node to have at least the specified number of peers.
func (tn *TestNode) WaitForPeerCount(count int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if tn.Network.PeerCount() >= count {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %d peers (have %d)", count, tn.Network.PeerCount())
}

// eventLoop handles incoming events and messages.
func (tn *TestNode) eventLoop() {
	defer tn.wg.Done()

	events := tn.Network.Events()
	messages := tn.Network.Messages()

	for {
		select {
		case <-tn.stopCh:
			return

		case event, ok := <-events:
			if !ok {
				return
			}
			tn.handleConnectionEvent(event)

		case msg, ok := <-messages:
			if !ok {
				return
			}
			tn.handleMessage(msg)
		}
	}
}

// handleConnectionEvent processes connection state changes.
func (tn *TestNode) handleConnectionEvent(event glueberry.ConnectionEvent) {
	peerID := event.PeerID

	switch event.State {
	case glueberry.StateConnected:
		tn.Network.OnPeerConnected(peerID, true)
		_ = tn.HandshakeHandler.OnPeerConnected(peerID, true)

	case glueberry.StateEstablished:
		// Notify any waiters that the connection is established
		tn.notifyEstablished(peerID)

		info := tn.HandshakeHandler.GetPeerInfo(peerID)
		if info != nil {
			tn.SyncReactor.OnPeerConnected(peerID, info.PeerHeight)

			// Get peer's multiaddr from libp2p peerstore for PEX
			// This works for both outbound (in address book) and inbound connections
			peerMultiaddr := ""
			if addrs := tn.GlueNode.PeerAddrs(peerID); len(addrs) > 0 {
				peerMultiaddr = fmt.Sprintf("%s/p2p/%s", addrs[0].String(), peerID.String())
			}
			tn.PexReactor.OnPeerConnected(peerID, peerMultiaddr, true)
		}

	case glueberry.StateDisconnected:
		tn.Network.OnPeerDisconnected(peerID)
		tn.HandshakeHandler.OnPeerDisconnected(peerID)
		tn.TransactionsReactor.OnPeerDisconnected(peerID)
		tn.BlocksReactor.OnPeerDisconnected(peerID)
		tn.ConsensusReactor.OnPeerDisconnected(peerID)
		tn.HousekeepingReactor.OnPeerDisconnected(peerID)
		tn.PexReactor.OnPeerDisconnected(peerID)
		tn.SyncReactor.OnPeerDisconnected(peerID)
	}
}

// handleMessage routes messages to the appropriate handler.
func (tn *TestNode) handleMessage(msg streams.IncomingMessage) {
	peerID := msg.PeerID
	data := msg.Data
	stream := msg.StreamName

	var err error
	switch stream {
	case p2p.StreamHandshake:
		err = tn.HandshakeHandler.HandleMessage(peerID, data)
	case p2p.StreamPEX:
		err = tn.PexReactor.HandleMessage(peerID, data)
	case p2p.StreamTransactions:
		err = tn.TransactionsReactor.HandleMessage(peerID, data)
	case p2p.StreamBlockSync:
		err = tn.SyncReactor.HandleMessage(peerID, data)
	case p2p.StreamBlocks:
		err = tn.BlocksReactor.HandleMessage(peerID, data)
	case p2p.StreamConsensus:
		err = tn.ConsensusReactor.HandleMessage(peerID, data)
	case p2p.StreamHousekeeping:
		err = tn.HousekeepingReactor.HandleMessage(peerID, data)
	}

	if err != nil {
		_ = tn.Network.AddPenalty(peerID, p2p.PenaltyInvalidMessage, p2p.ReasonInvalidMessage,
			fmt.Sprintf("error handling %s message: %v", stream, err))
	}
}

// MockApplication is a test application that tracks received events.
type MockApplication struct {
	mu                sync.Mutex
	CheckedTxs        [][]byte
	DeliveredTxs      [][]byte
	BlocksBegun       []int64
	BlocksEnded       []int64
	Commits           int
	ConsensusMessages []struct {
		PeerID peer.ID
		Data   []byte
	}
}

// NewMockApplication creates a new mock application.
func NewMockApplication() *MockApplication {
	return &MockApplication{}
}

// CheckTx records a transaction check.
func (m *MockApplication) CheckTx(tx []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CheckedTxs = append(m.CheckedTxs, tx)
	return nil
}

// BeginBlock records a block beginning.
func (m *MockApplication) BeginBlock(height int64, hash []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BlocksBegun = append(m.BlocksBegun, height)
	return nil
}

// DeliverTx records a transaction delivery.
func (m *MockApplication) DeliverTx(tx []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeliveredTxs = append(m.DeliveredTxs, tx)
	return nil
}

// EndBlock records a block end.
func (m *MockApplication) EndBlock() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.BlocksEnded = append(m.BlocksEnded, int64(len(m.BlocksBegun)))
	return nil
}

// Commit records a commit.
func (m *MockApplication) Commit() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Commits++
	return make([]byte, 32), nil
}

// Query returns nil.
func (m *MockApplication) Query(path string, data []byte) ([]byte, error) {
	return nil, nil
}

// HandleConsensusMessage records a consensus message.
func (m *MockApplication) HandleConsensusMessage(peerID peer.ID, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConsensusMessages = append(m.ConsensusMessages, struct {
		PeerID peer.ID
		Data   []byte
	}{peerID, data})
	return nil
}

// TxCount returns the number of checked transactions.
func (m *MockApplication) TxCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.CheckedTxs)
}

// ConsensusMessageCount returns the number of consensus messages received.
func (m *MockApplication) ConsensusMessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.ConsensusMessages)
}
