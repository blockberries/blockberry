package looseberry

import (
	"github.com/blockberries/blockberry/mempool"
	lbtypes "github.com/blockberries/looseberry/types"
)

// validatorSetAdapter adapts blockberry's ValidatorSet to looseberry's ValidatorSet.
type validatorSetAdapter struct {
	bbValidatorSet mempool.ValidatorSet
	lbValidatorSet lbtypes.ValidatorSet
}

// newValidatorSetAdapter creates a new validator set adapter.
func newValidatorSetAdapter(bb mempool.ValidatorSet) *validatorSetAdapter {
	// Create looseberry validators from blockberry validator set
	validators := make([]*lbtypes.Validator, bb.Count())
	for i := 0; i < bb.Count(); i++ {
		pk := bb.GetPublicKey(uint16(i))
		validators[i] = &lbtypes.Validator{
			Index:     uint16(i),
			PublicKey: lbtypes.PublicKey(pk),
			Power:     1, // Default power
		}
	}

	// Create looseberry validator set
	lbValSet := lbtypes.NewSimpleValidatorSet(validators, 0)

	return &validatorSetAdapter{
		bbValidatorSet: bb,
		lbValidatorSet: lbValSet,
	}
}

// looseberryValidatorSet is a wrapper that implements looseberry's ValidatorSet
// using a blockberry ValidatorSet.
type looseberryValidatorSet struct {
	bb mempool.ValidatorSet
}

// newLooseberryValidatorSet creates a looseberry-compatible validator set.
func newLooseberryValidatorSet(bb mempool.ValidatorSet) *looseberryValidatorSet {
	return &looseberryValidatorSet{bb: bb}
}

// Count returns the number of validators.
func (vs *looseberryValidatorSet) Count() int {
	return vs.bb.Count()
}

// GetByIndex returns a validator by index.
func (vs *looseberryValidatorSet) GetByIndex(index uint16) *lbtypes.Validator {
	pk := vs.bb.GetPublicKey(index)
	if pk == nil {
		return nil
	}
	return &lbtypes.Validator{
		Index:     index,
		PublicKey: lbtypes.PublicKey(pk),
		Power:     1,
	}
}

// Contains returns true if the index is a valid validator.
func (vs *looseberryValidatorSet) Contains(index uint16) bool {
	return vs.bb.GetPublicKey(index) != nil
}

// F returns the Byzantine fault tolerance threshold.
func (vs *looseberryValidatorSet) F() int {
	return vs.bb.F()
}

// Quorum returns the quorum size.
func (vs *looseberryValidatorSet) Quorum() int {
	return vs.bb.Quorum()
}

// Epoch returns the epoch number.
func (vs *looseberryValidatorSet) Epoch() uint64 {
	return 0 // Would need to be tracked separately
}

// VerifySignature verifies a signature from a validator.
func (vs *looseberryValidatorSet) VerifySignature(validatorIdx uint16, digest lbtypes.Hash, sig lbtypes.Signature) bool {
	return vs.bb.VerifySignature(validatorIdx, digest[:], sig.Bytes())
}

// Validators returns all validators.
func (vs *looseberryValidatorSet) Validators() []*lbtypes.Validator {
	count := vs.bb.Count()
	validators := make([]*lbtypes.Validator, count)
	for i := 0; i < count; i++ {
		validators[i] = vs.GetByIndex(uint16(i))
	}
	return validators
}

// Verify interface implementation
var _ lbtypes.ValidatorSet = (*looseberryValidatorSet)(nil)
