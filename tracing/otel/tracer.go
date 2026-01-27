// Package otel provides an OpenTelemetry-based implementation of abi.Tracer.
package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/blockberries/blockberry/abi"
)

// Tracer implements abi.Tracer using OpenTelemetry.
type Tracer struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// NewTracer creates a new OpenTelemetry-based tracer.
// The serviceName is used to identify this service in traces.
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		tracer:     otel.Tracer(serviceName),
		propagator: otel.GetTextMapPropagator(),
	}
}

// NewTracerWithProvider creates a tracer using a specific TracerProvider.
// This is useful for testing or when using a custom provider configuration.
func NewTracerWithProvider(serviceName string, provider trace.TracerProvider) *Tracer {
	return &Tracer{
		tracer:     provider.Tracer(serviceName),
		propagator: otel.GetTextMapPropagator(),
	}
}

// StartSpan starts a new span with the given name.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...abi.SpanOption) (context.Context, abi.Span) {
	// Convert ABI options to OTel options
	otelOpts := convertSpanOptions(opts)

	ctx, otelSpan := t.tracer.Start(ctx, name, otelOpts...)

	return ctx, &Span{span: otelSpan}
}

// SpanFromContext returns the current span from the context.
func (t *Tracer) SpanFromContext(ctx context.Context) abi.Span {
	otelSpan := trace.SpanFromContext(ctx)
	if !otelSpan.SpanContext().IsValid() {
		return nil
	}
	return &Span{span: otelSpan}
}

// Extract extracts span context from a carrier.
func (t *Tracer) Extract(ctx context.Context, carrier abi.Carrier) context.Context {
	return t.propagator.Extract(ctx, carrierAdapter{carrier})
}

// Inject injects span context into a carrier.
func (t *Tracer) Inject(ctx context.Context, carrier abi.Carrier) {
	t.propagator.Inject(ctx, carrierAdapter{carrier})
}

// Span wraps an OpenTelemetry span to implement abi.Span.
type Span struct {
	span trace.Span
}

// End completes the span.
func (s *Span) End() {
	s.span.End()
}

// SetName updates the span name.
func (s *Span) SetName(name string) {
	s.span.SetName(name)
}

// SetAttribute sets a key-value attribute on the span.
func (s *Span) SetAttribute(key string, value any) {
	s.span.SetAttributes(convertAttribute(key, value))
}

// SetAttributes sets multiple attributes at once.
func (s *Span) SetAttributes(attrs ...abi.SpanAttribute) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		otelAttrs = append(otelAttrs, convertAttribute(attr.Key, attr.Value))
	}
	s.span.SetAttributes(otelAttrs...)
}

// AddEvent adds an event to the span.
func (s *Span) AddEvent(name string, attrs ...abi.SpanAttribute) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		otelAttrs = append(otelAttrs, convertAttribute(attr.Key, attr.Value))
	}
	s.span.AddEvent(name, trace.WithAttributes(otelAttrs...))
}

// RecordError records an error on the span.
func (s *Span) RecordError(err error) {
	s.span.RecordError(err)
}

// SetStatus sets the span status.
func (s *Span) SetStatus(code abi.StatusCode, description string) {
	var otelCode codes.Code
	switch code {
	case abi.StatusOK:
		otelCode = codes.Ok
	case abi.StatusError:
		otelCode = codes.Error
	default:
		otelCode = codes.Unset
	}
	s.span.SetStatus(otelCode, description)
}

// IsRecording returns true if the span is recording events.
func (s *Span) IsRecording() bool {
	return s.span.IsRecording()
}

// SpanContext returns the span's context for propagation.
func (s *Span) SpanContext() abi.SpanContext {
	sc := s.span.SpanContext()
	return abi.SpanContext{
		TraceID:    sc.TraceID(),
		SpanID:     sc.SpanID(),
		TraceFlags: byte(sc.TraceFlags()),
		TraceState: sc.TraceState().String(),
		Remote:     sc.IsRemote(),
	}
}

// carrierAdapter adapts abi.Carrier to propagation.TextMapCarrier.
type carrierAdapter struct {
	carrier abi.Carrier
}

func (c carrierAdapter) Get(key string) string {
	return c.carrier.Get(key)
}

func (c carrierAdapter) Set(key, value string) {
	c.carrier.Set(key, value)
}

func (c carrierAdapter) Keys() []string {
	return c.carrier.Keys()
}

// convertAttribute converts a key-value pair to an OTel attribute.
func convertAttribute(key string, value any) attribute.KeyValue {
	switch v := value.(type) {
	case string:
		return attribute.String(key, v)
	case int:
		return attribute.Int(key, v)
	case int64:
		return attribute.Int64(key, v)
	case float64:
		return attribute.Float64(key, v)
	case bool:
		return attribute.Bool(key, v)
	case []byte:
		return attribute.String(key, string(v))
	case []string:
		return attribute.StringSlice(key, v)
	case []int:
		return attribute.IntSlice(key, v)
	case []int64:
		return attribute.Int64Slice(key, v)
	case []float64:
		return attribute.Float64Slice(key, v)
	case []bool:
		return attribute.BoolSlice(key, v)
	default:
		// Fallback to string representation
		return attribute.String(key, toString(v))
	}
}

// toString converts any value to a string.
func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

// convertSpanOptions converts ABI span options to OTel options.
func convertSpanOptions(opts []abi.SpanOption) []trace.SpanStartOption {
	// For now, we don't have complex option handling
	// This can be extended to support SpanKind, Links, etc.
	return nil
}

// Ensure Tracer implements abi.Tracer.
var _ abi.Tracer = (*Tracer)(nil)

// Ensure Span implements abi.Span.
var _ abi.Span = (*Span)(nil)
