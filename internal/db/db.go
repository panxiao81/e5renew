package db

import (
	"context"
	"database/sql"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return NewQueries(db)
}

func NewQueries(db DBTX) *Queries {
	return NewQueriesWithEngine(EnginePostgres, db)
}

func newAPILogStore(engine Engine, db DBTX) apiLogStore {
	apilog, _, _ := newStores(engine, db)
	return apilog
}

func newUserTokenStore(engine Engine, db DBTX) userTokenStore {
	_, tokens, _ := newStores(engine, db)
	return tokens
}

func NewHealthStore(engine Engine, db DBTX) HealthStore {
	_, _, health := newStores(engine, db)
	return health
}

func NewWithEngine(engine Engine, db DBTX) *Queries {
	return NewQueriesWithEngine(engine, db)
}

func NewQueriesWithEngine(engine Engine, db DBTX) *Queries {
	apilog, tokens, health := newStores(engine, db)
	return &Queries{db: db, engine: engine, apilog: apilog, tokens: tokens, health: health}
}

type Queries struct {
	db     DBTX
	engine Engine
	apilog apiLogStore
	tokens userTokenStore
	health HealthStore
}

func (q *Queries) PingContext(ctx context.Context) error { return q.health.PingContext(ctx) }

func (q *Queries) Stats() (sql.DBStats, error) { return q.health.Stats() }

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return NewQueriesWithEngine(q.engine, tx)
}
