package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "pgloop",
	Short: "PostgreSQL management CLI — safe by default",
	Long: `pgloop is a local-first CLI for the daily developer loop with PostgreSQL.
It covers the full lifecycle of a database change safely, without leaving the terminal.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// version is injected at build time via: -ldflags "-X github.com/liciomatos/pgloop/cmd.version=v0.1.0"
var version = "dev"

func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(patternsCmd)
}

func initConfig() {
	viper.SetConfigName(".pgloop")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.config/pgloop")

	// Lint defaults — CLI flags take precedence when provided.
	viper.SetDefault("lint.format", "terminal")
	viper.SetDefault("lint.fail_on", "CRITICAL")
	viper.SetDefault("lint.suggestions", true)
	viper.SetDefault("lint.pg_version", 0)

	viper.ReadInConfig() // silently ignored — config file is optional
}
