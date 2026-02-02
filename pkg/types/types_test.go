package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeight(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		h := Height(12345)
		require.Equal(t, "12345", h.String())
	})

	t.Run("Int64", func(t *testing.T) {
		h := Height(12345)
		require.Equal(t, int64(12345), h.Int64())
	})

	t.Run("zero value", func(t *testing.T) {
		var h Height
		require.Equal(t, "0", h.String())
		require.Equal(t, int64(0), h.Int64())
	})

	t.Run("negative value", func(t *testing.T) {
		h := Height(-1)
		require.Equal(t, "-1", h.String())
		require.Equal(t, int64(-1), h.Int64())
	})
}

func TestHash(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		h := Hash([]byte{0xde, 0xad, 0xbe, 0xef})
		require.Equal(t, "deadbeef", h.String())
	})

	t.Run("Bytes", func(t *testing.T) {
		original := []byte{0x01, 0x02, 0x03}
		h := Hash(original)
		require.Equal(t, original, h.Bytes())
	})

	t.Run("IsEmpty", func(t *testing.T) {
		require.True(t, Hash(nil).IsEmpty())
		require.True(t, Hash([]byte{}).IsEmpty())
		require.False(t, Hash([]byte{0x00}).IsEmpty())
	})

	t.Run("Equal", func(t *testing.T) {
		h1 := Hash([]byte{0x01, 0x02, 0x03})
		h2 := Hash([]byte{0x01, 0x02, 0x03})
		h3 := Hash([]byte{0x01, 0x02, 0x04})
		h4 := Hash([]byte{0x01, 0x02})

		require.True(t, h1.Equal(h2))
		require.False(t, h1.Equal(h3))
		require.False(t, h1.Equal(h4))
		require.True(t, Hash(nil).Equal(Hash(nil)))
		require.False(t, Hash(nil).Equal(h1))
	})

	t.Run("nil hash string", func(t *testing.T) {
		var h Hash
		require.Equal(t, "", h.String())
	})
}

func TestHashFromHex(t *testing.T) {
	t.Run("valid hex", func(t *testing.T) {
		h, err := HashFromHex("deadbeef")
		require.NoError(t, err)
		require.Equal(t, Hash([]byte{0xde, 0xad, 0xbe, 0xef}), h)
	})

	t.Run("uppercase hex", func(t *testing.T) {
		h, err := HashFromHex("DEADBEEF")
		require.NoError(t, err)
		require.Equal(t, Hash([]byte{0xde, 0xad, 0xbe, 0xef}), h)
	})

	t.Run("mixed case hex", func(t *testing.T) {
		h, err := HashFromHex("DeAdBeEf")
		require.NoError(t, err)
		require.Equal(t, Hash([]byte{0xde, 0xad, 0xbe, 0xef}), h)
	})

	t.Run("empty string", func(t *testing.T) {
		h, err := HashFromHex("")
		require.NoError(t, err)
		require.Equal(t, Hash([]byte{}), h)
	})

	t.Run("invalid hex", func(t *testing.T) {
		_, err := HashFromHex("not hex")
		require.Error(t, err)
	})

	t.Run("odd length hex", func(t *testing.T) {
		_, err := HashFromHex("abc")
		require.Error(t, err)
	})
}

func TestTx(t *testing.T) {
	t.Run("String short tx", func(t *testing.T) {
		tx := Tx([]byte{0x01, 0x02, 0x03})
		require.Equal(t, "010203", tx.String())
	})

	t.Run("String long tx truncated", func(t *testing.T) {
		// Create a tx longer than 32 bytes
		data := make([]byte, 64)
		for i := range data {
			data[i] = byte(i)
		}
		tx := Tx(data)
		str := tx.String()
		require.Contains(t, str, "...")
		require.Len(t, str, 64+3) // 32 bytes * 2 hex chars + "..."
	})

	t.Run("Bytes", func(t *testing.T) {
		original := []byte{0x01, 0x02, 0x03}
		tx := Tx(original)
		require.Equal(t, original, tx.Bytes())
	})

	t.Run("Size", func(t *testing.T) {
		tx := Tx([]byte{0x01, 0x02, 0x03})
		require.Equal(t, 3, tx.Size())
	})

	t.Run("nil tx", func(t *testing.T) {
		var tx Tx
		require.Equal(t, "", tx.String())
		require.Equal(t, 0, tx.Size())
	})
}

func TestBlock(t *testing.T) {
	t.Run("String short block", func(t *testing.T) {
		b := Block([]byte{0xab, 0xcd})
		require.Equal(t, "abcd", b.String())
	})

	t.Run("String long block truncated", func(t *testing.T) {
		data := make([]byte, 64)
		for i := range data {
			data[i] = byte(i)
		}
		b := Block(data)
		str := b.String()
		require.Contains(t, str, "...")
	})

	t.Run("Bytes", func(t *testing.T) {
		original := []byte{0x01, 0x02, 0x03}
		b := Block(original)
		require.Equal(t, original, b.Bytes())
	})

	t.Run("Size", func(t *testing.T) {
		b := Block([]byte{0x01, 0x02, 0x03, 0x04, 0x05})
		require.Equal(t, 5, b.Size())
	})
}

func TestPeerID(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		p := PeerID("12D3KooWTest")
		require.Equal(t, "12D3KooWTest", p.String())
	})

	t.Run("IsEmpty", func(t *testing.T) {
		require.True(t, PeerID("").IsEmpty())
		require.False(t, PeerID("12D3KooWTest").IsEmpty())
	})
}
