package abi

import (
	"time"
)

// ResourceLimits defines configurable limits for resource usage.
// These limits are enforced throughout the system to prevent resource exhaustion.
type ResourceLimits struct {
	// Transaction limits
	MaxTxSize   int64 // Maximum transaction size in bytes
	MaxTxsBytes int64 // Maximum total bytes of transactions in mempool

	// Block limits
	MaxBlockSize    int64 // Maximum block size in bytes
	MaxBlockTxs     int   // Maximum number of transactions per block
	MaxEvidenceSize int64 // Maximum evidence size in bytes

	// Message limits
	MaxMsgSize int64 // Maximum message size for network messages

	// Connection limits
	MaxPeers          int // Maximum number of peer connections
	MaxInboundPeers   int // Maximum inbound peer connections
	MaxOutboundPeers  int // Maximum outbound peer connections
	MaxPendingPeers   int // Maximum pending peer connections

	// Subscription limits
	MaxSubscribers         int // Maximum total subscriptions
	MaxSubscribersPerQuery int // Maximum subscriptions per query

	// Rate limits
	MaxTxsPerSecond   int // Maximum transactions accepted per second
	MaxMsgsPerSecond  int // Maximum messages per peer per second
	MaxBytesPerSecond int // Maximum bytes per peer per second
}

// DefaultResourceLimits returns sensible defaults for resource limits.
func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		// Transaction limits
		MaxTxSize:   1024 * 1024,      // 1 MB
		MaxTxsBytes: 64 * 1024 * 1024, // 64 MB

		// Block limits
		MaxBlockSize:    21 * 1024 * 1024, // 21 MB
		MaxBlockTxs:     10000,
		MaxEvidenceSize: 1024 * 1024, // 1 MB

		// Message limits
		MaxMsgSize: 10 * 1024 * 1024, // 10 MB

		// Connection limits
		MaxPeers:         50,
		MaxInboundPeers:  40,
		MaxOutboundPeers: 10,
		MaxPendingPeers:  10,

		// Subscription limits
		MaxSubscribers:         1000,
		MaxSubscribersPerQuery: 100,

		// Rate limits
		MaxTxsPerSecond:   1000,
		MaxMsgsPerSecond:  100,
		MaxBytesPerSecond: 10 * 1024 * 1024, // 10 MB/s
	}
}

// Validate checks that the limits are valid.
func (l *ResourceLimits) Validate() error {
	if l.MaxTxSize <= 0 {
		return ErrInvalidLimit
	}
	if l.MaxBlockSize <= 0 {
		return ErrInvalidLimit
	}
	if l.MaxMsgSize <= 0 {
		return ErrInvalidLimit
	}
	if l.MaxPeers < 0 {
		return ErrInvalidLimit
	}
	if l.MaxInboundPeers < 0 || l.MaxOutboundPeers < 0 {
		return ErrInvalidLimit
	}
	if l.MaxInboundPeers+l.MaxOutboundPeers > l.MaxPeers {
		return ErrInvalidLimit
	}
	return nil
}

// RateLimiter tracks request rates and enforces limits.
type RateLimiter interface {
	// Allow checks if a request should be allowed.
	// Returns true if the request is within rate limits.
	Allow(key string) bool

	// AllowN checks if n requests should be allowed.
	AllowN(key string, n int) bool

	// Reset resets the rate limit for a key.
	Reset(key string)

	// Close releases resources.
	Close()
}

// RateLimiterConfig configures rate limiting behavior.
type RateLimiterConfig struct {
	// Rate is the maximum number of events per interval.
	Rate int

	// Interval is the time window for rate limiting.
	Interval time.Duration

	// Burst is the maximum burst size (bucket capacity).
	Burst int

	// CleanupInterval is how often to clean up expired entries.
	CleanupInterval time.Duration
}

// DefaultRateLimiterConfig returns sensible defaults for rate limiting.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:            100,
		Interval:        time.Second,
		Burst:           200,
		CleanupInterval: time.Minute,
	}
}

// ConnectionLimiter tracks connection counts and enforces limits.
type ConnectionLimiter interface {
	// CanAcceptInbound checks if an inbound connection can be accepted.
	CanAcceptInbound() bool

	// CanDialOutbound checks if an outbound connection can be made.
	CanDialOutbound() bool

	// OnConnect is called when a connection is established.
	OnConnect(peerID []byte, isInbound bool)

	// OnDisconnect is called when a connection is closed.
	OnDisconnect(peerID []byte)

	// InboundCount returns the current inbound connection count.
	InboundCount() int

	// OutboundCount returns the current outbound connection count.
	OutboundCount() int

	// TotalCount returns the total connection count.
	TotalCount() int
}

// EclipseMitigation provides protection against eclipse attacks.
// An eclipse attack isolates a node by surrounding it with attacker-controlled peers.
type EclipseMitigation interface {
	// ShouldAcceptPeer determines if a peer connection should be accepted.
	// Returns false if accepting this peer would increase eclipse attack risk.
	ShouldAcceptPeer(peerID []byte, addr string, isInbound bool) bool

	// RecordPeerSource records where a peer address came from.
	// This helps detect if addresses are coming from a single source.
	RecordPeerSource(peerID []byte, sourceID []byte)

	// GetPeerDiversity returns a diversity score (0-100).
	// Lower scores indicate higher eclipse attack risk.
	GetPeerDiversity() int

	// OnPeerMisbehavior records peer misbehavior for scoring.
	OnPeerMisbehavior(peerID []byte, reason string)
}

// EclipseMitigationConfig configures eclipse attack mitigation.
type EclipseMitigationConfig struct {
	// MinPeerDiversity is the minimum diversity score to maintain.
	// If diversity drops below this, new connections may be restricted.
	MinPeerDiversity int

	// MaxPeersPerSubnet limits peers from the same subnet.
	MaxPeersPerSubnet int

	// MaxPeersFromSameSource limits peers learned from the same source.
	MaxPeersFromSameSource int

	// RequireOutbound requires some percentage of outbound connections.
	// This prevents attackers from filling all slots with inbound connections.
	RequireOutboundPercent int

	// TrustDuration is how long a well-behaved peer is trusted.
	TrustDuration time.Duration
}

// DefaultEclipseMitigationConfig returns sensible defaults for eclipse mitigation.
func DefaultEclipseMitigationConfig() EclipseMitigationConfig {
	return EclipseMitigationConfig{
		MinPeerDiversity:       30,
		MaxPeersPerSubnet:      3,
		MaxPeersFromSameSource: 5,
		RequireOutboundPercent: 20,
		TrustDuration:          7 * 24 * time.Hour, // 1 week
	}
}

// BandwidthLimiter tracks and limits bandwidth usage.
type BandwidthLimiter interface {
	// ReserveRead reserves bytes for reading.
	// Returns true if the reservation is allowed.
	ReserveRead(bytes int) bool

	// ReserveWrite reserves bytes for writing.
	// Returns true if the reservation is allowed.
	ReserveWrite(bytes int) bool

	// ReadBytesUsed returns bytes read in the current interval.
	ReadBytesUsed() int64

	// WriteBytesUsed returns bytes written in the current interval.
	WriteBytesUsed() int64

	// SetLimits updates the bandwidth limits.
	SetLimits(readBytesPerSecond, writeBytesPerSecond int64)
}

// BandwidthLimiterConfig configures bandwidth limiting.
type BandwidthLimiterConfig struct {
	// MaxReadBytesPerSecond is the maximum read bandwidth.
	MaxReadBytesPerSecond int64

	// MaxWriteBytesPerSecond is the maximum write bandwidth.
	MaxWriteBytesPerSecond int64

	// BurstMultiplier allows bursting up to this multiple of the rate.
	BurstMultiplier float64
}

// DefaultBandwidthLimiterConfig returns sensible defaults for bandwidth limiting.
func DefaultBandwidthLimiterConfig() BandwidthLimiterConfig {
	return BandwidthLimiterConfig{
		MaxReadBytesPerSecond:  50 * 1024 * 1024, // 50 MB/s
		MaxWriteBytesPerSecond: 50 * 1024 * 1024, // 50 MB/s
		BurstMultiplier:        2.0,
	}
}

// Security-related errors.
var (
	ErrInvalidLimit    = &SecurityError{Code: 1, Message: "invalid resource limit"}
	ErrRateLimited     = &SecurityError{Code: 2, Message: "rate limit exceeded"}
	ErrConnectionLimit = &SecurityError{Code: 3, Message: "connection limit reached"}
	ErrBandwidthLimit  = &SecurityError{Code: 4, Message: "bandwidth limit exceeded"}
	ErrEclipseRisk     = &SecurityError{Code: 5, Message: "eclipse attack risk detected"}
	ErrPeerBanned      = &SecurityError{Code: 6, Message: "peer is banned"}
	ErrMessageTooLarge = &SecurityError{Code: 7, Message: "message exceeds size limit"}
	ErrTxTooLarge      = &SecurityError{Code: 8, Message: "transaction exceeds size limit"}
)

// SecurityError represents a security-related error.
type SecurityError struct {
	Code    int
	Message string
}

// Error implements the error interface.
func (e *SecurityError) Error() string {
	return e.Message
}
