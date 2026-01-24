package pex

import (
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	schema "github.com/blockberries/blockberry/schema"
)

func TestNewReactor(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(
		true,
		30*time.Second,
		100,
		ab,
		nil, // No network for unit tests
		nil, // No peer manager for unit tests
		40,
		10,
	)

	require.NotNil(t, reactor)
	require.True(t, reactor.enabled)
	require.Equal(t, 30*time.Second, reactor.requestInterval)
	require.Equal(t, 100, reactor.maxAddresses)
	require.Equal(t, ab, reactor.addressBook)
	require.Equal(t, 40, reactor.maxInbound)
	require.Equal(t, 10, reactor.maxOutbound)
}

func TestReactor_StartStop(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(
		true,
		100*time.Millisecond,
		100,
		ab,
		nil,
		nil,
		40,
		10,
	)

	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.IsRunning())

	err = reactor.Stop()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())
}

func TestReactor_StartDisabled(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(
		false, // Disabled
		100*time.Millisecond,
		100,
		ab,
		nil,
		nil,
		40,
		10,
	)

	err := reactor.Start()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())
}

func TestReactor_HandleMessage_EmptyData(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("test-peer")
	err := reactor.HandleMessage(peerID, []byte{})
	require.Error(t, err)
}

func TestReactor_HandleMessage_InvalidTypeID(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("test-peer")

	w := cramberry.GetWriter()
	w.WriteTypeID(999)
	w.WriteRawBytes([]byte{0x00})
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peerID, data)
	require.Error(t, err)
}

func TestReactor_HandleAddressRequest(t *testing.T) {
	ab := NewAddressBook("")
	ab.AddPeer(peer.ID("peer-1"), "/ip4/127.0.0.1/tcp/26656", "peer-1", 50)
	ab.AddPeer(peer.ID("peer-2"), "/ip4/127.0.0.2/tcp/26656", "peer-2", 60)

	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("requester")
	lastSeen := time.Now().Add(-24 * time.Hour).Unix()

	req := &schema.AddressRequest{
		LastSeen: &lastSeen,
	}

	msgData, err := req.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDAddressRequest)
	w.WriteRawBytes(msgData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err = reactor.HandleMessage(peerID, data)
	require.NoError(t, err)
}

func TestReactor_HandleAddressResponse(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("responder")

	multiaddr1 := "/ip4/127.0.0.1/tcp/26656/p2p/12D3KooWNvSZnPi3RrhrTwEY4LuuBeB6K6facKUCJcyWG1J6t5qa"
	multiaddr2 := "/ip4/127.0.0.2/tcp/26656/p2p/12D3KooWNvSZnPi3RrhrTwEY4LuuBeB6K6facKUCJcyWG1J6t5qb"
	lastSeen := time.Now().Unix()
	latency1 := int32(50)
	latency2 := int32(60)
	nodeID1 := "12D3KooWNvSZnPi3RrhrTwEY4LuuBeB6K6facKUCJcyWG1J6t5qa"
	nodeID2 := "12D3KooWNvSZnPi3RrhrTwEY4LuuBeB6K6facKUCJcyWG1J6t5qb"

	resp := &schema.AddressResponse{
		Peers: []schema.AddressInfo{
			{
				Multiaddr: &multiaddr1,
				LastSeen:  &lastSeen,
				Latency:   &latency1,
				NodeId:    &nodeID1,
			},
			{
				Multiaddr: &multiaddr2,
				LastSeen:  &lastSeen,
				Latency:   &latency2,
				NodeId:    &nodeID2,
			},
		},
	}

	msgData, err := resp.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDAddressResponse)
	w.WriteRawBytes(msgData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	require.Equal(t, 0, ab.Size())

	err = reactor.HandleMessage(peerID, data)
	require.NoError(t, err)

	require.Equal(t, 2, ab.Size())
}

func TestReactor_HandleAddressResponse_InvalidNodeID(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("responder")

	multiaddr := "/ip4/127.0.0.1/tcp/26656"
	lastSeen := time.Now().Unix()
	latency := int32(50)
	invalidNodeID := "invalid-peer-id"

	resp := &schema.AddressResponse{
		Peers: []schema.AddressInfo{
			{
				Multiaddr: &multiaddr,
				LastSeen:  &lastSeen,
				Latency:   &latency,
				NodeId:    &invalidNodeID,
			},
		},
	}

	msgData, err := resp.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDAddressResponse)
	w.WriteRawBytes(msgData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err = reactor.HandleMessage(peerID, data)
	require.NoError(t, err)

	require.Equal(t, 0, ab.Size())
}

func TestReactor_OnPeerConnected(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("new-peer")
	multiaddr := "/ip4/127.0.0.1/tcp/26656"

	reactor.OnPeerConnected(peerID, multiaddr, true)

	require.True(t, ab.HasPeer(peerID))

	entry, ok := ab.GetPeer(peerID)
	require.True(t, ok)
	require.Equal(t, multiaddr, entry.Multiaddr)
}

func TestReactor_OnPeerDisconnected(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("peer")
	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)

	initialLastSeen, _ := ab.GetPeer(peerID)

	time.Sleep(10 * time.Millisecond)

	reactor.OnPeerDisconnected(peerID)

	entry, ok := ab.GetPeer(peerID)
	require.True(t, ok)
	require.GreaterOrEqual(t, entry.LastSeen, initialLastSeen.LastSeen)
}

func TestReactor_GetAddressBook(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	require.Equal(t, ab, reactor.GetAddressBook())
}

func TestReactor_EncodeMessage(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	lastSeen := time.Now().Unix()
	req := &schema.AddressRequest{
		LastSeen: &lastSeen,
	}

	data, err := reactor.encodeMessage(TypeIDAddressRequest, req)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDAddressRequest, typeID)

	var decoded schema.AddressRequest
	err = decoded.UnmarshalCramberry(r.Remaining())
	require.NoError(t, err)
	require.Equal(t, lastSeen, *decoded.LastSeen)
}

func TestReactorTypeIDConstants(t *testing.T) {
	require.Equal(t, cramberry.TypeID(131), TypeIDAddressRequest)
	require.Equal(t, cramberry.TypeID(132), TypeIDAddressResponse)

	req := &schema.AddressRequest{}
	resp := &schema.AddressResponse{}

	require.Equal(t, TypeIDAddressRequest, schema.PexMessageTypeID(req))
	require.Equal(t, TypeIDAddressResponse, schema.PexMessageTypeID(resp))
}

func TestReactor_HandleAddressRequest_ExcludesRequester(t *testing.T) {
	ab := NewAddressBook("")

	requesterID := peer.ID("requester")
	ab.AddPeer(requesterID, "/ip4/127.0.0.1/tcp/26656", requesterID.String(), 50)
	ab.AddPeer(peer.ID("other-peer"), "/ip4/127.0.0.2/tcp/26656", "other-peer", 60)

	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	lastSeen := time.Now().Add(-24 * time.Hour).Unix()
	req := &schema.AddressRequest{LastSeen: &lastSeen}

	msgData, err := req.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDAddressRequest)
	w.WriteRawBytes(msgData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err = reactor.HandleMessage(requesterID, data)
	require.NoError(t, err)
}

func TestReactor_HandleAddressResponse_MissingFields(t *testing.T) {
	ab := NewAddressBook("")
	reactor := NewReactor(true, time.Second, 100, ab, nil, nil, 40, 10)

	peerID := peer.ID("responder")

	multiaddr := "/ip4/127.0.0.1/tcp/26656"

	resp := &schema.AddressResponse{
		Peers: []schema.AddressInfo{
			{
				Multiaddr: &multiaddr,
			},
		},
	}

	msgData, err := resp.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDAddressResponse)
	w.WriteRawBytes(msgData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err = reactor.HandleMessage(peerID, data)
	require.NoError(t, err)

	require.Equal(t, 0, ab.Size())
}
