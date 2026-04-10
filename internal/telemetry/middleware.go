package telemetry

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

				if wrappedWriter.statusCode >= 400 {
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
