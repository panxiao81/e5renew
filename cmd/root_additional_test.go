package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestRequiredConfigForCommand_Nil(t *testing.T) {
	require.Equal(t, []string{"database.dsn"}, requiredConfigForCommand(nil))
}

func TestInitConfig_LoadsFileAndDefaults(t *testing.T) {
	origCfg := cfgFile
	t.Cleanup(func() {
		cfgFile = origCfg
		viper.Reset()
	})

	viper.Reset()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "custom.yaml")
	require.NoError(t, os.WriteFile(path, []byte("listen: ':9191'\ndatabase:\n  dsn: 'dsn-from-file'\n"), 0o644))

	cfgFile = path
	initConfig()

	require.Equal(t, ":9191", viper.GetString("listen"))
	require.Equal(t, "dsn-from-file", viper.GetString("database.dsn"))
	require.Equal(t, "mysql", viper.GetString("database.engine"))
	require.Equal(t, 3306, viper.GetInt("database.port"))
}

func TestExecuteExitsOnError(t *testing.T) {
	if os.Getenv("E5RENEW_EXECUTE_HELPER") == "1" {
		rootCmd = &cobra.Command{Use: "e5renew", RunE: func(*cobra.Command, []string) error { return assertErr{} }}
		Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "TestExecuteExitsOnError")
	cmd.Env = append(os.Environ(), "E5RENEW_EXECUTE_HELPER=1")
	err := cmd.Run()

	require.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok)
	require.Equal(t, 1, exitErr.ExitCode())
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
