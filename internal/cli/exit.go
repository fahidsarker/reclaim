package cli

import "fmt"

// ExitError is an error that carries a process exit code.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Message
}

func exitErrorf(code int, format string, args ...any) error {
	return &ExitError{Code: code, Message: fmt.Sprintf(format, args...)}
}
