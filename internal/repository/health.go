package repository

import (
	"context"
	"database/sql"

	"github.com/panxiao81/e5renew/internal/db"
)

type HealthRepository interface {
	PingContext(context.Context) error
	Stats() (sql.DBStats, error)
}

type healthRepository struct {
	store db.HealthStore
}

func NewHealthRepositoryWithEngine(engine db.Engine, conn db.DBTX) HealthRepository {
	return NewHealthRepository(db.NewHealthStore(engine, conn))
}

func NewHealthRepository(store db.HealthStore) HealthRepository {
	return &healthRepository{store: store}
}

func (r *healthRepository) PingContext(ctx context.Context) error {
	return r.store.PingContext(ctx)
}

func (r *healthRepository) Stats() (sql.DBStats, error) {
	return r.store.Stats()
}
