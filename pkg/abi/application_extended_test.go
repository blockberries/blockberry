package abi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSnapshotApplication tests the SnapshotApplication interface and types.
func TestSnapshotApplication(t *testing.T) {
	t.Run("Snapshot type", func(t *testing.T) {
		snapshot := &Snapshot{
			Height:   100,
			Format:   1,
			Chunks:   10,
			Hash:     []byte("hash"),
			Metadata: []byte("metadata"),
		}
		assert.Equal(t, int64(100), snapshot.Height)
		assert.Equal(t, uint32(1), snapshot.Format)
		assert.Equal(t, uint32(10), snapshot.Chunks)
		assert.Equal(t, []byte("hash"), snapshot.Hash)
		assert.Equal(t, []byte("metadata"), snapshot.Metadata)
	})

	t.Run("OfferResult String", func(t *testing.T) {
		assert.Equal(t, "Accept", OfferAccept.String())
		assert.Equal(t, "Abort", OfferAbort.String())
		assert.Equal(t, "Reject", OfferReject.String())
		assert.Equal(t, "RejectFormat", OfferRejectFormat.String())
		assert.Equal(t, "RejectSender", OfferRejectSender.String())
		assert.Equal(t, "Unknown", OfferResult(99).String())
	})

	t.Run("ApplyResult String", func(t *testing.T) {
		assert.Equal(t, "Accept", ApplyAccept.String())
		assert.Equal(t, "Abort", ApplyAbort.String())
		assert.Equal(t, "Retry", ApplyRetry.String())
		assert.Equal(t, "RetrySnapshot", ApplyRetrySnapshot.String())
		assert.Equal(t, "RejectSnapshot", ApplyRejectSnapshot.String())
		assert.Equal(t, "Unknown", ApplyResult(99).String())
	})

	t.Run("BaseSnapshotApplication defaults", func(t *testing.T) {
		app := &BaseSnapshotApplication{}

		// ListSnapshots returns nil
		snapshots := app.ListSnapshots()
		assert.Nil(t, snapshots)

		// LoadSnapshotChunk returns error
		_, err := app.LoadSnapshotChunk(100, 1, 0)
		assert.Equal(t, ErrNotImplemented, err)

		// OfferSnapshot rejects
		result := app.OfferSnapshot(&Snapshot{Height: 100})
		assert.Equal(t, OfferReject, result)

		// ApplySnapshotChunk rejects
		applyResult := app.ApplySnapshotChunk([]byte("chunk"), 0)
		assert.Equal(t, ApplyRejectSnapshot, applyResult)
	})

	t.Run("BaseSnapshotApplication implements interface", func(t *testing.T) {
		var app SnapshotApplication = &BaseSnapshotApplication{}
		require.NotNil(t, app)
	})
}

// TestProposerApplication tests the ProposerApplication interface and types.
func TestProposerApplication(t *testing.T) {
	t.Run("PrepareRequest and PrepareResponse", func(t *testing.T) {
		req := &PrepareRequest{
			MaxTxBytes:      1000000,
			Txs:             [][]byte{[]byte("tx1"), []byte("tx2")},
			Height:          100,
			Time:            time.Now(),
			ProposerAddress: []byte("proposer"),
		}
		assert.Equal(t, int64(1000000), req.MaxTxBytes)
		assert.Len(t, req.Txs, 2)

		resp := &PrepareResponse{
			Txs: req.Txs,
		}
		assert.Len(t, resp.Txs, 2)
	})

	t.Run("ProcessRequest and ProcessResponse", func(t *testing.T) {
		req := &ProcessRequest{
			Txs:             [][]byte{[]byte("tx1")},
			Hash:            []byte("hash"),
			Height:          100,
			Time:            time.Now(),
			ProposerAddress: []byte("proposer"),
		}
		assert.Equal(t, []byte("hash"), req.Hash)

		resp := &ProcessResponse{
			Status: ProcessAccept,
		}
		assert.Equal(t, ProcessAccept, resp.Status)
	})

	t.Run("ProcessStatus String", func(t *testing.T) {
		assert.Equal(t, "Accept", ProcessAccept.String())
		assert.Equal(t, "Reject", ProcessReject.String())
		assert.Equal(t, "Unknown", ProcessStatus(99).String())
	})

	t.Run("CommitInfo and VoteInfo", func(t *testing.T) {
		info := CommitInfo{
			Round: 1,
			Votes: []VoteInfo{
				{
					Validator:   Validator{Address: []byte("val1")},
					BlockIDFlag: BlockIDFlagCommit,
				},
			},
		}
		assert.Equal(t, uint32(1), info.Round)
		assert.Len(t, info.Votes, 1)
		assert.Equal(t, BlockIDFlagCommit, info.Votes[0].BlockIDFlag)
	})

	t.Run("Misbehavior types", func(t *testing.T) {
		mb := Misbehavior{
			Type:             MisbehaviorDuplicateVote,
			Validator:        Validator{Address: []byte("bad")},
			Height:           100,
			Time:             time.Now(),
			TotalVotingPower: 1000,
		}
		assert.Equal(t, MisbehaviorDuplicateVote, mb.Type)
	})

	t.Run("MisbehaviorType String", func(t *testing.T) {
		assert.Equal(t, "DuplicateVote", MisbehaviorDuplicateVote.String())
		assert.Equal(t, "LightClientAttack", MisbehaviorLightClientAttack.String())
		assert.Equal(t, "Unknown", MisbehaviorUnknown.String())
		assert.Equal(t, "Unknown", MisbehaviorType(99).String())
	})

	t.Run("BaseProposerApplication defaults", func(t *testing.T) {
		app := &BaseProposerApplication{}
		ctx := context.Background()

		// PrepareProposal passes through transactions
		prepReq := &PrepareRequest{
			Txs: [][]byte{[]byte("tx1"), []byte("tx2")},
		}
		prepResp := app.PrepareProposal(ctx, prepReq)
		assert.Equal(t, prepReq.Txs, prepResp.Txs)

		// ProcessProposal accepts all
		procReq := &ProcessRequest{Txs: [][]byte{[]byte("tx1")}}
		procResp := app.ProcessProposal(ctx, procReq)
		assert.Equal(t, ProcessAccept, procResp.Status)
	})

	t.Run("BaseProposerApplication implements interface", func(t *testing.T) {
		var app ProposerApplication = &BaseProposerApplication{}
		require.NotNil(t, app)
	})
}

// TestFinalityApplication tests the FinalityApplication interface and types.
func TestFinalityApplication(t *testing.T) {
	t.Run("ExtendVoteRequest and ExtendVoteResponse", func(t *testing.T) {
		req := &ExtendVoteRequest{
			Hash:   []byte("hash"),
			Height: 100,
			Time:   time.Now(),
		}
		assert.Equal(t, []byte("hash"), req.Hash)

		resp := &ExtendVoteResponse{
			VoteExtension: []byte("extension"),
		}
		assert.Equal(t, []byte("extension"), resp.VoteExtension)
	})

	t.Run("VerifyVoteExtRequest and VerifyVoteExtResponse", func(t *testing.T) {
		req := &VerifyVoteExtRequest{
			Hash:             []byte("hash"),
			ValidatorAddress: []byte("validator"),
			Height:           100,
			VoteExtension:    []byte("extension"),
		}
		assert.Equal(t, []byte("validator"), req.ValidatorAddress)

		resp := &VerifyVoteExtResponse{
			Status: VerifyAccept,
		}
		assert.Equal(t, VerifyAccept, resp.Status)
	})

	t.Run("VerifyStatus String", func(t *testing.T) {
		assert.Equal(t, "Accept", VerifyAccept.String())
		assert.Equal(t, "Reject", VerifyReject.String())
		assert.Equal(t, "Unknown", VerifyStatus(99).String())
	})

	t.Run("FinalizeBlockRequest and FinalizeBlockResponse", func(t *testing.T) {
		req := &FinalizeBlockRequest{
			Txs:             [][]byte{[]byte("tx1")},
			Hash:            []byte("hash"),
			Height:          100,
			Time:            time.Now(),
			ProposerAddress: []byte("proposer"),
		}
		assert.Len(t, req.Txs, 1)

		resp := &FinalizeBlockResponse{
			Events:           []Event{{Type: "test"}},
			TxResults:        []TxResult{{Code: 0}},
			ValidatorUpdates: []ValidatorUpdate{{Power: 100}},
			AppHash:          []byte("apphash"),
		}
		assert.Len(t, resp.Events, 1)
		assert.Len(t, resp.TxResults, 1)
		assert.Equal(t, []byte("apphash"), resp.AppHash)
	})

	t.Run("ExtendedCommitInfo and ExtendedVoteInfo", func(t *testing.T) {
		info := ExtendedCommitInfo{
			Round: 1,
			Votes: []ExtendedVoteInfo{
				{
					Validator:          Validator{Address: []byte("val1")},
					BlockIDFlag:        BlockIDFlagCommit,
					VoteExtension:      []byte("ext1"),
					ExtensionSignature: []byte("sig1"),
				},
			},
		}
		assert.Equal(t, uint32(1), info.Round)
		assert.Len(t, info.Votes, 1)
		assert.Equal(t, []byte("ext1"), info.Votes[0].VoteExtension)
	})

	t.Run("BaseFinalityApplication defaults", func(t *testing.T) {
		app := &BaseFinalityApplication{}
		ctx := context.Background()

		// ExtendVote returns empty extension
		extReq := &ExtendVoteRequest{Hash: []byte("hash"), Height: 100}
		extResp := app.ExtendVote(ctx, extReq)
		assert.Nil(t, extResp.VoteExtension)

		// VerifyVoteExtension accepts all
		verReq := &VerifyVoteExtRequest{VoteExtension: []byte("ext")}
		verResp := app.VerifyVoteExtension(ctx, verReq)
		assert.Equal(t, VerifyAccept, verResp.Status)

		// FinalizeBlock returns empty results
		finReq := &FinalizeBlockRequest{Txs: [][]byte{[]byte("tx1"), []byte("tx2")}}
		finResp := app.FinalizeBlock(ctx, finReq)
		assert.Len(t, finResp.TxResults, 2)
		assert.Nil(t, finResp.AppHash)
	})

	t.Run("BaseFinalityApplication implements interface", func(t *testing.T) {
		var app FinalityApplication = &BaseFinalityApplication{}
		require.NotNil(t, app)
	})
}

// TestErrors tests the ABI errors.
func TestErrors(t *testing.T) {
	assert.NotNil(t, ErrNotImplemented)
	assert.NotNil(t, ErrSnapshotNotFound)
	assert.NotNil(t, ErrChunkNotFound)
	assert.NotNil(t, ErrInvalidSnapshot)
	assert.NotNil(t, ErrInvalidChunk)
	assert.NotNil(t, ErrStateSyncInProgress)
	assert.NotNil(t, ErrProposalRejected)
	assert.NotNil(t, ErrVoteExtensionInvalid)

	// Verify they have meaningful messages
	assert.Contains(t, ErrNotImplemented.Error(), "not implemented")
	assert.Contains(t, ErrSnapshotNotFound.Error(), "snapshot")
}
