package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

var (
	statusRPCAddr string
	statusJSON    bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Query the node status",
	Long: `Query the status of a running Blockberry node via JSON-RPC.

Example:
  blockberry status
  blockberry status --rpc http://localhost:26657`,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().StringVar(&statusRPCAddr, "rpc", "http://localhost:26657", "JSON-RPC server address")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "output as JSON")
}

// StatusResponse represents the node status response.
type StatusResponse struct {
	NodeInfo struct {
		ChainID         string `json:"chain_id"`
		ProtocolVersion int32  `json:"protocol_version"`
		Role            string `json:"role"`
		Version         string `json:"version"`
	} `json:"node_info"`
	SyncInfo struct {
		LatestBlockHeight uint64    `json:"latest_block_height"`
		LatestBlockHash   string    `json:"latest_block_hash"`
		LatestBlockTime   time.Time `json:"latest_block_time"`
		CatchingUp        bool      `json:"catching_up"`
	} `json:"sync_info"`
	ValidatorInfo struct {
		Address     string `json:"address"`
		VotingPower int64  `json:"voting_power"`
	} `json:"validator_info"`
	Peers struct {
		Inbound  int `json:"inbound"`
		Outbound int `json:"outbound"`
	} `json:"peers"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Create JSON-RPC request
	reqBody := `{"jsonrpc":"2.0","id":1,"method":"status","params":[]}`

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post(statusRPCAddr, "application/json", nil)
	if err != nil {
		// Try a simple health check
		healthResp, healthErr := client.Get(statusRPCAddr + "/health")
		if healthErr != nil {
			return fmt.Errorf("cannot connect to node at %s: %w", statusRPCAddr, err)
		}
		defer healthResp.Body.Close()

		if healthResp.StatusCode != http.StatusOK {
			return fmt.Errorf("node at %s is not healthy (status: %d)", statusRPCAddr, healthResp.StatusCode)
		}

		fmt.Println("Node is running but status endpoint is not available")
		return nil
	}
	defer resp.Body.Close()

	_ = reqBody // Will use this when RPC is implemented

	// For now, just check if the server is responding
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("node returned status %d", resp.StatusCode)
	}

	// Parse response
	var rpcResp struct {
		Result StatusResponse `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		// Server is running but may not have status endpoint
		fmt.Println("Node is running (RPC available)")
		return nil
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	status := rpcResp.Result

	if statusJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	// Pretty print status
	fmt.Println("Node Status")
	fmt.Println("===========")
	fmt.Printf("Chain ID:        %s\n", status.NodeInfo.ChainID)
	fmt.Printf("Protocol:        %d\n", status.NodeInfo.ProtocolVersion)
	fmt.Printf("Role:            %s\n", status.NodeInfo.Role)
	fmt.Printf("Version:         %s\n", status.NodeInfo.Version)
	fmt.Println()
	fmt.Println("Sync Info")
	fmt.Println("---------")
	fmt.Printf("Latest Height:   %d\n", status.SyncInfo.LatestBlockHeight)
	fmt.Printf("Latest Hash:     %s\n", status.SyncInfo.LatestBlockHash)
	fmt.Printf("Latest Time:     %s\n", status.SyncInfo.LatestBlockTime.Format(time.RFC3339))
	fmt.Printf("Catching Up:     %v\n", status.SyncInfo.CatchingUp)
	fmt.Println()
	fmt.Println("Peers")
	fmt.Println("-----")
	fmt.Printf("Inbound:         %d\n", status.Peers.Inbound)
	fmt.Printf("Outbound:        %d\n", status.Peers.Outbound)

	if status.ValidatorInfo.VotingPower > 0 {
		fmt.Println()
		fmt.Println("Validator Info")
		fmt.Println("--------------")
		fmt.Printf("Address:         %s\n", status.ValidatorInfo.Address)
		fmt.Printf("Voting Power:    %d\n", status.ValidatorInfo.VotingPower)
	}

	return nil
}
