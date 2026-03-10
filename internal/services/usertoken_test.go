package services

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/db"
)

func newTestUserTokenService(t *testing.T) (*UserTokenService, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	viper.Set("encryption.key", "unit-test-encryption-key")
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

func TestUserTokenServiceSaveUserToken_CreateAndUpdate(t *testing.T) {

	tests := []struct {
		name       string
		seedExists bool
		token      *oauth2.Token
	}{
		{
			name:       "creates when no existing token",
			seedExists: false,
			token: &oauth2.Token{
				AccessToken:  "access-create",
				RefreshToken: "refresh-create",
				TokenType:    "Bearer",
				ExpiresIn:    600,
			},
		},
		{
			name:       "updates when existing token found",
			seedExists: true,
			token: &oauth2.Token{
				AccessToken:  "access-update",
				RefreshToken: "refresh-update",
				TokenType:    "Bearer",
				Expiry:       time.Now().Add(2 * time.Hour),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {

			service, mock, cleanup := newTestUserTokenService(t)
			defer cleanup()

			if tt.seedExists {
				rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
					AddRow(1, "alice@example.com", "enc-access", "enc-refresh", time.Now().Add(time.Hour), "Bearer")
				mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
					WithArgs("alice@example.com").
					WillReturnRows(rows)
				mock.ExpectExec(regexp.QuoteMeta("update user_tokens\nset access_token = $1,\n    refresh_token = $2,\n    expiry = $3,\n    token_type = $4\nwhere user_id = $5\n")).
					WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "Bearer", "alice@example.com").
					WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
					WithArgs("alice@example.com").
					WillReturnError(sql.ErrNoRows)
				mock.ExpectExec(regexp.QuoteMeta("insert into user_tokens (\n        user_id,\n        access_token,\n        refresh_token,\n        expiry,\n        token_type\n    )\nvalues ($1, $2, $3, $4, $5)\n")).
					WithArgs("alice@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "Bearer").
					WillReturnResult(sqlmock.NewResult(1, 1))
			}

			err := service.SaveUserToken(context.Background(), "alice@example.com", tt.token)
			require.NoError(t, err)
			require.False(t, tt.token.Expiry.IsZero())
		})
	}
}

func TestUserTokenServiceSaveUserTokenErrorBranches(t *testing.T) {

	t.Run("create failure returns wrapped error", func(t *testing.T) {

		service, mock, cleanup := newTestUserTokenService(t)
		defer cleanup()

		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(`(?s)insert into user_tokens`).
			WillReturnError(errors.New("insert failed"))

		err := service.SaveUserToken(context.Background(), "alice@example.com", &oauth2.Token{
			AccessToken:  "a",
			RefreshToken: "r",
			Expiry:       time.Now().Add(time.Hour),
			TokenType:    "Bearer",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to create user token")
	})

	t.Run("update failure returns wrapped error", func(t *testing.T) {

		service, mock, cleanup := newTestUserTokenService(t)
		defer cleanup()

		rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
			AddRow(1, "alice@example.com", "enc-access", "enc-refresh", time.Now().Add(time.Hour), "Bearer")
		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnRows(rows)
		mock.ExpectExec(`(?s)update user_tokens`).
			WillReturnError(errors.New("update failed"))

		err := service.SaveUserToken(context.Background(), "alice@example.com", &oauth2.Token{
			AccessToken:  "a",
			RefreshToken: "r",
			Expiry:       time.Now().Add(time.Hour),
			TokenType:    "Bearer",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to update user token")
	})
}

func TestUserTokenServiceGetAndHasUserToken(t *testing.T) {

	t.Run("get decrypts token", func(t *testing.T) {

		service, mock, cleanup := newTestUserTokenService(t)
		defer cleanup()

		encryptedAccess, err := service.encryption.Encrypt("access")
		require.NoError(t, err)
		encryptedRefresh, err := service.encryption.Encrypt("refresh")
		require.NoError(t, err)
		expiry := time.Now().Add(time.Hour)

		rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
			AddRow(1, "alice@example.com", encryptedAccess, encryptedRefresh, expiry, "Bearer")
		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnRows(rows)

		token, err := service.GetUserToken(context.Background(), "alice@example.com")
		require.NoError(t, err)
		require.Equal(t, "access", token.AccessToken)
		require.Equal(t, "refresh", token.RefreshToken)
		require.Equal(t, "Bearer", token.TokenType)
		require.WithinDuration(t, expiry, token.Expiry, time.Second)
	})

	t.Run("get returns decrypt error", func(t *testing.T) {

		service, mock, cleanup := newTestUserTokenService(t)
		defer cleanup()

		rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
			AddRow(1, "alice@example.com", "not-base64", "also-not-base64", time.Now().Add(time.Hour), "Bearer")
		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnRows(rows)

		_, err := service.GetUserToken(context.Background(), "alice@example.com")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to decrypt access token")
	})

	t.Run("has user token false on db error", func(t *testing.T) {

		service, mock, cleanup := newTestUserTokenService(t)
		defer cleanup()

		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnError(sql.ErrNoRows)

		hasToken, err := service.HasUserToken(context.Background(), "alice@example.com")
		require.NoError(t, err)
		require.False(t, hasToken)
	})
}
