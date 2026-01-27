package p2p

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Penalty thresholds and durations.
const (
	// PenaltyThresholdWarn is the threshold for warning level.
	PenaltyThresholdWarn = 10

	// PenaltyThresholdBan is the threshold for banning a peer.
	PenaltyThresholdBan = 100

	// PenaltyDecayRate is how many points decay per hour.
	PenaltyDecayRate = 1

	// BanDurationBase is the base duration for a ban.
	BanDurationBase = 1 * time.Hour

	// BanDurationMax is the maximum ban duration.
	BanDurationMax = 24 * time.Hour
)

// Penalty reasons and their point values.
const (
	PenaltyInvalidMessage    = 5
	PenaltyInvalidBlock      = 20
	PenaltyInvalidTx         = 5
	PenaltyTimeout           = 2
	PenaltyDuplicateMessage  = 1
	PenaltyProtocolViolation = 50
	PenaltyChainMismatch     = 100
	PenaltyVersionMismatch   = 100
)

// PenaltyReason describes why a penalty was applied.
type PenaltyReason string

const (
	ReasonInvalidMessage    PenaltyReason = "invalid_message"
	ReasonInvalidBlock      PenaltyReason = "invalid_block"
	ReasonInvalidTx         PenaltyReason = "invalid_tx"
	ReasonTimeout           PenaltyReason = "timeout"
	ReasonDuplicateMessage  PenaltyReason = "duplicate_message"
	ReasonProtocolViolation PenaltyReason = "protocol_violation"
	ReasonChainMismatch     PenaltyReason = "chain_mismatch"
	ReasonVersionMismatch   PenaltyReason = "version_mismatch"
)

// PenaltyEvent records a penalty applied to a peer.
type PenaltyEvent struct {
	PeerID    peer.ID
	Points    int64
	Reason    PenaltyReason
	Message   string
	Timestamp time.Time
}

// PenaltyRecord stores the penalty state for a peer, including history for decay calculation.
// This persists even when peers disconnect, allowing accurate decay over time.
type PenaltyRecord struct {
	Points     int64     // Current raw penalty points (before decay)
	LastDecay  time.Time // Last time decay was calculated
	BanCount   int       // Number of times this peer has been banned
	LastBanEnd time.Time // When the last ban ended (for progressive bans)
}

// PeerScorer manages peer scoring and penalties.
type PeerScorer struct {
	peerManager *PeerManager

	// Track penalty history for all peers (persists across disconnects)
	penaltyHistory map[peer.ID]*PenaltyRecord

	// Track ban times for calculating escalating bans (legacy, kept for compatibility)
	banCounts map[peer.ID]int

	// Penalty event log (for debugging/monitoring)
	events     []PenaltyEvent
	maxEvents  int
	eventsLock sync.Mutex

	mu sync.RWMutex
}

// NewPeerScorer creates a new peer scorer.
func NewPeerScorer(pm *PeerManager) *PeerScorer {
	return &PeerScorer{
		peerManager:    pm,
		penaltyHistory: make(map[peer.ID]*PenaltyRecord),
		banCounts:      make(map[peer.ID]int),
		events:         make([]PenaltyEvent, 0),
		maxEvents:      1000,
	}
}

// AddPenalty adds penalty points to a peer.
// Points are stored in penalty history so they persist across disconnects.
func (ps *PeerScorer) AddPenalty(peerID peer.ID, points int64, reason PenaltyReason, message string) {
	ps.mu.Lock()
	record := ps.penaltyHistory[peerID]
	if record == nil {
		record = &PenaltyRecord{
			LastDecay: time.Now(),
		}
		ps.penaltyHistory[peerID] = record
	}

	// Apply any pending decay before adding new points
	ps.applyDecayLocked(record)
	record.Points += points
	ps.mu.Unlock()

	// Also update the connected peer's state if they're connected
	if ps.peerManager != nil {
		state := ps.peerManager.GetPeer(peerID)
		if state != nil {
			state.AddPenalty(points)
		}
	}

	// Log the event
	ps.logEvent(PenaltyEvent{
		PeerID:    peerID,
		Points:    points,
		Reason:    reason,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// GetPenaltyPoints returns the current penalty points for a peer.
// For connected peers, returns the live state; for disconnected peers,
// calculates decay based on wall-clock time from penalty history.
func (ps *PeerScorer) GetPenaltyPoints(peerID peer.ID) int64 {
	// Try connected peer state first
	if ps.peerManager != nil {
		state := ps.peerManager.GetPeer(peerID)
		if state != nil {
			return state.GetPenaltyPoints()
		}
	}

	// Fall back to penalty history with wall-clock decay
	return ps.GetEffectivePenaltyPoints(peerID)
}

// ShouldBan returns true if the peer should be banned.
// Uses GetEffectivePenaltyPoints to ensure decay is applied for all peers.
func (ps *PeerScorer) ShouldBan(peerID peer.ID) bool {
	return ps.GetEffectivePenaltyPoints(peerID) >= PenaltyThresholdBan
}

// GetBanDuration returns the ban duration for a peer based on their history.
// Uses the consolidated penalty history for ban count tracking.
func (ps *PeerScorer) GetBanDuration(peerID peer.ID) time.Duration {
	ps.mu.RLock()
	record := ps.penaltyHistory[peerID]
	banCount := 0
	if record != nil {
		banCount = record.BanCount
	}
	ps.mu.RUnlock()

	// Exponential backoff: 1h, 2h, 4h, 8h, ... up to max
	// Cap shift amount to avoid overflow (24h max means we only need ~5 doublings)
	shift := max(0, min(banCount, 5))
	duration := BanDurationBase * time.Duration(1<<shift)
	if duration > BanDurationMax {
		duration = BanDurationMax
	}
	return duration
}

// RecordBan records that a peer was banned (for escalating future bans).
// Updates both the consolidated penalty history and the legacy banCounts map.
func (ps *PeerScorer) RecordBan(peerID peer.ID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Update consolidated penalty history
	record := ps.penaltyHistory[peerID]
	if record == nil {
		record = &PenaltyRecord{
			LastDecay: time.Now(),
		}
		ps.penaltyHistory[peerID] = record
	}
	record.BanCount++
	record.LastBanEnd = time.Now().Add(ps.getBanDurationLocked(record.BanCount - 1))

	// Also update legacy map for compatibility
	ps.banCounts[peerID]++
}

// getBanDurationLocked calculates ban duration for a given ban count.
// Must be called with ps.mu held.
func (ps *PeerScorer) getBanDurationLocked(banCount int) time.Duration {
	shift := max(0, min(banCount, 5))
	duration := BanDurationBase * time.Duration(1<<shift)
	if duration > BanDurationMax {
		duration = BanDurationMax
	}
	return duration
}

// ResetBanCount resets the ban count for a peer (e.g., after a long time without issues).
func (ps *PeerScorer) ResetBanCount(peerID peer.ID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Reset in penalty history
	if record := ps.penaltyHistory[peerID]; record != nil {
		record.BanCount = 0
		record.LastBanEnd = time.Time{}
	}

	// Also reset legacy map
	delete(ps.banCounts, peerID)
}

// DecayPenalties applies penalty decay to all peers.
// This should be called periodically (e.g., every hour).
func (ps *PeerScorer) DecayPenalties(points int64) {
	peers := ps.peerManager.AllPeers()
	for _, state := range peers {
		state.DecayPenalty(points)
	}
}

// StartDecayLoop starts a background goroutine that decays penalties.
// The loop runs until the stop channel is closed.
func (ps *PeerScorer) StartDecayLoop(stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ps.DecayPenalties(PenaltyDecayRate)
			case <-stop:
				return
			}
		}
	}()
}

// applyDecayLocked applies pending decay to a penalty record.
// Must be called with ps.mu held.
func (ps *PeerScorer) applyDecayLocked(record *PenaltyRecord) {
	if record == nil {
		return
	}

	elapsed := time.Since(record.LastDecay)
	hoursElapsed := int64(elapsed.Hours())
	if hoursElapsed > 0 {
		decay := hoursElapsed * PenaltyDecayRate
		record.Points -= decay
		if record.Points < 0 {
			record.Points = 0
		}
		record.LastDecay = time.Now()
	}
}

// GetEffectivePenaltyPoints returns the current penalty points for a peer
// after applying wall-clock-based decay. This works for both connected
// and disconnected peers.
func (ps *PeerScorer) GetEffectivePenaltyPoints(peerID peer.ID) int64 {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	record := ps.penaltyHistory[peerID]
	if record == nil {
		return 0
	}

	// Apply decay before returning
	ps.applyDecayLocked(record)
	return record.Points
}

// logEvent adds a penalty event to the log.
func (ps *PeerScorer) logEvent(event PenaltyEvent) {
	ps.eventsLock.Lock()
	defer ps.eventsLock.Unlock()

	ps.events = append(ps.events, event)

	// Trim if too many events
	if len(ps.events) > ps.maxEvents {
		ps.events = ps.events[len(ps.events)-ps.maxEvents:]
	}
}

// RecentEvents returns recent penalty events.
func (ps *PeerScorer) RecentEvents(count int) []PenaltyEvent {
	ps.eventsLock.Lock()
	defer ps.eventsLock.Unlock()

	if count > len(ps.events) {
		count = len(ps.events)
	}

	result := make([]PenaltyEvent, count)
	copy(result, ps.events[len(ps.events)-count:])
	return result
}

// PeerEventsCount returns the number of penalty events for a specific peer.
func (ps *PeerScorer) PeerEventsCount(peerID peer.ID) int {
	ps.eventsLock.Lock()
	defer ps.eventsLock.Unlock()

	count := 0
	for _, event := range ps.events {
		if event.PeerID == peerID {
			count++
		}
	}
	return count
}
