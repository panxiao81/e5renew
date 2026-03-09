package utils

import (
	"net/mail"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// ValidationResult represents the result of input validation
type ValidationResult struct {
	Valid  bool
	Errors []string
}

// Validator provides input validation utilities
type Validator struct{}

// NewValidator creates a new Validator instance
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateEmail validates email address format
func (v *Validator) ValidateEmail(email string) ValidationResult {
	errors := []string{}
	
	if email == "" {
		errors = append(errors, "email is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	if len(email) > 254 {
		errors = append(errors, "email is too long")
	}
	
	_, err := mail.ParseAddress(email)
	if err != nil {
		errors = append(errors, "invalid email format")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// ValidateUserID validates user ID format (typically email for this app)
func (v *Validator) ValidateUserID(userID string) ValidationResult {
	return v.ValidateEmail(userID)
}

// ValidateState validates OAuth state parameter
func (v *Validator) ValidateState(state string) ValidationResult {
	errors := []string{}
	
	if state == "" {
		errors = append(errors, "state parameter is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	if len(state) < 16 || len(state) > 128 {
		errors = append(errors, "state parameter has invalid length")
	}
	
	// State should be base64 encoded random data
	validStateRegex := regexp.MustCompile(`^[A-Za-z0-9+/]+=*$`)
	if !validStateRegex.MatchString(state) {
		errors = append(errors, "state parameter has invalid format")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// ValidateAuthCode validates OAuth authorization code
func (v *Validator) ValidateAuthCode(code string) ValidationResult {
	errors := []string{}
	
	if code == "" {
		errors = append(errors, "authorization code is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	if len(code) < 10 || len(code) > 1024 {
		errors = append(errors, "authorization code has invalid length")
	}
	
	// Basic sanity check - should contain alphanumeric characters and common symbols
	if !regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(code) {
		errors = append(errors, "authorization code has invalid format")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// ValidateString validates a general string input
func (v *Validator) ValidateString(input string, fieldName string, minLength, maxLength int) ValidationResult {
	errors := []string{}
	
	if input == "" && minLength > 0 {
		errors = append(errors, fieldName+" is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	if len(input) < minLength {
		errors = append(errors, fieldName+" is too short")
	}
	
	if len(input) > maxLength {
		errors = append(errors, fieldName+" is too long")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// SanitizeString removes potentially dangerous characters from input
func (v *Validator) SanitizeString(input string) string {
	// Remove control characters and normalize whitespace
	result := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1 // Remove control characters
		}
		return r
	}, input)
	
	// Normalize whitespace
	result = strings.TrimSpace(result)
	
	return result
}

// ValidateHTTPMethod validates HTTP method
func (v *Validator) ValidateHTTPMethod(method string) ValidationResult {
	errors := []string{}
	
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	
	if method == "" {
		errors = append(errors, "HTTP method is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	method = strings.ToUpper(method)
	
	if !slices.Contains(validMethods, method) {
		errors = append(errors, "invalid HTTP method")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// ValidateURL validates URL format (basic validation)
func (v *Validator) ValidateURL(url string) ValidationResult {
	errors := []string{}
	
	if url == "" {
		errors = append(errors, "URL is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	if len(url) > 2048 {
		errors = append(errors, "URL is too long")
	}
	
	// Basic URL pattern validation
	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !urlRegex.MatchString(url) {
		errors = append(errors, "invalid URL format")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// ValidateRequestPath validates HTTP request path
func (v *Validator) ValidateRequestPath(path string) ValidationResult {
	errors := []string{}
	
	if path == "" {
		errors = append(errors, "request path is required")
		return ValidationResult{Valid: false, Errors: errors}
	}
	
	if len(path) > 2048 {
		errors = append(errors, "request path is too long")
	}
	
	// Path should start with /
	if !strings.HasPrefix(path, "/") {
		errors = append(errors, "request path must start with /")
	}
	
	// Basic path validation - allow alphanumeric, /, -, _, ., and query parameters
	pathRegex := regexp.MustCompile(`^[A-Za-z0-9/_.-]+(\?[A-Za-z0-9=&_.-]*)?$`)
	if !pathRegex.MatchString(path) {
		errors = append(errors, "request path contains invalid characters")
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// ValidateJSONInput validates JSON input for XSS and injection attempts
func (v *Validator) ValidateJSONInput(input string) ValidationResult {
	errors := []string{}
	
	if input == "" {
		return ValidationResult{Valid: true, Errors: errors}
	}
	
	// Check for common XSS patterns
	xssPatterns := []string{
		"<script",
		"javascript:",
		"onload=",
		"onerror=",
		"onclick=",
		"onmouseover=",
		"<iframe",
		"<object",
		"<embed",
	}
	
	lowerInput := strings.ToLower(input)
	for _, pattern := range xssPatterns {
		if strings.Contains(lowerInput, pattern) {
			errors = append(errors, "input contains potentially dangerous content")
			break
		}
	}
	
	return ValidationResult{Valid: len(errors) == 0, Errors: errors}
}

// CombineValidationResults combines multiple validation results
func CombineValidationResults(results ...ValidationResult) ValidationResult {
	allErrors := []string{}
	valid := true
	
	for _, result := range results {
		if !result.Valid {
			valid = false
		}
		allErrors = append(allErrors, result.Errors...)
	}
	
	return ValidationResult{Valid: valid, Errors: allErrors}
}