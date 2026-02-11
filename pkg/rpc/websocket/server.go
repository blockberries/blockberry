// Package websocket provides WebSocket transport for RPC event subscriptions.
// Uses gobwas/ws for zero-allocation upgrades and efficient connection handling.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	bapitypes "github.com/blockberries/bapi/types"
	"github.com/blockberries/blockberry/pkg/events"
	"github.com/blockberries/blockberry/pkg/logging"
)

// Common errors.
var (
	ErrServerClosed       = errors.New("websocket server closed")
	ErrClientDisconnected = errors.New("client disconnected")
	ErrMaxClients         = errors.New("maximum clients reached")
	ErrMaxSubscriptions   = errors.New("maximum subscriptions per client reached")
	ErrInvalidMessage     = errors.New("invalid message format")
	ErrSubscribeFailed    = errors.New("subscribe failed")
	ErrUnsubscribeFailed  = errors.New("unsubscribe failed")
)

// Config contains WebSocket server configuration.
type Config struct {
	// ReadBufferSize is the read buffer size.
	ReadBufferSize int

	// WriteBufferSize is the write buffer size.
	WriteBufferSize int

	// MaxClients is the maximum number of connected clients.
	MaxClients int

	// MaxSubscriptionsPerClient is the max subscriptions per client.
	MaxSubscriptionsPerClient int

	// PingInterval is the interval for sending ping messages.
	PingInterval time.Duration

	// PongTimeout is the timeout for receiving pong responses.
	PongTimeout time.Duration

	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration

	// ReadTimeout is the timeout for read operations.
	ReadTimeout time.Duration

	// AllowedOrigins restricts allowed origins. Empty means all allowed.
	AllowedOrigins []string
}

// DefaultConfig returns sensible defaults for WebSocket configuration.
func DefaultConfig() Config {
	return Config{
		ReadBufferSize:            1024,
		WriteBufferSize:           1024,
		MaxClients:                100,
		MaxSubscriptionsPerClient: 10,
		PingInterval:              30 * time.Second,
		PongTimeout:               10 * time.Second,
		WriteTimeout:              10 * time.Second,
		ReadTimeout:               60 * time.Second,
		AllowedOrigins:            nil,
	}
}

// Server handles WebSocket connections for event subscriptions.
type Server struct {
	eventBus events.EventBus
	cfg      Config
	upgrader ws.HTTPUpgrader
	logger   *logging.Logger

	mu      sync.RWMutex
	clients map[string]*Client
	running atomic.Bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new WebSocket server.
func NewServer(eventBus events.EventBus, cfg Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		eventBus: eventBus,
		cfg:      cfg,
		logger:   logging.NewNopLogger().WithComponent("websocket"),
		clients:  make(map[string]*Client),
		ctx:      ctx,
		cancel:   cancel,
	}

	s.upgrader = ws.HTTPUpgrader{
		Timeout: 10 * time.Second,
		Header:  nil,
		Protocol: func(proto string) bool {
			return true // Accept any subprotocol
		},
	}

	return s
}

// SetLogger sets the logger for the WebSocket server.
func (s *Server) SetLogger(logger *logging.Logger) {
	if logger != nil {
		s.logger = logger.WithComponent("websocket")
	}
}

// Start starts the WebSocket server.
func (s *Server) Start() error {
	if s.running.Swap(true) {
		return nil // Already running
	}
	return nil
}

// Stop stops the WebSocket server and disconnects all clients.
func (s *Server) Stop() error {
	if !s.running.Swap(false) {
		return nil // Already stopped
	}

	s.cancel()

	// Close all client connections
	s.mu.Lock()
	for _, client := range s.clients {
		client.Close()
	}
	s.clients = make(map[string]*Client)
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	return s.running.Load()
}

// Handler returns an HTTP handler for WebSocket connections.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(s.handleWebSocket)
}

// ClientCount returns the number of connected clients.
func (s *Server) ClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

// handleWebSocket handles incoming WebSocket connections.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !s.running.Load() {
		http.Error(w, "server not running", http.StatusServiceUnavailable)
		return
	}

	// Check origin
	if !s.checkOrigin(r) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	// Check client limit
	s.mu.RLock()
	clientCount := len(s.clients)
	s.mu.RUnlock()

	if s.cfg.MaxClients > 0 && clientCount >= s.cfg.MaxClients {
		http.Error(w, "max clients reached", http.StatusServiceUnavailable)
		return
	}

	// Upgrade connection using gobwas/ws zero-allocation upgrade
	conn, _, _, err := s.upgrader.Upgrade(r, w)
	if err != nil {
		return // Upgrader already wrote error response
	}

	// Create client
	client := newClient(s, conn, r.RemoteAddr)

	// Register client
	s.mu.Lock()
	s.clients[client.id] = client
	s.mu.Unlock()

	// Start client handler
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		client.run()
		s.removeClient(client.id)
	}()
}

// removeClient removes a client from the server.
func (s *Server) removeClient(id string) {
	s.mu.Lock()
	delete(s.clients, id)
	s.mu.Unlock()
}

// checkOrigin validates the request origin.
func (s *Server) checkOrigin(r *http.Request) bool {
	if len(s.cfg.AllowedOrigins) == 0 {
		return true
	}

	origin := r.Header.Get("Origin")
	for _, allowed := range s.cfg.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// Client represents a connected WebSocket client.
type Client struct {
	id         string
	server     *Server
	conn       net.Conn
	remoteAddr string
	logger     *logging.Logger

	mu            sync.Mutex
	subscriptions map[string]*clientSubscription
	subCount      int

	writeMu sync.Mutex // Protects writes to conn
	sendCh  chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
}

// clientSubscription tracks a single subscription for a client.
type clientSubscription struct {
	query    events.Query
	eventCh  <-chan bapitypes.Event
	cancelFn context.CancelFunc
}

// newClient creates a new client.
func newClient(s *Server, conn net.Conn, remoteAddr string) *Client {
	ctx, cancel := context.WithCancel(s.ctx)
	clientID := fmt.Sprintf("%s-%d", remoteAddr, time.Now().UnixNano())
	return &Client{
		id:            clientID,
		server:        s,
		conn:          conn,
		remoteAddr:    remoteAddr,
		logger:        s.logger.With(slog.String("client_id", clientID)),
		subscriptions: make(map[string]*clientSubscription),
		sendCh:        make(chan []byte, 256),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// run handles the client connection lifecycle.
func (c *Client) run() {
	var wg sync.WaitGroup

	// Start writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.writeLoop()
	}()

	// Start reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.readLoop()
	}()

	// Start ping loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.pingLoop()
	}()

	// Wait for context cancellation
	<-c.ctx.Done()

	// Close connection and cleanup
	c.Close()
	wg.Wait()
}

// readLoop reads messages from the client.
func (c *Client) readLoop() {
	defer c.cancel()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(c.server.cfg.ReadTimeout))

		// Read message using wsutil - needs io.ReadWriter for control frame handling
		data, op, err := wsutil.ReadClientData(c.conn)
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				// Log error if needed
			}
			return
		}

		// Handle control frames
		if op == ws.OpPong {
			continue
		}

		if op == ws.OpClose {
			return
		}

		// Handle text/binary messages
		if op == ws.OpText || op == ws.OpBinary {
			c.handleMessage(data)
		}
	}
}

// writeLoop writes messages to the client.
func (c *Client) writeLoop() {
	defer c.cancel()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.sendCh:
			if err := c.writeMessage(ws.OpText, msg); err != nil {
				return
			}
		}
	}
}

// writeMessage writes a message to the connection with proper locking.
func (c *Client) writeMessage(op ws.OpCode, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	c.conn.SetWriteDeadline(time.Now().Add(c.server.cfg.WriteTimeout))
	return wsutil.WriteServerMessage(c.conn, op, data)
}

// pingLoop sends periodic ping messages.
func (c *Client) pingLoop() {
	ticker := time.NewTicker(c.server.cfg.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if err := c.writeMessage(ws.OpPing, nil); err != nil {
				c.cancel()
				return
			}
		}
	}
}

// handleMessage processes an incoming message from the client.
func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		c.sendError(msg.ID, ErrInvalidMessage)
		return
	}

	switch msg.Method {
	case "subscribe":
		c.handleSubscribe(msg)
	case "unsubscribe":
		c.handleUnsubscribe(msg)
	case "unsubscribe_all":
		c.handleUnsubscribeAll(msg)
	default:
		c.sendError(msg.ID, fmt.Errorf("unknown method: %s", msg.Method))
	}
}

// handleSubscribe handles a subscribe request.
func (c *Client) handleSubscribe(msg Message) {
	// Check subscription limit
	c.mu.Lock()
	if c.server.cfg.MaxSubscriptionsPerClient > 0 && c.subCount >= c.server.cfg.MaxSubscriptionsPerClient {
		c.mu.Unlock()
		c.sendError(msg.ID, ErrMaxSubscriptions)
		return
	}

	// Check if already subscribed
	queryStr := msg.Query
	if _, exists := c.subscriptions[queryStr]; exists {
		c.mu.Unlock()
		c.sendResult(msg.ID, map[string]any{"subscribed": true, "query": queryStr})
		return
	}
	c.mu.Unlock()

	// Parse query
	query := parseQuery(msg.Query)

	// Subscribe to event bus
	subCtx, subCancel := context.WithCancel(c.ctx)
	eventCh, err := c.server.eventBus.Subscribe(subCtx, c.id, query)
	if err != nil {
		subCancel()
		c.sendError(msg.ID, ErrSubscribeFailed)
		return
	}

	// Store subscription
	sub := &clientSubscription{
		query:    query,
		eventCh:  eventCh,
		cancelFn: subCancel,
	}

	c.mu.Lock()
	c.subscriptions[queryStr] = sub
	c.subCount++
	c.mu.Unlock()

	// Start event forwarder
	go c.forwardEvents(queryStr, sub)

	c.sendResult(msg.ID, map[string]any{"subscribed": true, "query": queryStr})
}

// handleUnsubscribe handles an unsubscribe request.
func (c *Client) handleUnsubscribe(msg Message) {
	queryStr := msg.Query

	c.mu.Lock()
	sub, exists := c.subscriptions[queryStr]
	if !exists {
		c.mu.Unlock()
		c.sendResult(msg.ID, map[string]any{"unsubscribed": true, "query": queryStr})
		return
	}

	delete(c.subscriptions, queryStr)
	c.subCount--
	c.mu.Unlock()

	// Cancel subscription
	sub.cancelFn()

	// Unsubscribe from event bus
	_ = c.server.eventBus.Unsubscribe(c.ctx, c.id, sub.query)

	c.sendResult(msg.ID, map[string]any{"unsubscribed": true, "query": queryStr})
}

// handleUnsubscribeAll handles an unsubscribe_all request.
func (c *Client) handleUnsubscribeAll(msg Message) {
	c.mu.Lock()
	subs := make(map[string]*clientSubscription, len(c.subscriptions))
	maps.Copy(subs, c.subscriptions)
	c.subscriptions = make(map[string]*clientSubscription)
	c.subCount = 0
	c.mu.Unlock()

	// Cancel all subscriptions
	for _, sub := range subs {
		sub.cancelFn()
	}

	// Unsubscribe from event bus
	_ = c.server.eventBus.UnsubscribeAll(c.ctx, c.id)

	c.sendResult(msg.ID, map[string]any{"unsubscribed_all": true})
}

// forwardEvents forwards events from the event bus to the client.
func (c *Client) forwardEvents(queryStr string, sub *clientSubscription) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case event, ok := <-sub.eventCh:
			if !ok {
				return
			}
			c.sendEvent(queryStr, event)
		}
	}
}

// sendEvent sends an event to the client.
func (c *Client) sendEvent(query string, event bapitypes.Event) {
	resp := Response{
		JSONRPC: "2.0",
		Result: EventData{
			Query: query,
			Event: event,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	select {
	case c.sendCh <- data:
	default:
		// Channel full, drop event and log warning
		c.logger.Warn("event dropped: send channel full",
			slog.String("query", query),
			slog.String("event_kind", event.Kind))
	}
}

// sendResult sends a successful result to the client.
func (c *Client) sendResult(id any, result any) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return
	}

	select {
	case c.sendCh <- data:
	default:
		// Channel full, drop result and log warning
		c.logger.Warn("result dropped: send channel full",
			slog.Any("request_id", id))
	}
}

// sendError sends an error to the client.
func (c *Client) sendError(id any, err error) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ErrorData{
			Code:    -32000,
			Message: err.Error(),
		},
	}

	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		return
	}

	select {
	case c.sendCh <- data:
	default:
		// Channel full, drop error and log warning
		c.logger.Warn("error response dropped: send channel full",
			slog.Any("request_id", id),
			slog.String("error", err.Error()))
	}
}

// Close closes the client connection.
func (c *Client) Close() {
	c.cancel()

	// Unsubscribe all
	c.mu.Lock()
	for _, sub := range c.subscriptions {
		sub.cancelFn()
	}
	c.subscriptions = make(map[string]*clientSubscription)
	c.subCount = 0
	c.mu.Unlock()

	// Unsubscribe from event bus
	_ = c.server.eventBus.UnsubscribeAll(context.Background(), c.id)

	// Send close frame before closing
	c.writeMu.Lock()
	_ = wsutil.WriteServerMessage(c.conn, ws.OpClose, nil)
	c.writeMu.Unlock()

	// Close connection
	c.conn.Close()
}

// Message represents a WebSocket message from the client.
type Message struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Query   string `json:"query,omitempty"`
}

// Response represents a WebSocket response to the client.
type Response struct {
	JSONRPC string     `json:"jsonrpc"`
	ID      any        `json:"id,omitempty"`
	Result  any        `json:"result,omitempty"`
	Error   *ErrorData `json:"error,omitempty"`
}

// ErrorData contains error information.
type ErrorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// EventData contains event data sent to clients.
type EventData struct {
	Query string          `json:"query"`
	Event bapitypes.Event `json:"event"`
}

// parseQuery parses a query string into an events.Query.
// Supported formats:
//   - "all" or "*" -> QueryAll
//   - "type=EventKind" -> QueryEventKind
//   - "key=value" -> QueryAttribute
func parseQuery(s string) events.Query {
	if s == "" || s == "all" || s == "*" {
		return events.QueryAll{}
	}

	// Check for type= prefix
	if len(s) > 5 && s[:5] == "type=" {
		return events.QueryEventKind{Kind: s[5:]}
	}

	// Check for key=value format
	for i, c := range s {
		if c == '=' && i > 0 && i < len(s)-1 {
			return events.QueryAttribute{
				Key:   s[:i],
				Value: s[i+1:],
			}
		}
	}

	// Default to event kind
	return events.QueryEventKind{Kind: s}
}
