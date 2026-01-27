package node

import (
	"errors"
	"fmt"

	"github.com/blockberries/blockberry/blockstore"
	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/consensus"
	"github.com/blockberries/blockberry/mempool"
	bsync "github.com/blockberries/blockberry/sync"
	"github.com/blockberries/blockberry/types"
)

// Role-based builder errors.
var (
	// ErrMissingConsensusEngine is returned when a validator role needs a consensus engine.
	ErrMissingConsensusEngine = errors.New("validator role requires a consensus engine")

	// ErrMissingBlockValidator is returned when block validation is required but not provided.
	ErrMissingBlockValidator = errors.New("role requires block validator")

	// ErrIncompatibleComponent is returned when a component doesn't match the role.
	ErrIncompatibleComponent = errors.New("component incompatible with role")
)

// RoleBasedBuilder creates a Node with components selected based on the node's role.
// It extends NodeBuilder with role-aware component selection.
type RoleBasedBuilder struct {
	cfg  *config.Config
	role types.NodeRole
	caps types.RoleCapabilities

	// Explicitly set components (override auto-selection)
	mempool         mempool.Mempool
	blockStore      blockstore.BlockStore
	consensusEngine consensus.ConsensusEngine
	blockValidator  bsync.BlockValidator
	callbacks       *types.NodeCallbacks

	// Build tracking
	err error
}

// NewRoleBasedBuilder creates a new RoleBasedBuilder with the given configuration.
// The role is taken from the configuration.
func NewRoleBasedBuilder(cfg *config.Config) *RoleBasedBuilder {
	role := cfg.Role
	if role == "" {
		role = types.DefaultRole()
	}

	return &RoleBasedBuilder{
		cfg:  cfg,
		role: role,
		caps: role.Capabilities(),
	}
}

// WithRole sets the node role explicitly.
// This overrides the role from the configuration.
func (b *RoleBasedBuilder) WithRole(role types.NodeRole) *RoleBasedBuilder {
	if b.err != nil {
		return b
	}
	if !role.IsValid() {
		b.err = fmt.Errorf("%w: %q", types.ErrInvalidRole, role)
		return b
	}
	b.role = role
	b.caps = role.Capabilities()
	return b
}

// WithMempool sets a custom mempool implementation.
// If not set, the builder will create one based on role.
func (b *RoleBasedBuilder) WithMempool(mp mempool.Mempool) *RoleBasedBuilder {
	if b.err != nil {
		return b
	}
	b.mempool = mp
	return b
}

// WithBlockStore sets a custom block store implementation.
// If not set, the builder will create one based on role.
func (b *RoleBasedBuilder) WithBlockStore(bs blockstore.BlockStore) *RoleBasedBuilder {
	if b.err != nil {
		return b
	}
	b.blockStore = bs
	return b
}

// WithConsensusEngine sets the consensus engine.
// Required for validator role.
func (b *RoleBasedBuilder) WithConsensusEngine(engine consensus.ConsensusEngine) *RoleBasedBuilder {
	if b.err != nil {
		return b
	}
	b.consensusEngine = engine
	return b
}

// WithBlockValidator sets the block validation function.
// Required for roles that process blocks.
func (b *RoleBasedBuilder) WithBlockValidator(v bsync.BlockValidator) *RoleBasedBuilder {
	if b.err != nil {
		return b
	}
	b.blockValidator = v
	return b
}

// WithCallbacks sets the node event callbacks.
func (b *RoleBasedBuilder) WithCallbacks(cb *types.NodeCallbacks) *RoleBasedBuilder {
	if b.err != nil {
		return b
	}
	b.callbacks = cb
	return b
}

// Build creates the Node with role-appropriate components.
func (b *RoleBasedBuilder) Build() (*Node, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Validate role requirements
	if err := b.validateRoleRequirements(); err != nil {
		return nil, fmt.Errorf("role requirements: %w", err)
	}

	// Create the base builder
	builder := NewNodeBuilder(b.cfg)

	// Set up components based on role
	if err := b.setupMempool(builder); err != nil {
		return nil, fmt.Errorf("setting up mempool: %w", err)
	}

	if err := b.setupBlockStore(builder); err != nil {
		return nil, fmt.Errorf("setting up block store: %w", err)
	}

	if err := b.setupConsensus(builder); err != nil {
		return nil, fmt.Errorf("setting up consensus: %w", err)
	}

	// Set block validator if provided
	if b.blockValidator != nil {
		builder.WithBlockValidator(b.blockValidator)
	}

	// Set callbacks if provided
	if b.callbacks != nil {
		builder.WithCallbacks(b.callbacks)
	}

	// Build the node
	node, err := builder.Build()
	if err != nil {
		return nil, err
	}

	// Store role information in node
	node.role = b.role

	// Configure consensus engine if provided
	if b.consensusEngine != nil {
		node.consensusReactor.SetEngine(b.consensusEngine)
	}

	return node, nil
}

// validateRoleRequirements checks that all required components are available.
func (b *RoleBasedBuilder) validateRoleRequirements() error {
	// Validators require a consensus engine
	if b.caps.RequiresConsensusEngine && b.consensusEngine == nil {
		return ErrMissingConsensusEngine
	}

	// Roles that store blocks need a validator
	if b.caps.StoresFullBlocks && b.blockValidator == nil {
		return ErrMissingBlockValidator
	}

	return nil
}

// setupMempool configures the mempool based on role.
func (b *RoleBasedBuilder) setupMempool(builder *NodeBuilder) error {
	if b.mempool != nil {
		// Use explicitly provided mempool
		builder.WithMempool(b.mempool)
		return nil
	}

	if !b.caps.RequiresMempool {
		// Role doesn't need a mempool, create a minimal no-op one
		builder.WithMempool(mempool.NewNoOpMempool())
		return nil
	}

	// Let the base builder create the default mempool
	return nil
}

// setupBlockStore configures the block store based on role.
func (b *RoleBasedBuilder) setupBlockStore(builder *NodeBuilder) error {
	if b.blockStore != nil {
		// Use explicitly provided block store
		builder.WithBlockStore(b.blockStore)
		return nil
	}

	if !b.caps.RequiresBlockStore {
		// Role doesn't need a block store, create a no-op one
		builder.WithBlockStore(blockstore.NewNoOpBlockStore())
		return nil
	}

	// For light nodes that need headers only
	if !b.caps.StoresFullBlocks && b.caps.RequiresBlockStore {
		builder.WithBlockStore(blockstore.NewHeaderOnlyBlockStore())
		return nil
	}

	// Let the base builder create the default block store
	return nil
}

// setupConsensus configures consensus handling based on role.
// Note: The consensus engine is configured on the node after build,
// as it needs access to network and peer manager created during build.
func (b *RoleBasedBuilder) setupConsensus(_ *NodeBuilder) error {
	// Consensus engine will be set after build via node.SetConsensusEngine()
	// This allows the engine to have access to network components
	return nil
}

// Role returns the role this builder is configured for.
func (b *RoleBasedBuilder) Role() types.NodeRole {
	return b.role
}

// Capabilities returns the capabilities for the configured role.
func (b *RoleBasedBuilder) Capabilities() types.RoleCapabilities {
	return b.caps
}

// MustBuild is like Build but panics if an error occurs.
func (b *RoleBasedBuilder) MustBuild() *Node {
	n, err := b.Build()
	if err != nil {
		panic(err)
	}
	return n
}

// NewRoleBasedNode creates a new node using role-based component selection.
// This is the recommended way to create a Node when role-specific behavior is needed.
//
// Example:
//
//	node, err := NewRoleBasedNode(cfg).
//		WithConsensusEngine(myEngine).
//		WithBlockValidator(myValidator).
//		Build()
func NewRoleBasedNode(cfg *config.Config) *RoleBasedBuilder {
	return NewRoleBasedBuilder(cfg)
}

// IsValidatorRole returns true if the role can participate in consensus.
func IsValidatorRole(role types.NodeRole) bool {
	caps := role.Capabilities()
	return caps.CanVote && caps.CanPropose
}

// RoleRequiresFullSync returns true if the role requires full block sync.
func RoleRequiresFullSync(role types.NodeRole) bool {
	caps := role.Capabilities()
	return caps.StoresFullBlocks
}

// RoleAcceptsInbound returns true if the role accepts inbound connections.
func RoleAcceptsInbound(role types.NodeRole) bool {
	caps := role.Capabilities()
	return caps.AcceptsInboundConnections
}
