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
	Short: "List all patterns detected by lint with their codes",
	Long:  "Displays the full table of patterns detectable by pgloop lint, including the code used with --ignore.",
	Run:   runPatterns,
}

func runPatterns(cmd *cobra.Command, args []string) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(tw, "CODE\tRISK\tLOCK\tPATTERN\tNOTE")
	fmt.Fprintln(tw, "────\t────\t────\t───────\t────")

	for _, p := range lockmapper.AllPatterns() {
		lock := string(p.LockMode)
		if lock == "NONE" {
			lock = "—"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.Code, p.Risk, lock, p.Name, p.VersionNote)
	}

	tw.Flush()
	fmt.Println()
	fmt.Println("Use --ignore P2,P9 with the lint command to suppress specific patterns.")
	fmt.Println("Use --pg-version 14 for version-accurate diagnosis.")
}
