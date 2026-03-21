package controller

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/environment"
)

func setupLoginController(tokenURL string) (LoginController, *scs.SessionManager) {
	auth := environment.Authenticator{Config: oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost:8080/oauth2/callback",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
			TokenURL: tokenURL,
		},
	}}
	sm := scs.New()
	app := environment.Application{SessionManager: sm, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	return NewLoginController(app, auth), sm
}

func TestLoginController_Login(t *testing.T) {
	controller, sessionManager := setupLoginController("http://127.0.0.1:1/token")

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()

	h := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller.Login(w, r)
	}))
	h.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}
	location := w.Header().Get("Location")
	if !strings.Contains(location, "login.microsoftonline.com") {
		t.Fatalf("expected microsoft redirect location, got %q", location)
	}
}

func TestLoginController_CallbackBranches(t *testing.T) {
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenSrv.Close()

	controller, sm := setupLoginController(tokenSrv.URL)

	tests := []struct {
		name        string
		query       string
		stateInSess string
		status      int
	}{
		{name: "oauth error", query: "error=access_denied", stateInSess: "abc", status: http.StatusBadRequest},
		{name: "invalid input", query: "state=x&code=y", stateInSess: "x", status: http.StatusBadRequest},
		{name: "state mismatch", query: "state=abcdefghijklmnopqrstuvwxyzABCDEFG&code=abcdefghijklmnopqrstuvwxyzABCDEFG", stateInSess: "different-state-abcdefghijklmnopqrstuvwxyz", status: http.StatusBadRequest},
		{name: "exchange error", query: "state=abcdefghijklmnopqrstuvwxyzABCDEFG&code=abcdefghijklmnopqrstuvwxyzABCDEFG", stateInSess: "abcdefghijklmnopqrstuvwxyzABCDEFG", status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?"+tt.query, nil)
			w := httptest.NewRecorder()
			h := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sm.Put(r.Context(), "state", tt.stateInSess)
				controller.Callback(w, r)
			}))
			h.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("expected %d got %d body=%q", tt.status, w.Code, w.Body.String())
			}
		})
	}
}

func TestLoginController_Logout(t *testing.T) {
	controller, sessionManager := setupLoginController("http://127.0.0.1:1/token")

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()

	h := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		controller.Logout(w, r)
	}))
	h.ServeHTTP(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}
	if location := w.Header().Get("Location"); location != "/" {
		t.Fatalf("expected redirect to '/', got %q", location)
	}
}

func TestGenerateRandomState(t *testing.T) {
	state1, err := generateRandomState()
	if err != nil || state1 == "" {
		t.Fatalf("unexpected state1 error/state: %v %q", err, state1)
	}
	state2, err := generateRandomState()
	if err != nil || state2 == "" {
		t.Fatalf("unexpected state2 error/state: %v %q", err, state2)
	}
	if state1 == state2 {
		t.Fatal("expected random states to differ")
	}
	if _, err := url.QueryUnescape(state1); err != nil {
		t.Fatalf("state should be URL-safe: %v", err)
	}
}

func TestLoginController_Login_WithViperConfigSmoke(t *testing.T) {
	viper.Set("azureAD.tenant", "test-tenant")
	_ = viper.GetString("azureAD.tenant")
}
