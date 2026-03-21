package services

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/repository"
)

func TestDatabaseTokenCredential_GetToken(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	viper.Set("encryption.key", "credential-test-key")
	encryption, err := NewEncryptionService()
	require.NoError(t, err)
	svc := NewUserTokenService(repository.NewUserTokenRepositoryWithEngine(db.EnginePostgres, sqlDB), &oauth2.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), encryption)

	t.Run("success", func(t *testing.T) {
		encA, _ := encryption.Encrypt("access")
		encR, _ := encryption.Encrypt("refresh")
		exp := time.Now().Add(time.Hour)
		rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).AddRow(1, "u1", encA, encR, exp, "Bearer")
		mock.ExpectQuery(`(?s)from user_tokens\s+where user_id = \$1`).WithArgs("u1").WillReturnRows(rows)

		cred := NewDatabaseTokenCredential("u1", svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
		tok, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{"scope"}})
		require.NoError(t, err)
		require.Equal(t, "access", tok.Token)
	})

	t.Run("token source error", func(t *testing.T) {
		mock.ExpectQuery(`(?s)from user_tokens\s+where user_id = \$1`).WithArgs("missing").WillReturnError(errors.New("db down"))
		cred := NewDatabaseTokenCredential("missing", svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
		_, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get token source")
	})

	require.NoError(t, mock.ExpectationsWereMet())
}
