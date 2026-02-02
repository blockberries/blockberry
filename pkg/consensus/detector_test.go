package consensus

import (
	"crypto/ed25519"
	"crypto/rand"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyBasedDetector_InterfaceCompliance(t *testing.T) {
	var _ ValidatorDetector = (*KeyBasedDetector)(nil)
}

func TestAddressBasedDetector_InterfaceCompliance(t *testing.T) {
	var _ ValidatorDetector = (*AddressBasedDetector)(nil)
}

func TestNewKeyBasedDetector(t *testing.T) {
	publicKey := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	detector := NewKeyBasedDetector(publicKey)
	require.NotNil(t, detector)

	// Verify it's a copy
	publicKey[0] = 99
	assert.Equal(t, byte(1), detector.publicKey[0])
}

func TestKeyBasedDetector_IsValidator(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Detector with validator's key
	detector := NewKeyBasedDetector(validators[1].PublicKey)
	assert.True(t, detector.IsValidator(valSet))

	// Detector with non-validator key
	nonValidator := []byte("not a validator key")
	detector2 := NewKeyBasedDetector(nonValidator)
	assert.False(t, detector2.IsValidator(valSet))
}

func TestKeyBasedDetector_IsValidator_NilSet(t *testing.T) {
	detector := NewKeyBasedDetector([]byte{1, 2, 3})
	assert.False(t, detector.IsValidator(nil))
}

func TestKeyBasedDetector_ValidatorIndex(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Detector for validator at index 2
	detector := NewKeyBasedDetector(validators[2].PublicKey)
	assert.Equal(t, 2, detector.ValidatorIndex(valSet))

	// Detector for non-validator
	detector2 := NewKeyBasedDetector([]byte("not a validator"))
	assert.Equal(t, -1, detector2.ValidatorIndex(valSet))
}

func TestKeyBasedDetector_ValidatorIndex_NilSet(t *testing.T) {
	detector := NewKeyBasedDetector([]byte{1, 2, 3})
	assert.Equal(t, -1, detector.ValidatorIndex(nil))
}

func TestKeyBasedDetector_GetValidator(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Detector for validator at index 1
	detector := NewKeyBasedDetector(validators[1].PublicKey)
	val := detector.GetValidator(valSet)
	require.NotNil(t, val)
	assert.Equal(t, uint16(1), val.Index)

	// Detector for non-validator
	detector2 := NewKeyBasedDetector([]byte("not a validator"))
	val2 := detector2.GetValidator(valSet)
	assert.Nil(t, val2)
}

func TestKeyBasedDetector_GetValidator_NilSet(t *testing.T) {
	detector := NewKeyBasedDetector([]byte{1, 2, 3})
	assert.Nil(t, detector.GetValidator(nil))
}

func TestKeyBasedDetector_PublicKey(t *testing.T) {
	publicKey := []byte{1, 2, 3, 4}
	detector := NewKeyBasedDetector(publicKey)

	retrieved := detector.PublicKey()
	assert.Equal(t, publicKey, retrieved)

	// Verify it's a copy
	retrieved[0] = 99
	assert.Equal(t, byte(1), detector.publicKey[0])
}

func TestNewAddressBasedDetector(t *testing.T) {
	address := []byte{1, 2, 3, 4, 5}
	detector := NewAddressBasedDetector(address)
	require.NotNil(t, detector)

	// Verify it's a copy
	address[0] = 99
	assert.Equal(t, byte(1), detector.address[0])
}

func TestAddressBasedDetector_IsValidator(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Detector with validator's address
	detector := NewAddressBasedDetector(validators[2].Address)
	assert.True(t, detector.IsValidator(valSet))

	// Detector with non-validator address
	detector2 := NewAddressBasedDetector([]byte("not an address"))
	assert.False(t, detector2.IsValidator(valSet))
}

func TestAddressBasedDetector_IsValidator_NilSet(t *testing.T) {
	detector := NewAddressBasedDetector([]byte{1, 2, 3})
	assert.False(t, detector.IsValidator(nil))
}

func TestAddressBasedDetector_ValidatorIndex(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Detector for validator at index 0
	detector := NewAddressBasedDetector(validators[0].Address)
	assert.Equal(t, 0, detector.ValidatorIndex(valSet))

	// Detector for non-validator
	detector2 := NewAddressBasedDetector([]byte("not an address"))
	assert.Equal(t, -1, detector2.ValidatorIndex(valSet))
}

func TestAddressBasedDetector_GetValidator(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Detector for validator at index 3
	detector := NewAddressBasedDetector(validators[3].Address)
	val := detector.GetValidator(valSet)
	require.NotNil(t, val)
	assert.Equal(t, uint16(3), val.Index)

	// Detector for non-validator
	detector2 := NewAddressBasedDetector([]byte("not an address"))
	val2 := detector2.GetValidator(valSet)
	assert.Nil(t, val2)
}

func TestAddressBasedDetector_Address(t *testing.T) {
	address := []byte{1, 2, 3, 4}
	detector := NewAddressBasedDetector(address)

	retrieved := detector.Address()
	assert.Equal(t, address, retrieved)

	// Verify it's a copy
	retrieved[0] = 99
	assert.Equal(t, byte(1), detector.address[0])
}

func TestNewValidatorStatusTracker(t *testing.T) {
	detector := NewKeyBasedDetector([]byte{1, 2, 3})
	tracker := NewValidatorStatusTracker(detector, nil)
	require.NotNil(t, tracker)
	assert.False(t, tracker.IsValidator())
	assert.Equal(t, -1, tracker.ValidatorIndex())
}

func TestValidatorStatusTracker_Update_BecomesValidator(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	detector := NewKeyBasedDetector(validators[1].PublicKey)

	var callbackCalled atomic.Bool
	var callbackIsValidator bool
	var callbackIndex int

	callback := func(isValidator bool, index int, _ *Validator) {
		callbackCalled.Store(true)
		callbackIsValidator = isValidator
		callbackIndex = index
	}

	tracker := NewValidatorStatusTracker(detector, callback)

	// Initial state: not a validator (empty set)
	assert.False(t, tracker.IsValidator())

	// Update with validator set containing this node
	changed := tracker.Update(valSet)
	assert.True(t, changed)
	assert.True(t, tracker.IsValidator())
	assert.Equal(t, 1, tracker.ValidatorIndex())

	// Callback should have been called
	assert.True(t, callbackCalled.Load())
	assert.True(t, callbackIsValidator)
	assert.Equal(t, 1, callbackIndex)
}

func TestValidatorStatusTracker_Update_NoChange(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	detector := NewKeyBasedDetector(validators[1].PublicKey)

	var callCount atomic.Int32
	callback := func(_ bool, _ int, _ *Validator) {
		callCount.Add(1)
	}

	tracker := NewValidatorStatusTracker(detector, callback)

	// First update
	changed := tracker.Update(valSet)
	assert.True(t, changed)
	assert.Equal(t, int32(1), callCount.Load())

	// Second update with same set - no change
	changed = tracker.Update(valSet)
	assert.False(t, changed)
	assert.Equal(t, int32(1), callCount.Load()) // Callback not called again
}

func TestValidatorStatusTracker_Update_LeavesValidatorSet(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	// Create another set without validator 1
	newValidators := []*Validator{validators[0], validators[2], validators[3]}
	newValSet, _ := NewSimpleValidatorSet(newValidators, 1)

	detector := NewKeyBasedDetector(validators[1].PublicKey)

	var lastIsValidator bool
	callback := func(isValidator bool, _ int, _ *Validator) {
		lastIsValidator = isValidator
	}

	tracker := NewValidatorStatusTracker(detector, callback)

	// Becomes validator
	tracker.Update(valSet)
	assert.True(t, tracker.IsValidator())
	assert.True(t, lastIsValidator)

	// Leaves validator set
	changed := tracker.Update(newValSet)
	assert.True(t, changed)
	assert.False(t, tracker.IsValidator())
	assert.False(t, lastIsValidator)
}

func TestValidatorStatusTracker_Validator(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	detector := NewKeyBasedDetector(validators[2].PublicKey)
	tracker := NewValidatorStatusTracker(detector, nil)

	// Before update, no validator info
	assert.Nil(t, tracker.Validator())

	// After update
	tracker.Update(valSet)
	val := tracker.Validator()
	require.NotNil(t, val)
	assert.Equal(t, uint16(2), val.Index)
}

func TestValidatorStatusTracker_SetCallback(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	detector := NewKeyBasedDetector(validators[0].PublicKey)

	var firstCalled, secondCalled atomic.Bool

	tracker := NewValidatorStatusTracker(detector, func(_ bool, _ int, _ *Validator) {
		firstCalled.Store(true)
	})

	tracker.Update(valSet)
	assert.True(t, firstCalled.Load())

	// Change callback
	tracker.SetCallback(func(_ bool, _ int, _ *Validator) {
		secondCalled.Store(true)
	})

	// Simulate leaving and rejoining
	emptySet, _ := NewSimpleValidatorSet([]*Validator{}, 0)
	tracker.Update(emptySet)
	tracker.Update(valSet)

	assert.True(t, secondCalled.Load())
}

func TestValidatorStatusTracker_NilCallback(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)

	detector := NewKeyBasedDetector(validators[0].PublicKey)
	tracker := NewValidatorStatusTracker(detector, nil)

	// Should not panic
	changed := tracker.Update(valSet)
	assert.True(t, changed)
	assert.True(t, tracker.IsValidator())
}

func TestValidatorStatusTracker_ConcurrentAccess(t *testing.T) {
	validators := makeTestValidators(4)
	valSet, _ := NewSimpleValidatorSet(validators, 0)
	emptySet, _ := NewSimpleValidatorSet([]*Validator{}, 0)

	detector := NewKeyBasedDetector(validators[0].PublicKey)
	tracker := NewValidatorStatusTracker(detector, nil)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				tracker.Update(valSet)
			} else {
				tracker.Update(emptySet)
			}
		}()
		go func() {
			defer wg.Done()
			_ = tracker.IsValidator()
		}()
		go func() {
			defer wg.Done()
			_ = tracker.ValidatorIndex()
		}()
	}
	wg.Wait()
}

func TestValidatorStatusTracker_IndexChange(t *testing.T) {
	// Create two validator sets where the same validator has different indices
	pub1, _, _ := ed25519.GenerateKey(rand.Reader)
	pub2, _, _ := ed25519.GenerateKey(rand.Reader)
	pub3, _, _ := ed25519.GenerateKey(rand.Reader)

	validators1 := []*Validator{
		{Index: 0, Address: pub1[:20], PublicKey: pub1, VotingPower: 100},
		{Index: 1, Address: pub2[:20], PublicKey: pub2, VotingPower: 100},
	}
	valSet1, _ := NewSimpleValidatorSet(validators1, 0)

	// Reordered - pub2 is now at index 0
	validators2 := []*Validator{
		{Index: 0, Address: pub2[:20], PublicKey: pub2, VotingPower: 100},
		{Index: 1, Address: pub1[:20], PublicKey: pub1, VotingPower: 100},
		{Index: 2, Address: pub3[:20], PublicKey: pub3, VotingPower: 100},
	}
	valSet2, _ := NewSimpleValidatorSet(validators2, 1)

	detector := NewKeyBasedDetector(pub1)

	var lastIndex int
	callback := func(_ bool, index int, _ *Validator) {
		lastIndex = index
	}

	tracker := NewValidatorStatusTracker(detector, callback)

	// First set - index 0
	tracker.Update(valSet1)
	assert.Equal(t, 0, lastIndex)
	assert.Equal(t, 0, tracker.ValidatorIndex())

	// Second set - index 1 (reordered)
	changed := tracker.Update(valSet2)
	assert.True(t, changed) // Index changed even though still a validator
	assert.Equal(t, 1, lastIndex)
	assert.Equal(t, 1, tracker.ValidatorIndex())
}
