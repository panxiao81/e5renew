package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// DatabaseTokenCredential implements azcore.TokenCredential interface
// It retrieves tokens from the database and handles automatic refresh
type DatabaseTokenCredential struct {
	userID           string
	userTokenService *UserTokenService
	logger           *slog.Logger
}

// NewDatabaseTokenCredential creates a new DatabaseTokenCredential
func NewDatabaseTokenCredential(userID string, userTokenService *UserTokenService, logger *slog.Logger) *DatabaseTokenCredential {
	return &DatabaseTokenCredential{
		userID:           userID,
		userTokenService: userTokenService,
		logger:           logger,
	}
}

// GetToken retrieves an access token from the database with automatic refresh
func (c *DatabaseTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "DatabaseTokenCredential.GetToken")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", c.userID),
		attribute.StringSlice("scopes", options.Scopes),
	)

	// Get token source with automatic refresh and database updates
	tokenSource, err := c.userTokenService.GetTokenSourceWithCallback(ctx, c.userID)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "failed_to_get_token_source"))
		return azcore.AccessToken{}, fmt.Errorf("failed to get token source for user %s: %w", c.userID, err)
	}

	// Get current valid token (may trigger refresh)
	token, err := tokenSource.Token()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "failed_to_get_token"))
		return azcore.AccessToken{}, fmt.Errorf("failed to get token for user %s: %w", c.userID, err)
	}

	// Convert oauth2.Token to azcore.AccessToken
	accessToken := azcore.AccessToken{
		Token:     token.AccessToken,
		ExpiresOn: token.Expiry,
	}

	span.SetAttributes(
		attribute.String("token_type", token.TokenType),
		attribute.String("token_expiry", token.Expiry.Format(time.RFC3339)),
		attribute.Bool("token_expired", token.Expiry.Before(time.Now())),
		attribute.Bool("has_refresh_token", token.RefreshToken != ""),
		attribute.Bool("success", true),
	)

	c.logger.Debug("Successfully retrieved access token via DatabaseTokenCredential",
		"userID", c.userID,
		"tokenExpiry", token.Expiry.Format(time.RFC3339))

	return accessToken, nil
}
