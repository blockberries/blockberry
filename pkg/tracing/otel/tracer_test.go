package otel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/blockberries/blockberry/pkg/tracing"
)

func createTestTracer(t *testing.T) (*Tracer, *tracetest.InMemoryExporter) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSyncer(exporter),
	)

	tracer := NewTracerWithProvider("test-service", provider)
	return tracer, exporter
}

func TestTracer_StartSpan(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "test-span")

	require.NotNil(t, span)
	require.True(t, span.IsRecording())

	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "test-span", spans[0].Name)
}

func TestTracer_SpanFromContext(t *testing.T) {
	tracer, _ := createTestTracer(t)

	ctx := context.Background()
	span := tracer.SpanFromContext(ctx)
	require.Nil(t, span)

	ctx, startedSpan := tracer.StartSpan(ctx, "test-span")
	defer startedSpan.End()

	fromCtx := tracer.SpanFromContext(ctx)
	require.NotNil(t, fromCtx)
}

func TestSpan_SetAttribute(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	span.SetAttribute("string-key", "string-value")
	span.SetAttribute("int-key", 42)
	span.SetAttribute("int64-key", int64(123456789))
	span.SetAttribute("float-key", 3.14)
	span.SetAttribute("bool-key", true)
	span.SetAttribute("bytes-key", []byte("bytes"))

	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	attrs := spans[0].Attributes
	require.NotEmpty(t, attrs)
}

func TestSpan_SetAttributes(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	span.SetAttributes(
		tracing.SpanString("key1", "value1"),
		tracing.SpanInt("key2", 42),
		tracing.SpanBool("key3", true),
	)

	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.GreaterOrEqual(t, len(spans[0].Attributes), 3)
}

func TestSpan_AddEvent(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	span.AddEvent("test-event", tracing.SpanString("event-key", "event-value"))

	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Events, 1)
	require.Equal(t, "test-event", spans[0].Events[0].Name)
}

func TestSpan_RecordError(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")

	testErr := errors.New("test error")
	span.RecordError(testErr)

	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.NotEmpty(t, spans[0].Events)
}

func TestSpan_SetStatus(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	tests := []struct {
		name string
		code tracing.StatusCode
		desc string
	}{
		{"OK", tracing.StatusOK, "success"},
		{"Error", tracing.StatusError, "something went wrong"},
		{"Unset", tracing.StatusUnset, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exporter.Reset()

			ctx := context.Background()
			_, span := tracer.StartSpan(ctx, "test-span")
			span.SetStatus(tt.code, tt.desc)
			span.End()

			spans := exporter.GetSpans()
			require.Len(t, spans, 1)
		})
	}
}

func TestSpan_SetName(t *testing.T) {
	tracer, exporter := createTestTracer(t)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "original-name")

	span.SetName("updated-name")

	span.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "updated-name", spans[0].Name)
}

func TestSpan_SpanContext(t *testing.T) {
	tracer, _ := createTestTracer(t)

	ctx := context.Background()
	_, span := tracer.StartSpan(ctx, "test-span")
	defer span.End()

	sc := span.SpanContext()

	require.True(t, sc.IsValid())
	require.NotEqual(t, [16]byte{}, sc.TraceID)
	require.NotEqual(t, [8]byte{}, sc.SpanID)
}

func TestCarrierAdapter(t *testing.T) {
	carrier := tracing.MapCarrier{
		"key1": "value1",
		"key2": "value2",
	}

	adapter := carrierAdapter{carrier: carrier}

	require.Equal(t, "value1", adapter.Get("key1"))
	require.Equal(t, "value2", adapter.Get("key2"))
	require.Equal(t, "", adapter.Get("missing"))

	adapter.Set("key3", "value3")
	require.Equal(t, "value3", adapter.Get("key3"))

	keys := adapter.Keys()
	require.Len(t, keys, 3)
}

func TestTracer_ExtractInject(t *testing.T) {
	tracer, _ := createTestTracer(t)

	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "test-span")
	defer span.End()

	carrier := tracing.MapCarrier{}
	tracer.Inject(ctx, carrier)

	require.NotEmpty(t, carrier)

	newCtx := tracer.Extract(context.Background(), carrier)
	require.NotNil(t, newCtx)
}

func TestConvertAttribute(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"string", "key", "value"},
		{"int", "key", 42},
		{"int64", "key", int64(123)},
		{"float64", "key", 3.14},
		{"bool", "key", true},
		{"bytes", "key", []byte("bytes")},
		{"string slice", "key", []string{"a", "b"}},
		{"int slice", "key", []int{1, 2, 3}},
		{"unknown", "key", struct{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := convertAttribute(tt.key, tt.value)
			require.Equal(t, tt.key, string(attr.Key))
		})
	}
}

func TestNewTracer(t *testing.T) {
	tracer := NewTracer("test-service")
	require.NotNil(t, tracer)
	require.NotNil(t, tracer.tracer)
	require.NotNil(t, tracer.propagator)
}

func TestInterfaceCompliance(t *testing.T) {
	var _ tracing.Tracer = (*Tracer)(nil)
	var _ tracing.Span = (*Span)(nil)
}
