package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestI18NCore(t *testing.T) {
	require.NoError(t, Init())
	loc := DefaultBundle.GetLocalizer("en")
	require.Equal(t, "Home", loc.T("nav.home", nil))
	require.Equal(t, "missing.id", loc.T("missing.id", nil))
	require.Equal(t, "Fallback", loc.TDefault("missing.id", "Fallback", nil))

	ctx := WithLocalizer(context.Background(), loc)
	require.NotNil(t, FromContext(ctx))
	require.NotNil(t, FromContext(context.Background()))
}

func TestGetLanguageFromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/?lang=zh", nil)
	require.Equal(t, "zh", GetLanguageFromRequest(r))

	r = httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: "lang", Value: "ja"})
	require.Equal(t, "ja", GetLanguageFromRequest(r))

	r = httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	require.Equal(t, "zh-CN", GetLanguageFromRequest(r))

	r = httptest.NewRequest("GET", "/", nil)
	require.Equal(t, "en", GetLanguageFromRequest(r))
}
