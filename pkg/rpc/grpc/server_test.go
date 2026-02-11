package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	bapitypes "github.com/blockberries/bapi/types"
	"github.com/blockberries/blockberry/pkg/events"
	"github.com/blockberries/blockberry/pkg/rpc"
	schema "github.com/blockberries/blockberry/schema"
)

// mockRPCServer implements rpc.Server for testing.
type mockRPCServer struct {
	healthResult     *rpc.HealthStatus
	statusResult     *rpc.NodeStatus
	netInfoResult    *rpc.NetInfo
	broadcastResult  *rpc.BroadcastResult
	queryResult      *bapitypes.StateQueryResult
	blockResult      *rpc.BlockResult
	txResult         *rpc.TxResult
	txSearchResults  []*rpc.TxResult
	peersResult      []rpc.PeerInfo
	consensusResult  *rpc.ConsensusState
	subscribeChannel chan bapitypes.Event
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

func (m *mockRPCServer) Query(ctx context.Context, path string, data []byte, height int64, prove bool) (*bapitypes.StateQueryResult, error) {
	return m.queryResult, m.err
}

func (m *mockRPCServer) Block(ctx context.Context, height int64) (*rpc.BlockResult, error) {
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

func (m *mockRPCServer) Subscribe(ctx context.Context, subscriber string, query events.Query) (<-chan bapitypes.Event, error) {
	return m.subscribeChannel, m.err
}

func (m *mockRPCServer) Unsubscribe(ctx context.Context, subscriber string, query events.Query) error {
	return m.err
}

func (m *mockRPCServer) UnsubscribeAll(ctx context.Context, subscriber string) error {
	return m.err
}

func TestNewServer(t *testing.T) {
	mock := &mockRPCServer{}
	config := DefaultConfig()

	server := NewServer(mock, config)

	require.NotNil(t, server)
	require.Equal(t, config.ListenAddr, server.config.ListenAddr)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	require.Equal(t, "0.0.0.0:26658", config.ListenAddr)
	require.Equal(t, 4*1024*1024, config.MaxRecvMsgSize)
	require.Equal(t, 4*1024*1024, config.MaxSendMsgSize)
	require.Equal(t, uint32(100), config.MaxConcurrentStreams)
}

func TestCramberryCodec(t *testing.T) {
	codec := NewCramberryCodec()
	require.Equal(t, CodecName, codec.Name())

	// Test marshaling and unmarshaling
	original := &schema.HealthRequest{}
	data, err := codec.Marshal(original)
	require.NoError(t, err)
	require.NotNil(t, data)

	var unmarshaled schema.HealthRequest
	err = codec.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
}

func TestCramberryCodec_MarshalNil(t *testing.T) {
	codec := NewCramberryCodec()

	data, err := codec.Marshal(nil)
	require.NoError(t, err)
	require.Nil(t, data)
}

func TestCramberryCodec_UnmarshalEmpty(t *testing.T) {
	codec := NewCramberryCodec()

	var msg schema.HealthRequest
	err := codec.Unmarshal([]byte{}, &msg)
	require.NoError(t, err)
}

func TestServerStartStop(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	mock := &mockRPCServer{
		healthResult: &rpc.HealthStatus{
			Status: "healthy",
		},
	}
	config := DefaultConfig()
	config.ListenAddr = listener.Addr().String()

	server := NewServer(mock, config)

	// Start
	err = server.Start()
	require.NoError(t, err)
	require.True(t, server.IsRunning())

	// Starting again should be no-op
	err = server.Start()
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Stop
	err = server.Stop()
	require.NoError(t, err)
	require.False(t, server.IsRunning())

	// Stopping again should be no-op
	err = server.Stop()
	require.NoError(t, err)

	_ = port // Used to construct address
}

func TestHealth(t *testing.T) {
	mock := &mockRPCServer{
		healthResult: &rpc.HealthStatus{
			Status: "healthy",
			Checks: map[string]rpc.HealthCheck{
				"test": {
					Status:  "pass",
					Message: "All good",
					Time:    time.Now(),
				},
			},
		},
	}

	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	resp, err := server.Health(ctx, &schema.HealthRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "healthy", *resp.Status)
	require.Len(t, resp.Checks, 1)
}

func TestStatus(t *testing.T) {
	mock := &mockRPCServer{
		statusResult: &rpc.NodeStatus{
			NodeInfo: rpc.NodeInfo{
				ID:      "node123",
				Moniker: "test-node",
				Network: "test-chain",
				Version: "1.0.0",
			},
			SyncInfo: rpc.SyncInfo{
				LatestBlockHeight: 100,
				CatchingUp:        false,
			},
		},
	}

	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	resp, err := server.Status(ctx, &schema.StatusRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "node123", *resp.NodeInfo.Id)
	require.Equal(t, "test-node", *resp.NodeInfo.Moniker)
	require.Equal(t, int64(100), *resp.SyncInfo.LatestBlockHeight)
	require.False(t, *resp.SyncInfo.CatchingUp)
}

func TestBroadcastTx(t *testing.T) {
	mock := &mockRPCServer{
		broadcastResult: &rpc.BroadcastResult{
			Code: 0,
			Hash: []byte("txhash123"),
			Log:  "success",
		},
	}

	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	tx := []byte("test transaction")
	mode := int32(rpc.BroadcastSync)
	resp, err := server.BroadcastTx(ctx, &schema.BroadcastTxRequest{
		Tx:   tx,
		Mode: &mode,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int32(0), *resp.Code)
	require.Equal(t, []byte("txhash123"), resp.Hash)
}

func TestBroadcastTx_MissingTx(t *testing.T) {
	mock := &mockRPCServer{}
	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	mode := int32(rpc.BroadcastSync)
	_, err := server.BroadcastTx(ctx, &schema.BroadcastTxRequest{
		Mode: &mode,
	})
	require.Error(t, err)
}

func TestQuery(t *testing.T) {
	mock := &mockRPCServer{
		queryResult: &bapitypes.StateQueryResult{
			Code:   0,
			Value:  []byte("result value"),
			Height: 50,
		},
	}

	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	path := "/store/key"
	resp, err := server.Query(ctx, &schema.QueryRequest{
		Path: &path,
		Data: []byte("query data"),
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int32(0), *resp.Code)
	require.Equal(t, []byte("result value"), resp.Value)
	require.Equal(t, int64(50), resp.Height)
}

func TestBlock(t *testing.T) {
	mock := &mockRPCServer{
		blockResult: &rpc.BlockResult{
			Block: &rpc.Block{
				Header: rpc.BlockHeader{
					Height: 100,
					Time:   time.Now(),
				},
			},
			BlockID: rpc.BlockID{
				Hash: []byte("blockhash"),
			},
		},
	}

	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	height := int64(100)
	resp, err := server.Block(ctx, &schema.BlockRequest{
		Height: &height,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(100), *resp.Block.Height)
	require.Equal(t, []byte("blockhash"), resp.BlockId.Hash)
}

func TestPeers(t *testing.T) {
	mock := &mockRPCServer{
		peersResult: []rpc.PeerInfo{
			{
				ID:               "peer1",
				Address:          "/ip4/127.0.0.1/tcp/26656",
				IsOutbound:       true,
				ConnectionStatus: "connected",
			},
			{
				ID:               "peer2",
				Address:          "/ip4/127.0.0.2/tcp/26656",
				IsOutbound:       false,
				ConnectionStatus: "connected",
			},
		},
	}

	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	resp, err := server.Peers(ctx, &schema.PeersRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Peers, 2)
	require.Equal(t, "peer1", *resp.Peers[0].Id)
	require.Equal(t, "peer2", *resp.Peers[1].Id)
}

func TestUnsubscribe(t *testing.T) {
	mock := &mockRPCServer{}
	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	subscriber := "test-sub"
	query := "type=Transfer"
	resp, err := server.Unsubscribe(ctx, &schema.UnsubscribeRequest{
		Subscriber: &subscriber,
		Query:      &query,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, *resp.Success)
}

func TestUnsubscribeAll(t *testing.T) {
	mock := &mockRPCServer{}
	server := NewServer(mock, DefaultConfig())
	ctx := context.Background()

	subscriber := "test-sub"
	resp, err := server.UnsubscribeAll(ctx, &schema.UnsubscribeAllRequest{
		Subscriber: &subscriber,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, *resp.Success)
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "all"},
		{"all", "all"},
		{"*", "all"},
		{"type=Transfer", "type"},
		{"sender=abc123", "attribute"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			query := parseQuery(tt.input)
			switch tt.expected {
			case "all":
				_, ok := query.(events.QueryAll)
				require.True(t, ok)
			case "type":
				q, ok := query.(events.QueryEventKind)
				require.True(t, ok)
				require.Equal(t, "Transfer", q.Kind)
			case "attribute":
				q, ok := query.(events.QueryAttribute)
				require.True(t, ok)
				require.Equal(t, "sender", q.Key)
				require.Equal(t, "abc123", q.Value)
			}
		})
	}
}

func TestGRPCIntegration(t *testing.T) {
	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	mock := &mockRPCServer{
		healthResult: &rpc.HealthStatus{
			Status: "healthy",
		},
	}

	config := DefaultConfig()
	config.ListenAddr = addr

	server := NewServer(mock, config)
	err = server.Start()
	require.NoError(t, err)
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client connection with Cramberry codec
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(NewCramberryCodec())),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Make a Health call using raw invoke
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &schema.HealthRequest{}
	resp := &schema.HealthResponse{}

	err = conn.Invoke(ctx, "/blockberry.Node/Health", req, resp)
	require.NoError(t, err)
	require.Equal(t, "healthy", *resp.Status)
}
