package jsonrpc

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blockberries/blockberry/abi"
	"github.com/blockberries/blockberry/rpc"
)

// Server is a JSON-RPC 2.0 server that wraps an RPC server.
type Server struct {
	rpc    rpc.Server
	config rpc.ServerConfig

	httpServer *http.Server
	listener   net.Listener

	methods map[string]MethodHandler
	running atomic.Bool
	mu      sync.RWMutex
}

// MethodHandler handles a specific RPC method.
type MethodHandler func(ctx context.Context, params json.RawMessage) (interface{}, error)

// NewServer creates a new JSON-RPC server.
func NewServer(rpcServer rpc.Server, config rpc.ServerConfig) *Server {
	s := &Server{
		rpc:     rpcServer,
		config:  config,
		methods: make(map[string]MethodHandler),
	}
	s.registerMethods()
	return s
}

// registerMethods registers all RPC methods.
func (s *Server) registerMethods() {
	// Health and status
	s.methods["health"] = s.handleHealth
	s.methods["status"] = s.handleStatus
	s.methods["net_info"] = s.handleNetInfo

	// Transaction methods
	s.methods["broadcast_tx_sync"] = s.handleBroadcastTxSync
	s.methods["broadcast_tx_async"] = s.handleBroadcastTxAsync
	s.methods["broadcast_tx_commit"] = s.handleBroadcastTxCommit

	// Query methods
	s.methods["abci_query"] = s.handleQuery
	s.methods["block"] = s.handleBlock
	s.methods["block_by_hash"] = s.handleBlockByHash
	s.methods["tx"] = s.handleTx
	s.methods["tx_search"] = s.handleTxSearch

	// Consensus methods
	s.methods["consensus_state"] = s.handleConsensusState
	s.methods["peers"] = s.handlePeers

	// Subscription methods
	s.methods["subscribe"] = s.handleSubscribe
	s.methods["unsubscribe"] = s.handleUnsubscribe
	s.methods["unsubscribe_all"] = s.handleUnsubscribeAll
}

// Start starts the JSON-RPC server.
func (s *Server) Start() error {
	if s.running.Swap(true) {
		return nil // Already running
	}

	addr := s.config.ListenAddr
	if strings.HasPrefix(addr, "tcp://") {
		addr = strings.TrimPrefix(addr, "tcp://")
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.running.Store(false)
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHTTP)

	s.httpServer = &http.Server{
		Handler:        mux,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
	}

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash
		}
	}()

	return nil
}

// Stop stops the JSON-RPC server.
func (s *Server) Stop() error {
	if !s.running.Swap(false) {
		return nil // Already stopped
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	return nil
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	return s.running.Load()
}

// handleHTTP handles HTTP requests.
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, s.config.MaxBodyBytes))
	if err != nil {
		s.writeError(w, nil, ErrParseError)
		return
	}

	// Check if batch request
	if len(body) > 0 && body[0] == '[' {
		s.handleBatch(w, r.Context(), body)
		return
	}

	// Single request
	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeError(w, nil, ErrParseError)
		return
	}

	resp := s.processRequest(r.Context(), &req)
	s.writeResponse(w, resp)
}

// handleBatch handles batch requests.
func (s *Server) handleBatch(w http.ResponseWriter, ctx context.Context, body []byte) {
	var batch BatchRequest
	if err := json.Unmarshal(body, &batch); err != nil {
		s.writeError(w, nil, ErrParseError)
		return
	}

	if len(batch) == 0 {
		s.writeError(w, nil, ErrInvalidRequest)
		return
	}

	responses := make(BatchResponse, len(batch))
	for i, req := range batch {
		responses[i] = *s.processRequest(ctx, &req)
	}

	data, _ := json.Marshal(responses)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// processRequest processes a single JSON-RPC request.
func (s *Server) processRequest(ctx context.Context, req *Request) *Response {
	if req.JSONRPC != "2.0" {
		return NewErrorResponse(req.ID, ErrInvalidRequest)
	}

	handler, ok := s.methods[req.Method]
	if !ok {
		return NewErrorResponse(req.ID, ErrMethodNotFound)
	}

	result, err := handler(ctx, req.Params)
	if err != nil {
		if rpcErr, ok := err.(*Error); ok {
			return NewErrorResponse(req.ID, rpcErr)
		}
		return NewErrorResponse(req.ID, NewErrorWithData(CodeInternalError, err.Error(), nil))
	}

	resp, err := NewResponse(req.ID, result)
	if err != nil {
		return NewErrorResponse(req.ID, ErrInternalError)
	}
	return resp
}

// writeResponse writes a JSON-RPC response.
func (s *Server) writeResponse(w http.ResponseWriter, resp *Response) {
	data, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// writeError writes a JSON-RPC error response.
func (s *Server) writeError(w http.ResponseWriter, id interface{}, err *Error) {
	s.writeResponse(w, NewErrorResponse(id, err))
}

// Method handlers

func (s *Server) handleHealth(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.rpc.Health(ctx)
}

func (s *Server) handleStatus(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.rpc.Status(ctx)
}

func (s *Server) handleNetInfo(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.rpc.NetInfo(ctx)
}

type broadcastTxParams struct {
	Tx string `json:"tx"` // hex-encoded
}

func (s *Server) handleBroadcastTxSync(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.broadcastTx(ctx, params, rpc.BroadcastSync)
}

func (s *Server) handleBroadcastTxAsync(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.broadcastTx(ctx, params, rpc.BroadcastAsync)
}

func (s *Server) handleBroadcastTxCommit(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.broadcastTx(ctx, params, rpc.BroadcastCommit)
}

func (s *Server) broadcastTx(ctx context.Context, params json.RawMessage, mode rpc.BroadcastMode) (interface{}, error) {
	var p broadcastTxParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	tx, err := hex.DecodeString(p.Tx)
	if err != nil {
		return nil, NewErrorWithData(CodeInvalidParams, "invalid hex encoding", nil)
	}

	return s.rpc.BroadcastTx(ctx, tx, mode)
}

type queryParams struct {
	Path   string `json:"path"`
	Data   string `json:"data"` // hex-encoded
	Height uint64 `json:"height"`
	Prove  bool   `json:"prove"`
}

func (s *Server) handleQuery(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p queryParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	var data []byte
	var err error
	if p.Data != "" {
		data, err = hex.DecodeString(p.Data)
		if err != nil {
			return nil, NewErrorWithData(CodeInvalidParams, "invalid hex encoding", nil)
		}
	}

	return s.rpc.Query(ctx, p.Path, data, p.Height, p.Prove)
}

type blockParams struct {
	Height uint64 `json:"height"`
}

func (s *Server) handleBlock(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p blockParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	return s.rpc.Block(ctx, p.Height)
}

type blockByHashParams struct {
	Hash string `json:"hash"` // hex-encoded
}

func (s *Server) handleBlockByHash(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p blockByHashParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	hash, err := hex.DecodeString(p.Hash)
	if err != nil {
		return nil, NewErrorWithData(CodeInvalidParams, "invalid hex encoding", nil)
	}

	return s.rpc.BlockByHash(ctx, hash)
}

type txParams struct {
	Hash string `json:"hash"` // hex-encoded
}

func (s *Server) handleTx(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p txParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	hash, err := hex.DecodeString(p.Hash)
	if err != nil {
		return nil, NewErrorWithData(CodeInvalidParams, "invalid hex encoding", nil)
	}

	return s.rpc.Tx(ctx, hash)
}

type txSearchParams struct {
	Query   string `json:"query"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

func (s *Server) handleTxSearch(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p txSearchParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PerPage <= 0 {
		p.PerPage = 30
	}

	results, total, err := s.rpc.TxSearch(ctx, p.Query, p.Page, p.PerPage)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"txs":        results,
		"total":      total,
		"page":       p.Page,
		"per_page":   p.PerPage,
		"total_pages": (total + p.PerPage - 1) / p.PerPage,
	}, nil
}

func (s *Server) handleConsensusState(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.rpc.ConsensusState(ctx)
}

func (s *Server) handlePeers(ctx context.Context, params json.RawMessage) (interface{}, error) {
	return s.rpc.Peers(ctx)
}

type subscribeParams struct {
	Subscriber string `json:"subscriber"`
	Query      string `json:"query"`
}

func (s *Server) handleSubscribe(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p subscribeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	// Parse query string into Query interface
	query := parseQuery(p.Query)

	_, err := s.rpc.Subscribe(ctx, p.Subscriber, query)
	if err != nil {
		return nil, err
	}

	return map[string]string{"status": "subscribed"}, nil
}

func (s *Server) handleUnsubscribe(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p subscribeParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	query := parseQuery(p.Query)

	if err := s.rpc.Unsubscribe(ctx, p.Subscriber, query); err != nil {
		return nil, err
	}

	return map[string]string{"status": "unsubscribed"}, nil
}

type unsubscribeAllParams struct {
	Subscriber string `json:"subscriber"`
}

func (s *Server) handleUnsubscribeAll(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p unsubscribeAllParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, ErrInvalidParams
	}

	if err := s.rpc.UnsubscribeAll(ctx, p.Subscriber); err != nil {
		return nil, err
	}

	return map[string]string{"status": "unsubscribed"}, nil
}

// parseQuery parses a query string into an abi.Query.
// Supports simple queries like "type=Transfer" or "all".
func parseQuery(queryStr string) abi.Query {
	queryStr = strings.TrimSpace(queryStr)

	if queryStr == "" || queryStr == "all" || queryStr == "*" {
		return abi.QueryAll{}
	}

	// Simple type query: "type=EventType"
	if strings.HasPrefix(queryStr, "type=") {
		eventType := strings.TrimPrefix(queryStr, "type=")
		return abi.QueryEventType{EventType: eventType}
	}

	// Simple attribute query: "key=value"
	if strings.Contains(queryStr, "=") {
		parts := strings.SplitN(queryStr, "=", 2)
		if len(parts) == 2 {
			return abi.QueryAttribute{Key: parts[0], Value: parts[1]}
		}
	}

	// Default to all
	return abi.QueryAll{}
}
