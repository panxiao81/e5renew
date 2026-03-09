package utils

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestErrorHandler() *ErrorHandler {
	return NewErrorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestHandleErrorNilErrorDoesNotPanic(t *testing.T) {
	handler := newTestErrorHandler()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.HandleError(rec, req, nil, http.StatusBadRequest, "Invalid request parameters")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	if body := rec.Body.String(); body == "" {
		t.Fatal("expected non-empty response body")
	}
}

func TestHandleJSONErrorNilErrorDoesNotPanic(t *testing.T) {
	handler := newTestErrorHandler()
	req := httptest.NewRequest(http.MethodGet, "/test-json", nil)
	rec := httptest.NewRecorder()

	handler.HandleJSONError(rec, req, nil, http.StatusBadRequest, "Invalid request parameters", "invalid_input")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var response ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if response.Error == "" {
		t.Fatal("expected error message in JSON response")
	}
	if response.Code != "invalid_input" {
		t.Fatalf("expected code %q, got %q", "invalid_input", response.Code)
	}
}
