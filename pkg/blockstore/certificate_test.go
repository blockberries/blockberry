package blockstore

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/types"
	loosetypes "github.com/blockberries/looseberry/types"
)

// makeTestCertificate creates a test certificate for the given parameters.
func makeTestCertificate(author uint16, round, epoch uint64) *loosetypes.Certificate {
	// Create a simple header
	header := &loosetypes.Header{
		Author:    author,
		Round:     round,
		Epoch:     epoch,
		BatchRefs: []loosetypes.BatchDigest{},
		Parents:   []loosetypes.CertificateRef{},
		Timestamp: time.Now().UnixNano(),
	}
	header.Digest = header.ComputeDigest()

	// Create minimal votes (just for testing)
	votes := []loosetypes.Vote{
		{Validator: 0, HeaderDigest: header.Digest},
		{Validator: 1, HeaderDigest: header.Digest},
		{Validator: 2, HeaderDigest: header.Digest},
	}

	return loosetypes.NewCertificate(header, votes)
}

// makeTestBatch creates a test batch for the given parameters.
func makeTestBatch(workerID, validatorID uint16, round uint64) *loosetypes.Batch {
	txs := []loosetypes.Transaction{
		loosetypes.Transaction([]byte("tx1")),
		loosetypes.Transaction([]byte("tx2")),
	}
	return loosetypes.NewBatch(workerID, validatorID, round, txs)
}

// TestCertificateStore runs certificate storage tests for all store implementations.
func TestCertificateStore(t *testing.T) {
	stores := map[string]func(t *testing.T) CertificateBlockStore{
		"LevelDB": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "blocks")
			store, err := NewLevelDBBlockStore(path)
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		},
		"BadgerDB": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "blocks")
			store, err := NewBadgerDBBlockStore(path)
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		},
		"Memory": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			return NewMemoryBlockStore()
		},
	}

	for name, createStore := range stores {
		t.Run(name, func(t *testing.T) {
			t.Run("SaveAndGetCertificate", func(t *testing.T) {
				store := createStore(t)
				testSaveAndGetCertificate(t, store)
			})

			t.Run("CertificateNotFound", func(t *testing.T) {
				store := createStore(t)
				testCertificateNotFound(t, store)
			})

			t.Run("DuplicateCertificate", func(t *testing.T) {
				store := createStore(t)
				testDuplicateCertificate(t, store)
			})

			t.Run("GetCertificatesForRound", func(t *testing.T) {
				store := createStore(t)
				testGetCertificatesForRound(t, store)
			})

			t.Run("GetCertificatesForHeight", func(t *testing.T) {
				store := createStore(t)
				testGetCertificatesForHeight(t, store)
			})

			t.Run("SaveAndGetBatch", func(t *testing.T) {
				store := createStore(t)
				testSaveAndGetBatch(t, store)
			})

			t.Run("BatchNotFound", func(t *testing.T) {
				store := createStore(t)
				testBatchNotFound(t, store)
			})

			t.Run("DuplicateBatch", func(t *testing.T) {
				store := createStore(t)
				testDuplicateBatch(t, store)
			})

			t.Run("DefensiveCopying", func(t *testing.T) {
				store := createStore(t)
				testDefensiveCopying(t, store)
			})

			t.Run("ConcurrentAccess", func(t *testing.T) {
				store := createStore(t)
				testConcurrentCertificateAccess(t, store)
			})
		})
	}
}

func testSaveAndGetCertificate(t *testing.T, store CertificateBlockStore) {
	cert := makeTestCertificate(0, 1, 1)

	// Save certificate
	err := store.SaveCertificate(cert)
	require.NoError(t, err)

	// Get certificate by digest
	retrieved, err := store.GetCertificate(cert.Digest())
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify certificate fields
	require.Equal(t, cert.Author(), retrieved.Author())
	require.Equal(t, cert.Round(), retrieved.Round())
	require.Equal(t, cert.Epoch(), retrieved.Epoch())
	require.Equal(t, cert.Digest(), retrieved.Digest())
}

func testCertificateNotFound(t *testing.T, store CertificateBlockStore) {
	// Try to get non-existent certificate
	nonExistentDigest := loosetypes.HashBytes([]byte("nonexistent"))
	_, err := store.GetCertificate(nonExistentDigest)
	require.ErrorIs(t, err, types.ErrCertificateNotFound)
}

func testDuplicateCertificate(t *testing.T, store CertificateBlockStore) {
	cert := makeTestCertificate(0, 1, 1)

	// Save certificate first time
	err := store.SaveCertificate(cert)
	require.NoError(t, err)

	// Try to save again - should fail
	err = store.SaveCertificate(cert)
	require.ErrorIs(t, err, types.ErrCertificateAlreadyExists)
}

func testGetCertificatesForRound(t *testing.T, store CertificateBlockStore) {
	round := uint64(5)

	// Save certificates from multiple validators for the same round
	for author := uint16(0); author < 4; author++ {
		cert := makeTestCertificate(author, round, 1)
		err := store.SaveCertificate(cert)
		require.NoError(t, err)
	}

	// Save certificate for a different round
	otherCert := makeTestCertificate(0, round+1, 1)
	err := store.SaveCertificate(otherCert)
	require.NoError(t, err)

	// Get certificates for the target round
	certs, err := store.GetCertificatesForRound(round)
	require.NoError(t, err)
	require.Len(t, certs, 4)

	// Verify ordering by validator index
	for i, cert := range certs {
		require.Equal(t, uint16(i), cert.Author())
		require.Equal(t, round, cert.Round())
	}

	// Get certificates for empty round
	emptyCerts, err := store.GetCertificatesForRound(999)
	require.NoError(t, err)
	require.Empty(t, emptyCerts)
}

func testGetCertificatesForHeight(t *testing.T, store CertificateBlockStore) {
	height := int64(100)

	// Save certificates and set their block heights
	for author := uint16(0); author < 3; author++ {
		cert := makeTestCertificate(author, uint64(10+author), 1)
		err := store.SaveCertificate(cert)
		require.NoError(t, err)

		err = store.SetCertificateBlockHeight(cert.Digest(), height)
		require.NoError(t, err)
	}

	// Get certificates for the height
	certs, err := store.GetCertificatesForHeight(height)
	require.NoError(t, err)
	require.Len(t, certs, 3)

	// Verify ordering by validator index
	for i, cert := range certs {
		require.Equal(t, uint16(i), cert.Author())
	}

	// Get certificates for empty height
	emptyCerts, err := store.GetCertificatesForHeight(999)
	require.NoError(t, err)
	require.Empty(t, emptyCerts)
}

func testSaveAndGetBatch(t *testing.T, store CertificateBlockStore) {
	batch := makeTestBatch(0, 1, 5)

	// Save batch
	err := store.SaveBatch(batch)
	require.NoError(t, err)

	// Get batch by digest
	retrieved, err := store.GetBatch(batch.Digest)
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Verify batch fields
	require.Equal(t, batch.WorkerID, retrieved.WorkerID)
	require.Equal(t, batch.ValidatorID, retrieved.ValidatorID)
	require.Equal(t, batch.Round, retrieved.Round)
	require.Equal(t, batch.Digest, retrieved.Digest)
	require.Equal(t, len(batch.Transactions), len(retrieved.Transactions))
}

func testBatchNotFound(t *testing.T, store CertificateBlockStore) {
	// Try to get non-existent batch
	nonExistentDigest := loosetypes.HashBytes([]byte("nonexistent"))
	_, err := store.GetBatch(nonExistentDigest)
	require.ErrorIs(t, err, types.ErrBatchNotFound)
}

func testDuplicateBatch(t *testing.T, store CertificateBlockStore) {
	batch := makeTestBatch(0, 1, 5)

	// Save batch first time
	err := store.SaveBatch(batch)
	require.NoError(t, err)

	// Try to save again - should fail
	err = store.SaveBatch(batch)
	require.ErrorIs(t, err, types.ErrBatchAlreadyExists)
}

func testDefensiveCopying(t *testing.T, store CertificateBlockStore) {
	cert := makeTestCertificate(0, 1, 1)
	originalDigest := cert.Digest()

	// Save certificate
	err := store.SaveCertificate(cert)
	require.NoError(t, err)

	// Retrieve certificate
	retrieved1, err := store.GetCertificate(originalDigest)
	require.NoError(t, err)

	// Retrieve again
	retrieved2, err := store.GetCertificate(originalDigest)
	require.NoError(t, err)

	// Verify they are different objects (defensive copy)
	require.NotSame(t, retrieved1, retrieved2)
}

func testConcurrentCertificateAccess(t *testing.T, store CertificateBlockStore) {
	const numGoroutines = 10
	const certsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*certsPerGoroutine)

	// Concurrent writes
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < certsPerGoroutine; i++ {
				author := uint16(goroutineID)
				round := uint64(goroutineID*certsPerGoroutine + i + 1)
				cert := makeTestCertificate(author, round, 1)

				if err := store.SaveCertificate(cert); err != nil {
					errors <- err
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
}

func TestSetCertificateBlockHeight_NotFound(t *testing.T) {
	stores := map[string]func(t *testing.T) CertificateBlockStore{
		"LevelDB": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "blocks")
			store, err := NewLevelDBBlockStore(path)
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		},
		"BadgerDB": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "blocks")
			store, err := NewBadgerDBBlockStore(path)
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		},
		"Memory": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			return NewMemoryBlockStore()
		},
	}

	for name, createStore := range stores {
		t.Run(name, func(t *testing.T) {
			store := createStore(t)
			nonExistentDigest := loosetypes.HashBytes([]byte("nonexistent"))
			err := store.SetCertificateBlockHeight(nonExistentDigest, 100)
			require.ErrorIs(t, err, types.ErrCertificateNotFound)
		})
	}
}

func TestNilCertificateAndBatch(t *testing.T) {
	stores := map[string]func(t *testing.T) CertificateBlockStore{
		"LevelDB": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "blocks")
			store, err := NewLevelDBBlockStore(path)
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		},
		"BadgerDB": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			tmpDir := t.TempDir()
			path := filepath.Join(tmpDir, "blocks")
			store, err := NewBadgerDBBlockStore(path)
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			return store
		},
		"Memory": func(t *testing.T) CertificateBlockStore {
			t.Helper()
			return NewMemoryBlockStore()
		},
	}

	for name, createStore := range stores {
		t.Run(name, func(t *testing.T) {
			store := createStore(t)

			// Test nil certificate
			err := store.SaveCertificate(nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "nil certificate")

			// Test nil batch
			err = store.SaveBatch(nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "nil batch")
		})
	}
}

// Benchmarks

func BenchmarkSaveCertificate(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cert := makeTestCertificate(uint16(i%256), uint64(i+1), 1)
		if err := store.SaveCertificate(cert); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetCertificate(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	// Pre-populate with certificates
	certs := make([]*loosetypes.Certificate, 1000)
	for i := 0; i < 1000; i++ {
		cert := makeTestCertificate(uint16(i%256), uint64(i+1), 1)
		if err := store.SaveCertificate(cert); err != nil {
			b.Fatal(err)
		}
		certs[i] = cert
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cert := certs[i%1000]
		if _, err := store.GetCertificate(cert.Digest()); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSaveBatch(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := makeTestBatch(uint16(i%256), uint16(i%256), uint64(i+1))
		if err := store.SaveBatch(batch); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetBatch(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	// Pre-populate with batches
	batches := make([]*loosetypes.Batch, 1000)
	for i := 0; i < 1000; i++ {
		batch := makeTestBatch(uint16(i%256), uint16(i%256), uint64(i+1))
		if err := store.SaveBatch(batch); err != nil {
			b.Fatal(err)
		}
		batches[i] = batch
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := batches[i%1000]
		if _, err := store.GetBatch(batch.Digest); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetCertificatesForRound(b *testing.B) {
	tmpDir := b.TempDir()
	path := filepath.Join(tmpDir, "blocks")

	store, err := NewLevelDBBlockStore(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	// Pre-populate with certificates across multiple rounds
	for round := uint64(1); round <= 100; round++ {
		for author := uint16(0); author < 10; author++ {
			cert := makeTestCertificate(author, round, 1)
			if err := store.SaveCertificate(cert); err != nil {
				b.Fatal(err)
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		round := uint64((i % 100) + 1)
		if _, err := store.GetCertificatesForRound(round); err != nil {
			b.Fatal(err)
		}
	}
}
