package middleware

import (
	"encoding/gob"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/requestctx"
)

var sessionUserGobOnce sync.Once

func TestSessionUserMiddleware(t *testing.T) {
	sessionUserGobOnce.Do(func() {
		gob.Register(oidc.IDToken{})
	})

	t.Run("injects valid session user into context", func(t *testing.T) {
		sm := scs.New()
		var subject string

		seed := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), "user", oidc.IDToken{Subject: "session-user"})
			w.WriteHeader(http.StatusNoContent)
		}))

		seedReq := httptest.NewRequest(http.MethodGet, "/seed", nil)
		seedResp := httptest.NewRecorder()
		seed.ServeHTTP(seedResp, seedReq)
		cookies := seedResp.Result().Cookies()
		require.NotEmpty(t, cookies)

		handler := sm.LoadAndSave(SessionUserMiddleware(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := requestctx.UserFromContext(r.Context())
			require.True(t, ok)
			require.NotNil(t, user)
			subject = user.Subject
			w.WriteHeader(http.StatusNoContent)
		})))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNoContent, rr.Code)
		require.Equal(t, "session-user", subject)
	})

	t.Run("injects valid session user already in current request context", func(t *testing.T) {
		sm := scs.New()
		var subject string

		handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), "user", oidc.IDToken{Subject: "session-user"})
			SessionUserMiddleware(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, ok := requestctx.UserFromContext(r.Context())
				require.True(t, ok)
				require.NotNil(t, user)
				subject = user.Subject
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(w, r)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNoContent, rr.Code)
		require.Equal(t, "session-user", subject)
	})

	t.Run("ignores missing or invalid session user", func(t *testing.T) {
		sm := scs.New()

		handler := sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sm.Put(r.Context(), "user", "invalid")
			SessionUserMiddleware(sm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, ok := requestctx.UserFromContext(r.Context())
				require.False(t, ok)
				require.Nil(t, user)
				w.WriteHeader(http.StatusNoContent)
			})).ServeHTTP(w, r)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		require.Equal(t, http.StatusNoContent, rr.Code)
	})
}
