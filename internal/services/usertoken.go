package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/repository"
)

// UserTokenService handles user OAuth2 token management
type UserTokenService struct {
	repo         repository.UserTokenRepository
	oauth2Config *oauth2.Config
	logger       *slog.Logger
	encryption   *EncryptionService
}

// NewUserTokenService creates a new UserTokenService instance
func NewUserTokenService(repo repository.UserTokenRepository, config *oauth2.Config, logger *slog.Logger, encryption *EncryptionService) *UserTokenService {
	return &UserTokenService{
		repo:         repo,
		oauth2Config: config,
		logger:       logger,
		encryption:   encryption,
	}
}

// SaveUserToken saves or updates a user's OAuth2 token in the database
func (s *UserTokenService) SaveUserToken(ctx context.Context, userID string, token *oauth2.Token) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "SaveUserToken")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("token_type", token.TokenType),
		attribute.Bool("has_refresh_token", token.RefreshToken != ""),
	)

	// Handle ExpiresIn to Expiry conversion
	if token.Expiry.IsZero() && token.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	span.SetAttributes(
		attribute.String("token_expiry", token.Expiry.Format(time.RFC3339)),
	)

	// Encrypt sensitive token data
	encryptedAccessToken, err := s.encryption.Encrypt(token.AccessToken)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := s.encryption.Encrypt(token.RefreshToken)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	// Check if token already exists
	_, err = s.repo.GetByUserID(ctx, userID)
	if err != nil {
		// Token doesn't exist, create new one
		err = s.repo.Create(ctx, repository.UserTokenRecord{
			UserID:       userID,
			AccessToken:  encryptedAccessToken,
			RefreshToken: encryptedRefreshToken,
			Expiry:       token.Expiry,
			TokenType:    token.TokenType,
		})
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to create user token: %w", err)
		}
		s.logger.Info("Created new user token", "userID", userID)
	} else {
		// Token exists, update it
		err = s.repo.Update(ctx, repository.UserTokenRecord{
			AccessToken:  encryptedAccessToken,
			RefreshToken: encryptedRefreshToken,
			Expiry:       token.Expiry,
			TokenType:    token.TokenType,
			UserID:       userID,
		})
		if err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to update user token: %w", err)
		}
		s.logger.Info("Updated existing user token", "userID", userID)
	}

	span.SetAttributes(attribute.Bool("token_saved", true))
	return nil
}

// GetUserToken retrieves a user's OAuth2 token from the database
func (s *UserTokenService) GetUserToken(ctx context.Context, userID string) (*oauth2.Token, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetUserToken")
	defer span.End()

	span.SetAttributes(attribute.String("user_id", userID))

	storedToken, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to get user token: %w", err)
	}

	// Decrypt the token data
	accessToken, err := s.encryption.Decrypt(storedToken.AccessToken)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	refreshToken, err := s.encryption.Decrypt(storedToken.RefreshToken)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       storedToken.Expiry,
		TokenType:    storedToken.TokenType,
	}

	span.SetAttributes(
		attribute.String("token_type", token.TokenType),
		attribute.String("token_expiry", token.Expiry.Format(time.RFC3339)),
		attribute.Bool("is_expired", token.Expiry.Before(time.Now())),
		attribute.Bool("has_refresh_token", token.RefreshToken != ""),
	)

	return token, nil
}

// UpdateUserToken updates a user's OAuth2 token in the database
func (s *UserTokenService) UpdateUserToken(ctx context.Context, userID string, token *oauth2.Token) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "UpdateUserToken")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("token_type", token.TokenType),
		attribute.String("token_expiry", token.Expiry.Format(time.RFC3339)),
	)

	// Encrypt sensitive token data
	encryptedAccessToken, err := s.encryption.Encrypt(token.AccessToken)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := s.encryption.Encrypt(token.RefreshToken)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to encrypt refresh token: %w", err)
	}

	err = s.repo.Update(ctx, repository.UserTokenRecord{
		AccessToken:  encryptedAccessToken,
		RefreshToken: encryptedRefreshToken,
		Expiry:       token.Expiry,
		TokenType:    token.TokenType,
		UserID:       userID,
	})
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to update user token: %w", err)
	}

	s.logger.Debug("Updated user token after refresh", "userID", userID)
	return nil
}

// GetAllUserIDs retrieves all user IDs that have stored tokens
func (s *UserTokenService) GetAllUserIDs(ctx context.Context) ([]string, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "GetAllUserIDs")
	defer span.End()

	tokens, err := s.repo.List(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("failed to list user tokens: %w", err)
	}

	userIDs := make([]string, len(tokens))
	for i, token := range tokens {
		userIDs[i] = token.UserID
	}

	span.SetAttributes(attribute.Int("user_count", len(userIDs)))
	return userIDs, nil
}

// DeleteUserToken removes a user's OAuth2 token from the database
func (s *UserTokenService) DeleteUserToken(ctx context.Context, userID string) error {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "DeleteUserToken")
	defer span.End()

	span.SetAttributes(attribute.String("user_id", userID))

	err := s.repo.DeleteByUserID(ctx, userID)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to delete user token: %w", err)
	}

	s.logger.Info("Deleted user token", "userID", userID)
	return nil
}

// HasUserToken checks if a user has a stored token
func (s *UserTokenService) HasUserToken(ctx context.Context, userID string) (bool, error) {
	tracer := otel.Tracer("github.com/panxiao81/e5renew/services")
	ctx, span := tracer.Start(ctx, "HasUserToken")
	defer span.End()

	span.SetAttributes(attribute.String("user_id", userID))

	_, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		// Token doesn't exist
		return false, nil
	}

	return true, nil
}
