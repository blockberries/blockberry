package p2p

import (
	"errors"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Stream registry errors.
var (
	ErrStreamAlreadyRegistered = errors.New("stream already registered")
	ErrStreamNotFound          = errors.New("stream not found")
	ErrStreamHandlerNotSet     = errors.New("stream handler not set")
	ErrInvalidStreamConfig     = errors.New("invalid stream configuration")
	ErrStreamInUse             = errors.New("stream is in use and cannot be unregistered")
)

// StreamConfig defines a P2P stream configuration.
// This is the canonical definition used throughout blockberry for dynamic stream registration.
type StreamConfig struct {
	// Name is the unique stream identifier (e.g., "consensus", "looseberry-batches").
	Name string

	// Encrypted indicates if the stream uses encryption after handshake.
	// All streams except "handshake" should typically be encrypted.
	Encrypted bool

	// MessageTypes lists the cramberry message type IDs that can be sent on this stream.
	// Used for documentation and validation. Empty means all types are allowed.
	MessageTypes []uint16

	// RateLimit is the maximum messages per second (0 = unlimited).
	RateLimit int

	// MaxMessageSize is the maximum message size in bytes (0 = default 10MB).
	MaxMessageSize int

	// Owner identifies which component owns this stream (e.g., "mempool", "consensus").
	// Used for tracking and cleanup when components are unregistered.
	Owner string
}

// Validate checks if the stream configuration is valid.
func (c *StreamConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: name cannot be empty", ErrInvalidStreamConfig)
	}
	if c.RateLimit < 0 {
		return fmt.Errorf("%w: rate limit cannot be negative", ErrInvalidStreamConfig)
	}
	if c.MaxMessageSize < 0 {
		return fmt.Errorf("%w: max message size cannot be negative", ErrInvalidStreamConfig)
	}
	return nil
}

// Clone creates a deep copy of the stream configuration.
func (c *StreamConfig) Clone() *StreamConfig {
	clone := &StreamConfig{
		Name:           c.Name,
		Encrypted:      c.Encrypted,
		RateLimit:      c.RateLimit,
		MaxMessageSize: c.MaxMessageSize,
		Owner:          c.Owner,
	}
	if len(c.MessageTypes) > 0 {
		clone.MessageTypes = make([]uint16, len(c.MessageTypes))
		copy(clone.MessageTypes, c.MessageTypes)
	}
	return clone
}

// StreamHandler processes messages received on a stream.
// It is called for each message received from a peer on the associated stream.
type StreamHandler func(peerID peer.ID, data []byte) error

// StreamRegistry manages dynamic stream registration.
// It provides a centralized place for components to register and manage P2P streams.
type StreamRegistry interface {
	// Register adds a new stream configuration.
	// Returns ErrStreamAlreadyRegistered if the stream name is already registered.
	Register(cfg StreamConfig) error

	// Unregister removes a stream by name.
	// Returns ErrStreamNotFound if the stream is not registered.
	// Returns ErrStreamInUse if the stream has an active handler.
	Unregister(name string) error

	// Get returns a stream configuration by name.
	// Returns nil if the stream is not found.
	Get(name string) *StreamConfig

	// All returns all registered stream configurations.
	// The returned slice is a copy and can be safely modified.
	All() []StreamConfig

	// Names returns all registered stream names.
	Names() []string

	// Has returns true if a stream with the given name is registered.
	Has(name string) bool

	// RegisterHandler sets the message handler for a stream.
	// Returns ErrStreamNotFound if the stream is not registered.
	// Setting handler to nil removes the handler.
	RegisterHandler(name string, handler StreamHandler) error

	// GetHandler returns the handler for a stream.
	// Returns nil if no handler is registered.
	GetHandler(name string) StreamHandler

	// ByOwner returns all stream configurations owned by the given owner.
	ByOwner(owner string) []StreamConfig

	// UnregisterByOwner removes all streams owned by the given owner.
	// Returns the number of streams unregistered.
	UnregisterByOwner(owner string) int
}

// streamEntry holds a stream configuration and its handler.
type streamEntry struct {
	config  StreamConfig
	handler StreamHandler
}

// InMemoryStreamRegistry is an in-memory implementation of StreamRegistry.
// It is thread-safe and suitable for production use.
type InMemoryStreamRegistry struct {
	streams map[string]*streamEntry
	mu      sync.RWMutex
}

// NewStreamRegistry creates a new in-memory stream registry.
func NewStreamRegistry() *InMemoryStreamRegistry {
	return &InMemoryStreamRegistry{
		streams: make(map[string]*streamEntry),
	}
}

// Register implements StreamRegistry.Register.
func (r *InMemoryStreamRegistry) Register(cfg StreamConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[cfg.Name]; exists {
		return fmt.Errorf("%w: %s", ErrStreamAlreadyRegistered, cfg.Name)
	}

	r.streams[cfg.Name] = &streamEntry{
		config: *cfg.Clone(),
	}
	return nil
}

// Unregister implements StreamRegistry.Unregister.
func (r *InMemoryStreamRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.streams[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrStreamNotFound, name)
	}

	// Don't allow unregistering streams with active handlers
	if entry.handler != nil {
		return fmt.Errorf("%w: %s has an active handler", ErrStreamInUse, name)
	}

	delete(r.streams, name)
	return nil
}

// Get implements StreamRegistry.Get.
func (r *InMemoryStreamRegistry) Get(name string) *StreamConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, exists := r.streams[name]; exists {
		return entry.config.Clone()
	}
	return nil
}

// All implements StreamRegistry.All.
func (r *InMemoryStreamRegistry) All() []StreamConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]StreamConfig, 0, len(r.streams))
	for _, entry := range r.streams {
		configs = append(configs, *entry.config.Clone())
	}
	return configs
}

// Names implements StreamRegistry.Names.
func (r *InMemoryStreamRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.streams))
	for name := range r.streams {
		names = append(names, name)
	}
	return names
}

// Has implements StreamRegistry.Has.
func (r *InMemoryStreamRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.streams[name]
	return exists
}

// RegisterHandler implements StreamRegistry.RegisterHandler.
func (r *InMemoryStreamRegistry) RegisterHandler(name string, handler StreamHandler) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.streams[name]
	if !exists {
		return fmt.Errorf("%w: %s", ErrStreamNotFound, name)
	}

	entry.handler = handler
	return nil
}

// GetHandler implements StreamRegistry.GetHandler.
func (r *InMemoryStreamRegistry) GetHandler(name string) StreamHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if entry, exists := r.streams[name]; exists {
		return entry.handler
	}
	return nil
}

// ByOwner implements StreamRegistry.ByOwner.
func (r *InMemoryStreamRegistry) ByOwner(owner string) []StreamConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]StreamConfig, 0)
	for _, entry := range r.streams {
		if entry.config.Owner == owner {
			configs = append(configs, *entry.config.Clone())
		}
	}
	return configs
}

// UnregisterByOwner implements StreamRegistry.UnregisterByOwner.
func (r *InMemoryStreamRegistry) UnregisterByOwner(owner string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	var count int
	for name, entry := range r.streams {
		if entry.config.Owner == owner && entry.handler == nil {
			delete(r.streams, name)
			count++
		}
	}
	return count
}

// ForceUnregister removes a stream regardless of whether it has an active handler.
// This should only be used during shutdown or error recovery.
func (r *InMemoryStreamRegistry) ForceUnregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[name]; exists {
		delete(r.streams, name)
		return true
	}
	return false
}

// Count returns the number of registered streams.
func (r *InMemoryStreamRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.streams)
}

// Clear removes all registered streams.
// This should only be used during testing or shutdown.
func (r *InMemoryStreamRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams = make(map[string]*streamEntry)
}

// Ensure InMemoryStreamRegistry implements StreamRegistry.
var _ StreamRegistry = (*InMemoryStreamRegistry)(nil)

// RegisterBuiltinStreams registers the core blockberry streams.
// This should be called during node initialization to set up default streams.
func RegisterBuiltinStreams(registry StreamRegistry) error {
	builtinStreams := []StreamConfig{
		{Name: StreamPEX, Encrypted: true, Owner: "pex", RateLimit: 100, MaxMessageSize: 1024 * 1024},
		{Name: StreamTransactions, Encrypted: true, Owner: "transactions", RateLimit: 1000, MaxMessageSize: 10 * 1024 * 1024},
		{Name: StreamBlockSync, Encrypted: true, Owner: "sync", RateLimit: 100, MaxMessageSize: 50 * 1024 * 1024},
		{Name: StreamBlocks, Encrypted: true, Owner: "blocks", RateLimit: 100, MaxMessageSize: 10 * 1024 * 1024},
		{Name: StreamConsensus, Encrypted: true, Owner: "consensus", RateLimit: 1000, MaxMessageSize: 10 * 1024 * 1024},
		{Name: StreamHousekeeping, Encrypted: true, Owner: "housekeeping", RateLimit: 10, MaxMessageSize: 1024},
	}

	for _, cfg := range builtinStreams {
		if err := registry.Register(cfg); err != nil {
			return fmt.Errorf("registering built-in stream %s: %w", cfg.Name, err)
		}
	}

	return nil
}
