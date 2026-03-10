package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/db"
)

type staticTokenSource struct {
	token *oauth2.Token
	err   error
}

func (s staticTokenSource) Token() (*oauth2.Token, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.token, nil
}

func newTokenSourceTestService(t *testing.T) (*UserTokenService, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	viper.Set("encryption.key", "token-source-test-key")
	encryption, err := NewEncryptionService()
	require.NoError(t, err)

	service := NewUserTokenService(
		db.New(sqlDB),
		&oauth2.Config{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		encryption,
	)

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return service, mock, cleanup
}

func TestDatabaseUpdatingTokenSourceToken(t *testing.T) {

	now := time.Now().UTC()
	initial := &oauth2.Token{AccessToken: "a1", RefreshToken: "r1", TokenType: "Bearer", Expiry: now.Add(time.Hour)}
	refreshed := &oauth2.Token{AccessToken: "a2", RefreshToken: "r2", TokenType: "Bearer", Expiry: now.Add(2 * time.Hour)}

	t.Run("refresh updates db and returns token", func(t *testing.T) {

		service, mock, cleanup := newTokenSourceTestService(t)
		defer cleanup()

		mock.ExpectExec(`(?s)update user_tokens`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), refreshed.Expiry, refreshed.TokenType, "alice@example.com").
			WillReturnResult(sqlmock.NewResult(0, 1))

		source := NewDatabaseUpdatingTokenSource(
			staticTokenSource{token: refreshed},
			"alice@example.com",
			service,
			initial,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)

		token, err := source.Token()
		require.NoError(t, err)
		require.Equal(t, "a2", token.AccessToken)
	})

	t.Run("unchanged token skips db update", func(t *testing.T) {

		service, _, cleanup := newTokenSourceTestService(t)
		defer cleanup()

		source := NewDatabaseUpdatingTokenSource(
			staticTokenSource{token: initial},
			"alice@example.com",
			service,
			initial,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)

		token, err := source.Token()
		require.NoError(t, err)
		require.Equal(t, initial.AccessToken, token.AccessToken)
	})

	t.Run("db update failure is logged but not returned", func(t *testing.T) {

		service, mock, cleanup := newTokenSourceTestService(t)
		defer cleanup()

		mock.ExpectExec(`(?s)update user_tokens`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), refreshed.Expiry, refreshed.TokenType, "alice@example.com").
			WillReturnError(errors.New("update failed"))

		source := NewDatabaseUpdatingTokenSource(
			staticTokenSource{token: refreshed},
			"alice@example.com",
			service,
			initial,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)

		token, err := source.Token()
		require.NoError(t, err)
		require.Equal(t, refreshed.AccessToken, token.AccessToken)
	})

	t.Run("underlying token source error bubbles up", func(t *testing.T) {

		service, _, cleanup := newTokenSourceTestService(t)
		defer cleanup()

		source := NewDatabaseUpdatingTokenSource(
			staticTokenSource{err: errors.New("source failed")},
			"alice@example.com",
			service,
			nil,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)

		token, err := source.Token()
		require.Nil(t, token)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get token from underlying source")
	})
}

func TestUserTokenServiceGetTokenSourceWithoutCallbackError(t *testing.T) {

	service, mock, cleanup := newTokenSourceTestService(t)
	defer cleanup()

	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
		WithArgs("alice@example.com").
		WillReturnError(errors.New("lookup failed"))

	_, err := service.GetTokenSourceWithoutCallback(context.Background(), "alice@example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get stored token")
}
