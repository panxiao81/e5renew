package controller

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
)

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
	if got := validateRedirectURL("https://x.local/oauth2/callback"); got != "" {
		t.Fatalf("validateRedirectURL unexpected: %s", got)
	}
	if got := validateRedirectURL("https://x.local/wrong"); got == "" {
		t.Fatal("expected redirect validation error")
	}
	if got := validateUserTokenRedirectURL("https://x.local/oauth2/callback", "https://x.local/oauth2/callback-user-token"); got != "" {
		t.Fatalf("validateUserTokenRedirectURL unexpected: %s", got)
	}
}
