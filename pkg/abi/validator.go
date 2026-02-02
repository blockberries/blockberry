package abi

import "bytes"

// Validator represents a consensus validator.
type Validator struct {
	// Address is the validator's unique address (typically derived from PublicKey).
	Address []byte

	// PublicKey is the validator's public key for signature verification.
	PublicKey []byte

	// VotingPower is the validator's voting power (stake weight).
	VotingPower int64

	// Index is the validator's position in the validator set (0-indexed).
	Index uint16
}

// Equal returns true if two validators have the same address and public key.
func (v Validator) Equal(other Validator) bool {
	return bytes.Equal(v.Address, other.Address) &&
		bytes.Equal(v.PublicKey, other.PublicKey)
}

// ValidatorUpdate represents a change to the validator set.
type ValidatorUpdate struct {
	// PublicKey is the validator's public key.
	PublicKey []byte

	// Power is the new voting power. Power = 0 removes the validator.
	Power int64
}

// IsRemoval returns true if this update removes the validator.
func (u ValidatorUpdate) IsRemoval() bool {
	return u.Power == 0
}

// ValidatorSet represents an immutable set of validators.
type ValidatorSet interface {
	// Validators returns all validators in the set.
	Validators() []Validator

	// GetByIndex returns the validator at the given index, or nil if not found.
	GetByIndex(index uint16) *Validator

	// GetByAddress returns the validator with the given address, or nil if not found.
	GetByAddress(addr []byte) *Validator

	// Count returns the number of validators.
	Count() int

	// TotalVotingPower returns the sum of all validators' voting power.
	TotalVotingPower() int64

	// Proposer returns the proposer for the given height and round.
	Proposer(height uint64, round uint32) *Validator

	// VerifySignature verifies a signature from a validator.
	VerifySignature(validatorIdx uint16, digest, sig []byte) bool

	// Quorum returns the minimum voting power for consensus (typically 2f+1).
	Quorum() int64

	// F returns the maximum number of Byzantine validators tolerated.
	F() int

	// Epoch returns the epoch number for this validator set.
	Epoch() uint64

	// Copy returns a deep copy of the validator set.
	Copy() ValidatorSet
}

// SimpleValidatorSet is a basic implementation of ValidatorSet.
type SimpleValidatorSet struct {
	validators       []Validator
	totalVotingPower int64
	epoch            uint64
}

// NewSimpleValidatorSet creates a new SimpleValidatorSet from the given validators.
func NewSimpleValidatorSet(validators []Validator, epoch uint64) *SimpleValidatorSet {
	vs := &SimpleValidatorSet{
		validators: make([]Validator, len(validators)),
		epoch:      epoch,
	}
	for i, v := range validators {
		vs.validators[i] = v
		vs.validators[i].Index = uint16(i)
		vs.totalVotingPower += v.VotingPower
	}
	return vs
}

// Validators returns all validators in the set.
// Returns deep copies to prevent external mutation.
func (vs *SimpleValidatorSet) Validators() []Validator {
	result := make([]Validator, len(vs.validators))
	for i, v := range vs.validators {
		result[i] = Validator{
			Address:     append([]byte(nil), v.Address...),
			PublicKey:   append([]byte(nil), v.PublicKey...),
			VotingPower: v.VotingPower,
			Index:       v.Index,
		}
	}
	return result
}

// GetByIndex returns the validator at the given index.
// Returns a deep copy to prevent external mutation.
func (vs *SimpleValidatorSet) GetByIndex(index uint16) *Validator {
	if int(index) >= len(vs.validators) {
		return nil
	}
	v := vs.validators[index]
	return &Validator{
		Address:     append([]byte(nil), v.Address...),
		PublicKey:   append([]byte(nil), v.PublicKey...),
		VotingPower: v.VotingPower,
		Index:       v.Index,
	}
}

// GetByAddress returns the validator with the given address.
// Returns a deep copy to prevent external mutation.
func (vs *SimpleValidatorSet) GetByAddress(addr []byte) *Validator {
	for i := range vs.validators {
		if bytes.Equal(vs.validators[i].Address, addr) {
			v := vs.validators[i]
			return &Validator{
				Address:     append([]byte(nil), v.Address...),
				PublicKey:   append([]byte(nil), v.PublicKey...),
				VotingPower: v.VotingPower,
				Index:       v.Index,
			}
		}
	}
	return nil
}

// Count returns the number of validators.
func (vs *SimpleValidatorSet) Count() int {
	return len(vs.validators)
}

// TotalVotingPower returns the sum of all validators' voting power.
func (vs *SimpleValidatorSet) TotalVotingPower() int64 {
	return vs.totalVotingPower
}

// Proposer returns the proposer for the given height and round using round-robin.
// Returns a deep copy to prevent external mutation.
func (vs *SimpleValidatorSet) Proposer(height uint64, round uint32) *Validator {
	if len(vs.validators) == 0 {
		return nil
	}
	idx := (height + uint64(round)) % uint64(len(vs.validators))
	v := vs.validators[idx]
	return &Validator{
		Address:     append([]byte(nil), v.Address...),
		PublicKey:   append([]byte(nil), v.PublicKey...),
		VotingPower: v.VotingPower,
		Index:       v.Index,
	}
}

// VerifySignature verifies a signature from a validator.
// This is a stub - real implementation would use crypto verification.
func (vs *SimpleValidatorSet) VerifySignature(validatorIdx uint16, digest, sig []byte) bool {
	// Stub implementation - real code would verify cryptographically
	return validatorIdx < uint16(len(vs.validators)) && len(sig) > 0
}

// Quorum returns the minimum voting power for consensus (2f+1).
func (vs *SimpleValidatorSet) Quorum() int64 {
	f := vs.F()
	return int64(2*f + 1)
}

// F returns the maximum number of Byzantine validators tolerated.
func (vs *SimpleValidatorSet) F() int {
	return (len(vs.validators) - 1) / 3
}

// Epoch returns the epoch number for this validator set.
func (vs *SimpleValidatorSet) Epoch() uint64 {
	return vs.epoch
}

// Copy returns a deep copy of the validator set.
func (vs *SimpleValidatorSet) Copy() ValidatorSet {
	validators := make([]Validator, len(vs.validators))
	for i, v := range vs.validators {
		validators[i] = Validator{
			Address:     append([]byte(nil), v.Address...),
			PublicKey:   append([]byte(nil), v.PublicKey...),
			VotingPower: v.VotingPower,
			Index:       v.Index,
		}
	}
	return &SimpleValidatorSet{
		validators:       validators,
		totalVotingPower: vs.totalVotingPower,
		epoch:            vs.epoch,
	}
}
