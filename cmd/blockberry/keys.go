package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage node keys",
	Long:  `Commands for managing node identity keys.`,
}

var keysGenerateCmd = &cobra.Command{
	Use:   "generate [output-file]",
	Short: "Generate a new node key",
	Long: `Generate a new Ed25519 keypair for node identity.

If no output file is specified, the key is printed to stdout.

Example:
  blockberry keys generate
  blockberry keys generate node_key.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runKeysGenerate,
}

var keysShowCmd = &cobra.Command{
	Use:   "show <key-file>",
	Short: "Show public key from a key file",
	Long: `Display the public key and node ID from a key file.

Example:
  blockberry keys show node_key.json`,
	Args: cobra.ExactArgs(1),
	RunE: runKeysShow,
}

func init() {
	keysCmd.AddCommand(keysGenerateCmd)
	keysCmd.AddCommand(keysShowCmd)
	rootCmd.AddCommand(keysCmd)
}

// NodeKey represents a node's keypair.
type NodeKey struct {
	PrivKey string `json:"priv_key"`
	PubKey  string `json:"pub_key"`
}

func runKeysGenerate(cmd *cobra.Command, args []string) error {
	// Generate keypair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	key := NodeKey{
		PrivKey: hex.EncodeToString(priv),
		PubKey:  hex.EncodeToString(pub),
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(key, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling key: %w", err)
	}

	// Output
	if len(args) == 0 {
		// Print to stdout
		fmt.Println(string(data))
		fmt.Fprintf(cmd.ErrOrStderr(), "\nNode ID: %s\n", hex.EncodeToString(pub))
	} else {
		// Write to file
		outputPath := args[0]
		if err := os.WriteFile(outputPath, data, 0600); err != nil {
			return fmt.Errorf("writing key file: %w", err)
		}
		fmt.Printf("Generated key: %s\n", outputPath)
		fmt.Printf("Node ID: %s\n", hex.EncodeToString(pub))
	}

	return nil
}

func runKeysShow(cmd *cobra.Command, args []string) error {
	keyPath := args[0]

	// Read key file
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("reading key file: %w", err)
	}

	var key NodeKey
	if err := json.Unmarshal(data, &key); err != nil {
		return fmt.Errorf("parsing key file: %w", err)
	}

	// Decode public key
	pubBytes, err := hex.DecodeString(key.PubKey)
	if err != nil {
		return fmt.Errorf("decoding public key: %w", err)
	}

	fmt.Printf("Public Key: %s\n", key.PubKey)
	fmt.Printf("Node ID:    %s\n", hex.EncodeToString(pubBytes))

	return nil
}
