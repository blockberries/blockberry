package p2p

import (
	"fmt"
	"sync"

	"github.com/blockberries/glueberry/pkg/streams"
	"github.com/libp2p/go-libp2p/core/peer"
)

// GlueberryStreamAdapter bridges the stream registry with glueberry's stream management.
// It provides dynamic stream registration and message routing capabilities.
type GlueberryStreamAdapter struct {
	registry StreamRegistry
	mu       sync.RWMutex
}

// NewGlueberryStreamAdapter creates a new adapter with the given stream registry.
func NewGlueberryStreamAdapter(registry StreamRegistry) *GlueberryStreamAdapter {
	return &GlueberryStreamAdapter{
		registry: registry,
	}
}

// Registry returns the underlying stream registry.
func (a *GlueberryStreamAdapter) Registry() StreamRegistry {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registry
}

// GetEncryptedStreamNames returns all encrypted stream names from the registry.
// This is used when calling glueberry's PrepareStreams method.
func (a *GlueberryStreamAdapter) GetEncryptedStreamNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	allConfigs := a.registry.All()
	names := make([]string, 0, len(allConfigs))
	for _, cfg := range allConfigs {
		if cfg.Encrypted {
			names = append(names, cfg.Name)
		}
	}
	return names
}

// GetAllStreamNames returns all stream names from the registry.
func (a *GlueberryStreamAdapter) GetAllStreamNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registry.Names()
}

// RouteMessage routes an incoming message to the appropriate handler.
// Returns ErrStreamNotFound if no stream is registered for the stream name.
// Returns ErrStreamHandlerNotSet if the stream has no handler.
func (a *GlueberryStreamAdapter) RouteMessage(msg streams.IncomingMessage) error {
	a.mu.RLock()
	handler := a.registry.GetHandler(msg.StreamName)
	streamExists := a.registry.Has(msg.StreamName)
	a.mu.RUnlock()

	if handler == nil {
		// Check if stream exists but has no handler
		if streamExists {
			return fmt.Errorf("%w: %s", ErrStreamHandlerNotSet, msg.StreamName)
		}
		return fmt.Errorf("%w: %s", ErrStreamNotFound, msg.StreamName)
	}

	return handler(msg.PeerID, msg.Data)
}

// RegisterStream registers a new stream with the registry.
// This is a convenience method that delegates to the registry.
func (a *GlueberryStreamAdapter) RegisterStream(cfg StreamConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.registry.Register(cfg)
}

// UnregisterStream removes a stream from the registry.
// This is a convenience method that delegates to the registry.
func (a *GlueberryStreamAdapter) UnregisterStream(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.registry.Unregister(name)
}

// SetHandler sets the message handler for a stream.
// This is a convenience method that delegates to the registry.
func (a *GlueberryStreamAdapter) SetHandler(name string, handler StreamHandler) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.registry.RegisterHandler(name, handler)
}

// HasStream returns true if the stream is registered.
func (a *GlueberryStreamAdapter) HasStream(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registry.Has(name)
}

// GetStreamConfig returns the configuration for a stream.
func (a *GlueberryStreamAdapter) GetStreamConfig(name string) *StreamConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registry.Get(name)
}

// StreamsByOwner returns all streams owned by the given owner.
func (a *GlueberryStreamAdapter) StreamsByOwner(owner string) []StreamConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registry.ByOwner(owner)
}

// UnregisterByOwner removes all streams owned by the given owner.
// Returns the number of streams unregistered.
func (a *GlueberryStreamAdapter) UnregisterByOwner(owner string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.registry.UnregisterByOwner(owner)
}

// StreamRegistrationCallback is called when a stream is registered or unregistered.
// This can be used to update glueberry's stream list for existing connections.
type StreamRegistrationCallback func(name string, registered bool)

// ObservableStreamAdapter wraps a GlueberryStreamAdapter with registration callbacks.
type ObservableStreamAdapter struct {
	*GlueberryStreamAdapter
	callbacks  []StreamRegistrationCallback
	callbackMu sync.RWMutex
}

// NewObservableStreamAdapter creates a new observable adapter.
func NewObservableStreamAdapter(registry StreamRegistry) *ObservableStreamAdapter {
	return &ObservableStreamAdapter{
		GlueberryStreamAdapter: NewGlueberryStreamAdapter(registry),
		callbacks:              make([]StreamRegistrationCallback, 0),
	}
}

// AddCallback adds a callback that will be invoked on stream registration changes.
func (a *ObservableStreamAdapter) AddCallback(cb StreamRegistrationCallback) {
	a.callbackMu.Lock()
	defer a.callbackMu.Unlock()
	a.callbacks = append(a.callbacks, cb)
}

// RegisterStreamWithCallback registers a stream and notifies callbacks.
func (a *ObservableStreamAdapter) RegisterStreamWithCallback(cfg StreamConfig) error {
	err := a.RegisterStream(cfg)
	if err == nil {
		a.notifyCallbacks(cfg.Name, true)
	}
	return err
}

// UnregisterStreamWithCallback unregisters a stream and notifies callbacks.
func (a *ObservableStreamAdapter) UnregisterStreamWithCallback(name string) error {
	err := a.UnregisterStream(name)
	if err == nil {
		a.notifyCallbacks(name, false)
	}
	return err
}

func (a *ObservableStreamAdapter) notifyCallbacks(name string, registered bool) {
	a.callbackMu.RLock()
	callbacks := make([]StreamRegistrationCallback, len(a.callbacks))
	copy(callbacks, a.callbacks)
	a.callbackMu.RUnlock()

	for _, cb := range callbacks {
		cb(name, registered)
	}
}

// StreamRouter provides message routing with rate limiting and validation.
type StreamRouter struct {
	adapter     *GlueberryStreamAdapter
	rateLimiter *RateLimiter
}

// NewStreamRouter creates a new stream router with optional rate limiting.
func NewStreamRouter(adapter *GlueberryStreamAdapter, rateLimiter *RateLimiter) *StreamRouter {
	return &StreamRouter{
		adapter:     adapter,
		rateLimiter: rateLimiter,
	}
}

// Route routes an incoming message after applying rate limiting.
func (r *StreamRouter) Route(msg streams.IncomingMessage) error {
	// Get stream config for validation
	cfg := r.adapter.GetStreamConfig(msg.StreamName)

	// Validate message size if configured
	if cfg != nil && cfg.MaxMessageSize > 0 && len(msg.Data) > cfg.MaxMessageSize {
		return fmt.Errorf("message too large for stream %s: %d > %d",
			msg.StreamName, len(msg.Data), cfg.MaxMessageSize)
	}

	// Apply rate limiting if configured
	if r.rateLimiter != nil {
		if !r.rateLimiter.Allow(msg.PeerID, msg.StreamName, len(msg.Data)) {
			peerStr := msg.PeerID.String()
			if len(peerStr) > 8 {
				peerStr = peerStr[:8]
			}
			return fmt.Errorf("rate limit exceeded for stream %s from peer %s",
				msg.StreamName, peerStr)
		}
	}

	return r.adapter.RouteMessage(msg)
}

// RouteWithPeer routes a message given peer ID, stream name, and data.
// This is a convenience method for manual message routing.
func (r *StreamRouter) RouteWithPeer(peerID peer.ID, streamName string, data []byte) error {
	msg := streams.IncomingMessage{
		PeerID:     peerID,
		StreamName: streamName,
		Data:       data,
	}
	return r.Route(msg)
}
