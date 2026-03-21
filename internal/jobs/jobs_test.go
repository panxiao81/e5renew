package jobs

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/services"
)

func makeServices(t *testing.T) (*services.APILogService, *services.MailService, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	apiSvc := services.NewAPILogService(db.New(sqlDB), logger)
	viper.Set("encryption.key", "jobs-test-key")
	encryption, err := services.NewEncryptionService()
	require.NoError(t, err)
	userSvc := services.NewUserTokenService(db.New(sqlDB), &oauth2.Config{}, logger, encryption)
	mailSvc := services.NewMailService(userSvc, apiSvc, logger)
	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return apiSvc, mailSvc, mock, cleanup
}

func TestJobSchedulerAndRegister(t *testing.T) {
	apiSvc, mailSvc, _, cleanup := makeServices(t)
	defer cleanup()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	js, err := NewJobScheduler(db.New(nil))
	require.NoError(t, err)
	defer js.Shutdown()

	require.NoError(t, js.RegisterLogCleanupJob(apiSvc, logger, 0))
	require.NoError(t, js.RegisterUserMailTokensJob(mailSvc, logger))
}

func TestJobExecutePaths(t *testing.T) {
	apiSvc, mailSvc, mock, cleanup := makeServices(t)
	defer cleanup()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cleanupJob := NewLogCleanupJob(apiSvc, logger, 0)
	mock.ExpectExec(`(?is)delete\s+from\s+api_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, cleanupJob.Execute(context.Background()))

	mailJob := NewProcessUserMailTokensJob(mailSvc, logger)
	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+order by user_id`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}))
	require.NoError(t, mailJob.Execute(context.Background()))
}

func TestJobExecuteErrorPaths(t *testing.T) {
	apiSvc, mailSvc, mock, cleanup := makeServices(t)
	defer cleanup()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cleanupJob := NewLogCleanupJob(apiSvc, logger, 1)
	mock.ExpectExec(`(?is)delete\s+from\s+api_logs`).WillReturnError(context.DeadlineExceeded)
	require.Error(t, cleanupJob.Execute(context.Background()))

	mailJob := NewProcessUserMailTokensJob(mailSvc, logger)
	mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+order by user_id`).
		WillReturnError(context.Canceled)
	require.Error(t, mailJob.Execute(context.Background()))
}
