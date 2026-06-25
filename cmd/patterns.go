package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/liciomatos/pgloop/internal/lockmapper"
	"github.com/spf13/cobra"
)

var patternsCmd = &cobra.Command{
	Use:   "patterns",
	Short: "Lista todos os padrões detectados pelo lint com seus códigos",
	Long:  "Exibe a tabela completa de padrões detectáveis pelo pgloop lint, incluindo o código usado em --ignore.",
	Run:   runPatterns,
}

func runPatterns(cmd *cobra.Command, args []string) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "CÓDIGO\tRISCO\tLOCK\tPADRÃO\tOBSERVAÇÃO")
	fmt.Fprintln(tw, "──────\t─────\t────\t──────\t──────────")

	for _, p := range lockmapper.AllPatterns() {
		lock := string(p.LockMode)
		if lock == "NONE" {
			lock = "—"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Code, p.Risk, lock, p.Name, p.VersionNote)
	}

	tw.Flush()
	fmt.Println()
	fmt.Println("Use --ignore P2,P9 no comando lint para suprimir padrões específicos.")
	fmt.Println("Use --pg-version 14 para diagnóstico preciso por versão do PostgreSQL.")
}
