package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information (set at build time)
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"

	// Global flags
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "blockberry",
	Short: "Blockberry blockchain node",
	Long: `Blockberry is a modular blockchain node framework.

It provides pluggable consensus, mempool, and storage engines
with support for various node roles (validator, full, seed, light).`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime),
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "config.toml", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Blockberry %s\n", Version)
		fmt.Printf("  Git commit: %s\n", GitCommit)
		fmt.Printf("  Built:      %s\n", BuildTime)
	},
}

// loadConfig loads configuration from the config file.
func loadConfig() error {
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", cfgFile)
	}
	return nil
}
