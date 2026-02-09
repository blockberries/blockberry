package node

import (
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/config"
	"github.com/blockberries/blockberry/pkg/consensus"
	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/blockberry/pkg/types"
)

func testConfig(t *testing.T) *config.Config {
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.Node.PrivateKeyPath = filepath.Join(tmpDir, "node.key")
	cfg.Network.AddressBookPath = filepath.Join(tmpDir, "addrbook.json")
	cfg.BlockStore.Path = filepath.Join(tmpDir, "blockstore")
	cfg.StateStore.Path = filepath.Join(tmpDir, "state")

	return cfg
}

func TestNodeBuilder_Basic(t *testing.T) {
	cfg := testConfig(t)

	node, err := NewNodeBuilder(cfg).Build()
	require.NoError(t, err)
	require.NotNil(t, node)

	// Verify node has all components
	require.NotNil(t, node.network)
	require.NotNil(t, node.blockStore)
	require.NotNil(t, node.mempool)
	require.NotNil(t, node.handshakeHandler)
	require.NotNil(t, node.transactionsReactor)
	require.NotNil(t, node.blocksReactor)
	require.NotNil(t, node.consensusReactor)
	require.NotNil(t, node.housekeepingReactor)
	require.NotNil(t, node.pexReactor)
	require.NotNil(t, node.syncReactor)
}

func TestNodeBuilder_WithMempool(t *testing.T) {
	cfg := testConfig(t)

	customMempool := mempool.NewMempool(cfg.Mempool)

	node, err := NewNodeBuilder(cfg).
		WithMempool(customMempool).
		Build()

	require.NoError(t, err)
	require.Equal(t, customMempool, node.mempool)
}

func TestNodeBuilder_WithCallbacks(t *testing.T) {
	cfg := testConfig(t)

	called := false
	callbacks := &types.NodeCallbacks{
		OnPeerConnected: func(peerID peer.ID, isOutbound bool) {
			called = true
		},
	}

	node, err := NewNodeBuilder(cfg).
		WithCallbacks(callbacks).
		Build()

	require.NoError(t, err)
	require.Equal(t, callbacks, node.callbacks)

	// Verify callback can be invoked
	node.callbacks.InvokePeerConnected("test-peer", true)
	require.True(t, called)
}

func TestNodeBuilder_WithBlockValidator(t *testing.T) {
	cfg := testConfig(t)

	validatorCalled := false
	validator := func(height int64, hash, data []byte) error {
		validatorCalled = true
		return nil
	}

	node, err := NewNodeBuilder(cfg).
		WithBlockValidator(validator).
		Build()

	require.NoError(t, err)
	require.NotNil(t, node.syncReactor)

	// The validator should be set on the sync reactor
	// We can't directly check it, but we verified it compiles and builds
	_ = validatorCalled
}

// mockConsensusHandler implements ConsensusHandler for testing.
type mockConsensusHandler struct {
	called bool
}

func (h *mockConsensusHandler) HandleConsensusMessage(peerID peer.ID, data []byte) error {
	h.called = true
	return nil
}

func TestNodeBuilder_WithConsensusHandler(t *testing.T) {
	cfg := testConfig(t)

	handler := &mockConsensusHandler{}

	node, err := NewNodeBuilder(cfg).
		WithConsensusHandler(handler).
		Build()

	require.NoError(t, err)
	require.NotNil(t, node.consensusReactor)
}

func TestNodeBuilder_InvalidConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.ChainID = "" // Invalid

	_, err := NewNodeBuilder(cfg).Build()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid configuration")
}

func TestNodeBuilder_Chaining(t *testing.T) {
	cfg := testConfig(t)

	callbacks := &types.NodeCallbacks{}
	validator := types.BlockValidator(func(height int64, hash, data []byte) error {
		return nil
	})

	node, err := NewNodeBuilder(cfg).
		WithBlockValidator(validator).
		WithCallbacks(callbacks).
		Build()

	require.NoError(t, err)
	require.Equal(t, callbacks, node.callbacks)
}

func TestNodeBuilder_MustBuild_Panics(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.ChainID = "" // Invalid

	require.Panics(t, func() {
		NewNodeBuilder(cfg).MustBuild()
	})
}

func TestNodeBuilder_MustBuild_Success(t *testing.T) {
	cfg := testConfig(t)

	require.NotPanics(t, func() {
		node := NewNodeBuilder(cfg).MustBuild()
		require.NotNil(t, node)
	})
}

func TestNewNodeWithBuilder(t *testing.T) {
	cfg := testConfig(t)

	// NewNodeWithBuilder is an alias for NewNodeBuilder
	builder := NewNodeWithBuilder(cfg)
	require.NotNil(t, builder)

	node, err := builder.Build()
	require.NoError(t, err)
	require.NotNil(t, node)
}

// Verify WithConsensusHandler actually sets the handler
func TestNodeBuilder_ConsensusHandlerIsSet(t *testing.T) {
	cfg := testConfig(t)

	handler := &mockConsensusHandler{}

	node, err := NewNodeBuilder(cfg).
		WithConsensusHandler(handler).
		Build()

	require.NoError(t, err)

	// The handler should be retrievable from the reactor
	actualHandler := node.consensusReactor.GetHandler()
	require.Equal(t, handler, actualHandler)
}

// Test that early error in chain prevents further operations
func TestNodeBuilder_ErrorPropagation(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Node.ChainID = "" // This will cause Build() to fail

	// Chain multiple operations - none should panic even with invalid config
	builder := NewNodeBuilder(cfg).
		WithCallbacks(&types.NodeCallbacks{}).
		WithBlockValidator(func(height int64, hash, data []byte) error { return nil })

	_, err := builder.Build()
	require.Error(t, err)
}

// Verify the builder can accept a custom consensus handler interface
func TestNodeBuilder_ConsensusHandlerInterface(t *testing.T) {
	cfg := testConfig(t)

	// Verify the handler type matches the interface
	var _ consensus.ConsensusHandler = &mockConsensusHandler{}

	node, err := NewNodeBuilder(cfg).
		WithConsensusHandler(&mockConsensusHandler{}).
		Build()

	require.NoError(t, err)
	require.NotNil(t, node)
}
