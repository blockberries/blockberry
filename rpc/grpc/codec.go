package grpc

import (
	"fmt"
	"reflect"

	"github.com/blockberries/cramberry/pkg/cramberry"
	"google.golang.org/grpc/encoding"
)

// CodecName is the name of the Cramberry codec for gRPC.
const CodecName = "cramberry"

// CramberryCodec implements the gRPC encoding.Codec interface using Cramberry
// for message serialization instead of the default Protocol Buffers.
type CramberryCodec struct {
	options cramberry.Options
}

// NewCramberryCodec creates a new Cramberry codec with default options.
func NewCramberryCodec() *CramberryCodec {
	return &CramberryCodec{
		options: cramberry.DefaultOptions,
	}
}

// NewCramberryCodecWithOptions creates a new Cramberry codec with custom options.
func NewCramberryCodecWithOptions(opts cramberry.Options) *CramberryCodec {
	return &CramberryCodec{
		options: opts,
	}
}

// CramberryMarshaler is an interface for types that can marshal themselves
// using the optimized Cramberry encoding.
type CramberryMarshaler interface {
	MarshalCramberry() ([]byte, error)
}

// CramberryUnmarshaler is an interface for types that can unmarshal themselves
// from Cramberry encoding.
type CramberryUnmarshaler interface {
	UnmarshalCramberry([]byte) error
}

// Marshal serializes a message to Cramberry format.
// If the message implements CramberryMarshaler, it uses the optimized method.
// Otherwise, it falls back to reflection-based marshaling.
func (c *CramberryCodec) Marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	// Use the optimized method if available
	if m, ok := v.(CramberryMarshaler); ok {
		return m.MarshalCramberry()
	}

	// Fall back to reflection-based marshaling
	return cramberry.MarshalWithOptions(v, c.options)
}

// Unmarshal deserializes a message from Cramberry format.
// If the message implements CramberryUnmarshaler, it uses the optimized method.
// Otherwise, it falls back to reflection-based unmarshaling.
func (c *CramberryCodec) Unmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}

	// Use the optimized method if available
	if m, ok := v.(CramberryUnmarshaler); ok {
		return m.UnmarshalCramberry(data)
	}

	// Fall back to reflection-based unmarshaling
	// The target must be a pointer
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return fmt.Errorf("unmarshal target must be a non-nil pointer, got %T", v)
	}

	return cramberry.UnmarshalWithOptions(data, v, c.options)
}

// Name returns the name of the codec.
func (c *CramberryCodec) Name() string {
	return CodecName
}

// Register registers the Cramberry codec with gRPC.
// This should be called before creating any gRPC connections or servers.
func Register() {
	RegisterWithOptions(cramberry.DefaultOptions)
}

// RegisterWithOptions registers the Cramberry codec with custom options.
func RegisterWithOptions(opts cramberry.Options) {
	encoding.RegisterCodec(NewCramberryCodecWithOptions(opts))
}

func init() {
	// Auto-register the codec when the package is imported
	Register()
}
