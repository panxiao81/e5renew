package cmd

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	"github.com/panxiao81/e5renew/internal/db"
)

func migrationDatabaseIdentifier(engine db.Engine) string {
	return engine.MigrationDatabaseName()
}

func newMigrationDriver(engine db.Engine, conn *sql.DB) (database.Driver, error) {
	switch engine {
	case db.EnginePostgres:
		return postgres.WithInstance(conn, &postgres.Config{})
	case db.EngineMySQL:
		fallthrough
	default:
		return mysql.WithInstance(conn, &mysql.Config{})
	}
}
