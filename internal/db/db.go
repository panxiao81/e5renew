package db

import (
	"context"
	"database/sql"
	"fmt"

	mydb "github.com/panxiao81/e5renew/internal/db/mysql"
	pgdb "github.com/panxiao81/e5renew/internal/db/postgres"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return NewWithEngine(EnginePostgres, db)
}

func NewWithEngine(engine Engine, db DBTX) *Queries {
	queries := &Queries{db: db, engine: engine}

	switch engine {
	case EngineMySQL:
		mysqlQueries := mydb.New(db)
		adapter := &mysqlAdapter{q: mysqlQueries}
		queries.apilog = adapter
		queries.tokens = adapter
	default:
		pgQueries := pgdb.New(db)
		queries.apilog = pgQueries
		queries.tokens = pgQueries
	}

	return queries
}

type Queries struct {
	db     DBTX
	engine Engine
	apilog APILogStore
	tokens UserTokenStore
}

func (q *Queries) PingContext(ctx context.Context) error {
	pinger, ok := q.db.(interface {
		PingContext(context.Context) error
	})
	if !ok {
		return fmt.Errorf("underlying DB does not implement PingContext")
	}
	return pinger.PingContext(ctx)
}

func (q *Queries) Stats() (sql.DBStats, error) {
	statser, ok := q.db.(interface {
		Stats() sql.DBStats
	})
	if !ok {
		return sql.DBStats{}, fmt.Errorf("underlying DB does not expose Stats")
	}
	return statser.Stats(), nil
}

func (q *Queries) WithTx(tx *sql.Tx) Database {
	return NewWithEngine(q.engine, tx)
}
