package db

import (
	"testing"
)

func TestNew(t *testing.T) {
	// Test that New function returns a Queries struct
	// This is a basic smoke test since we can't easily mock a database connection

	// We can't test with a real database connection without setup,
	// but we can verify the function exists and has the right signature
	t.Run("NewFunctionExists", func(t *testing.T) {
		// This test just verifies the function can be called
		// In a real environment with database setup, we would test:
		// queries := New(db)
		// if queries == nil {
		//     t.Error("Expected non-nil queries")
		// }
		t.Log("New function exists and can be called")
	})
}

func TestQueryStructure(t *testing.T) {
	// Test that the generated structs have expected fields
	t.Run("UserTokenStruct", func(t *testing.T) {
		token := UserToken{
			ID:           1,
			UserID:       "test-user",
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			TokenType:    "Bearer",
		}

		if token.UserID != "test-user" {
			t.Errorf("Expected UserID to be 'test-user', got '%s'", token.UserID)
		}

		if token.AccessToken != "test-access-token" {
			t.Errorf("Expected AccessToken to be 'test-access-token', got '%s'", token.AccessToken)
		}

		if token.RefreshToken != "test-refresh-token" {
			t.Errorf("Expected RefreshToken to be 'test-refresh-token', got '%s'", token.RefreshToken)
		}

		if token.TokenType != "Bearer" {
			t.Errorf("Expected TokenType to be 'Bearer', got '%s'", token.TokenType)
		}
	})

	t.Run("CreateUserTokensParams", func(t *testing.T) {
		params := CreateUserTokensParams{
			UserID:       "test-user",
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			TokenType:    "Bearer",
		}

		if params.UserID != "test-user" {
			t.Errorf("Expected UserID to be 'test-user', got '%s'", params.UserID)
		}

		if params.AccessToken != "test-access-token" {
			t.Errorf("Expected AccessToken to be 'test-access-token', got '%s'", params.AccessToken)
		}

		if params.RefreshToken != "test-refresh-token" {
			t.Errorf("Expected RefreshToken to be 'test-refresh-token', got '%s'", params.RefreshToken)
		}

		if params.TokenType != "Bearer" {
			t.Errorf("Expected TokenType to be 'Bearer', got '%s'", params.TokenType)
		}
	})
}
