package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/stretchr/testify/require"
)

func TestHealthRepositoryWithEngine(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	repo := NewHealthRepositoryWithEngine(db.EngineMySQL, sqlDB)

	require.NoError(t, repo.PingContext(context.Background()))
	stats, err := repo.Stats()
	require.NoError(t, err)
	require.GreaterOrEqual(t, stats.OpenConnections, 0)
}

func TestAPILogStoreImplementations(t *testing.T) {
	now := time.Now().UTC()

	t.Run("postgres store covers create delete and list methods", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer sqlDB.Close()

		store := newAPILogStore(db.EnginePostgres, sqlDB)

		mock.ExpectExec(`(?is)insert into api_logs`).WillReturnResult(sqlmock.NewResult(1, 1))
		_, err = store.CreateAPILog(context.Background(), db.CreateAPILogParams{ApiEndpoint: "users", HttpMethod: "GET", HttpStatusCode: 200, RequestTime: now, ResponseTime: now, DurationMs: 1, JobType: "job", Success: true})
		require.NoError(t, err)

		mock.ExpectExec(`(?is)delete from api_logs`).WithArgs(now).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, store.DeleteOldAPILogs(context.Background(), now))

		rows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
			AddRow(1, "u1", "users", "GET", 200, now, now, 10, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
		mock.ExpectQuery(`(?is)select id, user_id, api_endpoint`).WithArgs(int32(5), int32(0)).WillReturnRows(rows)
		logs, err := store.GetAPILogs(context.Background(), db.GetAPILogsParams{Limit: 5, Offset: 0})
		require.NoError(t, err)
		require.Len(t, logs, 1)

		jobRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
			AddRow(2, "u1", "users", "GET", 200, now, now, 10, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
		mock.ExpectQuery(`(?is)where job_type =`).WithArgs("job", int32(5), int32(0)).WillReturnRows(jobRows)
		_, err = store.GetAPILogsByJobType(context.Background(), db.GetAPILogsByJobTypeParams{JobType: "job", Limit: 5, Offset: 0})
		require.NoError(t, err)

		timeRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
			AddRow(3, "u1", "users", "GET", 200, now, now, 10, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
		mock.ExpectQuery(`(?is)where request_time >=`).WithArgs(now.Add(-time.Hour), now, int32(5), int32(0)).WillReturnRows(timeRows)
		_, err = store.GetAPILogsByTimeRange(context.Background(), db.GetAPILogsByTimeRangeParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now, Limit: 5, Offset: 0})
		require.NoError(t, err)

		userRows := sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
			AddRow(4, "u1", "users", "GET", 200, now, now, 10, sql.NullInt32{}, sql.NullInt32{}, sql.NullString{}, "job", true)
		mock.ExpectQuery(`(?is)where user_id =`).WithArgs(sql.NullString{String: "u1", Valid: true}, int32(5), int32(0)).WillReturnRows(userRows)
		_, err = store.GetAPILogsByUser(context.Background(), db.GetAPILogsByUserParams{UserID: sql.NullString{String: "u1", Valid: true}, Limit: 5, Offset: 0})
		require.NoError(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mysql store covers stats and list methods", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer sqlDB.Close()

		store := newAPILogStore(db.EngineMySQL, sqlDB)

		statsRows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
			AddRow(int64(2), int64(1), int64(1), []byte("10.5"), int32(1), int32(20))
		mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(statsRows)
		stats, err := store.GetAPILogStats(context.Background(), db.GetAPILogStatsParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
		require.NoError(t, err)
		require.Equal(t, 10.5, stats.AvgDurationMs)

		endpointRows := sqlmock.NewRows([]string{"api_endpoint", "total_requests", "successful_requests", "failed_requests", "avg_duration_ms"}).
			AddRow("users", int64(2), int64(1), int64(1), []byte("11.5"))
		mock.ExpectQuery(`(?is)group by api_endpoint`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(endpointRows)
		endpointStats, err := store.GetAPILogStatsByEndpoint(context.Background(), db.GetAPILogStatsByEndpointParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
		require.NoError(t, err)
		require.Equal(t, 11.5, endpointStats[0].AvgDurationMs)

		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO api_logs (")).WillReturnResult(sqlmock.NewResult(1, 1))
		_, err = store.CreateAPILog(context.Background(), db.CreateAPILogParams{ApiEndpoint: "users", HttpMethod: "GET", HttpStatusCode: 200, RequestTime: now, ResponseTime: now, DurationMs: 1, JobType: "job", Success: true})
		require.NoError(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUserTokenStoreImplementations(t *testing.T) {
	now := time.Now().UTC()

	t.Run("postgres store covers create update list delete", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer sqlDB.Close()

		store := newUserTokenStore(db.EnginePostgres, sqlDB)

		mock.ExpectExec(`(?is)insert into user_tokens`).WithArgs("u1", "a", "r", now, "Bearer").WillReturnResult(sqlmock.NewResult(1, 1))
		_, err = store.CreateUserTokens(context.Background(), db.CreateUserTokensParams{UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer"})
		require.NoError(t, err)

		mock.ExpectExec(`(?is)update user_tokens`).WithArgs("a2", "r2", now, "Bearer", "u1").WillReturnResult(sqlmock.NewResult(0, 1))
		_, err = store.UpdateUserTokens(context.Background(), db.UpdateUserTokensParams{AccessToken: "a2", RefreshToken: "r2", Expiry: now, TokenType: "Bearer", UserID: "u1"})
		require.NoError(t, err)

		listRows := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
			AddRow(1, "u1", "a", "r", now, "Bearer")
		mock.ExpectQuery(`(?is)select id, user_id, access_token`).WillReturnRows(listRows)
		list, err := store.ListUserTokens(context.Background())
		require.NoError(t, err)
		require.Len(t, list, 1)

		mock.ExpectExec(`(?is)delete from user_tokens`).WithArgs("u1").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, store.DeleteUserTokens(context.Background(), "u1"))

		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mysql store covers create get and update", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer sqlDB.Close()

		store := newUserTokenStore(db.EngineMySQL, sqlDB)

		mock.ExpectExec(`(?is)insert into user_tokens`).WithArgs("u1", "a", "r", now, "Bearer").WillReturnResult(sqlmock.NewResult(1, 1))
		_, err = store.CreateUserTokens(context.Background(), db.CreateUserTokensParams{UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer"})
		require.NoError(t, err)

		row := sqlmock.NewRows([]string{"id", "user_id", "access_token", "refresh_token", "expiry", "token_type"}).
			AddRow(1, "u1", "a", "r", now, "Bearer")
		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \?`).WithArgs("u1").WillReturnRows(row)
		token, err := store.GetUserToken(context.Background(), "u1")
		require.NoError(t, err)
		require.Equal(t, "u1", token.UserID)

		mock.ExpectExec(`(?is)update user_tokens`).WithArgs("a2", "r2", now, "Bearer", "u1").WillReturnResult(sqlmock.NewResult(0, 1))
		_, err = store.UpdateUserTokens(context.Background(), db.UpdateUserTokensParams{AccessToken: "a2", RefreshToken: "r2", Expiry: now, TokenType: "Bearer", UserID: "u1"})
		require.NoError(t, err)

		require.NoError(t, mock.ExpectationsWereMet())
	})
}
