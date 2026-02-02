package consensus

import (
	"fmt"
	"sync"
)

// ConsensusType identifies a consensus engine implementation.
type ConsensusType string

// Built-in consensus types.
const (
	// TypeNone is a no-op consensus engine for full nodes that don't participate
	// in consensus. They receive and validate blocks but don't propose or vote.
	TypeNone ConsensusType = "none"
)

// ConsensusConstructor creates a consensus engine from configuration.
// The constructor receives the consensus config and should create an
// appropriate engine instance.
type ConsensusConstructor func(cfg *ConsensusConfig) (ConsensusEngine, error)

// Factory creates consensus engine instances from configuration.
// It maintains a registry of consensus constructors that can be extended
// with custom implementations.
type Factory struct {
	registry map[ConsensusType]ConsensusConstructor
	mu       sync.RWMutex
}

// NewFactory creates a new consensus factory with built-in implementations registered.
func NewFactory() *Factory {
	f := &Factory{
		registry: make(map[ConsensusType]ConsensusConstructor),
	}

	// Register built-in engines
	f.Register(TypeNone, NewNullConsensusFromConfig)

	return f
}

// Register adds a consensus constructor to the factory.
// If a constructor with the same type already exists, it is replaced.
// This allows custom implementations to override built-in ones.
func (f *Factory) Register(consensusType ConsensusType, constructor ConsensusConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registry[consensusType] = constructor
}

// Unregister removes a consensus constructor from the factory.
func (f *Factory) Unregister(consensusType ConsensusType) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.registry, consensusType)
}

// Create creates a consensus engine using the registered constructor for the given type.
// Returns an error if the type is not registered.
func (f *Factory) Create(cfg *ConsensusConfig) (ConsensusEngine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("consensus config is required")
	}

	f.mu.RLock()
	constructor, ok := f.registry[ConsensusType(cfg.Type)]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown consensus type: %s", cfg.Type)
	}

	return constructor(cfg)
}

// Has returns true if a constructor is registered for the given type.
func (f *Factory) Has(consensusType ConsensusType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.registry[consensusType]
	return ok
}

// Types returns all registered consensus types.
func (f *Factory) Types() []ConsensusType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]ConsensusType, 0, len(f.registry))
	for t := range f.registry {
		types = append(types, t)
	}
	return types
}

// DefaultFactory is the global factory instance with built-in consensus engines.
// Applications can register custom consensus engines with this factory.
var DefaultFactory = NewFactory()

// CreateFromConfig creates a consensus engine using the default factory.
// This is a convenience function for simple use cases.
func CreateFromConfig(cfg *ConsensusConfig) (ConsensusEngine, error) {
	return DefaultFactory.Create(cfg)
}

// RegisterConsensus registers a custom consensus constructor with the default factory.
// This allows applications to add custom consensus implementations.
func RegisterConsensus(consensusType ConsensusType, constructor ConsensusConstructor) {
	DefaultFactory.Register(consensusType, constructor)
}
