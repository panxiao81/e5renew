package db

import (
	"context"
	"database/sql"
	"strings"
)

// WrapDBTXForEngine adapts placeholder style for the target engine.
// Generated queries in internal/db use postgres-style placeholders ($1, $2, ...).
func WrapDBTXForEngine(engine Engine, db DBTX) DBTX {
	if engine != EngineMySQL {
		return db
	}
	return &mysqlRebindingDB{db: db}
}

type mysqlRebindingDB struct {
	db DBTX
}

func (m *mysqlRebindingDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return m.db.ExecContext(ctx, rebindDollarToQuestion(query), args...)
}

func (m *mysqlRebindingDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return m.db.PrepareContext(ctx, rebindDollarToQuestion(query))
}

func (m *mysqlRebindingDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, rebindDollarToQuestion(query), args...)
}

func (m *mysqlRebindingDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return m.db.QueryRowContext(ctx, rebindDollarToQuestion(query), args...)
}

func (m *mysqlRebindingDB) PingContext(ctx context.Context) error {
	pinger, ok := m.db.(interface {
		PingContext(context.Context) error
	})
	if !ok {
		return nil
	}
	return pinger.PingContext(ctx)
}

func (m *mysqlRebindingDB) Stats() sql.DBStats {
	statser, ok := m.db.(interface {
		Stats() sql.DBStats
	})
	if !ok {
		return sql.DBStats{}
	}
	return statser.Stats()
}

func rebindDollarToQuestion(query string) string {
	var b strings.Builder
	b.Grow(len(query))

	inSingleQuote := false
	for i := 0; i < len(query); i++ {
		ch := query[i]

		if ch == '\'' {
			b.WriteByte(ch)
			if inSingleQuote && i+1 < len(query) && query[i+1] == '\'' {
				b.WriteByte(query[i+1])
				i++
				continue
			}
			inSingleQuote = !inSingleQuote
			continue
		}

		if ch == '$' && !inSingleQuote {
			j := i + 1
			for ; j < len(query) && query[j] >= '0' && query[j] <= '9'; j++ {
			}
			if j > i+1 {
				b.WriteByte('?')
				i = j - 1
				continue
			}
		}

		b.WriteByte(ch)
	}

	return b.String()
}
