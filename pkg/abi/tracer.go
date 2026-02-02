package abi

import (
	"context"
)

// Tracer provides distributed tracing capabilities.
// Implementations can integrate with OpenTelemetry, Jaeger, Zipkin, etc.
type Tracer interface {
	// StartSpan starts a new span with the given name.
	// The returned context contains the span and should be passed to child operations.
	// Call End() on the returned Span when the operation completes.
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)

	// SpanFromContext returns the current span from the context, or nil if none.
	SpanFromContext(ctx context.Context) Span

	// Extract extracts span context from a carrier (e.g., HTTP headers).
	Extract(ctx context.Context, carrier Carrier) context.Context

	// Inject injects span context into a carrier (e.g., HTTP headers).
	Inject(ctx context.Context, carrier Carrier)
}

// Span represents a single operation within a trace.
type Span interface {
	// End completes the span.
	// Must be called when the operation is complete.
	End()

	// SetName updates the span name.
	SetName(name string)

	// SetAttribute sets a key-value attribute on the span.
	SetAttribute(key string, value any)

	// SetAttributes sets multiple attributes at once.
	SetAttributes(attrs ...SpanAttribute)

	// AddEvent adds an event to the span.
	AddEvent(name string, attrs ...SpanAttribute)

	// RecordError records an error on the span.
	// This does not set the span status; call SetStatus separately if needed.
	RecordError(err error)

	// SetStatus sets the span status.
	SetStatus(code StatusCode, description string)

	// IsRecording returns true if the span is recording events.
	IsRecording() bool

	// SpanContext returns the span's context for propagation.
	SpanContext() SpanContext
}

// SpanContext contains the identifiers and flags for a span.
type SpanContext struct {
	// TraceID is the trace identifier.
	TraceID [16]byte

	// SpanID is the span identifier.
	SpanID [8]byte

	// TraceFlags contains trace flags.
	TraceFlags byte

	// TraceState contains vendor-specific trace state.
	TraceState string

	// Remote indicates if this context was received from a remote source.
	Remote bool
}

// IsValid returns true if the span context has valid trace and span IDs.
func (sc SpanContext) IsValid() bool {
	return sc.TraceID != [16]byte{} && sc.SpanID != [8]byte{}
}

// IsSampled returns true if the trace is sampled.
func (sc SpanContext) IsSampled() bool {
	return sc.TraceFlags&0x01 == 0x01
}

// SpanAttribute represents a key-value pair for span attributes and events.
type SpanAttribute struct {
	Key   string
	Value any
}

// SpanString creates a string span attribute.
func SpanString(key, value string) SpanAttribute {
	return SpanAttribute{Key: key, Value: value}
}

// SpanInt creates an integer span attribute.
func SpanInt(key string, value int) SpanAttribute {
	return SpanAttribute{Key: key, Value: value}
}

// SpanInt64 creates an int64 span attribute.
func SpanInt64(key string, value int64) SpanAttribute {
	return SpanAttribute{Key: key, Value: value}
}

// SpanFloat64 creates a float64 span attribute.
func SpanFloat64(key string, value float64) SpanAttribute {
	return SpanAttribute{Key: key, Value: value}
}

// SpanBool creates a boolean span attribute.
func SpanBool(key string, value bool) SpanAttribute {
	return SpanAttribute{Key: key, Value: value}
}

// SpanBytes creates a byte slice span attribute.
func SpanBytes(key string, value []byte) SpanAttribute {
	return SpanAttribute{Key: key, Value: value}
}

// StatusCode represents the status of a span.
type StatusCode int

const (
	// StatusUnset is the default status.
	StatusUnset StatusCode = iota

	// StatusOK indicates the operation completed successfully.
	StatusOK

	// StatusError indicates an error occurred.
	StatusError
)

// String returns the string representation of the status code.
func (c StatusCode) String() string {
	switch c {
	case StatusOK:
		return "OK"
	case StatusError:
		return "Error"
	default:
		return "Unset"
	}
}

// SpanOption configures span creation.
type SpanOption interface {
	applySpan(*spanConfig)
}

type spanConfig struct {
	kind       SpanKind
	attributes []SpanAttribute
	links      []Link
	newRoot    bool
	timestamp  int64
}

// SpanKind indicates the kind of span.
type SpanKind int

const (
	// SpanKindInternal is the default, indicating an internal operation.
	SpanKindInternal SpanKind = iota

	// SpanKindServer indicates a server-side operation.
	SpanKindServer

	// SpanKindClient indicates a client-side operation.
	SpanKindClient

	// SpanKindProducer indicates a message producer.
	SpanKindProducer

	// SpanKindConsumer indicates a message consumer.
	SpanKindConsumer
)

// String returns the string representation of the span kind.
func (k SpanKind) String() string {
	switch k {
	case SpanKindServer:
		return "Server"
	case SpanKindClient:
		return "Client"
	case SpanKindProducer:
		return "Producer"
	case SpanKindConsumer:
		return "Consumer"
	default:
		return "Internal"
	}
}

// Link represents a link to another span.
type Link struct {
	SpanContext SpanContext
	Attributes  []SpanAttribute
}

// Carrier is the interface for propagating span context.
type Carrier interface {
	// Get returns the value for a key.
	Get(key string) string

	// Set sets a key-value pair.
	Set(key, value string)

	// Keys returns all keys in the carrier.
	Keys() []string
}

// MapCarrier implements Carrier using a map.
type MapCarrier map[string]string

// Get returns the value for a key.
func (c MapCarrier) Get(key string) string {
	return c[key]
}

// Set sets a key-value pair.
func (c MapCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns all keys in the carrier.
func (c MapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// NullTracer is a no-op implementation of Tracer.
// Use this when tracing is disabled.
type NullTracer struct{}

// StartSpan returns a no-op span.
func (NullTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, nullSpan{}
}

// SpanFromContext always returns nil.
func (NullTracer) SpanFromContext(ctx context.Context) Span {
	return nil
}

// Extract returns the context unchanged.
func (NullTracer) Extract(ctx context.Context, carrier Carrier) context.Context {
	return ctx
}

// Inject does nothing.
func (NullTracer) Inject(ctx context.Context, carrier Carrier) {}

// nullSpan is a no-op span.
type nullSpan struct{}

func (nullSpan) End()                                          {}
func (nullSpan) SetName(name string)                           {}
func (nullSpan) SetAttribute(key string, value any)            {}
func (nullSpan) SetAttributes(attrs ...SpanAttribute)          {}
func (nullSpan) AddEvent(name string, attrs ...SpanAttribute)  {}
func (nullSpan) RecordError(err error)                     {}
func (nullSpan) SetStatus(code StatusCode, description string) {}
func (nullSpan) IsRecording() bool                         { return false }
func (nullSpan) SpanContext() SpanContext                  { return SpanContext{} }

// Ensure implementations satisfy interfaces.
var (
	_ Tracer  = NullTracer{}
	_ Span    = nullSpan{}
	_ Carrier = MapCarrier{}
)

// TracerConfig contains configuration for the tracer.
type TracerConfig struct {
	// Enabled enables tracing.
	Enabled bool

	// ServiceName is the name of the service for tracing.
	ServiceName string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// Environment is the deployment environment (e.g., "production", "staging").
	Environment string

	// SampleRate is the fraction of traces to sample (0.0 to 1.0).
	SampleRate float64

	// Exporter specifies the trace exporter type.
	// Supported values: "jaeger", "zipkin", "otlp", "stdout", "none"
	Exporter string

	// ExporterEndpoint is the endpoint for the trace exporter.
	ExporterEndpoint string

	// PropagationFormat specifies the context propagation format.
	// Supported values: "w3c", "b3", "jaeger"
	PropagationFormat string
}

// DefaultTracerConfig returns sensible defaults for tracer configuration.
func DefaultTracerConfig() TracerConfig {
	return TracerConfig{
		Enabled:           false,
		ServiceName:       "blockberry",
		ServiceVersion:    "0.0.0",
		Environment:       "development",
		SampleRate:        0.1,
		Exporter:          "none",
		ExporterEndpoint:  "",
		PropagationFormat: "w3c",
	}
}

// Standard span names for common operations.
const (
	SpanCheckTx     = "CheckTx"
	SpanBeginBlock  = "BeginBlock"
	SpanExecuteTx   = "ExecuteTx"
	SpanEndBlock    = "EndBlock"
	SpanCommit      = "Commit"
	SpanQuery       = "Query"
	SpanBroadcastTx = "BroadcastTx"
	SpanBlockSync   = "BlockSync"
	SpanStateSync   = "StateSync"
	SpanConsensus   = "Consensus"
	SpanPropose     = "Propose"
	SpanPrevote     = "Prevote"
	SpanPrecommit   = "Precommit"
)

// Standard attribute keys for spans.
const (
	AttrTxHash      = "tx.hash"
	AttrTxSize      = "tx.size"
	AttrTxCode      = "tx.code"
	AttrBlockHeight = "block.height"
	AttrBlockHash   = "block.hash"
	AttrBlockSize   = "block.size"
	AttrBlockTxs    = "block.txs"
	AttrQueryPath   = "query.path"
	AttrPeerID      = "peer.id"
	AttrPeerAddr    = "peer.addr"
	AttrStream      = "stream"
	AttrRound       = "consensus.round"
	AttrStep        = "consensus.step"
)
