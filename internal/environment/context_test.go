package environment

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/stretchr/testify/require"

	"github.com/panxiao81/e5renew/internal/repository"
	"github.com/panxiao81/e5renew/internal/view"
)

type fakeHealthStore struct{}

func (fakeHealthStore) PingContext(context.Context) error { return nil }
func (fakeHealthStore) Stats() (sql.DBStats, error)       { return sql.DBStats{}, nil }

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

	healthStore := fakeHealthStore{}
	app = NewApplication(logger, tmpl, sm, healthStore)
	require.Equal(t, healthStore, app.DB)

	var _ repository.HealthRepository = healthStore
}
