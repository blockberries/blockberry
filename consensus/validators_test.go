package consensus

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestValidators(count int) []*Validator {
	validators := make([]*Validator, count)
	for i := range count {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		validators[i] = &Validator{
			Index:       uint16(i),
			Address:     pub[:20],
			PublicKey:   pub,
			VotingPower: int64(100 + i*10),
		}
	}
	return validators
}

func TestNewSimpleValidatorSet(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, err := NewSimpleValidatorSet(validators, 1)
	require.NoError(t, err)
	require.NotNil(t, valSet)

	assert.Equal(t, 4, valSet.Count())
	assert.Equal(t, uint64(1), valSet.Epoch())
	assert.Equal(t, int64(100+110+120+130), valSet.TotalVotingPower())
}

func TestNewSimpleValidatorSet_Empty(t *testing.T) {
	valSet, err := NewSimpleValidatorSet([]*Validator{}, 0)
	require.NoError(t, err)
	require.NotNil(t, valSet)

	assert.Equal(t, 0, valSet.Count())
	assert.Equal(t, int64(0), valSet.TotalVotingPower())
}

func TestNewSimpleValidatorSet_NilValidator(t *testing.T) {
	validators := []*Validator{nil}
	_, err := NewSimpleValidatorSet(validators, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is nil")
}

func TestNewSimpleValidatorSet_InvalidVotingPower(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{1}, VotingPower: 0},
	}
	_, err := NewSimpleValidatorSet(validators, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidVotingPower)
}

func TestNewSimpleValidatorSet_DuplicateAddress(t *testing.T) {
	validators := []*Validator{
		{Address: []byte{1, 2, 3}, VotingPower: 100},
		{Address: []byte{1, 2, 3}, VotingPower: 100}, // Duplicate
	}
	_, err := NewSimpleValidatorSet(validators, 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDuplicateValidator)
}

func TestSimpleValidatorSet_GetByIndex(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Valid index
	v := valSet.GetByIndex(2)
	require.NotNil(t, v)
	assert.Equal(t, uint16(2), v.Index)

	// Invalid index
	v = valSet.GetByIndex(10)
	assert.Nil(t, v)
}

func TestSimpleValidatorSet_GetByAddress(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Valid address
	v := valSet.GetByAddress(validators[1].Address)
	require.NotNil(t, v)
	assert.Equal(t, uint16(1), v.Index)

	// Invalid address
	v = valSet.GetByAddress([]byte{99, 99, 99})
	assert.Nil(t, v)
}

func TestSimpleValidatorSet_Contains(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	assert.True(t, valSet.Contains(validators[0].Address))
	assert.False(t, valSet.Contains([]byte{99, 99, 99}))
}

func TestSimpleValidatorSet_GetProposer(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Deterministic selection based on height + round
	p0 := valSet.GetProposer(0, 0)
	assert.Equal(t, uint16(0), p0.Index)

	p1 := valSet.GetProposer(1, 0)
	assert.Equal(t, uint16(1), p1.Index)

	p2 := valSet.GetProposer(0, 2)
	assert.Equal(t, uint16(2), p2.Index)

	p3 := valSet.GetProposer(3, 4)
	assert.Equal(t, uint16(3), p3.Index)
}

func TestSimpleValidatorSet_GetProposer_Empty(t *testing.T) {
	valSet, _ := NewSimpleValidatorSet([]*Validator{}, 0)
	assert.Nil(t, valSet.GetProposer(0, 0))
}

func TestSimpleValidatorSet_Quorum(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Total power = 100+110+120+130 = 460
	// Quorum = 2/3 * 460 + 1 = 307
	assert.Equal(t, int64(307), valSet.Quorum())
}

func TestSimpleValidatorSet_F(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{1, 0},  // f = 0
		{2, 0},  // f = 0
		{3, 0},  // f = 0
		{4, 1},  // f = 1
		{5, 1},  // f = 1
		{6, 1},  // f = 1
		{7, 2},  // f = 2
		{10, 3}, // f = 3
	}

	for _, tt := range tests {
		validators := makeTestValidators(tt.count)
		valSet, _ := NewSimpleValidatorSet(validators, 0)
		assert.Equal(t, tt.expected, valSet.F(), "count=%d", tt.count)
	}
}

func TestSimpleValidatorSet_VerifyCommit_NilCommit(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	err := valSet.VerifyCommit(1, []byte{1, 2, 3}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil commit")
}

func TestSimpleValidatorSet_VerifyCommit_HeightMismatch(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	commit := &Commit{Height: 5}
	err := valSet.VerifyCommit(10, []byte{1, 2, 3}, commit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestSimpleValidatorSet_VerifyCommit_HashMismatch(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	commit := &Commit{Height: 10, BlockHash: []byte{4, 5, 6}}
	err := valSet.VerifyCommit(10, []byte{1, 2, 3}, commit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "block hash does not match")
}

func TestSimpleValidatorSet_VerifyCommit_ValidSignatures(t *testing.T) {
	// Create validators with real keys
	validators := make([]*Validator, 4)
	privKeys := make([]ed25519.PrivateKey, 4)

	for i := range 4 {
		pub, priv, _ := ed25519.GenerateKey(rand.Reader)
		privKeys[i] = priv
		validators[i] = &Validator{
			Index:       uint16(i),
			Address:     pub[:20],
			PublicKey:   pub,
			VotingPower: 100,
		}
	}

	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Create commit with valid signatures (need 3 out of 4 for quorum)
	height := int64(10)
	round := int32(0)
	blockHash := []byte{1, 2, 3, 4}
	signBytes := makeCommitSignBytes(height, round, blockHash)

	commit := &Commit{
		Height:    height,
		Round:     round,
		BlockHash: blockHash,
		Signatures: []CommitSig{
			{ValidatorIndex: 0, Signature: ed25519.Sign(privKeys[0], signBytes)},
			{ValidatorIndex: 1, Signature: ed25519.Sign(privKeys[1], signBytes)},
			{ValidatorIndex: 2, Signature: ed25519.Sign(privKeys[2], signBytes)},
			{ValidatorIndex: 3, Signature: nil}, // Absent
		},
	}

	err := valSet.VerifyCommit(height, blockHash, commit)
	require.NoError(t, err)
}

func TestSimpleValidatorSet_VerifyCommit_InsufficientQuorum(t *testing.T) {
	validators := make([]*Validator, 4)
	privKeys := make([]ed25519.PrivateKey, 4)

	for i := range 4 {
		pub, priv, _ := ed25519.GenerateKey(rand.Reader)
		privKeys[i] = priv
		validators[i] = &Validator{
			Index:       uint16(i),
			Address:     pub[:20],
			PublicKey:   pub,
			VotingPower: 100,
		}
	}

	valSet, _ := NewSimpleValidatorSet(validators, 0)

	height := int64(10)
	round := int32(0)
	blockHash := []byte{1, 2, 3, 4}
	signBytes := makeCommitSignBytes(height, round, blockHash)

	// Only 2 signatures (not enough for quorum)
	commit := &Commit{
		Height:    height,
		Round:     round,
		BlockHash: blockHash,
		Signatures: []CommitSig{
			{ValidatorIndex: 0, Signature: ed25519.Sign(privKeys[0], signBytes)},
			{ValidatorIndex: 1, Signature: ed25519.Sign(privKeys[1], signBytes)},
		},
	}

	err := valSet.VerifyCommit(height, blockHash, commit)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInsufficientQuorum)
}

func TestSimpleValidatorSet_Validators(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	result := valSet.Validators()
	assert.Len(t, result, 4)

	// Verify it's a copy
	result[0].VotingPower = 999
	assert.NotEqual(t, int64(999), valSet.GetByIndex(0).VotingPower)
}

func TestSimpleValidatorSet_Copy(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 1)

	copy := valSet.Copy()
	require.NotNil(t, copy)

	assert.Equal(t, valSet.Count(), copy.Count())
	assert.Equal(t, valSet.TotalVotingPower(), copy.TotalVotingPower())
	assert.Equal(t, valSet.Epoch(), copy.Epoch())

	// Verify it's a deep copy
	copy.validators[0].VotingPower = 999
	assert.NotEqual(t, int64(999), valSet.GetByIndex(0).VotingPower)
}

func TestNewIndexedValidatorSet(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, err := NewIndexedValidatorSet(validators, 1)
	require.NoError(t, err)
	require.NotNil(t, valSet)

	assert.Equal(t, 4, valSet.Count())
}

func TestIndexedValidatorSet_GetByAddress(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewIndexedValidatorSet(validators, 0)

	// O(1) lookup
	v := valSet.GetByAddress(validators[2].Address)
	require.NotNil(t, v)
	assert.Equal(t, uint16(2), v.Index)

	// Not found
	v = valSet.GetByAddress([]byte{99, 99, 99})
	assert.Nil(t, v)
}

func TestIndexedValidatorSet_Contains(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewIndexedValidatorSet(validators, 0)

	assert.True(t, valSet.Contains(validators[0].Address))
	assert.False(t, valSet.Contains([]byte{99, 99, 99}))
}

func TestIndexedValidatorSet_Copy(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewIndexedValidatorSet(validators, 1)

	copy := valSet.Copy()
	require.NotNil(t, copy)

	// Verify the index is also copied
	v := copy.GetByAddress(validators[1].Address)
	require.NotNil(t, v)
	assert.Equal(t, uint16(1), v.Index)
}

func TestWeightedProposerSelection(t *testing.T) {
	validators := []*Validator{
		{Index: 0, VotingPower: 100},
		{Index: 1, VotingPower: 200},
		{Index: 2, VotingPower: 300},
	}

	wps := NewWeightedProposerSelection(validators)
	require.NotNil(t, wps)

	// Initially all priorities are 0, first validator is selected
	p := wps.GetProposer()
	require.NotNil(t, p)
}

func TestWeightedProposerSelection_Empty(t *testing.T) {
	wps := NewWeightedProposerSelection([]*Validator{})
	assert.Nil(t, wps.GetProposer())
}

func TestWeightedProposerSelection_IncrementPriority(t *testing.T) {
	validators := []*Validator{
		{Index: 0, VotingPower: 100},
		{Index: 1, VotingPower: 200},
	}

	wps := NewWeightedProposerSelection(validators)

	// Track proposers over multiple rounds
	proposers := make(map[uint16]int)
	for range 30 {
		p := wps.GetProposer()
		proposers[p.Index]++
		wps.IncrementProposerPriority()
	}

	// Validator 1 (power 200) should propose roughly twice as often as validator 0 (power 100)
	// Allow some variance
	assert.Greater(t, proposers[1], proposers[0])
}

func TestNewInMemoryValidatorSetStore(t *testing.T) {
	store := NewInMemoryValidatorSetStore()
	require.NotNil(t, store)
}

func TestInMemoryValidatorSetStore_SaveLoad(t *testing.T) {
	store := NewInMemoryValidatorSetStore()

	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	err := store.SaveValidatorSet(10, valSet)
	require.NoError(t, err)

	loaded, err := store.LoadValidatorSet(10)
	require.NoError(t, err)
	assert.Equal(t, valSet.Count(), loaded.Count())
}

func TestInMemoryValidatorSetStore_LoadNotFound(t *testing.T) {
	store := NewInMemoryValidatorSetStore()

	_, err := store.LoadValidatorSet(10)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorSetNotFound)
}

func TestInMemoryValidatorSetStore_Latest(t *testing.T) {
	store := NewInMemoryValidatorSetStore()

	validators1 := makeTestValidators(4)
	valSet1, _ := NewSimpleValidatorSet(validators1, 1)

	validators2 := makeTestValidators(5)
	valSet2, _ := NewSimpleValidatorSet(validators2, 2)

	_ = store.SaveValidatorSet(10, valSet1)
	_ = store.SaveValidatorSet(20, valSet2)

	latest, err := store.LatestValidatorSet()
	require.NoError(t, err)
	assert.Equal(t, 5, latest.Count())
}

func TestInMemoryValidatorSetStore_LatestEmpty(t *testing.T) {
	store := NewInMemoryValidatorSetStore()

	_, err := store.LatestValidatorSet()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrValidatorSetNotFound)
}

func TestInMemoryValidatorSetStore_ValidatorSetAtHeight(t *testing.T) {
	store := NewInMemoryValidatorSetStore()

	validators1 := makeTestValidators(4)
	valSet1, _ := NewSimpleValidatorSet(validators1, 1)

	validators2 := makeTestValidators(5)
	valSet2, _ := NewSimpleValidatorSet(validators2, 2)

	// Set 1 active from height 10
	_ = store.SaveValidatorSet(10, valSet1)
	// Set 2 active from height 100
	_ = store.SaveValidatorSet(100, valSet2)

	// Query at height 50 should return set 1
	active, err := store.ValidatorSetAtHeight(50)
	require.NoError(t, err)
	assert.Equal(t, 4, active.Count())

	// Query at height 150 should return set 2
	active, err = store.ValidatorSetAtHeight(150)
	require.NoError(t, err)
	assert.Equal(t, 5, active.Count())

	// Query at height 5 (before any set) should fail
	_, err = store.ValidatorSetAtHeight(5)
	require.Error(t, err)
}
