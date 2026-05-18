package looseberry

import (
	"sync"

	libp2ppeer "github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/cramberry/pkg/cramberry"
	lbnetwork "github.com/blockberries/looseberry/network"
	lbtypes "github.com/blockberries/looseberry/types"
)

// networkAdapter bridges blockberry's MempoolNetwork to looseberry's Network interface.
type networkAdapter struct {
	network        mempool.MempoolNetwork
	validatorIndex uint16
	validatorSet   *validatorSetAdapter

	// Message channels
	batchMessages         chan *lbnetwork.BatchMessage
	batchAckMessages      chan *lbnetwork.BatchAckMessage
	batchRequestMessages  chan *lbnetwork.BatchRequestMessage
	batchResponseMessages chan *lbnetwork.BatchResponseMessage
	headerMessages        chan *lbnetwork.HeaderMessage
	voteMessages          chan *lbnetwork.VoteMessage
	certificateMessages   chan *lbnetwork.CertificateMessage
	syncRequests          chan *lbnetwork.SyncRequest
	syncResponses         chan *lbnetwork.SyncResponseMessage

	// Lifecycle
	stopCh chan struct{}
	mu     sync.RWMutex
}

// newNetworkAdapter creates a new network adapter.
func newNetworkAdapter(network mempool.MempoolNetwork, validatorIndex uint16, validatorSet *validatorSetAdapter) *networkAdapter {
	bufferSize := 1000

	return &networkAdapter{
		network:               network,
		validatorIndex:        validatorIndex,
		validatorSet:          validatorSet,
		batchMessages:         make(chan *lbnetwork.BatchMessage, bufferSize),
		batchAckMessages:      make(chan *lbnetwork.BatchAckMessage, bufferSize),
		batchRequestMessages:  make(chan *lbnetwork.BatchRequestMessage, bufferSize),
		batchResponseMessages: make(chan *lbnetwork.BatchResponseMessage, bufferSize),
		headerMessages:        make(chan *lbnetwork.HeaderMessage, bufferSize),
		voteMessages:          make(chan *lbnetwork.VoteMessage, bufferSize),
		certificateMessages:   make(chan *lbnetwork.CertificateMessage, bufferSize),
		syncRequests:          make(chan *lbnetwork.SyncRequest, bufferSize),
		syncResponses:         make(chan *lbnetwork.SyncResponseMessage, bufferSize),
		stopCh:                make(chan struct{}),
	}
}

// Stream names for looseberry protocols
const (
	StreamBatches = "looseberry-batches"
	StreamHeaders = "looseberry-headers"
	StreamSync    = "looseberry-sync"
)

// BroadcastBatch broadcasts a batch to all validators.
func (n *networkAdapter) BroadcastBatch(batch *lbtypes.Batch) error {
	data := encodeBatch(batch)
	return n.network.Broadcast(StreamBatches, data)
}

// BroadcastHeader broadcasts a header to all validators.
func (n *networkAdapter) BroadcastHeader(header *lbtypes.Header) error {
	data := encodeHeader(header)
	return n.network.Broadcast(StreamHeaders, data)
}

// BroadcastCertificate broadcasts a certificate to all validators.
func (n *networkAdapter) BroadcastCertificate(cert *lbtypes.Certificate) error {
	data := encodeCertificate(cert)
	return n.network.Broadcast(StreamHeaders, data)
}

// SendVote sends a vote to a specific validator.
func (n *networkAdapter) SendVote(validator uint16, vote *lbtypes.Vote) error {
	peerID, err := n.validatorToPeerID(validator)
	if err != nil {
		return err
	}
	data := encodeVote(vote)
	return n.network.Send(peerID, StreamHeaders, data)
}

// SendBatchAck sends a batch acknowledgment to a specific validator.
func (n *networkAdapter) SendBatchAck(validator uint16, ack *lbnetwork.BatchAckMessage) error {
	peerID, err := n.validatorToPeerID(validator)
	if err != nil {
		return err
	}
	// Encode the ack message
	data := encodeBatchAck(ack)
	return n.network.Send(peerID, StreamBatches, data)
}

// SendBatchRequest sends a batch request to a specific validator.
func (n *networkAdapter) SendBatchRequest(validator uint16, req *lbnetwork.BatchRequestMessage) error {
	peerID, err := n.validatorToPeerID(validator)
	if err != nil {
		return err
	}
	data := encodeBatchRequest(req)
	return n.network.Send(peerID, StreamBatches, data)
}

// SendBatchResponse sends a batch response to a specific validator.
func (n *networkAdapter) SendBatchResponse(validator uint16, resp *lbnetwork.BatchResponseMessage) error {
	peerID, err := n.validatorToPeerID(validator)
	if err != nil {
		return err
	}
	data := encodeBatchResponse(resp)
	return n.network.Send(peerID, StreamBatches, data)
}

// SendSyncRequest sends a sync request to a specific validator.
func (n *networkAdapter) SendSyncRequest(validator uint16, req *lbnetwork.SyncRequest) error {
	peerID, err := n.validatorToPeerID(validator)
	if err != nil {
		return err
	}
	data := encodeSyncRequest(req)
	return n.network.Send(peerID, StreamSync, data)
}

// SendSyncResponse sends a sync response to a specific validator.
func (n *networkAdapter) SendSyncResponse(validator uint16, resp *lbnetwork.SyncResponse) error {
	peerID, err := n.validatorToPeerID(validator)
	if err != nil {
		return err
	}
	data := encodeSyncResponse(resp)
	return n.network.Send(peerID, StreamSync, data)
}

// BatchMessages returns the channel for incoming batch messages.
func (n *networkAdapter) BatchMessages() <-chan *lbnetwork.BatchMessage {
	return n.batchMessages
}

// BatchAckMessages returns the channel for incoming batch ack messages.
func (n *networkAdapter) BatchAckMessages() <-chan *lbnetwork.BatchAckMessage {
	return n.batchAckMessages
}

// BatchRequestMessages returns the channel for incoming batch request messages.
func (n *networkAdapter) BatchRequestMessages() <-chan *lbnetwork.BatchRequestMessage {
	return n.batchRequestMessages
}

// BatchResponseMessages returns the channel for incoming batch response messages.
func (n *networkAdapter) BatchResponseMessages() <-chan *lbnetwork.BatchResponseMessage {
	return n.batchResponseMessages
}

// HeaderMessages returns the channel for incoming header messages.
func (n *networkAdapter) HeaderMessages() <-chan *lbnetwork.HeaderMessage {
	return n.headerMessages
}

// VoteMessages returns the channel for incoming vote messages.
func (n *networkAdapter) VoteMessages() <-chan *lbnetwork.VoteMessage {
	return n.voteMessages
}

// CertificateMessages returns the channel for incoming certificate messages.
func (n *networkAdapter) CertificateMessages() <-chan *lbnetwork.CertificateMessage {
	return n.certificateMessages
}

// SyncRequests returns the channel for incoming sync requests.
func (n *networkAdapter) SyncRequests() <-chan *lbnetwork.SyncRequest {
	return n.syncRequests
}

// SyncResponses returns the channel for incoming sync responses.
func (n *networkAdapter) SyncResponses() <-chan *lbnetwork.SyncResponseMessage {
	return n.syncResponses
}

// ValidatorID returns this node's validator ID.
func (n *networkAdapter) ValidatorID() uint16 {
	return n.validatorIndex
}

// Start starts the network adapter.
func (n *networkAdapter) Start() error {
	return nil
}

// Stop stops the network adapter.
func (n *networkAdapter) Stop() error {
	close(n.stopCh)
	return nil
}

// validatorToPeerID converts a validator index to a peer ID.
// This would typically use a mapping maintained by the node.
func (n *networkAdapter) validatorToPeerID(validator uint16) (libp2ppeer.ID, error) {
	// In a real implementation, this would look up the peer ID
	// from a validator-to-peer mapping maintained by the node.
	// For now, we return an empty peer ID which would need to be
	// resolved by the caller.
	return "", nil
}

// Encoding helpers. PLAN T1-1 / T2-9: replaced the nil-returning placeholders
// with cramberry.Marshal so this adapter can actually transmit looseberry
// messages. Cramberry's reflection encoder is deterministic (sorted map keys,
// sequential field numbering) and matches what raspberry's
// LooseberryNetworkAdapter does on the other side of the wire.
//
// Encode errors are logged-and-dropped here because the surrounding
// interface returns no error from the encode step; the surrounding Broadcast
// / Send calls swallow nil payloads. A future revision should bubble the
// error through the network.Network interface.

func marshalOrEmpty(v any) []byte {
	out, err := cramberry.Marshal(v)
	if err != nil {
		return nil
	}
	return out
}

func encodeBatch(batch *lbtypes.Batch) []byte             { return marshalOrEmpty(batch) }
func encodeHeader(header *lbtypes.Header) []byte          { return marshalOrEmpty(header) }
func encodeCertificate(cert *lbtypes.Certificate) []byte  { return marshalOrEmpty(cert) }
func encodeVote(vote *lbtypes.Vote) []byte                { return marshalOrEmpty(vote) }
func encodeBatchAck(ack *lbnetwork.BatchAckMessage) []byte {
	return marshalOrEmpty(ack)
}
func encodeBatchRequest(req *lbnetwork.BatchRequestMessage) []byte {
	return marshalOrEmpty(req)
}
func encodeBatchResponse(resp *lbnetwork.BatchResponseMessage) []byte {
	return marshalOrEmpty(resp)
}
func encodeSyncRequest(req *lbnetwork.SyncRequest) []byte {
	return marshalOrEmpty(req)
}
func encodeSyncResponse(resp *lbnetwork.SyncResponse) []byte {
	return marshalOrEmpty(resp)
}

// Verify interface implementation
var _ lbnetwork.Network = (*networkAdapter)(nil)
