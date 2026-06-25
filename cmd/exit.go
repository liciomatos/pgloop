package cmd

import "fmt"

// ExitError carries the lint exit code without printing an additional message.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
