package types

import (
	"crypto/sha256"
)

const (
	// HashSize is the size of a SHA-256 hash in bytes.
	HashSize = sha256.Size // 32 bytes
)

// HashTx computes the SHA-256 hash of a transaction.
func HashTx(tx Tx) Hash {
	if tx == nil {
		return nil
	}
	h := sha256.Sum256(tx)
	return h[:]
}

// HashBlock computes the SHA-256 hash of a block.
func HashBlock(block Block) Hash {
	if block == nil {
		return nil
	}
	h := sha256.Sum256(block)
	return h[:]
}

// HashBytes computes the SHA-256 hash of arbitrary bytes.
func HashBytes(data []byte) Hash {
	if data == nil {
		return nil
	}
	h := sha256.Sum256(data)
	return h[:]
}

// HashConcat computes the SHA-256 hash of the concatenation of two hashes.
// This is useful for building merkle trees.
func HashConcat(left, right Hash) Hash {
	h := sha256.New()
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// EmptyHash returns the hash of an empty byte slice.
func EmptyHash() Hash {
	h := sha256.Sum256([]byte{})
	return h[:]
}
