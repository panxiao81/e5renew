package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds all application metrics
type Metrics struct {
	// HTTP metrics
	HTTPRequestDuration metric.Float64Histogram
	HTTPRequestCount    metric.Int64Counter
	HTTPActiveRequests  metric.Int64UpDownCounter

	// Authentication metrics
	AuthAttempts     metric.Int64Counter
	AuthSuccess      metric.Int64Counter
	AuthFailures     metric.Int64Counter
	SessionCreated   metric.Int64Counter
	SessionDestroyed metric.Int64Counter

	// Database metrics
	DBConnections      metric.Int64UpDownCounter
	DBQueryDuration    metric.Float64Histogram
	DBQueryCount       metric.Int64Counter
	DBConnectionErrors metric.Int64Counter

	// Job metrics
	JobExecutions    metric.Int64Counter
	JobDuration      metric.Float64Histogram
	JobFailures      metric.Int64Counter
	GraphAPIRequests metric.Int64Counter
	GraphAPIErrors   metric.Int64Counter

	// Application metrics
	AppStartTime  metric.Float64Gauge
	ConfigReloads metric.Int64Counter
	ErrorCount    metric.Int64Counter

	logger *slog.Logger
}

// NewMetrics creates a new metrics instance
func NewMetrics(meter metric.Meter, logger *slog.Logger) (*Metrics, error) {
	var err error
	m := &Metrics{
		logger: logger,
	}

	// HTTP metrics
	m.HTTPRequestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_request_duration_seconds metric: %w", err)
	}

	m.HTTPRequestCount, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_requests_total metric: %w", err)
	}

	m.HTTPActiveRequests, err = meter.Int64UpDownCounter(
		"http_active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create http_active_requests metric: %w", err)
	}

	// Authentication metrics
	m.AuthAttempts, err = meter.Int64Counter(
		"auth_attempts_total",
		metric.WithDescription("Total number of authentication attempts"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth_attempts_total metric: %w", err)
	}

	m.AuthSuccess, err = meter.Int64Counter(
		"auth_success_total",
		metric.WithDescription("Total number of successful authentications"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth_success_total metric: %w", err)
	}

	m.AuthFailures, err = meter.Int64Counter(
		"auth_failures_total",
		metric.WithDescription("Total number of authentication failures"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth_failures_total metric: %w", err)
	}

	m.SessionCreated, err = meter.Int64Counter(
		"sessions_created_total",
		metric.WithDescription("Total number of sessions created"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sessions_created_total metric: %w", err)
	}

	m.SessionDestroyed, err = meter.Int64Counter(
		"sessions_destroyed_total",
		metric.WithDescription("Total number of sessions destroyed"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create sessions_destroyed_total metric: %w", err)
	}

	// Database metrics
	m.DBConnections, err = meter.Int64UpDownCounter(
		"database_connections_active",
		metric.WithDescription("Number of active database connections"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database_connections_active metric: %w", err)
	}

	m.DBQueryDuration, err = meter.Float64Histogram(
		"database_query_duration_seconds",
		metric.WithDescription("Duration of database queries in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database_query_duration_seconds metric: %w", err)
	}

	m.DBQueryCount, err = meter.Int64Counter(
		"database_queries_total",
		metric.WithDescription("Total number of database queries"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database_queries_total metric: %w", err)
	}

	m.DBConnectionErrors, err = meter.Int64Counter(
		"database_connection_errors_total",
		metric.WithDescription("Total number of database connection errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create database_connection_errors_total metric: %w", err)
	}

	// Job metrics
	m.JobExecutions, err = meter.Int64Counter(
		"job_executions_total",
		metric.WithDescription("Total number of job executions"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create job_executions_total metric: %w", err)
	}

	m.JobDuration, err = meter.Float64Histogram(
		"job_duration_seconds",
		metric.WithDescription("Duration of job executions in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create job_duration_seconds metric: %w", err)
	}

	m.JobFailures, err = meter.Int64Counter(
		"job_failures_total",
		metric.WithDescription("Total number of job failures"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create job_failures_total metric: %w", err)
	}

	m.GraphAPIRequests, err = meter.Int64Counter(
		"graph_api_requests_total",
		metric.WithDescription("Total number of Microsoft Graph API requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph_api_requests_total metric: %w", err)
	}

	m.GraphAPIErrors, err = meter.Int64Counter(
		"graph_api_errors_total",
		metric.WithDescription("Total number of Microsoft Graph API errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create graph_api_errors_total metric: %w", err)
	}

	// Application metrics
	m.AppStartTime, err = meter.Float64Gauge(
		"app_start_time_seconds",
		metric.WithDescription("Unix timestamp when the application started"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create app_start_time_seconds metric: %w", err)
	}

	m.ConfigReloads, err = meter.Int64Counter(
		"config_reloads_total",
		metric.WithDescription("Total number of configuration reloads"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create config_reloads_total metric: %w", err)
	}

	m.ErrorCount, err = meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create errors_total metric: %w", err)
	}

	// Record application start time
	m.AppStartTime.Record(context.Background(), float64(time.Now().Unix()))

	logger.Info("Metrics initialized successfully")
	return m, nil
}

// RecordHTTPRequest records HTTP request metrics
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("method", method),
		attribute.String("path", path),
		attribute.Int("status_code", statusCode),
		attribute.String("status_class", fmt.Sprintf("%dxx", statusCode/100)),
	}

	m.HTTPRequestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.HTTPRequestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordHTTPActiveRequest records active HTTP request metrics
func (m *Metrics) RecordHTTPActiveRequest(ctx context.Context, delta int64) {
	m.HTTPActiveRequests.Add(ctx, delta)
}

// RecordAuthAttempt records authentication attempt metrics
func (m *Metrics) RecordAuthAttempt(ctx context.Context, success bool, provider string) {
	attrs := []attribute.KeyValue{
		attribute.String("provider", provider),
	}

	m.AuthAttempts.Add(ctx, 1, metric.WithAttributes(attrs...))
	if success {
		m.AuthSuccess.Add(ctx, 1, metric.WithAttributes(attrs...))
	} else {
		m.AuthFailures.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordSessionEvent records session lifecycle events
func (m *Metrics) RecordSessionEvent(ctx context.Context, event string) {
	attrs := []attribute.KeyValue{
		attribute.String("event", event),
	}

	switch event {
	case "created":
		m.SessionCreated.Add(ctx, 1, metric.WithAttributes(attrs...))
	case "destroyed":
		m.SessionDestroyed.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordDBQuery records database query metrics
func (m *Metrics) RecordDBQuery(ctx context.Context, operation string, success bool, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.Bool("success", success),
	}

	m.DBQueryDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.DBQueryCount.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordDBConnection records database connection metrics
func (m *Metrics) RecordDBConnection(ctx context.Context, delta int64) {
	m.DBConnections.Add(ctx, delta)
}

// RecordDBConnectionError records database connection errors
func (m *Metrics) RecordDBConnectionError(ctx context.Context) {
	m.DBConnectionErrors.Add(ctx, 1)
}

// RecordJobExecution records job execution metrics
func (m *Metrics) RecordJobExecution(ctx context.Context, jobName string, success bool, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String("job_name", jobName),
		attribute.Bool("success", success),
	}

	m.JobExecutions.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.JobDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))

	if !success {
		m.JobFailures.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordGraphAPIRequest records Microsoft Graph API request metrics
func (m *Metrics) RecordGraphAPIRequest(ctx context.Context, operation string, success bool) {
	attrs := []attribute.KeyValue{
		attribute.String("operation", operation),
		attribute.Bool("success", success),
	}

	m.GraphAPIRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
	if !success {
		m.GraphAPIErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordError records application errors
func (m *Metrics) RecordError(ctx context.Context, errorType string) {
	attrs := []attribute.KeyValue{
		attribute.String("error_type", errorType),
	}

	m.ErrorCount.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordConfigReload records configuration reload events
func (m *Metrics) RecordConfigReload(ctx context.Context) {
	m.ConfigReloads.Add(ctx, 1)
}
