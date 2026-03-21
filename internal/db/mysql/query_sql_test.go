package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestQueriesCorePaths(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	q := New(db)
	now := time.Now().UTC()

	mock.ExpectExec(`(?is)insert into api_logs`).WillReturnResult(sqlmock.NewResult(1, 1))
	_, err = q.CreateAPILog(context.Background(), CreateAPILogParams{UserID: sql.NullString{String: "u", Valid: true}, ApiEndpoint: "e", HttpMethod: "GET", HttpStatusCode: 200, RequestTime: now, ResponseTime: now, DurationMs: 1, RequestSize: sql.NullInt32{}, ResponseSize: sql.NullInt32{}, ErrorMessage: sql.NullString{}, JobType: "j", Success: true})
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).AddRow(1, "u", "a", "r", now, "Bearer")
	mock.ExpectQuery(`(?is)from user_tokens`).WillReturnRows(rows)
	_, err = q.ListUserTokens(context.Background())
	require.NoError(t, err)

	mock.ExpectExec(`(?is)update user_tokens`).WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = q.UpdateUserTokens(context.Background(), UpdateUserTokensParams{AccessToken: "a2", RefreshToken: "r2", Expiry: now, TokenType: "Bearer", UserID: "u"})
	require.NoError(t, err)

	logRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
		AddRow(1, "u", "e", "GET", 200, now, now, 1, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "j", true)
	mock.ExpectQuery(`(?is)from api_logs`).WillReturnRows(logRows)
	_, err = q.GetAPILogs(context.Background(), GetAPILogsParams{Limit: 1, Offset: 0})
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
