package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
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
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	_, span := tracer.Start(context.Background(), "DatabaseUpdatingTokenSource.Token")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", d.userID),
		attribute.Bool("has_last_token", d.lastToken != nil),
	)

	// Get token from underlying TokenSource (may trigger refresh)
	token, err := d.TokenSource.Token()
	if err != nil {
		span.RecordError(err)
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

	span.SetAttributes(
		attribute.Bool("token_refreshed", tokenRefreshed),
		attribute.String("token_type", token.TokenType),
		attribute.String("token_expiry", token.Expiry.Format(time.RFC3339)),
		attribute.Bool("token_expired", token.Expiry.Before(time.Now())),
	)

	// If token was refreshed, update the database
	if tokenRefreshed {
		updateCtx := context.Background()
		if err := d.service.UpdateUserToken(updateCtx, d.userID, token); err != nil {
			// Log error but don't fail the token request
			d.logger.Error("Failed to update refreshed token in database",
				"userID", d.userID,
				"error", err)
			span.RecordError(err)
			span.SetAttributes(attribute.Bool("database_update_failed", true))
		} else {
			d.logger.Debug("Updated refreshed token in database", "userID", d.userID)
			span.SetAttributes(attribute.Bool("database_updated", true))
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
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetTokenSourceWithCallback")
	defer span.End()

	span.SetAttributes(attribute.String("user_id", userID))

	// Get the stored token from database
	storedToken, err := s.GetUserToken(ctx, userID)
	if err != nil {
		span.RecordError(err)
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
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetTokenSourceWithoutCallback")
	defer span.End()

	span.SetAttributes(attribute.String("user_id", userID))

	// Get the stored token from database
	storedToken, err := s.GetUserToken(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get stored token for user %s: %w", userID, err)
	}

	// Create and return base TokenSource
	return s.oauth2Config.TokenSource(ctx, storedToken), nil
}
