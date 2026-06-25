package output

import (
	"fmt"

	"github.com/liciomatos/pgloop/internal/lockmapper"
)

func renderGitHub(file string, results []lockmapper.LintResult) {
	for _, result := range results {
		line := result.Line
		if line < 1 {
			line = 1
		}
		fmt.Printf("::%s file=%s,line=%d::[pgloop P%d] %s\n",
			riskToLevel(result.Risk), file, line, result.Pattern, result.Message)
	}
}
