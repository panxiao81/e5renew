package cmd

import (
	"os"
	"path/filepath"
	"testing"

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
