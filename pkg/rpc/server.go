package rpc

import (
	"context"

	"github.com/blockberries/blockberry/pkg/abi"
	"github.com/blockberries/blockberry/pkg/types"
)

// Server is the RPC server interface.
// It provides methods for querying node status, broadcasting transactions,
// and subscribing to events.
type Server interface {
	types.Component

	// Health returns the health status of the node.
	Health(ctx context.Context) (*HealthStatus, error)

	// Status returns the current status of the node.
	Status(ctx context.Context) (*NodeStatus, error)

	// NetInfo returns network information.
	NetInfo(ctx context.Context) (*NetInfo, error)

	// BroadcastTx broadcasts a transaction.
	BroadcastTx(ctx context.Context, tx []byte, mode BroadcastMode) (*BroadcastResult, error)

	// Query performs an application query.
	Query(ctx context.Context, path string, data []byte, height int64, prove bool) (*abi.QueryResult, error)

	// Block returns a block by height.
	// Use height 0 for the latest block.
	Block(ctx context.Context, height int64) (*BlockResult, error)

	// BlockByHash returns a block by hash.
	BlockByHash(ctx context.Context, hash []byte) (*BlockResult, error)

	// Tx returns a transaction by hash.
	Tx(ctx context.Context, hash []byte) (*TxResult, error)

	// TxSearch searches for transactions matching a query.
	TxSearch(ctx context.Context, query string, page, perPage int) ([]*TxResult, int, error)

	// Peers returns information about connected peers.
	Peers(ctx context.Context) ([]PeerInfo, error)

	// ConsensusState returns the current consensus state.
	ConsensusState(ctx context.Context) (*ConsensusState, error)

	// Subscribe subscribes to events matching the query.
	// Returns a channel that receives events.
	Subscribe(ctx context.Context, subscriber string, query abi.Query) (<-chan abi.Event, error)

	// Unsubscribe unsubscribes from events.
	Unsubscribe(ctx context.Context, subscriber string, query abi.Query) error

	// UnsubscribeAll unsubscribes from all events.
	UnsubscribeAll(ctx context.Context, subscriber string) error
}

// Handler handles RPC method calls.
// This is implemented by transport-specific handlers (JSON-RPC, gRPC, etc.).
type Handler interface {
	// ServeRPC handles an RPC request and returns the response.
	ServeRPC(ctx context.Context, method string, params []byte) ([]byte, error)
}

// ServerConfig contains configuration for the RPC server.
type ServerConfig struct {
	// ListenAddr is the address to listen on (e.g., "tcp://0.0.0.0:26657").
	ListenAddr string

	// MaxOpenConnections is the maximum number of simultaneous connections.
	MaxOpenConnections int

	// MaxSubscriptionsPerClient is the max subscriptions per client.
	MaxSubscriptionsPerClient int

	// SubscriptionBufferSize is the buffer size for subscription channels.
	SubscriptionBufferSize int

	// TimeoutBroadcast is the timeout for broadcast operations.
	TimeoutBroadcastTxCommit int64 // milliseconds

	// MaxBodyBytes is the maximum size of request body.
	MaxBodyBytes int64

	// MaxHeaderBytes is the maximum size of request headers.
	MaxHeaderBytes int

	// TLS enables TLS.
	TLS *TLSConfig
}

// TLSConfig contains TLS configuration.
type TLSConfig struct {
	// CertFile is the path to the certificate file.
	CertFile string

	// KeyFile is the path to the key file.
	KeyFile string
}

// DefaultServerConfig returns sensible defaults for RPC server configuration.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ListenAddr:                "tcp://0.0.0.0:26657",
		MaxOpenConnections:        900,
		MaxSubscriptionsPerClient: 5,
		SubscriptionBufferSize:    100,
		TimeoutBroadcastTxCommit:  10000, // 10 seconds
		MaxBodyBytes:              1000000,
		MaxHeaderBytes:            1 << 20, // 1MB
	}
}

// Common RPC errors.
var (
	ErrInvalidRequest    = &RPCError{Code: -32600, Message: "Invalid Request"}
	ErrMethodNotFound    = &RPCError{Code: -32601, Message: "Method not found"}
	ErrInvalidParams     = &RPCError{Code: -32602, Message: "Invalid params"}
	ErrInternalError     = &RPCError{Code: -32603, Message: "Internal error"}
	ErrParseError        = &RPCError{Code: -32700, Message: "Parse error"}
	ErrTxNotFound        = &RPCError{Code: -32000, Message: "Transaction not found"}
	ErrBlockNotFound     = &RPCError{Code: -32001, Message: "Block not found"}
	ErrInvalidHeight     = &RPCError{Code: -32002, Message: "Invalid height"}
	ErrTxBroadcastFailed = &RPCError{Code: -32003, Message: "Transaction broadcast failed"}
)

// RPCError represents an RPC error.
type RPCError struct {
	// Code is the error code.
	Code int

	// Message is the error message.
	Message string

	// Data contains additional error data.
	Data interface{}
}

// Error implements the error interface.
func (e *RPCError) Error() string {
	return e.Message
}

// WithData returns a copy of the error with additional data.
func (e *RPCError) WithData(data interface{}) *RPCError {
	return &RPCError{
		Code:    e.Code,
		Message: e.Message,
		Data:    data,
	}
}
