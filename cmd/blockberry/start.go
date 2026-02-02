package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/blockberries/blockberry/pkg/config"
	"github.com/blockberries/blockberry/pkg/logging"
	"github.com/blockberries/blockberry/pkg/node"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the node",
	Long: `Start the Blockberry node with the specified configuration.

The node will run until interrupted (Ctrl+C) or receives a termination signal.

Example:
  blockberry start --config config.toml`,
	RunE: runStart,
}

func runStart(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Initialize logger based on config
	logger := createLogger(cfg.Logging)

	logger.Info("Starting Blockberry node",
		"chain_id", cfg.Node.ChainID,
		"role", cfg.Role,
		"version", Version,
	)

	// Create node
	n, err := node.NewNode(cfg)
	if err != nil {
		return fmt.Errorf("creating node: %w", err)
	}

	// Start node
	if err := n.Start(); err != nil {
		return fmt.Errorf("starting node: %w", err)
	}

	logger.Info("Node started successfully",
		"listen_addrs", cfg.Network.ListenAddrs,
	)

	// Wait for interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received signal, shutting down", "signal", sig)
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down")
	}

	// Stop node
	if err := n.Stop(); err != nil {
		logger.Error("Error stopping node", "error", err)
		return fmt.Errorf("stopping node: %w", err)
	}

	logger.Info("Node stopped gracefully")
	return nil
}

// createLogger creates a logger based on configuration.
func createLogger(cfg config.LoggingConfig) *logging.Logger {
	// Parse log level
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Determine output
	var w = os.Stderr
	switch strings.ToLower(cfg.Output) {
	case "stdout":
		w = os.Stdout
	case "stderr", "":
		w = os.Stderr
	}

	// Create logger based on format
	switch strings.ToLower(cfg.Format) {
	case "json":
		return logging.NewJSONLogger(w, level)
	default:
		return logging.NewTextLogger(w, level)
	}
}
