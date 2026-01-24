package statestore

import (
	"fmt"
	"sync"

	"github.com/cosmos/iavl"
	idb "github.com/cosmos/iavl/db"

	"github.com/blockberries/blockberry/types"
)

// IAVLStore implements StateStore using a cosmos/iavl merkle tree.
type IAVLStore struct {
	tree *iavl.MutableTree
	db   idb.DB
	mu   sync.RWMutex
}

// NewIAVLStore creates a new IAVL-backed state store.
// path is the directory for persistent storage.
// cacheSize is the number of nodes to cache in memory.
func NewIAVLStore(path string, cacheSize int) (*IAVLStore, error) {
	db, err := idb.NewGoLevelDB("state", path)
	if err != nil {
		return nil, fmt.Errorf("opening leveldb for iavl: %w", err)
	}

	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger())

	// Load the latest version if it exists
	if _, err := tree.Load(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading iavl tree: %w", err)
	}

	return &IAVLStore{
		tree: tree,
		db:   db,
	}, nil
}

// NewMemoryIAVLStore creates an in-memory IAVL store for testing.
func NewMemoryIAVLStore(cacheSize int) (*IAVLStore, error) {
	db := idb.NewMemDB()
	tree := iavl.NewMutableTree(db, cacheSize, false, iavl.NewNopLogger())

	return &IAVLStore{
		tree: tree,
		db:   db,
	}, nil
}

// Get retrieves the value for a key.
func (s *IAVLStore) Get(key []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, err := s.tree.Get(key)
	if err != nil {
		return nil, fmt.Errorf("getting key: %w", err)
	}
	return value, nil
}

// Has checks if a key exists.
func (s *IAVLStore) Has(key []byte) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	has, err := s.tree.Has(key)
	if err != nil {
		return false, fmt.Errorf("checking key existence: %w", err)
	}
	return has, nil
}

// Set stores a key-value pair in the working tree.
func (s *IAVLStore) Set(key []byte, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key == nil {
		return fmt.Errorf("key cannot be nil")
	}
	if value == nil {
		return fmt.Errorf("value cannot be nil")
	}

	_, err := s.tree.Set(key, value)
	if err != nil {
		return fmt.Errorf("setting key: %w", err)
	}
	return nil
}

// Delete removes a key from the working tree.
func (s *IAVLStore) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key == nil {
		return fmt.Errorf("key cannot be nil")
	}

	_, _, err := s.tree.Remove(key)
	if err != nil {
		return fmt.Errorf("removing key: %w", err)
	}
	return nil
}

// Commit saves the current working tree as a new version.
func (s *IAVLStore) Commit() ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash, version, err := s.tree.SaveVersion()
	if err != nil {
		return nil, 0, fmt.Errorf("saving version: %w", err)
	}
	return hash, version, nil
}

// RootHash returns the root hash of the current working tree.
func (s *IAVLStore) RootHash() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tree.WorkingHash()
}

// Version returns the latest committed version number.
func (s *IAVLStore) Version() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tree.Version()
}

// LoadVersion loads a specific version of the tree.
func (s *IAVLStore) LoadVersion(version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.tree.LoadVersion(version)
	if err != nil {
		return fmt.Errorf("loading version %d: %w", version, err)
	}
	return nil
}

// GetProof returns a merkle proof for a key.
func (s *IAVLStore) GetProof(key []byte) (*Proof, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if key == nil {
		return nil, types.ErrKeyNotFound
	}

	// Get the value to check existence
	value, err := s.tree.Get(key)
	if err != nil {
		return nil, fmt.Errorf("getting value for proof: %w", err)
	}

	// Get the ICS23 commitment proof
	proof, err := s.tree.GetProof(key)
	if err != nil {
		return nil, fmt.Errorf("getting proof: %w", err)
	}

	// Serialize the proof
	proofBytes, err := proof.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshaling proof: %w", err)
	}

	return &Proof{
		Key:        key,
		Value:      value,
		Exists:     value != nil,
		RootHash:   s.tree.WorkingHash(),
		Version:    s.tree.Version(),
		ProofBytes: proofBytes,
	}, nil
}

// Close closes the store and releases resources.
func (s *IAVLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Close()
}

// VersionExists checks if a specific version exists.
func (s *IAVLStore) VersionExists(version int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tree.VersionExists(version)
}

// GetVersioned retrieves a value at a specific version.
func (s *IAVLStore) GetVersioned(key []byte, version int64) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, err := s.tree.GetVersioned(key, version)
	if err != nil {
		return nil, fmt.Errorf("getting versioned key: %w", err)
	}
	return value, nil
}
