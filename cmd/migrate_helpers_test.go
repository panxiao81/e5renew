package cmd

import (
	"database/sql"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-migrate/migrate/v4/database/mysql"
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

func TestNewMigrationDriver_DefaultsToMySQL(t *testing.T) {
	dbConn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock DB: %v", err)
	}
	defer dbConn.Close()
	defer func() {
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sqlmock expectations: %v", err)
		}
	}()

	mock.ExpectQuery(`SELECT DATABASE\(\)`).WillReturnRows(sqlmock.NewRows([]string{"DATABASE()"}).AddRow("e5renew_test"))
	mock.ExpectQuery(`SELECT GET_LOCK\(\?, 10\)`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK(?, 10)"}).AddRow(1))
	mock.ExpectQuery(`SHOW TABLES LIKE 'schema_migrations'`).WillReturnRows(sqlmock.NewRows([]string{"Tables_in_e5renew_test (schema_migrations)"}))
	mock.ExpectExec("CREATE TABLE `schema_migrations`").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT RELEASE_LOCK\(\?\)`).WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	driver, err := newMigrationDriver(db.EngineMySQL, dbConn)
	if err != nil {
		t.Fatalf("expected mysql migrate driver without error, got %v", err)
	}

	if _, ok := driver.(*mysql.Mysql); !ok {
		t.Fatalf("expected *mysql.Mysql driver, got %T", driver)
	}

	mock.ExpectQuery(`SELECT DATABASE\(\)`).WillReturnRows(sqlmock.NewRows([]string{"DATABASE()"}).AddRow("e5renew_test"))
	mock.ExpectQuery(`SELECT GET_LOCK\(\?, 10\)`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"GET_LOCK(?, 10)"}).AddRow(1))
	mock.ExpectQuery(`SHOW TABLES LIKE 'schema_migrations'`).WillReturnRows(sqlmock.NewRows([]string{"Tables_in_e5renew_test (schema_migrations)"}))
	mock.ExpectExec("CREATE TABLE `schema_migrations`").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT RELEASE_LOCK\(\?\)`).WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	driver, err = newMigrationDriver(db.Engine("unknown"), dbConn)
	if err != nil {
		t.Fatalf("expected default mysql migrate driver without error, got %v", err)
	}

	if _, ok := driver.(*mysql.Mysql); !ok {
		t.Fatalf("expected default *mysql.Mysql driver, got %T", driver)
	}
}

var _ *sql.DB
