package cmd

import (
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/spf13/viper"
)

func TestGetMigrator_MissingDSN(t *testing.T) {
	viper.Set("database.dsn", "")
	_, err := getMigrator()
	if err == nil {
		t.Fatal("expected error for missing DSN")
	}
}

func TestGetMigrator_InvalidDSN(t *testing.T) {
	viper.Set("database.engine", "postgres")
	viper.Set("database.dsn", "postgres://bad:bad@127.0.0.1:1/none?sslmode=disable")
	_, err := getMigrator()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestModuleRoot(t *testing.T) {
	root := moduleRoot()
	if root == "" {
		t.Fatal("expected non-empty module root")
	}
	if !filepath.IsAbs(root) {
		t.Fatalf("expected absolute path, got %q", root)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("expected go.mod under module root: %v", err)
	}
}

func TestMigrationsPath_Order(t *testing.T) {
	tmp := t.TempDir()
	custom := filepath.Join(tmp, "custom")
	envDir := filepath.Join(tmp, "env")
	_ = os.MkdirAll(filepath.Join(custom, "postgres"), 0o755)
	_ = os.MkdirAll(filepath.Join(envDir, "postgres"), 0o755)

	t.Run("viper config takes precedence", func(t *testing.T) {
		t.Setenv("E5RENEW_MIGRATIONS_DIR", envDir)
		viper.Set("migrations.path", custom)
		got := migrationsPath(db.EnginePostgres)
		want := filepath.Join(custom, "postgres")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})

	t.Run("env used when config empty", func(t *testing.T) {
		t.Setenv("E5RENEW_MIGRATIONS_DIR", envDir)
		viper.Set("migrations.path", "")
		got := migrationsPath(db.EnginePostgres)
		want := filepath.Join(envDir, "postgres")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}

type stubMigrationDriver struct{}

func (stubMigrationDriver) Open(string) (database.Driver, error) { return nil, nil }
func (stubMigrationDriver) Close() error                         { return nil }
func (stubMigrationDriver) Lock() error                          { return nil }
func (stubMigrationDriver) Unlock() error                        { return nil }
func (stubMigrationDriver) Run(io.Reader) error                  { return nil }
func (stubMigrationDriver) SetVersion(int, bool) error           { return nil }
func (stubMigrationDriver) Version() (int, bool, error)          { return 0, false, nil }
func (stubMigrationDriver) Drop() error                          { return nil }

func TestGetMigrator_ErrorPaths(t *testing.T) {
	originalOpen := openMigrationDB
	originalMySQL := newMySQLMigrationDriver
	originalPostgres := newPostgresMigrationDriver
	originalNewMigrator := newMigratorWithDatabaseInstance
	originalEngine := viper.GetString("database.engine")
	originalDSN := viper.GetString("database.dsn")
	originalMigrationsPath := viper.GetString("migrations.path")
	t.Cleanup(func() {
		openMigrationDB = originalOpen
		newMySQLMigrationDriver = originalMySQL
		newPostgresMigrationDriver = originalPostgres
		newMigratorWithDatabaseInstance = originalNewMigrator
		viper.Set("database.engine", originalEngine)
		viper.Set("database.dsn", originalDSN)
		viper.Set("migrations.path", originalMigrationsPath)
	})

	viper.Set("database.dsn", "test-dsn")
	viper.Set("migrations.path", t.TempDir())

	t.Run("database open failure is wrapped", func(t *testing.T) {
		viper.Set("database.engine", "mysql")
		openMigrationDB = func(engine db.Engine, dsn string) (*sql.DB, error) {
			return nil, errors.New("open failed")
		}

		m, err := getMigrator()
		if m != nil {
			t.Fatal("expected nil migrator")
		}
		if err == nil || err.Error() != "failed to connect to database: open failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("mysql driver creation failure closes db", func(t *testing.T) {
		viper.Set("database.engine", "mysql")
		dbConn, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock db: %v", err)
		}
		defer dbConn.Close()
		openMigrationDB = func(engine db.Engine, dsn string) (*sql.DB, error) {
			return dbConn, nil
		}
		newMySQLMigrationDriver = func(conn *sql.DB) (database.Driver, error) {
			return nil, errors.New("mysql driver failed")
		}

		m, err := getMigrator()
		if m != nil {
			t.Fatal("expected nil migrator")
		}
		if err == nil || err.Error() != "failed to create MySQL driver: mysql driver failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("postgres driver creation failure closes db", func(t *testing.T) {
		viper.Set("database.engine", "postgres")
		dbConn, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock db: %v", err)
		}
		defer dbConn.Close()
		openMigrationDB = func(engine db.Engine, dsn string) (*sql.DB, error) {
			return dbConn, nil
		}
		newPostgresMigrationDriver = func(conn *sql.DB) (database.Driver, error) {
			return nil, errors.New("postgres driver failed")
		}

		m, err := getMigrator()
		if m != nil {
			t.Fatal("expected nil migrator")
		}
		if err == nil || err.Error() != "failed to create PostgreSQL driver: postgres driver failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("migrator creation failure is wrapped", func(t *testing.T) {
		viper.Set("database.engine", "mysql")
		dbConn, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to create sqlmock db: %v", err)
		}
		defer dbConn.Close()
		openMigrationDB = func(engine db.Engine, dsn string) (*sql.DB, error) {
			return dbConn, nil
		}
		newMySQLMigrationDriver = func(conn *sql.DB) (database.Driver, error) {
			return stubMigrationDriver{}, nil
		}
		newMigratorWithDatabaseInstance = func(sourceURL, databaseName string, driver database.Driver) (*migrate.Migrate, error) {
			return nil, errors.New("migrator create failed")
		}

		m, err := getMigrator()
		if m != nil {
			t.Fatal("expected nil migrator")
		}
		if err == nil || err.Error() != "failed to create migrator: migrator create failed" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
