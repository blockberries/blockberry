package statestore

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewIAVLStore(t *testing.T) {
	t.Run("creates new store", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "state")

		store, err := NewIAVLStore(path, 100)
		require.NoError(t, err)
		require.NotNil(t, store)
		defer store.Close()

		require.Equal(t, int64(0), store.Version())
	})

	t.Run("reopens existing store", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "state")

		// Create and populate store
		store1, err := NewIAVLStore(path, 100)
		require.NoError(t, err)

		err = store1.Set([]byte("key"), []byte("value"))
		require.NoError(t, err)

		_, version, err := store1.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(1), version)
		require.NoError(t, store1.Close())

		// Reopen store
		store2, err := NewIAVLStore(path, 100)
		require.NoError(t, err)
		defer store2.Close()

		require.Equal(t, int64(1), store2.Version())

		// Verify data
		value, err := store2.Get([]byte("key"))
		require.NoError(t, err)
		require.Equal(t, []byte("value"), value)
	})
}

func TestMemoryIAVLStore(t *testing.T) {
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	require.Equal(t, int64(0), store.Version())
}

func TestSetAndGet(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("sets and gets value", func(t *testing.T) {
		err := store.Set([]byte("key1"), []byte("value1"))
		require.NoError(t, err)

		value, err := store.Get([]byte("key1"))
		require.NoError(t, err)
		require.Equal(t, []byte("value1"), value)
	})

	t.Run("returns nil for non-existent key", func(t *testing.T) {
		value, err := store.Get([]byte("nonexistent"))
		require.NoError(t, err)
		require.Nil(t, value)
	})

	t.Run("overwrites existing value", func(t *testing.T) {
		err := store.Set([]byte("key2"), []byte("original"))
		require.NoError(t, err)

		err = store.Set([]byte("key2"), []byte("updated"))
		require.NoError(t, err)

		value, err := store.Get([]byte("key2"))
		require.NoError(t, err)
		require.Equal(t, []byte("updated"), value)
	})

	t.Run("rejects nil key", func(t *testing.T) {
		err := store.Set(nil, []byte("value"))
		require.Error(t, err)
	})

	t.Run("rejects nil value", func(t *testing.T) {
		err := store.Set([]byte("key"), nil)
		require.Error(t, err)
	})
}

func TestHas(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	err := store.Set([]byte("exists"), []byte("value"))
	require.NoError(t, err)

	has, err := store.Has([]byte("exists"))
	require.NoError(t, err)
	require.True(t, has)

	has, err = store.Has([]byte("missing"))
	require.NoError(t, err)
	require.False(t, has)
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("deletes existing key", func(t *testing.T) {
		err := store.Set([]byte("toDelete"), []byte("value"))
		require.NoError(t, err)

		has, err := store.Has([]byte("toDelete"))
		require.NoError(t, err)
		require.True(t, has)

		err = store.Delete([]byte("toDelete"))
		require.NoError(t, err)

		has, err = store.Has([]byte("toDelete"))
		require.NoError(t, err)
		require.False(t, has)
	})

	t.Run("delete non-existent key is no-op", func(t *testing.T) {
		err := store.Delete([]byte("nonexistent"))
		require.NoError(t, err)
	})

	t.Run("rejects nil key", func(t *testing.T) {
		err := store.Delete(nil)
		require.Error(t, err)
	})
}

func TestCommit(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	t.Run("commits changes", func(t *testing.T) {
		err := store.Set([]byte("key"), []byte("value"))
		require.NoError(t, err)

		hash, version, err := store.Commit()
		require.NoError(t, err)
		require.NotNil(t, hash)
		require.Equal(t, int64(1), version)
		require.Equal(t, int64(1), store.Version())
	})

	t.Run("increments version", func(t *testing.T) {
		for i := int64(2); i <= 5; i++ {
			err := store.Set([]byte("key"), []byte("value"))
			require.NoError(t, err)

			_, version, err := store.Commit()
			require.NoError(t, err)
			require.Equal(t, i, version)
		}
	})

	t.Run("different data produces different hashes", func(t *testing.T) {
		store1 := newTestStore(t)
		defer store1.Close()

		store2 := newTestStore(t)
		defer store2.Close()

		require.NoError(t, store1.Set([]byte("key"), []byte("value1")))
		hash1, _, err := store1.Commit()
		require.NoError(t, err)

		require.NoError(t, store2.Set([]byte("key"), []byte("value2")))
		hash2, _, err := store2.Commit()
		require.NoError(t, err)

		require.NotEqual(t, hash1, hash2)
	})
}

func TestRootHash(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Empty tree has nil hash
	hash1 := store.RootHash()

	// Add data
	require.NoError(t, store.Set([]byte("key"), []byte("value")))

	// Working hash changes after Set
	hash2 := store.RootHash()
	require.NotEqual(t, hash1, hash2)

	// Commit and verify hash stability
	commitHash, _, err := store.Commit()
	require.NoError(t, err)
	hash3 := store.RootHash()
	require.Equal(t, commitHash, hash3)
}

func TestVersioning(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Version 1: key1 = a
	require.NoError(t, store.Set([]byte("key1"), []byte("a")))
	_, _, err := store.Commit()
	require.NoError(t, err)

	// Version 2: key1 = b, key2 = x
	require.NoError(t, store.Set([]byte("key1"), []byte("b")))
	require.NoError(t, store.Set([]byte("key2"), []byte("x")))
	_, _, err = store.Commit()
	require.NoError(t, err)

	// Version 3: key2 = y
	require.NoError(t, store.Set([]byte("key2"), []byte("y")))
	_, _, err = store.Commit()
	require.NoError(t, err)

	t.Run("version exists", func(t *testing.T) {
		require.True(t, store.VersionExists(1))
		require.True(t, store.VersionExists(2))
		require.True(t, store.VersionExists(3))
		require.False(t, store.VersionExists(4))
		require.False(t, store.VersionExists(0))
	})

	t.Run("get versioned value", func(t *testing.T) {
		val, err := store.GetVersioned([]byte("key1"), 1)
		require.NoError(t, err)
		require.Equal(t, []byte("a"), val)

		val, err = store.GetVersioned([]byte("key1"), 2)
		require.NoError(t, err)
		require.Equal(t, []byte("b"), val)

		// key2 didn't exist in version 1
		val, err = store.GetVersioned([]byte("key2"), 1)
		require.NoError(t, err)
		require.Nil(t, val)
	})

	t.Run("load version", func(t *testing.T) {
		// Current is version 3
		require.Equal(t, int64(3), store.Version())

		// Load version 1
		err := store.LoadVersion(1)
		require.NoError(t, err)
		require.Equal(t, int64(1), store.Version())

		// Now reads show version 1 data
		val, err := store.Get([]byte("key1"))
		require.NoError(t, err)
		require.Equal(t, []byte("a"), val)
	})
}

func TestGetProof(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Add and commit data
	require.NoError(t, store.Set([]byte("existing"), []byte("value")))
	_, _, err := store.Commit()
	require.NoError(t, err)

	t.Run("proof for existing key", func(t *testing.T) {
		proof, err := store.GetProof([]byte("existing"))
		require.NoError(t, err)
		require.NotNil(t, proof)
		require.Equal(t, []byte("existing"), proof.Key)
		require.Equal(t, []byte("value"), proof.Value)
		require.True(t, proof.Exists)
		require.NotNil(t, proof.RootHash)
		require.NotEmpty(t, proof.ProofBytes)
	})

	t.Run("proof for non-existing key", func(t *testing.T) {
		proof, err := store.GetProof([]byte("nonexistent"))
		require.NoError(t, err)
		require.NotNil(t, proof)
		require.Equal(t, []byte("nonexistent"), proof.Key)
		require.Nil(t, proof.Value)
		require.False(t, proof.Exists)
	})

	t.Run("nil key returns error", func(t *testing.T) {
		_, err := store.GetProof(nil)
		require.Error(t, err)
	})
}

func TestClose(t *testing.T) {
	store := newTestStore(t)

	err := store.Close()
	require.NoError(t, err)
}

func TestConcurrentAccess(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	const numGoroutines = 10
	const opsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*opsPerGoroutine)

	// Concurrent writes (each goroutine uses unique keys)
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := []byte(fmt.Sprintf("g%d_k%d", goroutineID, i))
				value := []byte(fmt.Sprintf("value_%d_%d", goroutineID, i))

				if err := store.Set(key, value); err != nil {
					errors <- err
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}

	// Commit the changes
	_, _, err := store.Commit()
	require.NoError(t, err)

	// Concurrent reads
	wg = sync.WaitGroup{}
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				key := []byte(fmt.Sprintf("g%d_k%d", goroutineID, i))
				_, err := store.Get(key)
				if err != nil {
					t.Errorf("failed to get key %s: %v", key, err)
				}
			}
		}(g)
	}

	wg.Wait()
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "state")

	// Create store and add data across multiple versions
	store1, err := NewIAVLStore(path, 100)
	require.NoError(t, err)

	for i := 1; i <= 5; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		value := []byte(fmt.Sprintf("value%d", i))
		setErr := store1.Set(key, value)
		require.NoError(t, setErr)

		_, version, commitErr := store1.Commit()
		require.NoError(t, commitErr)
		require.Equal(t, int64(i), version)
	}

	lastHash := store1.RootHash()
	require.NoError(t, store1.Close())

	// Reopen and verify
	store2, err := NewIAVLStore(path, 100)
	require.NoError(t, err)
	defer store2.Close()

	require.Equal(t, int64(5), store2.Version())
	require.Equal(t, lastHash, store2.RootHash())

	// Verify all keys
	for i := 1; i <= 5; i++ {
		key := []byte(fmt.Sprintf("key%d", i))
		expected := []byte(fmt.Sprintf("value%d", i))

		value, err := store2.Get(key)
		require.NoError(t, err)
		require.Equal(t, expected, value)
	}

	// Verify versions exist
	for i := int64(1); i <= 5; i++ {
		require.True(t, store2.VersionExists(i))
	}
}

func TestLargeValues(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Test with 1MB value
	largeValue := make([]byte, 1024*1024)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	err := store.Set([]byte("large"), largeValue)
	require.NoError(t, err)

	_, _, err = store.Commit()
	require.NoError(t, err)

	loaded, err := store.Get([]byte("large"))
	require.NoError(t, err)
	require.Equal(t, largeValue, loaded)
}

func TestManyKeys(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	const numKeys = 1000

	// Insert many keys
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key_%05d", i))
		value := []byte(fmt.Sprintf("value_%d", i))
		err := store.Set(key, value)
		require.NoError(t, err)
	}

	_, _, err := store.Commit()
	require.NoError(t, err)

	// Verify all keys
	for i := 0; i < numKeys; i++ {
		key := []byte(fmt.Sprintf("key_%05d", i))
		expected := []byte(fmt.Sprintf("value_%d", i))

		has, err := store.Has(key)
		require.NoError(t, err)
		require.True(t, has)

		value, err := store.Get(key)
		require.NoError(t, err)
		require.Equal(t, expected, value)
	}
}

func TestProof_VerifyExistence(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Set some values
	require.NoError(t, store.Set([]byte("key1"), []byte("value1")))
	require.NoError(t, store.Set([]byte("key2"), []byte("value2")))
	_, _, err := store.Commit()
	require.NoError(t, err)

	// Get proof for existing key
	proof, err := store.GetProof([]byte("key1"))
	require.NoError(t, err)
	require.NotNil(t, proof)
	require.True(t, proof.Exists)
	require.Equal(t, []byte("key1"), proof.Key)
	require.Equal(t, []byte("value1"), proof.Value)

	// Get root hash
	rootHash := store.RootHash()

	// Verify proof against correct root hash
	valid, err := proof.Verify(rootHash)
	require.NoError(t, err)
	require.True(t, valid, "existence proof should be valid")

	// Verify consistent check
	require.True(t, proof.VerifyConsistent(rootHash))
}

func TestProof_VerifyNonExistence(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Set some values but NOT the key we'll prove against
	require.NoError(t, store.Set([]byte("aaa"), []byte("value_aaa")))
	require.NoError(t, store.Set([]byte("zzz"), []byte("value_zzz")))
	_, _, err := store.Commit()
	require.NoError(t, err)

	// Get non-existence proof for key that doesn't exist
	proof, err := store.GetProof([]byte("mmm"))
	require.NoError(t, err)
	require.NotNil(t, proof)
	require.False(t, proof.Exists)
	require.Nil(t, proof.Value)

	// Verify proof
	rootHash := store.RootHash()
	valid, err := proof.Verify(rootHash)
	require.NoError(t, err)
	require.True(t, valid, "non-existence proof should be valid")
}

func TestProof_RejectsTamperedProof(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	require.NoError(t, store.Set([]byte("key1"), []byte("value1")))
	_, _, err := store.Commit()
	require.NoError(t, err)

	proof, err := store.GetProof([]byte("key1"))
	require.NoError(t, err)
	rootHash := store.RootHash()

	t.Run("tampered value", func(t *testing.T) {
		// Create a copy of the proof with tampered value
		tamperedProof := &Proof{
			Key:        proof.Key,
			Value:      []byte("tampered_value"),
			Exists:     proof.Exists,
			RootHash:   proof.RootHash,
			Version:    proof.Version,
			ProofBytes: proof.ProofBytes,
		}

		valid, err := tamperedProof.Verify(rootHash)
		require.NoError(t, err)
		require.False(t, valid, "tampered value proof should be invalid")
	})

	t.Run("tampered key", func(t *testing.T) {
		// Create a copy of the proof with tampered key
		tamperedProof := &Proof{
			Key:        []byte("tampered_key"),
			Value:      proof.Value,
			Exists:     proof.Exists,
			RootHash:   proof.RootHash,
			Version:    proof.Version,
			ProofBytes: proof.ProofBytes,
		}

		valid, err := tamperedProof.Verify(rootHash)
		require.NoError(t, err)
		require.False(t, valid, "tampered key proof should be invalid")
	})

	t.Run("wrong root hash", func(t *testing.T) {
		// Verify against a different root hash
		wrongHash := make([]byte, len(rootHash))
		copy(wrongHash, rootHash)
		wrongHash[0] ^= 0xFF // Flip some bits

		valid, err := proof.Verify(wrongHash)
		require.NoError(t, err)
		require.False(t, valid, "proof against wrong root should be invalid")
	})
}

func TestProof_NilAndEdgeCases(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	require.NoError(t, store.Set([]byte("key"), []byte("value")))
	_, _, err := store.Commit()
	require.NoError(t, err)

	rootHash := store.RootHash()

	t.Run("nil proof", func(t *testing.T) {
		var nilProof *Proof
		valid, err := nilProof.Verify(rootHash)
		require.Error(t, err)
		require.False(t, valid)
	})

	t.Run("empty root hash", func(t *testing.T) {
		proof, err := store.GetProof([]byte("key"))
		require.NoError(t, err)

		valid, err := proof.Verify(nil)
		require.Error(t, err)
		require.False(t, valid)

		valid, err = proof.Verify([]byte{})
		require.Error(t, err)
		require.False(t, valid)
	})

	t.Run("empty proof bytes", func(t *testing.T) {
		proof := &Proof{
			Key:        []byte("key"),
			Value:      []byte("value"),
			Exists:     true,
			RootHash:   rootHash,
			ProofBytes: nil,
		}

		valid, err := proof.Verify(rootHash)
		require.Error(t, err)
		require.False(t, valid)
	})

	t.Run("invalid proof bytes", func(t *testing.T) {
		proof := &Proof{
			Key:        []byte("key"),
			Value:      []byte("value"),
			Exists:     true,
			RootHash:   rootHash,
			ProofBytes: []byte("not a valid proof"),
		}

		valid, err := proof.Verify(rootHash)
		require.Error(t, err)
		require.False(t, valid)
	})
}

func TestProof_VerifyConsistent(t *testing.T) {
	t.Run("nil proof", func(t *testing.T) {
		var nilProof *Proof
		require.False(t, nilProof.VerifyConsistent([]byte("hash")))
	})

	t.Run("nil stored root hash", func(t *testing.T) {
		proof := &Proof{
			Key:      []byte("key"),
			Value:    []byte("value"),
			RootHash: nil,
		}
		require.False(t, proof.VerifyConsistent([]byte("hash")))
	})

	t.Run("nil input root hash", func(t *testing.T) {
		proof := &Proof{
			Key:      []byte("key"),
			Value:    []byte("value"),
			RootHash: []byte("stored_hash"),
		}
		require.False(t, proof.VerifyConsistent(nil))
	})

	t.Run("matching hashes", func(t *testing.T) {
		hash := []byte("matching_hash")
		proof := &Proof{
			Key:      []byte("key"),
			Value:    []byte("value"),
			RootHash: hash,
		}
		require.True(t, proof.VerifyConsistent(hash))
	})

	t.Run("different hashes", func(t *testing.T) {
		proof := &Proof{
			Key:      []byte("key"),
			Value:    []byte("value"),
			RootHash: []byte("hash1"),
		}
		require.False(t, proof.VerifyConsistent([]byte("hash2")))
	})
}

func TestProof_VerifyWithMultipleVersions(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Version 1
	require.NoError(t, store.Set([]byte("key"), []byte("v1")))
	hash1, _, err := store.Commit()
	require.NoError(t, err)

	proof1, err := store.GetProof([]byte("key"))
	require.NoError(t, err)

	// Version 2 - update the key
	require.NoError(t, store.Set([]byte("key"), []byte("v2")))
	hash2, _, err := store.Commit()
	require.NoError(t, err)

	proof2, err := store.GetProof([]byte("key"))
	require.NoError(t, err)

	// Proof 1 should verify against hash1
	valid1, err := proof1.Verify(hash1)
	require.NoError(t, err)
	require.True(t, valid1)

	// Proof 1 should NOT verify against hash2 (tree has changed)
	valid1Against2, err := proof1.Verify(hash2)
	require.NoError(t, err)
	require.False(t, valid1Against2)

	// Proof 2 should verify against hash2
	valid2, err := proof2.Verify(hash2)
	require.NoError(t, err)
	require.True(t, valid2)

	// Proof 2 should NOT verify against hash1
	valid2Against1, err := proof2.Verify(hash1)
	require.NoError(t, err)
	require.False(t, valid2Against1)
}

// Helper function

func newTestStore(t *testing.T) *IAVLStore {
	t.Helper()
	store, err := NewMemoryIAVLStore(100)
	require.NoError(t, err)
	return store
}
