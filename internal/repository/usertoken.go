package repository

import (
	"context"
	"time"

	"github.com/panxiao81/e5renew/internal/db"
)

type UserToken struct {
	ID           int64
	UserID       string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	TokenType    string
}

type UserTokenRecord struct {
	UserID       string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	TokenType    string
}

type UserTokenRepository interface {
	Create(context.Context, UserTokenRecord) error
	Update(context.Context, UserTokenRecord) error
	GetByUserID(context.Context, string) (*UserToken, error)
	List(context.Context) ([]UserToken, error)
	DeleteByUserID(context.Context, string) error
}

type userTokenRepository struct {
	store userTokenStore
}

func NewUserTokenRepositoryWithEngine(engine db.Engine, conn db.DBTX) UserTokenRepository {
	return NewUserTokenRepository(newUserTokenStore(engine, conn))
}

func NewUserTokenRepository(store userTokenStore) UserTokenRepository {
	return &userTokenRepository{store: store}
}

func (r *userTokenRepository) Create(ctx context.Context, record UserTokenRecord) error {
	_, err := r.store.CreateUserTokens(ctx, db.CreateUserTokensParams{
		UserID:       record.UserID,
		AccessToken:  record.AccessToken,
		RefreshToken: record.RefreshToken,
		Expiry:       record.Expiry,
		TokenType:    record.TokenType,
	})
	return err
}

func (r *userTokenRepository) Update(ctx context.Context, record UserTokenRecord) error {
	_, err := r.store.UpdateUserTokens(ctx, db.UpdateUserTokensParams{
		AccessToken:  record.AccessToken,
		RefreshToken: record.RefreshToken,
		Expiry:       record.Expiry,
		TokenType:    record.TokenType,
		UserID:       record.UserID,
	})
	return err
}

func (r *userTokenRepository) GetByUserID(ctx context.Context, userID string) (*UserToken, error) {
	token, err := r.store.GetUserToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &UserToken{
		ID:           token.ID,
		UserID:       token.UserID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		TokenType:    token.TokenType,
	}, nil
}

func (r *userTokenRepository) List(ctx context.Context) ([]UserToken, error) {
	tokens, err := r.store.ListUserTokens(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]UserToken, len(tokens))
	for i, token := range tokens {
		result[i] = UserToken{
			ID:           token.ID,
			UserID:       token.UserID,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			Expiry:       token.Expiry,
			TokenType:    token.TokenType,
		}
	}
	return result, nil
}

func (r *userTokenRepository) DeleteByUserID(ctx context.Context, userID string) error {
	return r.store.DeleteUserTokens(ctx, userID)
}
