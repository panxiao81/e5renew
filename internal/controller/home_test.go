package controller

import (
	"context"
	"encoding/gob"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/i18n"
	"github.com/panxiao81/e5renew/internal/services"
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

func TestHomeController_TriggerMailAPIBranches(t *testing.T) {
	controller, sm := setupHomeController(t)

	tests := []struct {
		name   string
		seed   func(context.Context)
		status int
	}{
		{
			name:   "missing_claims",
			seed:   func(ctx context.Context) { sm.Put(ctx, "user", oidc.IDToken{Subject: "u"}) },
			status: http.StatusUnauthorized,
		},
		{
			name: "invalid_claims",
			seed: func(ctx context.Context) {
				sm.Put(ctx, "user", oidc.IDToken{Subject: "u"})
				sm.Put(ctx, "claims", "bad")
			},
			status: http.StatusUnauthorized,
		},
		{
			name: "service_not_configured",
			seed: func(ctx context.Context) {
				sm.Put(ctx, "user", oidc.IDToken{Subject: "u"})
				sm.Put(ctx, "claims", environment.AzureADClaims{Email: "a@example.com"})
			},
			status: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/user/trigger-mail", nil)
			w := httptest.NewRecorder()
			h := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.seed(r.Context())
				controller.TriggerMailAPI(w, r)
			}))
			h.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("expected %d got %d body=%q", tt.status, w.Code, w.Body.String())
			}
		})
	}
}

func TestHomeController_UserSuccessRendersTokenState(t *testing.T) {
	controller, sm := setupHomeController(t)
	originalHasUserToken := hasUserTokenForHome
	originalGetUserToken := getUserTokenForHome
	t.Cleanup(func() {
		hasUserTokenForHome = originalHasUserToken
		getUserTokenForHome = originalGetUserToken
	})

	expiry := time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC)
	hasUserTokenForHome = func(s *services.UserTokenService, ctx context.Context, userID string) (bool, error) {
		return true, nil
	}
	getUserTokenForHome = func(s *services.UserTokenService, ctx context.Context, userID string) (*oauth2.Token, error) {
		return &oauth2.Token{AccessToken: "access", Expiry: expiry}, nil
	}
	controller.userTokenService = &services.UserTokenService{}

	req := httptest.NewRequest(http.MethodGet, "/user", nil)
	w := httptest.NewRecorder()
	h := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "user", oidc.IDToken{Subject: "u"})
		sm.Put(r.Context(), "claims", environment.AzureADClaims{Name: "Test User", PreferredUsername: "test@example.com", Email: "test@example.com"})
		sm.Put(r.Context(), "token", oauth2.Token{AccessToken: "token"})
		controller.User(w, r)
	}))
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%q", http.StatusOK, w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !containsAll(body, "Test User", "Authorized", expiry.Format("2006-01-02 15:04:05")) {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestHomeController_TriggerMailAPIFullBranches(t *testing.T) {
	originalHasUserToken := hasUserTokenForHome
	originalProcess := processUserMailActivityForHome
	t.Cleanup(func() {
		hasUserTokenForHome = originalHasUserToken
		processUserMailActivityForHome = originalProcess
	})

	tests := []struct {
		name           string
		hasToken       bool
		hasTokenErr    error
		processErr     error
		wantStatus     int
		wantBodySubstr string
	}{
		{name: "token check failure", hasTokenErr: context.Canceled, wantStatus: http.StatusInternalServerError, wantBodySubstr: "token_check_failed"},
		{name: "missing user token", hasToken: false, wantStatus: http.StatusBadRequest, wantBodySubstr: "No personal mail access token found"},
		{name: "mail processing failure", hasToken: true, processErr: context.DeadlineExceeded, wantStatus: http.StatusInternalServerError, wantBodySubstr: "mail_processing_failed"},
		{name: "success", hasToken: true, wantStatus: http.StatusOK, wantBodySubstr: "Mail API call completed successfully"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, sm := setupHomeController(t)
			controller.userTokenService = &services.UserTokenService{}
			controller.mailService = &services.MailService{}

			hasUserTokenForHome = func(s *services.UserTokenService, ctx context.Context, userID string) (bool, error) {
				return tt.hasToken, tt.hasTokenErr
			}
			processUserMailActivityForHome = func(s *services.MailService, ctx context.Context, userID string) error {
				return tt.processErr
			}

			req := httptest.NewRequest(http.MethodPost, "/user/trigger-mail", nil)
			w := httptest.NewRecorder()
			h := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sm.Put(r.Context(), "user", oidc.IDToken{Subject: "u"})
				sm.Put(r.Context(), "claims", environment.AzureADClaims{Email: "a@example.com"})
				controller.TriggerMailAPI(w, r)
			}))
			h.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d got %d body=%q", tt.wantStatus, w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tt.wantBodySubstr) {
				t.Fatalf("expected body to contain %q, got %q", tt.wantBodySubstr, w.Body.String())
			}
		})
	}
}

func TestHomeController_About(t *testing.T) {
	controller, _ := setupHomeController(t)

	result := controller.About()
	if result == "" {
		t.Fatal("expected non-empty about string")
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
