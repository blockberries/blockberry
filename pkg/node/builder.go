package node

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"

	"github.com/blockberries/glueberry"

	"github.com/blockberries/blockberry/pkg/blockstore"
	"github.com/blockberries/blockberry/pkg/config"
	"github.com/blockberries/blockberry/pkg/consensus"
	"github.com/blockberries/blockberry/internal/handlers"
	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/blockberry/internal/p2p"
	"github.com/blockberries/blockberry/internal/pex"
	bsync "github.com/blockberries/blockberry/internal/sync"
	"github.com/blockberries/blockberry/pkg/types"
)


// NodeBuilder provides a fluent interface for constructing a Node.
// It ensures single-pass initialization and proper validation.
type NodeBuilder struct {
	cfg *config.Config

	// Pluggable components (optional)
	privateKey       ed25519.PrivateKey
	mempool          mempool.Mempool
	blockStore       blockstore.BlockStore
	consensusHandler consensus.ConsensusHandler
	blockValidator   types.BlockValidator
	callbacks        *types.NodeCallbacks

	// Error tracking during build
	err error
}

// NewNodeBuilder creates a new NodeBuilder with the given configuration.
func NewNodeBuilder(cfg *config.Config) *NodeBuilder {
	return &NodeBuilder{cfg: cfg}
}

// WithPrivateKey sets a pre-loaded private key, skipping file-based key loading.
// Use this when the calling application manages its own key format.
func (b *NodeBuilder) WithPrivateKey(key ed25519.PrivateKey) *NodeBuilder {
	if b.err != nil {
		return b
	}
	b.privateKey = key
	return b
}

// WithMempool sets a custom mempool implementation.
func (b *NodeBuilder) WithMempool(mp mempool.Mempool) *NodeBuilder {
	if b.err != nil {
		return b
	}
	b.mempool = mp
	return b
}

// WithBlockStore sets a custom block store implementation.
func (b *NodeBuilder) WithBlockStore(bs blockstore.BlockStore) *NodeBuilder {
	if b.err != nil {
		return b
	}
	b.blockStore = bs
	return b
}

// WithConsensusHandler sets the consensus message handler.
func (b *NodeBuilder) WithConsensusHandler(ch consensus.ConsensusHandler) *NodeBuilder {
	if b.err != nil {
		return b
	}
	b.consensusHandler = ch
	return b
}

// WithBlockValidator sets the block validation function.
// This is REQUIRED for the node to process blocks properly.
func (b *NodeBuilder) WithBlockValidator(v types.BlockValidator) *NodeBuilder {
	if b.err != nil {
		return b
	}
	b.blockValidator = v
	return b
}

// WithCallbacks sets the node event callbacks.
func (b *NodeBuilder) WithCallbacks(cb *types.NodeCallbacks) *NodeBuilder {
	if b.err != nil {
		return b
	}
	b.callbacks = cb
	return b
}

// Build creates the Node with all configured components.
// Returns an error if the configuration is invalid or component creation fails.
func (b *NodeBuilder) Build() (*Node, error) {
	if b.err != nil {
		return nil, b.err
	}

	cfg := b.cfg

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Use pre-loaded key or load from file
	privateKey := b.privateKey
	if privateKey == nil {
		var err error
		privateKey, err = loadOrGenerateKey(cfg.Node.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("loading private key: %w", err)
		}
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

	// Create or use provided block store
	bs := b.blockStore
	if bs == nil {
		bs, err = blockstore.NewLevelDBBlockStore(cfg.BlockStore.Path)
		if err != nil {
			return nil, fmt.Errorf("creating block store: %w", err)
		}
	}

	// Create or use provided mempool
	mp := b.mempool
	if mp == nil {
		mp = mempool.NewMempool(cfg.Mempool)
	}

	// Create address book for PEX
	addressBook := pex.NewAddressBook(cfg.Network.AddressBookPath)
	addressBook.AddSeeds(cfg.Network.Seeds.Addrs)

	peerManager := network.PeerManager()

	// Create handlers using configuration values
	handshakeHandler := handlers.NewHandshakeHandler(
		cfg.Node.ChainID,
		cfg.Node.ProtocolVersion,
		nodeID,
		publicKey,
		network,
		peerManager,
		func() int64 { return bs.Height() },
	)

	transactionsReactor := handlers.NewTransactionsReactor(
		mp,
		network,
		peerManager,
		cfg.Handlers.Transactions.RequestInterval.Duration(),
		cfg.Handlers.Transactions.BatchSize,
	)

	blocksReactor := handlers.NewBlockReactor(
		bs,
		network,
		peerManager,
	)

	consensusReactor := handlers.NewConsensusReactor(
		network,
		peerManager,
	)

	// Set consensus handler if provided
	if b.consensusHandler != nil {
		consensusReactor.SetHandler(b.consensusHandler)
	}

	housekeepingReactor := handlers.NewHousekeepingReactor(
		network,
		peerManager,
		cfg.Housekeeping.LatencyProbeInterval.Duration(),
	)

	pexReactor := pex.NewReactor(
		cfg.PEX.Enabled,
		cfg.PEX.RequestInterval.Duration(),
		cfg.PEX.MaxAddressesPerResponse,
		addressBook,
		network,
		peerManager,
		cfg.Network.MaxInboundPeers,
		cfg.Network.MaxOutboundPeers,
	)

	syncReactor := bsync.NewSyncReactor(
		bs,
		network,
		peerManager,
		cfg.Handlers.Sync.SyncInterval.Duration(),
		cfg.Handlers.Sync.BatchSize,
	)

	// Set block validator if provided
	if b.blockValidator != nil {
		syncReactor.SetValidator(b.blockValidator)
	}

	// Create node with all components initialized
	n := &Node{
		cfg:                 cfg,
		privateKey:          privateKey,
		nodeID:              nodeID,
		glueNode:            glueNode,
		network:             network,
		blockStore:          bs,
		mempool:             mp,
		handshakeHandler:    handshakeHandler,
		transactionsReactor: transactionsReactor,
		blocksReactor:       blocksReactor,
		consensusReactor:    consensusReactor,
		housekeepingReactor: housekeepingReactor,
		pexReactor:          pexReactor,
		syncReactor:         syncReactor,
		callbacks:           b.callbacks,
		stopCh:              make(chan struct{}),
	}

	return n, nil
}

// MustBuild is like Build but panics if an error occurs.
// Use only when you're certain the configuration is valid.
func (b *NodeBuilder) MustBuild() *Node {
	n, err := b.Build()
	if err != nil {
		panic(err)
	}
	return n
}

// NewNodeWithBuilder creates a new node using the builder pattern.
// This is the recommended way to create a Node.
//
// Example:
//
//	node, err := NewNodeWithBuilder(cfg).
//		WithBlockValidator(myValidator).
//		WithCallbacks(myCallbacks).
//		Build()
func NewNodeWithBuilder(cfg *config.Config) *NodeBuilder {
	return NewNodeBuilder(cfg)
}
