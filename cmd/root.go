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
	Short: "CLI de gerenciamento PostgreSQL — segurança por padrão",
	Long: `pgloop é um CLI local-first para o loop diário do desenvolvedor com PostgreSQL.
Cobre o ciclo completo de uma mudança de banco de forma segura, sem sair do terminal.`,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// version é injetada em build time via: -ldflags "-X github.com/liciomatos/pgloop/cmd.version=v0.1.0"
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
		fmt.Fprintln(os.Stderr, "erro:", err)
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

	// Valores padrão para lint — flags sobrescrevem se fornecidas.
	viper.SetDefault("lint.format", "terminal")
	viper.SetDefault("lint.fail_on", "CRITICAL")
	viper.SetDefault("lint.suggestions", true)
	viper.SetDefault("lint.pg_version", 0)

	viper.ReadInConfig() // falha silenciosa — config é opcional
}
