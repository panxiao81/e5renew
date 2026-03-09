package cmd

import (
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4/database/postgres"

	"github.com/panxiao81/e5renew/internal/db"
)

const (
	migratePostgresEnv = "E5RENEW_TEST_POSTGRES_DSN"
	migrateDockerCmd   = "docker run --rm -d --name e5renew-test-postgres -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=e5renew_test -p 15432:5432 postgres:15"
)

func requireMigrationPostgresDSN(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv(migratePostgresEnv)
	if dsn == "" {
		t.Skipf("Migration integration tests require %s. Start Postgres with: %s", migratePostgresEnv, migrateDockerCmd)
	}

	return dsn
}

func TestMigrationDatabaseIdentifier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		engine db.Engine
		want   string
	}{
		{name: "mysql driver name", engine: db.EngineMySQL, want: "mysql"},
		{name: "postgres driver name", engine: db.EnginePostgres, want: "postgres"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := migrationDatabaseIdentifier(tt.engine); got != tt.want {
				t.Fatalf("migrationDatabaseIdentifier(%s) = %q; want %q", tt.engine, got, tt.want)
			}
		})
	}
}

func TestNewMigrationDriver_Postgres(t *testing.T) {
	dsn := requireMigrationPostgresDSN(t)

	dbConn, err := db.NewDBWithEngine(db.EnginePostgres, dsn)
	if err != nil {
		t.Fatalf("failed to open postgres DB for migrations: %v", err)
	}
	defer dbConn.Close()

	driver, err := newMigrationDriver(db.EnginePostgres, dbConn)
	if err != nil {
		t.Fatalf("expected postgres migrate driver without error, got %v", err)
	}

	if _, ok := driver.(*postgres.Postgres); !ok {
		t.Fatalf("expected *postgres.Postgres driver, got %T", driver)
	}
}
