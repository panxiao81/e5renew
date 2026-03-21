package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/stretchr/testify/require"
)

type fakeUserTokenStore struct {
	createArg db.CreateUserTokensParams
	createErr error
	updateArg db.UpdateUserTokensParams
	updateErr error
	getUserID string
	getToken  db.UserToken
	getErr    error
	list      []db.UserToken
	listErr   error
	deleteID  string
	deleteErr error
}

func TestNewUserTokenRepositoryWithEngine(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	expiry := time.Now().UTC().Add(time.Hour)
	rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
		AddRow(1, "u1", "a", "r", expiry, "Bearer")
	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \?`).
		WithArgs("u1").
		WillReturnRows(rows)

	repo := NewUserTokenRepositoryWithEngine(db.EngineMySQL, sqlDB)
	token, err := repo.GetByUserID(context.Background(), "u1")
	require.NoError(t, err)
	require.Equal(t, "u1", token.UserID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func (f *fakeUserTokenStore) CreateUserTokens(_ context.Context, arg db.CreateUserTokensParams) (sql.Result, error) {
	f.createArg = arg
	return nil, f.createErr
}
func (f *fakeUserTokenStore) DeleteUserTokens(_ context.Context, userID string) error {
	f.deleteID = userID
	return f.deleteErr
}
func (f *fakeUserTokenStore) GetUserToken(_ context.Context, userID string) (db.UserToken, error) {
	f.getUserID = userID
	return f.getToken, f.getErr
}
func (f *fakeUserTokenStore) ListUserTokens(context.Context) ([]db.UserToken, error) {
	return f.list, f.listErr
}
func (f *fakeUserTokenStore) UpdateUserTokens(_ context.Context, arg db.UpdateUserTokensParams) (sql.Result, error) {
	f.updateArg = arg
	return nil, f.updateErr
}

func TestUserTokenRepository(t *testing.T) {
	t.Run("maps create and update params", func(t *testing.T) {
		store := &fakeUserTokenStore{}
		repo := NewUserTokenRepository(store)
		expiry := time.Now().Add(time.Hour)

		err := repo.Create(context.Background(), UserTokenRecord{UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: expiry, TokenType: "Bearer"})
		require.NoError(t, err)
		require.Equal(t, "u1", store.createArg.UserID)

		err = repo.Update(context.Background(), UserTokenRecord{UserID: "u1", AccessToken: "a2", RefreshToken: "r2", Expiry: expiry, TokenType: "Bearer"})
		require.NoError(t, err)
		require.Equal(t, "a2", store.updateArg.AccessToken)
	})

	t.Run("maps get and list results", func(t *testing.T) {
		expiry := time.Now().Add(time.Hour)
		store := &fakeUserTokenStore{
			getToken: db.UserToken{ID: 1, UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: expiry, TokenType: "Bearer"},
			list:     []db.UserToken{{ID: 2, UserID: "u2", AccessToken: "a2", RefreshToken: "r2", Expiry: expiry, TokenType: "Bearer"}},
		}
		repo := NewUserTokenRepository(store)

		token, err := repo.GetByUserID(context.Background(), "u1")
		require.NoError(t, err)
		require.Equal(t, "u1", token.UserID)

		list, err := repo.List(context.Background())
		require.NoError(t, err)
		require.Len(t, list, 1)
		require.Equal(t, "u2", list[0].UserID)
	})

	t.Run("delete delegates and errors pass through", func(t *testing.T) {
		store := &fakeUserTokenStore{deleteErr: errors.New("boom")}
		repo := NewUserTokenRepository(store)

		err := repo.DeleteByUserID(context.Background(), "u1")
		require.EqualError(t, err, "boom")
		require.Equal(t, "u1", store.deleteID)
	})
}
