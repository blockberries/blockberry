package grpc

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/blockberries/blockberry/abi"
	"github.com/blockberries/blockberry/security"
)

// RateLimitConfig contains rate limiting configuration.
type RateLimitConfig struct {
	// Enabled controls whether rate limiting is active.
	Enabled bool

	// GlobalRate is the overall requests per interval for all clients.
	GlobalRate int

	// PerClientRate is the requests per interval per client IP.
	PerClientRate int

	// Interval is the time window for rate calculations.
	Interval time.Duration

	// Burst is the maximum burst size allowed.
	Burst int

	// CleanupInterval is how often to clean up stale entries.
	CleanupInterval time.Duration

	// ExemptMethods are methods that bypass rate limiting.
	// Format: "/service.Name/MethodName"
	ExemptMethods []string

	// ExemptClients are client IPs that bypass rate limiting.
	ExemptClients []string
}

// DefaultRateLimitConfig returns sensible rate limiting defaults.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Enabled:         false,
		GlobalRate:      1000,            // 1000 requests per second globally
		PerClientRate:   100,             // 100 requests per second per client
		Interval:        time.Second,
		Burst:           50,              // Allow bursts up to 50 requests
		CleanupInterval: 5 * time.Minute,
		ExemptMethods: []string{
			"/blockberry.Node/Health",
		},
	}
}

// RateLimiter handles rate limiting for gRPC requests.
// All public methods are safe for concurrent use.
type RateLimiter struct {
	config         RateLimitConfig
	globalLimiter  abi.RateLimiter
	clientLimiter  abi.RateLimiter
	exemptMethods  map[string]struct{}
	exemptClients  map[string]struct{}
	mu             sync.RWMutex // Protects exemptMethods and exemptClients
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config:        config,
		exemptMethods: make(map[string]struct{}),
		exemptClients: make(map[string]struct{}),
	}

	for _, method := range config.ExemptMethods {
		rl.exemptMethods[method] = struct{}{}
	}

	for _, client := range config.ExemptClients {
		rl.exemptClients[client] = struct{}{}
	}

	// Create global limiter
	rl.globalLimiter = security.NewTokenBucketLimiter(abi.RateLimiterConfig{
		Rate:            config.GlobalRate,
		Interval:        config.Interval,
		Burst:           config.Burst * 10, // Higher burst for global
		CleanupInterval: config.CleanupInterval,
	})

	// Create per-client limiter
	rl.clientLimiter = security.NewTokenBucketLimiter(abi.RateLimiterConfig{
		Rate:            config.PerClientRate,
		Interval:        config.Interval,
		Burst:           config.Burst,
		CleanupInterval: config.CleanupInterval,
	})

	return rl
}

// UnaryInterceptor returns a unary server interceptor for rate limiting.
func (rl *RateLimiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !rl.config.Enabled {
			return handler(ctx, req)
		}

		if err := rl.checkLimit(ctx, info.FullMethod); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// StreamInterceptor returns a stream server interceptor for rate limiting.
func (rl *RateLimiter) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !rl.config.Enabled {
			return handler(srv, ss)
		}

		// Rate limit the initial stream setup
		if err := rl.checkLimit(ss.Context(), info.FullMethod); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

// checkLimit checks if the request should be rate limited.
func (rl *RateLimiter) checkLimit(ctx context.Context, method string) error {
	clientIP := rl.getClientIP(ctx)

	// Check if method or client is exempt (read lock for map access)
	rl.mu.RLock()
	_, methodExempt := rl.exemptMethods[method]
	_, clientExempt := rl.exemptClients[clientIP]
	rl.mu.RUnlock()

	if methodExempt || clientExempt {
		return nil
	}

	// Check global rate limit
	if !rl.globalLimiter.Allow("global") {
		return status.Error(codes.ResourceExhausted, "global rate limit exceeded")
	}

	// Check per-client rate limit
	if !rl.clientLimiter.Allow(clientIP) {
		return status.Error(codes.ResourceExhausted, "rate limit exceeded")
	}

	return nil
}

// getClientIP extracts the client IP from context.
func (rl *RateLimiter) getClientIP(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}
	return "unknown"
}

// Close releases resources.
func (rl *RateLimiter) Close() {
	if closer, ok := rl.globalLimiter.(interface{ Close() }); ok {
		closer.Close()
	}
	if closer, ok := rl.clientLimiter.(interface{ Close() }); ok {
		closer.Close()
	}
}

// AddExemptClient adds a client IP to the exempt list.
func (rl *RateLimiter) AddExemptClient(clientIP string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.exemptClients[clientIP] = struct{}{}
}

// RemoveExemptClient removes a client IP from the exempt list.
func (rl *RateLimiter) RemoveExemptClient(clientIP string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.exemptClients, clientIP)
}

// AddExemptMethod adds a method to the exempt list.
func (rl *RateLimiter) AddExemptMethod(method string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.exemptMethods[method] = struct{}{}
}

// RemoveExemptMethod removes a method from the exempt list.
func (rl *RateLimiter) RemoveExemptMethod(method string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.exemptMethods, method)
}

// Reset resets the rate limit for a specific client.
func (rl *RateLimiter) Reset(clientIP string) {
	rl.clientLimiter.Reset(clientIP)
}

// ResetGlobal resets the global rate limit.
func (rl *RateLimiter) ResetGlobal() {
	rl.globalLimiter.Reset("global")
}
