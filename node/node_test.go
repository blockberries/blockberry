package node

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
