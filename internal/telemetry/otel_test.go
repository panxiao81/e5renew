package telemetry

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestNewConfigFromViper(t *testing.T) {
	viper.Set("otel.service_name", "e5renew")
	viper.Set("otel.service_version", "test")
	viper.Set("otel.environment", "ci")
	viper.Set("otel.otlp_endpoint", "")
	viper.Set("otel.enable_tracing", true)
	viper.Set("otel.enable_metrics", true)
	viper.Set("otel.enable_stdout", true)

	cfg := NewConfig()
	require.Equal(t, "e5renew", cfg.ServiceName)
	require.True(t, cfg.EnableTracing)
	require.True(t, cfg.EnableMetrics)
	require.True(t, cfg.EnableStdout)
}

func TestProviderLifecycle(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := NewProvider(context.Background(), nil, logger)
	require.Error(t, err)

	p, err := NewProvider(context.Background(), &Config{ServiceName: "e5renew", ServiceVersion: "test", Environment: "ci", EnableTracing: true, EnableMetrics: true, EnableStdout: true}, logger)
	require.NoError(t, err)
	require.NotNil(t, p.GetTracer("x"))
	require.NotNil(t, p.GetMeter("x"))
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestProviderInitValidation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := NewProvider(context.Background(), &Config{ServiceName: "x", ServiceVersion: "1", Environment: "dev", EnableTracing: true, EnableMetrics: false, EnableStdout: false, OTLPEndpoint: ""}, logger)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize tracing")
}
