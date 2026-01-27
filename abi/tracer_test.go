package abi

import (
	"context"
	"errors"
	"testing"
)

func TestNullTracer_StartSpan(t *testing.T) {
	tracer := NullTracer{}
	ctx := context.Background()

	newCtx, span := tracer.StartSpan(ctx, "test-span")

	// Context should be returned unchanged
	if newCtx != ctx {
		t.Error("StartSpan() should return unchanged context")
	}

	// Span should be non-nil
	if span == nil {
		t.Error("StartSpan() returned nil span")
	}

	// Span should be nullSpan
	if _, ok := span.(nullSpan); !ok {
		t.Error("StartSpan() should return nullSpan")
	}
}

func TestNullTracer_SpanFromContext(t *testing.T) {
	tracer := NullTracer{}
	ctx := context.Background()

	span := tracer.SpanFromContext(ctx)

	if span != nil {
		t.Errorf("SpanFromContext() = %v, want nil", span)
	}
}

func TestNullTracer_Extract(t *testing.T) {
	tracer := NullTracer{}
	ctx := context.Background()
	carrier := MapCarrier{}

	newCtx := tracer.Extract(ctx, carrier)

	if newCtx != ctx {
		t.Error("Extract() should return unchanged context")
	}
}

func TestNullTracer_Inject(t *testing.T) {
	tracer := NullTracer{}
	ctx := context.Background()
	carrier := MapCarrier{}

	// Should not panic
	tracer.Inject(ctx, carrier)

	// Carrier should remain empty
	if len(carrier) != 0 {
		t.Errorf("Inject() modified carrier, got %v", carrier)
	}
}

func TestNullSpan_Operations(t *testing.T) {
	tracer := NullTracer{}
	_, span := tracer.StartSpan(context.Background(), "test")

	// All operations should be no-ops (not panic)
	span.End()
	span.SetName("new-name")
	span.SetAttribute("key", "value")
	span.SetAttributes(SpanString("k1", "v1"), SpanInt("k2", 42))
	span.AddEvent("event", SpanBool("flag", true))
	span.RecordError(errors.New("test error"))
	span.SetStatus(StatusError, "error description")
}

func TestNullSpan_IsRecording(t *testing.T) {
	tracer := NullTracer{}
	_, span := tracer.StartSpan(context.Background(), "test")

	if span.IsRecording() {
		t.Error("IsRecording() = true, want false")
	}
}

func TestNullSpan_SpanContext(t *testing.T) {
	tracer := NullTracer{}
	_, span := tracer.StartSpan(context.Background(), "test")

	sc := span.SpanContext()

	// Should return empty SpanContext
	if sc.IsValid() {
		t.Error("SpanContext().IsValid() = true, want false")
	}
	if sc.IsSampled() {
		t.Error("SpanContext().IsSampled() = true, want false")
	}
}

func TestSpanContext_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		sc    SpanContext
		valid bool
	}{
		{
			name:  "empty",
			sc:    SpanContext{},
			valid: false,
		},
		{
			name: "only trace ID",
			sc: SpanContext{
				TraceID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			},
			valid: false,
		},
		{
			name: "only span ID",
			sc: SpanContext{
				SpanID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			},
			valid: false,
		},
		{
			name: "both IDs",
			sc: SpanContext{
				TraceID: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
				SpanID:  [8]byte{1, 2, 3, 4, 5, 6, 7, 8},
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sc.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestSpanContext_IsSampled(t *testing.T) {
	tests := []struct {
		name    string
		flags   byte
		sampled bool
	}{
		{"no flags", 0x00, false},
		{"sampled", 0x01, true},
		{"other flags only", 0x02, false},
		{"sampled with other flags", 0x03, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := SpanContext{TraceFlags: tt.flags}
			if got := sc.IsSampled(); got != tt.sampled {
				t.Errorf("IsSampled() = %v, want %v", got, tt.sampled)
			}
		})
	}
}

func TestMapCarrier(t *testing.T) {
	carrier := MapCarrier{}

	// Set values
	carrier.Set("key1", "value1")
	carrier.Set("key2", "value2")

	// Get values
	if got := carrier.Get("key1"); got != "value1" {
		t.Errorf("Get(key1) = %q, want %q", got, "value1")
	}
	if got := carrier.Get("key2"); got != "value2" {
		t.Errorf("Get(key2) = %q, want %q", got, "value2")
	}
	if got := carrier.Get("missing"); got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}

	// Keys
	keys := carrier.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() returned %d keys, want 2", len(keys))
	}
}

func TestSpanAttribute_Helpers(t *testing.T) {
	// Test all attribute helper functions
	strAttr := SpanString("str", "value")
	if strAttr.Key != "str" || strAttr.Value != "value" {
		t.Errorf("SpanString() = %+v, unexpected", strAttr)
	}

	intAttr := SpanInt("int", 42)
	if intAttr.Key != "int" || intAttr.Value != 42 {
		t.Errorf("SpanInt() = %+v, unexpected", intAttr)
	}

	int64Attr := SpanInt64("int64", int64(123456789))
	if int64Attr.Key != "int64" || int64Attr.Value != int64(123456789) {
		t.Errorf("SpanInt64() = %+v, unexpected", int64Attr)
	}

	floatAttr := SpanFloat64("float", 3.14)
	if floatAttr.Key != "float" || floatAttr.Value != 3.14 {
		t.Errorf("SpanFloat64() = %+v, unexpected", floatAttr)
	}

	boolAttr := SpanBool("bool", true)
	if boolAttr.Key != "bool" || boolAttr.Value != true {
		t.Errorf("SpanBool() = %+v, unexpected", boolAttr)
	}

	bytesAttr := SpanBytes("bytes", []byte{1, 2, 3})
	if bytesAttr.Key != "bytes" {
		t.Errorf("SpanBytes() key = %q, want %q", bytesAttr.Key, "bytes")
	}
}

func TestStatusCode_String(t *testing.T) {
	tests := []struct {
		code     StatusCode
		expected string
	}{
		{StatusUnset, "Unset"},
		{StatusOK, "OK"},
		{StatusError, "Error"},
		{StatusCode(99), "Unset"}, // Unknown defaults to Unset
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.code.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSpanKind_String(t *testing.T) {
	tests := []struct {
		kind     SpanKind
		expected string
	}{
		{SpanKindInternal, "Internal"},
		{SpanKindServer, "Server"},
		{SpanKindClient, "Client"},
		{SpanKindProducer, "Producer"},
		{SpanKindConsumer, "Consumer"},
		{SpanKind(99), "Internal"}, // Unknown defaults to Internal
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDefaultTracerConfig(t *testing.T) {
	cfg := DefaultTracerConfig()

	if cfg.Enabled {
		t.Error("Enabled should be false by default")
	}
	if cfg.ServiceName != "blockberry" {
		t.Errorf("ServiceName = %q, want %q", cfg.ServiceName, "blockberry")
	}
	if cfg.Environment != "development" {
		t.Errorf("Environment = %q, want %q", cfg.Environment, "development")
	}
	if cfg.SampleRate != 0.1 {
		t.Errorf("SampleRate = %f, want %f", cfg.SampleRate, 0.1)
	}
	if cfg.Exporter != "none" {
		t.Errorf("Exporter = %q, want %q", cfg.Exporter, "none")
	}
	if cfg.PropagationFormat != "w3c" {
		t.Errorf("PropagationFormat = %q, want %q", cfg.PropagationFormat, "w3c")
	}
}

func TestSpanNameConstants(t *testing.T) {
	// Verify span names are defined
	names := []string{
		SpanCheckTx,
		SpanBeginBlock,
		SpanExecuteTx,
		SpanEndBlock,
		SpanCommit,
		SpanQuery,
		SpanBroadcastTx,
		SpanBlockSync,
		SpanStateSync,
		SpanConsensus,
		SpanPropose,
		SpanPrevote,
		SpanPrecommit,
	}

	seen := make(map[string]bool)
	for _, name := range names {
		if name == "" {
			t.Error("Span name constant is empty")
		}
		if seen[name] {
			t.Errorf("Duplicate span name: %q", name)
		}
		seen[name] = true
	}
}

func TestAttrKeyConstants(t *testing.T) {
	// Verify attribute keys are defined
	keys := []string{
		AttrTxHash,
		AttrTxSize,
		AttrTxCode,
		AttrBlockHeight,
		AttrBlockHash,
		AttrBlockSize,
		AttrBlockTxs,
		AttrQueryPath,
		AttrPeerID,
		AttrPeerAddr,
		AttrStream,
		AttrRound,
		AttrStep,
	}

	seen := make(map[string]bool)
	for _, key := range keys {
		if key == "" {
			t.Error("Attribute key constant is empty")
		}
		if seen[key] {
			t.Errorf("Duplicate attribute key: %q", key)
		}
		seen[key] = true
	}
}

func TestInterfaces(t *testing.T) {
	// Verify interface implementations
	var _ Tracer = NullTracer{}
	var _ Span = nullSpan{}
	var _ Carrier = MapCarrier{}
}
