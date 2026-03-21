package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestQueriesRemainingFunctions(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	q := New(db)
	now := time.Now().UTC()

	mock.ExpectExec(`(?is)insert into user_tokens`).WillReturnResult(sqlmock.NewResult(1, 1))
	_, err = q.CreateUserTokens(context.Background(), CreateUserTokensParams{UserID: "u", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer"})
	require.NoError(t, err)

	mock.ExpectExec(`(?is)delete from api_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, q.DeleteOldAPILogs(context.Background(), now))

	mock.ExpectExec(`(?is)delete from user_tokens`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, q.DeleteUserTokens(context.Background(), "u"))

	statsRow := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(10, 9, 1, float64(12), int64(5), int64(20))
	mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WillReturnRows(statsRow)
	_, err = q.GetAPILogStats(context.Background(), GetAPILogStatsParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)

	statsByEndpointRows := sqlmock.NewRows([]string{"api_endpoint", "total_requests", "successful_requests", "failed_requests", "avg_duration_ms"}).
		AddRow("users", 3, 3, 0, float64(11))
	mock.ExpectQuery(`(?is)group by api_endpoint`).WillReturnRows(statsByEndpointRows)
	_, err = q.GetAPILogStatsByEndpoint(context.Background(), GetAPILogStatsByEndpointParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)

	logCols := []string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}
	oneLog := sqlmock.NewRows(logCols).AddRow(1, "u", "users", "GET", 200, now, now, 1, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
	mock.ExpectQuery(`(?is)where job_type =`).WillReturnRows(oneLog)
	_, err = q.GetAPILogsByJobType(context.Background(), GetAPILogsByJobTypeParams{JobType: "job", Limit: 1, Offset: 0})
	require.NoError(t, err)

	oneLog2 := sqlmock.NewRows(logCols).AddRow(2, "u", "users", "GET", 200, now, now, 1, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
	mock.ExpectQuery(`(?is)where request_time >=`).WillReturnRows(oneLog2)
	_, err = q.GetAPILogsByTimeRange(context.Background(), GetAPILogsByTimeRangeParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now, Limit: 1, Offset: 0})
	require.NoError(t, err)

	oneLog3 := sqlmock.NewRows(logCols).AddRow(3, "u", "users", "GET", 200, now, now, 1, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
	mock.ExpectQuery(`(?is)where user_id =`).WillReturnRows(oneLog3)
	_, err = q.GetAPILogsByUser(context.Background(), GetAPILogsByUserParams{UserID: sql.NullString{String: "u", Valid: true}, Limit: 1, Offset: 0})
	require.NoError(t, err)

	utRows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).AddRow(1, "u", "a", "r", now, "Bearer")
	mock.ExpectQuery(`(?is)from user_tokens\s+where user_id`).WillReturnRows(utRows)
	_, err = q.GetUserToken(context.Background(), "u")
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}
