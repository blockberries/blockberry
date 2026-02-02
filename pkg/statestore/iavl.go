package statestore

import (
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/iavl"
	idb "github.com/cosmos/iavl/db"

	"github.com/blockberries/blockberry/pkg/types"
)

// IAVLStore implements StateStore using a cosmos/iavl merkle tree.
type IAVLStore struct {
	tree     *iavl.MutableTree
	db       idb.DB
	pruneCfg *StatePruneConfig
	pruning  bool
	mu       sync.RWMutex
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

// PruneVersions removes old versions of the state.
// Versions that should be kept according to the config are preserved.
// Note: IAVL only supports deleting versions up to a point, not selective deletion.
// If checkpoint versions need to be kept, this method will keep the nearest checkpoint.
func (s *IAVLStore) PruneVersions(beforeVersion int64) (*StatePruneResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()

	// Validate inputs
	if beforeVersion <= 0 {
		return nil, ErrInvalidPruneVersion
	}

	currentVersion := s.tree.Version()
	if beforeVersion > currentVersion {
		return nil, fmt.Errorf("%w: version %d exceeds current version %d",
			ErrInvalidPruneVersion, beforeVersion, currentVersion)
	}

	// Check if pruning is already in progress
	if s.pruning {
		return nil, ErrStatePruningInProgress
	}
	s.pruning = true
	defer func() { s.pruning = false }()

	// Get available versions (IAVL returns []int)
	availVersions := s.tree.AvailableVersions()
	if len(availVersions) == 0 {
		return &StatePruneResult{
			OldestVersion: 0,
			NewestVersion: currentVersion,
			Duration:      time.Since(start),
		}, nil
	}

	// Find the actual prune target, considering checkpoints
	pruneTarget := beforeVersion
	if s.pruneCfg != nil && s.pruneCfg.KeepEvery > 0 {
		// Find the highest checkpoint below beforeVersion that we want to keep
		for v := beforeVersion - 1; v > 0; v-- {
			if s.pruneCfg.ShouldKeep(v, currentVersion) {
				// Stop before this checkpoint
				pruneTarget = v
				break
			}
		}
	}

	// Count versions that will be deleted
	var prunedCount int64
	for _, v := range availVersions {
		if int64(v) < pruneTarget {
			prunedCount++
		}
	}

	// Delete versions up to pruneTarget-1 (IAVL's DeleteVersionsTo is inclusive)
	if pruneTarget > 1 {
		if err := s.tree.DeleteVersionsTo(pruneTarget - 1); err != nil {
			return nil, fmt.Errorf("deleting versions to %d: %w", pruneTarget-1, err)
		}
	}

	// Get new oldest version
	availVersions = s.tree.AvailableVersions()
	var oldestVersion int64
	if len(availVersions) > 0 {
		oldestVersion = int64(availVersions[0])
	}

	return &StatePruneResult{
		PrunedCount:   prunedCount,
		OldestVersion: oldestVersion,
		NewestVersion: currentVersion,
		Duration:      time.Since(start),
	}, nil
}

// AvailableVersions returns the range of available versions.
func (s *IAVLStore) AvailableVersions() (oldest, newest int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.tree.AvailableVersions()
	if len(versions) == 0 {
		return 0, 0
	}

	return int64(versions[0]), int64(versions[len(versions)-1])
}

// StatePruneConfig returns the current state pruning configuration.
func (s *IAVLStore) StatePruneConfig() *StatePruneConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pruneCfg
}

// SetStatePruneConfig updates the state pruning configuration.
func (s *IAVLStore) SetStatePruneConfig(cfg *StatePruneConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneCfg = cfg
}

// AllVersions returns all available versions.
func (s *IAVLStore) AllVersions() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions := s.tree.AvailableVersions()
	result := make([]int64, len(versions))
	for i, v := range versions {
		result[i] = int64(v)
	}
	return result
}

// Ensure IAVLStore implements PrunableStateStore.
var _ PrunableStateStore = (*IAVLStore)(nil)
