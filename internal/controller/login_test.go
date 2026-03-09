package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/spf13/viper"
)

func TestLoginController_Login(t *testing.T) {
	// Set up test configuration
	viper.Set("azureAD.tenant", "test-tenant")
	viper.Set("azureAD.clientID", "test-client-id")
	viper.Set("azureAD.clientSecret", "test-client-secret")
	viper.Set("azureAD.redirectURL", "http://localhost:8080/oauth2/callback")

	// Create test authenticator
	auth, err := environment.NewAuthenticator()
	if err != nil {
		t.Skip("Skipping test due to authenticator creation error:", err)
	}

	// Create session manager
	sessionManager := scs.New()

	// Create application context
	app := environment.Application{
		SessionManager: sessionManager,
	}

	// Create login controller
	controller := NewLoginController(app, *auth)

	// Create test request
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()

	// Add session context
	ctx := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller.Login(w, r)
	}))

	// Execute request
	ctx.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusTemporaryRedirect {
		t.Errorf("Expected status code %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}

	// Check that Location header is set
	location := w.Header().Get("Location")
	if location == "" {
		t.Error("Expected Location header to be set")
	}

	// Check that location contains Microsoft login URL
	if !contains(location, "login.microsoftonline.com") {
		t.Error("Expected redirect to Microsoft login URL")
	}
}

func TestLoginController_Logout(t *testing.T) {
	// Create session manager
	sessionManager := scs.New()

	// Create application context
	app := environment.Application{
		SessionManager: sessionManager,
	}

	// Create login controller with mock auth
	controller := LoginController{
		Application: app,
	}

	// Create test request
	req := httptest.NewRequest("GET", "/logout", nil)
	w := httptest.NewRecorder()

	// Add session context
	ctx := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller.Logout(w, r)
	}))

	// Execute request
	ctx.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusTemporaryRedirect {
		t.Errorf("Expected status code %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}

	// Check redirect location
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("Expected redirect to '/', got '%s'", location)
	}
}

func TestGenerateRandomState(t *testing.T) {
	state1, err := generateRandomState()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	state2, err := generateRandomState()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// States should be different
	if state1 == state2 {
		t.Error("Expected different states, got same")
	}

	// State should not be empty
	if len(state1) == 0 {
		t.Error("Expected non-empty state")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && containsHelper(s[1:], substr)
}

func containsHelper(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[:len(substr)] == substr {
		return true
	}
	return containsHelper(s[1:], substr)
}
