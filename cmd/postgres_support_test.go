package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestRunCommand_ContainsPostgresSessionStoreSelection(t *testing.T) {
	content, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatalf("failed to read cmd/run.go: %v", err)
	}

	source := string(content)
	if !strings.Contains(source, "postgresstore") {
		t.Fatalf("cmd/run.go should include postgresstore for postgres session backend")
	}
	if !strings.Contains(source, "database.engine") {
		t.Fatalf("cmd/run.go should branch session store by database.engine")
	}
}

func TestGetMigrator_PostgresDriverWithDocker(t *testing.T) {
	originalDSN := viper.GetString("database.dsn")
	originalEngine := viper.GetString("database.engine")
	t.Cleanup(func() {
		viper.Set("database.dsn", originalDSN)
		viper.Set("database.engine", originalEngine)
	})

	dsn := os.Getenv("E5RENEW_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("Postgres integration test requires E5RENEW_TEST_POSTGRES_DSN")
	}

	viper.Set("database.engine", "postgres")
	viper.Set("database.dsn", dsn)

	m, err := getMigrator()
	if err != nil {
		t.Fatalf("getMigrator should support postgres engine with Docker Postgres (%s): %v", dsn, err)
	}
	defer m.Close()
}

func TestMigrateCommand_ContainsPostgresDriverSelection(t *testing.T) {
	content, err := os.ReadFile("migrate.go")
	if err != nil {
		t.Fatalf("failed to read cmd/migrate.go: %v", err)
	}

	source := string(content)
	if !strings.Contains(source, "database/postgres") {
		t.Fatalf("cmd/migrate.go should import golang-migrate postgres driver")
	}
	if !strings.Contains(source, "postgres.WithInstance") {
		t.Fatalf("cmd/migrate.go should create postgres migration driver when engine=postgres")
	}
	if !strings.Contains(source, "database.engine") {
		t.Fatalf("cmd/migrate.go should branch migration driver by database.engine")
	}
}
