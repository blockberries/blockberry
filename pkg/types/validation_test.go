package types

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateHeight(t *testing.T) {
	tests := []struct {
		name    string
		height  int64
		wantErr error
	}{
		{name: "valid zero", height: 0, wantErr: nil},
		{name: "valid positive", height: 100, wantErr: nil},
		{name: "valid large", height: 1000000000, wantErr: nil},
		{name: "valid max", height: MaxBlockHeight, wantErr: nil},
		{name: "invalid negative", height: -1, wantErr: ErrInvalidHeight},
		{name: "invalid exceeds max", height: MaxBlockHeight + 1, wantErr: ErrInvalidHeight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeight(tt.height)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateHeightRange(t *testing.T) {
	tests := []struct {
		name    string
		from    int64
		to      int64
		wantErr bool
	}{
		{name: "valid same", from: 10, to: 10, wantErr: false},
		{name: "valid range", from: 10, to: 100, wantErr: false},
		{name: "valid zero to positive", from: 0, to: 100, wantErr: false},
		{name: "invalid from > to", from: 100, to: 10, wantErr: true},
		{name: "invalid negative from", from: -1, to: 10, wantErr: true},
		{name: "invalid negative to", from: 10, to: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeightRange(tt.from, tt.to)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBatchSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int32
		maxSize int32
		wantErr error
	}{
		{name: "valid min", size: 1, maxSize: 0, wantErr: nil},
		{name: "valid middle", size: 100, maxSize: 0, wantErr: nil},
		{name: "valid at max", size: MaxBatchSize, maxSize: 0, wantErr: nil},
		{name: "valid custom max", size: 50, maxSize: 100, wantErr: nil},
		{name: "invalid zero", size: 0, maxSize: 0, wantErr: ErrInvalidBatchSize},
		{name: "invalid negative", size: -1, maxSize: 0, wantErr: ErrInvalidBatchSize},
		{name: "invalid exceeds max", size: MaxBatchSize + 1, maxSize: 0, wantErr: ErrInvalidBatchSize},
		{name: "invalid exceeds custom max", size: 101, maxSize: 100, wantErr: ErrInvalidBatchSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBatchSize(tt.size, tt.maxSize)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMessageSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "valid small", size: 100, wantErr: false},
		{name: "valid max", size: MaxMessageSize, wantErr: false},
		{name: "invalid zero", size: 0, wantErr: true},
		{name: "invalid negative", size: -1, wantErr: true},
		{name: "invalid exceeds max", size: MaxMessageSize + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageSize(tt.size)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTransactionSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "valid small", size: 100, wantErr: false},
		{name: "valid max", size: MaxTransactionSize, wantErr: false},
		{name: "invalid zero", size: 0, wantErr: true},
		{name: "invalid exceeds max", size: MaxTransactionSize + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransactionSize(tt.size)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateKeySize(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{name: "valid small", key: []byte("key"), wantErr: false},
		{name: "valid max", key: make([]byte, MaxKeySize), wantErr: false},
		{name: "invalid empty", key: []byte{}, wantErr: true},
		{name: "invalid nil", key: nil, wantErr: true},
		{name: "invalid exceeds max", key: make([]byte, MaxKeySize+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKeySize(tt.key)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateHash(t *testing.T) {
	tests := []struct {
		name        string
		hash        []byte
		expectedLen int
		wantErr     bool
	}{
		{name: "valid any length", hash: []byte("hash"), expectedLen: 0, wantErr: false},
		{name: "valid exact length", hash: make([]byte, 32), expectedLen: 32, wantErr: false},
		{name: "invalid empty", hash: []byte{}, expectedLen: 0, wantErr: true},
		{name: "invalid nil", hash: nil, expectedLen: 0, wantErr: true},
		{name: "invalid wrong length", hash: make([]byte, 16), expectedLen: 32, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHash(tt.hash, tt.expectedLen)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateBlockData(t *testing.T) {
	height := int64(100)
	negHeight := int64(-1)

	tests := []struct {
		name    string
		height  *int64
		hash    []byte
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid",
			height:  &height,
			hash:    []byte("hash"),
			data:    []byte("data"),
			wantErr: false,
		},
		{
			name:    "valid nil height",
			height:  nil,
			hash:    []byte("hash"),
			data:    []byte("data"),
			wantErr: false,
		},
		{
			name:    "invalid negative height",
			height:  &negHeight,
			hash:    []byte("hash"),
			data:    []byte("data"),
			wantErr: true,
		},
		{
			name:    "invalid empty hash",
			height:  &height,
			hash:    []byte{},
			data:    []byte("data"),
			wantErr: true,
		},
		{
			name:    "invalid empty data",
			height:  &height,
			hash:    []byte("hash"),
			data:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBlockData(tt.height, tt.hash, tt.data)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClampBatchSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int32
		maxSize int32
		want    int32
	}{
		{name: "within range", size: 50, maxSize: 0, want: 50},
		{name: "at min", size: 1, maxSize: 0, want: 1},
		{name: "below min", size: 0, maxSize: 0, want: 1},
		{name: "above max", size: MaxBatchSize + 100, maxSize: 0, want: MaxBatchSize},
		{name: "custom max", size: 200, maxSize: 100, want: 100},
		{name: "negative", size: -5, maxSize: 0, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampBatchSize(tt.size, tt.maxSize)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMustNotBeNil(t *testing.T) {
	t.Run("nil pointer", func(t *testing.T) {
		var ptr *int = nil
		err := MustNotBeNil(ptr, "test_field")
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNilPointer)
		require.Contains(t, err.Error(), "test_field")
	})

	t.Run("non-nil pointer", func(t *testing.T) {
		val := 42
		ptr := &val
		err := MustNotBeNil(ptr, "test_field")
		require.NoError(t, err)
	})

	t.Run("nil interface", func(t *testing.T) {
		err := MustNotBeNil(nil, "test_field")
		require.Error(t, err)
	})
}

func TestValidationErrorsWrapping(t *testing.T) {
	t.Run("ValidateHeight wraps ErrInvalidHeight", func(t *testing.T) {
		err := ValidateHeight(-1)
		require.True(t, errors.Is(err, ErrInvalidHeight))
	})

	t.Run("ValidateBatchSize wraps ErrInvalidBatchSize", func(t *testing.T) {
		err := ValidateBatchSize(0, 0)
		require.True(t, errors.Is(err, ErrInvalidBatchSize))
	})

	t.Run("ValidateKeySize wraps ErrEmptyData", func(t *testing.T) {
		err := ValidateKeySize(nil)
		require.True(t, errors.Is(err, ErrEmptyData))
	})

	t.Run("ValidateKeySize wraps ErrDataTooLarge", func(t *testing.T) {
		err := ValidateKeySize(make([]byte, MaxKeySize+1))
		require.True(t, errors.Is(err, ErrDataTooLarge))
	})
}
