package controller

import (
	"context"
	"encoding/gob"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/i18n"
	"github.com/panxiao81/e5renew/internal/view"
)

var i18nInitOnce sync.Once
var sessionGobInitOnce sync.Once

func setupHomeController(t *testing.T) (*HomeController, *scs.SessionManager) {
	t.Helper()

	i18nInitOnce.Do(func() {
		if err := i18n.Init(); err != nil {
			t.Fatalf("failed to initialize i18n bundle: %v", err)
		}
	})

	tmpl, err := view.New()
	if err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	sessionManager := scs.New()

	sessionGobInitOnce.Do(func() {
		gob.Register(oidc.IDToken{})
		gob.Register(oauth2.Token{})
		gob.Register(environment.AzureADClaims{})
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	app := environment.Application{
		Logger:         logger,
		Template:       tmpl,
		SessionManager: sessionManager,
	}

	return NewHomeController(app, nil, nil), sessionManager
}

func TestHomeController_IndexWithoutSessionContext(t *testing.T) {
	controller, _ := setupHomeController(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	controller.Index(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHomeController_UserWithoutSessionContext(t *testing.T) {
	controller, _ := setupHomeController(t)

	req := httptest.NewRequest(http.MethodGet, "/user", nil)
	w := httptest.NewRecorder()

	controller.User(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, w.Code)
	}
	if location := w.Header().Get("Location"); location != "/login" {
		t.Fatalf("expected redirect location %q, got %q", "/login", location)
	}
}

func TestHomeController_UserMissingOrInvalidSessionValues(t *testing.T) {
	controller, sessionManager := setupHomeController(t)

	validUser := oidc.IDToken{Subject: "test-user"}
	validClaims := environment.AzureADClaims{Email: "test@example.com"}
	validToken := oauth2.Token{AccessToken: "access-token"}

	testCases := []struct {
		name string
		seed func(context.Context)
	}{
		{
			name: "missing_user_value",
			seed: func(ctx context.Context) {
				sessionManager.Put(ctx, "claims", validClaims)
				sessionManager.Put(ctx, "token", validToken)
			},
		},
		{
			name: "missing_all_values",
			seed: func(ctx context.Context) {},
		},
		{
			name: "invalid_user_value",
			seed: func(ctx context.Context) {
				sessionManager.Put(ctx, "user", "not-a-token")
				sessionManager.Put(ctx, "claims", validClaims)
				sessionManager.Put(ctx, "token", validToken)
			},
		},
		{
			name: "missing_claims",
			seed: func(ctx context.Context) {
				sessionManager.Put(ctx, "user", validUser)
				sessionManager.Put(ctx, "token", validToken)
			},
		},
		{
			name: "invalid_claims",
			seed: func(ctx context.Context) {
				sessionManager.Put(ctx, "user", validUser)
				sessionManager.Put(ctx, "claims", 12345)
				sessionManager.Put(ctx, "token", validToken)
			},
		},
		{
			name: "missing_token",
			seed: func(ctx context.Context) {
				sessionManager.Put(ctx, "user", validUser)
				sessionManager.Put(ctx, "claims", validClaims)
			},
		},
		{
			name: "invalid_token",
			seed: func(ctx context.Context) {
				sessionManager.Put(ctx, "user", validUser)
				sessionManager.Put(ctx, "claims", validClaims)
				sessionManager.Put(ctx, "token", true)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/user", nil)
			w := httptest.NewRecorder()

			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("User handler panicked for %s: %v", tc.name, recovered)
				}
			}()

			handler := sessionManager.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.seed(r.Context())
				controller.User(w, r)
			}))
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusTemporaryRedirect {
				t.Fatalf("expected status %d, got %d", http.StatusTemporaryRedirect, w.Code)
			}
			if location := w.Header().Get("Location"); location != "/login" {
				t.Fatalf("expected redirect location %q, got %q", "/login", location)
			}
		})
	}
}

func TestHomeController_TypeConvertersSupportPointers(t *testing.T) {
	idToken := &oidc.IDToken{Subject: "test-user"}
	if got, ok := asIDToken(idToken); !ok || got.Subject != "test-user" {
		t.Fatalf("expected pointer ID token conversion to succeed, ok=%v, token=%+v", ok, got)
	}

	claims := &environment.AzureADClaims{Email: "test@example.com"}
	if got, ok := asAzureADClaims(claims); !ok || got.Email != "test@example.com" {
		t.Fatalf("expected pointer claims conversion to succeed, ok=%v, claims=%+v", ok, got)
	}

	token := &oauth2.Token{AccessToken: "access-token"}
	if got, ok := asOAuth2Token(token); !ok || got.AccessToken != "access-token" {
		t.Fatalf("expected pointer oauth2 token conversion to succeed, ok=%v, token=%+v", ok, got)
	}
}

func TestHomeController_TriggerMailAPIWithoutSessionContext(t *testing.T) {
	controller, _ := setupHomeController(t)

	req := httptest.NewRequest(http.MethodPost, "/user/trigger-mail", nil)
	w := httptest.NewRecorder()

	controller.TriggerMailAPI(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHomeController_About(t *testing.T) {
	controller, _ := setupHomeController(t)

	result := controller.About()
	if result == "" {
		t.Fatal("expected non-empty about string")
	}
}
