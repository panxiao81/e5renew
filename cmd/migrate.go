package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/panxiao81/e5renew/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
	Long:  `Manage database schema migrations using golang-migrate`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := getMigrator()
		if err != nil {
			return err
		}
		defer m.Close()

		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migration failed: %w", err)
		}

		fmt.Println("All migrations applied successfully")
		return nil
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down [steps]",
	Short: "Rollback migrations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		steps := 1
		if len(args) > 0 {
			var err error
			steps, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid steps argument: %w", err)
			}
			if steps < 1 {
				return fmt.Errorf("steps must be positive")
			}
		}

		m, err := getMigrator()
		if err != nil {
			return err
		}
		defer m.Close()

		if err := m.Steps(-steps); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("rollback failed: %w", err)
		}

		fmt.Printf("Rolled back %d migration(s) successfully\n", steps)
		return nil
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := getMigrator()
		if err != nil {
			return err
		}
		defer m.Close()

		version, dirty, err := m.Version()
		if err != nil {
			if err == migrate.ErrNilVersion {
				fmt.Println("No migrations have been applied")
				return nil
			}
			return fmt.Errorf("failed to get migration status: %w", err)
		}

		fmt.Printf("Current migration version: %d\n", version)
		if dirty {
			fmt.Println("Status: DIRTY (migration failed)")
		} else {
			fmt.Println("Status: OK")
		}

		return nil
	},
}

var migrateForceCmd = &cobra.Command{
	Use:   "force <version>",
	Short: "Force migration to a specific version (use with caution)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		version, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid version argument: %w", err)
		}

		m, err := getMigrator()
		if err != nil {
			return err
		}
		defer m.Close()

		if err := m.Force(version); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}

		fmt.Printf("Forced migration version to %d\n", version)
		return nil
	},
}

var migrateVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current migration version",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := getMigrator()
		if err != nil {
			return err
		}
		defer m.Close()

		version, dirty, err := m.Version()
		if err != nil {
			if err == migrate.ErrNilVersion {
				fmt.Println("No version")
				return nil
			}
			return fmt.Errorf("failed to get version: %w", err)
		}

		fmt.Printf("Version: %d", version)
		if dirty {
			fmt.Print(" (dirty)")
		}
		fmt.Println()

		return nil
	},
}

var openMigrationDB = db.NewDBWithEngine
var newMySQLMigrationDriver = func(conn *sql.DB) (database.Driver, error) {
	return mysql.WithInstance(conn, &mysql.Config{})
}
var newPostgresMigrationDriver = func(conn *sql.DB) (database.Driver, error) {
	return postgres.WithInstance(conn, &postgres.Config{})
}
var newMigratorWithDatabaseInstance = migrate.NewWithDatabaseInstance

func getMigrator() (*migrate.Migrate, error) {
	dsn := viper.GetString("database.dsn")
	if dsn == "" {
		return nil, fmt.Errorf("database DSN not configured")
	}

	engine := db.ParseEngine(viper.GetString("database.engine"))
	sqlDB, err := openMigrationDB(engine, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	var (
		driver          database.Driver
		driverName      string
		migrationSource string
	)
	switch engine {
	case db.EnginePostgres:
		driverName = "postgres"
		migrationSource = fmt.Sprintf("file://%s", migrationsPath(engine))
		driver, err = newPostgresMigrationDriver(sqlDB)
		if err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("failed to create PostgreSQL driver: %w", err)
		}
	default:
		driverName = "mysql"
		migrationSource = fmt.Sprintf("file://%s", migrationsPath(engine))
		driver, err = newMySQLMigrationDriver(sqlDB)
		if err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("failed to create MySQL driver: %w", err)
		}
	}

	m, err := newMigratorWithDatabaseInstance(
		migrationSource,
		driverName,
		driver,
	)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return m, nil
}

func migrationsPath(engine db.Engine) string {
	if configured := strings.TrimSpace(viper.GetString("migrations.path")); configured != "" {
		return filepath.Join(configured, string(engine))
	}
	if envDir := strings.TrimSpace(os.Getenv("E5RENEW_MIGRATIONS_DIR")); envDir != "" {
		return filepath.Join(envDir, string(engine))
	}

	candidates := []string{"/app/migrations"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "migrations"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "migrations"))
	}
	if root := moduleRoot(); root != "" {
		candidates = append(candidates, filepath.Join(root, "migrations"))
	}

	for _, base := range candidates {
		path := filepath.Join(base, string(engine))
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}

	return filepath.Join("/app/migrations", string(engine))
}

func moduleRoot() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	dir := filepath.Dir(filepath.Dir(filename))
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateForceCmd)
	migrateCmd.AddCommand(migrateVersionCmd)
	rootCmd.AddCommand(migrateCmd)
}
