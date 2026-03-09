package middleware

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// APILogEntry represents a logged API call
type APILogEntry struct {
	UserID         *string   `json:"user_id,omitempty"`
	APIEndpoint    string    `json:"api_endpoint"`
	HTTPMethod     string    `json:"http_method"`
	HTTPStatusCode int       `json:"http_status_code"`
	RequestTime    time.Time `json:"request_time"`
	ResponseTime   time.Time `json:"response_time"`
	DurationMs     int       `json:"duration_ms"`
	RequestSize    int       `json:"request_size"`
	ResponseSize   int       `json:"response_size"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	JobType        string    `json:"job_type"`
	Success        bool      `json:"success"`
}

// APILogger interface for logging API calls
type APILogger interface {
	LogAPICall(ctx context.Context, entry APILogEntry) error
}

// APILoggerConfig holds configuration for API logging
type APILoggerConfig struct {
	BaseURL       string
	APILogService APILogger
	Logger        *slog.Logger
	UserID        *string // Optional user ID for user-specific calls
	JobType       string  // Type of job making the API call
}

// APILoggerTransport wraps an http.RoundTripper to log API calls
type APILoggerTransport struct {
	Transport http.RoundTripper
	Config    APILoggerConfig
}

// NewAPILoggerTransport creates a new API logger transport
func NewAPILoggerTransport(transport http.RoundTripper, config APILoggerConfig) *APILoggerTransport {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &APILoggerTransport{
		Transport: transport,
		Config:    config,
	}
}

// RoundTrip implements the http.RoundTripper interface
func (t *APILoggerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/middleware")
	_, span := tracer.Start(req.Context(), "APILoggerTransport.RoundTrip")
	defer span.End()

	// Only log Graph API calls
	if !strings.Contains(req.URL.Host, "graph.microsoft.com") {
		return t.Transport.RoundTrip(req)
	}

	startTime := time.Now()

	// Extract endpoint from URL
	endpoint := t.extractEndpoint(req.URL.Path)

	// Get request size
	requestSize := 0
	if req.Body != nil {
		if req.ContentLength > 0 {
			requestSize = int(req.ContentLength)
		}
	}

	span.SetAttributes(
		attribute.String("api_endpoint", endpoint),
		attribute.String("http_method", req.Method),
		attribute.String("job_type", t.Config.JobType),
		attribute.Int("request_size", requestSize),
	)

	if t.Config.UserID != nil {
		span.SetAttributes(attribute.String("user_id", *t.Config.UserID))
	}

	// Make the request
	resp, err := t.Transport.RoundTrip(req)
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Prepare log entry
	logEntry := APILogEntry{
		UserID:       t.Config.UserID,
		APIEndpoint:  endpoint,
		HTTPMethod:   req.Method,
		RequestTime:  startTime,
		ResponseTime: endTime,
		DurationMs:   int(duration.Milliseconds()),
		RequestSize:  requestSize,
		JobType:      t.Config.JobType,
	}

	if err != nil {
		// Handle transport error
		logEntry.HTTPStatusCode = 0
		logEntry.Success = false
		errorMsg := err.Error()
		logEntry.ErrorMessage = &errorMsg
		logEntry.ResponseSize = 0

		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("success", false),
			attribute.String("error", errorMsg),
		)
	} else {
		// Handle successful response
		logEntry.HTTPStatusCode = resp.StatusCode
		logEntry.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
		logEntry.ResponseSize = int(resp.ContentLength)

		// If response length is unknown, try to read it
		if resp.ContentLength == -1 {
			if body, err := io.ReadAll(resp.Body); err == nil {
				logEntry.ResponseSize = len(body)
				// Create a new reader from the body
				resp.Body = io.NopCloser(strings.NewReader(string(body)))
			}
		}

		// Set error message for non-successful responses
		if !logEntry.Success {
			errorMsg := fmt.Sprintf("HTTP %d %s", resp.StatusCode, resp.Status)
			logEntry.ErrorMessage = &errorMsg
		}

		span.SetAttributes(
			attribute.Int("http_status_code", resp.StatusCode),
			attribute.Bool("success", logEntry.Success),
			attribute.Int("response_size", logEntry.ResponseSize),
		)
	}

	span.SetAttributes(
		attribute.Int("duration_ms", logEntry.DurationMs),
	)

	// Log the API call asynchronously
	go func() {
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := t.Config.APILogService.LogAPICall(logCtx, logEntry); err != nil {
			t.Config.Logger.Error("Failed to log API call", "error", err)
		}
	}()

	return resp, err
}

// extractEndpoint extracts a clean endpoint name from the URL path
func (t *APILoggerTransport) extractEndpoint(path string) string {
	// Remove leading slash and version
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "v1.0/")

	// Handle common patterns
	if path == "" {
		return "root"
	}

	// Remove query parameters if any
	if idx := strings.Index(path, "?"); idx != -1 {
		path = path[:idx]
	}

	// Simplify common patterns
	switch {
	case strings.HasPrefix(path, "me/"):
		return "me/" + strings.Split(path[3:], "/")[0]
	case strings.HasPrefix(path, "users/"):
		parts := strings.Split(path, "/")
		if len(parts) > 2 {
			return "users/" + parts[2]
		}
		return "users"
	case strings.HasPrefix(path, "groups/"):
		parts := strings.Split(path, "/")
		if len(parts) > 2 {
			return "groups/" + parts[2]
		}
		return "groups"
	default:
		// Take the first segment
		parts := strings.Split(path, "/")
		return parts[0]
	}
}

// NewAPILoggerClient creates an HTTP client with API logging
func NewAPILoggerClient(config APILoggerConfig) *http.Client {
	transport := NewAPILoggerTransport(http.DefaultTransport, config)
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}
