package db

import (
	"context"
	"database/sql"
	"fmt"

	pgdb "github.com/panxiao81/e5renew/internal/db/postgres"
)

type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func New(db DBTX) *Queries {
	return &Queries{
		db: db,
		pg: pgdb.New(db),
	}
}

type Queries struct {
	db DBTX
	pg *pgdb.Queries
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

func (q *Queries) WithTx(tx *sql.Tx) *Queries {
	return New(tx)
}
