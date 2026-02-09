// Package statestore provides merkleized state storage interface and implementations.
package statestore

import (
	"bytes"
	"fmt"

	ics23 "github.com/cosmos/ics23/go"

	"github.com/blockberries/blockberry/pkg/types"
)

// StateStore defines the interface for merkleized key-value state storage.
// Implementations must be safe for concurrent use.
type StateStore interface {
	// Get retrieves the value for a key.
	// Returns nil, nil if the key does not exist.
	Get(key []byte) ([]byte, error)

	// Has checks if a key exists.
	Has(key []byte) (bool, error)

	// Set stores a key-value pair in the working tree.
	// The change is not persisted until Commit is called.
	Set(key []byte, value []byte) error

	// Delete removes a key from the working tree.
	// The change is not persisted until Commit is called.
	Delete(key []byte) error

	// Commit saves the current working tree as a new version.
	// Returns the root hash and version number.
	Commit() (hash []byte, version int64, err error)

	// RootHash returns the root hash of the current tree.
	// For the working tree, this reflects uncommitted changes.
	RootHash() []byte

	// Version returns the latest committed version number.
	// Returns 0 if no versions have been committed.
	Version() int64

	// LoadVersion loads a specific version of the tree.
	// All subsequent operations will be based on this version.
	LoadVersion(version int64) error

	// GetProof returns a merkle proof for a key.
	// The proof can be used to verify the key's existence or non-existence.
	GetProof(key []byte) (*Proof, error)

	// Close closes the store and releases resources.
	Close() error
}

// Proof represents a merkle proof for a key in the state store.
// It can prove either the existence or non-existence of a key.
type Proof struct {
	// Key is the key this proof is for.
	Key []byte

	// Value is the value if the key exists, nil otherwise.
	Value []byte

	// Exists indicates whether the key exists in the tree.
	Exists bool

	// RootHash is the root hash of the tree this proof was generated from.
	RootHash []byte

	// Version is the version of the tree this proof was generated from.
	Version int64

	// ProofBytes contains the serialized ICS23 commitment proof.
	ProofBytes []byte
}

// Verify verifies the proof against the given root hash.
// Returns true if the proof is valid, false otherwise.
// For existence proofs, it verifies the key-value pair exists at the root.
// For non-existence proofs, it verifies the key does not exist.
func (p *Proof) Verify(rootHash []byte) (bool, error) {
	if p == nil {
		return false, types.ErrInvalidProof
	}

	if len(p.ProofBytes) == 0 {
		return false, types.ErrInvalidProof
	}

	if len(rootHash) == 0 {
		return false, fmt.Errorf("%w: empty root hash", types.ErrInvalidProof)
	}

	// Unmarshal the ICS23 commitment proof
	var commitmentProof ics23.CommitmentProof
	if err := commitmentProof.Unmarshal(p.ProofBytes); err != nil {
		return false, fmt.Errorf("%w: failed to unmarshal proof: %v", types.ErrInvalidProof, err)
	}

	// Verify by computing the root hash from the proof and comparing.
	if p.Exists {
		existProof := commitmentProof.GetExist()
		if existProof == nil {
			return false, fmt.Errorf("%w: not an existence proof", types.ErrInvalidProof)
		}
		// Verify the proof key and value match
		if !bytes.Equal(existProof.Key, p.Key) || !bytes.Equal(existProof.Value, p.Value) {
			return false, nil
		}
		calculatedRoot, err := existProof.Calculate()
		if err != nil {
			return false, fmt.Errorf("%w: failed to calculate root: %v", types.ErrInvalidProof, err)
		}
		return bytes.Equal(calculatedRoot, rootHash), nil
	}

	// Non-existence proof
	nonExistProof := commitmentProof.GetNonexist()
	if nonExistProof == nil {
		return false, fmt.Errorf("%w: not a non-existence proof", types.ErrInvalidProof)
	}
	if !bytes.Equal(nonExistProof.Key, p.Key) {
		return false, nil
	}
	// Verify neighbor proofs calculate to the same root
	if nonExistProof.Left != nil {
		leftRoot, err := nonExistProof.Left.Calculate()
		if err != nil {
			return false, fmt.Errorf("%w: failed to calculate left root: %v", types.ErrInvalidProof, err)
		}
		if !bytes.Equal(leftRoot, rootHash) {
			return false, nil
		}
	}
	if nonExistProof.Right != nil {
		rightRoot, err := nonExistProof.Right.Calculate()
		if err != nil {
			return false, fmt.Errorf("%w: failed to calculate right root: %v", types.ErrInvalidProof, err)
		}
		if !bytes.Equal(rightRoot, rootHash) {
			return false, nil
		}
	}
	// At least one neighbor must be present
	if nonExistProof.Left == nil && nonExistProof.Right == nil {
		return false, fmt.Errorf("%w: no neighbor proofs", types.ErrInvalidProof)
	}
	return true, nil
}

// VerifyConsistent checks that the proof's stored root hash matches the given root hash.
// This is useful for verifying that a proof was generated from a specific state.
func (p *Proof) VerifyConsistent(rootHash []byte) bool {
	if p == nil || len(p.RootHash) == 0 || len(rootHash) == 0 {
		return false
	}
	return bytes.Equal(p.RootHash, rootHash)
}
