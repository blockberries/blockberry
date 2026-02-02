package mempool

import (
	"fmt"
	"sync"

	"github.com/blockberries/blockberry/pkg/config"
)

// MempoolType identifies a mempool implementation.
type MempoolType string

// Built-in mempool types.
const (
	// TypeSimple is the basic hash-based mempool with FIFO ordering.
	TypeSimple MempoolType = "simple"

	// TypePriority is a priority-based mempool where higher priority txs are reaped first.
	TypePriority MempoolType = "priority"

	// TypeTTL is a mempool with time-to-live expiration for transactions.
	TypeTTL MempoolType = "ttl"

	// TypeLooseberry is the DAG-based mempool for BFT consensus.
	TypeLooseberry MempoolType = "looseberry"
)

// MempoolConstructor creates a mempool from configuration.
// The constructor receives the full mempool config and should extract
// the relevant settings for the specific implementation.
type MempoolConstructor func(cfg *config.MempoolConfig) (Mempool, error)

// Factory creates mempool instances from configuration.
// It maintains a registry of mempool constructors that can be extended
// with custom implementations.
type Factory struct {
	registry map[MempoolType]MempoolConstructor
	mu       sync.RWMutex
}

// NewFactory creates a new mempool factory with built-in implementations registered.
func NewFactory() *Factory {
	f := &Factory{
		registry: make(map[MempoolType]MempoolConstructor),
	}

	// Register built-in mempools
	f.Register(TypeSimple, NewSimpleMempoolFromConfig)
	f.Register(TypePriority, NewPriorityMempoolFromConfig)
	f.Register(TypeTTL, NewTTLMempoolFromConfig)

	return f
}

// Register adds a mempool constructor to the factory.
// If a constructor with the same type already exists, it is replaced.
// This allows custom implementations to override built-in ones.
func (f *Factory) Register(mempoolType MempoolType, constructor MempoolConstructor) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registry[mempoolType] = constructor
}

// Unregister removes a mempool constructor from the factory.
func (f *Factory) Unregister(mempoolType MempoolType) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.registry, mempoolType)
}

// Create creates a mempool using the registered constructor for the given type.
// Returns an error if the type is not registered.
func (f *Factory) Create(cfg *config.MempoolConfig) (Mempool, error) {
	f.mu.RLock()
	constructor, ok := f.registry[MempoolType(cfg.Type)]
	f.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown mempool type: %s", cfg.Type)
	}

	return constructor(cfg)
}

// Has returns true if a constructor is registered for the given type.
func (f *Factory) Has(mempoolType MempoolType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.registry[mempoolType]
	return ok
}

// Types returns all registered mempool types.
func (f *Factory) Types() []MempoolType {
	f.mu.RLock()
	defer f.mu.RUnlock()

	types := make([]MempoolType, 0, len(f.registry))
	for t := range f.registry {
		types = append(types, t)
	}
	return types
}

// NewSimpleMempoolFromConfig creates a SimpleMempool from configuration.
func NewSimpleMempoolFromConfig(cfg *config.MempoolConfig) (Mempool, error) {
	return NewSimpleMempool(cfg.MaxTxs, cfg.MaxBytes), nil
}

// NewPriorityMempoolFromConfig creates a PriorityMempool from configuration.
func NewPriorityMempoolFromConfig(cfg *config.MempoolConfig) (Mempool, error) {
	pcfg := PriorityMempoolConfig{
		MaxTxs:   cfg.MaxTxs,
		MaxBytes: cfg.MaxBytes,
	}

	// Use default priority function
	mp := NewPriorityMempool(pcfg)
	return mp, nil
}

// NewTTLMempoolFromConfig creates a TTLMempool from configuration.
func NewTTLMempoolFromConfig(cfg *config.MempoolConfig) (Mempool, error) {
	tcfg := TTLMempoolConfig{
		MaxTxs:          cfg.MaxTxs,
		MaxBytes:        cfg.MaxBytes,
		TTL:             cfg.TTL.Duration(),
		CleanupInterval: cfg.CleanupInterval.Duration(),
	}

	mp := NewTTLMempool(tcfg)
	return mp, nil
}

// DefaultFactory is the global factory instance with built-in mempools.
// Applications can register custom mempools with this factory.
var DefaultFactory = NewFactory()

// CreateFromConfig creates a mempool using the default factory.
// This is a convenience function for simple use cases.
func CreateFromConfig(cfg *config.MempoolConfig) (Mempool, error) {
	return DefaultFactory.Create(cfg)
}

// RegisterMempool registers a custom mempool constructor with the default factory.
// This allows applications to add custom mempool implementations.
func RegisterMempool(mempoolType MempoolType, constructor MempoolConstructor) {
	DefaultFactory.Register(mempoolType, constructor)
}
