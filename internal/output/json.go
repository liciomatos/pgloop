package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/liciomatos/pgloop/internal/lockmapper"
)

type jsonAnnotation struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Level      string `json:"level"`
	LockMode   string `json:"lock_mode"`
	Pattern    int    `json:"pattern"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type jsonReport struct {
	File        string           `json:"file"`
	TotalIssues int              `json:"total_issues"`
	Critical    int              `json:"critical"`
	Warn        int              `json:"warn"`
	Issues      []jsonAnnotation `json:"issues"`
}

func renderJSON(file string, results []lockmapper.LintResult) error {
	critical, warn := countByLevel(results)
	issues := make([]jsonAnnotation, 0, len(results))

	for _, result := range results {
		issues = append(issues, jsonAnnotation{
			File:       file,
			Line:       result.Line,
			Level:      riskToLevel(result.Risk),
			LockMode:   string(result.LockMode),
			Pattern:    int(result.Pattern),
			Message:    result.Message,
			Suggestion: result.Suggestion,
		})
	}

	report := jsonReport{
		File:        file,
		TotalIssues: len(results),
		Critical:    critical,
		Warn:        warn,
		Issues:      issues,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}
	return nil
}
