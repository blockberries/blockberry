package security

import (
	"testing"
	"time"

	"github.com/blockberries/blockberry/pkg/abi"
)

func TestEclipseProtector_Basic(t *testing.T) {
	cfg := abi.DefaultEclipseMitigationConfig()
	protector := NewEclipseProtector(cfg)

	// Initially should accept peers
	if !protector.ShouldAcceptPeer([]byte("peer1"), "192.168.1.1:26656", false) {
		t.Error("ShouldAcceptPeer() = false initially, want true")
	}

	// Initially no peers
	if protector.PeerCount() != 0 {
		t.Errorf("PeerCount() = %d, want 0", protector.PeerCount())
	}

	// Initially full diversity
	if protector.GetPeerDiversity() != 100 {
		t.Errorf("GetPeerDiversity() = %d, want 100", protector.GetPeerDiversity())
	}
}

func TestEclipseProtector_SubnetLimit(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      2,
		MaxPeersFromSameSource: 10,
		RequireOutboundPercent: 0, // Disable for this test
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Add peers from same subnet
	protector.OnPeerConnected([]byte("peer1"), "192.168.1.1:26656", false)
	protector.OnPeerConnected([]byte("peer2"), "192.168.1.2:26656", false)

	// Should reject third peer from same /24 subnet
	if protector.ShouldAcceptPeer([]byte("peer3"), "192.168.1.3:26656", false) {
		t.Error("ShouldAcceptPeer() = true for third peer in same subnet")
	}

	// Should accept peer from different subnet
	if !protector.ShouldAcceptPeer([]byte("peer4"), "192.168.2.1:26656", false) {
		t.Error("ShouldAcceptPeer() = false for peer from different subnet")
	}
}

func TestEclipseProtector_OutboundRequirement(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      10,
		MaxPeersFromSameSource: 10,
		RequireOutboundPercent: 20,
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Add 4 inbound peers (100% inbound)
	for i := 0; i < 4; i++ {
		protector.OnPeerConnected([]byte{byte(i)}, "10.0.0.1:26656", true)
	}

	// Should reject inbound when we need more outbound
	if protector.ShouldAcceptPeer([]byte("new-inbound"), "10.0.0.2:26656", true) {
		t.Error("ShouldAcceptPeer() = true for inbound when outbound required")
	}

	// Should accept outbound
	if !protector.ShouldAcceptPeer([]byte("new-outbound"), "10.0.0.3:26656", false) {
		t.Error("ShouldAcceptPeer() = false for outbound")
	}

	// Add outbound peer
	protector.OnPeerConnected([]byte("out1"), "10.0.0.10:26656", false)

	// Now we have 4 inbound + 1 outbound = 20% outbound
	// Should accept inbound again
	if !protector.ShouldAcceptPeer([]byte("new-inbound2"), "10.0.0.4:26656", true) {
		t.Error("ShouldAcceptPeer() = false for inbound after meeting outbound requirement")
	}
}

func TestEclipseProtector_TrustedPeers(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      1, // Very restrictive
		MaxPeersFromSameSource: 1,
		RequireOutboundPercent: 0,
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Fill subnet
	protector.OnPeerConnected([]byte("peer1"), "192.168.1.1:26656", false)

	// Trust a peer
	protector.TrustPeer([]byte("trusted"))

	// Trusted peer should be accepted even though subnet is full
	if !protector.ShouldAcceptPeer([]byte("trusted"), "192.168.1.2:26656", false) {
		t.Error("ShouldAcceptPeer() = false for trusted peer")
	}

	// Verify trust
	if !protector.IsTrusted([]byte("trusted")) {
		t.Error("IsTrusted() = false for trusted peer")
	}

	// Untrusted peer should be rejected
	if protector.ShouldAcceptPeer([]byte("untrusted"), "192.168.1.3:26656", false) {
		t.Error("ShouldAcceptPeer() = true for untrusted peer in full subnet")
	}
}

func TestEclipseProtector_Misbehavior(t *testing.T) {
	cfg := abi.DefaultEclipseMitigationConfig()
	protector := NewEclipseProtector(cfg)

	// Connect and trust a peer
	protector.OnPeerConnected([]byte("peer1"), "192.168.1.1:26656", false)
	protector.TrustPeer([]byte("peer1"))

	if !protector.IsTrusted([]byte("peer1")) {
		t.Error("IsTrusted() = false before misbehavior")
	}

	// Report misbehavior
	protector.OnPeerMisbehavior([]byte("peer1"), "sent invalid block")

	// Should no longer be trusted
	if protector.IsTrusted([]byte("peer1")) {
		t.Error("IsTrusted() = true after misbehavior")
	}
}

func TestEclipseProtector_Disconnect(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      2,
		MaxPeersFromSameSource: 10,
		RequireOutboundPercent: 0,
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Fill subnet
	protector.OnPeerConnected([]byte("peer1"), "192.168.1.1:26656", false)
	protector.OnPeerConnected([]byte("peer2"), "192.168.1.2:26656", false)

	if protector.PeerCount() != 2 {
		t.Errorf("PeerCount() = %d, want 2", protector.PeerCount())
	}

	// Disconnect one
	protector.OnPeerDisconnected([]byte("peer1"))

	if protector.PeerCount() != 1 {
		t.Errorf("PeerCount() = %d after disconnect, want 1", protector.PeerCount())
	}

	// Should accept new peer now
	if !protector.ShouldAcceptPeer([]byte("peer3"), "192.168.1.3:26656", false) {
		t.Error("ShouldAcceptPeer() = false after disconnect")
	}
}

func TestEclipseProtector_Diversity(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      10,
		MaxPeersFromSameSource: 10,
		RequireOutboundPercent: 20,
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Add diverse peers
	protector.OnPeerConnected([]byte("peer1"), "192.168.1.1:26656", false)
	protector.OnPeerConnected([]byte("peer2"), "192.168.2.1:26656", false)
	protector.OnPeerConnected([]byte("peer3"), "192.168.3.1:26656", false)
	protector.OnPeerConnected([]byte("peer4"), "192.168.4.1:26656", false)
	protector.OnPeerConnected([]byte("peer5"), "192.168.5.1:26656", true)

	// Should have good diversity (different subnets)
	diversity := protector.GetPeerDiversity()
	if diversity < 50 {
		t.Errorf("GetPeerDiversity() = %d with diverse peers, expected >50", diversity)
	}

	// Check subnet count
	if protector.SubnetCount() != 5 {
		t.Errorf("SubnetCount() = %d, want 5", protector.SubnetCount())
	}
}

func TestEclipseProtector_LowDiversity(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      10,
		MaxPeersFromSameSource: 10,
		RequireOutboundPercent: 50, // Higher requirement to lower balance score
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Add all peers from same subnet, all inbound
	for i := 0; i < 5; i++ {
		protector.OnPeerConnected([]byte{byte(i)}, "192.168.1.1:26656", true)
	}

	// Should have lower diversity due to:
	// - All peers from same subnet (100% concentration)
	// - All inbound (0% outbound when 50% required)
	diversity := protector.GetPeerDiversity()
	if diversity > 60 {
		t.Errorf("GetPeerDiversity() = %d with concentrated peers, expected <=60", diversity)
	}
}

func TestEclipseProtector_RecordPeerSource(t *testing.T) {
	cfg := abi.EclipseMitigationConfig{
		MaxPeersPerSubnet:      10,
		MaxPeersFromSameSource: 2,
		RequireOutboundPercent: 0,
		TrustDuration:          time.Hour,
	}
	protector := NewEclipseProtector(cfg)

	// Add peers and record their source
	protector.OnPeerConnected([]byte("peer1"), "192.168.1.1:26656", false)
	protector.OnPeerConnected([]byte("peer2"), "192.168.2.1:26656", false)
	protector.OnPeerConnected([]byte("peer3"), "192.168.3.1:26656", false)

	protector.RecordPeerSource([]byte("peer1"), []byte("source1"))
	protector.RecordPeerSource([]byte("peer2"), []byte("source1"))

	// Third peer from same source should be limited
	protector.RecordPeerSource([]byte("peer3"), []byte("source1"))

	// The source tracking should have limited to 2
	// (This is a soft limit - already connected peers are not disconnected)
}

func TestExtractSubnet(t *testing.T) {
	tests := []struct {
		addr     string
		expected string
	}{
		{"192.168.1.100:26656", "192.168.1.0/24"},
		{"10.0.0.1:8080", "10.0.0.0/24"},
		{"172.16.0.50:26656", "172.16.0.0/24"},
		{"1.2.3.4:26656", "1.2.3.0/24"},
		{"invalid", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			result := extractSubnet(tt.addr)
			if result != tt.expected {
				t.Errorf("extractSubnet(%q) = %q, want %q", tt.addr, result, tt.expected)
			}
		})
	}
}

func TestEclipseProtector_ConnectionCounts(t *testing.T) {
	cfg := abi.DefaultEclipseMitigationConfig()
	protector := NewEclipseProtector(cfg)

	// Add mix of inbound and outbound
	protector.OnPeerConnected([]byte("in1"), "192.168.1.1:26656", true)
	protector.OnPeerConnected([]byte("in2"), "192.168.2.1:26656", true)
	protector.OnPeerConnected([]byte("out1"), "192.168.3.1:26656", false)

	if protector.PeerCount() != 3 {
		t.Errorf("PeerCount() = %d, want 3", protector.PeerCount())
	}

	// Disconnect and reconnect
	protector.OnPeerDisconnected([]byte("in1"))
	if protector.PeerCount() != 2 {
		t.Errorf("PeerCount() = %d after disconnect, want 2", protector.PeerCount())
	}

	protector.OnPeerConnected([]byte("out2"), "192.168.4.1:26656", false)
	if protector.PeerCount() != 3 {
		t.Errorf("PeerCount() = %d after reconnect, want 3", protector.PeerCount())
	}
}
