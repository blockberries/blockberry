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

	"github.com/blockberries/blockberry/abi"
)

// ProviderConfig contains configuration for creating a TracerProvider.
type ProviderConfig struct {
	// ServiceName is the name of the service.
	ServiceName string

	// ServiceVersion is the version of the service.
	ServiceVersion string

	// Environment is the deployment environment (e.g., "production", "staging").
	Environment string

	// Exporter specifies the exporter type: "otlp-grpc", "otlp-http", "stdout", "none".
	Exporter string

	// Endpoint is the exporter endpoint (for OTLP exporters).
	Endpoint string

	// SampleRate is the sampling rate (0.0 to 1.0).
	SampleRate float64

	// Insecure disables TLS for the connection (for development).
	Insecure bool
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

// ProviderFromConfig creates a TracerProvider from an abi.TracerConfig.
func ProviderFromConfig(cfg abi.TracerConfig) (*sdktrace.TracerProvider, error) {
	return NewProvider(ProviderConfig{
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		Environment:    cfg.Environment,
		Exporter:       cfg.Exporter,
		Endpoint:       cfg.ExporterEndpoint,
		SampleRate:     cfg.SampleRate,
		Insecure:       true, // Default to insecure for development
	})
}

// NewProvider creates a new TracerProvider based on the configuration.
func NewProvider(cfg ProviderConfig) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// Create resource with service information
	// Note: We create a new resource without merging with Default() to avoid
	// schema URL conflicts between different semconv versions.
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
		semconv.DeploymentEnvironment(cfg.Environment),
	)

	// Create exporter
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
		// Jaeger now supports OTLP natively.
		// Use OTLP gRPC to send to Jaeger's OTLP endpoint (default: localhost:4317).
		// For older Jaeger setups, use "jaeger-thrift" if needed.
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
		// Zipkin exporter sends traces in Zipkin's native JSON format.
		// Default endpoint: http://localhost:9411/api/v2/spans
		endpoint := cfg.Endpoint
		if endpoint == "" || endpoint == "localhost:4317" {
			// Use default Zipkin endpoint if OTLP default was specified
			endpoint = "http://localhost:9411/api/v2/spans"
		}
		exp, err := zipkin.New(endpoint)
		if err != nil {
			return nil, fmt.Errorf("creating Zipkin exporter: %w", err)
		}
		exporter = exp

	case "none", "":
		// No exporter - traces will be recorded but not exported
		exporter = nil

	default:
		return nil, fmt.Errorf("unknown exporter type: %s", cfg.Exporter)
	}

	// Create sampler
	var sampler sdktrace.Sampler
	if cfg.SampleRate <= 0 {
		sampler = sdktrace.NeverSample()
	} else if cfg.SampleRate >= 1 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create provider options
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	// Create provider
	provider := sdktrace.NewTracerProvider(opts...)

	return provider, nil
}

// SetupGlobalTracer sets up the global OpenTelemetry tracer and propagator.
// This should be called once at application startup.
func SetupGlobalTracer(cfg abi.TracerConfig) (*Tracer, func(context.Context) error, error) {
	if !cfg.Enabled {
		// Return null tracer when disabled
		return nil, func(context.Context) error { return nil }, nil
	}

	provider, err := ProviderFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("creating provider: %w", err)
	}

	// Set as global provider
	otel.SetTracerProvider(provider)

	// Set up propagator based on format
	var prop propagation.TextMapPropagator
	switch cfg.PropagationFormat {
	case "w3c", "":
		prop = propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		)
	case "b3":
		// B3 propagation requires additional import
		// For now, fall back to W3C
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

	// Create tracer
	tracer := NewTracerWithProvider(cfg.ServiceName, provider)

	// Return shutdown function
	shutdown := func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}

	return tracer, shutdown, nil
}
