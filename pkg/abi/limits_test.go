package abi

import (
	"testing"
	"time"
)

func TestDefaultResourceLimits(t *testing.T) {
	limits := DefaultResourceLimits()

	// Check transaction limits
	if limits.MaxTxSize <= 0 {
		t.Error("MaxTxSize should be positive")
	}
	if limits.MaxTxsBytes <= 0 {
		t.Error("MaxTxsBytes should be positive")
	}

	// Check block limits
	if limits.MaxBlockSize <= 0 {
		t.Error("MaxBlockSize should be positive")
	}
	if limits.MaxBlockTxs <= 0 {
		t.Error("MaxBlockTxs should be positive")
	}

	// Check connection limits
	if limits.MaxPeers <= 0 {
		t.Error("MaxPeers should be positive")
	}
	if limits.MaxInboundPeers+limits.MaxOutboundPeers > limits.MaxPeers {
		t.Error("Inbound + Outbound should not exceed MaxPeers")
	}

	// Check rate limits
	if limits.MaxTxsPerSecond <= 0 {
		t.Error("MaxTxsPerSecond should be positive")
	}
}

func TestResourceLimits_Validate(t *testing.T) {
	tests := []struct {
		name    string
		limits  ResourceLimits
		wantErr bool
	}{
		{
			name:    "valid defaults",
			limits:  DefaultResourceLimits(),
			wantErr: false,
		},
		{
			name: "invalid MaxTxSize",
			limits: ResourceLimits{
				MaxTxSize:    0,
				MaxBlockSize: 1000,
				MaxMsgSize:   1000,
			},
			wantErr: true,
		},
		{
			name: "invalid MaxBlockSize",
			limits: ResourceLimits{
				MaxTxSize:    1000,
				MaxBlockSize: 0,
				MaxMsgSize:   1000,
			},
			wantErr: true,
		},
		{
			name: "invalid MaxMsgSize",
			limits: ResourceLimits{
				MaxTxSize:    1000,
				MaxBlockSize: 1000,
				MaxMsgSize:   0,
			},
			wantErr: true,
		},
		{
			name: "negative MaxPeers",
			limits: ResourceLimits{
				MaxTxSize:    1000,
				MaxBlockSize: 1000,
				MaxMsgSize:   1000,
				MaxPeers:     -1,
			},
			wantErr: true,
		},
		{
			name: "inbound + outbound exceeds max",
			limits: ResourceLimits{
				MaxTxSize:        1000,
				MaxBlockSize:     1000,
				MaxMsgSize:       1000,
				MaxPeers:         10,
				MaxInboundPeers:  8,
				MaxOutboundPeers: 5,
			},
			wantErr: true,
		},
		{
			name: "valid custom limits",
			limits: ResourceLimits{
				MaxTxSize:        1000,
				MaxBlockSize:     1000,
				MaxMsgSize:       1000,
				MaxPeers:         10,
				MaxInboundPeers:  5,
				MaxOutboundPeers: 5,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.limits.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	cfg := DefaultRateLimiterConfig()

	if cfg.Rate <= 0 {
		t.Error("Rate should be positive")
	}
	if cfg.Interval <= 0 {
		t.Error("Interval should be positive")
	}
	if cfg.Burst <= 0 {
		t.Error("Burst should be positive")
	}
	if cfg.CleanupInterval <= 0 {
		t.Error("CleanupInterval should be positive")
	}
}

func TestDefaultEclipseMitigationConfig(t *testing.T) {
	cfg := DefaultEclipseMitigationConfig()

	if cfg.MinPeerDiversity <= 0 || cfg.MinPeerDiversity > 100 {
		t.Error("MinPeerDiversity should be between 1 and 100")
	}
	if cfg.MaxPeersPerSubnet <= 0 {
		t.Error("MaxPeersPerSubnet should be positive")
	}
	if cfg.MaxPeersFromSameSource <= 0 {
		t.Error("MaxPeersFromSameSource should be positive")
	}
	if cfg.RequireOutboundPercent < 0 || cfg.RequireOutboundPercent > 100 {
		t.Error("RequireOutboundPercent should be between 0 and 100")
	}
	if cfg.TrustDuration <= 0 {
		t.Error("TrustDuration should be positive")
	}
}

func TestDefaultBandwidthLimiterConfig(t *testing.T) {
	cfg := DefaultBandwidthLimiterConfig()

	if cfg.MaxReadBytesPerSecond <= 0 {
		t.Error("MaxReadBytesPerSecond should be positive")
	}
	if cfg.MaxWriteBytesPerSecond <= 0 {
		t.Error("MaxWriteBytesPerSecond should be positive")
	}
	if cfg.BurstMultiplier <= 0 {
		t.Error("BurstMultiplier should be positive")
	}
}

func TestSecurityError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *SecurityError
		want string
	}{
		{
			name: "ErrInvalidLimit",
			err:  ErrInvalidLimit,
			want: "invalid resource limit",
		},
		{
			name: "ErrRateLimited",
			err:  ErrRateLimited,
			want: "rate limit exceeded",
		},
		{
			name: "ErrConnectionLimit",
			err:  ErrConnectionLimit,
			want: "connection limit reached",
		},
		{
			name: "ErrBandwidthLimit",
			err:  ErrBandwidthLimit,
			want: "bandwidth limit exceeded",
		},
		{
			name: "ErrEclipseRisk",
			err:  ErrEclipseRisk,
			want: "eclipse attack risk detected",
		},
		{
			name: "ErrPeerBanned",
			err:  ErrPeerBanned,
			want: "peer is banned",
		},
		{
			name: "ErrMessageTooLarge",
			err:  ErrMessageTooLarge,
			want: "message exceeds size limit",
		},
		{
			name: "ErrTxTooLarge",
			err:  ErrTxTooLarge,
			want: "transaction exceeds size limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimiterConfig_Values(t *testing.T) {
	cfg := RateLimiterConfig{
		Rate:            50,
		Interval:        2 * time.Second,
		Burst:           100,
		CleanupInterval: 5 * time.Minute,
	}

	if cfg.Rate != 50 {
		t.Errorf("Rate = %d, want 50", cfg.Rate)
	}
	if cfg.Interval != 2*time.Second {
		t.Errorf("Interval = %v, want 2s", cfg.Interval)
	}
	if cfg.Burst != 100 {
		t.Errorf("Burst = %d, want 100", cfg.Burst)
	}
	if cfg.CleanupInterval != 5*time.Minute {
		t.Errorf("CleanupInterval = %v, want 5m", cfg.CleanupInterval)
	}
}

func TestResourceLimits_SpecificValues(t *testing.T) {
	limits := ResourceLimits{
		MaxTxSize:              512 * 1024,
		MaxTxsBytes:            32 * 1024 * 1024,
		MaxBlockSize:           10 * 1024 * 1024,
		MaxBlockTxs:            5000,
		MaxEvidenceSize:        512 * 1024,
		MaxMsgSize:             5 * 1024 * 1024,
		MaxPeers:               100,
		MaxInboundPeers:        80,
		MaxOutboundPeers:       20,
		MaxPendingPeers:        20,
		MaxSubscribers:         500,
		MaxSubscribersPerQuery: 50,
		MaxTxsPerSecond:        500,
		MaxMsgsPerSecond:       50,
		MaxBytesPerSecond:      5 * 1024 * 1024,
	}

	if err := limits.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}

	if limits.MaxTxSize != 512*1024 {
		t.Errorf("MaxTxSize = %d, want %d", limits.MaxTxSize, 512*1024)
	}
	if limits.MaxPeers != 100 {
		t.Errorf("MaxPeers = %d, want 100", limits.MaxPeers)
	}
}
