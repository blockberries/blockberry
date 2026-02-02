package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestDefaultAuthConfig(t *testing.T) {
	config := DefaultAuthConfig()

	require.False(t, config.Enabled)
	require.Empty(t, config.APIKeys)
	require.Contains(t, config.PublicMethods, "/blockberry.Node/Health")
	require.Contains(t, config.PublicMethods, "/blockberry.Node/Status")
}

func TestNewAuthenticator(t *testing.T) {
	config := AuthConfig{
		Enabled:       true,
		APIKeys:       []string{"key1", "key2"},
		PublicMethods: []string{"/public/method"},
	}

	auth := NewAuthenticator(config)

	require.NotNil(t, auth)
	require.True(t, auth.config.Enabled)
	require.Len(t, auth.apiKeySet, 2)
	require.Len(t, auth.publicMethods, 1)
}

func TestAuthenticator_ValidateKey(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		APIKeys: []string{"valid-key-123", "another-valid-key"},
	}
	auth := NewAuthenticator(config)

	// Test valid keys
	require.True(t, auth.validateKey("valid-key-123"))
	require.True(t, auth.validateKey("another-valid-key"))

	// Test invalid keys
	require.False(t, auth.validateKey("invalid-key"))
	require.False(t, auth.validateKey(""))
	require.False(t, auth.validateKey("valid-key-12")) // Substring
}

func TestAuthenticator_IsPublicMethod(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		PublicMethods: []string{
			"/blockberry.Node/Health",
			"/blockberry.Node/Status",
		},
	}
	auth := NewAuthenticator(config)

	require.True(t, auth.isPublicMethod("/blockberry.Node/Health"))
	require.True(t, auth.isPublicMethod("/blockberry.Node/Status"))
	require.False(t, auth.isPublicMethod("/blockberry.Node/BroadcastTx"))
	require.False(t, auth.isPublicMethod("/other.Service/Method"))
}

func TestAuthenticator_AddRemoveAPIKey(t *testing.T) {
	auth := NewAuthenticator(DefaultAuthConfig())

	// Initially no keys
	require.False(t, auth.validateKey("new-key"))

	// Add key
	auth.AddAPIKey("new-key")
	require.True(t, auth.validateKey("new-key"))

	// Remove key
	auth.RemoveAPIKey("new-key")
	require.False(t, auth.validateKey("new-key"))
}

func TestAuthenticator_SetPublicMethod(t *testing.T) {
	auth := NewAuthenticator(DefaultAuthConfig())

	// Add public method
	auth.SetPublicMethod("/custom/method", true)
	require.True(t, auth.isPublicMethod("/custom/method"))

	// Remove public method
	auth.SetPublicMethod("/custom/method", false)
	require.False(t, auth.isPublicMethod("/custom/method"))
}

func TestAuthenticator_UnaryInterceptor_Disabled(t *testing.T) {
	config := AuthConfig{Enabled: false}
	auth := NewAuthenticator(config)

	interceptor := auth.UnaryInterceptor()

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

func TestAuthenticator_UnaryInterceptor_PublicMethod(t *testing.T) {
	config := AuthConfig{
		Enabled:       true,
		PublicMethods: []string{"/blockberry.Node/Health"},
	}
	auth := NewAuthenticator(config)

	interceptor := auth.UnaryInterceptor()

	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/Health",
	}

	// Public method should work without credentials
	resp, err := interceptor(context.Background(), "request", info, handler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)
	require.True(t, handlerCalled)
}

func TestAuthenticator_UnaryInterceptor_MissingAuth(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		APIKeys: []string{"valid-key"},
	}
	auth := NewAuthenticator(config)

	interceptor := auth.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// No metadata - should fail
	_, err := interceptor(context.Background(), "request", info, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthenticator_UnaryInterceptor_BearerToken(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		APIKeys: []string{"valid-key"},
	}
	auth := NewAuthenticator(config)

	interceptor := auth.UnaryInterceptor()

	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Valid Bearer token
	md := metadata.Pairs("authorization", "Bearer valid-key")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "request", info, handler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)
	require.True(t, handlerCalled)
}

func TestAuthenticator_UnaryInterceptor_APIKeyHeader(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		APIKeys: []string{"valid-api-key"},
	}
	auth := NewAuthenticator(config)

	interceptor := auth.UnaryInterceptor()

	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Valid x-api-key header
	md := metadata.Pairs("x-api-key", "valid-api-key")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "request", info, handler)
	require.NoError(t, err)
	require.Equal(t, "response", resp)
	require.True(t, handlerCalled)
}

func TestAuthenticator_UnaryInterceptor_InvalidKey(t *testing.T) {
	config := AuthConfig{
		Enabled: true,
		APIKeys: []string{"valid-key"},
	}
	auth := NewAuthenticator(config)

	interceptor := auth.UnaryInterceptor()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/blockberry.Node/BroadcastTx",
	}

	// Invalid key
	md := metadata.Pairs("authorization", "Bearer invalid-key")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, "request", info, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.Unauthenticated, st.Code())
}

func TestGetAuthInfo(t *testing.T) {
	// No auth info in context
	ctx := context.Background()
	info := GetAuthInfo(ctx)
	require.False(t, info.Authenticated)
	require.Equal(t, "none", info.Method)

	// With auth info in context
	authInfo := &AuthInfo{Authenticated: true, Method: "api_key"}
	ctx = context.WithValue(context.Background(), authContextKey{}, authInfo)
	info = GetAuthInfo(ctx)
	require.True(t, info.Authenticated)
	require.Equal(t, "api_key", info.Method)
}
