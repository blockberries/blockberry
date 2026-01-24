// Package main demonstrates a minimal blockberry node setup.
//
// This example creates a node with default configuration, starts it,
// and waits for a shutdown signal.
//
// Usage:
//
//	go run main.go [config.toml]
//
// If no config file is provided, a default configuration is used.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/node"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "path to config file (optional)")
	flag.Parse()

	// Load or create configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Create node
	n, err := node.NewNode(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating node: %v\n", err)
		os.Exit(1)
	}

	// Start node
	if err := n.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting node: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Node started with peer ID: %s\n", n.PeerID())
	fmt.Printf("Listening on: %v\n", cfg.Network.ListenAddrs)
	fmt.Println("Press Ctrl+C to stop...")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")

	// Stop node
	if err := n.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping node: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Node stopped successfully")
}

// loadConfig loads configuration from file or returns defaults.
func loadConfig(path string) (*config.Config, error) {
	if path != "" {
		return config.LoadConfig(path)
	}

	// Use default configuration with some adjustments for local testing
	cfg := config.DefaultConfig()

	// Ensure data directories exist
	if err := cfg.EnsureDataDirs(); err != nil {
		return nil, fmt.Errorf("creating data directories: %w", err)
	}

	return cfg, nil
}
