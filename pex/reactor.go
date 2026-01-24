package pex

import (
	"fmt"
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

// PEX message type IDs from schema.
const (
	TypeIDAddressRequest  cramberry.TypeID = 131
	TypeIDAddressResponse cramberry.TypeID = 132
)

// Reactor handles peer exchange protocol messages.
type Reactor struct {
	// Configuration
	enabled         bool
	requestInterval time.Duration
	maxAddresses    int

	// Dependencies
	addressBook *AddressBook
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// State
	running bool
	stopCh  chan struct{}
	mu      sync.RWMutex

	// Connection limits
	maxInbound  int
	maxOutbound int
}

// NewReactor creates a new PEX reactor.
func NewReactor(
	enabled bool,
	requestInterval time.Duration,
	maxAddresses int,
	addressBook *AddressBook,
	network *p2p.Network,
	peerManager *p2p.PeerManager,
	maxInbound int,
	maxOutbound int,
) *Reactor {
	return &Reactor{
		enabled:         enabled,
		requestInterval: requestInterval,
		maxAddresses:    maxAddresses,
		addressBook:     addressBook,
		network:         network,
		peerManager:     peerManager,
		maxInbound:      maxInbound,
		maxOutbound:     maxOutbound,
		stopCh:          make(chan struct{}),
	}
}

// Start starts the PEX reactor.
func (r *Reactor) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}

	if !r.enabled {
		return nil
	}

	r.running = true
	r.stopCh = make(chan struct{})

	go r.requestLoop()
	go r.connectionLoop()

	return nil
}

// Stop stops the PEX reactor.
func (r *Reactor) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	r.running = false
	close(r.stopCh)

	return r.addressBook.Save()
}

// HandleMessage processes an incoming PEX message.
func (r *Reactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	rd := cramberry.NewReader(data)
	typeID := rd.ReadTypeID()
	if rd.Err() != nil {
		return fmt.Errorf("reading type ID: %w", rd.Err())
	}

	payload := rd.Remaining()

	switch typeID {
	case TypeIDAddressRequest:
		return r.handleAddressRequest(peerID, payload)
	case TypeIDAddressResponse:
		return r.handleAddressResponse(peerID, payload)
	default:
		return fmt.Errorf("%w: unknown PEX message type %d", types.ErrInvalidMessage, typeID)
	}
}

// handleAddressRequest processes an incoming AddressRequest.
func (r *Reactor) handleAddressRequest(peerID peer.ID, data []byte) error {
	var req schema.AddressRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return fmt.Errorf("decoding AddressRequest: %w", err)
	}

	if err := req.Validate(); err != nil {
		return fmt.Errorf("validating AddressRequest: %w", err)
	}

	peers := r.addressBook.GetPeersForExchange(*req.LastSeen, r.maxAddresses)

	var response schema.AddressResponse
	for _, entry := range peers {
		if entry.NodeID == peerID.String() {
			continue
		}

		multiaddr := entry.Multiaddr
		lastSeen := entry.LastSeen
		latency := entry.Latency
		nodeID := entry.NodeID

		response.Peers = append(response.Peers, schema.AddressInfo{
			Multiaddr: &multiaddr,
			LastSeen:  &lastSeen,
			Latency:   &latency,
			NodeId:    &nodeID,
		})
	}

	return r.sendAddressResponse(peerID, &response)
}

// handleAddressResponse processes an incoming AddressResponse.
func (r *Reactor) handleAddressResponse(peerID peer.ID, data []byte) error {
	var resp schema.AddressResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return fmt.Errorf("decoding AddressResponse: %w", err)
	}

	for _, info := range resp.Peers {
		if info.NodeId == nil || info.Multiaddr == nil {
			continue
		}

		newPeerID, err := peer.Decode(*info.NodeId)
		if err != nil {
			continue
		}

		latency := int32(0)
		if info.Latency != nil {
			latency = *info.Latency
		}

		r.addressBook.AddPeer(newPeerID, *info.Multiaddr, *info.NodeId, latency)
	}

	return nil
}

// sendAddressRequest sends an AddressRequest to a peer.
func (r *Reactor) sendAddressRequest(peerID peer.ID) error {
	lastSeen := time.Now().Add(-24 * time.Hour).Unix()

	req := &schema.AddressRequest{
		LastSeen: &lastSeen,
	}

	data, err := r.encodeMessage(TypeIDAddressRequest, req)
	if err != nil {
		return fmt.Errorf("encoding AddressRequest: %w", err)
	}

	if r.network != nil {
		if err := r.network.Send(peerID, p2p.StreamPEX, data); err != nil {
			return fmt.Errorf("sending AddressRequest: %w", err)
		}
	}

	return nil
}

// sendAddressResponse sends an AddressResponse to a peer.
func (r *Reactor) sendAddressResponse(peerID peer.ID, resp *schema.AddressResponse) error {
	data, err := r.encodeMessage(TypeIDAddressResponse, resp)
	if err != nil {
		return fmt.Errorf("encoding AddressResponse: %w", err)
	}

	if r.network != nil {
		if err := r.network.Send(peerID, p2p.StreamPEX, data); err != nil {
			return fmt.Errorf("sending AddressResponse: %w", err)
		}
	}

	return nil
}

// encodeMessage encodes a PEX message with its type ID prefix.
func (r *Reactor) encodeMessage(typeID cramberry.TypeID, msg interface {
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

// requestLoop periodically sends AddressRequests to connected peers.
func (r *Reactor) requestLoop() {
	ticker := time.NewTicker(r.requestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.sendAddressRequests()
		}
	}
}

// sendAddressRequests sends AddressRequest to all connected peers.
func (r *Reactor) sendAddressRequests() {
	if r.peerManager == nil {
		return
	}

	peers := r.peerManager.GetConnectedPeers()
	for _, peerID := range peers {
		_ = r.sendAddressRequest(peerID)
	}
}

// connectionLoop periodically checks if we need more outbound connections.
func (r *Reactor) connectionLoop() {
	ticker := time.NewTicker(r.requestInterval * 2)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.ensureOutboundConnections()
		}
	}
}

// ensureOutboundConnections attempts to maintain outbound connection count.
func (r *Reactor) ensureOutboundConnections() {
	if r.peerManager == nil || r.network == nil {
		return
	}

	outbound := r.peerManager.OutboundPeerCount()
	needed := r.maxOutbound - outbound
	if needed <= 0 {
		return
	}

	connected := make(map[peer.ID]bool)
	for _, peerID := range r.peerManager.GetConnectedPeers() {
		connected[peerID] = true
	}

	candidates := r.addressBook.GetPeersToConnect(connected, needed)
	for _, entry := range candidates {
		peerID, err := peer.Decode(entry.NodeID)
		if err != nil {
			continue
		}

		r.addressBook.RecordAttempt(peerID)

		if err := r.network.ConnectMultiaddr(entry.Multiaddr); err != nil {
			continue
		}

		r.addressBook.ResetAttempts(peerID)
	}
}

// OnPeerConnected should be called when a new peer connection is established.
func (r *Reactor) OnPeerConnected(peerID peer.ID, multiaddr string, isOutbound bool) {
	r.addressBook.UpdateLastSeen(peerID)
	r.addressBook.ResetAttempts(peerID)

	if !r.addressBook.HasPeer(peerID) {
		r.addressBook.AddPeer(peerID, multiaddr, peerID.String(), 0)
	}
}

// OnPeerDisconnected should be called when a peer disconnects.
func (r *Reactor) OnPeerDisconnected(peerID peer.ID) {
	r.addressBook.UpdateLastSeen(peerID)
}

// GetAddressBook returns the address book.
func (r *Reactor) GetAddressBook() *AddressBook {
	return r.addressBook
}

// IsRunning returns whether the reactor is running.
func (r *Reactor) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// RequestAddresses manually requests addresses from a specific peer.
func (r *Reactor) RequestAddresses(peerID peer.ID) error {
	return r.sendAddressRequest(peerID)
}
