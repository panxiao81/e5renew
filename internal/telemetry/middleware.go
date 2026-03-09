package telemetry

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// HTTPMiddleware creates a middleware for HTTP tracing and metrics
func HTTPMiddleware(serviceName string, metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Wrap with OpenTelemetry HTTP instrumentation
		otelhttpHandler := otelhttp.NewHandler(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				start := time.Now()

				// Add active request metric
				metrics.RecordHTTPActiveRequest(r.Context(), 1)
				defer metrics.RecordHTTPActiveRequest(r.Context(), -1)

				// Create a custom response writer to capture status code
				wrappedWriter := &responseWriter{
					ResponseWriter: w,
					statusCode:     200, // Default status code
				}

				// Add request attributes to span
				span := trace.SpanFromContext(r.Context())
				span.SetAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.scheme", r.URL.Scheme),
					attribute.String("http.host", r.Host),
					attribute.String("http.user_agent", r.UserAgent()),
					attribute.String("http.remote_addr", r.RemoteAddr),
				)

				// Call the next handler
				next.ServeHTTP(wrappedWriter, r)

				// Record metrics
				duration := time.Since(start)
				metrics.RecordHTTPRequest(
					r.Context(),
					r.Method,
					r.URL.Path,
					wrappedWriter.statusCode,
					duration,
				)

				// Add response attributes to span
				span.SetAttributes(
					attribute.Int("http.status_code", wrappedWriter.statusCode),
					attribute.String("http.status_text", http.StatusText(wrappedWriter.statusCode)),
					attribute.Int64("http.response_size", wrappedWriter.bytesWritten),
				)

				// Set span status based on HTTP status code
				if wrappedWriter.statusCode >= 400 {
					span.RecordError(nil)
					if wrappedWriter.statusCode >= 500 {
						metrics.RecordError(r.Context(), "http_server_error")
					} else {
						metrics.RecordError(r.Context(), "http_client_error")
					}
				}
			}),
			serviceName,
		)

		return otelhttpHandler
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int64
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

// StartSpan starts a new span with the given name and attributes
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ctx, span := tracer.Start(ctx, name)
	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return ctx, span
}

// RecordError records an error in the current span and metrics
func RecordError(ctx context.Context, metrics *Metrics, err error, errorType string) {
	// Record error in span
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)

	// Record error in metrics
	metrics.RecordError(ctx, errorType)
}

// AddSpanEvent adds an event to the current span
func AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetSpanAttributes sets attributes on the current span
func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}
