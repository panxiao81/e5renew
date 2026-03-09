package cmd

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"

	"github.com/panxiao81/e5renew/internal/db"
)

func newSessionStoreForEngine(engine db.Engine, conn *sql.DB, cleanupInterval time.Duration) (scs.Store, func(), error) {
	switch engine {
	case db.EnginePostgres:
		store := postgresstore.NewWithCleanupInterval(conn, cleanupInterval)
		return store, store.StopCleanup, nil
	case db.EngineMySQL:
		store := mysqlstore.NewWithCleanupInterval(conn, cleanupInterval)
		return store, store.StopCleanup, nil
	default:
		return nil, nil, fmt.Errorf("unsupported session store engine: %s", engine)
	}
}
