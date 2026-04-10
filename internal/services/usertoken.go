package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
	// Handle ExpiresIn to Expiry conversion
	if token.Expiry.IsZero() && token.ExpiresIn > 0 {
		token.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	}

	// Encrypt sensitive token data
	encryptedAccessToken, err := s.encryption.Encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := s.encryption.Encrypt(token.RefreshToken)
	if err != nil {
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
			return fmt.Errorf("failed to update user token: %w", err)
		}
		s.logger.Info("Updated existing user token", "userID", userID)
	}
	return nil
}

// GetUserToken retrieves a user's OAuth2 token from the database
func (s *UserTokenService) GetUserToken(ctx context.Context, userID string) (*oauth2.Token, error) {
	storedToken, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user token: %w", err)
	}

	// Decrypt the token data
	accessToken, err := s.encryption.Decrypt(storedToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	refreshToken, err := s.encryption.Decrypt(storedToken.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt refresh token: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       storedToken.Expiry,
		TokenType:    storedToken.TokenType,
	}

	return token, nil
}

// UpdateUserToken updates a user's OAuth2 token in the database
func (s *UserTokenService) UpdateUserToken(ctx context.Context, userID string, token *oauth2.Token) error {
	// Encrypt sensitive token data
	encryptedAccessToken, err := s.encryption.Encrypt(token.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken, err := s.encryption.Encrypt(token.RefreshToken)
	if err != nil {
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
		return fmt.Errorf("failed to update user token: %w", err)
	}

	s.logger.Debug("Updated user token after refresh", "userID", userID)
	return nil
}

// GetAllUserIDs retrieves all user IDs that have stored tokens
func (s *UserTokenService) GetAllUserIDs(ctx context.Context) ([]string, error) {
	tokens, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list user tokens: %w", err)
	}

	userIDs := make([]string, len(tokens))
	for i, token := range tokens {
		userIDs[i] = token.UserID
	}

	return userIDs, nil
}

// DeleteUserToken removes a user's OAuth2 token from the database
func (s *UserTokenService) DeleteUserToken(ctx context.Context, userID string) error {
	err := s.repo.DeleteByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user token: %w", err)
	}

	s.logger.Info("Deleted user token", "userID", userID)
	return nil
}

// HasUserToken checks if a user has a stored token
func (s *UserTokenService) HasUserToken(ctx context.Context, userID string) (bool, error) {
	_, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		// Token doesn't exist
		return false, nil
	}

	return true, nil
}
