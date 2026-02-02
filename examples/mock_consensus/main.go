// Package main demonstrates using the consensus interface.
//
// This example shows how to implement a simple round-robin block proposer
// using blockberry's consensus message passing. This is NOT a production
// consensus algorithm - it's meant to demonstrate the integration pattern.
//
// Usage:
//
//	go run main.go
package main

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/blockberries/blockberry/pkg/config"
	"github.com/blockberries/blockberry/internal/handlers"
	"github.com/blockberries/blockberry/pkg/node"
	"github.com/blockberries/blockberry/pkg/types"
)

func main() {
	// Create configuration
	cfg := config.DefaultConfig()
	if err := cfg.EnsureDataDirs(); err != nil {
		fmt.Printf("Error creating data dirs: %v\n", err)
		return
	}

	// Create mock consensus handler
	consensus := NewMockConsensus()

	// Create node with consensus handler.
	// Note: AcceptAllBlockValidator is for demonstration only.
	// Production code should implement proper block validation.
	n, err := node.NewNode(cfg,
		node.WithConsensusHandler(consensus),
		node.WithBlockValidator(types.AcceptAllBlockValidator),
	)
	if err != nil {
		fmt.Printf("Error creating node: %v\n", err)
		return
	}

	// Set the node reference in consensus (for broadcasting)
	consensus.SetNode(n)

	fmt.Printf("Node created with mock consensus\n")
	fmt.Printf("Peer ID: %s\n", n.PeerID())

	// Demonstrate consensus message handling
	demonstrateConsensus(consensus)
}

// demonstrateConsensus shows how consensus messages are handled.
func demonstrateConsensus(c *MockConsensus) {
	fmt.Println("\n--- Mock Consensus Demo ---")

	// Simulate receiving proposals
	testPeer := peer.ID("test-peer-123")

	// Create and handle a proposal message
	proposal := c.createProposal(1, []byte("block data for height 1"))
	fmt.Printf("Created proposal: height=%d\n", 1)

	if err := c.HandleConsensusMessage(testPeer, proposal); err != nil {
		fmt.Printf("Error handling proposal: %v\n", err)
	}

	// Create and handle a vote message
	vote := c.createVote(1, true)
	fmt.Printf("Created vote: height=%d accept=%v\n", 1, true)

	if err := c.HandleConsensusMessage(testPeer, vote); err != nil {
		fmt.Printf("Error handling vote: %v\n", err)
	}

	// Create and handle a commit message
	commit := c.createCommit(1)
	fmt.Printf("Created commit: height=%d\n", 1)

	if err := c.HandleConsensusMessage(testPeer, commit); err != nil {
		fmt.Printf("Error handling commit: %v\n", err)
	}

	// Show current state
	fmt.Printf("\nCurrent consensus height: %d\n", c.Height())
	fmt.Printf("Pending votes: %d\n", c.PendingVotes())
}

// Message types for our mock consensus protocol.
const (
	MsgTypeProposal byte = 0x01
	MsgTypeVote     byte = 0x02
	MsgTypeCommit   byte = 0x03
)

// MockConsensus implements a simple mock consensus protocol.
// It demonstrates how to use blockberry's consensus message passing.
type MockConsensus struct {
	height       int64
	pendingVotes map[int64]int
	node         *node.Node
	mu           sync.RWMutex
}

// NewMockConsensus creates a new mock consensus handler.
func NewMockConsensus() *MockConsensus {
	return &MockConsensus{
		height:       0,
		pendingVotes: make(map[int64]int),
	}
}

// SetNode sets the node reference for broadcasting.
func (c *MockConsensus) SetNode(n *node.Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.node = n
}

// HandleConsensusMessage processes incoming consensus messages.
// This method is called by blockberry when a message arrives on the consensus stream.
func (c *MockConsensus) HandleConsensusMessage(peerID peer.ID, data []byte) error {
	if len(data) < 1 {
		return types.ErrInvalidMessage
	}

	msgType := data[0]
	payload := data[1:]

	switch msgType {
	case MsgTypeProposal:
		return c.handleProposal(peerID, payload)
	case MsgTypeVote:
		return c.handleVote(peerID, payload)
	case MsgTypeCommit:
		return c.handleCommit(peerID, payload)
	default:
		return fmt.Errorf("unknown consensus message type: %d", msgType)
	}
}

// handleProposal processes a block proposal.
func (c *MockConsensus) handleProposal(peerID peer.ID, data []byte) error {
	if len(data) < 8 {
		return types.ErrInvalidMessage
	}

	height := int64(binary.BigEndian.Uint64(data[:8]))
	blockData := data[8:]

	fmt.Printf("[Consensus] Received proposal from %s: height=%d size=%d bytes\n",
		peerID.String()[:8], height, len(blockData))

	// In a real implementation, you would:
	// 1. Validate the proposal
	// 2. Vote on it
	// 3. Broadcast your vote

	return nil
}

// handleVote processes a vote message.
func (c *MockConsensus) handleVote(peerID peer.ID, data []byte) error {
	if len(data) < 9 {
		return types.ErrInvalidMessage
	}

	height := int64(binary.BigEndian.Uint64(data[:8]))
	accept := data[8] == 1

	fmt.Printf("[Consensus] Received vote from %s: height=%d accept=%v\n",
		peerID.String()[:8], height, accept)

	c.mu.Lock()
	if accept {
		c.pendingVotes[height]++
	}
	c.mu.Unlock()

	// In a real implementation, you would:
	// 1. Tally votes
	// 2. Check for quorum
	// 3. Commit if quorum reached

	return nil
}

// handleCommit processes a commit message.
func (c *MockConsensus) handleCommit(peerID peer.ID, data []byte) error {
	if len(data) < 8 {
		return types.ErrInvalidMessage
	}

	height := int64(binary.BigEndian.Uint64(data[:8]))

	fmt.Printf("[Consensus] Received commit from %s: height=%d\n",
		peerID.String()[:8], height)

	c.mu.Lock()
	if height > c.height {
		c.height = height
	}
	c.mu.Unlock()

	return nil
}

// createProposal creates a proposal message.
func (c *MockConsensus) createProposal(height int64, blockData []byte) []byte {
	msg := make([]byte, 1+8+len(blockData))
	msg[0] = MsgTypeProposal
	binary.BigEndian.PutUint64(msg[1:9], uint64(height))
	copy(msg[9:], blockData)
	return msg
}

// createVote creates a vote message.
func (c *MockConsensus) createVote(height int64, accept bool) []byte {
	msg := make([]byte, 1+8+1)
	msg[0] = MsgTypeVote
	binary.BigEndian.PutUint64(msg[1:9], uint64(height))
	if accept {
		msg[9] = 1
	}
	return msg
}

// createCommit creates a commit message.
func (c *MockConsensus) createCommit(height int64) []byte {
	msg := make([]byte, 1+8)
	msg[0] = MsgTypeCommit
	binary.BigEndian.PutUint64(msg[1:9], uint64(height))
	return msg
}

// Height returns the current consensus height.
func (c *MockConsensus) Height() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.height
}

// PendingVotes returns the number of pending votes at all heights.
func (c *MockConsensus) PendingVotes() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := 0
	for _, count := range c.pendingVotes {
		total += count
	}
	return total
}

// ProposeBlock proposes a new block (would be called by a leader).
func (c *MockConsensus) ProposeBlock(blockData []byte) error {
	c.mu.RLock()
	n := c.node
	height := c.height + 1
	c.mu.RUnlock()

	if n == nil {
		return fmt.Errorf("node not set")
	}

	// Create and broadcast proposal
	proposal := c.createProposal(height, blockData)

	// In a real implementation, you would use the consensus reactor to broadcast
	fmt.Printf("[Consensus] Proposing block at height %d\n", height)
	_ = proposal // Would broadcast via n.Network()

	return nil
}

// Verify interface compliance
var _ handlers.ConsensusHandler = (*MockConsensus)(nil)
