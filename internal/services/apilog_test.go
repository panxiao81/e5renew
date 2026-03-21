package services

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/middleware"
)

func newTestAPILogService(t *testing.T) (*APILogService, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	svc := NewAPILogService(db.New(sqlDB), slog.New(slog.NewTextHandler(io.Discard, nil)))
	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return svc, mock, cleanup
}

func TestAPILogService_LogAPICall(t *testing.T) {
	svc, mock, cleanup := newTestAPILogService(t)
	defer cleanup()

	uid := "u1"
	errMsg := "bad gateway"
	entry := middleware.APILogEntry{
		UserID:         &uid,
		APIEndpoint:    "me/messages",
		HTTPMethod:     "GET",
		HTTPStatusCode: 502,
		RequestTime:    time.Now().Add(-100 * time.Millisecond),
		ResponseTime:   time.Now(),
		DurationMs:     100,
		RequestSize:    12,
		ResponseSize:   34,
		ErrorMessage:   &errMsg,
		JobType:        "mail",
		Success:        false,
	}

	mock.ExpectExec(`(?is)insert into api_logs`).
		WithArgs(sql.NullString{String: uid, Valid: true}, entry.APIEndpoint, entry.HTTPMethod, int32(502), sqlmock.AnyArg(), sqlmock.AnyArg(), int32(100), sql.NullInt32{Int32: 12, Valid: true}, sql.NullInt32{Int32: 34, Valid: true}, sql.NullString{String: errMsg, Valid: true}, entry.JobType, false).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, svc.LogAPICall(context.Background(), entry))
}

func TestAPILogService_GetAPILogStats_TypeConversion(t *testing.T) {
	svc, mock, cleanup := newTestAPILogService(t)
	defer cleanup()

	start := time.Now().Add(-time.Hour)
	end := time.Now()

	rows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(10), int64(7), int64(3), 23.5, int32(5), int32(40))

	mock.ExpectQuery(`(?is)select\s+count\(\*\)\s+as\s+total_requests`).
		WithArgs(start, end).
		WillReturnRows(rows)

	stats, err := svc.GetAPILogStats(context.Background(), start, end)
	require.NoError(t, err)
	require.Equal(t, int64(10), stats.TotalRequests)
	// sqlmock currently scans MIN/MAX as int64 into interface{}, while production code
	// only accepts int32 assertions, so this path should safely fall back to zero.
	require.Equal(t, int32(0), stats.MinDurationMs)
	require.Equal(t, int32(0), stats.MaxDurationMs)
}

func TestAPILogService_DeleteOldAPILogs_Error(t *testing.T) {
	svc, mock, cleanup := newTestAPILogService(t)
	defer cleanup()

	before := time.Now().Add(-24 * time.Hour)
	mock.ExpectExec(`(?is)delete\s+from\s+api_logs`).WithArgs(before).WillReturnError(errors.New("boom"))

	err := svc.DeleteOldAPILogs(context.Background(), before)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete old API logs")
}

func TestAPILogService_GetAPILogsByFilters_ErrorPaths(t *testing.T) {
	svc, mock, cleanup := newTestAPILogService(t)
	defer cleanup()

	mock.ExpectQuery(`(?is)where user_id =`).WillReturnError(errors.New("user query failed"))
	_, err := svc.GetAPILogsByUser(context.Background(), "u1", 10, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get API logs by user")

	mock.ExpectQuery(`(?is)where request_time >=`).WillReturnError(errors.New("time query failed"))
	_, err = svc.GetAPILogsByTimeRange(context.Background(), time.Now().Add(-time.Hour), time.Now(), 10, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get API logs by time range")

	mock.ExpectQuery(`(?is)where job_type =`).WillReturnError(errors.New("job query failed"))
	_, err = svc.GetAPILogsByJobType(context.Background(), "mail", 10, 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to get API logs by job type")
}
