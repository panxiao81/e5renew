package view

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/i18n"
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
}
