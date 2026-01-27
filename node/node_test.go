package node

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

func TestLoadOrGenerateKey_Generate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node_key")

	// Should generate new key
	key, err := loadOrGenerateKey(path)
	require.NoError(t, err)
	require.Len(t, key, ed25519.PrivateKeySize)

	// Key should be saved to file
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte(key), data)
}

func TestLoadOrGenerateKey_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node_key")

	// Generate a key
	_, originalKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	// Save it
	err = os.WriteFile(path, originalKey, 0600)
	require.NoError(t, err)

	// Load should return same key
	key, err := loadOrGenerateKey(path)
	require.NoError(t, err)
	require.Equal(t, originalKey, key)
}

func TestLoadOrGenerateKey_LoadHex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node_key")

	// Generate a key
	_, originalKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	// Save as hex
	hexKey := make([]byte, len(originalKey)*2)
	for i, b := range originalKey {
		hexKey[i*2] = "0123456789abcdef"[b>>4]
		hexKey[i*2+1] = "0123456789abcdef"[b&0x0f]
	}
	err = os.WriteFile(path, hexKey, 0600)
	require.NoError(t, err)

	// Load should return same key
	key, err := loadOrGenerateKey(path)
	require.NoError(t, err)
	require.Equal(t, originalKey, key)
}

func TestLoadOrGenerateKey_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "node_key")

	// Write invalid data
	err := os.WriteFile(path, []byte("invalid"), 0600)
	require.NoError(t, err)

	// Should fail
	_, err = loadOrGenerateKey(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid key file")
}

func TestParseMultiaddrs(t *testing.T) {
	addrs := []string{
		"/ip4/127.0.0.1/tcp/4000",
		"/ip4/0.0.0.0/tcp/4001",
	}

	result, err := parseMultiaddrs(addrs)
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestParseMultiaddrs_Invalid(t *testing.T) {
	addrs := []string{
		"not-a-multiaddr",
	}

	_, err := parseMultiaddrs(addrs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing multiaddr")
}

func TestParseMultiaddrs_Empty(t *testing.T) {
	result, err := parseMultiaddrs(nil)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestOption_WithMempool(t *testing.T) {
	// Test that the option function works correctly
	n := &Node{}
	opt := WithMempool(nil)
	opt(n)
	require.Nil(t, n.mempool)
}

func TestOption_WithBlockStore(t *testing.T) {
	n := &Node{}
	opt := WithBlockStore(nil)
	opt(n)
	require.Nil(t, n.blockStore)
}

func TestNode_StopNotStarted(t *testing.T) {
	n := &Node{
		started: false,
		stopCh:  make(chan struct{}),
	}

	err := n.Stop()
	require.ErrorIs(t, err, types.ErrNodeNotStarted)
}

func TestNode_StoppingFlag(t *testing.T) {
	n := &Node{}

	// Initially stopping should be false
	require.False(t, n.stopping.Load())

	// Setting it should work
	n.stopping.Store(true)
	require.True(t, n.stopping.Load())
}

func TestDefaultShutdownTimeout(t *testing.T) {
	// Verify the default shutdown timeout is reasonable
	require.Equal(t, 5*time.Second, DefaultShutdownTimeout)
}

func TestComponentNameConstants(t *testing.T) {
	// Verify component name constants are defined
	require.Equal(t, "network", ComponentNetwork)
	require.Equal(t, "handshake-handler", ComponentHandshake)
	require.Equal(t, "pex-reactor", ComponentPEX)
	require.Equal(t, "housekeeping-reactor", ComponentHousekeeping)
	require.Equal(t, "transactions-reactor", ComponentTransactions)
	require.Equal(t, "block-reactor", ComponentBlocks)
	require.Equal(t, "consensus-reactor", ComponentConsensus)
	require.Equal(t, "sync-reactor", ComponentSync)
}

func TestComponentNames(t *testing.T) {
	n := &Node{}
	names := n.ComponentNames()

	require.Len(t, names, 8)
	require.Contains(t, names, ComponentNetwork)
	require.Contains(t, names, ComponentHandshake)
	require.Contains(t, names, ComponentPEX)
	require.Contains(t, names, ComponentHousekeeping)
	require.Contains(t, names, ComponentTransactions)
	require.Contains(t, names, ComponentBlocks)
	require.Contains(t, names, ComponentConsensus)
	require.Contains(t, names, ComponentSync)
}

func TestGetComponent_Unknown(t *testing.T) {
	n := &Node{}

	_, err := n.GetComponent("unknown-component")
	require.Error(t, err)
	require.Contains(t, err.Error(), "component not found")
}

func TestCallbacks_Nil(t *testing.T) {
	n := &Node{}

	// Callbacks should be nil by default
	require.Nil(t, n.Callbacks())
}

func TestCallbacks_SetAndGet(t *testing.T) {
	n := &Node{}

	cb := &types.NodeCallbacks{
		OnPeerConnected: func(peerID peer.ID, isOutbound bool) {},
	}

	n.SetCallbacks(cb)
	require.Equal(t, cb, n.Callbacks())
}

func TestCallbacks_SetNil(t *testing.T) {
	n := &Node{}

	cb := &types.NodeCallbacks{}
	n.SetCallbacks(cb)
	require.NotNil(t, n.Callbacks())

	n.SetCallbacks(nil)
	require.Nil(t, n.Callbacks())
}

func TestOption_WithCallbacks(t *testing.T) {
	cb := &types.NodeCallbacks{}
	opt := WithCallbacks(cb)

	n := &Node{}
	opt(n)

	require.Equal(t, cb, n.callbacks)
}
