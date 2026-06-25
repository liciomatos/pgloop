package output

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/liciomatos/pgloop/internal/lockmapper"
)

const (
	colorRed    = lipgloss.Color("9")
	colorYellow = lipgloss.Color("11")
	colorGreen  = lipgloss.Color("10")
	colorGray   = lipgloss.Color("8")
	colorBlue   = lipgloss.Color("12")
)

var (
	styleCritical = lipgloss.NewStyle().Bold(true).Foreground(colorRed)
	styleWarn     = lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	styleOK       = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	styleDim      = lipgloss.NewStyle().Foreground(colorGray)
	styleBold     = lipgloss.NewStyle().Bold(true)
	styleCode     = lipgloss.NewStyle().Foreground(colorBlue)
)

func renderTerminal(file string, results []lockmapper.LintResult, showSuggestions bool) {
	if len(results) == 0 {
		fmt.Println(styleOK.Render("✓") + " Nenhum problema encontrado em " + styleBold.Render(file))
		return
	}

	fmt.Printf("\n%s  %s\n\n", styleBold.Render("pgloop lint"), styleDim.Render(file))

	for _, result := range results {
		icon, levelStyle := iconAndStyle(result.Risk)
		fmt.Printf("%s  %s\n", icon, levelStyle.Render(string(result.Risk)))

		if result.Line > 0 {
			fmt.Printf("   %s  linha %d\n", styleDim.Render("→"), result.Line)
		}

		if !result.Synthetic {
			fmt.Printf("   %s\n", styleCode.Render(truncate(result.Statement, 80)))
		}

		fmt.Printf("   %s  %s\n", styleDim.Render("Lock:"), string(result.LockMode))
		fmt.Printf("   %s\n", result.Message)

		if showSuggestions && result.Suggestion != "" {
			fmt.Printf("\n   %s\n", styleDim.Render("Sugestão:"))
			for _, line := range strings.Split(result.Suggestion, "\n") {
				fmt.Printf("   %s\n", styleCode.Render(line))
			}
		}
		fmt.Println()
	}

	critical, warn := countByLevel(results)
	summary := fmt.Sprintf("Total: %d problema(s)", len(results))
	if critical > 0 {
		summary += "  " + styleCritical.Render(fmt.Sprintf("%d CRITICAL", critical))
	}
	if warn > 0 {
		summary += "  " + styleWarn.Render(fmt.Sprintf("%d WARN", warn))
	}
	fmt.Println(summary)
}

func iconAndStyle(risk lockmapper.RiskLevel) (string, lipgloss.Style) {
	switch risk {
	case lockmapper.RiskCritical:
		return styleCritical.Render("✖"), styleCritical
	case lockmapper.RiskWarn:
		return styleWarn.Render("⚠"), styleWarn
	default:
		return styleOK.Render("✓"), styleOK
	}
}

func countByLevel(results []lockmapper.LintResult) (critical, warn int) {
	for _, result := range results {
		switch result.Risk {
		case lockmapper.RiskCritical:
			critical++
		case lockmapper.RiskWarn:
			warn++
		}
	}
	return
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
