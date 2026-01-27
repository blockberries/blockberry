package jsonrpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/blockberries/blockberry/abi"
	"github.com/blockberries/blockberry/rpc"
)

// mockRPCServer implements rpc.Server for testing.
type mockRPCServer struct {
	healthResult     *rpc.HealthStatus
	statusResult     *rpc.NodeStatus
	netInfoResult    *rpc.NetInfo
	broadcastResult  *rpc.BroadcastResult
	queryResult      *abi.QueryResponse
	blockResult      *rpc.BlockResult
	txResult         *rpc.TxResult
	txSearchResults  []*rpc.TxResult
	peersResult      []rpc.PeerInfo
	consensusResult  *rpc.ConsensusState
	subscribeChannel chan abi.Event
	err              error
}

func (m *mockRPCServer) Start() error    { return nil }
func (m *mockRPCServer) Stop() error     { return nil }
func (m *mockRPCServer) IsRunning() bool { return true }

func (m *mockRPCServer) Health(ctx context.Context) (*rpc.HealthStatus, error) {
	return m.healthResult, m.err
}

func (m *mockRPCServer) Status(ctx context.Context) (*rpc.NodeStatus, error) {
	return m.statusResult, m.err
}

func (m *mockRPCServer) NetInfo(ctx context.Context) (*rpc.NetInfo, error) {
	return m.netInfoResult, m.err
}

func (m *mockRPCServer) BroadcastTx(ctx context.Context, tx []byte, mode rpc.BroadcastMode) (*rpc.BroadcastResult, error) {
	return m.broadcastResult, m.err
}

func (m *mockRPCServer) Query(ctx context.Context, path string, data []byte, height uint64, prove bool) (*abi.QueryResponse, error) {
	return m.queryResult, m.err
}

func (m *mockRPCServer) Block(ctx context.Context, height uint64) (*rpc.BlockResult, error) {
	return m.blockResult, m.err
}

func (m *mockRPCServer) BlockByHash(ctx context.Context, hash []byte) (*rpc.BlockResult, error) {
	return m.blockResult, m.err
}

func (m *mockRPCServer) Tx(ctx context.Context, hash []byte) (*rpc.TxResult, error) {
	return m.txResult, m.err
}

func (m *mockRPCServer) TxSearch(ctx context.Context, query string, page, perPage int) ([]*rpc.TxResult, int, error) {
	return m.txSearchResults, len(m.txSearchResults), m.err
}

func (m *mockRPCServer) Peers(ctx context.Context) ([]rpc.PeerInfo, error) {
	return m.peersResult, m.err
}

func (m *mockRPCServer) ConsensusState(ctx context.Context) (*rpc.ConsensusState, error) {
	return m.consensusResult, m.err
}

func (m *mockRPCServer) Subscribe(ctx context.Context, subscriber string, query abi.Query) (<-chan abi.Event, error) {
	return m.subscribeChannel, m.err
}

func (m *mockRPCServer) Unsubscribe(ctx context.Context, subscriber string, query abi.Query) error {
	return m.err
}

func (m *mockRPCServer) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return m.err
}

func TestNewServer(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()

	server := NewServer(mock, config)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if len(server.methods) == 0 {
		t.Error("no methods registered")
	}
}

func TestServer_StartStop(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	config.ListenAddr = "tcp://127.0.0.1:0" // Random port

	server := NewServer(mock, config)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !server.IsRunning() {
		t.Error("IsRunning() = false after Start()")
	}

	// Starting again should be no-op
	if err := server.Start(); err != nil {
		t.Errorf("Second Start() error = %v", err)
	}

	if err := server.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if server.IsRunning() {
		t.Error("IsRunning() = true after Stop()")
	}

	// Stopping again should be no-op
	if err := server.Stop(); err != nil {
		t.Errorf("Second Stop() error = %v", err)
	}
}

func TestServer_HandleHealth(t *testing.T) {
	mock := &mockRPCServer{
		healthResult: &rpc.HealthStatus{
			Status: "healthy",
			Checks: map[string]rpc.HealthCheck{
				"consensus": {Status: "pass", Message: "ok"},
			},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "2.0",
		Method:  "health",
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.ID != 1 {
		t.Errorf("ID = %v, want 1", resp.ID)
	}
}

func TestServer_HandleStatus(t *testing.T) {
	mock := &mockRPCServer{
		statusResult: &rpc.NodeStatus{
			NodeInfo: rpc.NodeInfo{
				Moniker: "test-node",
				Network: "test-chain",
			},
			SyncInfo: rpc.SyncInfo{
				LatestBlockHeight: 100,
				CatchingUp:        false,
			},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "2.0",
		Method:  "status",
		ID:      "test-id",
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleBroadcastTxSync(t *testing.T) {
	mock := &mockRPCServer{
		broadcastResult: &rpc.BroadcastResult{
			Code: abi.CodeOK,
			Hash: []byte("tx-hash"),
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	txBytes := []byte("test-transaction")
	params, _ := json.Marshal(broadcastTxParams{Tx: hex.EncodeToString(txBytes)})

	req := Request{
		JSONRPC: "2.0",
		Method:  "broadcast_tx_sync",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleBroadcastTxAsync(t *testing.T) {
	mock := &mockRPCServer{
		broadcastResult: &rpc.BroadcastResult{
			Code: abi.CodeOK,
			Hash: []byte("tx-hash"),
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	txBytes := []byte("test-transaction")
	params, _ := json.Marshal(broadcastTxParams{Tx: hex.EncodeToString(txBytes)})

	req := Request{
		JSONRPC: "2.0",
		Method:  "broadcast_tx_async",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleBroadcastTxCommit(t *testing.T) {
	mock := &mockRPCServer{
		broadcastResult: &rpc.BroadcastResult{
			Code:   abi.CodeOK,
			Hash:   []byte("tx-hash"),
			Height: 100,
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	txBytes := []byte("test-transaction")
	params, _ := json.Marshal(broadcastTxParams{Tx: hex.EncodeToString(txBytes)})

	req := Request{
		JSONRPC: "2.0",
		Method:  "broadcast_tx_commit",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleBroadcastTx_InvalidHex(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(broadcastTxParams{Tx: "not-valid-hex-!!!"})

	req := Request{
		JSONRPC: "2.0",
		Method:  "broadcast_tx_sync",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error == nil {
		t.Fatal("expected error for invalid hex")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeInvalidParams)
	}
}

func TestServer_HandleBlock(t *testing.T) {
	mock := &mockRPCServer{
		blockResult: &rpc.BlockResult{
			Block: &abi.Block{
				Header: abi.BlockHeader{
					Height: 100,
				},
			},
			BlockID: rpc.BlockID{
				Hash: []byte("block-hash"),
			},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(blockParams{Height: 100})

	req := Request{
		JSONRPC: "2.0",
		Method:  "block",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleBlockByHash(t *testing.T) {
	mock := &mockRPCServer{
		blockResult: &rpc.BlockResult{
			Block: &abi.Block{
				Header: abi.BlockHeader{
					Height: 100,
				},
			},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(blockByHashParams{Hash: hex.EncodeToString([]byte("block-hash"))})

	req := Request{
		JSONRPC: "2.0",
		Method:  "block_by_hash",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleTx(t *testing.T) {
	mock := &mockRPCServer{
		txResult: &rpc.TxResult{
			Hash:   []byte("tx-hash"),
			Height: 100,
			Index:  0,
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(txParams{Hash: hex.EncodeToString([]byte("tx-hash"))})

	req := Request{
		JSONRPC: "2.0",
		Method:  "tx",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleTxSearch(t *testing.T) {
	mock := &mockRPCServer{
		txSearchResults: []*rpc.TxResult{
			{Hash: []byte("tx1"), Height: 100},
			{Hash: []byte("tx2"), Height: 101},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(txSearchParams{Query: "tx.height>99", Page: 1, PerPage: 10})

	req := Request{
		JSONRPC: "2.0",
		Method:  "tx_search",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["total"] != float64(2) {
		t.Errorf("total = %v, want 2", result["total"])
	}
}

func TestServer_HandleTxSearch_DefaultPagination(t *testing.T) {
	mock := &mockRPCServer{
		txSearchResults: []*rpc.TxResult{},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	// Send with page=0 and per_page=0 to test defaults
	params, _ := json.Marshal(txSearchParams{Query: "tx.height>99"})

	req := Request{
		JSONRPC: "2.0",
		Method:  "tx_search",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["page"] != float64(1) {
		t.Errorf("page = %v, want 1", result["page"])
	}
	if result["per_page"] != float64(30) {
		t.Errorf("per_page = %v, want 30", result["per_page"])
	}
}

func TestServer_HandleQuery(t *testing.T) {
	mock := &mockRPCServer{
		queryResult: &abi.QueryResponse{
			Key:   []byte("key"),
			Value: []byte("value"),
			Code:  abi.CodeOK,
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(queryParams{
		Path:   "/store/key",
		Data:   hex.EncodeToString([]byte("query-data")),
		Height: 100,
		Prove:  true,
	})

	req := Request{
		JSONRPC: "2.0",
		Method:  "abci_query",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleConsensusState(t *testing.T) {
	mock := &mockRPCServer{
		consensusResult: &rpc.ConsensusState{
			Height:    100,
			Round:     0,
			Step:      "prevote",
			StartTime: time.Now(),
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "2.0",
		Method:  "consensus_state",
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandlePeers(t *testing.T) {
	mock := &mockRPCServer{
		peersResult: []rpc.PeerInfo{
			{ID: "peer1", Address: "192.168.1.1:26656"},
			{ID: "peer2", Address: "192.168.1.2:26656"},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "2.0",
		Method:  "peers",
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleSubscribe(t *testing.T) {
	ch := make(chan abi.Event, 1)
	mock := &mockRPCServer{
		subscribeChannel: ch,
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(subscribeParams{
		Subscriber: "test-client",
		Query:      "type=Transfer",
	})

	req := Request{
		JSONRPC: "2.0",
		Method:  "subscribe",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleUnsubscribe(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(subscribeParams{
		Subscriber: "test-client",
		Query:      "type=Transfer",
	})

	req := Request{
		JSONRPC: "2.0",
		Method:  "unsubscribe",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleUnsubscribeAll(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	params, _ := json.Marshal(unsubscribeAllParams{
		Subscriber: "test-client",
	})

	req := Request{
		JSONRPC: "2.0",
		Method:  "unsubscribe_all",
		Params:  params,
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "2.0",
		Method:  "nonexistent_method",
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeMethodNotFound)
	}
}

func TestServer_InvalidJSONRPCVersion(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "1.0",
		Method:  "health",
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error == nil {
		t.Fatal("expected error for invalid JSONRPC version")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeInvalidRequest)
	}
}

func TestServer_InvalidParams(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := Request{
		JSONRPC: "2.0",
		Method:  "broadcast_tx_sync",
		Params:  []byte("not-valid-json"),
		ID:      1,
	}

	resp := server.processRequest(context.Background(), &req)

	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != CodeInvalidParams {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeInvalidParams)
	}
}

func TestServer_HandleHTTP(t *testing.T) {
	mock := &mockRPCServer{
		healthResult: &rpc.HealthStatus{Status: "healthy"},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	reqBody := `{"jsonrpc":"2.0","method":"health","id":1}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()

	server.handleHTTP(w, req)

	res := w.Result()
	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", res.StatusCode, http.StatusOK)
	}

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestServer_HandleHTTP_MethodNotAllowed(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	server.handleHTTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("StatusCode = %d, want %d", res.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestServer_HandleHTTP_InvalidJSON(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not-valid-json"))
	w := httptest.NewRecorder()

	server.handleHTTP(w, req)

	res := w.Result()
	body, _ := io.ReadAll(res.Body)

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if resp.Error.Code != CodeParseError {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeParseError)
	}
}

func TestServer_HandleBatch(t *testing.T) {
	mock := &mockRPCServer{
		healthResult: &rpc.HealthStatus{Status: "healthy"},
		statusResult: &rpc.NodeStatus{
			NodeInfo: rpc.NodeInfo{Moniker: "test"},
		},
	}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	reqBody := `[
		{"jsonrpc":"2.0","method":"health","id":1},
		{"jsonrpc":"2.0","method":"status","id":2}
	]`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()

	server.handleHTTP(w, req)

	res := w.Result()
	body, _ := io.ReadAll(res.Body)

	var responses BatchResponse
	if err := json.Unmarshal(body, &responses); err != nil {
		t.Fatalf("failed to unmarshal batch response: %v", err)
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	for i, resp := range responses {
		if resp.Error != nil {
			t.Errorf("response[%d] unexpected error: %v", i, resp.Error)
		}
	}
}

func TestServer_HandleBatch_Empty(t *testing.T) {
	mock := &mockRPCServer{}
	config := rpc.DefaultServerConfig()
	server := NewServer(mock, config)

	reqBody := `[]`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()

	server.handleHTTP(w, req)

	res := w.Result()
	body, _ := io.ReadAll(res.Body)

	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for empty batch")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeInvalidRequest)
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{
			name:     "empty string",
			input:    "",
			wantType: "all",
		},
		{
			name:     "all keyword",
			input:    "all",
			wantType: "all",
		},
		{
			name:     "asterisk",
			input:    "*",
			wantType: "all",
		},
		{
			name:     "type query",
			input:    "type=Transfer",
			wantType: "type=Transfer",
		},
		{
			name:     "attribute query",
			input:    "sender=addr123",
			wantType: "sender=addr123",
		},
		{
			name:     "with whitespace",
			input:    "  all  ",
			wantType: "all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := parseQuery(tt.input)
			if query.String() != tt.wantType {
				t.Errorf("parseQuery(%q).String() = %q, want %q", tt.input, query.String(), tt.wantType)
			}
		})
	}
}
