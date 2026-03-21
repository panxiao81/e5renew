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

type fakeAPILogStore struct {
	createArg        db.CreateAPILogParams
	createErr        error
	deleteBefore     time.Time
	deleteErr        error
	stats            db.GetAPILogStatsRow
	statsErr         error
	endpointStats    []db.GetAPILogStatsByEndpointRow
	endpointStatsErr error
	logs             []db.ApiLog
	logsErr          error
	jobType          string
	userID           string
	start            time.Time
	end              time.Time
	limit            int32
	offset           int32
}

func TestNewAPILogRepositoryWithEngine(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(2), int64(1), int64(1), []byte("11.5"), int32(1), int32(20))
	mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(rows)

	repo := NewAPILogRepositoryWithEngine(db.EngineMySQL, sqlDB)
	stats, err := repo.GetAPILogStats(context.Background(), now.Add(-time.Hour), now)
	require.NoError(t, err)
	require.Equal(t, 11.5, stats.AvgDurationMs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func (f *fakeAPILogStore) CreateAPILog(_ context.Context, arg db.CreateAPILogParams) (sql.Result, error) {
	f.createArg = arg
	return nil, f.createErr
}
func (f *fakeAPILogStore) DeleteOldAPILogs(_ context.Context, before time.Time) error {
	f.deleteBefore = before
	return f.deleteErr
}
func (f *fakeAPILogStore) GetAPILogStats(_ context.Context, arg db.GetAPILogStatsParams) (db.GetAPILogStatsRow, error) {
	f.start = arg.RequestTime
	f.end = arg.RequestTime_2
	return f.stats, f.statsErr
}
func (f *fakeAPILogStore) GetAPILogStatsByEndpoint(_ context.Context, arg db.GetAPILogStatsByEndpointParams) ([]db.GetAPILogStatsByEndpointRow, error) {
	f.start = arg.RequestTime
	f.end = arg.RequestTime_2
	return f.endpointStats, f.endpointStatsErr
}
func (f *fakeAPILogStore) GetAPILogs(_ context.Context, arg db.GetAPILogsParams) ([]db.ApiLog, error) {
	f.limit = arg.Limit
	f.offset = arg.Offset
	return f.logs, f.logsErr
}
func (f *fakeAPILogStore) GetAPILogsByJobType(_ context.Context, arg db.GetAPILogsByJobTypeParams) ([]db.ApiLog, error) {
	f.jobType = arg.JobType
	f.limit = arg.Limit
	f.offset = arg.Offset
	return f.logs, f.logsErr
}
func (f *fakeAPILogStore) GetAPILogsByTimeRange(_ context.Context, arg db.GetAPILogsByTimeRangeParams) ([]db.ApiLog, error) {
	f.start = arg.RequestTime
	f.end = arg.RequestTime_2
	f.limit = arg.Limit
	f.offset = arg.Offset
	return f.logs, f.logsErr
}
func (f *fakeAPILogStore) GetAPILogsByUser(_ context.Context, arg db.GetAPILogsByUserParams) ([]db.ApiLog, error) {
	f.userID = arg.UserID.String
	f.limit = arg.Limit
	f.offset = arg.Offset
	return f.logs, f.logsErr
}

func TestAPILogRepository(t *testing.T) {
	t.Run("maps create params", func(t *testing.T) {
		store := &fakeAPILogStore{}
		repo := NewAPILogRepository(store)

		err := repo.CreateAPILog(context.Background(), APILogEntry{ApiEndpoint: "users", HttpMethod: "GET", HttpStatusCode: 200, DurationMs: 10, JobType: "client_credentials", Success: true})
		require.NoError(t, err)
		require.Equal(t, "users", store.createArg.ApiEndpoint)
	})

	t.Run("maps logs and filter args", func(t *testing.T) {
		store := &fakeAPILogStore{logs: []db.ApiLog{{ID: 1, ApiEndpoint: "users"}}}
		repo := NewAPILogRepository(store)

		logs, err := repo.GetAPILogsByUser(context.Background(), "u1", 10, 5)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		require.Equal(t, int64(1), logs[0].ID)
		require.Equal(t, "u1", store.userID)
		require.Equal(t, int32(10), store.limit)
		require.Equal(t, int32(5), store.offset)
	})

	t.Run("maps stats and endpoint stats", func(t *testing.T) {
		store := &fakeAPILogStore{
			stats:         db.GetAPILogStatsRow{TotalRequests: 3, SuccessfulRequests: 2, FailedRequests: 1, AvgDurationMs: 12.5, MinDurationMs: int32(5), MaxDurationMs: int32(20)},
			endpointStats: []db.GetAPILogStatsByEndpointRow{{ApiEndpoint: "users", TotalRequests: 3, SuccessfulRequests: 2, FailedRequests: 1, AvgDurationMs: 12.5}},
		}
		repo := NewAPILogRepository(store)
		start := time.Now().Add(-time.Hour)
		end := time.Now()

		stats, err := repo.GetAPILogStats(context.Background(), start, end)
		require.NoError(t, err)
		require.Equal(t, int64(3), stats.TotalRequests)

		endpointStats, err := repo.GetAPILogStatsByEndpoint(context.Background(), start, end)
		require.NoError(t, err)
		require.Len(t, endpointStats, 1)
		require.Equal(t, "users", endpointStats[0].APIEndpoint)
	})

	t.Run("passes through errors", func(t *testing.T) {
		store := &fakeAPILogStore{logsErr: errors.New("boom")}
		repo := NewAPILogRepository(store)

		logs, err := repo.GetAPILogs(context.Background(), 10, 0)
		require.Nil(t, logs)
		require.EqualError(t, err, "boom")
	})
}
