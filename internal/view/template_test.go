package view

import (
	"bytes"
	"context"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/i18n"
	"github.com/panxiao81/e5renew/internal/requestctx"
)

func TestTemplateRender(t *testing.T) {
	tpl, err := New()
	require.NoError(t, err)

	t.Run("render fallback translation funcs", func(t *testing.T) {
		var out bytes.Buffer
		err := tpl.Render(&out, "index.html", map[string]any{"Title": "Home"})
		require.NoError(t, err)
		html := out.String()
		require.Contains(t, html, "home.welcome")
		require.Contains(t, html, "app.title")
	})

	t.Run("render with context localizer", func(t *testing.T) {
		require.NoError(t, i18n.Init())
		ctx := i18n.WithLocalizer(context.Background(), i18n.DefaultBundle.GetLocalizer("en"))

		var out bytes.Buffer
		err := tpl.RenderWithContext(ctx, &out, "index.html", map[string]any{"Title": "Home"})
		require.NoError(t, err)
		html := out.String()
		require.Contains(t, html, "Welcome to E5 Application!")
		require.Contains(t, html, "E5Renew")
	})

	t.Run("render errors for missing template", func(t *testing.T) {
		var out bytes.Buffer
		err := tpl.Render(&out, "missing.html", nil)
		require.Error(t, err)
	})

	t.Run("render with context interpolates dict values", func(t *testing.T) {
		require.NoError(t, i18n.Init())
		ctx := i18n.WithLocalizer(context.Background(), i18n.DefaultBundle.GetLocalizer("en"))

		data := map[string]any{
			"Title":        "User",
			"Debug":        false,
			"HasUserToken": false,
			"Claims": map[string]any{
				"Name":              "Test User",
				"PreferredUsername": "test@example.com",
			},
			"User": map[string]any{
				"Subject": "user-123",
			},
		}

		var out bytes.Buffer
		err := tpl.RenderWithContext(ctx, &out, "user.html", data)
		require.NoError(t, err)

		html := out.String()
		require.Contains(t, html, "Test User")
		require.Contains(t, html, "User ID: user-123")
		require.Contains(t, html, "Not Authorized")
	})

	t.Run("render with context uses default localizer from bare context", func(t *testing.T) {
		require.NoError(t, i18n.Init())

		data := map[string]any{
			"Title":       "Logs",
			"CurrentPage": 2,
			"NextPage":    3,
			"PrevPage":    1,
			"TimeRange":   "24h",
			"JobType":     "",
			"UserID":      "",
		}

		var out bytes.Buffer
		err := tpl.RenderWithContext(context.Background(), &out, "logs.html", data)
		require.NoError(t, err)

		html := out.String()
		require.Contains(t, html, "API Logs")
		require.Contains(t, html, "Page 2")
		require.Contains(t, html, "No logs found")
	})

	t.Run("render fallback helpers cover dict safeHTML and tDefault", func(t *testing.T) {
		var out bytes.Buffer
		err := tpl.Render(&out, "test_helpers.html", nil)
		require.NoError(t, err)

		html := out.String()
		require.Contains(t, html, "<strong>safe</strong>")
		require.Contains(t, html, `<div id="dict-valid">value</div>`)
		require.Contains(t, html, `<div id="dict-odd">map[]</div>`)
		require.Contains(t, html, `<div id="dict-bad-key">map[]</div>`)
		require.Contains(t, html, `<div id="t-default">Fallback text</div>`)
	})

	t.Run("render with context helper fallback default message", func(t *testing.T) {
		require.NoError(t, i18n.Init())
		ctx := i18n.WithLocalizer(context.Background(), i18n.DefaultBundle.GetLocalizer("en"))

		var out bytes.Buffer
		err := tpl.RenderWithContext(ctx, &out, "test_helpers.html", nil)
		require.NoError(t, err)

		html := out.String()
		require.Contains(t, html, `<div id="t-default">Fallback text</div>`)
		require.Contains(t, html, `<div id="dict-valid">value</div>`)
	})

	t.Run("render with context injects user from context when absent", func(t *testing.T) {
		require.NoError(t, i18n.Init())
		ctx := i18n.WithLocalizer(context.Background(), i18n.DefaultBundle.GetLocalizer("en"))
		ctx = requestctx.WithUser(ctx, &oidc.IDToken{Subject: "context-user"})

		var out bytes.Buffer
		err := tpl.RenderWithContext(ctx, &out, "index.html", map[string]any{"Title": "Home"})
		require.NoError(t, err)

		html := out.String()
		require.Contains(t, html, ">API Logs<")
		require.Contains(t, html, ">Logout<")
		require.NotContains(t, html, ">Login<")
	})

	t.Run("render with context keeps explicit user over context default", func(t *testing.T) {
		require.NoError(t, i18n.Init())
		ctx := i18n.WithLocalizer(context.Background(), i18n.DefaultBundle.GetLocalizer("en"))
		ctx = requestctx.WithUser(ctx, &oidc.IDToken{Subject: "context-user"})

		var out bytes.Buffer
		err := tpl.RenderWithContext(ctx, &out, "index.html", map[string]any{
			"Title": "Home",
			"User":  nil,
		})
		require.NoError(t, err)

		html := out.String()
		require.Contains(t, html, ">Login<")
		require.NotContains(t, html, ">Logout<")
	})
}
