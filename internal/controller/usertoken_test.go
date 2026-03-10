package controller

import (
	"database/sql"
	"encoding/gob"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/panxiao81/e5renew/internal/environment"
	"github.com/panxiao81/e5renew/internal/services"
	"github.com/panxiao81/e5renew/internal/view"
)

var userTokenSessionOnce sync.Once

func setupUserTokenController(t *testing.T, tokenURL string) (*UserTokenController, *scs.SessionManager, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	viper.Set("encryption.key", "user-token-controller-test-key")
	encryption, err := services.NewEncryptionService()
	require.NoError(t, err)

	tmpl, err := view.New()
	require.NoError(t, err)

	sessions := scs.New()
	userTokenSessionOnce.Do(func() {
		gob.Register(oidc.IDToken{})
		gob.Register(oauth2.Token{})
		gob.Register(environment.AzureADClaims{})
	})

	app := environment.Application{
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Template:       tmpl,
		SessionManager: sessions,
	}

	service := services.NewUserTokenService(db.New(sqlDB), &oauth2.Config{}, app.Logger, encryption)
	auth := environment.Authenticator{Config: oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "http://localhost/oauth2/callback-user-token",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize",
			TokenURL: tokenURL,
		},
	}}
	controller := NewUserTokenController(app, auth, service)

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return controller, sessions, mock, cleanup
}

func seedUserTokenSession(sm *scs.SessionManager, r *http.Request, state string) {
	sm.Put(r.Context(), "user", "session-user")
	sm.Put(r.Context(), "claims", environment.AzureADClaims{Email: "alice@example.com", Name: "Alice"})
	if state != "" {
		sm.Put(r.Context(), "user_token_state", state)
	}
}

func TestUserTokenControllerAuthorizeAndCallbackAndRevoke(t *testing.T) {

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"access-token","refresh_token":"refresh-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	t.Run("authorize redirects to login when session missing", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/user/authorize-token", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			controller.AuthorizeUserToken(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		require.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("authorize stores state and redirects", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/user/authorize-token", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, "")
			controller.AuthorizeUserToken(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		location := w.Header().Get("Location")
		require.Contains(t, location, "login.microsoftonline.com")
		require.Contains(t, location, "state=")
	})

	t.Run("callback oauth error returns 400", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback-user-token?error=access_denied&error_description=denied", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, "any")
			controller.UserTokenCallback(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("callback invalid input returns 400", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback-user-token?state=x&code=y", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, "x")
			controller.UserTokenCallback(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("callback state mismatch returns 400", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		goodState := "abcdefghijklmnop1234567890ABCDEFG"
		code := "abcdefghijklmnopqrstuvwxyzABCDEFG"
		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback-user-token?state="+url.QueryEscape(goodState)+"&code="+url.QueryEscape(code), nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, "DIFFERENT_STATE_abcdefghijklmnopqrstuvwxyz")
			controller.UserTokenCallback(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("callback redirects login if user session disappeared", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		state := "abcdefghijklmnop1234567890ABCDEFG"
		code := "abcdefghijklmnopqrstuvwxyzABCDEFG"
		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback-user-token?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code), nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessions.Put(r.Context(), "user_token_state", state)
			controller.UserTokenCallback(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		require.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("callback save token failure returns 500", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		state := "abcdefghijklmnop1234567890ABCDEFG"
		code := "abcdefghijklmnopqrstuvwxyzABCDEFG"
		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(`(?s)insert into user_tokens`).WillReturnError(sql.ErrConnDone)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback-user-token?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code), nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, state)
			controller.UserTokenCallback(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("callback success redirects to user", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		state := "abcdefghijklmnop1234567890ABCDEFG"
		code := "abcdefghijklmnopqrstuvwxyzABCDEFG"
		mock.ExpectQuery(`(?s)select id, user_id, access_token, refresh_token, expiry, token_type\s+from user_tokens\s+where user_id = \$1`).
			WithArgs("alice@example.com").
			WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(`(?s)insert into user_tokens`).
			WithArgs("alice@example.com", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "Bearer").
			WillReturnResult(sqlmock.NewResult(1, 1))

		req := httptest.NewRequest(http.MethodGet, "/oauth2/callback-user-token?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code), nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, state)
			controller.UserTokenCallback(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		require.Equal(t, "/user", w.Header().Get("Location"))
	})

	t.Run("revoke redirects if not logged in", func(t *testing.T) {

		controller, sessions, _, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		req := httptest.NewRequest(http.MethodPost, "/user/revoke-token", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			controller.RevokeUserToken(w, r)
		}))
		h.ServeHTTP(w, req)

		require.Equal(t, http.StatusTemporaryRedirect, w.Code)
		require.Equal(t, "/login", w.Header().Get("Location"))
	})

	t.Run("revoke error returns 500", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		mock.ExpectExec(`(?s)delete from user_tokens`).
			WithArgs("alice@example.com").
			WillReturnError(sql.ErrConnDone)

		req := httptest.NewRequest(http.MethodPost, "/user/revoke-token", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, "")
			controller.RevokeUserToken(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("revoke success redirects see other", func(t *testing.T) {

		controller, sessions, mock, cleanup := setupUserTokenController(t, tokenServer.URL)
		defer cleanup()

		mock.ExpectExec(`(?s)delete from user_tokens`).
			WithArgs("alice@example.com").
			WillReturnResult(sqlmock.NewResult(0, 1))

		req := httptest.NewRequest(http.MethodPost, "/user/revoke-token", nil)
		w := httptest.NewRecorder()

		h := sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seedUserTokenSession(sessions, r, "")
			controller.RevokeUserToken(w, r)
		}))
		h.ServeHTTP(w, req)
		require.Equal(t, http.StatusSeeOther, w.Code)
		require.Equal(t, "/user", w.Header().Get("Location"))
	})
}

func TestGenerateUserTokenState(t *testing.T) {

	s1, err := generateUserTokenState()
	require.NoError(t, err)
	require.NotEmpty(t, s1)

	s2, err := generateUserTokenState()
	require.NoError(t, err)
	require.NotEqual(t, s1, s2)
}
