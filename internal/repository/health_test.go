package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeHealthStore struct {
	pingErr  error
	stats    sql.DBStats
	statsErr error
}

func (f *fakeHealthStore) PingContext(context.Context) error { return f.pingErr }
func (f *fakeHealthStore) Stats() (sql.DBStats, error) {
	if f.statsErr != nil {
		return sql.DBStats{}, f.statsErr
	}
	return f.stats, nil
}

func TestHealthRepository(t *testing.T) {
	store := &fakeHealthStore{stats: sql.DBStats{OpenConnections: 4}}
	repo := NewHealthRepository(store)

	require.NoError(t, repo.PingContext(context.Background()))
	stats, err := repo.Stats()
	require.NoError(t, err)
	require.Equal(t, 4, stats.OpenConnections)

	store.pingErr = errors.New("ping boom")
	store.statsErr = errors.New("stats boom")
	require.EqualError(t, repo.PingContext(context.Background()), "ping boom")
	_, err = repo.Stats()
	require.EqualError(t, err, "stats boom")
}
