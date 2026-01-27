package handlers

import (
	"sync"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/mempool"
	"github.com/blockberries/blockberry/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

// Transactions message type IDs from schema.
const (
	TypeIDTransactionsRequest     cramberry.TypeID = 133
	TypeIDTransactionsResponse    cramberry.TypeID = 134
	TypeIDTransactionDataRequest  cramberry.TypeID = 135
	TypeIDTransactionDataResponse cramberry.TypeID = 136
)

// DefaultMaxPendingAge is the default maximum age for pending requests.
const DefaultMaxPendingAge = 60 * time.Second

// TransactionsReactor handles transaction gossiping between peers.
// In passive mode (when using a DAG mempool), it only receives transactions
// and routes them to the mempool without active gossip.
type TransactionsReactor struct {
	// Dependencies
	mempool     mempool.Mempool
	dagMempool  mempool.DAGMempool // Non-nil when using a DAG mempool
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// Configuration
	requestInterval time.Duration
	batchSize       int32
	maxPending      int           // Max pending data requests per peer
	maxPendingAge   time.Duration // Max age for pending requests before cleanup
	passiveMode     bool          // True for DAG mempools (no active gossip)

	// State tracking
	pendingRequests map[peer.ID]map[string]time.Time // peerID -> txHash -> requestTime
	mu              sync.RWMutex

	// Lifecycle
	running bool
	stop    chan struct{}
	wg      sync.WaitGroup
}

// NewTransactionsReactor creates a new transactions reactor.
// If the mempool implements DAGMempool, passive mode is automatically enabled.
func NewTransactionsReactor(
	mp mempool.Mempool,
	network *p2p.Network,
	peerManager *p2p.PeerManager,
	requestInterval time.Duration,
	batchSize int32,
) *TransactionsReactor {
	r := &TransactionsReactor{
		mempool:         mp,
		network:         network,
		peerManager:     peerManager,
		requestInterval: requestInterval,
		batchSize:       batchSize,
		maxPending:      100,
		maxPendingAge:   DefaultMaxPendingAge,
		pendingRequests: make(map[peer.ID]map[string]time.Time),
		stop:            make(chan struct{}),
	}

	// Auto-detect passive mode based on mempool type
	if dagMp, ok := mp.(mempool.DAGMempool); ok {
		r.dagMempool = dagMp
		r.passiveMode = true
	}

	return r
}

// Name returns the component name for identification.
func (r *TransactionsReactor) Name() string {
	return "transactions-reactor"
}

// Start begins the transaction gossip loop.
// In passive mode, the gossip loop is not started since the DAG mempool
// handles its own transaction propagation.
func (r *TransactionsReactor) Start() error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return nil
	}
	r.running = true
	r.stop = make(chan struct{})
	r.mu.Unlock()

	// Skip gossip loop in passive mode - DAG mempool handles propagation
	if !r.passiveMode {
		r.wg.Add(1)
		go r.gossipLoop()
	}

	return nil
}

// Stop halts the transaction gossip loop.
func (r *TransactionsReactor) Stop() error {
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
func (r *TransactionsReactor) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// IsPassiveMode returns true if the reactor is in passive mode.
// In passive mode, the reactor only receives transactions and routes them
// to the mempool without active gossip.
func (r *TransactionsReactor) IsPassiveMode() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.passiveMode
}

// SetPassiveMode enables or disables passive mode.
// This should typically be set before Start() is called.
func (r *TransactionsReactor) SetPassiveMode(passive bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.passiveMode = passive
}

// DAGMempool returns the DAG mempool if one is configured, nil otherwise.
func (r *TransactionsReactor) DAGMempool() mempool.DAGMempool {
	return r.dagMempool
}

// gossipLoop periodically requests transactions from peers.
func (r *TransactionsReactor) gossipLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.requestInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.cleanupStaleRequests()
			r.requestTransactionsFromPeers()
		}
	}
}

// requestTransactionsFromPeers sends TransactionsRequest to all connected peers.
func (r *TransactionsReactor) requestTransactionsFromPeers() {
	if r.peerManager == nil || r.network == nil {
		return
	}

	peers := r.peerManager.GetConnectedPeers()
	for _, peerID := range peers {
		if err := r.SendTransactionsRequest(peerID); err != nil {
			// Log error but continue with other peers
			continue
		}
	}
}

// SendTransactionsRequest sends a TransactionsRequest to a peer.
func (r *TransactionsReactor) SendTransactionsRequest(peerID peer.ID) error {
	if r.network == nil {
		return nil
	}

	req := &schema.TransactionsRequest{
		BatchSize: &r.batchSize,
	}

	data, err := r.encodeMessage(TypeIDTransactionsRequest, req)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamTransactions, data)
}

// HandleMessage processes incoming transaction messages.
// In passive mode, only transaction data responses are processed and routed
// to the mempool. Active gossip requests/responses are ignored since the
// DAG mempool handles its own propagation.
func (r *TransactionsReactor) HandleMessage(peerID peer.ID, data []byte) error {
	if len(data) == 0 {
		return types.ErrInvalidMessage
	}

	// Read type ID from first byte(s)
	reader := cramberry.NewReader(data)
	typeID := reader.ReadTypeID()
	if reader.Err() != nil {
		return types.ErrInvalidMessage
	}

	// Get remaining data after type ID
	remaining := reader.Remaining()

	// In passive mode, only process incoming transaction data
	// Skip gossip-related messages (request/response for hashes)
	if r.passiveMode {
		switch typeID {
		case TypeIDTransactionDataResponse:
			return r.handleTransactionDataResponse(peerID, remaining)
		case TypeIDTransactionsRequest, TypeIDTransactionsResponse,
			TypeIDTransactionDataRequest:
			// Ignore gossip messages in passive mode
			return nil
		default:
			return types.ErrUnknownMessageType
		}
	}

	switch typeID {
	case TypeIDTransactionsRequest:
		return r.handleTransactionsRequest(peerID, remaining)
	case TypeIDTransactionsResponse:
		return r.handleTransactionsResponse(peerID, remaining)
	case TypeIDTransactionDataRequest:
		return r.handleTransactionDataRequest(peerID, remaining)
	case TypeIDTransactionDataResponse:
		return r.handleTransactionDataResponse(peerID, remaining)
	default:
		return types.ErrUnknownMessageType
	}
}

// handleTransactionsRequest responds with transaction hashes from the mempool.
// If there are more transactions than batchSize, multiple responses are sent.
func (r *TransactionsReactor) handleTransactionsRequest(peerID peer.ID, data []byte) error {
	var req schema.TransactionsRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	// Validate required field
	if req.BatchSize == nil {
		return types.ErrInvalidMessage
	}

	// Validate and clamp batch size
	batchSize := int(types.ClampBatchSize(*req.BatchSize, types.MaxBatchSize))

	if r.mempool == nil {
		return nil
	}

	// Get all transaction hashes from mempool, filtering out ones peer already has
	allHashes := r.mempool.TxHashes()
	var txHashes []schema.TransactionHash
	for _, hash := range allHashes {
		if r.peerManager != nil && !r.peerManager.ShouldSendTx(peerID, hash) {
			continue
		}
		txHashes = append(txHashes, schema.TransactionHash{Hash: hash})
	}

	if len(txHashes) == 0 {
		return nil
	}

	// Send responses in batches
	for i := 0; i < len(txHashes); i += batchSize {
		end := i + batchSize
		if end > len(txHashes) {
			end = len(txHashes)
		}

		resp := &schema.TransactionsResponse{
			Transactions: txHashes[i:end],
		}
		if err := r.sendTransactionsResponse(peerID, resp); err != nil {
			return err
		}
	}

	return nil
}

// sendTransactionsResponse sends a TransactionsResponse to a peer.
func (r *TransactionsReactor) sendTransactionsResponse(peerID peer.ID, resp *schema.TransactionsResponse) error {
	if r.network == nil {
		return nil
	}

	data, err := r.encodeMessage(TypeIDTransactionsResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamTransactions, data)
}

// handleTransactionsResponse processes received transaction hashes and requests missing data.
func (r *TransactionsReactor) handleTransactionsResponse(peerID peer.ID, data []byte) error {
	var resp schema.TransactionsResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if r.mempool == nil {
		return nil
	}

	// Find transactions we don't have
	missing := make([]schema.TransactionHash, 0, len(resp.Transactions))
	for _, txHash := range resp.Transactions {
		if len(txHash.Hash) == 0 {
			continue
		}

		// Check if we already have this transaction
		if r.mempool.HasTx(txHash.Hash) {
			continue
		}

		// Check if we already have a pending request for this
		if r.hasPendingRequest(peerID, txHash.Hash) {
			continue
		}
		missing = append(missing, txHash)
	}

	if len(missing) == 0 {
		return nil
	}

	// Request data for missing transactions
	return r.sendTransactionDataRequest(peerID, missing)
}

// hasPendingRequest checks if we have a pending data request for a transaction.
func (r *TransactionsReactor) hasPendingRequest(peerID peer.ID, txHash []byte) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if pending, ok := r.pendingRequests[peerID]; ok {
		_, exists := pending[string(txHash)]
		return exists
	}
	return false
}

// sendTransactionDataRequest sends a TransactionDataRequest to a peer.
func (r *TransactionsReactor) sendTransactionDataRequest(peerID peer.ID, txHashes []schema.TransactionHash) error {
	if r.network == nil {
		return nil
	}

	// Track pending requests
	r.mu.Lock()
	if r.pendingRequests[peerID] == nil {
		r.pendingRequests[peerID] = make(map[string]time.Time)
	}
	now := time.Now()
	for _, txHash := range txHashes {
		r.pendingRequests[peerID][string(txHash.Hash)] = now
	}
	r.mu.Unlock()

	req := &schema.TransactionDataRequest{
		Transactions: txHashes,
	}

	data, err := r.encodeMessage(TypeIDTransactionDataRequest, req)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamTransactions, data)
}

// handleTransactionDataRequest responds with full transaction data.
func (r *TransactionsReactor) handleTransactionDataRequest(peerID peer.ID, data []byte) error {
	var req schema.TransactionDataRequest
	if err := req.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if r.mempool == nil {
		return nil
	}

	// Get transaction data from mempool
	txData := make([]schema.TransactionData, 0, len(req.Transactions))
	for _, txHash := range req.Transactions {
		if len(txHash.Hash) == 0 {
			continue
		}

		tx, err := r.mempool.GetTx(txHash.Hash)
		if err != nil {
			// Transaction not found, skip
			continue
		}

		txData = append(txData, schema.TransactionData{
			Hash: txHash.Hash,
			Data: tx,
		})

		// Mark as sent to this peer
		if r.peerManager != nil {
			_ = r.peerManager.MarkTxSent(peerID, txHash.Hash)
		}
	}

	if len(txData) == 0 {
		return nil
	}

	resp := &schema.TransactionDataResponse{
		Transactions: txData,
	}

	return r.sendTransactionDataResponse(peerID, resp)
}

// sendTransactionDataResponse sends a TransactionDataResponse to a peer.
func (r *TransactionsReactor) sendTransactionDataResponse(peerID peer.ID, resp *schema.TransactionDataResponse) error {
	if r.network == nil {
		return nil
	}

	data, err := r.encodeMessage(TypeIDTransactionDataResponse, resp)
	if err != nil {
		return err
	}

	return r.network.Send(peerID, p2p.StreamTransactions, data)
}

// handleTransactionDataResponse processes received transaction data.
func (r *TransactionsReactor) handleTransactionDataResponse(peerID peer.ID, data []byte) error {
	var resp schema.TransactionDataResponse
	if err := resp.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if r.mempool == nil {
		return nil
	}

	for _, txData := range resp.Transactions {
		// Validate transaction data
		if len(txData.Hash) == 0 || len(txData.Data) == 0 {
			continue
		}

		// Validate transaction size
		if err := types.ValidateTransactionSize(len(txData.Data)); err != nil {
			if r.network != nil {
				_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidTx, p2p.ReasonInvalidTx, "transaction too large")
			}
			continue
		}

		// Clear pending request
		r.clearPendingRequest(peerID, txData.Hash)

		// Verify hash matches content using constant-time comparison to prevent timing attacks
		computedHash := types.HashTx(txData.Data)
		if !types.HashEqual(computedHash, txData.Hash) {
			// Hash mismatch - peer sent bad data
			if r.network != nil {
				_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidTx, p2p.ReasonInvalidTx, "transaction hash mismatch")
			}
			continue
		}

		// Add to mempool
		if err := r.mempool.AddTx(txData.Data); err != nil {
			// Could be duplicate or mempool full, not necessarily an error
			continue
		}

		// Mark as received from this peer
		if r.peerManager != nil {
			_ = r.peerManager.MarkTxReceived(peerID, txData.Hash)
		}
	}

	return nil
}

// clearPendingRequest removes a pending request for a transaction.
func (r *TransactionsReactor) clearPendingRequest(peerID peer.ID, txHash []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if pending, ok := r.pendingRequests[peerID]; ok {
		delete(pending, string(txHash))
		if len(pending) == 0 {
			delete(r.pendingRequests, peerID)
		}
	}
}

// cleanupStaleRequests removes pending requests that have exceeded maxPendingAge.
// This prevents unbounded memory growth from requests that never receive responses.
func (r *TransactionsReactor) cleanupStaleRequests() {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	for peerID, pending := range r.pendingRequests {
		for txHash, requestTime := range pending {
			if now.Sub(requestTime) > r.maxPendingAge {
				delete(pending, txHash)
			}
		}
		if len(pending) == 0 {
			delete(r.pendingRequests, peerID)
		}
	}
}

// OnPeerDisconnected cleans up state for a disconnected peer.
func (r *TransactionsReactor) OnPeerDisconnected(peerID peer.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.pendingRequests, peerID)
}

// BroadcastTx broadcasts a new transaction to all peers.
// In passive mode, this is a no-op since the DAG mempool handles its own propagation.
func (r *TransactionsReactor) BroadcastTx(tx []byte) error {
	// In passive mode, DAG mempool handles propagation
	if r.passiveMode {
		return nil
	}

	if r.network == nil || r.peerManager == nil {
		return nil
	}

	txHash := types.HashTx(tx)

	// Get peers who don't have this transaction
	peers := r.peerManager.PeersToSendTx(txHash)
	if len(peers) == 0 {
		return nil
	}

	// Send transaction data directly to peers
	txData := []schema.TransactionData{
		{Hash: txHash, Data: tx},
	}

	resp := &schema.TransactionDataResponse{
		Transactions: txData,
	}

	data, err := r.encodeMessage(TypeIDTransactionDataResponse, resp)
	if err != nil {
		return err
	}

	for _, peerID := range peers {
		if err := r.network.Send(peerID, p2p.StreamTransactions, data); err != nil {
			continue
		}
		_ = r.peerManager.MarkTxSent(peerID, txHash)
	}

	return nil
}

// encodeMessage encodes a message with its type ID prefix.
func (r *TransactionsReactor) encodeMessage(typeID cramberry.TypeID, msg interface {
	MarshalCramberry() ([]byte, error)
}) ([]byte, error) {
	msgData, err := msg.MarshalCramberry()
	if err != nil {
		return nil, err
	}

	// Write type ID prefix + message data
	w := cramberry.GetWriter()
	defer cramberry.PutWriter(w)

	w.WriteTypeID(typeID)
	w.WriteRawBytes(msgData)

	if w.Err() != nil {
		return nil, w.Err()
	}

	return w.BytesCopy(), nil
}
