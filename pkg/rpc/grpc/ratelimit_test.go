package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func TestDefaultRateLimitConfig(t *testing.T) {
	config := DefaultRateLimitConfig()

	require.False(t, config.Enabled)
	require.Equal(t, 1000, config.GlobalRate)
	require.Equal(t, 100, config.PerClientRate)
	require.Equal(t, time.Second, config.Interval)
	require.Equal(t, 50, config.Burst)
	require.Contains(t, config.ExemptMethods, "/blockberry.Node/Health")
}

func TestNewRateLimiter(t *testing.T) {
	config := RateLimitConfig{
		Enabled:         true,
		GlobalRate:      100,
		PerClientRate:   10,
		Interval:        time.Second,
		Burst:           5,
		CleanupInterval: time.Minute,
		ExemptMethods:   []string{"/exempt/method"},
		ExemptClients:   []string{"127.0.0.1:12345"},
	}

	rl := NewRateLimiter(config)
	defer rl.Close()

	require.NotNil(t, rl)
	require.NotNil(t, rl.globalLimiter)
	require.NotNil(t, rl.clientLimiter)
	require.Len(t, rl.exemptMethods, 1)
	require.Len(t, rl.exemptClients, 1)
}

func TestRateLimiter_UnaryInterceptor_Disabled(t *testing.T) {
	config := RateLimitConfig{Enabled: false}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	resp, err := interceptor(context.Background(), "request", info, handler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)
	require.True(t, handlerCalled)
}

func TestRateLimiter_UnaryInterceptor_ExemptMethod(t *testing.T) {
	config := RateLimitConfig{
		Enabled:       true,
		GlobalRate:    1,
		PerClientRate: 1,
		Interval:      time.Hour, // Very restrictive
		Burst:         1,
		ExemptMethods: []string{"/blockberry.Node/Health"},
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/Health",
	}

	// Should work many times for exempt method
	for i := 0; i < 100; i++ {
		resp, err := interceptor(context.Background(), "request", info, handler)
		require.NoError(t, err)
		require.Equal(t, "response", resp)
	}
}

func TestRateLimiter_UnaryInterceptor_ExemptClient(t *testing.T) {
	config := RateLimitConfig{
		Enabled:       true,
		GlobalRate:    1,
		PerClientRate: 1,
		Interval:      time.Hour, // Very restrictive
		Burst:         1,
		ExemptClients: []string{"127.0.0.1:12345"},
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Create context with peer info for exempt client
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	// Should work many times for exempt client
	for i := 0; i < 100; i++ {
		resp, err := interceptor(ctx, "request", info, handler)
		require.NoError(t, err)
		require.Equal(t, "response", resp)
	}
}

func TestRateLimiter_UnaryInterceptor_RateLimited(t *testing.T) {
	config := RateLimitConfig{
		Enabled:         true,
		GlobalRate:      1000, // High global rate
		PerClientRate:   2,    // Very low per-client rate
		Interval:        time.Second,
		Burst:           2,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Create context with peer info
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.1:54321")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	// First few requests should succeed (burst)
	for i := 0; i < 2; i++ {
		resp, err := interceptor(ctx, "request", info, handler)
		require.NoError(t, err)
		require.Equal(t, "response", resp)
	}

	// Next request should be rate limited
	_, err := interceptor(ctx, "request", info, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.ResourceExhausted, st.Code())
}

func TestRateLimiter_GlobalRateLimit(t *testing.T) {
	// Configure so global limit is hit before per-client limit
	// Burst = 5 means:
	// - Global burst = 5 * 10 = 50 tokens
	// - Per-client burst = 5 tokens
	// We'll use 10 different clients, each making 5 requests = 50 total
	// This exhausts global limit without any single client exceeding per-client
	config := RateLimitConfig{
		Enabled:         true,
		GlobalRate:      2,    // Very low global rate
		PerClientRate:   1000, // High per-client rate
		Interval:        time.Hour, // Very long interval to prevent refill
		Burst:           5,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Use multiple clients to exhaust global limit
	// 10 clients * 5 requests each = 50 requests = global burst
	for clientNum := 0; clientNum < 10; clientNum++ {
		addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("192.168.1.%d:54321", clientNum+1))
		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

		for i := 0; i < 5; i++ {
			_, err := interceptor(ctx, "request", info, handler)
			require.NoError(t, err, "client %d request %d should succeed", clientNum, i)
		}
	}

	// Any additional request should hit global limit
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.2.1:54321")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	_, err := interceptor(ctx, "request", info, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.ResourceExhausted, st.Code())
	require.Contains(t, st.Message(), "global")
}

func TestRateLimiter_AddRemoveExemptClient(t *testing.T) {
	config := RateLimitConfig{
		Enabled:       true,
		PerClientRate: 1,
		Interval:      time.Hour,
		Burst:         1,
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	clientIP := "10.0.0.1:12345"

	// Not exempt initially
	_, ok := rl.exemptClients[clientIP]
	require.False(t, ok)

	// Add exempt client
	rl.AddExemptClient(clientIP)
	_, ok = rl.exemptClients[clientIP]
	require.True(t, ok)

	// Remove exempt client
	rl.RemoveExemptClient(clientIP)
	_, ok = rl.exemptClients[clientIP]
	require.False(t, ok)
}

func TestRateLimiter_AddRemoveExemptMethod(t *testing.T) {
	config := RateLimitConfig{
		Enabled: true,
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	method := "/custom/method"

	// Not exempt initially
	_, ok := rl.exemptMethods[method]
	require.False(t, ok)

	// Add exempt method
	rl.AddExemptMethod(method)
	_, ok = rl.exemptMethods[method]
	require.True(t, ok)

	// Remove exempt method
	rl.RemoveExemptMethod(method)
	_, ok = rl.exemptMethods[method]
	require.False(t, ok)
}

func TestRateLimiter_Reset(t *testing.T) {
	config := RateLimitConfig{
		Enabled:         true,
		GlobalRate:      1000,
		PerClientRate:   2,
		Interval:        time.Second,
		Burst:           2,
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	clientIP := "192.168.1.100:54321"
	addr, _ := net.ResolveTCPAddr("tcp", clientIP)
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	// Exhaust the rate limit
	for i := 0; i < 2; i++ {
		_, err := interceptor(ctx, "request", info, handler)
		require.NoError(t, err)
	}

	// Should be rate limited
	_, err := interceptor(ctx, "request", info, handler)
	require.Error(t, err)

	// Reset the client's limit
	rl.Reset(clientIP)

	// Should work again
	resp, err := interceptor(ctx, "request", info, handler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)
}

func TestRateLimiter_ResetGlobal(t *testing.T) {
	// Use multiple clients to hit global limit without hitting per-client limit
	config := RateLimitConfig{
		Enabled:         true,
		GlobalRate:      2,
		PerClientRate:   1000, // High per-client rate
		Interval:        time.Hour, // Long interval to prevent refill
		Burst:           5,    // Global gets 50 tokens, per-client gets 5
		CleanupInterval: time.Minute,
	}
	rl := NewRateLimiter(config)
	defer rl.Close()

	interceptor := rl.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Use 10 clients * 5 requests = 50 requests to exhaust global limit
	for clientNum := 0; clientNum < 10; clientNum++ {
		addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("192.168.100.%d:54321", clientNum+1))
		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

		for i := 0; i < 5; i++ {
			_, err := interceptor(ctx, "request", info, handler)
			require.NoError(t, err, "client %d request %d should succeed", clientNum, i)
		}
	}

	// New client should be globally rate limited
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.200.1:54321")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})

	_, err := interceptor(ctx, "request", info, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Contains(t, st.Message(), "global")

	// Reset global limit
	rl.ResetGlobal()

	// Should work again
	resp, err := interceptor(ctx, "request", info, handler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)
}

func TestRateLimiter_GetClientIP(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimitConfig())
	defer rl.Close()

	// No peer info
	ip := rl.getClientIP(context.Background())
	require.Equal(t, "unknown", ip)

	// With peer info
	addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.1:54321")
	ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: addr})
	ip = rl.getClientIP(ctx)
	require.Equal(t, "192.168.1.1:54321", ip)
}
