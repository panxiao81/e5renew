package environment

import (
	"testing"

	"github.com/spf13/viper"
)

func TestAzureADEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		expected string
	}{
		{
			name:     "WithTenant",
			tenant:   "test-tenant",
			expected: "https://login.microsoftonline.com/test-tenant/v2.0",
		},
		{
			name:     "EmptyTenant",
			tenant:   "",
			expected: "https://login.microsoftonline.com/common/v2.0",
		},
		{
			name:     "CommonTenant",
			tenant:   "common",
			expected: "https://login.microsoftonline.com/common/v2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := azureADEndpoint(tt.tenant)
			if result != tt.expected {
				t.Errorf("azureADEndpoint(%q) = %q, want %q", tt.tenant, result, tt.expected)
			}
		})
	}
}

func TestNewAuthenticator(t *testing.T) {
	// Set up test configuration
	viper.Set("azureAD.tenant", "test-tenant")
	viper.Set("azureAD.clientID", "test-client-id")
	viper.Set("azureAD.clientSecret", "test-client-secret")
	viper.Set("azureAD.redirectURL", "http://localhost:8080/oauth2/callback")

	// This test might fail in CI/CD without internet access
	// We'll skip it if we can't create the authenticator
	auth, err := NewAuthenticator()
	if err != nil {
		t.Skip("Skipping test due to authenticator creation error:", err)
	}

	// Check that authenticator is not nil
	if auth == nil {
		t.Error("Expected non-nil authenticator")
	}

	// Check that provider is set
	if auth.Provider == nil {
		t.Error("Expected provider to be set")
	}

	// Check that config is set
	if auth.Config.ClientID == "" {
		t.Error("Expected ClientID to be set")
	}

	if auth.Config.ClientSecret == "" {
		t.Error("Expected ClientSecret to be set")
	}

	if auth.Config.RedirectURL == "" {
		t.Error("Expected RedirectURL to be set")
	}

	// Check that scopes are set
	if len(auth.Config.Scopes) == 0 {
		t.Error("Expected scopes to be set")
	}
}

func TestAzureADClaims(t *testing.T) {
	// Test that AzureADClaims struct has expected fields
	claims := AzureADClaims{
		Name:              "Test User",
		PreferredUsername: "test@example.com",
		Email:             "test@example.com",
	}

	if claims.Name != "Test User" {
		t.Errorf("Expected Name to be 'Test User', got '%s'", claims.Name)
	}

	if claims.PreferredUsername != "test@example.com" {
		t.Errorf("Expected PreferredUsername to be 'test@example.com', got '%s'", claims.PreferredUsername)
	}

	if claims.Email != "test@example.com" {
		t.Errorf("Expected Email to be 'test@example.com', got '%s'", claims.Email)
	}
}

func TestDeriveUserTokenRedirectURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple path",
			in:   "http://localhost:8080/oauth2/callback",
			want: "http://localhost:8080/oauth2/callback-user-token",
		},
		{
			name: "with query preserved",
			in:   "https://example.com/oauth2/callback?x=1",
			want: "https://example.com/oauth2/callback-user-token?x=1",
		},
		{
			name: "invalid url fallback",
			in:   "://broken",
			want: "://broken-user-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveUserTokenRedirectURL(tt.in); got != tt.want {
				t.Fatalf("DeriveUserTokenRedirectURL(%q)=%q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestAzureADIssuerFromConfig(t *testing.T) {
	viper.Set("azureAD.issuer", "")
	viper.Set("azureAD.tenant", "tenant-a")
	if got := AzureADIssuerFromConfig(); got != "https://login.microsoftonline.com/tenant-a/v2.0" {
		t.Fatalf("AzureADIssuerFromConfig() with tenant got %q", got)
	}

	viper.Set("azureAD.issuer", "https://login.microsoftonline.com/custom/v2.0")
	if got := AzureADIssuerFromConfig(); got != "https://login.microsoftonline.com/custom/v2.0" {
		t.Fatalf("AzureADIssuerFromConfig() with explicit issuer got %q", got)
	}
}
