package handlers

import (
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/internal/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/pkg/types"
)

// Housekeeping message type IDs from schema.
const (
	TypeIDLatencyRequest   cramberry.TypeID = 140
	TypeIDLatencyResponse  cramberry.TypeID = 141
	TypeIDFirewallRequest  cramberry.TypeID = 142
	TypeIDFirewallResponse cramberry.TypeID = 143
)

// HousekeepingReactor handles latency probes and firewall detection.
type HousekeepingReactor struct {
	// Dependencies
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// Configuration
	probeInterval time.Duration

	// Pending latency probes
	pendingProbes map[peer.ID]int64 // peerID -> request timestamp
	mu            sync.RWMutex

	// Lifecycle
	running bool
	stop    chan struct{}
	wg      sync.WaitGroup
}

// NewHousekeepingReactor creates a new housekeeping reactor.
func NewHousekeepingReactor(
	network *p2p.Network,
	peerManager *p2p.PeerManager,
	probeInterval time.Duration,
) *HousekeepingReactor {
	return &HousekeepingReactor{
		network:       network,
		peerManager:   peerManager,
		probeInterval: probeInterval,
		pendingProbes: make(map[peer.ID]int64),
		stop:          make(chan struct{}),
	}
}

// Name returns the component name for identification.
func (r *HousekeepingReactor) Name() string {
	return "housekeeping-reactor"
}

// Start begins the periodic latency probe loop.
func (r *HousekeepingReactor) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.stop = make(chan struct{})
	r.mu.Unlock()

	r.wg.Add(1)
	go r.probeLoop()

	return nil
}

// Stop halts the latency probe loop.
func (r *HousekeepingReactor) Stop() error {
	r.mu.Lock()
	if !r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = false
	close(r.stop)
	r.mu.Unlock()

	r.wg.Wait()
	return nil
}

// IsRunning returns whether the reactor is running.
func (r *HousekeepingReactor) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// probeLoop periodically sends latency probes to all peers.
func (r *HousekeepingReactor) probeLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.probeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.sendLatencyProbes()
		}
	}
}

// sendLatencyProbes sends latency requests to all connected peers.
func (r *HousekeepingReactor) sendLatencyProbes() {
	if r.peerManager == nil || r.network == nil {
		return
	}

	peers := r.peerManager.GetConnectedPeers()
	now := time.Now().UnixNano()

	for _, peerID := range peers {
		if err := r.SendLatencyRequest(peerID, now); err != nil {
			continue
		}
	}
}

// SendLatencyRequest sends a latency request to a peer.
func (r *HousekeepingReactor) SendLatencyRequest(peerID peer.ID, timestamp int64) error {
	if r.network == nil {
		return nil
	}

	req := &schema.LatencyRequest{
		Timestamp: &timestamp,
	}

	data, err := r.encodeMessage(TypeIDLatencyRequest, req)
	if err != nil {
		return err
	}

	// Track pending request
	r.mu.Lock()
	r.pendingProbes[peerID] = timestamp
	r.mu.Unlock()

	return r.network.Send(peerID, p2p.StreamHousekeeping, data)
}

// HandleMessage processes incoming housekeeping messages.
func (r *HousekeepingReactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	reader := cramberry.NewReader(data)
	typeID := reader.ReadTypeID()
	if reader.Err() != nil {
		return types.ErrInvalidMessage
	}

	remaining := reader.Remaining()

	switch typeID {
	case TypeIDLatencyRequest:
		return r.handleLatencyRequest(peerID, remaining)
	case TypeIDLatencyResponse:
		return r.handleLatencyResponse(peerID, remaining)
	case TypeIDFirewallRequest:
		return r.handleFirewallRequest(peerID, remaining)
	case TypeIDFirewallResponse:
		return r.handleFirewallResponse(peerID, remaining)
	default:
		return types.ErrUnknownMessageType
	}
}

// handleLatencyRequest processes a latency probe request.
func (r *HousekeepingReactor) handleLatencyRequest(peerID peer.ID, data []byte) error {
	var req schema.LatencyRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if req.Timestamp == nil {
		return types.ErrInvalidMessage
	}

	// Calculate processing latency
	now := time.Now().UnixNano()
	processingTime := now - *req.Timestamp

	// Send response with the processing time
	return r.sendLatencyResponse(peerID, processingTime)
}

// sendLatencyResponse sends a latency response to a peer.
func (r *HousekeepingReactor) sendLatencyResponse(peerID peer.ID, latency int64) error {
	if r.network == nil {
		return nil
	}

	resp := &schema.LatencyResponse{
		Latency: &latency,
	}

	data, err := r.encodeMessage(TypeIDLatencyResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamHousekeeping, data)
}

// handleLatencyResponse processes a latency probe response.
func (r *HousekeepingReactor) handleLatencyResponse(peerID peer.ID, data []byte) error {
	var resp schema.LatencyResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if resp.Latency == nil {
		return types.ErrInvalidMessage
	}

	// Get the original request timestamp
	r.mu.Lock()
	requestTime, ok := r.pendingProbes[peerID]
	if ok {
		delete(r.pendingProbes, peerID)
	}
	r.mu.Unlock()

	if !ok {
		// No pending request for this peer
		return nil
	}

	// Calculate RTT
	now := time.Now().UnixNano()
	rtt := time.Duration(now - requestTime)

	// Update peer latency in peer manager
	if r.peerManager != nil {
		_ = r.peerManager.UpdateLatency(peerID, rtt)
	}

	return nil
}

// handleFirewallRequest handles a firewall detection request (stub).
func (r *HousekeepingReactor) handleFirewallRequest(peerID peer.ID, data []byte) error {
	var req schema.FirewallRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	// Stub: Just respond with the same endpoint
	return r.sendFirewallResponse(peerID, req.Endpoint)
}

// sendFirewallResponse sends a firewall response (stub).
func (r *HousekeepingReactor) sendFirewallResponse(peerID peer.ID, endpoint *string) error {
	if r.network == nil || endpoint == nil {
		return nil
	}

	resp := &schema.FirewallResponse{
		Endpoint: endpoint,
	}

	data, err := r.encodeMessage(TypeIDFirewallResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamHousekeeping, data)
}

// handleFirewallResponse handles a firewall detection response (stub).
func (r *HousekeepingReactor) handleFirewallResponse(peerID peer.ID, data []byte) error {
	var resp schema.FirewallResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	// Stub: Log result (in production, would update reachability status)
	// For now, just acknowledge the response
	return nil
}

// OnPeerDisconnected cleans up state for a disconnected peer.
func (r *HousekeepingReactor) OnPeerDisconnected(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.pendingProbes, peerID)
}

// GetPendingProbes returns the number of pending latency probes.
func (r *HousekeepingReactor) GetPendingProbes() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.pendingProbes)
}

// encodeMessage encodes a message with its type ID prefix.
func (r *HousekeepingReactor) encodeMessage(typeID cramberry.TypeID, msg interface {
	MarshalCramberry() ([]byte, error)
}) ([]byte, error) {
	msgData, err := msg.MarshalCramberry()
	if err != nil {
		return nil, err
	}

	w := cramberry.GetWriter()
	defer cramberry.PutWriter(w)

	w.WriteTypeID(typeID)
	w.WriteRawBytes(msgData)

	if w.Err() != nil {
		return nil, w.Err()
	}

	return w.BytesCopy(), nil
}
