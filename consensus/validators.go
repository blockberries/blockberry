package consensus

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"
)

// Common validator errors.
var (
	ErrValidatorNotFound    = errors.New("validator not found")
	ErrDuplicateValidator   = errors.New("duplicate validator")
	ErrInvalidVotingPower   = errors.New("invalid voting power")
	ErrInvalidPublicKey     = errors.New("invalid public key")
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrInsufficientQuorum   = errors.New("insufficient quorum")
	ErrValidatorSetNotFound = errors.New("validator set not found")
)

// SimpleValidatorSet is a basic implementation of ValidatorSet.
// It stores validators in a slice and provides O(n) lookups.
// For larger validator sets, consider using an indexed implementation.
type SimpleValidatorSet struct {
	validators []*Validator
	totalPower int64
	epoch      uint64
	mu         sync.RWMutex
}

// NewSimpleValidatorSet creates a new SimpleValidatorSet from a list of validators.
func NewSimpleValidatorSet(validators []*Validator, epoch uint64) (*SimpleValidatorSet, error) {
	if len(validators) == 0 {
		return &SimpleValidatorSet{
			validators: make([]*Validator, 0),
			epoch:      epoch,
		}, nil
	}

	// Validate and calculate total voting power
	seen := make(map[string]bool)
	var totalPower int64

	for i, v := range validators {
		if v == nil {
			return nil, fmt.Errorf("validator at index %d is nil", i)
		}
		if v.VotingPower <= 0 {
			return nil, fmt.Errorf("%w: validator %d has power %d", ErrInvalidVotingPower, i, v.VotingPower)
		}

		// Check for duplicates by address
		addrKey := string(v.Address)
		if seen[addrKey] {
			return nil, fmt.Errorf("%w: address %x", ErrDuplicateValidator, v.Address)
		}
		seen[addrKey] = true

		totalPower += v.VotingPower
	}

	// Copy validators to avoid external mutation
	validatorsCopy := make([]*Validator, len(validators))
	for i, v := range validators {
		//nolint:gosec // Safe conversion: validator count limited by consensus protocol
		validatorsCopy[i] = &Validator{
			Index:            uint16(i),
			Address:          append([]byte{}, v.Address...),
			PublicKey:        append([]byte{}, v.PublicKey...),
			VotingPower:      v.VotingPower,
			ProposerPriority: v.ProposerPriority,
		}
	}

	return &SimpleValidatorSet{
		validators: validatorsCopy,
		totalPower: totalPower,
		epoch:      epoch,
	}, nil
}

// Count returns the number of validators.
func (vs *SimpleValidatorSet) Count() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return len(vs.validators)
}

// GetByIndex returns the validator at the given index.
// The returned struct is a deep copy; callers may safely modify it.
func (vs *SimpleValidatorSet) GetByIndex(index uint16) *Validator {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if int(index) >= len(vs.validators) {
		return nil
	}
	v := vs.validators[index]
	return &Validator{
		Index:            v.Index,
		Address:          append([]byte(nil), v.Address...),
		PublicKey:        append([]byte(nil), v.PublicKey...),
		VotingPower:      v.VotingPower,
		ProposerPriority: v.ProposerPriority,
	}
}

// GetByAddress returns the validator with the given address.
// The returned struct is a deep copy; callers may safely modify it.
func (vs *SimpleValidatorSet) GetByAddress(address []byte) *Validator {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	for _, v := range vs.validators {
		if bytes.Equal(v.Address, address) {
			return &Validator{
				Index:            v.Index,
				Address:          append([]byte(nil), v.Address...),
				PublicKey:        append([]byte(nil), v.PublicKey...),
				VotingPower:      v.VotingPower,
				ProposerPriority: v.ProposerPriority,
			}
		}
	}
	return nil
}

// Contains returns true if the address is a validator.
func (vs *SimpleValidatorSet) Contains(address []byte) bool {
	return vs.GetByAddress(address) != nil
}

// GetProposer returns the proposer for the given height and round.
// Uses a simple round-robin algorithm based on proposer priority.
func (vs *SimpleValidatorSet) GetProposer(height int64, round int32) *Validator {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if len(vs.validators) == 0 {
		return nil
	}

	// Simple deterministic selection: (height + round) mod n
	// For production, use weighted proposer selection with priority
	idx := int((height + int64(round))) % len(vs.validators)
	return vs.validators[idx]
}

// TotalVotingPower returns the sum of all voting power.
func (vs *SimpleValidatorSet) TotalVotingPower() int64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.totalPower
}

// Quorum returns the minimum voting power for consensus (2f+1).
func (vs *SimpleValidatorSet) Quorum() int64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// For BFT, quorum is 2f+1 where f is max Byzantine nodes
	// f = (n-1)/3, so quorum = 2*((n-1)/3) + 1 = (2n+1)/3
	// In terms of voting power: quorum = 2/3 * total + 1
	// Use overflow-safe calculation for large values
	if vs.totalPower > math.MaxInt64/2 {
		// For very large values, use division-first approach (slightly conservative)
		return vs.totalPower/3*2 + 1
	}
	return (2*vs.totalPower)/3 + 1
}

// F returns the maximum number of Byzantine validators tolerated.
func (vs *SimpleValidatorSet) F() int {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return (len(vs.validators) - 1) / 3
}

// Epoch returns the epoch number for this validator set.
func (vs *SimpleValidatorSet) Epoch() uint64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.epoch
}

// VerifyCommit verifies a commit against this validator set.
func (vs *SimpleValidatorSet) VerifyCommit(height int64, blockHash []byte, commit *Commit) error {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	if commit == nil {
		return errors.New("nil commit")
	}

	if commit.Height != height {
		return fmt.Errorf("commit height %d does not match expected %d", commit.Height, height)
	}

	if !bytes.Equal(commit.BlockHash, blockHash) {
		return fmt.Errorf("commit block hash does not match")
	}

	// Calculate voting power from valid signatures
	var votingPower int64
	for _, sig := range commit.Signatures {
		if sig.Signature == nil {
			continue // Skip absent validators
		}

		if int(sig.ValidatorIndex) >= len(vs.validators) {
			continue // Invalid validator index
		}

		validator := vs.validators[sig.ValidatorIndex]

		// Verify signature (assuming Ed25519)
		if len(validator.PublicKey) != ed25519.PublicKeySize {
			continue
		}

		// Validate signature length to prevent panic in ed25519.Verify
		if len(sig.Signature) != ed25519.SignatureSize {
			continue
		}

		// For production: construct proper sign bytes from height/round/blockHash
		signBytes := makeCommitSignBytes(height, commit.Round, blockHash)
		if !ed25519.Verify(validator.PublicKey, signBytes, sig.Signature) {
			continue // Invalid signature
		}

		votingPower += validator.VotingPower
	}

	// Use overflow-safe quorum calculation for large values
	var quorum int64
	if vs.totalPower > math.MaxInt64/2 {
		quorum = vs.totalPower/3*2 + 1
	} else {
		quorum = (2*vs.totalPower)/3 + 1
	}
	if votingPower < quorum {
		return fmt.Errorf("%w: got %d, need %d", ErrInsufficientQuorum, votingPower, quorum)
	}

	return nil
}

// Validators returns all validators (deep copy).
func (vs *SimpleValidatorSet) Validators() []*Validator {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	result := make([]*Validator, len(vs.validators))
	for i, v := range vs.validators {
		result[i] = &Validator{
			Index:            v.Index,
			Address:          append([]byte{}, v.Address...),
			PublicKey:        append([]byte{}, v.PublicKey...),
			VotingPower:      v.VotingPower,
			ProposerPriority: v.ProposerPriority,
		}
	}
	return result
}

// Copy returns a deep copy of the validator set.
func (vs *SimpleValidatorSet) Copy() *SimpleValidatorSet {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	validators := make([]*Validator, len(vs.validators))
	for i, v := range vs.validators {
		validators[i] = &Validator{
			Index:            v.Index,
			Address:          append([]byte{}, v.Address...),
			PublicKey:        append([]byte{}, v.PublicKey...),
			VotingPower:      v.VotingPower,
			ProposerPriority: v.ProposerPriority,
		}
	}

	return &SimpleValidatorSet{
		validators: validators,
		totalPower: vs.totalPower,
		epoch:      vs.epoch,
	}
}

// IndexedValidatorSet is an optimized ValidatorSet with O(1) address lookups.
// Use this for larger validator sets where O(n) lookups become expensive.
type IndexedValidatorSet struct {
	*SimpleValidatorSet
	byAddress map[string]*Validator
}

// NewIndexedValidatorSet creates a new IndexedValidatorSet.
func NewIndexedValidatorSet(validators []*Validator, epoch uint64) (*IndexedValidatorSet, error) {
	simple, err := NewSimpleValidatorSet(validators, epoch)
	if err != nil {
		return nil, err
	}

	// Build address index
	byAddress := make(map[string]*Validator, len(simple.validators))
	for _, v := range simple.validators {
		byAddress[string(v.Address)] = v
	}

	return &IndexedValidatorSet{
		SimpleValidatorSet: simple,
		byAddress:          byAddress,
	}, nil
}

// GetByAddress returns the validator with the given address (O(1)).
func (vs *IndexedValidatorSet) GetByAddress(address []byte) *Validator {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.byAddress[string(address)]
}

// Contains returns true if the address is a validator (O(1)).
func (vs *IndexedValidatorSet) Contains(address []byte) bool {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	_, ok := vs.byAddress[string(address)]
	return ok
}

// Copy returns a deep copy of the indexed validator set.
func (vs *IndexedValidatorSet) Copy() *IndexedValidatorSet {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	simple := vs.SimpleValidatorSet.Copy()
	byAddress := make(map[string]*Validator, len(simple.validators))
	for _, v := range simple.validators {
		byAddress[string(v.Address)] = v
	}

	return &IndexedValidatorSet{
		SimpleValidatorSet: simple,
		byAddress:          byAddress,
	}
}

// WeightedProposerSelection implements proposer selection weighted by voting power.
// This is used for PoS systems where validators with more stake propose more often.
type WeightedProposerSelection struct {
	validators   []*Validator
	totalPower   int64
	priorities   []int64 // Per-validator proposer priority
	lastProposer int     // Index of last proposer
}

// NewWeightedProposerSelection creates a new weighted proposer selection.
func NewWeightedProposerSelection(validators []*Validator) *WeightedProposerSelection {
	priorities := make([]int64, len(validators))
	var totalPower int64
	for _, v := range validators {
		totalPower += v.VotingPower
	}

	return &WeightedProposerSelection{
		validators:   validators,
		totalPower:   totalPower,
		priorities:   priorities,
		lastProposer: -1,
	}
}

// GetProposer returns the proposer for the current state.
// Call IncrementProposerPriority after each round to update.
func (wps *WeightedProposerSelection) GetProposer() *Validator {
	if len(wps.validators) == 0 {
		return nil
	}

	// Find validator with highest priority
	maxPriority := wps.priorities[0]
	maxIdx := 0
	for i, p := range wps.priorities {
		if p > maxPriority {
			maxPriority = p
			maxIdx = i
		}
	}

	return wps.validators[maxIdx]
}

// IncrementProposerPriority updates priorities after a round.
// Should be called after each block is committed.
func (wps *WeightedProposerSelection) IncrementProposerPriority() {
	if len(wps.validators) == 0 {
		return
	}

	// Find proposer (highest priority)
	maxPriority := wps.priorities[0]
	proposerIdx := 0
	for i, p := range wps.priorities {
		if p > maxPriority {
			maxPriority = p
			proposerIdx = i
		}
	}

	// Update priorities: add voting power, then subtract total from proposer
	for i, v := range wps.validators {
		wps.priorities[i] += v.VotingPower
	}
	wps.priorities[proposerIdx] -= wps.totalPower

	wps.lastProposer = proposerIdx
}

// ValidatorSetStore persists validator sets at different heights.
type ValidatorSetStore interface {
	// SaveValidatorSet persists a validator set at a height.
	SaveValidatorSet(height int64, valSet ValidatorSet) error

	// LoadValidatorSet retrieves the validator set at a height.
	LoadValidatorSet(height int64) (ValidatorSet, error)

	// LatestValidatorSet returns the most recent validator set.
	LatestValidatorSet() (ValidatorSet, error)

	// ValidatorSetAtHeight returns the validator set that was active at height.
	// This handles cases where validator sets change at epoch boundaries.
	ValidatorSetAtHeight(height int64) (ValidatorSet, error)
}

// InMemoryValidatorSetStore is an in-memory implementation of ValidatorSetStore.
// Useful for testing and development.
type InMemoryValidatorSetStore struct {
	sets          map[int64]ValidatorSet
	sortedHeights []int64 // Cached sorted heights for efficient lookups
	latest        int64
	mu            sync.RWMutex
}

// NewInMemoryValidatorSetStore creates a new in-memory validator set store.
func NewInMemoryValidatorSetStore() *InMemoryValidatorSetStore {
	return &InMemoryValidatorSetStore{
		sets:          make(map[int64]ValidatorSet),
		sortedHeights: make([]int64, 0),
		latest:        0,
	}
}

// SaveValidatorSet persists a validator set at a height.
func (s *InMemoryValidatorSetStore) SaveValidatorSet(height int64, valSet ValidatorSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if height already exists
	if _, exists := s.sets[height]; !exists {
		// Insert into sorted heights using binary search to maintain order
		idx, _ := slices.BinarySearch(s.sortedHeights, height)
		s.sortedHeights = slices.Insert(s.sortedHeights, idx, height)
	}

	s.sets[height] = valSet
	if height > s.latest {
		s.latest = height
	}
	return nil
}

// LoadValidatorSet retrieves the validator set at a height.
func (s *InMemoryValidatorSetStore) LoadValidatorSet(height int64) (ValidatorSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	valSet, ok := s.sets[height]
	if !ok {
		return nil, fmt.Errorf("%w at height %d", ErrValidatorSetNotFound, height)
	}
	return valSet, nil
}

// LatestValidatorSet returns the most recent validator set.
func (s *InMemoryValidatorSetStore) LatestValidatorSet() (ValidatorSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.latest == 0 {
		return nil, ErrValidatorSetNotFound
	}
	return s.sets[s.latest], nil
}

// ValidatorSetAtHeight returns the validator set active at the given height.
// Uses binary search on cached sorted heights for O(log n) lookup.
func (s *InMemoryValidatorSetStore) ValidatorSetAtHeight(height int64) (ValidatorSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.sortedHeights) == 0 {
		return nil, fmt.Errorf("%w at height %d", ErrValidatorSetNotFound, height)
	}

	// Find the validator set that was active at this height
	// (the most recent one at or before this height)
	// Binary search for the insertion point, then back up to find the active set
	idx, found := slices.BinarySearch(s.sortedHeights, height)

	var activeHeight int64
	if found {
		// Exact match
		activeHeight = s.sortedHeights[idx]
	} else if idx > 0 {
		// Use the height just before the insertion point
		activeHeight = s.sortedHeights[idx-1]
	} else {
		// height is less than all stored heights
		return nil, fmt.Errorf("%w at height %d", ErrValidatorSetNotFound, height)
	}

	return s.sets[activeHeight], nil
}

// makeCommitSignBytes creates the bytes to be signed for a commit.
// In production, this should match the canonical encoding used by the consensus.
func makeCommitSignBytes(height int64, round int32, blockHash []byte) []byte {
	// Simple encoding for demonstration
	// Production systems should use a canonical encoding like Amino or Protobuf
	data := make([]byte, 8+4+len(blockHash))
	data[0] = byte(height >> 56)
	data[1] = byte(height >> 48)
	data[2] = byte(height >> 40)
	data[3] = byte(height >> 32)
	data[4] = byte(height >> 24)
	data[5] = byte(height >> 16)
	data[6] = byte(height >> 8)
	data[7] = byte(height)
	data[8] = byte(round >> 24)
	data[9] = byte(round >> 16)
	data[10] = byte(round >> 8)
	data[11] = byte(round)
	copy(data[12:], blockHash)
	return data
}

// Verify interface compliance.
var (
	_ ValidatorSet = (*SimpleValidatorSet)(nil)
	_ ValidatorSet = (*IndexedValidatorSet)(nil)
)
