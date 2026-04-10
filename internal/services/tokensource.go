package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/panxiao81/e5renew/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"
)

// DatabaseUpdatingTokenSource wraps oauth2.TokenSource to automatically
// update the database when tokens are refreshed
type DatabaseUpdatingTokenSource struct {
	oauth2.TokenSource
	userID    string
	service   *UserTokenService
	lastToken *oauth2.Token
	logger    *slog.Logger
}

// NewDatabaseUpdatingTokenSource creates a new DatabaseUpdatingTokenSource
func NewDatabaseUpdatingTokenSource(tokenSource oauth2.TokenSource, userID string, service *UserTokenService, initialToken *oauth2.Token, logger *slog.Logger) *DatabaseUpdatingTokenSource {
	return &DatabaseUpdatingTokenSource{
		TokenSource: tokenSource,
		userID:      userID,
		service:     service,
		lastToken:   initialToken,
		logger:      logger,
	}
}

// Token returns a token, refreshing if necessary and updating the database
func (d *DatabaseUpdatingTokenSource) Token() (*oauth2.Token, error) {
	_, span := telemetry.StartSpan(context.Background(), "github.com/panxiao81/e5renew/services", "DatabaseUpdatingTokenSource.Token", attribute.String("user_id", d.userID))
	defer span.End()

	// Get token from underlying TokenSource (may trigger refresh)
	token, err := d.TokenSource.Token()
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to get token from underlying source: %w", err)
	}

	// Check if token was refreshed by comparing with last known token
	tokenRefreshed := false
	if d.lastToken == nil {
		// First time getting token
		tokenRefreshed = true
	} else {
		// Check if access token changed or expiry changed
		if d.lastToken.AccessToken != token.AccessToken ||
			!d.lastToken.Expiry.Equal(token.Expiry) {
			tokenRefreshed = true
		}
	}

	// If token was refreshed, update the database
	if tokenRefreshed {
		updateCtx := context.Background()
		if err := d.service.UpdateUserToken(updateCtx, d.userID, token); err != nil {
			telemetry.RecordSpanError(span, err)
			// Log error but don't fail the token request
			d.logger.Error("Failed to update refreshed token in database",
				"userID", d.userID,
				"error", err)
		} else {
			d.logger.Debug("Updated refreshed token in database", "userID", d.userID)
		}

		// Update our last known token
		d.lastToken = &oauth2.Token{
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Expiry:       token.Expiry,
			TokenType:    token.TokenType,
		}
	}

	return token, nil
}

// GetTokenSourceWithCallback creates a TokenSource that automatically updates the database on refresh
func (s *UserTokenService) GetTokenSourceWithCallback(ctx context.Context, userID string) (oauth2.TokenSource, error) {
	ctx, span := telemetry.StartSpan(ctx, "github.com/panxiao81/e5renew/services", "GetTokenSourceWithCallback", attribute.String("user_id", userID))
	defer span.End()

	// Get the stored token from database
	storedToken, err := s.GetUserToken(ctx, userID)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to get stored token for user %s: %w", userID, err)
	}

	// Create base TokenSource from OAuth2 config
	baseTokenSource := s.oauth2Config.TokenSource(ctx, storedToken)

	// Wrap with database updating functionality
	return NewDatabaseUpdatingTokenSource(
		baseTokenSource,
		userID,
		s,
		storedToken,
		s.logger,
	), nil
}

// GetTokenSourceWithoutCallback creates a regular TokenSource without database updates
// This is useful for one-off operations where you don't want to update the database
func (s *UserTokenService) GetTokenSourceWithoutCallback(ctx context.Context, userID string) (oauth2.TokenSource, error) {
	ctx, span := telemetry.StartSpan(ctx, "github.com/panxiao81/e5renew/services", "GetTokenSourceWithoutCallback", attribute.String("user_id", userID))
	defer span.End()

	// Get the stored token from database
	storedToken, err := s.GetUserToken(ctx, userID)
	if err != nil {
		telemetry.RecordSpanError(span, err)
		return nil, fmt.Errorf("failed to get stored token for user %s: %w", userID, err)
	}

	// Create and return base TokenSource
	return s.oauth2Config.TokenSource(ctx, storedToken), nil
}
