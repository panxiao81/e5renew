/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "e5renew",
	Short: "Microsoft E5 subscription renewal service",
	Long: `E5renew is a web service that helps maintain Microsoft Office 365 E5 subscriptions 
by automatically calling Microsoft Graph API endpoints at regular intervals.

The service provides a web interface for Azure AD authentication and schedules 
periodic API calls to keep subscriptions active.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateConfigForCommand(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.e5renew.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv() // read in environment variables that match
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	// Set environment variable prefix and enable automatic env reading
	viper.SetEnvPrefix("e5renew")

	// Load main config file
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".e5renew" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(".") // Also search in current directory
		viper.SetConfigType("yaml")
		viper.SetConfigName(".e5renew")
	}

	// Set default values
	viper.SetDefault("listen", ":8080")
	viper.SetDefault("debug", false)
	viper.SetDefault("database.engine", "mysql")
	viper.SetDefault("database.port", 3306)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func requiredConfigForCommand(cmd *cobra.Command) []string {
	if cmd == nil {
		return []string{"database.dsn"}
	}

	commandPath := cmd.CommandPath()
	if strings.Contains(commandPath, " migrate") {
		return []string{"database.dsn"}
	}

	return []string{
		"azureAD.tenant",
		"azureAD.clientID",
		"azureAD.clientSecret",
		"azureAD.redirectURL",
		"database.dsn",
	}
}

// validateConfigForCommand checks that required configuration values exist for the current command.
func validateConfigForCommand(cmd *cobra.Command) error {
	required := requiredConfigForCommand(cmd)
	var missing []string
	for _, key := range required {
		if value := viper.GetString(key); strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf(
			"missing required configuration values: %s. set these in config file or environment variables with E5RENEW_ prefix",
			strings.Join(missing, ", "),
		)
	}
	return nil
}
