package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
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
	// Get token source with automatic refresh and database updates
	tokenSource, err := c.userTokenService.GetTokenSourceWithCallback(ctx, c.userID)
	if err != nil {
		return azcore.AccessToken{}, fmt.Errorf("failed to get token source for user %s: %w", c.userID, err)
	}

	// Get current valid token (may trigger refresh)
	token, err := tokenSource.Token()
	if err != nil {
		return azcore.AccessToken{}, fmt.Errorf("failed to get token for user %s: %w", c.userID, err)
	}

	// Convert oauth2.Token to azcore.AccessToken
	accessToken := azcore.AccessToken{
		Token:     token.AccessToken,
		ExpiresOn: token.Expiry,
	}

	c.logger.Debug("Successfully retrieved access token via DatabaseTokenCredential",
		"userID", c.userID,
		"tokenExpiry", token.Expiry.Format(time.RFC3339))

	return accessToken, nil
}
