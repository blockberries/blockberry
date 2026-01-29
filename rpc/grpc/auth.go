package grpc

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthConfig contains authentication configuration.
type AuthConfig struct {
	// Enabled controls whether authentication is required.
	Enabled bool

	// APIKeys is a list of valid API keys for key-based authentication.
	// Keys should be stored hashed in production.
	APIKeys []string

	// PublicMethods are methods that don't require authentication.
	// Format: "/service.Name/MethodName" (e.g., "/blockberry.Node/Health")
	PublicMethods []string
}

// DefaultAuthConfig returns authentication configuration with auth disabled.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Enabled: false,
		PublicMethods: []string{
			"/blockberry.Node/Health",
			"/blockberry.Node/Status",
		},
	}
}

// Authenticator handles authentication for gRPC requests.
type Authenticator struct {
	config        AuthConfig
	apiKeySet     map[string]struct{}
	publicMethods map[string]struct{}
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(config AuthConfig) *Authenticator {
	auth := &Authenticator{
		config:        config,
		apiKeySet:     make(map[string]struct{}),
		publicMethods: make(map[string]struct{}),
	}

	for _, key := range config.APIKeys {
		auth.apiKeySet[key] = struct{}{}
	}

	for _, method := range config.PublicMethods {
		auth.publicMethods[method] = struct{}{}
	}

	return auth
}

// UnaryInterceptor returns a unary server interceptor for authentication.
func (a *Authenticator) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !a.config.Enabled {
			return handler(ctx, req)
		}

		// Check if method is public
		if a.isPublicMethod(info.FullMethod) {
			return handler(ctx, req)
		}

		// Validate authentication
		if err := a.authenticate(ctx); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}
}

// StreamInterceptor returns a stream server interceptor for authentication.
func (a *Authenticator) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !a.config.Enabled {
			return handler(srv, ss)
		}

		// Check if method is public
		if a.isPublicMethod(info.FullMethod) {
			return handler(srv, ss)
		}

		// Validate authentication
		if err := a.authenticate(ss.Context()); err != nil {
			return err
		}

		return handler(srv, ss)
	}
}

// isPublicMethod checks if a method doesn't require authentication.
func (a *Authenticator) isPublicMethod(method string) bool {
	_, ok := a.publicMethods[method]
	return ok
}

// authenticate validates the request credentials.
func (a *Authenticator) authenticate(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Try API key authentication
	if err := a.validateAPIKey(md); err == nil {
		return nil
	}

	return status.Error(codes.Unauthenticated, "invalid or missing credentials")
}

// validateAPIKey validates an API key from metadata.
func (a *Authenticator) validateAPIKey(md metadata.MD) error {
	// Check for Authorization header with Bearer scheme
	authHeaders := md.Get("authorization")
	if len(authHeaders) > 0 {
		auth := authHeaders[0]
		if strings.HasPrefix(auth, "Bearer ") {
			key := strings.TrimPrefix(auth, "Bearer ")
			if a.validateKey(key) {
				return nil
			}
		}
	}

	// Check for x-api-key header
	apiKeyHeaders := md.Get("x-api-key")
	if len(apiKeyHeaders) > 0 {
		if a.validateKey(apiKeyHeaders[0]) {
			return nil
		}
	}

	return fmt.Errorf("invalid API key")
}

// validateKey checks if a key is valid using constant-time comparison.
func (a *Authenticator) validateKey(key string) bool {
	for validKey := range a.apiKeySet {
		if subtle.ConstantTimeCompare([]byte(key), []byte(validKey)) == 1 {
			return true
		}
	}
	return false
}

// AddAPIKey adds an API key at runtime.
func (a *Authenticator) AddAPIKey(key string) {
	a.apiKeySet[key] = struct{}{}
}

// RemoveAPIKey removes an API key at runtime.
func (a *Authenticator) RemoveAPIKey(key string) {
	delete(a.apiKeySet, key)
}

// SetPublicMethod adds a method to the public (no-auth) list.
func (a *Authenticator) SetPublicMethod(method string, public bool) {
	if public {
		a.publicMethods[method] = struct{}{}
	} else {
		delete(a.publicMethods, method)
	}
}

// authContextKey is the key for storing authentication info in context.
type authContextKey struct{}

// AuthInfo contains authenticated user information.
type AuthInfo struct {
	// Authenticated indicates if the request was authenticated.
	Authenticated bool

	// Method indicates the authentication method used.
	Method string // "api_key", "none"
}

// GetAuthInfo retrieves authentication info from context.
func GetAuthInfo(ctx context.Context) *AuthInfo {
	if info, ok := ctx.Value(authContextKey{}).(*AuthInfo); ok {
		return info
	}
	return &AuthInfo{Authenticated: false, Method: "none"}
}
