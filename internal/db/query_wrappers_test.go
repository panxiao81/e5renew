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
	require.Equal(t, "me/messages", logs[0].ApiEndpoint)

	statsRows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(4), int64(3), int64(1), []byte("12.5"), int32(2), int32(20))
	mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(statsRows)
	stats, err := q.GetAPILogStats(context.Background(), GetAPILogStatsParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)
	require.Equal(t, 12.5, stats.AvgDurationMs)

	endpointRows := sqlmock.NewRows([]string{"api_endpoint", "total_requests", "successful_requests", "failed_requests", "avg_duration_ms"}).
		AddRow("users", int64(2), int64(2), int64(0), "7.5")
	mock.ExpectQuery(`(?is)group by api_endpoint`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(endpointRows)
	endpointStats, err := q.GetAPILogStatsByEndpoint(context.Background(), GetAPILogStatsByEndpointParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)
	require.Len(t, endpointStats, 1)
	require.Equal(t, 7.5, endpointStats[0].AvgDurationMs)

	jobRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
		AddRow(2, "u1", "users", "GET", 200, now, now, 5, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "renewal", true)
	mock.ExpectQuery(`(?is)where job_type =`).WithArgs("renewal", int32(5), int32(0)).WillReturnRows(jobRows)
	jobLogs, err := q.GetAPILogsByJobType(context.Background(), GetAPILogsByJobTypeParams{JobType: "renewal", Limit: 5, Offset: 0})
	require.NoError(t, err)
	require.Len(t, jobLogs, 1)

	timeRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
		AddRow(3, "u1", "users", "GET", 200, now, now, 6, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "renewal", true)
	mock.ExpectQuery(`(?is)where request_time >=`).WithArgs(now.Add(-2*time.Hour), now, int32(5), int32(1)).WillReturnRows(timeRows)
	timeLogs, err := q.GetAPILogsByTimeRange(context.Background(), GetAPILogsByTimeRangeParams{RequestTime: now.Add(-2 * time.Hour), RequestTime_2: now, Limit: 5, Offset: 1})
	require.NoError(t, err)
	require.Len(t, timeLogs, 1)

	userRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
		AddRow(4, "u2", "users", "GET", 200, now, now, 7, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "renewal", true)
	mock.ExpectQuery(`(?is)where user_id =`).WithArgs(sql.NullString{String: "u2", Valid: true}, int32(5), int32(2)).WillReturnRows(userRows)
	userLogs, err := q.GetAPILogsByUser(context.Background(), GetAPILogsByUserParams{UserID: sql.NullString{String: "u2", Valid: true}, Limit: 5, Offset: 2})
	require.NoError(t, err)
	require.Len(t, userLogs, 1)

	tokenRows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
		AddRow(1, "u1", "a", "r", now, "Bearer")
	mock.ExpectQuery(`(?is)select id, user_id, access_token`).WillReturnRows(tokenRows)
	tokens, err := q.ListUserTokens(context.Background())
	require.NoError(t, err)
	require.Len(t, tokens, 1)

	oneTokenRows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
		AddRow(2, "u2", "a2", "r2", now, "Bearer")
	mock.ExpectQuery(`(?is)from user_tokens\s+where user_id`).WithArgs("u2").WillReturnRows(oneTokenRows)
	token, err := q.GetUserToken(context.Background(), "u2")
	require.NoError(t, err)
	require.Equal(t, "u2", token.UserID)

	mock.ExpectExec(`(?is)delete from api_logs`).WithArgs(now.Add(-24 * time.Hour)).WillReturnResult(sqlmock.NewResult(0, 2))
	err = q.DeleteOldAPILogs(context.Background(), now.Add(-24*time.Hour))
	require.NoError(t, err)

	mock.ExpectExec(`(?is)delete from user_tokens`).WithArgs("u2").WillReturnResult(sqlmock.NewResult(0, 1))
	err = q.DeleteUserTokens(context.Background(), "u2")
	require.NoError(t, err)

	mock.ExpectExec(`(?is)update user_tokens`).WithArgs("a2", "r2", now, "Bearer", "u1").WillReturnResult(sqlmock.NewResult(0, 1))
	_, err = q.UpdateUserTokens(context.Background(), UpdateUserTokensParams{AccessToken: "a2", RefreshToken: "r2", Expiry: now, TokenType: "Bearer", UserID: "u1"})
	require.NoError(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAsFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
	}{
		{name: "float64", input: 1.5, want: 1.5},
		{name: "float32", input: float32(2.5), want: 2.5},
		{name: "int64", input: int64(3), want: 3},
		{name: "int32", input: int32(4), want: 4},
		{name: "int", input: 5, want: 5},
		{name: "bytes", input: []byte("6.5"), want: 6.5},
		{name: "string", input: "7.5", want: 7.5},
		{name: "fallback", input: true, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, asFloat64(tt.input))
		})
	}
}
