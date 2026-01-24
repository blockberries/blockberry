// Package statestore provides merkleized state storage interface and implementations.
package statestore

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
func (p *Proof) Verify(rootHash []byte) (bool, error) {
	if p == nil {
		return false, nil
	}
	// Proof verification is delegated to the store implementation
	// since it requires access to the ICS23 spec
	return false, nil
}
