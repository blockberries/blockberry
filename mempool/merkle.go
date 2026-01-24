package mempool

import (
	"github.com/blockberries/blockberry/types"
)

// MerkleTree is an in-memory merkle tree for computing root hashes.
// It maintains a dynamic set of leaf hashes and efficiently updates
// the root hash on insertions and deletions.
type MerkleTree struct {
	// leaves stores the leaf hashes in the order they were added
	leaves [][]byte
	// leafIndex maps hash (as string) to index in leaves slice
	leafIndex map[string]int
}

// NewMerkleTree creates a new empty merkle tree.
func NewMerkleTree() *MerkleTree {
	return &MerkleTree{
		leaves:    make([][]byte, 0),
		leafIndex: make(map[string]int),
	}
}

// Add inserts a hash into the tree.
// Returns true if the hash was added, false if it already exists.
func (t *MerkleTree) Add(hash []byte) bool {
	key := string(hash)
	if _, exists := t.leafIndex[key]; exists {
		return false
	}
	t.leafIndex[key] = len(t.leaves)
	t.leaves = append(t.leaves, hash)
	return true
}

// Remove deletes a hash from the tree.
// Returns true if the hash was removed, false if it didn't exist.
func (t *MerkleTree) Remove(hash []byte) bool {
	key := string(hash)
	idx, exists := t.leafIndex[key]
	if !exists {
		return false
	}

	// Swap with last element and shrink
	lastIdx := len(t.leaves) - 1
	if idx != lastIdx {
		// Move last element to the removed position
		t.leaves[idx] = t.leaves[lastIdx]
		t.leafIndex[string(t.leaves[idx])] = idx
	}

	// Remove last element
	t.leaves = t.leaves[:lastIdx]
	delete(t.leafIndex, key)
	return true
}

// Has checks if a hash exists in the tree.
func (t *MerkleTree) Has(hash []byte) bool {
	_, exists := t.leafIndex[string(hash)]
	return exists
}

// Size returns the number of leaves in the tree.
func (t *MerkleTree) Size() int {
	return len(t.leaves)
}

// RootHash computes and returns the merkle root hash.
// Returns nil if the tree is empty.
// Uses a bottom-up construction of the tree.
func (t *MerkleTree) RootHash() []byte {
	n := len(t.leaves)
	if n == 0 {
		return nil
	}
	if n == 1 {
		return t.leaves[0]
	}

	// Copy leaves to avoid modifying the original
	level := make([][]byte, n)
	copy(level, t.leaves)

	// Build tree bottom-up
	for len(level) > 1 {
		nextLevel := make([][]byte, 0, (len(level)+1)/2)

		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				// Hash pair of nodes
				combined := types.HashConcat(level[i], level[i+1])
				nextLevel = append(nextLevel, combined)
			} else {
				// Odd node, promote to next level
				nextLevel = append(nextLevel, level[i])
			}
		}
		level = nextLevel
	}

	return level[0]
}

// Clear removes all hashes from the tree.
func (t *MerkleTree) Clear() {
	t.leaves = t.leaves[:0]
	t.leafIndex = make(map[string]int)
}

// Leaves returns a copy of all leaf hashes.
func (t *MerkleTree) Leaves() [][]byte {
	result := make([][]byte, len(t.leaves))
	copy(result, t.leaves)
	return result
}
