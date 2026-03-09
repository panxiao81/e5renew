package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ErrUnsupportedEngine is returned when the requested database engine is not recognized.
var ErrUnsupportedEngine = errors.New("unsupported database engine")

// Engine represents a supported database engine.
type Engine string

const (
	EngineMySQL    Engine = "mysql"
	EnginePostgres Engine = "postgres"
)

// ParseEngine normalizes a string into a known Engine, defaulting to MySQL.
func ParseEngine(value string) Engine {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "postgres", "postgresql", "pg":
		return EnginePostgres
	default:
		return EngineMySQL
	}
}

// DriverName returns the registered sql driver name for this engine.
func (e Engine) DriverName() (string, error) {
	switch e {
	case EnginePostgres:
		return "pgx", nil
	case EngineMySQL:
		fallthrough
	default:
		return "mysql", nil
	}
}

// MigrationDatabaseName returns the golang-migrate identifier for this engine.
func (e Engine) MigrationDatabaseName() string {
	switch e {
	case EnginePostgres:
		return "postgres"
	default:
		return "mysql"
	}
}

// NewDBWithEngine opens a sql.DB configured for the requested engine.
func NewDBWithEngine(engine Engine, dsn string) (*sql.DB, error) {
	driver, err := engine.DriverName()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedEngine, engine)
	}
	return sql.Open(driver, dsn)
}

// NewDB creates a mysql connection to preserve the previous API contract.
func NewDB(dsn string) (*sql.DB, error) {
	return NewDBWithEngine(EngineMySQL, dsn)
}
