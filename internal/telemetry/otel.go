package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Config holds the OpenTelemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	EnableTracing  bool
	EnableMetrics  bool
	EnableStdout   bool
}

// Provider manages OpenTelemetry providers
type Provider struct {
	traceProvider  *trace.TracerProvider
	metricProvider *metric.MeterProvider
	logger         *slog.Logger
}

// NewConfig creates a new OpenTelemetry configuration from viper settings
func NewConfig() *Config {
	return &Config{
		ServiceName:    viper.GetString("otel.service_name"),
		ServiceVersion: viper.GetString("otel.service_version"),
		Environment:    viper.GetString("otel.environment"),
		OTLPEndpoint:   viper.GetString("otel.otlp_endpoint"),
		EnableTracing:  viper.GetBool("otel.enable_tracing"),
		EnableMetrics:  viper.GetBool("otel.enable_metrics"),
		EnableStdout:   viper.GetBool("otel.enable_stdout"),
	}
}

// NewProvider creates a new OpenTelemetry provider
func NewProvider(ctx context.Context, config *Config, logger *slog.Logger) (*Provider, error) {
	if config == nil {
		return nil, errors.New("config is required")
	}

	provider := &Provider{
		logger: logger,
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracing
	if config.EnableTracing {
		tp, err := provider.initTracing(ctx, config, res)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tracing: %w", err)
		}
		provider.traceProvider = tp
		otel.SetTracerProvider(tp)
	}

	// Initialize metrics
	if config.EnableMetrics {
		mp, err := provider.initMetrics(ctx, config, res)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize metrics: %w", err)
		}
		provider.metricProvider = mp
		otel.SetMeterProvider(mp)
	}

	// Set text map propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("OpenTelemetry initialized successfully",
		slog.String("service", config.ServiceName),
		slog.String("version", config.ServiceVersion),
		slog.String("environment", config.Environment),
		slog.Bool("tracing", config.EnableTracing),
		slog.Bool("metrics", config.EnableMetrics),
	)

	return provider, nil
}

// initTracing initializes the tracing provider
func (p *Provider) initTracing(ctx context.Context, config *Config, res *resource.Resource) (*trace.TracerProvider, error) {
	var exporter trace.SpanExporter
	var err error

	if config.EnableStdout {
		// Use stdout exporter for development
		exporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	} else if config.OTLPEndpoint != "" {
		// Use OTLP exporter for production
		exporter, err = otlptracehttp.New(ctx,
			otlptracehttp.WithEndpoint(config.OTLPEndpoint),
			otlptracehttp.WithInsecure(), // Use WithTLSClientConfig for secure connections
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
		}
	} else {
		return nil, errors.New("either enable_stdout or otlp_endpoint must be configured for tracing")
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)

	return tp, nil
}

// initMetrics initializes the metrics provider
func (p *Provider) initMetrics(ctx context.Context, config *Config, res *resource.Resource) (*metric.MeterProvider, error) {
	var exporter metric.Exporter
	var err error

	if config.EnableStdout {
		// Use stdout exporter for development
		exporter, err = stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout metric exporter: %w", err)
		}
	} else if config.OTLPEndpoint != "" {
		// Use OTLP exporter for production
		exporter, err = otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpoint(config.OTLPEndpoint),
			otlpmetrichttp.WithInsecure(), // Use WithTLSClientConfig for secure connections
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
		}
	} else {
		return nil, errors.New("either enable_stdout or otlp_endpoint must be configured for metrics")
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter,
			metric.WithInterval(30*time.Second),
		)),
		metric.WithResource(res),
	)

	return mp, nil
}

// Shutdown gracefully shuts down the OpenTelemetry providers
func (p *Provider) Shutdown(ctx context.Context) error {
	var errors []error

	if p.traceProvider != nil {
		if err := p.traceProvider.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown trace provider: %w", err))
		}
	}

	if p.metricProvider != nil {
		if err := p.metricProvider.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown metric provider: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("shutdown errors: %v", errors)
	}

	p.logger.Info("OpenTelemetry shutdown completed")
	return nil
}

// GetTracer returns a tracer for the given name
func (p *Provider) GetTracer(name string) oteltrace.Tracer {
	if p.traceProvider == nil {
		return otel.Tracer(name)
	}
	return p.traceProvider.Tracer(name)
}

// GetMeter returns a meter for the given name
func (p *Provider) GetMeter(name string) otelmetric.Meter {
	if p.metricProvider == nil {
		return otel.Meter(name)
	}
	return p.metricProvider.Meter(name)
}
