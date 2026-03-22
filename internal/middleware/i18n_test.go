package middleware

import (
	"crypto/tls"
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
		require.False(t, cookies[0].Secure)
	})

	t.Run("secure cookie is enabled for https requests", func(t *testing.T) {
		handler := I18nMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)
		req.TLS = &tls.ConnectionState{}
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		res := rr.Result()
		defer res.Body.Close()

		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		require.True(t, cookies[0].Secure)
	})

	t.Run("secure cookie is enabled behind https proxy", func(t *testing.T) {
		handler := I18nMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		res := rr.Result()
		defer res.Body.Close()

		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		require.True(t, cookies[0].Secure)
	})

	t.Run("secure cookie is enabled for multi proxy x-forwarded-proto", func(t *testing.T) {
		handler := I18nMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)
		req.Header.Add("X-Forwarded-Proto", "https,http")
		req.Header.Add("X-Forwarded-Proto", "http")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		res := rr.Result()
		defer res.Body.Close()

		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		require.True(t, cookies[0].Secure)
	})

	t.Run("secure cookie is enabled for forwarded proto across entries", func(t *testing.T) {
		handler := I18nMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		req := httptest.NewRequest(http.MethodGet, "/?lang=zh", nil)
		req.Header.Set("Forwarded", "for=1.2.3.4;proto=https, for=5.6.7.8;proto=http")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		res := rr.Result()
		defer res.Body.Close()

		cookies := res.Cookies()
		require.Len(t, cookies, 1)
		require.True(t, cookies[0].Secure)
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
