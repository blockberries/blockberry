// Package types provides common types used throughout blockberry.
package types

import (
	"github.com/libp2p/go-libp2p/core/peer"
)

// VoteType represents the type of consensus vote.
type VoteType int

// Vote type constants.
const (
	VoteTypePrevote   VoteType = 1
	VoteTypePrecommit VoteType = 2
)

// PeerInfo contains information about a connected peer after handshake.
type PeerInfo struct {
	// NodeID is the peer's hex-encoded public key.
	NodeID string
	// ChainID is the peer's chain ID.
	ChainID string
	// ProtocolVersion is the peer's protocol version.
	ProtocolVersion int32
	// Height is the peer's current block height.
	Height int64
}

// NodeCallbacks defines callbacks for node events.
// All callbacks are optional - nil callbacks are no-ops.
// Callbacks allow components to communicate without tight coupling,
// enabling pluggable consensus engines and custom application logic.
type NodeCallbacks struct {
	// Transaction callbacks

	// OnTxReceived is called when a transaction is received from a peer.
	// The transaction has not yet been validated.
	OnTxReceived func(peerID peer.ID, tx []byte)

	// OnTxValidated is called after a transaction is validated.
	// err is nil if validation succeeded, otherwise contains the error.
	OnTxValidated func(tx []byte, err error)

	// OnTxBroadcast is called when a transaction is broadcast to peers.
	// peers contains the list of peer IDs the transaction was sent to.
	OnTxBroadcast func(tx []byte, peers []peer.ID)

	// OnTxAdded is called when a transaction is successfully added to the mempool.
	OnTxAdded func(txHash []byte, tx []byte)

	// OnTxRemoved is called when a transaction is removed from the mempool.
	// reason indicates why the transaction was removed (e.g., "committed", "expired", "replaced").
	OnTxRemoved func(txHash []byte, reason string)

	// Block callbacks

	// OnBlockReceived is called when a block is received from a peer.
	// The block has not yet been validated.
	OnBlockReceived func(peerID peer.ID, height int64, hash, data []byte)

	// OnBlockValidated is called after a block is validated.
	// err is nil if validation succeeded, otherwise contains the error.
	OnBlockValidated func(height int64, hash []byte, err error)

	// OnBlockCommitted is called when a block is committed to the block store.
	OnBlockCommitted func(height int64, hash []byte)

	// OnBlockStored is called when a block is stored in the block store.
	// This may happen during sync (historical blocks) or when a new block is committed.
	OnBlockStored func(height int64, hash []byte)

	// Peer callbacks

	// OnPeerConnected is called when a new peer connection is established.
	// isOutbound indicates if we initiated the connection.
	// At this point, the handshake has not yet completed.
	OnPeerConnected func(peerID peer.ID, isOutbound bool)

	// OnPeerHandshaked is called after a peer completes the handshake.
	// info contains information about the peer from the handshake.
	OnPeerHandshaked func(peerID peer.ID, info *PeerInfo)

	// OnPeerDisconnected is called when a peer disconnects.
	OnPeerDisconnected func(peerID peer.ID)

	// OnPeerPenalized is called when a peer receives a penalty.
	OnPeerPenalized func(peerID peer.ID, points int64, reason string)

	// Consensus callbacks (set by consensus engine)

	// OnConsensusMessage is called when a consensus message is received.
	// The consensus engine should handle the message appropriately.
	OnConsensusMessage func(peerID peer.ID, data []byte)

	// OnProposalReady is called when the consensus engine needs a block proposal.
	// Returns the proposed block data, or an error if proposal fails.
	OnProposalReady func(height int64) ([]byte, error)

	// OnVoteReady is called when the consensus engine needs to cast a vote.
	// Returns the signed vote data, or an error if voting fails.
	OnVoteReady func(height int64, round int32, voteType VoteType) ([]byte, error)

	// Sync callbacks

	// OnSyncStarted is called when block synchronization begins.
	OnSyncStarted func(startHeight, targetHeight int64)

	// OnSyncProgress is called periodically during synchronization.
	OnSyncProgress func(currentHeight, targetHeight int64)

	// OnSyncCompleted is called when block synchronization completes.
	OnSyncCompleted func(height int64)
}

// DefaultCallbacks returns a NodeCallbacks with no-op implementations.
// This is useful as a base when you only want to override specific callbacks.
func DefaultCallbacks() *NodeCallbacks {
	return &NodeCallbacks{}
}

// Clone creates a copy of the callbacks.
// This is useful for creating modified versions of existing callbacks.
func (c *NodeCallbacks) Clone() *NodeCallbacks {
	if c == nil {
		return nil
	}
	return &NodeCallbacks{
		OnTxReceived:       c.OnTxReceived,
		OnTxValidated:      c.OnTxValidated,
		OnTxBroadcast:      c.OnTxBroadcast,
		OnTxAdded:          c.OnTxAdded,
		OnTxRemoved:        c.OnTxRemoved,
		OnBlockReceived:    c.OnBlockReceived,
		OnBlockValidated:   c.OnBlockValidated,
		OnBlockCommitted:   c.OnBlockCommitted,
		OnBlockStored:      c.OnBlockStored,
		OnPeerConnected:    c.OnPeerConnected,
		OnPeerHandshaked:   c.OnPeerHandshaked,
		OnPeerDisconnected: c.OnPeerDisconnected,
		OnPeerPenalized:    c.OnPeerPenalized,
		OnConsensusMessage: c.OnConsensusMessage,
		OnProposalReady:    c.OnProposalReady,
		OnVoteReady:        c.OnVoteReady,
		OnSyncStarted:      c.OnSyncStarted,
		OnSyncProgress:     c.OnSyncProgress,
		OnSyncCompleted:    c.OnSyncCompleted,
	}
}

// Merge merges another set of callbacks into this one.
// Non-nil callbacks in other will override callbacks in c.
// Returns c for chaining.
func (c *NodeCallbacks) Merge(other *NodeCallbacks) *NodeCallbacks {
	if c == nil || other == nil {
		return c
	}

	if other.OnTxReceived != nil {
		c.OnTxReceived = other.OnTxReceived
	}
	if other.OnTxValidated != nil {
		c.OnTxValidated = other.OnTxValidated
	}
	if other.OnTxBroadcast != nil {
		c.OnTxBroadcast = other.OnTxBroadcast
	}
	if other.OnTxAdded != nil {
		c.OnTxAdded = other.OnTxAdded
	}
	if other.OnTxRemoved != nil {
		c.OnTxRemoved = other.OnTxRemoved
	}
	if other.OnBlockReceived != nil {
		c.OnBlockReceived = other.OnBlockReceived
	}
	if other.OnBlockValidated != nil {
		c.OnBlockValidated = other.OnBlockValidated
	}
	if other.OnBlockCommitted != nil {
		c.OnBlockCommitted = other.OnBlockCommitted
	}
	if other.OnBlockStored != nil {
		c.OnBlockStored = other.OnBlockStored
	}
	if other.OnPeerConnected != nil {
		c.OnPeerConnected = other.OnPeerConnected
	}
	if other.OnPeerHandshaked != nil {
		c.OnPeerHandshaked = other.OnPeerHandshaked
	}
	if other.OnPeerDisconnected != nil {
		c.OnPeerDisconnected = other.OnPeerDisconnected
	}
	if other.OnPeerPenalized != nil {
		c.OnPeerPenalized = other.OnPeerPenalized
	}
	if other.OnConsensusMessage != nil {
		c.OnConsensusMessage = other.OnConsensusMessage
	}
	if other.OnProposalReady != nil {
		c.OnProposalReady = other.OnProposalReady
	}
	if other.OnVoteReady != nil {
		c.OnVoteReady = other.OnVoteReady
	}
	if other.OnSyncStarted != nil {
		c.OnSyncStarted = other.OnSyncStarted
	}
	if other.OnSyncProgress != nil {
		c.OnSyncProgress = other.OnSyncProgress
	}
	if other.OnSyncCompleted != nil {
		c.OnSyncCompleted = other.OnSyncCompleted
	}

	return c
}

// Safe callback invocation helpers.
// These functions check for nil before calling, providing safe invocation.

// InvokeTxReceived safely invokes OnTxReceived if set.
func (c *NodeCallbacks) InvokeTxReceived(peerID peer.ID, tx []byte) {
	if c != nil && c.OnTxReceived != nil {
		c.OnTxReceived(peerID, tx)
	}
}

// InvokeTxValidated safely invokes OnTxValidated if set.
func (c *NodeCallbacks) InvokeTxValidated(tx []byte, err error) {
	if c != nil && c.OnTxValidated != nil {
		c.OnTxValidated(tx, err)
	}
}

// InvokeTxBroadcast safely invokes OnTxBroadcast if set.
func (c *NodeCallbacks) InvokeTxBroadcast(tx []byte, peers []peer.ID) {
	if c != nil && c.OnTxBroadcast != nil {
		c.OnTxBroadcast(tx, peers)
	}
}

// InvokeTxAdded safely invokes OnTxAdded if set.
func (c *NodeCallbacks) InvokeTxAdded(txHash []byte, tx []byte) {
	if c != nil && c.OnTxAdded != nil {
		c.OnTxAdded(txHash, tx)
	}
}

// InvokeTxRemoved safely invokes OnTxRemoved if set.
func (c *NodeCallbacks) InvokeTxRemoved(txHash []byte, reason string) {
	if c != nil && c.OnTxRemoved != nil {
		c.OnTxRemoved(txHash, reason)
	}
}

// InvokeBlockReceived safely invokes OnBlockReceived if set.
func (c *NodeCallbacks) InvokeBlockReceived(peerID peer.ID, height int64, hash, data []byte) {
	if c != nil && c.OnBlockReceived != nil {
		c.OnBlockReceived(peerID, height, hash, data)
	}
}

// InvokeBlockValidated safely invokes OnBlockValidated if set.
func (c *NodeCallbacks) InvokeBlockValidated(height int64, hash []byte, err error) {
	if c != nil && c.OnBlockValidated != nil {
		c.OnBlockValidated(height, hash, err)
	}
}

// InvokeBlockCommitted safely invokes OnBlockCommitted if set.
func (c *NodeCallbacks) InvokeBlockCommitted(height int64, hash []byte) {
	if c != nil && c.OnBlockCommitted != nil {
		c.OnBlockCommitted(height, hash)
	}
}

// InvokeBlockStored safely invokes OnBlockStored if set.
func (c *NodeCallbacks) InvokeBlockStored(height int64, hash []byte) {
	if c != nil && c.OnBlockStored != nil {
		c.OnBlockStored(height, hash)
	}
}

// InvokePeerConnected safely invokes OnPeerConnected if set.
func (c *NodeCallbacks) InvokePeerConnected(peerID peer.ID, isOutbound bool) {
	if c != nil && c.OnPeerConnected != nil {
		c.OnPeerConnected(peerID, isOutbound)
	}
}

// InvokePeerHandshaked safely invokes OnPeerHandshaked if set.
func (c *NodeCallbacks) InvokePeerHandshaked(peerID peer.ID, info *PeerInfo) {
	if c != nil && c.OnPeerHandshaked != nil {
		c.OnPeerHandshaked(peerID, info)
	}
}

// InvokePeerDisconnected safely invokes OnPeerDisconnected if set.
func (c *NodeCallbacks) InvokePeerDisconnected(peerID peer.ID) {
	if c != nil && c.OnPeerDisconnected != nil {
		c.OnPeerDisconnected(peerID)
	}
}

// InvokePeerPenalized safely invokes OnPeerPenalized if set.
func (c *NodeCallbacks) InvokePeerPenalized(peerID peer.ID, points int64, reason string) {
	if c != nil && c.OnPeerPenalized != nil {
		c.OnPeerPenalized(peerID, points, reason)
	}
}

// InvokeConsensusMessage safely invokes OnConsensusMessage if set.
func (c *NodeCallbacks) InvokeConsensusMessage(peerID peer.ID, data []byte) {
	if c != nil && c.OnConsensusMessage != nil {
		c.OnConsensusMessage(peerID, data)
	}
}

// InvokeSyncStarted safely invokes OnSyncStarted if set.
func (c *NodeCallbacks) InvokeSyncStarted(startHeight, targetHeight int64) {
	if c != nil && c.OnSyncStarted != nil {
		c.OnSyncStarted(startHeight, targetHeight)
	}
}

// InvokeSyncProgress safely invokes OnSyncProgress if set.
func (c *NodeCallbacks) InvokeSyncProgress(currentHeight, targetHeight int64) {
	if c != nil && c.OnSyncProgress != nil {
		c.OnSyncProgress(currentHeight, targetHeight)
	}
}

// InvokeSyncCompleted safely invokes OnSyncCompleted if set.
func (c *NodeCallbacks) InvokeSyncCompleted(height int64) {
	if c != nil && c.OnSyncCompleted != nil {
		c.OnSyncCompleted(height)
	}
}
