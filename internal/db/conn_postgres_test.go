package db

import (
	"context"
	"os"
	"testing"
	"time"
)

const (
	postgresTestDSNEnv = "E5RENEW_TEST_POSTGRES_DSN"
	dockerPostgresCmd  = "docker run --rm -d --name e5renew-test-postgres -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=e5renew_test -p 15432:5432 postgres:15"
)

func requirePostgresDSN(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv(postgresTestDSNEnv)
	if dsn == "" {
		t.Skipf("Postgres integration tests require %s. Start Postgres with: %s", postgresTestDSNEnv, dockerPostgresCmd)
	}

	return dsn
}

func TestEngineDriverNameMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		engine Engine
		want   string
	}{
		{name: "mysql uses mysql driver", engine: EngineMySQL, want: "mysql"},
		{name: "postgres uses pgx driver", engine: EnginePostgres, want: "pgx"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.engine.DriverName()
			if err != nil {
				t.Fatalf("DriverName returned error for %q: %v", tt.engine, err)
			}
			if got != tt.want {
				t.Fatalf("DriverName(%q) = %q; want %q", tt.engine, got, tt.want)
			}
		})
	}
}

func TestEngineMigrationName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		engine Engine
		want   string
	}{
		{name: "mysql migration database", engine: EngineMySQL, want: "mysql"},
		{name: "postgres migration database", engine: EnginePostgres, want: "postgres"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.engine.MigrationDatabaseName(); got != tt.want {
				t.Fatalf("MigrationDatabaseName(%q) = %q; want %q", tt.engine, got, tt.want)
			}
		})
	}
}

func TestNewDBWithEngine_PostgresConnection(t *testing.T) {
	dsn := requirePostgresDSN(t)

	dbConn, err := NewDBWithEngine(EnginePostgres, dsn)
	if err != nil {
		t.Fatalf("failed to open postgres DB: %v", err)
	}
	defer dbConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := dbConn.PingContext(ctx); err != nil {
		t.Fatalf("expected PingContext success against Postgres (%s); got %v", dsn, err)
	}
}
