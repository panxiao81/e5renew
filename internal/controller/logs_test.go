package controller

import (
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/i18n"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/view"
)

var logsI18nOnce sync.Once

func setupLogsController(t *testing.T) (*LogsController, *scs.SessionManager, sqlmock.Sqlmock, func()) {
	t.Helper()

	logsI18nOnce.Do(func() {
		require.NoError(t, i18n.Init())
	})

	tmpl, err := view.New()
	require.NoError(t, err)

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	app := environment.Application{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Template:       tmpl,
		SessionManager: scs.New(),
	}

	service := services.NewAPILogService(db.New(sqlDB), app.Logger)
	controller := NewLogsController(app, service)

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return controller, app.SessionManager, mock, cleanup
}

func seedUserSession(sm *scs.SessionManager, r *http.Request) {
	sm.Put(r.Context(), "user", "session-user")
}

func mockLogsRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "user_id", "api_endpoint", "http_method", "http_status_code", "request_time", "response_time", "duration_ms", "request_size", "response_size", "error_message", "job_type", "success"}).
		AddRow(
			int64(1),
			sql.NullString{String: "alice@example.com", Valid: true},
			"/me/messages",
			"GET",
			int32(200),
			now,
			now.Add(100*time.Millisecond),
			int32(100),
			sql.NullInt32{Int32: 12, Valid: true},
			sql.NullInt32{Int32: 34, Valid: true},
			sql.NullString{},
			"user_mail",
			true,
		)
}

func mockStatsRow() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(10), int64(9), int64(1), float64(120), int32(50), int32(300))
}

func mockEndpointStatsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"api_endpoint", "total_requests", "successful_requests", "failed_requests", "avg_duration_ms"}).
		AddRow("/me/messages", int64(10), int64(9), int64(1), float64(120))
}

func TestLogsControllerIndexAndStats(t *testing.T) {

	t.Run("index redirects when unauthenticated", func(t *testing.T) {

		controller, sessions, _, cleanup := setupLogsController(t)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			controller.Index(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		require.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("index job_type filter renders", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupLogsController(t)
		defer cleanup()

		mock.ExpectQuery(`(?s)FROM api_logs\s+WHERE job_type = \$1`).
			WithArgs("user_mail", int32(50), int32(50)).
			WillReturnRows(mockLogsRows())
		mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\).*FROM api_logs\s+WHERE request_time >= \$1 AND request_time <= \$2`).
			WillReturnRows(mockStatsRow())
		mock.ExpectQuery(`(?s)SELECT\s+api_endpoint,\s+COUNT\(\*\).*GROUP BY api_endpoint`).
			WillReturnRows(mockEndpointStatsRows())

		req := httptest.NewRequest(http.MethodGet, "/logs?page=2&job_type=user_mail", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserSession(sessions, r)
			controller.Index(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Contains(t, w.Body.String(), "API Logs")
	})

	t.Run("index user filter with stats errors still renders", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupLogsController(t)
		defer cleanup()

		mock.ExpectQuery(`(?s)FROM api_logs\s+WHERE user_id = \$1`).
			WithArgs(sql.NullString{String: "alice@example.com", Valid: true}, int32(50), int32(0)).
			WillReturnRows(mockLogsRows())
		mock.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\).*FROM api_logs\s+WHERE request_time >= \$1 AND request_time <= \$2`).
			WillReturnError(sql.ErrConnDone)
		mock.ExpectQuery(`(?s)SELECT\s+api_endpoint,\s+COUNT\(\*\).*GROUP BY api_endpoint`).
			WillReturnError(sql.ErrConnDone)

		req := httptest.NewRequest(http.MethodGet, "/logs?user_id=alice@example.com", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserSession(sessions, r)
			controller.Index(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("index returns 500 when log query fails", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupLogsController(t)
		defer cleanup()

		mock.ExpectQuery(`(?s)FROM api_logs\s+WHERE request_time >= \$1 AND request_time <= \$2`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int32(50), int32(0)).
			WillReturnError(sql.ErrConnDone)

		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserSession(sessions, r)
			controller.Index(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("stats endpoint success and failure branches", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupLogsController(t)
		defer cleanup()

		mock.ExpectQuery(regexp.QuoteMeta("SELECT\n    COUNT(*) as total_requests,")).WillReturnRows(mockStatsRow())
		mock.ExpectQuery(`(?s)SELECT\s+api_endpoint,\s+COUNT\(\*\).*GROUP BY api_endpoint`).
			WillReturnRows(mockEndpointStatsRows())

		req := httptest.NewRequest(http.MethodGet, "/logs/stats?time_range=1h", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserSession(sessions, r)
			controller.Stats(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		controller2, sessions2, mock2, cleanup2 := setupLogsController(t)
		defer cleanup2()
		mock2.ExpectQuery(`(?s)SELECT\s+COUNT\(\*\).*FROM api_logs\s+WHERE request_time >= \$1 AND request_time <= \$2`).
			WillReturnError(sql.ErrConnDone)

		req2 := httptest.NewRequest(http.MethodGet, "/logs/stats", nil)
		w2 := httptest.NewRecorder()
		h2 := sessions2.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserSession(sessions2, r)
			controller2.Stats(w, r)
		}))
		h2.ServeHTTP(w2, req2)
		require.Equal(t, http.StatusInternalServerError, w2.Code)
	})
}
