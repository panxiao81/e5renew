package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/alexedwards/scs/postgresstore"

	"github.com/panxiao81/e5renew/internal/db"
)

const (
	sessionPostgresEnv = "E5RENEW_TEST_POSTGRES_DSN"
	sessionDockerCmd   = "docker run --rm -d --name e5renew-test-postgres -e POSTGRES_PASSWORD=secret -e POSTGRES_DB=e5renew_test -p 15432:5432 postgres:15"
)

func requireSessionPostgresDSN(t *testing.T) string {
	t.Helper()

	dsn := os.Getenv(sessionPostgresEnv)
	if dsn == "" {
		t.Skipf("Session store integration tests require %s. Start Postgres with: %s", sessionPostgresEnv, sessionDockerCmd)
	}

	return dsn
}

func TestNewSessionStoreForEngine_Postgres(t *testing.T) {
	dsn := requireSessionPostgresDSN(t)

	dbConn, err := db.NewDBWithEngine(db.EnginePostgres, dsn)
	if err != nil {
		t.Fatalf("failed to open postgres DB for sessions: %v", err)
	}
	defer dbConn.Close()

	store, cleanup, err := newSessionStoreForEngine(db.EnginePostgres, dbConn, 5*time.Minute)
	if err != nil {
		t.Fatalf("expected session store for postgres engine, got error: %v", err)
	}
	if cleanup == nil {
		t.Fatalf("expected cleanup function to be returned")
	}
	defer cleanup()

	if _, ok := store.(*postgresstore.PostgresStore); !ok {
		t.Fatalf("postgres engine should create postgresstore, got %T", store)
	}
}
