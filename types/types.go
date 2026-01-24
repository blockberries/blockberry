// Package types provides common type definitions for blockberry.
package types

import (
	"encoding/hex"
	"fmt"
)

// Height represents a block height in the blockchain.
type Height int64

// Hash represents a cryptographic hash (typically 32 bytes for SHA-256).
type Hash []byte

// Tx represents an opaque transaction as raw bytes.
// The structure of transactions is defined by the application.
type Tx []byte

// Block represents an opaque block as raw bytes.
// The structure of blocks is defined by the application.
type Block []byte

// String returns the height as a string.
func (h Height) String() string {
	return fmt.Sprintf("%d", h)
}

// Int64 returns the height as an int64.
func (h Height) Int64() int64 {
	return int64(h)
}

// String returns the hash as a hexadecimal string.
func (h Hash) String() string {
	return hex.EncodeToString(h)
}

// Bytes returns the raw bytes of the hash.
func (h Hash) Bytes() []byte {
	return []byte(h)
}

// IsEmpty returns true if the hash is nil or zero-length.
func (h Hash) IsEmpty() bool {
	return len(h) == 0
}

// Equal returns true if the hashes are equal.
func (h Hash) Equal(other Hash) bool {
	if len(h) != len(other) {
		return false
	}
	for i := range h {
		if h[i] != other[i] {
			return false
		}
	}
	return true
}

// HashFromHex parses a hexadecimal string into a Hash.
func HashFromHex(s string) (Hash, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}
	return Hash(b), nil
}

// String returns the transaction as a hexadecimal string.
func (tx Tx) String() string {
	if len(tx) > 32 {
		return hex.EncodeToString(tx[:32]) + "..."
	}
	return hex.EncodeToString(tx)
}

// Bytes returns the raw bytes of the transaction.
func (tx Tx) Bytes() []byte {
	return []byte(tx)
}

// Size returns the size of the transaction in bytes.
func (tx Tx) Size() int {
	return len(tx)
}

// String returns the block as a truncated hexadecimal string.
func (b Block) String() string {
	if len(b) > 32 {
		return hex.EncodeToString(b[:32]) + "..."
	}
	return hex.EncodeToString(b)
}

// Bytes returns the raw bytes of the block.
func (b Block) Bytes() []byte {
	return []byte(b)
}

// Size returns the size of the block in bytes.
func (b Block) Size() int {
	return len(b)
}

// PeerID is a type alias for peer identification.
// This is typically the libp2p peer ID string.
type PeerID string

// String returns the peer ID as a string.
func (p PeerID) String() string {
	return string(p)
}

// IsEmpty returns true if the peer ID is empty.
func (p PeerID) IsEmpty() bool {
	return p == ""
}
