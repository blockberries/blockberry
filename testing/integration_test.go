package testing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

// cleanupNode is a helper that cleans up a node and ignores errors.
// Used in defer statements where we can't handle cleanup errors.
func cleanupNode(node *TestNode) {
	_ = node.Cleanup()
}

// TestTwoNodes_Handshake tests that two nodes can connect and complete handshake.
func TestTwoNodes_Handshake(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes with same chain ID
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	t.Logf("Node1 PeerID: %s", node1.PeerID().String()[:8])
	t.Logf("Node2 PeerID: %s", node2.PeerID().String()[:8])

	// Connect node1 to node2
	require.NoError(t, node1.ConnectTo(node2))

	// Wait for connection to be established
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))
	require.NoError(t, node2.WaitForConnection(node1.PeerID(), 10*time.Second))

	// Verify both nodes see each other as connected
	require.Equal(t, 1, node1.Network.PeerCount())
	require.Equal(t, 1, node2.Network.PeerCount())

	t.Log("Handshake completed successfully")
}

// TestTwoNodes_ChainIDMismatch tests that nodes with different chain IDs don't connect.
func TestTwoNodes_ChainIDMismatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes with different chain IDs
	cfg1 := &TestNodeConfig{
		ChainID:         "chain-1",
		ProtocolVersion: 1,
	}
	cfg2 := &TestNodeConfig{
		ChainID:         "chain-2",
		ProtocolVersion: 1,
	}

	node1, err := NewTestNode(cfg1)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg2)
	require.NoError(t, err)
	defer cleanupNode(node2)

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	// Connect node1 to node2
	require.NoError(t, node1.ConnectTo(node2))

	// Wait a bit for handshake to fail
	time.Sleep(2 * time.Second)

	// Nodes should not be connected (handshake should have failed due to chain ID mismatch)
	require.Equal(t, 0, node1.Network.PeerCount())
	require.Equal(t, 0, node2.Network.PeerCount())

	t.Log("Chain ID mismatch correctly prevented connection")
}

// TestTwoNodes_TransactionGossip tests transaction gossiping between nodes.
func TestTwoNodes_TransactionGossip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	// Connect and wait for handshake
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))

	// Add a transaction to node1's mempool
	tx := []byte("test-transaction-1")
	require.NoError(t, node1.Mempool.AddTx(tx))
	require.True(t, node1.Mempool.HasTx(types.HashTx(tx)))

	// Wait for transaction to be gossiped to node2
	// Transaction gossip requires multiple request/response rounds:
	// 1. Node2 requests tx hashes from node1
	// 2. Node1 responds with hashes
	// 3. Node2 requests tx data for unknown txs
	// 4. Node1 responds with tx data
	txHash := types.HashTx(tx)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if node2.Mempool.HasTx(txHash) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify transaction propagated
	require.True(t, node2.Mempool.HasTx(txHash), "transaction should have propagated to node2")

	t.Log("Transaction gossiping works correctly")
}

// TestTwoNodes_BlockPropagation tests block propagation between nodes.
func TestTwoNodes_BlockPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	// Connect and wait for handshake
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))

	// Create and store a block in node1
	height := int64(1)
	blockData := []byte("block-data-1")
	blockHash := types.HashBlock(blockData)
	require.NoError(t, node1.BlockStore.SaveBlock(height, blockHash, blockData))

	// Broadcast the block from node1
	_ = node1.BlocksReactor.BroadcastBlock(height, blockHash, blockData)

	// Wait for block to propagate to node2
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if node2.BlockStore.HasBlock(height) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify block propagated
	require.True(t, node2.BlockStore.HasBlock(height), "block should have propagated to node2")

	// Verify block data matches
	hash, data, err := node2.BlockStore.LoadBlock(height)
	require.NoError(t, err)
	require.Equal(t, blockHash, types.Hash(hash))
	require.Equal(t, blockData, data)

	t.Log("Block propagation works correctly")
}

// TestTwoNodes_BlockSync tests block synchronization between nodes.
func TestTwoNodes_BlockSync(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	// Add some blocks to node1 before starting
	for i := int64(1); i <= 5; i++ {
		blockData := []byte{byte(i)}
		blockHash := types.HashBlock(blockData)
		require.NoError(t, node1.BlockStore.SaveBlock(i, blockHash, blockData))
	}
	require.Equal(t, int64(5), node1.BlockStore.Height())

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	// Connect and wait for handshake
	require.NoError(t, node2.ConnectTo(node1))
	require.NoError(t, node2.WaitForConnection(node1.PeerID(), 10*time.Second))

	// Update node2's sync reactor with node1's height
	node2.SyncReactor.UpdatePeerHeight(node1.PeerID(), node1.BlockStore.Height())

	// Wait for node2 to sync blocks
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if node2.BlockStore.Height() >= 5 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify blocks synced
	require.Equal(t, int64(5), node2.BlockStore.Height(), "node2 should have synced all blocks")

	// Verify block data matches
	for i := int64(1); i <= 5; i++ {
		hash1, data1, err1 := node1.BlockStore.LoadBlock(i)
		require.NoError(t, err1)
		hash2, data2, err2 := node2.BlockStore.LoadBlock(i)
		require.NoError(t, err2)
		require.Equal(t, hash1, hash2)
		require.Equal(t, data1, data2)
	}

	t.Log("Block sync works correctly")
}

// TestTwoNodes_ConsensusMessages tests consensus message exchange.
func TestTwoNodes_ConsensusMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	// Create mock applications to receive consensus messages
	app1 := NewMockApplication()
	app2 := NewMockApplication()

	// Set consensus handlers
	node1.ConsensusReactor.SetHandler(app1)
	node2.ConsensusReactor.SetHandler(app2)

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	// Connect and wait for handshake
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))

	// Send consensus message from node1 to node2
	consensusMsg := []byte("consensus-vote-1")
	require.NoError(t, node1.ConsensusReactor.SendConsensusMessage(node2.PeerID(), consensusMsg))

	// Wait for message to be received
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if app2.ConsensusMessageCount() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify message was received
	require.Equal(t, 1, app2.ConsensusMessageCount(), "node2 should have received consensus message")

	t.Log("Consensus message exchange works correctly")
}

// TestThreeNodes_MeshNetwork tests that three nodes form a mesh network.
func TestThreeNodes_MeshNetwork(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create three nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	node3, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node3)

	// Start all nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())
	require.NoError(t, node3.Start())

	t.Logf("Node1: %s", node1.PeerID().String()[:8])
	t.Logf("Node2: %s", node2.PeerID().String()[:8])
	t.Logf("Node3: %s", node3.PeerID().String()[:8])

	// Connect nodes: 1 -> 2, 1 -> 3, 2 -> 3
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.ConnectTo(node3))
	require.NoError(t, node2.ConnectTo(node3))

	// Wait for all connections to be established
	require.NoError(t, node1.WaitForPeerCount(2, 15*time.Second))
	require.NoError(t, node2.WaitForPeerCount(2, 15*time.Second))
	require.NoError(t, node3.WaitForPeerCount(2, 15*time.Second))

	// Verify each node has 2 peers
	require.Equal(t, 2, node1.Network.PeerCount())
	require.Equal(t, 2, node2.Network.PeerCount())
	require.Equal(t, 2, node3.Network.PeerCount())

	t.Log("Three-node mesh network formed correctly")
}

// TestThreeNodes_TransactionPropagation tests transaction propagation across three nodes.
func TestThreeNodes_TransactionPropagation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create three nodes in a chain: node1 -> node2 -> node3
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node2)

	node3, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node3)

	// Start all nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())
	require.NoError(t, node3.Start())

	// Connect in chain: node1 -> node2 -> node3
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))

	require.NoError(t, node2.ConnectTo(node3))
	require.NoError(t, node2.WaitForConnection(node3.PeerID(), 10*time.Second))

	// Add transaction to node1
	tx := []byte("propagated-transaction")
	require.NoError(t, node1.Mempool.AddTx(tx))
	txHash := types.HashTx(tx)

	// Wait for transaction to propagate to node3 (through node2)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if node3.Mempool.HasTx(txHash) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify transaction reached all nodes
	require.True(t, node2.Mempool.HasTx(txHash), "transaction should have reached node2")
	require.True(t, node3.Mempool.HasTx(txHash), "transaction should have propagated to node3")

	t.Log("Transaction propagation across network works correctly")
}

// TestNode_Disconnect tests node disconnection handling.
func TestNode_Disconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)

	// Start both nodes
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())

	// Connect and wait for handshake
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))

	// Verify connected
	require.Equal(t, 1, node1.Network.PeerCount())
	require.Equal(t, 1, node2.Network.PeerCount())

	// Stop node2 - this will clean it up completely
	require.NoError(t, node2.Stop())
	require.NoError(t, node2.Cleanup())

	// Disconnect explicitly from node1's side
	_ = node1.Network.Disconnect(node2.PeerID())

	// Wait a bit for disconnect to propagate
	time.Sleep(500 * time.Millisecond)

	t.Log("Disconnect handling tested")
}

// TestNode_Reconnect tests that a new node can connect after another disconnects.
func TestNode_Reconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create first pair of nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	require.NoError(t, err)

	// Start both nodes and connect
	require.NoError(t, node1.Start())
	require.NoError(t, node2.Start())
	require.NoError(t, node1.ConnectTo(node2))
	require.NoError(t, node1.WaitForConnection(node2.PeerID(), 10*time.Second))

	// Stop node2 completely
	require.NoError(t, node2.Stop())
	require.NoError(t, node2.Cleanup())

	// Create a new node3 and connect to node1
	node3, err := NewTestNode(cfg)
	require.NoError(t, err)
	defer cleanupNode(node3)

	require.NoError(t, node3.Start())
	require.NoError(t, node1.ConnectTo(node3))
	require.NoError(t, node1.WaitForConnection(node3.PeerID(), 10*time.Second))

	// Verify node1 can connect to new nodes
	require.True(t, node1.Network.PeerCount() >= 1)

	t.Log("Connecting to new node after disconnect works")
}

// BenchmarkTransactionGossip benchmarks transaction gossiping throughput.
func BenchmarkTransactionGossip(b *testing.B) {
	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanupNode(node2)

	// Start and connect
	if err := node1.Start(); err != nil {
		b.Fatal(err)
	}
	if err := node2.Start(); err != nil {
		b.Fatal(err)
	}
	if err := node1.ConnectTo(node2); err != nil {
		b.Fatal(err)
	}
	if err := node1.WaitForConnection(node2.PeerID(), 10*time.Second); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tx := []byte{byte(i), byte(i >> 8), byte(i >> 16)}
		if err := node1.Mempool.AddTx(tx); err != nil {
			b.Fatal(err)
		}
	}

	// Wait for propagation
	deadline := time.Now().Add(30 * time.Second)
	for node2.Mempool.Size() < b.N && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	b.StopTimer()
}

// BenchmarkBlockPropagation benchmarks block propagation latency.
func BenchmarkBlockPropagation(b *testing.B) {
	// Create two nodes
	cfg := DefaultTestNodeConfig()

	node1, err := NewTestNode(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanupNode(node1)

	node2, err := NewTestNode(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer cleanupNode(node2)

	// Start and connect
	if err := node1.Start(); err != nil {
		b.Fatal(err)
	}
	if err := node2.Start(); err != nil {
		b.Fatal(err)
	}
	if err := node1.ConnectTo(node2); err != nil {
		b.Fatal(err)
	}
	if err := node1.WaitForConnection(node2.PeerID(), 10*time.Second); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		height := int64(i + 1)
		blockData := make([]byte, 1024) // 1KB block
		blockData[0] = byte(i)
		blockHash := types.HashBlock(blockData)

		if err := node1.BlockStore.SaveBlock(height, blockHash, blockData); err != nil {
			b.Fatal(err)
		}
		_ = node1.BlocksReactor.BroadcastBlock(height, blockHash, blockData)

		// Wait for propagation
		deadline := time.Now().Add(5 * time.Second)
		for !node2.BlockStore.HasBlock(height) && time.Now().Before(deadline) {
			time.Sleep(1 * time.Millisecond)
		}
	}

	b.StopTimer()
}
