package output

import (
	"fmt"
	"strings"

	"github.com/liciomatos/pgloop/internal/lockmapper"
)

// Renderer formats and writes lint results to a specific destination.
type Renderer interface {
	// Render writes results to the configured destination.
	// Returns an error only on I/O failures (e.g. JSON encode fails).
	Render(file string, results []lockmapper.LintResult) error
}

// NewRenderer returns the Renderer for the requested format.
// showSuggestions only has effect for the "terminal" format.
func NewRenderer(format string, showSuggestions bool) (Renderer, error) {
	switch strings.ToLower(format) {
	case "terminal", "":
		return &terminalRenderer{showSuggestions: showSuggestions}, nil
	case "json":
		return &jsonRenderer{}, nil
	case "github":
		return &gitHubRenderer{}, nil
	default:
		return nil, fmt.Errorf("unknown format %q: use terminal, json, or github", format)
	}
}

type terminalRenderer struct {
	showSuggestions bool
}

func (tr *terminalRenderer) Render(file string, results []lockmapper.LintResult) error {
	renderTerminal(file, results, tr.showSuggestions)
	return nil
}

type jsonRenderer struct{}

func (jr *jsonRenderer) Render(file string, results []lockmapper.LintResult) error {
	return renderJSON(file, results)
}

type gitHubRenderer struct{}

func (gr *gitHubRenderer) Render(file string, results []lockmapper.LintResult) error {
	renderGitHub(file, results)
	return nil
}
