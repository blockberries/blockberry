package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/blockberries/blockberry/pkg/config"
)

var (
	initChainID  string
	initMoniker  string
	initDataDir  string
	initRole     string
	initOverride bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new node",
	Long: `Initialize a new Blockberry node with configuration files and keys.

This command creates:
  - config.toml: Node configuration
  - node_key.json: Node identity key
  - data/: Data directory for blocks and state

Example:
  blockberry init --chain-id mychain --moniker mynode`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initChainID, "chain-id", "blockberry-testnet", "chain ID for the network")
	initCmd.Flags().StringVar(&initMoniker, "moniker", "", "node moniker (human-readable name)")
	initCmd.Flags().StringVar(&initDataDir, "data-dir", ".", "directory for configuration and data")
	initCmd.Flags().StringVar(&initRole, "role", "full", "node role (validator, full, seed, light, archive)")
	initCmd.Flags().BoolVar(&initOverride, "force", false, "override existing configuration")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Determine data directory
	dataDir := initDataDir
	if dataDir == "" {
		dataDir = "."
	}

	// Check if config already exists
	configPath := filepath.Join(dataDir, "config.toml")
	if _, err := os.Stat(configPath); err == nil && !initOverride {
		return fmt.Errorf("config.toml already exists; use --force to override")
	}

	// Generate moniker if not provided
	moniker := initMoniker
	if moniker == "" {
		hostname, err := os.Hostname()
		if err != nil {
			moniker = "blockberry-node"
		} else {
			moniker = hostname
		}
	}

	// Create default config
	cfg := config.DefaultConfig()
	cfg.Node.ChainID = initChainID
	cfg.Role = config.NodeRole(initRole)
	cfg.Node.PrivateKeyPath = filepath.Join(dataDir, "node_key.json")
	cfg.Network.AddressBookPath = filepath.Join(dataDir, "addrbook.json")
	cfg.BlockStore.Path = filepath.Join(dataDir, "data", "blockstore")
	cfg.StateStore.Path = filepath.Join(dataDir, "data", "state")

	// Validate role
	if !cfg.Role.IsValid() {
		return fmt.Errorf("invalid role: %s (must be one of: validator, full, seed, light, archive)", initRole)
	}

	// Create directories
	dirs := []string{
		dataDir,
		filepath.Join(dataDir, "data"),
		filepath.Join(dataDir, "data", "blockstore"),
		filepath.Join(dataDir, "data", "state"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Generate node key if it doesn't exist
	keyPath := cfg.Node.PrivateKeyPath
	if _, err := os.Stat(keyPath); os.IsNotExist(err) || initOverride {
		if err := generateNodeKey(keyPath); err != nil {
			return fmt.Errorf("generating node key: %w", err)
		}
		fmt.Printf("Generated node key: %s\n", keyPath)
	}

	// Write config file
	if err := config.WriteConfigFile(configPath, cfg); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	fmt.Printf("Initialized Blockberry node\n")
	fmt.Printf("  Chain ID:    %s\n", initChainID)
	fmt.Printf("  Moniker:     %s\n", moniker)
	fmt.Printf("  Role:        %s\n", initRole)
	fmt.Printf("  Config:      %s\n", configPath)
	fmt.Printf("  Data dir:    %s\n", filepath.Join(dataDir, "data"))

	return nil
}

// generateNodeKey generates a new Ed25519 keypair and saves it.
func generateNodeKey(path string) error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	// Create key file content
	content := fmt.Sprintf(`{
  "priv_key": "%s",
  "pub_key": "%s"
}
`, hex.EncodeToString(priv), hex.EncodeToString(pub))

	// Write with restricted permissions
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing key file: %w", err)
	}

	return nil
}
