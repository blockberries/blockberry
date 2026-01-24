package mempool

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

func TestMerkleTree_Add(t *testing.T) {
	tree := NewMerkleTree()

	t.Run("adds new hash", func(t *testing.T) {
		hash := types.HashBytes([]byte("tx1"))
		added := tree.Add(hash)
		require.True(t, added)
		require.Equal(t, 1, tree.Size())
		require.True(t, tree.Has(hash))
	})

	t.Run("rejects duplicate", func(t *testing.T) {
		hash := types.HashBytes([]byte("tx1"))
		added := tree.Add(hash)
		require.False(t, added)
		require.Equal(t, 1, tree.Size())
	})

	t.Run("adds multiple hashes", func(t *testing.T) {
		hash2 := types.HashBytes([]byte("tx2"))
		hash3 := types.HashBytes([]byte("tx3"))

		require.True(t, tree.Add(hash2))
		require.True(t, tree.Add(hash3))
		require.Equal(t, 3, tree.Size())
	})
}

func TestMerkleTree_Remove(t *testing.T) {
	tree := NewMerkleTree()

	hash1 := types.HashBytes([]byte("tx1"))
	hash2 := types.HashBytes([]byte("tx2"))
	hash3 := types.HashBytes([]byte("tx3"))

	tree.Add(hash1)
	tree.Add(hash2)
	tree.Add(hash3)

	t.Run("removes existing hash", func(t *testing.T) {
		removed := tree.Remove(hash2)
		require.True(t, removed)
		require.Equal(t, 2, tree.Size())
		require.False(t, tree.Has(hash2))
		require.True(t, tree.Has(hash1))
		require.True(t, tree.Has(hash3))
	})

	t.Run("returns false for non-existent", func(t *testing.T) {
		removed := tree.Remove(types.HashBytes([]byte("nonexistent")))
		require.False(t, removed)
		require.Equal(t, 2, tree.Size())
	})

	t.Run("removes last element", func(t *testing.T) {
		tree.Remove(hash1)
		tree.Remove(hash3)
		require.Equal(t, 0, tree.Size())
	})
}

func TestMerkleTree_RootHash(t *testing.T) {
	t.Run("empty tree returns nil", func(t *testing.T) {
		tree := NewMerkleTree()
		require.Nil(t, tree.RootHash())
	})

	t.Run("single element returns that element", func(t *testing.T) {
		tree := NewMerkleTree()
		hash := types.HashBytes([]byte("tx1"))
		tree.Add(hash)
		require.Equal(t, []byte(hash), tree.RootHash())
	})

	t.Run("two elements", func(t *testing.T) {
		tree := NewMerkleTree()
		hash1 := types.HashBytes([]byte("tx1"))
		hash2 := types.HashBytes([]byte("tx2"))
		tree.Add(hash1)
		tree.Add(hash2)

		expected := types.HashConcat(hash1, hash2)
		require.Equal(t, []byte(expected), tree.RootHash())
	})

	t.Run("three elements (odd count)", func(t *testing.T) {
		tree := NewMerkleTree()
		hash1 := types.HashBytes([]byte("tx1"))
		hash2 := types.HashBytes([]byte("tx2"))
		hash3 := types.HashBytes([]byte("tx3"))
		tree.Add(hash1)
		tree.Add(hash2)
		tree.Add(hash3)

		// First level: hash(h1,h2), h3
		level1Left := types.HashConcat(hash1, hash2)
		// Second level: hash(level1Left, h3)
		expected := types.HashConcat(level1Left, hash3)
		require.Equal(t, []byte(expected), tree.RootHash())
	})

	t.Run("four elements", func(t *testing.T) {
		tree := NewMerkleTree()
		hash1 := types.HashBytes([]byte("tx1"))
		hash2 := types.HashBytes([]byte("tx2"))
		hash3 := types.HashBytes([]byte("tx3"))
		hash4 := types.HashBytes([]byte("tx4"))
		tree.Add(hash1)
		tree.Add(hash2)
		tree.Add(hash3)
		tree.Add(hash4)

		// First level: hash(h1,h2), hash(h3,h4)
		left := types.HashConcat(hash1, hash2)
		right := types.HashConcat(hash3, hash4)
		// Second level: hash(left, right)
		expected := types.HashConcat(left, right)
		require.Equal(t, []byte(expected), tree.RootHash())
	})

	t.Run("root changes after add", func(t *testing.T) {
		tree := NewMerkleTree()
		hash1 := types.HashBytes([]byte("tx1"))
		tree.Add(hash1)
		root1 := tree.RootHash()

		hash2 := types.HashBytes([]byte("tx2"))
		tree.Add(hash2)
		root2 := tree.RootHash()

		require.NotEqual(t, root1, root2)
	})

	t.Run("root changes after remove", func(t *testing.T) {
		tree := NewMerkleTree()
		hash1 := types.HashBytes([]byte("tx1"))
		hash2 := types.HashBytes([]byte("tx2"))
		tree.Add(hash1)
		tree.Add(hash2)
		root1 := tree.RootHash()

		tree.Remove(hash2)
		root2 := tree.RootHash()

		require.NotEqual(t, root1, root2)
		require.Equal(t, []byte(hash1), root2)
	})
}

func TestMerkleTree_Clear(t *testing.T) {
	tree := NewMerkleTree()

	for i := 0; i < 10; i++ {
		tree.Add(types.HashBytes([]byte{byte(i)}))
	}
	require.Equal(t, 10, tree.Size())

	tree.Clear()
	require.Equal(t, 0, tree.Size())
	require.Nil(t, tree.RootHash())
}

func TestMerkleTree_Leaves(t *testing.T) {
	tree := NewMerkleTree()

	hashes := make([][]byte, 5)
	for i := 0; i < 5; i++ {
		hashes[i] = types.HashBytes([]byte{byte(i)})
		tree.Add(hashes[i])
	}

	leaves := tree.Leaves()
	require.Len(t, leaves, 5)

	// Modifying returned slice shouldn't affect tree
	leaves[0] = []byte("modified")
	treeLeaves := tree.Leaves()
	require.NotEqual(t, leaves[0], treeLeaves[0])
}

func TestMerkleTree_Deterministic(t *testing.T) {
	// Same inputs should produce same root
	tree1 := NewMerkleTree()
	tree2 := NewMerkleTree()

	hashes := make([][]byte, 10)
	for i := 0; i < 10; i++ {
		hashes[i] = types.HashBytes([]byte{byte(i)})
	}

	for _, h := range hashes {
		tree1.Add(h)
		tree2.Add(h)
	}

	require.Equal(t, tree1.RootHash(), tree2.RootHash())
}
