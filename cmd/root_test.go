package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestValidateConfigForCommand(t *testing.T) {
	original := map[string]string{
		"azureAD.tenant":       viper.GetString("azureAD.tenant"),
		"azureAD.clientID":     viper.GetString("azureAD.clientID"),
		"azureAD.clientSecret": viper.GetString("azureAD.clientSecret"),
		"azureAD.redirectURL":  viper.GetString("azureAD.redirectURL"),
		"database.dsn":         viper.GetString("database.dsn"),
	}
	t.Cleanup(func() {
		for key, val := range original {
			viper.Set(key, val)
		}
	})

	setBaseConfig := func() {
		viper.Set("azureAD.tenant", "test-tenant")
		viper.Set("azureAD.clientID", "test-client-id")
		viper.Set("azureAD.clientSecret", "test-client-secret")
		viper.Set("azureAD.redirectURL", "http://localhost:8080/callback")
		viper.Set("database.dsn", "user:pass@tcp(localhost:3306)/db")
	}

	t.Run("RunCommandRequiresAzureAndDatabase", func(t *testing.T) {
		setBaseConfig()
		cmd := &cobra.Command{Use: "e5renew"}
		if err := validateConfigForCommand(cmd); err != nil {
			t.Fatalf("expected valid config, got error: %v", err)
		}
	})

	t.Run("MigrateOnlyRequiresDatabase", func(t *testing.T) {
		setBaseConfig()
		viper.Set("azureAD.tenant", "")
		viper.Set("azureAD.clientID", "")
		viper.Set("azureAD.clientSecret", "")
		viper.Set("azureAD.redirectURL", "")
		root := &cobra.Command{Use: "e5renew"}
		cmd := &cobra.Command{Use: "migrate"}
		root.AddCommand(cmd)
		if err := validateConfigForCommand(cmd); err != nil {
			t.Fatalf("expected migrate config to pass without azure fields, got error: %v", err)
		}
	})

	t.Run("MigrateMissingDatabaseFails", func(t *testing.T) {
		setBaseConfig()
		viper.Set("database.dsn", "")
		root := &cobra.Command{Use: "e5renew"}
		cmd := &cobra.Command{Use: "migrate"}
		root.AddCommand(cmd)
		if err := validateConfigForCommand(cmd); err == nil {
			t.Fatal("expected error when database.dsn is missing for migrate")
		}
	})

	t.Run("RunCommandMissingAzureFails", func(t *testing.T) {
		setBaseConfig()
		viper.Set("azureAD.clientID", "")
		cmd := &cobra.Command{Use: "e5renew"}
		if err := validateConfigForCommand(cmd); err == nil {
			t.Fatal("expected error when azureAD.clientID is missing")
		}
	})
}
