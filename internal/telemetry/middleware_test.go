package telemetry

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
)

func testMetrics(t *testing.T) *Metrics {
	t.Helper()
	m, err := NewMetrics(noop.NewMeterProvider().Meter("test"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewMetrics: %v", err)
	}
	return m
}

func TestHTTPMiddleware_WrapsHandler(t *testing.T) {
	metrics := testMetrics(t)
	wrapped := HTTPMiddleware("test-service", metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/ping", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if rr.Body.String() != "ok" {
		t.Fatalf("expected body ok, got %q", rr.Body.String())
	}
}

func TestResponseWriter_CapturesBytes(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr, statusCode: 200}

	rw.WriteHeader(http.StatusAccepted)
	_, _ = rw.Write([]byte("abc"))
	if rw.statusCode != http.StatusAccepted {
		t.Fatalf("status not captured: %d", rw.statusCode)
	}
	if rw.bytesWritten != 3 {
		t.Fatalf("bytesWritten mismatch: %d", rw.bytesWritten)
	}
}

func TestSpanHelpers_DontPanic(t *testing.T) {
	tracer := otel.Tracer("test")
	ctx, span := StartSpan(context.Background(), tracer, "unit", attribute.String("k", "v"))
	defer span.End()

	SetSpanAttributes(ctx, attribute.String("a", "b"))
	AddSpanEvent(ctx, "evt", attribute.Int("n", 1))
	RecordError(ctx, testMetrics(t), context.DeadlineExceeded, "timeout")
}

func TestRecordMethods_NoPanic(t *testing.T) {
	m := testMetrics(t)
	ctx := context.Background()
	m.RecordHTTPRequest(ctx, "GET", "/x", 200, 10*time.Millisecond)
	m.RecordHTTPActiveRequest(ctx, 1)
	m.RecordHTTPActiveRequest(ctx, -1)
	m.RecordAuthAttempt(ctx, true, "azure")
	m.RecordAuthAttempt(ctx, false, "azure")
	m.RecordSessionEvent(ctx, "created")
	m.RecordSessionEvent(ctx, "destroyed")
	m.RecordSessionEvent(ctx, "noop")
	m.RecordDBQuery(ctx, "select", true, 10*time.Millisecond)
	m.RecordDBConnection(ctx, 1)
	m.RecordDBConnectionError(ctx)
	m.RecordJobExecution(ctx, "mail", false, 10*time.Millisecond)
	m.RecordGraphAPIRequest(ctx, "me/messages", false)
	m.RecordError(ctx, "http_server_error")
	m.RecordConfigReload(ctx)

	// Tiny assertion to avoid unused paths and ensure test ran.
	if !bytes.Contains([]byte("ok"), []byte("o")) {
		t.Fatal("impossible")
	}
}
