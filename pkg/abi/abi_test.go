package abi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResultCode(t *testing.T) {
	t.Run("IsOK", func(t *testing.T) {
		assert.True(t, CodeOK.IsOK())
		assert.False(t, CodeInvalidTx.IsOK())
		assert.False(t, CodeNotAuthorized.IsOK())
	})

	t.Run("IsError", func(t *testing.T) {
		assert.False(t, CodeOK.IsError())
		assert.True(t, CodeInvalidTx.IsError())
	})

	t.Run("IsAppError", func(t *testing.T) {
		assert.False(t, CodeOK.IsAppError())
		assert.False(t, CodeInvalidTx.IsAppError())
		assert.True(t, ResultCode(100).IsAppError())
		assert.True(t, ResultCode(500).IsAppError())
	})

	t.Run("String", func(t *testing.T) {
		assert.Equal(t, "OK", CodeOK.String())
		assert.Equal(t, "InvalidTx", CodeInvalidTx.String())
		assert.Equal(t, "NotAuthorized", CodeNotAuthorized.String())
		assert.Equal(t, "AppError(150)", ResultCode(150).String())
	})
}

func TestTransaction(t *testing.T) {
	t.Run("ComputeHash", func(t *testing.T) {
		tx := &Transaction{Data: []byte("test transaction")}
		hash := tx.ComputeHash()
		assert.Len(t, hash, 32) // SHA-256
		assert.Equal(t, hash, tx.Hash)

		// Computing again should return same hash
		hash2 := tx.ComputeHash()
		assert.Equal(t, hash, hash2)
	})

	t.Run("ValidateBasic", func(t *testing.T) {
		tx := &Transaction{}
		assert.Error(t, tx.ValidateBasic())

		tx.Data = []byte("valid")
		assert.NoError(t, tx.ValidateBasic())
	})

	t.Run("Size", func(t *testing.T) {
		tx := &Transaction{Data: []byte("12345")}
		assert.Equal(t, 5, tx.Size())
	})
}

func TestTxCheckResult(t *testing.T) {
	t.Run("IsOK", func(t *testing.T) {
		result := &TxCheckResult{Code: CodeOK}
		assert.True(t, result.IsOK())

		result.Code = CodeInvalidTx
		assert.False(t, result.IsOK())

		var nilResult *TxCheckResult
		assert.False(t, nilResult.IsOK())
	})
}

func TestTxResult(t *testing.T) {
	t.Run("IsSuccess", func(t *testing.T) {
		result := &TxResult{Code: 0}
		assert.True(t, result.IsSuccess())

		result.Code = 1
		assert.False(t, result.IsSuccess())
	})
}

func TestEvent(t *testing.T) {
	t.Run("NewEvent", func(t *testing.T) {
		event := NewEvent("transfer")
		assert.Equal(t, "transfer", event.Type)
		assert.Empty(t, event.Attributes)
	})

	t.Run("AddAttribute", func(t *testing.T) {
		event := NewEvent("transfer").
			AddAttribute("sender", []byte("addr1")).
			AddStringAttribute("recipient", "addr2").
			AddIndexedAttribute("amount", []byte("100"))

		assert.Len(t, event.Attributes, 3)
		assert.Equal(t, "sender", event.Attributes[0].Key)
		assert.Equal(t, []byte("addr1"), event.Attributes[0].Value)
		assert.False(t, event.Attributes[0].Index)

		assert.Equal(t, "recipient", event.Attributes[1].Key)
		assert.Equal(t, "addr2", event.Attributes[1].StringValue())

		assert.Equal(t, "amount", event.Attributes[2].Key)
		assert.True(t, event.Attributes[2].Index)
	})
}

func TestQueryResponse(t *testing.T) {
	t.Run("IsOK", func(t *testing.T) {
		resp := &QueryResponse{Code: CodeOK}
		assert.True(t, resp.IsOK())

		resp.Code = CodeNotFound
		assert.False(t, resp.IsOK())
	})

	t.Run("Exists", func(t *testing.T) {
		resp := &QueryResponse{Code: CodeOK, Value: []byte("value")}
		assert.True(t, resp.Exists())

		resp.Value = nil
		assert.False(t, resp.Exists())

		resp.Code = CodeNotFound
		assert.False(t, resp.Exists())
	})
}

func TestSimpleValidatorSet(t *testing.T) {
	validators := []Validator{
		{Address: []byte("val1"), PublicKey: []byte("pk1"), VotingPower: 10},
		{Address: []byte("val2"), PublicKey: []byte("pk2"), VotingPower: 20},
		{Address: []byte("val3"), PublicKey: []byte("pk3"), VotingPower: 30},
	}

	vs := NewSimpleValidatorSet(validators, 1)

	t.Run("Count", func(t *testing.T) {
		assert.Equal(t, 3, vs.Count())
	})

	t.Run("TotalVotingPower", func(t *testing.T) {
		assert.Equal(t, int64(60), vs.TotalVotingPower())
	})

	t.Run("GetByIndex", func(t *testing.T) {
		v := vs.GetByIndex(0)
		require.NotNil(t, v)
		assert.Equal(t, []byte("val1"), v.Address)

		v = vs.GetByIndex(99)
		assert.Nil(t, v)
	})

	t.Run("GetByAddress", func(t *testing.T) {
		v := vs.GetByAddress([]byte("val2"))
		require.NotNil(t, v)
		assert.Equal(t, int64(20), v.VotingPower)

		v = vs.GetByAddress([]byte("nonexistent"))
		assert.Nil(t, v)
	})

	t.Run("Proposer", func(t *testing.T) {
		// Round-robin selection
		p := vs.Proposer(0, 0)
		require.NotNil(t, p)
		assert.Equal(t, []byte("val1"), p.Address)

		p = vs.Proposer(1, 0)
		assert.Equal(t, []byte("val2"), p.Address)

		p = vs.Proposer(0, 1)
		assert.Equal(t, []byte("val2"), p.Address)
	})

	t.Run("F_and_Quorum", func(t *testing.T) {
		// With 3 validators, F = (3-1)/3 = 0
		assert.Equal(t, 0, vs.F())
		assert.Equal(t, int64(1), vs.Quorum()) // 2*0+1 = 1
	})

	t.Run("Epoch", func(t *testing.T) {
		assert.Equal(t, uint64(1), vs.Epoch())
	})

	t.Run("Copy", func(t *testing.T) {
		copy := vs.Copy()
		assert.Equal(t, vs.Count(), copy.Count())
		assert.Equal(t, vs.TotalVotingPower(), copy.TotalVotingPower())

		// Modifying copy shouldn't affect original
		copyVals := copy.Validators()
		copyVals[0].VotingPower = 999
		assert.Equal(t, int64(10), vs.GetByIndex(0).VotingPower)
	})
}

func TestValidatorUpdate(t *testing.T) {
	t.Run("IsRemoval", func(t *testing.T) {
		update := ValidatorUpdate{PublicKey: []byte("pk"), Power: 100}
		assert.False(t, update.IsRemoval())

		update.Power = 0
		assert.True(t, update.IsRemoval())
	})
}

func TestBaseApplication(t *testing.T) {
	app := &BaseApplication{}
	ctx := context.Background()

	t.Run("InitChain", func(t *testing.T) {
		err := app.InitChain(ctx, nil, nil)
		assert.NoError(t, err)
	})

	t.Run("CheckTx_FailClosed", func(t *testing.T) {
		err := app.CheckTx(ctx, []byte("tx"))
		assert.Error(t, err)
	})

	t.Run("ExecuteTx_FailClosed", func(t *testing.T) {
		result, err := app.ExecuteTx(ctx, []byte("tx"))
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Query_FailClosed", func(t *testing.T) {
		result, err := app.Query(ctx, "/test", nil, 0)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("BeginBlock", func(t *testing.T) {
		err := app.BeginBlock(ctx, &BlockHeader{Height: int64(1)})
		assert.NoError(t, err)
	})

	t.Run("EndBlock", func(t *testing.T) {
		result, err := app.EndBlock(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("Commit", func(t *testing.T) {
		result, err := app.Commit(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAcceptAllApplication(t *testing.T) {
	app := &AcceptAllApplication{}
	ctx := context.Background()

	t.Run("CheckTx_AcceptsAll", func(t *testing.T) {
		err := app.CheckTx(ctx, []byte("any"))
		assert.NoError(t, err)
	})

	t.Run("ExecuteTx_AcceptsAll", func(t *testing.T) {
		result, err := app.ExecuteTx(ctx, []byte("any"))
		assert.NoError(t, err)
		assert.True(t, result.IsSuccess())
	})

	t.Run("Query_AcceptsAll", func(t *testing.T) {
		result, err := app.Query(ctx, "/any", nil, 0)
		assert.NoError(t, err)
		assert.True(t, result.IsSuccess())
	})
}

func TestBlockHeader(t *testing.T) {
	header := BlockHeader{
		Height:          int64(100),
		Time:            time.Now(),
		LastBlockHash:   []byte("prevhash"),
		ProposerAddress: []byte("proposer"),
	}

	assert.Equal(t, int64(100), header.Height)
	assert.NotEmpty(t, header.LastBlockHash)
}

func TestEvidenceType(t *testing.T) {
	assert.Equal(t, "DuplicateVote", EvidenceDuplicateVote.String())
	assert.Equal(t, "LightClientAttack", EvidenceLightClientAttack.String())
	assert.Equal(t, "Unknown", EvidenceUnknown.String())
}

func TestBlockIDFlag(t *testing.T) {
	assert.Equal(t, "Absent", BlockIDFlagAbsent.String())
	assert.Equal(t, "Commit", BlockIDFlagCommit.String())
	assert.Equal(t, "Nil", BlockIDFlagNil.String())
}

func TestCheckTxMode(t *testing.T) {
	assert.Equal(t, "New", CheckTxNew.String())
	assert.Equal(t, "Recheck", CheckTxRecheck.String())
	assert.Equal(t, "Recovery", CheckTxRecovery.String())
}
