package websocket

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/abi"
	"github.com/blockberries/blockberry/events"
)

// dialWS connects to a WebSocket server and returns the connection.
func dialWS(t *testing.T, url string, headers http.Header) net.Conn {
	t.Helper()

	// Convert http:// to ws://
	url = strings.Replace(url, "http://", "ws://", 1)

	dialer := ws.Dialer{
		Header: ws.HandshakeHeaderHTTP(headers),
	}

	conn, _, _, err := dialer.Dial(context.Background(), url)
	require.NoError(t, err)
	return conn
}

// sendMessage sends a JSON message over WebSocket.
func sendMessage(t *testing.T, conn net.Conn, msg Message) {
	t.Helper()
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	err = wsutil.WriteClientMessage(conn, ws.OpText, data)
	require.NoError(t, err)
}

// readResponse reads and parses a JSON response from WebSocket.
func readResponse(t *testing.T, conn net.Conn) Response {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(time.Second))
	data, _, err := wsutil.ReadServerData(conn)
	require.NoError(t, err)

	var resp Response
	require.NoError(t, json.Unmarshal(data, &resp))
	return resp
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	require.Equal(t, 1024, cfg.ReadBufferSize)
	require.Equal(t, 1024, cfg.WriteBufferSize)
	require.Equal(t, 100, cfg.MaxClients)
	require.Equal(t, 10, cfg.MaxSubscriptionsPerClient)
	require.Equal(t, 30*time.Second, cfg.PingInterval)
	require.Equal(t, 10*time.Second, cfg.PongTimeout)
	require.Equal(t, 10*time.Second, cfg.WriteTimeout)
	require.Equal(t, 60*time.Second, cfg.ReadTimeout)
	require.Nil(t, cfg.AllowedOrigins)
}

func TestNewServer(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	cfg := DefaultConfig()
	server := NewServer(bus, cfg)

	require.NotNil(t, server)
	require.Equal(t, bus, server.eventBus)
	require.False(t, server.IsRunning())
}

func TestServer_StartStop(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())

	// Start
	require.NoError(t, server.Start())
	require.True(t, server.IsRunning())

	// Double start is no-op
	require.NoError(t, server.Start())
	require.True(t, server.IsRunning())

	// Stop
	require.NoError(t, server.Stop())
	require.False(t, server.IsRunning())

	// Double stop is no-op
	require.NoError(t, server.Stop())
	require.False(t, server.IsRunning())
}

func TestServer_Handler(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	handler := server.Handler()
	require.NotNil(t, handler)
}

func TestServer_ClientConnection(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Wait for client to register
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 1, server.ClientCount())
}

func TestServer_MaxClients(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	cfg := DefaultConfig()
	cfg.MaxClients = 2
	server := NewServer(bus, cfg)
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// Connect first two clients
	conn1 := dialWS(t, ts.URL, nil)
	defer conn1.Close()

	conn2 := dialWS(t, ts.URL, nil)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 2, server.ClientCount())

	// Third client should be rejected
	url := strings.Replace(ts.URL, "http://", "ws://", 1)
	_, _, _, err := ws.Dial(context.Background(), url)
	require.Error(t, err)
}

func TestServer_Subscribe(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Send subscribe message
	sendMessage(t, conn, Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "subscribe",
		Query:   "all",
	})

	// Read response
	resp := readResponse(t, conn)
	require.Equal(t, "2.0", resp.JSONRPC)
	require.Equal(t, float64(1), resp.ID)
	require.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)
}

func TestServer_ReceiveEvents(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Subscribe
	sendMessage(t, conn, Message{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "subscribe",
		Query:   "all",
	})

	// Read subscribe response
	resp := readResponse(t, conn)
	require.Nil(t, resp.Error)

	// Publish event
	event := abi.NewEvent("TestEvent").AddStringAttribute("key", "value")
	require.NoError(t, bus.Publish(context.Background(), event))

	// Read event
	eventResp := readResponse(t, conn)
	require.Nil(t, eventResp.Error)
	require.NotNil(t, eventResp.Result)
}

func TestServer_Unsubscribe(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Subscribe
	sendMessage(t, conn, Message{JSONRPC: "2.0", ID: 1, Method: "subscribe", Query: "type=TestEvent"})
	_ = readResponse(t, conn) // Consume response

	// Unsubscribe
	sendMessage(t, conn, Message{JSONRPC: "2.0", ID: 2, Method: "unsubscribe", Query: "type=TestEvent"})

	resp := readResponse(t, conn)
	require.Nil(t, resp.Error)
}

func TestServer_UnsubscribeAll(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Subscribe to multiple queries
	for i := 1; i <= 3; i++ {
		sendMessage(t, conn, Message{JSONRPC: "2.0", ID: i, Method: "subscribe", Query: "type=Event" + string(rune('0'+i))})
		_ = readResponse(t, conn) // Consume response
	}

	// Unsubscribe all
	sendMessage(t, conn, Message{JSONRPC: "2.0", ID: 10, Method: "unsubscribe_all"})

	resp := readResponse(t, conn)
	require.Nil(t, resp.Error)
}

func TestServer_MaxSubscriptionsPerClient(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	cfg := DefaultConfig()
	cfg.MaxSubscriptionsPerClient = 2
	server := NewServer(bus, cfg)
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// First two subscriptions should succeed
	for i := 1; i <= 2; i++ {
		sendMessage(t, conn, Message{JSONRPC: "2.0", ID: i, Method: "subscribe", Query: "type=Event" + string(rune('0'+i))})
		resp := readResponse(t, conn)
		require.Nil(t, resp.Error)
	}

	// Third subscription should fail
	sendMessage(t, conn, Message{JSONRPC: "2.0", ID: 3, Method: "subscribe", Query: "type=Event3"})
	resp := readResponse(t, conn)
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Message, "maximum subscriptions")
}

func TestServer_InvalidMessage(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Send invalid JSON
	err := wsutil.WriteClientMessage(conn, ws.OpText, []byte("not json"))
	require.NoError(t, err)

	resp := readResponse(t, conn)
	require.NotNil(t, resp.Error)
}

func TestServer_UnknownMethod(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	sendMessage(t, conn, Message{JSONRPC: "2.0", ID: 1, Method: "unknown_method"})

	resp := readResponse(t, conn)
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Message, "unknown method")
}

func TestServer_ClientDisconnect(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)

	// Wait for client to register
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 1, server.ClientCount())

	// Disconnect client
	conn.Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 0, server.ClientCount())
}

func TestServer_StopDisconnectsClients(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	require.NoError(t, server.Start())

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 1, server.ClientCount())

	// Stop server
	require.NoError(t, server.Stop())
	require.Equal(t, 0, server.ClientCount())
}

func TestServer_ConcurrentSubscriptions(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	cfg := DefaultConfig()
	cfg.MaxSubscriptionsPerClient = 100
	server := NewServer(bus, cfg)
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	conn := dialWS(t, ts.URL, nil)
	defer conn.Close()

	// Subscribe sequentially (WebSocket doesn't allow concurrent writes)
	for i := range 10 {
		sendMessage(t, conn, Message{JSONRPC: "2.0", ID: i, Method: "subscribe", Query: "type=Event" + string(rune('A'+i))})
	}

	// Read all responses
	for range 10 {
		resp := readResponse(t, conn)
		require.Nil(t, resp.Error)
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected abi.Query
	}{
		{"empty", "", abi.QueryAll{}},
		{"all", "all", abi.QueryAll{}},
		{"star", "*", abi.QueryAll{}},
		{"type", "type=NewBlock", abi.QueryEventType{EventType: "NewBlock"}},
		{"attribute", "sender=alice", abi.QueryAttribute{Key: "sender", Value: "alice"}},
		{"event type only", "NewBlock", abi.QueryEventType{EventType: "NewBlock"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseQuery(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestServer_AllowedOrigins(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	cfg := DefaultConfig()
	cfg.AllowedOrigins = []string{"http://allowed.com"}
	server := NewServer(bus, cfg)
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	url := strings.Replace(ts.URL, "http://", "ws://", 1)

	// Allowed origin should work
	headers := http.Header{}
	headers.Set("Origin", "http://allowed.com")
	dialer := ws.Dialer{Header: ws.HandshakeHeaderHTTP(headers)}
	conn, _, _, err := dialer.Dial(context.Background(), url)
	require.NoError(t, err)
	conn.Close()

	// Disallowed origin should fail
	headers.Set("Origin", "http://disallowed.com")
	dialer = ws.Dialer{Header: ws.HandshakeHeaderHTTP(headers)}
	_, _, _, err = dialer.Dial(context.Background(), url)
	require.Error(t, err)
}

func TestServer_AllowAllOrigins(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	cfg := DefaultConfig()
	cfg.AllowedOrigins = []string{"*"}
	server := NewServer(bus, cfg)
	require.NoError(t, server.Start())
	defer server.Stop()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	url := strings.Replace(ts.URL, "http://", "ws://", 1)

	// Any origin should work with "*"
	headers := http.Header{}
	headers.Set("Origin", "http://any-origin.com")
	dialer := ws.Dialer{Header: ws.HandshakeHeaderHTTP(headers)}
	conn, _, _, err := dialer.Dial(context.Background(), url)
	require.NoError(t, err)
	conn.Close()
}

func TestServer_NotRunning(t *testing.T) {
	bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
	require.NoError(t, bus.Start())
	defer bus.Stop()

	server := NewServer(bus, DefaultConfig())
	// Don't start the server

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	url := strings.Replace(ts.URL, "http://", "ws://", 1)
	_, _, _, err := ws.Dial(context.Background(), url)
	require.Error(t, err)
}
