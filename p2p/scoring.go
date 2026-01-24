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

// PeerScorer manages peer scoring and penalties.
type PeerScorer struct {
	peerManager *PeerManager

	// Track ban times for calculating escalating bans
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
		peerManager: pm,
		banCounts:   make(map[peer.ID]int),
		events:      make([]PenaltyEvent, 0),
		maxEvents:   1000,
	}
}

// AddPenalty adds penalty points to a peer.
func (ps *PeerScorer) AddPenalty(peerID peer.ID, points int64, reason PenaltyReason, message string) {
	state := ps.peerManager.GetPeer(peerID)
	if state == nil {
		return
	}

	state.AddPenalty(points)

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
func (ps *PeerScorer) GetPenaltyPoints(peerID peer.ID) int64 {
	state := ps.peerManager.GetPeer(peerID)
	if state == nil {
		return 0
	}
	return state.GetPenaltyPoints()
}

// ShouldBan returns true if the peer should be banned.
func (ps *PeerScorer) ShouldBan(peerID peer.ID) bool {
	return ps.GetPenaltyPoints(peerID) >= PenaltyThresholdBan
}

// GetBanDuration returns the ban duration for a peer based on their history.
func (ps *PeerScorer) GetBanDuration(peerID peer.ID) time.Duration {
	ps.mu.RLock()
	banCount := ps.banCounts[peerID]
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
func (ps *PeerScorer) RecordBan(peerID peer.ID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.banCounts[peerID]++
}

// ResetBanCount resets the ban count for a peer (e.g., after a long time without issues).
func (ps *PeerScorer) ResetBanCount(peerID peer.ID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
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
