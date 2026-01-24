package handlers

import (
	"sync"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/blockstore"
	"github.com/blockberries/blockberry/p2p"
	schema "github.com/blockberries/blockberry/schema"
	"github.com/blockberries/blockberry/types"
)

// Block message type ID from schema.
const (
	TypeIDBlockData cramberry.TypeID = 139
)

// BlockValidator is a callback for validating blocks before storing.
type BlockValidator func(height int64, hash, data []byte) error

// BlockReactor handles real-time block propagation between peers.
type BlockReactor struct {
	// Dependencies
	blockStore  blockstore.BlockStore
	network     *p2p.Network
	peerManager *p2p.PeerManager

	// Block validation callback
	validator BlockValidator

	// Callback for when a new block is received
	onBlockReceived func(height int64, hash, data []byte)

	mu sync.RWMutex
}

// NewBlockReactor creates a new block reactor.
func NewBlockReactor(
	blockStore blockstore.BlockStore,
	network *p2p.Network,
	peerManager *p2p.PeerManager,
) *BlockReactor {
	return &BlockReactor{
		blockStore:  blockStore,
		network:     network,
		peerManager: peerManager,
	}
}

// SetValidator sets the block validation callback.
func (r *BlockReactor) SetValidator(v BlockValidator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validator = v
}

// SetOnBlockReceived sets the callback for when a new block is received.
func (r *BlockReactor) SetOnBlockReceived(fn func(height int64, hash, data []byte)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onBlockReceived = fn
}

// HandleMessage processes incoming block messages.
func (r *BlockReactor) HandleMessage(peerID peer.ID, data []byte) error {
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
	case TypeIDBlockData:
		return r.handleBlockData(peerID, remaining)
	default:
		return types.ErrUnknownMessageType
	}
}

// handleBlockData processes an incoming block.
func (r *BlockReactor) handleBlockData(peerID peer.ID, data []byte) error {
	var block schema.BlockData
	if err := block.UnmarshalCramberry(data); err != nil {
		return types.ErrInvalidMessage
	}

	if block.Height == nil || len(block.Hash) == 0 || len(block.Data) == 0 {
		return types.ErrInvalidMessage
	}

	height := *block.Height

	// Check if we already have this block
	if r.blockStore != nil && r.blockStore.HasBlock(height) {
		// Mark as received from peer even though we already have it
		if r.peerManager != nil {
			_ = r.peerManager.MarkBlockReceived(peerID, height)
		}
		return nil
	}

	// Verify hash
	computedHash := types.HashBlock(block.Data)
	if string(computedHash) != string(block.Hash) {
		if r.network != nil {
			_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock, "block hash mismatch")
		}
		return nil
	}

	// Validate block if validator is set
	r.mu.RLock()
	validator := r.validator
	onBlockReceived := r.onBlockReceived
	r.mu.RUnlock()

	if validator != nil {
		if err := validator(height, block.Hash, block.Data); err != nil {
			if r.network != nil {
				_ = r.network.AddPenalty(peerID, p2p.PenaltyInvalidBlock, p2p.ReasonInvalidBlock, err.Error())
			}
			return nil
		}
	}

	// Store block
	if r.blockStore != nil {
		if err := r.blockStore.SaveBlock(height, block.Hash, block.Data); err != nil {
			// Could be duplicate or storage error
			return nil
		}
	}

	// Mark as received from this peer
	if r.peerManager != nil {
		_ = r.peerManager.MarkBlockReceived(peerID, height)
	}

	// Notify callback
	if onBlockReceived != nil {
		onBlockReceived(height, block.Hash, block.Data)
	}

	// Relay to other peers who don't have it
	r.relayBlock(peerID, height, block.Hash, block.Data)

	return nil
}

// relayBlock sends a block to peers who don't have it.
func (r *BlockReactor) relayBlock(fromPeer peer.ID, height int64, hash, blockData []byte) {
	if r.network == nil || r.peerManager == nil {
		return
	}

	peers := r.peerManager.PeersToSendBlock(height)
	if len(peers) == 0 {
		return
	}

	// Encode the block once
	data, err := r.encodeBlockData(height, hash, blockData)
	if err != nil {
		return
	}

	for _, peerID := range peers {
		// Don't send back to the peer we received it from
		if peerID == fromPeer {
			continue
		}

		if err := r.network.Send(peerID, p2p.StreamBlocks, data); err != nil {
			continue
		}
		_ = r.peerManager.MarkBlockSent(peerID, height)
	}
}

// BroadcastBlock broadcasts a new block to all connected peers.
func (r *BlockReactor) BroadcastBlock(height int64, hash, blockData []byte) error {
	if r.network == nil || r.peerManager == nil {
		return nil
	}

	// Get peers who don't have this block
	peers := r.peerManager.PeersToSendBlock(height)
	if len(peers) == 0 {
		return nil
	}

	// Encode the block once
	data, err := r.encodeBlockData(height, hash, blockData)
	if err != nil {
		return err
	}

	for _, peerID := range peers {
		if err := r.network.Send(peerID, p2p.StreamBlocks, data); err != nil {
			continue
		}
		_ = r.peerManager.MarkBlockSent(peerID, height)
	}

	return nil
}

// encodeBlockData encodes a block with its type ID prefix.
func (r *BlockReactor) encodeBlockData(height int64, hash, blockData []byte) ([]byte, error) {
	block := &schema.BlockData{
		Height: &height,
		Hash:   hash,
		Data:   blockData,
	}

	msgData, err := block.MarshalCramberry()
	if err != nil {
		return nil, err
	}

	w := cramberry.GetWriter()
	defer cramberry.PutWriter(w)

	w.WriteTypeID(TypeIDBlockData)
	w.WriteRawBytes(msgData)

	if w.Err() != nil {
		return nil, w.Err()
	}

	return w.BytesCopy(), nil
}

// OnPeerDisconnected is called when a peer disconnects.
// BlockReactor doesn't maintain per-peer state, so this is a no-op.
func (r *BlockReactor) OnPeerDisconnected(peerID peer.ID) {
	// No per-peer state to clean up
}
