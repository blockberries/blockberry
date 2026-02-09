package grpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/blockberries/blockberry/pkg/abi"
	"github.com/blockberries/blockberry/pkg/rpc"
	schema "github.com/blockberries/blockberry/schema"
)

// Config contains configuration for the gRPC server.
type Config struct {
	// ListenAddr is the address to listen on (e.g., "0.0.0.0:26658").
	ListenAddr string

	// MaxRecvMsgSize is the maximum message size in bytes the server can receive.
	MaxRecvMsgSize int

	// MaxSendMsgSize is the maximum message size in bytes the server can send.
	MaxSendMsgSize int

	// MaxConcurrentStreams is the maximum number of concurrent streams per connection.
	MaxConcurrentStreams uint32

	// ConnectionTimeout is the timeout for establishing connections.
	ConnectionTimeout time.Duration

	// TLS configuration for secure connections.
	TLS *TLSConfig

	// Auth contains authentication configuration.
	Auth AuthConfig

	// RateLimit contains rate limiting configuration.
	RateLimit RateLimitConfig
}

// TLSConfig contains TLS configuration.
type TLSConfig struct {
	CertFile string
	KeyFile  string
}

// DefaultConfig returns sensible defaults for gRPC server configuration.
func DefaultConfig() Config {
	return Config{
		ListenAddr:           "0.0.0.0:26658",
		MaxRecvMsgSize:       4 * 1024 * 1024,   // 4 MB
		MaxSendMsgSize:       4 * 1024 * 1024,   // 4 MB
		MaxConcurrentStreams: 100,
		ConnectionTimeout:    30 * time.Second,
		Auth:                 DefaultAuthConfig(),
		RateLimit:            DefaultRateLimitConfig(),
	}
}

// Server is a gRPC server that uses Cramberry encoding.
type Server struct {
	rpcServer     rpc.Server
	config        Config
	grpcServer    *grpc.Server
	listener      net.Listener
	running       atomic.Bool
	authenticator *Authenticator
	rateLimiter   *RateLimiter
}

// NewServer creates a new gRPC server.
func NewServer(rpcServer rpc.Server, config Config) *Server {
	return &Server{
		rpcServer:     rpcServer,
		config:        config,
		authenticator: NewAuthenticator(config.Auth),
		rateLimiter:   NewRateLimiter(config.RateLimit),
	}
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	if s.running.Swap(true) {
		return nil // Already running
	}

	// Create listener
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		s.running.Store(false)
		return fmt.Errorf("failed to listen on %s: %w", s.config.ListenAddr, err)
	}
	s.listener = listener

	// Build server options
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.config.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(s.config.MaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			Time:              2 * time.Minute,
			Timeout:           20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             1 * time.Minute,
			PermitWithoutStream: true,
		}),
		// Add authentication and rate limiting interceptors
		grpc.ChainUnaryInterceptor(
			s.rateLimiter.UnaryInterceptor(),
			s.authenticator.UnaryInterceptor(),
		),
		grpc.ChainStreamInterceptor(
			s.rateLimiter.StreamInterceptor(),
			s.authenticator.StreamInterceptor(),
		),
	}

	// Add TLS if configured
	if s.config.TLS != nil {
		creds, err := credentials.NewServerTLSFromFile(s.config.TLS.CertFile, s.config.TLS.KeyFile)
		if err != nil {
			listener.Close() // Close listener to prevent resource leak
			s.running.Store(false)
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	// Create gRPC server
	s.grpcServer = grpc.NewServer(opts...)

	// Register service
	RegisterNodeServiceServer(s.grpcServer, s)

	// Start serving
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil {
			// Log error but don't crash
		}
	}()

	return nil
}

// Stop stops the gRPC server.
func (s *Server) Stop() error {
	if !s.running.Swap(false) {
		return nil // Already stopped
	}

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	if s.rateLimiter != nil {
		s.rateLimiter.Close()
	}

	return nil
}

// IsRunning returns true if the server is running.
func (s *Server) IsRunning() bool {
	return s.running.Load()
}

// NodeServiceServer is the interface implemented by the gRPC server.
type NodeServiceServer interface {
	Health(context.Context, *schema.HealthRequest) (*schema.HealthResponse, error)
	Status(context.Context, *schema.StatusRequest) (*schema.StatusResponse, error)
	NetInfo(context.Context, *schema.NetInfoRequest) (*schema.NetInfoResponse, error)
	BroadcastTx(context.Context, *schema.BroadcastTxRequest) (*schema.BroadcastTxResponse, error)
	Query(context.Context, *schema.QueryRequest) (*schema.QueryResponse, error)
	Block(context.Context, *schema.BlockRequest) (*schema.BlockResponse, error)
	BlockByHash(context.Context, *schema.BlockByHashRequest) (*schema.BlockResponse, error)
	Tx(context.Context, *schema.TxRequest) (*schema.TxResponse, error)
	TxSearch(context.Context, *schema.TxSearchRequest) (*schema.TxSearchResponse, error)
	Peers(context.Context, *schema.PeersRequest) (*schema.PeersResponse, error)
	ConsensusState(context.Context, *schema.ConsensusStateRequest) (*schema.ConsensusStateResponse, error)
	Subscribe(*schema.SubscribeRequest, *subscribeStream) error
	Unsubscribe(context.Context, *schema.UnsubscribeRequest) (*schema.UnsubscribeResponse, error)
	UnsubscribeAll(context.Context, *schema.UnsubscribeAllRequest) (*schema.UnsubscribeAllResponse, error)
}

// RegisterNodeServiceServer registers the node service with the gRPC server.
// This uses a custom registration since we're not using protoc-generated code.
func RegisterNodeServiceServer(s *grpc.Server, srv NodeServiceServer) {
	s.RegisterService(&nodeServiceDesc, srv)
}

// nodeServiceDesc describes the service for registration.
var nodeServiceDesc = grpc.ServiceDesc{
	ServiceName: "blockberry.Node",
	HandlerType: (*NodeServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Health",
			Handler:    healthHandler,
		},
		{
			MethodName: "Status",
			Handler:    statusHandler,
		},
		{
			MethodName: "NetInfo",
			Handler:    netInfoHandler,
		},
		{
			MethodName: "BroadcastTx",
			Handler:    broadcastTxHandler,
		},
		{
			MethodName: "Query",
			Handler:    queryHandler,
		},
		{
			MethodName: "Block",
			Handler:    blockHandler,
		},
		{
			MethodName: "BlockByHash",
			Handler:    blockByHashHandler,
		},
		{
			MethodName: "Tx",
			Handler:    txHandler,
		},
		{
			MethodName: "TxSearch",
			Handler:    txSearchHandler,
		},
		{
			MethodName: "Peers",
			Handler:    peersHandler,
		},
		{
			MethodName: "ConsensusState",
			Handler:    consensusStateHandler,
		},
		{
			MethodName: "Unsubscribe",
			Handler:    unsubscribeHandler,
		},
		{
			MethodName: "UnsubscribeAll",
			Handler:    unsubscribeAllHandler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       subscribeStreamHandler,
			ServerStreams: true,
		},
	},
	Metadata: "blockberry.cram",
}

// Method handlers

func healthHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.HealthRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Health(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Health",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Health(ctx, req.(*schema.HealthRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func statusHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.StatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Status",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Status(ctx, req.(*schema.StatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func netInfoHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.NetInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).NetInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/NetInfo",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).NetInfo(ctx, req.(*schema.NetInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func broadcastTxHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.BroadcastTxRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).BroadcastTx(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/BroadcastTx",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).BroadcastTx(ctx, req.(*schema.BroadcastTxRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func queryHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.QueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Query(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Query",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Query(ctx, req.(*schema.QueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func blockHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.BlockRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Block(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Block",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Block(ctx, req.(*schema.BlockRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func blockByHashHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.BlockByHashRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).BlockByHash(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/BlockByHash",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).BlockByHash(ctx, req.(*schema.BlockByHashRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func txHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.TxRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Tx(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Tx",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Tx(ctx, req.(*schema.TxRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func txSearchHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.TxSearchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).TxSearch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/TxSearch",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).TxSearch(ctx, req.(*schema.TxSearchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func peersHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.PeersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Peers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Peers",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Peers(ctx, req.(*schema.PeersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func consensusStateHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.ConsensusStateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).ConsensusState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/ConsensusState",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).ConsensusState(ctx, req.(*schema.ConsensusStateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func unsubscribeHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.UnsubscribeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).Unsubscribe(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/Unsubscribe",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).Unsubscribe(ctx, req.(*schema.UnsubscribeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func unsubscribeAllHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(schema.UnsubscribeAllRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(*Server).UnsubscribeAll(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blockberry.Node/UnsubscribeAll",
	}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(*Server).UnsubscribeAll(ctx, req.(*schema.UnsubscribeAllRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// subscribeStream is a wrapper for server-side streaming.
type subscribeStream struct {
	grpc.ServerStream
}

func (s *subscribeStream) Send(resp *schema.SubscribeResponse) error {
	return s.ServerStream.SendMsg(resp)
}

func subscribeStreamHandler(srv any, stream grpc.ServerStream) error {
	in := new(schema.SubscribeRequest)
	if err := stream.RecvMsg(in); err != nil {
		return err
	}
	return srv.(*Server).Subscribe(in, &subscribeStream{stream})
}

// Service method implementations

// Health returns health status.
func (s *Server) Health(ctx context.Context, req *schema.HealthRequest) (*schema.HealthResponse, error) {
	health, err := s.rpcServer.Health(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &schema.HealthResponse{
		Status: &health.Status,
	}

	for name, check := range health.Checks {
		checkStatus := check.Status
		checkMsg := check.Message
		checkTime := check.Time.Unix()
		resp.Checks = append(resp.Checks, schema.GrpcHealthCheck{
			Status: &checkStatus,
			Msg:    checkMsg,
			Time:   &checkTime,
		})
		// Note: we lose the check name in this conversion
		_ = name
	}

	return resp, nil
}

// Status returns node status.
func (s *Server) Status(ctx context.Context, req *schema.StatusRequest) (*schema.StatusResponse, error) {
	nodeStatus, err := s.rpcServer.Status(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &schema.StatusResponse{
		NodeInfo: schema.GrpcNodeInfo{
			Id:         &nodeStatus.NodeInfo.ID,
			Moniker:    &nodeStatus.NodeInfo.Moniker,
			Network:    &nodeStatus.NodeInfo.Network,
			Version:    &nodeStatus.NodeInfo.Version,
			ListenAddr: nodeStatus.NodeInfo.ListenAddr,
		},
		SyncInfo: schema.GrpcSyncInfo{
			LatestBlockHash:     nodeStatus.SyncInfo.LatestBlockHash,
			LatestAppHash:       nodeStatus.SyncInfo.LatestAppHash,
			LatestBlockHeight:   ptr(int64(nodeStatus.SyncInfo.LatestBlockHeight)),
			LatestBlockTime:     ptr(nodeStatus.SyncInfo.LatestBlockTime.Unix()),
			EarliestBlockHeight: ptr(int64(nodeStatus.SyncInfo.EarliestBlockHeight)),
			EarliestBlockTime:   ptr(nodeStatus.SyncInfo.EarliestBlockTime.Unix()),
			CatchingUp:          &nodeStatus.SyncInfo.CatchingUp,
		},
	}

	if nodeStatus.ValidatorInfo != nil {
		resp.ValidatorInfo = schema.GrpcValidatorInfo{
			Address:     nodeStatus.ValidatorInfo.Address,
			PublicKey:   nodeStatus.ValidatorInfo.PublicKey,
			VotingPower: &nodeStatus.ValidatorInfo.VotingPower,
		}
	}

	return resp, nil
}

// NetInfo returns network information.
func (s *Server) NetInfo(ctx context.Context, req *schema.NetInfoRequest) (*schema.NetInfoResponse, error) {
	netInfo, err := s.rpcServer.NetInfo(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	numPeers := int32(netInfo.NumPeers)
	resp := &schema.NetInfoResponse{
		Listening: &netInfo.Listening,
		Listeners: netInfo.Listeners,
		NumPeers:  &numPeers,
	}

	for _, peer := range netInfo.Peers {
		peerInfo := schema.GrpcPeerInfo{
			Id:               &peer.ID,
			Address:          &peer.Address,
			IsOutbound:       &peer.IsOutbound,
			ConnectionStatus: &peer.ConnectionStatus,
		}
		if peer.NodeInfo != nil {
			peerInfo.NodeInfo = schema.GrpcNodeInfo{
				Id:         &peer.NodeInfo.ID,
				Moniker:    &peer.NodeInfo.Moniker,
				Network:    &peer.NodeInfo.Network,
				Version:    &peer.NodeInfo.Version,
				ListenAddr: peer.NodeInfo.ListenAddr,
			}
		}
		resp.Peers = append(resp.Peers, peerInfo)
	}

	return resp, nil
}

// BroadcastTx broadcasts a transaction.
func (s *Server) BroadcastTx(ctx context.Context, req *schema.BroadcastTxRequest) (*schema.BroadcastTxResponse, error) {
	if req.Tx == nil {
		return nil, status.Error(codes.InvalidArgument, "tx is required")
	}
	if req.Mode == nil {
		return nil, status.Error(codes.InvalidArgument, "mode is required")
	}

	mode := rpc.BroadcastMode(*req.Mode)
	result, err := s.rpcServer.BroadcastTx(ctx, req.Tx, mode)
	if err != nil {
		return nil, toGRPCError(err)
	}

	code := int32(result.Code)
	resp := &schema.BroadcastTxResponse{
		Code: &code,
		Hash: result.Hash,
		Log:  result.Log,
		Data: result.Data,
	}
	if result.Height > 0 {
		height := int64(result.Height)
		resp.Height = height
	}

	return resp, nil
}

// Query performs an application query.
func (s *Server) Query(ctx context.Context, req *schema.QueryRequest) (*schema.QueryResponse, error) {
	if req.Path == nil {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	height := int64(0)
	if req.Height != 0 {
		height = int64(req.Height)
	}

	result, err := s.rpcServer.Query(ctx, *req.Path, req.Data, height, req.Prove)
	if err != nil {
		return nil, toGRPCError(err)
	}

	code := int32(result.Code)
	resultHeight := int64(result.Height)
	resp := &schema.QueryResponse{
		Code:   &code,
		Value:  result.Data,
		Height: resultHeight,
	}
	if result.Log != "" {
		resp.Log = result.Log
	}
	// Note: Proof serialization not implemented - would need custom serialization

	return resp, nil
}

// Block returns a block by height.
func (s *Server) Block(ctx context.Context, req *schema.BlockRequest) (*schema.BlockResponse, error) {
	if req.Height == nil {
		return nil, status.Error(codes.InvalidArgument, "height is required")
	}

	result, err := s.rpcServer.Block(ctx, int64(*req.Height))
	if err != nil {
		return nil, toGRPCError(err)
	}

	return blockResultToResponse(result), nil
}

// BlockByHash returns a block by hash.
func (s *Server) BlockByHash(ctx context.Context, req *schema.BlockByHashRequest) (*schema.BlockResponse, error) {
	if req.Hash == nil {
		return nil, status.Error(codes.InvalidArgument, "hash is required")
	}

	result, err := s.rpcServer.BlockByHash(ctx, req.Hash)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return blockResultToResponse(result), nil
}

// Tx returns a transaction by hash.
func (s *Server) Tx(ctx context.Context, req *schema.TxRequest) (*schema.TxResponse, error) {
	if req.Hash == nil {
		return nil, status.Error(codes.InvalidArgument, "hash is required")
	}

	result, err := s.rpcServer.Tx(ctx, req.Hash)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &schema.TxResponse{
		Tx: txResultToRPC(result),
	}, nil
}

// TxSearch searches for transactions.
func (s *Server) TxSearch(ctx context.Context, req *schema.TxSearchRequest) (*schema.TxSearchResponse, error) {
	if req.Query == nil {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	page := int(*req.Page)
	perPage := int(*req.PerPage)
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 30
	}

	results, total, err := s.rpcServer.TxSearch(ctx, *req.Query, page, perPage)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &schema.TxSearchResponse{
		Total:      ptr(int32(total)),
		Page:       ptr(int32(page)),
		PerPage:    ptr(int32(perPage)),
		TotalPages: ptr(int32((total + perPage - 1) / perPage)),
	}

	for _, result := range results {
		resp.Txs = append(resp.Txs, txResultToRPC(result))
	}

	return resp, nil
}

// Peers returns connected peer information.
func (s *Server) Peers(ctx context.Context, req *schema.PeersRequest) (*schema.PeersResponse, error) {
	peers, err := s.rpcServer.Peers(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &schema.PeersResponse{}
	for _, peer := range peers {
		peerInfo := schema.GrpcPeerInfo{
			Id:               &peer.ID,
			Address:          &peer.Address,
			IsOutbound:       &peer.IsOutbound,
			ConnectionStatus: &peer.ConnectionStatus,
		}
		if peer.NodeInfo != nil {
			peerInfo.NodeInfo = schema.GrpcNodeInfo{
				Id:         &peer.NodeInfo.ID,
				Moniker:    &peer.NodeInfo.Moniker,
				Network:    &peer.NodeInfo.Network,
				Version:    &peer.NodeInfo.Version,
				ListenAddr: peer.NodeInfo.ListenAddr,
			}
		}
		resp.Peers = append(resp.Peers, peerInfo)
	}

	return resp, nil
}

// ConsensusState returns consensus state.
func (s *Server) ConsensusState(ctx context.Context, req *schema.ConsensusStateRequest) (*schema.ConsensusStateResponse, error) {
	state, err := s.rpcServer.ConsensusState(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &schema.ConsensusStateResponse{
		Height:    ptr(int64(state.Height)),
		Round:     ptr(int32(state.Round)),
		Step:      &state.Step,
		StartTime: ptr(state.StartTime.Unix()),
	}

	for _, v := range state.Validators {
		resp.Validators = append(resp.Validators, schema.GrpcValidatorInfo{
			Address:     v.Address,
			PublicKey:   v.PublicKey,
			VotingPower: &v.VotingPower,
		})
	}

	return resp, nil
}

// Subscribe subscribes to events (streaming).
func (s *Server) Subscribe(req *schema.SubscribeRequest, stream *subscribeStream) error {
	if req.Subscriber == nil || req.Query == nil {
		return status.Error(codes.InvalidArgument, "subscriber and query are required")
	}

	ctx := stream.Context()
	query := parseQuery(*req.Query)

	events, err := s.rpcServer.Subscribe(ctx, *req.Subscriber, query)
	if err != nil {
		return toGRPCError(err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			// Convert event to response
			eventType := event.Type
			height := int64(0)
			// Try to extract height from attributes
			for _, attr := range event.Attributes {
				if attr.Key == "height" {
					// Parse height from value
					_ = attr // Height extraction would need proper parsing
				}
			}

			resp := &schema.SubscribeResponse{
				Event: schema.EventMessage{
					Type:   &eventType,
					Data:   nil, // Events don't have raw data in our model
					Height: &height,
				},
			}
			if err := stream.Send(resp); err != nil {
				return err
			}
		}
	}
}

// Unsubscribe unsubscribes from events.
func (s *Server) Unsubscribe(ctx context.Context, req *schema.UnsubscribeRequest) (*schema.UnsubscribeResponse, error) {
	if req.Subscriber == nil || req.Query == nil {
		return nil, status.Error(codes.InvalidArgument, "subscriber and query are required")
	}

	query := parseQuery(*req.Query)
	if err := s.rpcServer.Unsubscribe(ctx, *req.Subscriber, query); err != nil {
		return nil, toGRPCError(err)
	}

	success := true
	return &schema.UnsubscribeResponse{Success: &success}, nil
}

// UnsubscribeAll unsubscribes from all events.
func (s *Server) UnsubscribeAll(ctx context.Context, req *schema.UnsubscribeAllRequest) (*schema.UnsubscribeAllResponse, error) {
	if req.Subscriber == nil {
		return nil, status.Error(codes.InvalidArgument, "subscriber is required")
	}

	if err := s.rpcServer.UnsubscribeAll(ctx, *req.Subscriber); err != nil {
		return nil, toGRPCError(err)
	}

	success := true
	return &schema.UnsubscribeAllResponse{Success: &success}, nil
}

// Helper functions

func ptr[T any](v T) *T {
	return &v
}

func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	if rpcErr, ok := err.(*rpc.RPCError); ok {
		switch rpcErr.Code {
		case -32600:
			return status.Error(codes.InvalidArgument, rpcErr.Message)
		case -32601:
			return status.Error(codes.Unimplemented, rpcErr.Message)
		case -32602:
			return status.Error(codes.InvalidArgument, rpcErr.Message)
		case -32000, -32001: // Not found errors
			return status.Error(codes.NotFound, rpcErr.Message)
		default:
			return status.Error(codes.Internal, rpcErr.Message)
		}
	}

	return status.Error(codes.Internal, err.Error())
}

func blockResultToResponse(result *rpc.BlockResult) *schema.BlockResponse {
	if result == nil || result.Block == nil {
		return &schema.BlockResponse{}
	}

	block := result.Block
	grpcBlock := schema.GrpcBlock{
		Height:        ptr(int64(block.Header.Height)),
		Hash:          result.BlockID.Hash,
		Time:          ptr(block.Header.Time.Unix()),
		LastBlockHash: block.Header.LastBlockHash,
		// DataHash, ValidatorsHash, AppHash not in BlockHeader - use empty
	}

	// Add transactions
	for _, tx := range block.Txs {
		if tx != nil {
			grpcBlock.Txs = append(grpcBlock.Txs, tx)
		}
	}

	return &schema.BlockResponse{
		Block: grpcBlock,
		BlockId: schema.GrpcBlockId{
			Hash:       result.BlockID.Hash,
			PartsTotal: ptr(int32(result.BlockID.PartSetHeader.Total)),
			PartsHash:  result.BlockID.PartSetHeader.Hash,
		},
	}
}

func txResultToRPC(result *rpc.TxResult) schema.GrpcTxResult {
	if result == nil {
		return schema.GrpcTxResult{}
	}

	rpcResult := schema.GrpcTxResult{
		Hash:   result.Hash,
		Height: ptr(int64(result.Height)),
		Index:  ptr(int32(result.Index)),
	}

	if result.Result != nil {
		rpcResult.Code = ptr(int32(result.Result.Code))
		if result.Result.Log != "" {
			rpcResult.Log = result.Result.Log
		}
		rpcResult.Data = result.Result.Data
		rpcResult.GasWanted = ptr(result.Result.GasWanted)
		rpcResult.GasUsed = ptr(result.Result.GasUsed)
	}

	return rpcResult
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
