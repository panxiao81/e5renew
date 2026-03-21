package controller

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
)

type fakeHealthDB struct {
	pingErr  error
	stats    sql.DBStats
	statsErr error
}

func (f fakeHealthDB) CreateAPILog(context.Context, db.CreateAPILogParams) (sql.Result, error) {
	return nil, nil
}
func (f fakeHealthDB) DeleteOldAPILogs(context.Context, time.Time) error { return nil }
func (f fakeHealthDB) GetAPILogStats(context.Context, db.GetAPILogStatsParams) (db.GetAPILogStatsRow, error) {
	return db.GetAPILogStatsRow{}, nil
}
func (f fakeHealthDB) GetAPILogStatsByEndpoint(context.Context, db.GetAPILogStatsByEndpointParams) ([]db.GetAPILogStatsByEndpointRow, error) {
	return nil, nil
}
func (f fakeHealthDB) GetAPILogs(context.Context, db.GetAPILogsParams) ([]db.ApiLog, error) {
	return nil, nil
}
func (f fakeHealthDB) GetAPILogsByJobType(context.Context, db.GetAPILogsByJobTypeParams) ([]db.ApiLog, error) {
	return nil, nil
}
func (f fakeHealthDB) GetAPILogsByTimeRange(context.Context, db.GetAPILogsByTimeRangeParams) ([]db.ApiLog, error) {
	return nil, nil
}
func (f fakeHealthDB) GetAPILogsByUser(context.Context, db.GetAPILogsByUserParams) ([]db.ApiLog, error) {
	return nil, nil
}
func (f fakeHealthDB) CreateUserTokens(context.Context, db.CreateUserTokensParams) (sql.Result, error) {
	return nil, nil
}
func (f fakeHealthDB) DeleteUserTokens(context.Context, string) error { return nil }
func (f fakeHealthDB) GetUserToken(context.Context, string) (db.UserToken, error) {
	return db.UserToken{}, nil
}
func (f fakeHealthDB) ListUserTokens(context.Context) ([]db.UserToken, error) { return nil, nil }
func (f fakeHealthDB) UpdateUserTokens(context.Context, db.UpdateUserTokensParams) (sql.Result, error) {
	return nil, nil
}
func (f fakeHealthDB) PingContext(context.Context) error { return f.pingErr }
func (f fakeHealthDB) Stats() (sql.DBStats, error) {
	if f.statsErr != nil {
		return sql.DBStats{}, f.statsErr
	}
	return f.stats, nil
}
func (f fakeHealthDB) WithTx(*sql.Tx) db.Database { return f }

func setupHealthController(t *testing.T, issuerURL string) (*HealthController, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	viper.Set("database.engine", "postgres")
	viper.Set("database.dsn", "postgres://u:p@localhost:5432/db?sslmode=disable")
	viper.Set("azureAD.tenant", "common")
	viper.Set("azureAD.clientID", "client")
	viper.Set("azureAD.redirectURL", "https://localhost/oauth2/callback")
	viper.Set("azureAD.issuer", issuerURL)

	app := environment.Application{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DB: db.New(sqlDB)}
	cleanup := func() {
		mock.ExpectClose()
		_ = sqlDB.Close()
		_ = mock.ExpectationsWereMet()
	}
	return NewHealthController(app), cleanup
}

func oidcServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"` + "http://" + r.Host + `","authorization_endpoint":"http://` + r.Host + `/authorize","token_endpoint":"http://` + r.Host + `/token","jwks_uri":"http://` + r.Host + `/jwks"}`))
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	})
	return httptest.NewServer(mux)
}

func TestHealthController_Endpoints(t *testing.T) {
	oidc := oidcServer()
	defer oidc.Close()

	controller, cleanup := setupHealthController(t, oidc.URL)
	defer cleanup()

	r := chi.NewRouter()
	controller.RegisterRoutes(r)

	t.Run("live", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "alive" {
			t.Fatalf("unexpected live response code=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("ready", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != "ready" {
			t.Fatalf("unexpected ready response code=%d body=%q", w.Code, w.Body.String())
		}
	})

	t.Run("health", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("unexpected health response code=%d body=%q", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"status":"up"`) {
			t.Fatalf("expected up status body=%q", w.Body.String())
		}
	})
}

func TestHealthHelpers(t *testing.T) {
	if got := sanitizeDSN(""); got != "(empty)" {
		t.Fatalf("sanitizeDSN empty = %q", got)
	}
	if got := sanitizeDSN("postgres://user:secret@db.example.com:5432/e5renew"); got != "postgres://db.example.com:5432/e5renew" {
		t.Fatalf("sanitizeDSN url = %q", got)
	}
	if got := sanitizeDSN("user:secret@tcp(localhost:3306)/db"); got != "user://(none)" {
		t.Fatalf("sanitizeDSN mysql = %q", got)
	}
	if got := sanitizeDSN("abcdefghijklmnopqrstuvwxyz"); got != "abcdefghijklmnopqrstuvwx..." {
		t.Fatalf("sanitizeDSN long = %q", got)
	}
	if got := sanitizeDSN("short-dsn"); got != "short-dsn" {
		t.Fatalf("sanitizeDSN short = %q", got)
	}
	if got := validateRedirectURL("https://x.local/oauth2/callback"); got != "" {
		t.Fatalf("validateRedirectURL unexpected: %s", got)
	}
	if got := validateRedirectURL("https://x.local/wrong"); got == "" {
		t.Fatal("expected redirect validation error")
	}
	if got := validateRedirectURL(""); got == "" {
		t.Fatal("expected empty redirect validation error")
	}
	if got := validateUserTokenRedirectURL("https://x.local/oauth2/callback", "https://x.local/oauth2/callback-user-token"); got != "" {
		t.Fatalf("validateUserTokenRedirectURL unexpected: %s", got)
	}
	if got := validateUserTokenRedirectURL("https://x.local/oauth2/callback", "https://y.local/oauth2/callback-user-token"); got == "" {
		t.Fatal("expected host mismatch error")
	}
	if got := validateUserTokenRedirectURL("https://x.local/oauth2/callback", "https://x.local/wrong"); got == "" {
		t.Fatal("expected user token path mismatch error")
	}
}

func TestHealthController_ReadyAndHelpers(t *testing.T) {
	t.Run("ready returns service unavailable when db is down", func(t *testing.T) {
		viper.Set("database.engine", "mysql")
		viper.Set("database.dsn", "user:pass@tcp(localhost:3306)/db")
		controller := NewHealthController(environment.Application{
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			DB: fakeHealthDB{
				pingErr:  errors.New("db unavailable"),
				statsErr: errors.New("stats unavailable"),
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()
		controller.Ready(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
		}
		if strings.TrimSpace(w.Body.String()) != "not ready" {
			t.Fatalf("unexpected body %q", w.Body.String())
		}
	})

	t.Run("checkDatabase reports metadata on stats error and ping failure", func(t *testing.T) {
		viper.Set("database.engine", "mysql")
		viper.Set("database.dsn", "user:secret@tcp(localhost:3306)/db")
		controller := NewHealthController(environment.Application{
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			DB: fakeHealthDB{
				pingErr:  errors.New("ping failed"),
				statsErr: errors.New("stats failed"),
			},
		})

		check := controller.checkDatabase(context.Background())
		if check.Status != HealthStatusDown {
			t.Fatalf("expected down status, got %s", check.Status)
		}
		if !strings.Contains(check.Message, "Database ping failed") {
			t.Fatalf("unexpected message %q", check.Message)
		}
		if check.Metadata["pool_stats_error"] != "stats failed" {
			t.Fatalf("unexpected pool_stats_error %q", check.Metadata["pool_stats_error"])
		}
		if check.Metadata["dsn_hint"] != "user://(none)" {
			t.Fatalf("unexpected dsn hint %q", check.Metadata["dsn_hint"])
		}
	})

	t.Run("health becomes unknown when issuer is healthy but redirect config is suspicious", func(t *testing.T) {
		server := oidcServer()
		defer server.Close()

		controller, cleanup := setupHealthController(t, server.URL)
		defer cleanup()
		viper.Set("azureAD.redirectURL", "https://localhost/wrong")

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		controller.Health(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, w.Code, w.Body.String())
		}
		body := w.Body.String()
		if !strings.Contains(body, `"status":"unknown"`) {
			t.Fatalf("expected unknown status body=%q", body)
		}
		if !strings.Contains(body, `"redirect_warning"`) {
			t.Fatalf("expected redirect warning in body=%q", body)
		}
	})
}
