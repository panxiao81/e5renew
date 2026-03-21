package environment

import (
	"io"
	"log/slog"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/view"
)

func TestNewApplication(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tmpl, err := view.New()
	require.NoError(t, err)
	sm := scs.New()

	app := NewApplication(logger, tmpl, sm, nil)
	require.Same(t, logger, app.Logger)
	require.Same(t, tmpl, app.Template)
	require.Same(t, sm, app.SessionManager)
	require.Nil(t, app.DB)
}
