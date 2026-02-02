package consensus

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// NullConsensus is a no-op consensus engine for full nodes.
// It receives and stores blocks but does not participate in consensus.
// This is useful for nodes that only need to follow the chain without
// validating or proposing blocks.
type NullConsensus struct {
	config  *ConsensusConfig
	deps    ConsensusDependencies
	height  atomic.Int64
	round   atomic.Int32
	running atomic.Bool
	mu      sync.RWMutex
}

// NewNullConsensus creates a new null consensus engine.
func NewNullConsensus() *NullConsensus {
	return &NullConsensus{}
}

// NewNullConsensusFromConfig creates a NullConsensus from configuration.
// This is the factory constructor.
func NewNullConsensusFromConfig(cfg *ConsensusConfig) (ConsensusEngine, error) {
	nc := NewNullConsensus()
	nc.config = cfg
	return nc, nil
}

// Name returns the component name.
func (nc *NullConsensus) Name() string {
	return "null-consensus"
}

// IsRunning returns true if the engine is running.
func (nc *NullConsensus) IsRunning() bool {
	return nc.running.Load()
}

// Start starts the consensus engine.
func (nc *NullConsensus) Start() error {
	if nc.running.Swap(true) {
		return fmt.Errorf("null consensus already running")
	}
	return nil
}

// Stop stops the consensus engine.
func (nc *NullConsensus) Stop() error {
	nc.running.Store(false)
	return nil
}

// Initialize sets up the consensus engine with its dependencies.
func (nc *NullConsensus) Initialize(deps ConsensusDependencies) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()
	nc.deps = deps
	if deps.Config != nil {
		nc.config = deps.Config
	}
	return nil
}

// ProcessBlock processes a block received from the network.
// For NullConsensus, this simply updates the height and notifies callbacks.
func (nc *NullConsensus) ProcessBlock(block *Block) error {
	if block == nil {
		return fmt.Errorf("nil block")
	}

	// Update height
	nc.height.Store(block.Height)
	nc.round.Store(block.Round)

	// Notify callback if set
	nc.mu.RLock()
	callbacks := nc.deps.Callbacks
	nc.mu.RUnlock()

	if err := callbacks.InvokeOnBlockCommitted(block.Height, block.Hash); err != nil {
		return err
	}

	return nil
}

// GetHeight returns the current consensus height.
func (nc *NullConsensus) GetHeight() int64 {
	return nc.height.Load()
}

// GetRound returns the current consensus round.
func (nc *NullConsensus) GetRound() int32 {
	return nc.round.Load()
}

// IsValidator returns false since NullConsensus is for full nodes only.
func (nc *NullConsensus) IsValidator() bool {
	return false
}

// ValidatorSet returns nil since NullConsensus doesn't track validators.
func (nc *NullConsensus) ValidatorSet() ValidatorSet {
	return nil
}

// Verify NullConsensus implements ConsensusEngine.
var _ ConsensusEngine = (*NullConsensus)(nil)
