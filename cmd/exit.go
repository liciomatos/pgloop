package cmd

import "fmt"

// ExitError carrega o exit code da análise sem imprimir mensagem adicional.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
