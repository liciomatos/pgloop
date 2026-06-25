package output

import (
	"fmt"
	"strings"

	"github.com/liciomatos/pgloop/internal/lockmapper"
)

// Renderer formata e escreve resultados de lint para um destino específico.
type Renderer interface {
	// Render escreve os resultados no destino configurado.
	// Retorna erro apenas em falhas de I/O (ex: encode JSON falha).
	Render(file string, results []lockmapper.LintResult) error
}

// NewRenderer retorna o Renderer correspondente ao formato solicitado.
// showSuggestions só tem efeito no formato "terminal".
func NewRenderer(format string, showSuggestions bool) (Renderer, error) {
	switch strings.ToLower(format) {
	case "terminal", "":
		return &terminalRenderer{showSuggestions: showSuggestions}, nil
	case "json":
		return &jsonRenderer{}, nil
	case "github":
		return &gitHubRenderer{}, nil
	default:
		return nil, fmt.Errorf("formato desconhecido %q: use terminal, json ou github", format)
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
