package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/panxiao81/e5renew/internal/db"
	"github.com/spf13/viper"
)

func TestMigrationsPath_UsesEnvOverride(t *testing.T) {
	t.Setenv("E5RENEW_MIGRATIONS_DIR", "/tmp/custom-migrations")
	viper.Set("migrations.path", "")

	got := migrationsPath(db.EnginePostgres)
	want := filepath.Join("/tmp/custom-migrations", "postgres")
	if got != want {
		t.Fatalf("expected env override path %q, got %q", want, got)
	}
}

func TestMigrationsPath_FindsRepoMigrations(t *testing.T) {
	viper.Set("migrations.path", "")
	_ = os.Unsetenv("E5RENEW_MIGRATIONS_DIR")

	got := migrationsPath(db.EngineMySQL)
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("expected migrations path to exist, got %q: %v", got, err)
	}
}
