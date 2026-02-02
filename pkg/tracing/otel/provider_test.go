package otel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/abi"
)

func TestDefaultProviderConfig(t *testing.T) {
	cfg := DefaultProviderConfig()

	require.Equal(t, "blockberry", cfg.ServiceName)
	require.Equal(t, "0.0.0", cfg.ServiceVersion)
	require.Equal(t, "development", cfg.Environment)
	require.Equal(t, "none", cfg.Exporter)
	require.Equal(t, 0.1, cfg.SampleRate)
}

func TestNewProvider_None(t *testing.T) {
	cfg := ProviderConfig{
		ServiceName: "test-service",
		Exporter:    "none",
		SampleRate:  1.0,
	}

	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Cleanup
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestNewProvider_Stdout(t *testing.T) {
	cfg := ProviderConfig{
		ServiceName: "test-service",
		Exporter:    "stdout",
		SampleRate:  1.0,
	}

	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Cleanup
	err = provider.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestNewProvider_InvalidExporter(t *testing.T) {
	cfg := ProviderConfig{
		ServiceName: "test-service",
		Exporter:    "invalid",
	}

	_, err := NewProvider(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown exporter type")
}

func TestNewProvider_SampleRates(t *testing.T) {
	tests := []struct {
		name       string
		sampleRate float64
	}{
		{"never sample", 0.0},
		{"always sample", 1.0},
		{"ratio based", 0.5},
		{"negative", -1.0},
		{"over 1", 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ProviderConfig{
				ServiceName: "test-service",
				Exporter:    "none",
				SampleRate:  tt.sampleRate,
			}

			provider, err := NewProvider(cfg)
			require.NoError(t, err)
			require.NotNil(t, provider)

			err = provider.Shutdown(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestProviderFromConfig(t *testing.T) {
	cfg := abi.TracerConfig{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "testing",
		Exporter:       "none",
		SampleRate:     0.5,
	}

	provider, err := ProviderFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	err = provider.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestSetupGlobalTracer_Disabled(t *testing.T) {
	cfg := abi.TracerConfig{
		Enabled: false,
	}

	tracer, shutdown, err := SetupGlobalTracer(cfg)
	require.NoError(t, err)
	require.Nil(t, tracer)
	require.NotNil(t, shutdown)

	// Shutdown should work even when disabled
	err = shutdown(context.Background())
	require.NoError(t, err)
}

func TestSetupGlobalTracer_Enabled(t *testing.T) {
	cfg := abi.TracerConfig{
		Enabled:           true,
		ServiceName:       "test-service",
		ServiceVersion:    "1.0.0",
		Environment:       "testing",
		Exporter:          "none",
		SampleRate:        1.0,
		PropagationFormat: "w3c",
	}

	tracer, shutdown, err := SetupGlobalTracer(cfg)
	require.NoError(t, err)
	require.NotNil(t, tracer)
	require.NotNil(t, shutdown)

	// Use the tracer
	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "test-span")
	span.SetAttribute("key", "value")
	span.End()

	// Cleanup
	err = shutdown(ctx)
	require.NoError(t, err)
}

func TestSetupGlobalTracer_PropagationFormats(t *testing.T) {
	formats := []string{"w3c", "b3", "unknown", ""}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			cfg := abi.TracerConfig{
				Enabled:           true,
				ServiceName:       "test-service",
				Exporter:          "none",
				SampleRate:        1.0,
				PropagationFormat: format,
			}

			tracer, shutdown, err := SetupGlobalTracer(cfg)
			require.NoError(t, err)
			require.NotNil(t, tracer)

			err = shutdown(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestNewProvider_Jaeger(t *testing.T) {
	// Note: This test just verifies the exporter can be created.
	// The actual connection to Jaeger will fail in test environment.
	cfg := ProviderConfig{
		ServiceName: "test-service",
		Exporter:    "jaeger",
		Endpoint:    "localhost:4317",
		SampleRate:  1.0,
		Insecure:    true,
	}

	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Cleanup - may timeout since Jaeger isn't running
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = provider.Shutdown(ctx)
}

func TestNewProvider_Zipkin(t *testing.T) {
	// Note: This test just verifies the exporter can be created.
	// The actual connection to Zipkin will fail in test environment.
	cfg := ProviderConfig{
		ServiceName: "test-service",
		Exporter:    "zipkin",
		Endpoint:    "http://localhost:9411/api/v2/spans",
		SampleRate:  1.0,
	}

	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Cleanup - may timeout since Zipkin isn't running
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = provider.Shutdown(ctx)
}

func TestNewProvider_Zipkin_DefaultEndpoint(t *testing.T) {
	// When using Zipkin with OTLP default endpoint, should use Zipkin default
	cfg := ProviderConfig{
		ServiceName: "test-service",
		Exporter:    "zipkin",
		Endpoint:    "localhost:4317", // OTLP default
		SampleRate:  1.0,
	}

	provider, err := NewProvider(cfg)
	require.NoError(t, err)
	require.NotNil(t, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = provider.Shutdown(ctx)
}

func TestExporterTypes(t *testing.T) {
	exporters := []struct {
		name     string
		exporter string
		endpoint string
	}{
		{"none", "none", ""},
		{"empty", "", ""},
		{"stdout", "stdout", ""},
		{"otlp", "otlp", "localhost:4317"},
		{"otlp-grpc", "otlp-grpc", "localhost:4317"},
		{"otlp-http", "otlp-http", "localhost:4318"},
		{"jaeger", "jaeger", "localhost:4317"},
		{"zipkin", "zipkin", "http://localhost:9411/api/v2/spans"},
	}

	for _, tt := range exporters {
		t.Run(tt.name, func(t *testing.T) {
			cfg := ProviderConfig{
				ServiceName: "test-service",
				Exporter:    tt.exporter,
				Endpoint:    tt.endpoint,
				SampleRate:  1.0,
				Insecure:    true,
			}

			provider, err := NewProvider(cfg)
			require.NoError(t, err)
			require.NotNil(t, provider)

			// Quick shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_ = provider.Shutdown(ctx)
		})
	}
}
