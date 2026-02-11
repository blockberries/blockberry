// Package otel provides an OpenTelemetry-based implementation of tracing.Tracer.
package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/blockberries/blockberry/pkg/tracing"
)

// Tracer implements tracing.Tracer using OpenTelemetry.
type Tracer struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// NewTracer creates a new OpenTelemetry-based tracer.
func NewTracer(serviceName string) *Tracer {
	return &Tracer{
		tracer:     otel.Tracer(serviceName),
		propagator: otel.GetTextMapPropagator(),
	}
}

// NewTracerWithProvider creates a tracer using a specific TracerProvider.
func NewTracerWithProvider(serviceName string, provider trace.TracerProvider) *Tracer {
	return &Tracer{
		tracer:     provider.Tracer(serviceName),
		propagator: otel.GetTextMapPropagator(),
	}
}

// StartSpan starts a new span with the given name.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...tracing.SpanOption) (context.Context, tracing.Span) {
	otelOpts := convertSpanOptions(opts)
	ctx, otelSpan := t.tracer.Start(ctx, name, otelOpts...)
	return ctx, &Span{span: otelSpan}
}

// SpanFromContext returns the current span from the context.
func (t *Tracer) SpanFromContext(ctx context.Context) tracing.Span {
	otelSpan := trace.SpanFromContext(ctx)
	if !otelSpan.SpanContext().IsValid() {
		return nil
	}
	return &Span{span: otelSpan}
}

// Extract extracts span context from a carrier.
func (t *Tracer) Extract(ctx context.Context, carrier tracing.Carrier) context.Context {
	return t.propagator.Extract(ctx, carrierAdapter{carrier})
}

// Inject injects span context into a carrier.
func (t *Tracer) Inject(ctx context.Context, carrier tracing.Carrier) {
	t.propagator.Inject(ctx, carrierAdapter{carrier})
}

// Span wraps an OpenTelemetry span to implement tracing.Span.
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
func (s *Span) SetAttributes(attrs ...tracing.SpanAttribute) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		otelAttrs = append(otelAttrs, convertAttribute(attr.Key, attr.Value))
	}
	s.span.SetAttributes(otelAttrs...)
}

// AddEvent adds an event to the span.
func (s *Span) AddEvent(name string, attrs ...tracing.SpanAttribute) {
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
func (s *Span) SetStatus(code tracing.StatusCode, description string) {
	var otelCode codes.Code
	switch code {
	case tracing.StatusOK:
		otelCode = codes.Ok
	case tracing.StatusError:
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
func (s *Span) SpanContext() tracing.SpanContext {
	sc := s.span.SpanContext()
	return tracing.SpanContext{
		TraceID:    sc.TraceID(),
		SpanID:     sc.SpanID(),
		TraceFlags: byte(sc.TraceFlags()),
		TraceState: sc.TraceState().String(),
		Remote:     sc.IsRemote(),
	}
}

// carrierAdapter adapts tracing.Carrier to propagation.TextMapCarrier.
type carrierAdapter struct {
	carrier tracing.Carrier
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
		return attribute.String(key, toString(v))
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

func convertSpanOptions(opts []tracing.SpanOption) []trace.SpanStartOption {
	return nil
}

var _ tracing.Tracer = (*Tracer)(nil)
var _ tracing.Span = (*Span)(nil)
