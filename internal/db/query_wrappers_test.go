package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestQueriesMySQLWrappers(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	q := NewWithEngine(EngineMySQL, sqlDB)
	now := time.Now().UTC()

	mock.ExpectExec(`(?is)insert into api_logs`).WillReturnResult(sqlmock.NewResult(1, 1))
	_, err = q.CreateAPILog(context.Background(), CreateAPILogParams{
		UserID:         sql.NullString{String: "u1", Valid: true},
		ApiEndpoint:    "me/messages",
		HttpMethod:     "GET",
		HttpStatusCode: 200,
		RequestTime:    now,
		ResponseTime:   now,
		DurationMs:     10,
		RequestSize:    sql.NullInt32{Int32: 1, Valid: true},
		ResponseSize:   sql.NullInt32{Int32: 2, Valid: true},
		ErrorMessage:   sql.NullString{},
		JobType:        "mail",
		Success:        true,
	})
	require.NoError(t, err)

	logRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
		AddRow(1, "u1", "me/messages", "GET", 200, now, now, 10, sql.NullInt32{Int32: 1, Valid: true}, sql.NullInt32{Int32: 2, Valid: true}, sql.NullString{}, "mail", true)
	mock.ExpectQuery(`(?is)select id, user_id, api_endpoint`).WithArgs(int32(10), int32(0)).WillReturnRows(logRows)
	logs, err := q.GetAPILogs(context.Background(), GetAPILogsParams{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, logs, 1)

	tokenRows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
		AddRow(1, "u1", "a", "r", now, "Bearer")
	mock.ExpectQuery(`(?is)select id, user_id, access_token`).WillReturnRows(tokenRows)
	tokens, err := q.ListUserTokens(context.Background())
	require.NoError(t, err)
	require.Len(t, tokens, 1)

	mock.ExpectExec(`(?is)update user_tokens`).WithArgs("a2", "r2", now, "Bearer", "u1").WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = q.UpdateUserTokens(context.Background(), UpdateUserTokensParams{AccessToken: "a2", RefreshToken: "r2", Expiry: now, TokenType: "Bearer", UserID: "u1"})
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
