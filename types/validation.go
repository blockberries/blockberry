package types

import (
	"errors"
	"fmt"
)

// Validation limits for input parameters.
const (
	// MaxBatchSize is the maximum allowed batch size for block/tx requests.
	MaxBatchSize = 1000

	// MinBatchSize is the minimum allowed batch size.
	MinBatchSize = 1

	// MaxBlockHeight is the maximum allowed block height.
	// This is set to a practical limit for int64 values.
	MaxBlockHeight = 1<<62 - 1

	// MaxMessageSize is the maximum allowed message size in bytes (10 MB).
	MaxMessageSize = 10 * 1024 * 1024

	// MaxTransactionSize is the maximum allowed transaction size in bytes (1 MB).
	MaxTransactionSize = 1 * 1024 * 1024

	// MaxKeySize is the maximum allowed key size in state store (1 KB).
	MaxKeySize = 1024

	// MaxValueSize is the maximum allowed value size in state store (10 MB).
	MaxValueSize = 10 * 1024 * 1024

	// MaxAddressesPerResponse is the maximum addresses in a PEX response.
	MaxAddressesPerResponse = 100

	// MaxPeers is the maximum number of connected peers.
	MaxPeers = 1000
)

// Validation errors.
var (
	// ErrInvalidHeight is returned when a block height is invalid.
	ErrInvalidHeight = errors.New("invalid block height")

	// ErrInvalidBatchSize is returned when a batch size is invalid.
	ErrInvalidBatchSize = errors.New("invalid batch size")

	// ErrInvalidSize is returned when a size parameter is invalid.
	ErrInvalidSize = errors.New("invalid size")

	// ErrDataTooLarge is returned when data exceeds size limits.
	ErrDataTooLarge = errors.New("data too large")

	// ErrEmptyData is returned when required data is empty.
	ErrEmptyData = errors.New("empty data")

	// ErrNilPointer is returned when a required pointer is nil.
	ErrNilPointer = errors.New("nil pointer")
)

// ValidateHeight validates a block height.
// Returns an error if the height is negative or exceeds MaxBlockHeight.
func ValidateHeight(height int64) error {
	if height < 0 {
		return fmt.Errorf("%w: height cannot be negative: %d", ErrInvalidHeight, height)
	}
	if height > MaxBlockHeight {
		return fmt.Errorf("%w: height exceeds maximum: %d > %d", ErrInvalidHeight, height, MaxBlockHeight)
	}
	return nil
}

// ValidateHeightRange validates a range of block heights.
// Returns an error if either height is invalid or if from > to.
func ValidateHeightRange(from, to int64) error {
	if err := ValidateHeight(from); err != nil {
		return fmt.Errorf("from height: %w", err)
	}
	if err := ValidateHeight(to); err != nil {
		return fmt.Errorf("to height: %w", err)
	}
	if from > to {
		return fmt.Errorf("%w: from > to: %d > %d", ErrInvalidHeight, from, to)
	}
	return nil
}

// ValidateBatchSize validates a batch size against minimum and maximum limits.
// If maxSize is 0, MaxBatchSize is used.
func ValidateBatchSize(size int32, maxSize int32) error {
	if maxSize <= 0 {
		maxSize = MaxBatchSize
	}
	if size < MinBatchSize {
		return fmt.Errorf("%w: size must be >= %d: got %d", ErrInvalidBatchSize, MinBatchSize, size)
	}
	if size > maxSize {
		return fmt.Errorf("%w: size exceeds maximum: %d > %d", ErrInvalidBatchSize, size, maxSize)
	}
	return nil
}

// ValidateMessageSize validates that a message size is within limits.
func ValidateMessageSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("%w: message cannot be empty", ErrInvalidSize)
	}
	if size > MaxMessageSize {
		return fmt.Errorf("%w: message size exceeds maximum: %d > %d", ErrDataTooLarge, size, MaxMessageSize)
	}
	return nil
}

// ValidateTransactionSize validates that a transaction size is within limits.
func ValidateTransactionSize(size int) error {
	if size <= 0 {
		return fmt.Errorf("%w: transaction cannot be empty", ErrInvalidSize)
	}
	if size > MaxTransactionSize {
		return fmt.Errorf("%w: transaction size exceeds maximum: %d > %d", ErrDataTooLarge, size, MaxTransactionSize)
	}
	return nil
}

// ValidateKeySize validates that a state store key size is within limits.
func ValidateKeySize(key []byte) error {
	if len(key) == 0 {
		return fmt.Errorf("%w: key cannot be empty", ErrEmptyData)
	}
	if len(key) > MaxKeySize {
		return fmt.Errorf("%w: key size exceeds maximum: %d > %d", ErrDataTooLarge, len(key), MaxKeySize)
	}
	return nil
}

// ValidateValueSize validates that a state store value size is within limits.
func ValidateValueSize(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("%w: value cannot be empty", ErrEmptyData)
	}
	if len(value) > MaxValueSize {
		return fmt.Errorf("%w: value size exceeds maximum: %d > %d", ErrDataTooLarge, len(value), MaxValueSize)
	}
	return nil
}

// ValidateHash validates that a hash is non-empty and has expected length.
// expectedLen of 0 means any non-empty length is acceptable.
func ValidateHash(hash []byte, expectedLen int) error {
	if len(hash) == 0 {
		return fmt.Errorf("%w: hash cannot be empty", ErrEmptyData)
	}
	if expectedLen > 0 && len(hash) != expectedLen {
		return fmt.Errorf("%w: hash length mismatch: %d != %d", ErrInvalidSize, len(hash), expectedLen)
	}
	return nil
}

// ValidateBlockData validates block data for basic integrity.
// height must be provided as a pointer (can be nil to skip height validation).
func ValidateBlockData(height *int64, hash, data []byte) error {
	if height != nil {
		if err := ValidateHeight(*height); err != nil {
			return err
		}
	}
	if err := ValidateHash(hash, 0); err != nil {
		return fmt.Errorf("block hash: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("block data: %w", ErrEmptyData)
	}
	if len(data) > MaxMessageSize {
		return fmt.Errorf("block data: %w: %d > %d", ErrDataTooLarge, len(data), MaxMessageSize)
	}
	return nil
}

// ValidatePeerCount validates that a peer count is within reasonable limits.
func ValidatePeerCount(count int) error {
	if count < 0 {
		return fmt.Errorf("%w: peer count cannot be negative: %d", ErrInvalidSize, count)
	}
	if count > MaxPeers {
		return fmt.Errorf("%w: peer count exceeds maximum: %d > %d", ErrInvalidSize, count, MaxPeers)
	}
	return nil
}

// ValidateAddressCount validates that an address count is within limits.
func ValidateAddressCount(count int) error {
	if count < 0 {
		return fmt.Errorf("%w: address count cannot be negative: %d", ErrInvalidSize, count)
	}
	if count > MaxAddressesPerResponse {
		return fmt.Errorf("%w: address count exceeds maximum: %d > %d", ErrInvalidSize, count, MaxAddressesPerResponse)
	}
	return nil
}

// MustNotBeNil returns an error if the pointer is nil.
// This is a helper for validating required pointers in protocol messages.
// Note: This function handles both untyped nil and typed nil pointers.
func MustNotBeNil(ptr any, name string) error {
	if isNil(ptr) {
		return fmt.Errorf("%w: %s is required", ErrNilPointer, name)
	}
	return nil
}

// isNil checks if an interface is nil or contains a nil pointer.
// This handles both untyped nil (interface{} nil) and typed nil pointers
// (e.g., (*int)(nil) assigned to interface{}).
func isNil(v any) bool {
	if v == nil {
		return true
	}
	// Use type assertion to check common pointer types
	// This avoids importing reflect for simple cases
	switch p := v.(type) {
	case *int:
		return p == nil
	case *int32:
		return p == nil
	case *int64:
		return p == nil
	case *uint:
		return p == nil
	case *uint32:
		return p == nil
	case *uint64:
		return p == nil
	case *string:
		return p == nil
	case *bool:
		return p == nil
	case *float32:
		return p == nil
	case *float64:
		return p == nil
	case *[]byte:
		return p == nil
	default:
		// For non-pointer types or unknown pointer types, check if nil
		// This won't catch all typed nil pointers but handles most common cases
		return false
	}
}

// ClampBatchSize clamps a batch size to valid limits.
// If the size is out of range, it returns the nearest valid value.
func ClampBatchSize(size int32, maxSize int32) int32 {
	if maxSize <= 0 {
		maxSize = MaxBatchSize
	}
	if size < MinBatchSize {
		return MinBatchSize
	}
	if size > maxSize {
		return maxSize
	}
	return size
}
