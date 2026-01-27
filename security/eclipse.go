package security

import (
	"net"
	"sync"
	"time"

	"github.com/blockberries/blockberry/abi"
)

// EclipseProtector implements abi.EclipseMitigation.
// It tracks peer diversity and prevents eclipse attacks by:
// 1. Limiting peers from the same subnet
// 2. Limiting peers learned from the same source
// 3. Requiring minimum outbound connections
// 4. Tracking peer behavior and penalizing bad actors
type EclipseProtector struct {
	cfg abi.EclipseMitigationConfig

	// Peer tracking
	peers       map[string]*peerInfo
	subnetPeers map[string][]string // subnet -> peer IDs
	sourcePeers map[string][]string // source ID -> peer IDs learned from that source

	// Trust tracking
	trustedPeers map[string]time.Time // peer ID -> trust expiration

	// Connection counts
	inboundCount  int
	outboundCount int

	mu sync.RWMutex
}

type peerInfo struct {
	id         string
	addr       string
	subnet     string
	sourceID   string
	isInbound  bool
	connectedAt time.Time
	misbehaviors int
}

// NewEclipseProtector creates a new eclipse attack protector.
func NewEclipseProtector(cfg abi.EclipseMitigationConfig) *EclipseProtector {
	return &EclipseProtector{
		cfg:          cfg,
		peers:        make(map[string]*peerInfo),
		subnetPeers:  make(map[string][]string),
		sourcePeers:  make(map[string][]string),
		trustedPeers: make(map[string]time.Time),
	}
}

// ShouldAcceptPeer determines if a peer connection should be accepted.
func (p *EclipseProtector) ShouldAcceptPeer(peerID []byte, addr string, isInbound bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := string(peerID)

	// Always accept trusted peers
	if expiry, ok := p.trustedPeers[id]; ok && time.Now().Before(expiry) {
		return true
	}

	// Check outbound requirement
	if isInbound {
		totalPeers := p.inboundCount + p.outboundCount
		if totalPeers > 0 {
			outboundPercent := float64(p.outboundCount) / float64(totalPeers) * 100
			if outboundPercent < float64(p.cfg.RequireOutboundPercent) {
				// We need more outbound connections, reject inbound
				return false
			}
		}
	}

	// Extract subnet from address
	subnet := extractSubnet(addr)
	if subnet != "" {
		// Check subnet limit
		peersInSubnet := p.subnetPeers[subnet]
		if len(peersInSubnet) >= p.cfg.MaxPeersPerSubnet {
			return false
		}
	}

	return true
}

// RecordPeerSource records where a peer address came from.
func (p *EclipseProtector) RecordPeerSource(peerID []byte, sourceID []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := string(peerID)
	source := string(sourceID)

	// Check if we already have too many peers from this source
	peersFromSource := p.sourcePeers[source]
	if len(peersFromSource) >= p.cfg.MaxPeersFromSameSource {
		return
	}

	// Record the source
	if info, ok := p.peers[id]; ok {
		info.sourceID = source
	}
	p.sourcePeers[source] = append(p.sourcePeers[source], id)
}

// GetPeerDiversity returns a diversity score (0-100).
func (p *EclipseProtector) GetPeerDiversity() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	totalPeers := len(p.peers)
	if totalPeers == 0 {
		return 100 // No peers = no concentration
	}

	// Calculate subnet diversity
	subnetScore := p.calculateSubnetDiversity(totalPeers)

	// Calculate source diversity
	sourceScore := p.calculateSourceDiversity(totalPeers)

	// Calculate inbound/outbound balance
	balanceScore := p.calculateBalanceScore()

	// Weight the scores
	diversity := (subnetScore*40 + sourceScore*40 + balanceScore*20) / 100

	if diversity > 100 {
		diversity = 100
	}
	if diversity < 0 {
		diversity = 0
	}

	return diversity
}

// calculateSubnetDiversity calculates diversity based on subnet distribution.
func (p *EclipseProtector) calculateSubnetDiversity(totalPeers int) int {
	if len(p.subnetPeers) == 0 {
		return 100
	}

	// Find max concentration in any subnet
	maxConcentration := 0
	for _, peers := range p.subnetPeers {
		if len(peers) > maxConcentration {
			maxConcentration = len(peers)
		}
	}

	// Score based on concentration
	concentrationPercent := float64(maxConcentration) / float64(totalPeers) * 100

	// Lower concentration = higher score
	score := 100 - int(concentrationPercent)
	if score < 0 {
		score = 0
	}
	return score
}

// calculateSourceDiversity calculates diversity based on peer sources.
func (p *EclipseProtector) calculateSourceDiversity(totalPeers int) int {
	if len(p.sourcePeers) == 0 {
		return 100
	}

	// Find max concentration from any source
	maxConcentration := 0
	for _, peers := range p.sourcePeers {
		if len(peers) > maxConcentration {
			maxConcentration = len(peers)
		}
	}

	// Score based on concentration
	concentrationPercent := float64(maxConcentration) / float64(totalPeers) * 100

	score := 100 - int(concentrationPercent)
	if score < 0 {
		score = 0
	}
	return score
}

// calculateBalanceScore calculates score based on inbound/outbound balance.
func (p *EclipseProtector) calculateBalanceScore() int {
	total := p.inboundCount + p.outboundCount
	if total == 0 {
		return 100
	}

	outboundPercent := float64(p.outboundCount) / float64(total) * 100

	// Ideal is around 20-30% outbound
	if outboundPercent >= float64(p.cfg.RequireOutboundPercent) {
		return 100
	}

	// Score drops as we have fewer outbound
	return int(outboundPercent / float64(p.cfg.RequireOutboundPercent) * 100)
}

// OnPeerMisbehavior records peer misbehavior for scoring.
func (p *EclipseProtector) OnPeerMisbehavior(peerID []byte, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := string(peerID)
	if info, ok := p.peers[id]; ok {
		info.misbehaviors++

		// Remove from trusted if misbehaving
		delete(p.trustedPeers, id)
	}
}

// OnPeerConnected is called when a peer connects.
func (p *EclipseProtector) OnPeerConnected(peerID []byte, addr string, isInbound bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := string(peerID)
	subnet := extractSubnet(addr)

	info := &peerInfo{
		id:          id,
		addr:        addr,
		subnet:      subnet,
		isInbound:   isInbound,
		connectedAt: time.Now(),
	}

	p.peers[id] = info

	if subnet != "" {
		p.subnetPeers[subnet] = append(p.subnetPeers[subnet], id)
	}

	if isInbound {
		p.inboundCount++
	} else {
		p.outboundCount++
	}
}

// OnPeerDisconnected is called when a peer disconnects.
func (p *EclipseProtector) OnPeerDisconnected(peerID []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := string(peerID)
	info, ok := p.peers[id]
	if !ok {
		return
	}

	// Update connection counts
	if info.isInbound {
		p.inboundCount--
	} else {
		p.outboundCount--
	}

	// Remove from subnet tracking
	if info.subnet != "" {
		peers := p.subnetPeers[info.subnet]
		for i, pid := range peers {
			if pid == id {
				p.subnetPeers[info.subnet] = append(peers[:i], peers[i+1:]...)
				break
			}
		}
		if len(p.subnetPeers[info.subnet]) == 0 {
			delete(p.subnetPeers, info.subnet)
		}
	}

	// Remove from source tracking
	if info.sourceID != "" {
		peers := p.sourcePeers[info.sourceID]
		for i, pid := range peers {
			if pid == id {
				p.sourcePeers[info.sourceID] = append(peers[:i], peers[i+1:]...)
				break
			}
		}
		if len(p.sourcePeers[info.sourceID]) == 0 {
			delete(p.sourcePeers, info.sourceID)
		}
	}

	// Check if peer should be trusted for future connections
	if info.misbehaviors == 0 && time.Since(info.connectedAt) > 24*time.Hour {
		p.trustedPeers[id] = time.Now().Add(p.cfg.TrustDuration)
	}

	delete(p.peers, id)
}

// TrustPeer explicitly trusts a peer (e.g., from config).
func (p *EclipseProtector) TrustPeer(peerID []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := string(peerID)
	p.trustedPeers[id] = time.Now().Add(p.cfg.TrustDuration)
}

// IsTrusted checks if a peer is trusted.
func (p *EclipseProtector) IsTrusted(peerID []byte) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	id := string(peerID)
	expiry, ok := p.trustedPeers[id]
	return ok && time.Now().Before(expiry)
}

// PeerCount returns the current peer count.
func (p *EclipseProtector) PeerCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.peers)
}

// SubnetCount returns the number of distinct subnets.
func (p *EclipseProtector) SubnetCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.subnetPeers)
}

// extractSubnet extracts the /24 subnet from an address.
func extractSubnet(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return ""
	}

	// Get IPv4 address
	ip4 := ip.To4()
	if ip4 == nil {
		// For IPv6, use /48 prefix
		if len(ip) >= 6 {
			return ip[:6].String()
		}
		return ""
	}

	// Return /24 subnet as "x.y.z.0/24"
	subnet := &net.IPNet{
		IP:   net.IPv4(ip4[0], ip4[1], ip4[2], 0),
		Mask: net.CIDRMask(24, 32),
	}
	return subnet.String()
}

// Ensure EclipseProtector implements abi.EclipseMitigation.
var _ abi.EclipseMitigation = (*EclipseProtector)(nil)
