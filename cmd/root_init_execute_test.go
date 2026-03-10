package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestInitConfig_LoadsDefaultsAndEnv(t *testing.T) {
	oldCfg := cfgFile
	oldWd, _ := os.Getwd()
	t.Cleanup(func() {
		cfgFile = oldCfg
		_ = os.Chdir(oldWd)
		viper.Reset()
	})

	viper.Reset()
	cfgFile = ""
	t.Setenv("E5RENEW_DATABASE_DSN", "env-dsn")

	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmp, ".e5renew.yaml"), []byte("listen: ':9090'\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	initConfig()

	if got := viper.GetString("listen"); got != ":9090" {
		t.Fatalf("listen should come from config file, got %q", got)
	}
	if got := viper.GetString("database.engine"); got != "mysql" {
		t.Fatalf("database.engine default mismatch, got %q", got)
	}
	if got := viper.GetInt("database.port"); got != 3306 {
		t.Fatalf("database.port default mismatch, got %d", got)
	}
	if got := viper.GetString("database.dsn"); got != "env-dsn" {
		t.Fatalf("expected env var binding via replacer/prefix, got %q", got)
	}
}

func TestInitConfig_UsesExplicitConfigFile(t *testing.T) {
	oldCfg := cfgFile
	t.Cleanup(func() {
		cfgFile = oldCfg
		viper.Reset()
	})

	viper.Reset()
	path := filepath.Join(t.TempDir(), "custom.yaml")
	if err := os.WriteFile(path, []byte("debug: true\nlisten: ':8181'\n"), 0o600); err != nil {
		t.Fatalf("write custom config: %v", err)
	}

	cfgFile = path
	initConfig()

	if got := viper.GetString("listen"); got != ":8181" {
		t.Fatalf("explicit config not loaded, listen=%q", got)
	}
	if !viper.GetBool("debug") {
		t.Fatal("expected debug=true from explicit config")
	}
}

func TestExecute_ExitCodes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("subprocess exit code assertions are flaky on windows")
	}

	t.Run("error exits with code 1", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestExecuteHelper")
		cmd.Env = append(os.Environ(), "E5RENEW_EXEC_HELPER=1", "E5RENEW_EXEC_MODE=error")
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got err=%v", err)
		}
	})

	t.Run("success exits with code 0", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestExecuteHelper")
		cmd.Env = append(os.Environ(), "E5RENEW_EXEC_HELPER=1", "E5RENEW_EXEC_MODE=success")
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected success exit, got %v", err)
		}
	})
}

func TestExecuteHelper(t *testing.T) {
	if os.Getenv("E5RENEW_EXEC_HELPER") != "1" {
		return
	}

	original := rootCmd
	defer func() { rootCmd = original }()

	switch os.Getenv("E5RENEW_EXEC_MODE") {
	case "error":
		rootCmd = &cobra.Command{Use: "test", RunE: func(*cobra.Command, []string) error { return errors.New("boom") }}
	case "success":
		rootCmd = &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	default:
		t.Fatalf("unknown helper mode")
	}

	Execute()
}
