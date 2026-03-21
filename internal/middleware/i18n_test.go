package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	appi18n "github.com/panxiao81/e5renew/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestI18nMiddleware(t *testing.T) {
	require.NoError(t, appi18n.Init())

	t.Run("query language sets cookie and context localizer", func(t *testing.T) {
		var translated string

		handler := I18nMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			translated = appi18n.FromContext(r.Context()).T("nav.home", nil)
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		res := rr.Result()
		defer res.Body.Close()

		require.Equal(t, http.StatusNoContent, res.StatusCode)
		require.Equal(t, "首页", translated)

		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, "lang", cookies[0].Name)
		require.Equal(t, "zh", cookies[0].Value)
		require.Equal(t, "/", cookies[0].Path)
		require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
		require.True(t, cookies[0].HttpOnly)
		require.Equal(t, 31536000, cookies[0].MaxAge)
	})

	t.Run("header language is used when no query or cookie exists", func(t *testing.T) {
		var translated string

		handler := I18nMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			translated = appi18n.FromContext(r.Context()).T("nav.home", nil)
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		res := rr.Result()
		defer res.Body.Close()

		require.Equal(t, http.StatusOK, res.StatusCode)
		require.Equal(t, "首页", translated)
		require.Empty(t, res.Cookies())
	})
}
