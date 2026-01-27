package sync

import (
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	schema "github.com/blockberries/blockberry/schema"
)

func TestStateSyncReactor_NewStateSyncReactor(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		[]byte("testhash"),
		10*time.Second,
		30*time.Second,
		3,
	)

	require.NotNil(t, reactor)
	require.Equal(t, "statesync-reactor", reactor.Name())
	require.Equal(t, StateSyncIdle, reactor.State())
	require.Equal(t, int64(100), reactor.trustHeight)
	require.Equal(t, []byte("testhash"), reactor.trustHash)
}

func TestStateSyncReactor_StartStop(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		[]byte("testhash"),
		10*time.Second,
		30*time.Second,
		3,
	)

	require.False(t, reactor.IsRunning())

	err := reactor.Start()
	require.NoError(t, err)
	require.True(t, reactor.IsRunning())
	require.Equal(t, StateSyncDiscovering, reactor.State())

	// Double start should be idempotent
	err = reactor.Start()
	require.NoError(t, err)

	err = reactor.Stop()
	require.NoError(t, err)
	require.False(t, reactor.IsRunning())

	// Double stop should be idempotent
	err = reactor.Stop()
	require.NoError(t, err)
}

func TestStateSyncReactor_Progress(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		[]byte("testhash"),
		10*time.Second,
		30*time.Second,
		3,
	)

	// No offer selected - progress is 0
	require.Equal(t, 0, reactor.Progress())

	// Simulate selected offer with 4 chunks
	reactor.selectedOffer = &SnapshotOffer{
		Chunks: 4,
	}
	reactor.chunks = make([][]byte, 4)
	reactor.chunkStatus = make([]bool, 4)

	// No chunks received - progress is 0
	require.Equal(t, 0, reactor.Progress())

	// 2 of 4 chunks received - progress is 50%
	reactor.chunkStatus[0] = true
	reactor.chunkStatus[1] = true
	require.Equal(t, 50, reactor.Progress())

	// All chunks received - progress is 100%
	reactor.chunkStatus[2] = true
	reactor.chunkStatus[3] = true
	require.Equal(t, 100, reactor.Progress())
}

func TestStateSyncReactor_HandleMessage_InvalidTypeID(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Create message with invalid type ID
	w := cramberry.GetWriter()
	w.WriteTypeID(255)
	w.WriteRawBytes([]byte{1, 2, 3})
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err := reactor.HandleMessage(peer.ID("test-peer"), data)
	require.Error(t, err)
	require.Contains(t, err.Error(), "type ID 255")
}

func TestStateSyncReactor_HandleMessage_EmptyMessage(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	err := reactor.HandleMessage(peer.ID("test-peer"), []byte{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty message")
}

func TestStateSyncReactor_EncodeMessage(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	minHeight := int64(100)
	req := &schema.SnapshotsRequest{
		MinHeight: &minHeight,
	}

	data, err := reactor.encodeMessage(TypeIDSnapshotsRequest, req)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify type ID
	r := cramberry.NewReader(data)
	typeID := r.ReadTypeID()
	require.NoError(t, r.Err())
	require.Equal(t, TypeIDSnapshotsRequest, typeID)
}

func TestStateSyncReactor_HandleSnapshotsResponse(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Set state to discovering
	reactor.state = StateSyncDiscovering

	// Create response
	height := int64(150)
	chunks := int32(5)
	createdAt := time.Now().UnixNano()
	resp := &schema.SnapshotsResponse{
		Snapshots: []schema.SnapshotMetadata{
			{
				Height:    &height,
				Hash:      []byte("snaphash123"),
				Chunks:    &chunks,
				AppHash:   []byte("apphash456"),
				CreatedAt: createdAt,
			},
		},
	}

	respData, err := resp.MarshalCramberry()
	require.NoError(t, err)

	// Create message with type ID
	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDSnapshotsResponse)
	w.WriteRawBytes(respData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	peerID := peer.ID("test-peer")
	err = reactor.HandleMessage(peerID, data)
	require.NoError(t, err)

	// Verify offer was stored
	require.Len(t, reactor.offers, 1)
	hashHex := "736e617068617368313233" // hex of "snaphash123"
	offer, exists := reactor.offers[hashHex]
	require.True(t, exists)
	require.Equal(t, int64(150), offer.Height)
	require.Equal(t, 5, offer.Chunks)
	require.Equal(t, peerID, offer.PeerID)
}

func TestStateSyncReactor_HandleSnapshotsResponse_WrongState(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Set state to downloading (not discovering)
	reactor.state = StateSyncDownloading

	height := int64(150)
	chunks := int32(5)
	resp := &schema.SnapshotsResponse{
		Snapshots: []schema.SnapshotMetadata{
			{
				Height:    &height,
				Hash:      []byte("snaphash123"),
				Chunks:    &chunks,
				AppHash:   []byte("apphash456"),
				CreatedAt: 0,
			},
		},
	}

	respData, err := resp.MarshalCramberry()
	require.NoError(t, err)

	w := cramberry.GetWriter()
	w.WriteTypeID(TypeIDSnapshotsResponse)
	w.WriteRawBytes(respData)
	data := w.BytesCopy()
	cramberry.PutWriter(w)

	err = reactor.HandleMessage(peer.ID("test-peer"), data)
	require.NoError(t, err)

	// Offer should not be stored
	require.Len(t, reactor.offers, 0)
}

func TestStateSyncReactor_SelectBestSnapshot(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100, // trust height
		[]byte("trusthash"),
		10*time.Second,
		30*time.Second,
		3,
	)

	// Add multiple offers
	reactor.offers = map[string]*SnapshotOffer{
		"hash1": {
			PeerID:  peer.ID("peer1"),
			Height:  90, // Below trust height - should be ignored
			Hash:    []byte("hash1"),
			Chunks:  3,
			AppHash: []byte("app1"),
		},
		"hash2": {
			PeerID:  peer.ID("peer2"),
			Height:  100, // At trust height, matching trust hash
			Hash:    []byte("hash2"),
			Chunks:  4,
			AppHash: []byte("trusthash"),
		},
		"hash3": {
			PeerID:  peer.ID("peer3"),
			Height:  150, // Above trust height - should be selected
			Hash:    []byte("hash3"),
			Chunks:  5,
			AppHash: []byte("app3"),
		},
	}

	reactor.selectBestSnapshot()

	require.NotNil(t, reactor.selectedOffer)
	require.Equal(t, int64(150), reactor.selectedOffer.Height)
	require.Equal(t, 5, reactor.selectedOffer.Chunks)
	require.Len(t, reactor.chunks, 5)
	require.Len(t, reactor.chunkStatus, 5)
	require.Equal(t, StateSyncDownloading, reactor.state)
}

func TestStateSyncReactor_SelectBestSnapshot_TrustHashMismatch(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100, // trust height
		[]byte("trusthash"),
		10*time.Second,
		30*time.Second,
		3,
	)

	// Only one offer at trust height with wrong hash
	reactor.offers = map[string]*SnapshotOffer{
		"hash1": {
			PeerID:  peer.ID("peer1"),
			Height:  100, // At trust height
			Hash:    []byte("hash1"),
			Chunks:  3,
			AppHash: []byte("wronghash"), // Doesn't match trust hash
		},
	}

	reactor.selectBestSnapshot()

	// Should not select anything
	require.Nil(t, reactor.selectedOffer)
}

func TestStateSyncReactor_TypeIDConstants(t *testing.T) {
	require.Equal(t, cramberry.TypeID(144), TypeIDSnapshotsRequest)
	require.Equal(t, cramberry.TypeID(145), TypeIDSnapshotsResponse)
	require.Equal(t, cramberry.TypeID(146), TypeIDSnapshotChunkRequest)
	require.Equal(t, cramberry.TypeID(147), TypeIDSnapshotChunkResponse)

	// Verify against schema
	minHeight := int64(100)
	require.Equal(t, TypeIDSnapshotsRequest, schema.StateSyncMessageTypeID(&schema.SnapshotsRequest{MinHeight: &minHeight}))
	require.Equal(t, TypeIDSnapshotsResponse, schema.StateSyncMessageTypeID(&schema.SnapshotsResponse{}))
	idx := int32(0)
	require.Equal(t, TypeIDSnapshotChunkRequest, schema.StateSyncMessageTypeID(&schema.SnapshotChunkRequest{SnapshotHash: []byte("x"), ChunkIndex: &idx}))
	require.Equal(t, TypeIDSnapshotChunkResponse, schema.StateSyncMessageTypeID(&schema.SnapshotChunkResponse{SnapshotHash: []byte("x"), ChunkIndex: &idx, Data: []byte("x"), ChunkHash: []byte("x")}))
}

func TestStateSyncReactor_StateString(t *testing.T) {
	require.Equal(t, "idle", StateSyncIdle.String())
	require.Equal(t, "discovering", StateSyncDiscovering.String())
	require.Equal(t, "downloading", StateSyncDownloading.String())
	require.Equal(t, "applying", StateSyncApplying.String())
	require.Equal(t, "complete", StateSyncComplete.String())
	require.Equal(t, "failed", StateSyncFailed.String())
	require.Equal(t, "unknown", StateSyncState(99).String())
}

func TestStateSyncReactor_OnPeerDisconnected(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	peerID := peer.ID("test-peer")
	otherPeerID := peer.ID("other-peer")

	// Add pending chunks from both peers
	reactor.pendingChunks[0] = peerID
	reactor.pendingChunks[1] = otherPeerID
	reactor.pendingChunks[2] = peerID
	reactor.lastChunkTime[0] = time.Now()
	reactor.lastChunkTime[1] = time.Now()
	reactor.lastChunkTime[2] = time.Now()

	reactor.OnPeerDisconnected(peerID)

	// Chunks from disconnected peer should be removed
	require.Len(t, reactor.pendingChunks, 1)
	require.Contains(t, reactor.pendingChunks, 1)
	require.Len(t, reactor.lastChunkTime, 1)
	require.Contains(t, reactor.lastChunkTime, 1)
}

func TestStateSyncReactor_Callbacks(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	var completedHeight int64
	var completedHash []byte
	var failedErr error

	reactor.SetOnComplete(func(height int64, appHash []byte) {
		completedHeight = height
		completedHash = appHash
	})

	reactor.SetOnFailed(func(err error) {
		failedErr = err
	})

	// Verify callbacks are set
	require.NotNil(t, reactor.onComplete)
	require.NotNil(t, reactor.onFailed)

	// Test values are still zero since callbacks haven't been invoked
	require.Equal(t, int64(0), completedHeight)
	require.Nil(t, completedHash)
	require.Nil(t, failedErr)
}

func TestStateSyncReactor_HandleSnapshotsRequest(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Create request
	minHeight := int64(50)
	req := &schema.SnapshotsRequest{
		MinHeight: &minHeight,
	}

	reqData, err := req.MarshalCramberry()
	require.NoError(t, err)

	// Without snapshot store, should return nil (no error)
	err = reactor.handleSnapshotsRequest(peer.ID("test-peer"), reqData)
	require.NoError(t, err)
}

func TestStateSyncReactor_HandleSnapshotsRequest_InvalidMessage(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Request without required field
	req := &schema.SnapshotsRequest{}
	reqData, err := req.MarshalCramberry()
	require.NoError(t, err)

	err = reactor.handleSnapshotsRequest(peer.ID("test-peer"), reqData)
	require.Error(t, err)
}

func TestStateSyncReactor_HandleChunkRequest(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Create request
	idx := int32(0)
	req := &schema.SnapshotChunkRequest{
		SnapshotHash: []byte("snaphash"),
		ChunkIndex:   &idx,
	}

	reqData, err := req.MarshalCramberry()
	require.NoError(t, err)

	// Without snapshot store, should return nil (no error)
	err = reactor.handleChunkRequest(peer.ID("test-peer"), reqData)
	require.NoError(t, err)
}

func TestStateSyncReactor_HandleChunkRequest_InvalidMessage(t *testing.T) {
	reactor := NewStateSyncReactor(
		nil,
		nil,
		nil,
		100,
		nil,
		10*time.Second,
		30*time.Second,
		3,
	)

	// Request without required fields
	req := &schema.SnapshotChunkRequest{}
	reqData, err := req.MarshalCramberry()
	require.NoError(t, err)

	err = reactor.handleChunkRequest(peer.ID("test-peer"), reqData)
	require.Error(t, err)
}
