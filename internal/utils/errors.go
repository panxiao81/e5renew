package utils

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// ErrorHandler provides safe error handling for HTTP responses
type ErrorHandler struct {
	logger *slog.Logger
}

// NewErrorHandler creates a new ErrorHandler instance
func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// HandleError safely handles errors by logging the full error and returning a sanitized response
func (h *ErrorHandler) HandleError(w http.ResponseWriter, r *http.Request, err error, statusCode int, userMessage string) {
	errorMessage := "<nil>"
	if err != nil {
		errorMessage = err.Error()
	}

	// Log the full error with context
	h.logger.Error("HTTP error",
		"error", errorMessage,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"status_code", statusCode,
	)

	// Return sanitized error to user
	http.Error(w, userMessage, statusCode)
}

// HandleJSONError safely handles errors by logging the full error and returning a sanitized JSON response
func (h *ErrorHandler) HandleJSONError(w http.ResponseWriter, r *http.Request, err error, statusCode int, userMessage string, code string) {
	errorMessage := "<nil>"
	if err != nil {
		errorMessage = err.Error()
	}

	// Log the full error with context
	h.logger.Error("HTTP JSON error",
		"error", errorMessage,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"status_code", statusCode,
		"code", code,
	)

	// Return sanitized JSON error to user
	errorResponse := ErrorResponse{
		Error:   userMessage,
		Code:    code,
		Message: userMessage,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}

// SanitizeError removes sensitive information from error messages
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Remove common sensitive patterns
	sensitivePatterns := []string{
		"password",
		"secret",
		"token",
		"key",
		"credential",
		"auth",
		"login",
		"database",
		"sql",
		"connection",
		"dsn",
	}

	// Convert to lowercase for case-insensitive matching
	lowerErr := strings.ToLower(errStr)

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerErr, pattern) {
			return "An internal error occurred"
		}
	}

	// Remove file paths and line numbers
	if strings.Contains(errStr, "/") && strings.Contains(errStr, ":") {
		return "An internal error occurred"
	}

	return errStr
}

// GetSafeErrorMessage returns a user-friendly error message based on the error type
func GetSafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "not found"):
		return "The requested resource was not found"
	case strings.Contains(errStr, "unauthorized"):
		return "You are not authorized to access this resource"
	case strings.Contains(errStr, "forbidden"):
		return "Access to this resource is forbidden"
	case strings.Contains(errStr, "timeout"):
		return "The request timed out. Please try again"
	case strings.Contains(errStr, "connection"):
		return "A connection error occurred. Please try again"
	case strings.Contains(errStr, "invalid"):
		return "Invalid input provided"
	case strings.Contains(errStr, "duplicate"):
		return "A duplicate entry was found"
	default:
		return "An internal server error occurred"
	}
}

// LogAndReturnError logs the error and returns a safe error message
func LogAndReturnError(logger *slog.Logger, err error, context map[string]any) string {
	errorMessage := "<nil>"
	if err != nil {
		errorMessage = err.Error()
	}

	// Create log fields
	logFields := []any{
		"error", errorMessage,
	}

	for key, value := range context {
		logFields = append(logFields, key, value)
	}

	logger.Error("Application error", logFields...)

	return GetSafeErrorMessage(err)
}
