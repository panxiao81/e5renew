package utils

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorAdditionalBranches(t *testing.T) {
	v := NewValidator()

	require.False(t, v.ValidateEmail("").Valid)
	require.False(t, v.ValidateEmail(strings.Repeat("a", 300)+"@example.com").Valid)
	require.True(t, v.ValidateUserID("user@example.com").Valid)

	require.False(t, v.ValidateString("", "name", 1, 5).Valid)
	require.False(t, v.ValidateString("a", "name", 2, 5).Valid)
	require.False(t, v.ValidateString("abcdef", "name", 0, 5).Valid)
	require.True(t, v.ValidateString("abcd", "name", 0, 5).Valid)

	require.Equal(t, "a\tb\nc", v.SanitizeString("\x00 a\tb\nc \x1f"))

	require.False(t, v.ValidateHTTPMethod("TRACE").Valid)
	require.True(t, v.ValidateHTTPMethod("post").Valid)

	require.False(t, v.ValidateURL("").Valid)
	require.False(t, v.ValidateURL("ftp://example.com").Valid)
	require.True(t, v.ValidateURL("https://example.com/path?q=1").Valid)

	require.False(t, v.ValidateRequestPath("").Valid)
	require.False(t, v.ValidateRequestPath("relative/path").Valid)
	require.False(t, v.ValidateRequestPath("/bad path").Valid)
	require.True(t, v.ValidateRequestPath("/ok/path?q=1").Valid)

	require.True(t, v.ValidateJSONInput("").Valid)
	require.False(t, v.ValidateJSONInput("<script>alert(1)</script>").Valid)
	require.True(t, v.ValidateJSONInput(`{"ok":true}`).Valid)

	combined := CombineValidationResults(
		ValidationResult{Valid: true},
		ValidationResult{Valid: false, Errors: []string{"e1"}},
		ValidationResult{Valid: false, Errors: []string{"e2"}},
	)
	require.False(t, combined.Valid)
	require.Equal(t, []string{"e1", "e2"}, combined.Errors)
}

func TestErrorHelpersAdditionalBranches(t *testing.T) {
	require.Equal(t, "", SanitizeError(nil))
	require.Equal(t, "An internal error occurred", SanitizeError(errors.New("database password mismatch")))
	require.Equal(t, "An internal error occurred", SanitizeError(errors.New("/tmp/app.go:42 boom")))
	require.Equal(t, "plain failure", SanitizeError(errors.New("plain failure")))

	require.Equal(t, "", GetSafeErrorMessage(nil))
	require.Equal(t, "The requested resource was not found", GetSafeErrorMessage(errors.New("record not found")))
	require.Equal(t, "You are not authorized to access this resource", GetSafeErrorMessage(errors.New("unauthorized request")))
	require.Equal(t, "Access to this resource is forbidden", GetSafeErrorMessage(errors.New("forbidden")))
	require.Equal(t, "The request timed out. Please try again", GetSafeErrorMessage(errors.New("timeout while reading")))
	require.Equal(t, "A connection error occurred. Please try again", GetSafeErrorMessage(errors.New("connection reset")))
	require.Equal(t, "Invalid input provided", GetSafeErrorMessage(errors.New("invalid state")))
	require.Equal(t, "A duplicate entry was found", GetSafeErrorMessage(errors.New("duplicate key")))
	require.Equal(t, "An internal server error occurred", GetSafeErrorMessage(errors.New("other")))

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	require.Equal(t, "An internal server error occurred", LogAndReturnError(logger, errors.New("boom"), map[string]any{"k": "v"}))
}

func TestErrorHandlerJSONAndPlain(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewErrorHandler(logger)
	r := httptest.NewRequest("GET", "/x", nil)

	w := httptest.NewRecorder()
	h.HandleError(w, r, errors.New("boom"), 418, "teapot")
	require.Equal(t, 418, w.Code)
	require.Contains(t, w.Body.String(), "teapot")

	w2 := httptest.NewRecorder()
	h.HandleJSONError(w2, r, errors.New("boom"), 400, "bad", "E_BAD")
	require.Equal(t, "application/json", w2.Header().Get("Content-Type"))
	require.Equal(t, 400, w2.Code)
	require.Contains(t, w2.Body.String(), `"error":"bad"`)
	require.Contains(t, w2.Body.String(), `"code":"E_BAD"`)
}
