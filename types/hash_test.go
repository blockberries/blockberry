package types

import (
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashSize(t *testing.T) {
	require.Equal(t, 32, HashSize)
	require.Equal(t, sha256.Size, HashSize)
}

func TestHashTx(t *testing.T) {
	t.Run("basic hash", func(t *testing.T) {
		tx := Tx([]byte("test transaction"))
		h := HashTx(tx)
		require.Len(t, h, HashSize)
	})

	t.Run("deterministic", func(t *testing.T) {
		tx := Tx([]byte("test transaction"))
		h1 := HashTx(tx)
		h2 := HashTx(tx)
		require.True(t, h1.Equal(h2))
	})

	t.Run("different txs have different hashes", func(t *testing.T) {
		tx1 := Tx([]byte("tx1"))
		tx2 := Tx([]byte("tx2"))
		h1 := HashTx(tx1)
		h2 := HashTx(tx2)
		require.False(t, h1.Equal(h2))
	})

	t.Run("nil tx", func(t *testing.T) {
		h := HashTx(nil)
		require.Nil(t, h)
	})

	t.Run("empty tx", func(t *testing.T) {
		h := HashTx(Tx([]byte{}))
		require.Len(t, h, HashSize)
		// Empty hash should equal EmptyHash
		require.True(t, h.Equal(EmptyHash()))
	})

	t.Run("matches sha256 directly", func(t *testing.T) {
		data := []byte("test")
		tx := Tx(data)
		h := HashTx(tx)

		expected := sha256.Sum256(data)
		require.Equal(t, expected[:], h.Bytes())
	})
}

func TestHashBlock(t *testing.T) {
	t.Run("basic hash", func(t *testing.T) {
		block := Block([]byte("test block data"))
		h := HashBlock(block)
		require.Len(t, h, HashSize)
	})

	t.Run("deterministic", func(t *testing.T) {
		block := Block([]byte("test block"))
		h1 := HashBlock(block)
		h2 := HashBlock(block)
		require.True(t, h1.Equal(h2))
	})

	t.Run("different blocks have different hashes", func(t *testing.T) {
		b1 := Block([]byte("block1"))
		b2 := Block([]byte("block2"))
		h1 := HashBlock(b1)
		h2 := HashBlock(b2)
		require.False(t, h1.Equal(h2))
	})

	t.Run("nil block", func(t *testing.T) {
		h := HashBlock(nil)
		require.Nil(t, h)
	})

	t.Run("matches sha256 directly", func(t *testing.T) {
		data := []byte("block")
		block := Block(data)
		h := HashBlock(block)

		expected := sha256.Sum256(data)
		require.Equal(t, expected[:], h.Bytes())
	})
}

func TestHashBytes(t *testing.T) {
	t.Run("basic hash", func(t *testing.T) {
		h := HashBytes([]byte("test data"))
		require.Len(t, h, HashSize)
	})

	t.Run("nil data", func(t *testing.T) {
		h := HashBytes(nil)
		require.Nil(t, h)
	})

	t.Run("empty data", func(t *testing.T) {
		h := HashBytes([]byte{})
		require.Len(t, h, HashSize)
	})

	t.Run("matches sha256", func(t *testing.T) {
		data := []byte("hello world")
		h := HashBytes(data)

		expected := sha256.Sum256(data)
		require.Equal(t, expected[:], h.Bytes())
	})
}

func TestHashConcat(t *testing.T) {
	t.Run("basic concat", func(t *testing.T) {
		h1 := HashBytes([]byte("left"))
		h2 := HashBytes([]byte("right"))
		result := HashConcat(h1, h2)
		require.Len(t, result, HashSize)
	})

	t.Run("order matters", func(t *testing.T) {
		h1 := HashBytes([]byte("a"))
		h2 := HashBytes([]byte("b"))
		result1 := HashConcat(h1, h2)
		result2 := HashConcat(h2, h1)
		require.False(t, result1.Equal(result2))
	})

	t.Run("deterministic", func(t *testing.T) {
		h1 := HashBytes([]byte("left"))
		h2 := HashBytes([]byte("right"))
		result1 := HashConcat(h1, h2)
		result2 := HashConcat(h1, h2)
		require.True(t, result1.Equal(result2))
	})

	t.Run("empty hashes", func(t *testing.T) {
		result := HashConcat(Hash{}, Hash{})
		require.Len(t, result, HashSize)
	})

	t.Run("matches manual sha256 of concatenation", func(t *testing.T) {
		h1 := HashBytes([]byte("left"))
		h2 := HashBytes([]byte("right"))
		result := HashConcat(h1, h2)

		// Manual concatenation
		concat := append(h1.Bytes(), h2.Bytes()...)
		expected := sha256.Sum256(concat)
		require.Equal(t, expected[:], result.Bytes())
	})
}

func TestEmptyHash(t *testing.T) {
	t.Run("returns correct hash", func(t *testing.T) {
		h := EmptyHash()
		require.Len(t, h, HashSize)
	})

	t.Run("deterministic", func(t *testing.T) {
		h1 := EmptyHash()
		h2 := EmptyHash()
		require.True(t, h1.Equal(h2))
	})

	t.Run("matches sha256 of empty", func(t *testing.T) {
		h := EmptyHash()
		expected := sha256.Sum256([]byte{})
		require.Equal(t, expected[:], h.Bytes())
	})

	t.Run("known value", func(t *testing.T) {
		// SHA-256 of empty string is a well-known value
		h := EmptyHash()
		// e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
		require.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h.String())
	})
}

func BenchmarkHashTx(b *testing.B) {
	tx := Tx(make([]byte, 256))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashTx(tx)
	}
}

func BenchmarkHashBlock(b *testing.B) {
	block := Block(make([]byte, 1024))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashBlock(block)
	}
}

func BenchmarkHashConcat(b *testing.B) {
	h1 := HashBytes([]byte("left"))
	h2 := HashBytes([]byte("right"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HashConcat(h1, h2)
	}
}
