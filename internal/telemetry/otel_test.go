package telemetry

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/resource"
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

func TestProviderInitMetricsAndAccessors(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	res, err := resource.New(context.Background())
	require.NoError(t, err)

	t.Run("init metrics validation without exporters", func(t *testing.T) {
		p := &Provider{logger: logger}
		mp, err := p.initMetrics(context.Background(), &Config{}, res)
		require.Nil(t, mp)
		require.EqualError(t, err, "either enable_stdout or otlp_endpoint must be configured for metrics")
	})

	t.Run("init metrics with stdout succeeds", func(t *testing.T) {
		p := &Provider{logger: logger}
		mp, err := p.initMetrics(context.Background(), &Config{EnableStdout: true}, res)
		require.NoError(t, err)
		require.NotNil(t, mp)
		require.NoError(t, mp.Shutdown(context.Background()))
	})

	t.Run("get tracer and meter use configured providers", func(t *testing.T) {
		p, err := NewProvider(context.Background(), &Config{
			ServiceName:    "e5renew",
			ServiceVersion: "test",
			Environment:    "ci",
			EnableTracing:  true,
			EnableMetrics:  true,
			EnableStdout:   true,
		}, logger)
		require.NoError(t, err)

		tracer := p.GetTracer("configured-tracer")
		meter := p.GetMeter("configured-meter")
		require.NotNil(t, tracer)
		require.NotNil(t, meter)
		require.NoError(t, p.Shutdown(context.Background()))
	})
}
