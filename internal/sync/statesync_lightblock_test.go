package sync

import (
	"errors"
	"testing"
	"time"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	schema "github.com/blockberries/blockberry/schema"
)

// stubLightBlockProvider implements LightBlockProvider with canned
// responses. Used to test the server-side path of the reactor.
type stubLightBlockProvider struct {
	header     []byte
	commit     []byte
	validators []byte
	err        error
}

func (s *stubLightBlockProvider) LightBlockAt(height int64) ([]byte, []byte, []byte, error) {
	if s.err != nil {
		return nil, nil, nil, s.err
	}
	return s.header, s.commit, s.validators, nil
}

// stubLightBlockVerifier implements LightBlockVerifier with a canned
// outcome. Used to test the client-side verification path.
type stubLightBlockVerifier struct {
	appHash []byte // returned on success
	err     error  // non-nil triggers the reject path
	called  int    // number of times VerifyLightBlock was invoked
}

func (s *stubLightBlockVerifier) VerifyLightBlock(_ int64, _, _, _ []byte) ([]byte, error) {
	s.called++
	if s.err != nil {
		return nil, s.err
	}
	return s.appHash, nil
}

func TestStateSyncReactor_TypeIDConstants_LightBlock(t *testing.T) {
	require.Equal(t, cramberry.TypeID(148), TypeIDLightBlockRequest)
	require.Equal(t, cramberry.TypeID(149), TypeIDLightBlockResponse)
}

func TestStateSyncReactor_SetLightBlockProviderAndVerifier(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)

	// Defaults: nil.
	require.Nil(t, r.lightBlockProvider)
	require.Nil(t, r.lightBlockVerifier)

	// Set, then read back.
	prov := &stubLightBlockProvider{}
	r.SetLightBlockProvider(prov)
	require.Same(t, prov, r.lightBlockProvider.(*stubLightBlockProvider))

	ver := &stubLightBlockVerifier{}
	r.SetLightBlockVerifier(ver)
	require.Same(t, ver, r.lightBlockVerifier.(*stubLightBlockVerifier))

	// Nil out — also valid.
	r.SetLightBlockProvider(nil)
	r.SetLightBlockVerifier(nil)
	require.Nil(t, r.lightBlockProvider)
	require.Nil(t, r.lightBlockVerifier)
}

func TestStateSyncReactor_HandleLightBlockRequest_NoProvider(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)

	// Build a valid LightBlockRequest payload (just the message body —
	// HandleMessage strips the TypeID before dispatching to the handler).
	height := int64(50)
	req := &schema.LightBlockRequest{Height: &height}
	data, err := req.MarshalCramberry()
	require.NoError(t, err)

	// With no provider AND nil network, the handler returns nil without
	// crashing — sendLightBlockResponse silently no-ops on nil network.
	err = r.handleLightBlockRequest(peer.ID("peer1"), data)
	require.NoError(t, err)
}

func TestStateSyncReactor_HandleLightBlockRequest_InvalidPayload(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)

	// Garbage bytes — must produce ErrInvalidMessage.
	err := r.handleLightBlockRequest(peer.ID("peer1"), []byte{0xFF, 0xFF})
	require.Error(t, err)
}

func TestStateSyncReactor_HandleLightBlockResponse_NoVerifier(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)

	height := int64(50)
	resp := &schema.LightBlockResponse{
		Height:            &height,
		HeaderBytes:       []byte{0x01},
		CommitBytes:       []byte{0x02},
		ValidatorSetBytes: []byte{0x03},
	}
	data, err := resp.MarshalCramberry()
	require.NoError(t, err)

	// No verifier — handler drops silently, no error, no verification.
	err = r.handleLightBlockResponse(peer.ID("peer1"), data)
	require.NoError(t, err)
	require.Len(t, r.verifiedHeights, 0)
}

func TestStateSyncReactor_HandleLightBlockResponse_EmptyResponse(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)
	ver := &stubLightBlockVerifier{}
	r.SetLightBlockVerifier(ver)

	height := int64(50)
	r.pendingLightBlock[height] = true // simulate a prior request

	resp := &schema.LightBlockResponse{
		Height:            &height,
		HeaderBytes:       nil, // peer has no data
		CommitBytes:       nil,
		ValidatorSetBytes: nil,
	}
	data, err := resp.MarshalCramberry()
	require.NoError(t, err)

	err = r.handleLightBlockResponse(peer.ID("peer1"), data)
	require.NoError(t, err)

	// Pending flag cleared (so we can retry against another peer).
	require.False(t, r.pendingLightBlock[height])
	// Verifier was NOT invoked — empty response means "peer has no data".
	require.Equal(t, 0, ver.called)
	// Not marked verified.
	require.Empty(t, r.verifiedHeights)
}

func TestStateSyncReactor_HandleLightBlockResponse_VerifierAccepts(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)
	ver := &stubLightBlockVerifier{appHash: []byte{0xAA, 0xBB}}
	r.SetLightBlockVerifier(ver)

	height := int64(50)
	resp := &schema.LightBlockResponse{
		Height:            &height,
		HeaderBytes:       []byte{0x01},
		CommitBytes:       []byte{0x02},
		ValidatorSetBytes: []byte{0x03},
	}
	data, err := resp.MarshalCramberry()
	require.NoError(t, err)

	require.NoError(t, r.handleLightBlockResponse(peer.ID("peer1"), data))
	require.Equal(t, 1, ver.called)
	require.NotNil(t, r.verifiedHeights[height])
	require.Equal(t, []byte{0xAA, 0xBB}, r.verifiedHeights[height].appHash)
}

func TestStateSyncReactor_HandleLightBlockResponse_VerifierRejects(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 0, nil, time.Second, time.Second, 3)
	ver := &stubLightBlockVerifier{err: errors.New("commit invalid")}
	r.SetLightBlockVerifier(ver)

	height := int64(50)
	resp := &schema.LightBlockResponse{
		Height:            &height,
		HeaderBytes:       []byte{0x01},
		CommitBytes:       []byte{0x02},
		ValidatorSetBytes: []byte{0x03},
	}
	data, err := resp.MarshalCramberry()
	require.NoError(t, err)

	// Handler returns nil even on reject — the penalty path is
	// best-effort (the network is nil here, so AddPenalty is a no-op).
	require.NoError(t, r.handleLightBlockResponse(peer.ID("peer1"), data))
	require.Equal(t, 1, ver.called)
	require.Empty(t, r.verifiedHeights)
}

func TestStateSyncReactor_SelectBestSnapshot_WithVerifierRequiresVerification(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 50, nil, time.Second, time.Second, 3)
	r.SetLightBlockVerifier(&stubLightBlockVerifier{})

	// Offer at h=100 with AppHash X.
	r.offers["a"] = &SnapshotOffer{
		PeerID: peer.ID("p"), Height: 100, Hash: []byte("a"), Chunks: 1,
		AppHash: []byte{0xAA},
	}

	r.selectBestSnapshot()
	// Verifier installed, no verifiedHeights entry — must NOT select.
	require.Nil(t, r.selectedOffer)

	// Add a verified entry with matching AppHash — now eligible.
	r.verifiedHeights[100] = &verifiedLightBlock{height: 100, appHash: []byte{0xAA}}
	r.selectBestSnapshot()
	require.NotNil(t, r.selectedOffer)
	require.Equal(t, int64(100), r.selectedOffer.Height)
}

func TestStateSyncReactor_SelectBestSnapshot_WithVerifierAppHashMismatch(t *testing.T) {
	r := NewStateSyncReactor(nil, nil, nil, 50, nil, time.Second, time.Second, 3)
	r.SetLightBlockVerifier(&stubLightBlockVerifier{})

	// Offer at h=100 with AppHash X.
	r.offers["a"] = &SnapshotOffer{
		PeerID: peer.ID("p"), Height: 100, Hash: []byte("a"), Chunks: 1,
		AppHash: []byte{0xAA},
	}
	// Verified header at h=100 says AppHash is Y — doesn't match the offer.
	r.verifiedHeights[100] = &verifiedLightBlock{height: 100, appHash: []byte{0xBB}}

	r.selectBestSnapshot()
	require.Nil(t, r.selectedOffer, "verifier+mismatched-AppHash must NOT select")
}

func TestStateSyncReactor_SelectBestSnapshot_LegacyPathStillWorksWithoutVerifier(t *testing.T) {
	// No verifier installed — fall back to the legacy AppHash-equality
	// check at trustHeight.
	r := NewStateSyncReactor(nil, nil, nil, 100, []byte{0xAA}, time.Second, time.Second, 3)

	// Offer at the trust height with matching AppHash.
	r.offers["a"] = &SnapshotOffer{
		PeerID: peer.ID("p"), Height: 100, Hash: []byte("a"), Chunks: 1,
		AppHash: []byte{0xAA},
	}

	r.selectBestSnapshot()
	require.NotNil(t, r.selectedOffer)
	require.Equal(t, int64(100), r.selectedOffer.Height)
}
