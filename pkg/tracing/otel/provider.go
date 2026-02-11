package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/blockberries/blockberry/pkg/tracing"
)

// ProviderConfig contains configuration for creating a TracerProvider.
type ProviderConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Exporter       string
	Endpoint       string
	SampleRate     float64
	Insecure       bool
}

// DefaultProviderConfig returns sensible defaults for provider configuration.
func DefaultProviderConfig() ProviderConfig {
	return ProviderConfig{
		ServiceName:    "blockberry",
		ServiceVersion: "0.0.0",
		Environment:    "development",
		Exporter:       "none",
		Endpoint:       "localhost:4317",
		SampleRate:     0.1,
		Insecure:       true,
	}
}

// ProviderFromConfig creates a TracerProvider from a tracing.TracerConfig.
func ProviderFromConfig(cfg tracing.TracerConfig) (*sdktrace.TracerProvider, error) {
	return NewProvider(ProviderConfig{
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		Environment:    cfg.Environment,
		Exporter:       cfg.Exporter,
		Endpoint:       cfg.ExporterEndpoint,
		SampleRate:     cfg.SampleRate,
		Insecure:       true,
	})
}

// NewProvider creates a new TracerProvider based on the configuration.
func NewProvider(cfg ProviderConfig) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment(cfg.Environment),
	)

	var exporter sdktrace.SpanExporter
	switch cfg.Exporter {
	case "otlp-grpc", "otlp":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exp, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
		if err != nil {
			return nil, fmt.Errorf("creating OTLP gRPC exporter: %w", err)
		}
		exporter = exp

	case "otlp-http":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exp, err := otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
		if err != nil {
			return nil, fmt.Errorf("creating OTLP HTTP exporter: %w", err)
		}
		exporter = exp

	case "stdout":
		exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("creating stdout exporter: %w", err)
		}
		exporter = exp

	case "jaeger":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exp, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
		if err != nil {
			return nil, fmt.Errorf("creating Jaeger OTLP exporter: %w", err)
		}
		exporter = exp

	case "zipkin":
		endpoint := cfg.Endpoint
		if endpoint == "" || endpoint == "localhost:4317" {
			endpoint = "http://localhost:9411/api/v2/spans"
		}
		exp, err := zipkin.New(endpoint)
		if err != nil {
			return nil, fmt.Errorf("creating Zipkin exporter: %w", err)
		}
		exporter = exp

	case "none", "":
		exporter = nil

	default:
		return nil, fmt.Errorf("unknown exporter type: %s", cfg.Exporter)
	}

	var sampler sdktrace.Sampler
	if cfg.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else if cfg.SampleRate >= 1 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	provider := sdktrace.NewTracerProvider(opts...)

	return provider, nil
}

// SetupGlobalTracer sets up the global OpenTelemetry tracer and propagator.
func SetupGlobalTracer(cfg tracing.TracerConfig) (*Tracer, func(context.Context) error, error) {
	if !cfg.Enabled {
		return nil, func(context.Context) error { return nil }, nil
	}

	provider, err := ProviderFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("creating provider: %w", err)
	}

	otel.SetTracerProvider(provider)

	var prop propagation.TextMapPropagator
	switch cfg.PropagationFormat {
	case "w3c", "":
		prop = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	case "b3":
		prop = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	default:
		prop = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	}
	otel.SetTextMapPropagator(prop)

	tracer := NewTracerWithProvider(cfg.ServiceName, provider)

	shutdown := func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}

	return tracer, shutdown, nil
}
