package utils

import (
	"fmt"
	"os"

	"github.com/jeeftor/qmp-controller/internal/logging"
)

// ErrorExitCode represents different types of errors with their exit codes
type ErrorExitCode int

const (
	ExitCodeGeneral     ErrorExitCode = 1
	ExitCodeValidation  ErrorExitCode = 1
	ExitCodeConnection  ErrorExitCode = 2
	ExitCodeFileSystem  ErrorExitCode = 3
	ExitCodePermission  ErrorExitCode = 4
	ExitCodeTimeout     ErrorExitCode = 5
)

// FatalError handles fatal errors with consistent logging and exit behavior
func FatalError(err error, context string) {
	logging.UserErrorf("%s: %v", context, err)
	os.Exit(int(ExitCodeGeneral))
}

// FatalErrorWithCode handles fatal errors with specific exit codes
func FatalErrorWithCode(err error, context string, exitCode ErrorExitCode) {
	logging.UserErrorf("%s: %v", context, err)
	os.Exit(int(exitCode))
}

// ValidationError handles argument validation errors with usage information
func ValidationError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(int(ExitCodeValidation))
}

// ValidationErrorWithUsage handles validation errors and shows usage
func ValidationErrorWithUsage(err error, usage string) {
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
	fmt.Fprintf(os.Stderr, "Usage: %s\n", usage)
	os.Exit(int(ExitCodeValidation))
}

// ConnectionError handles QMP connection errors consistently
func ConnectionError(vmid string, err error) {
	logging.UserErrorf("Failed to connect to VM %s: %v", vmid, err)
	os.Exit(int(ExitCodeConnection))
}

// FileSystemError handles file operation errors
func FileSystemError(operation string, path string, err error) {
	logging.UserErrorf("Failed to %s '%s': %v", operation, path, err)
	os.Exit(int(ExitCodeFileSystem))
}

// PermissionError handles permission-related errors
func PermissionError(operation string, resource string, err error) {
	logging.UserErrorf("Permission denied: %s %s: %v", operation, resource, err)
	os.Exit(int(ExitCodePermission))
}

// TimeoutError handles timeout-related errors
func TimeoutError(operation string, timeout string, err error) {
	logging.UserErrorf("Timeout during %s (timeout: %s): %v", operation, timeout, err)
	os.Exit(int(ExitCodeTimeout))
}

// Must is a helper that panics on error (for initialization code)
func Must(err error, context string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", context, err))
	}
}

// WarnOnError logs a warning for non-fatal errors
func WarnOnError(err error, context string) {
	if err != nil {
		logging.UserWarnf("Warning: %s: %v", context, err)
	}
}

// CheckError is a convenience function for common error checking patterns
func CheckError(err error, context string) {
	if err != nil {
		FatalError(err, context)
	}
}

// CheckErrorWithCode is like CheckError but with a specific exit code
func CheckErrorWithCode(err error, context string, exitCode ErrorExitCode) {
	if err != nil {
		FatalErrorWithCode(err, context, exitCode)
	}
}

// MultiError represents multiple errors that occurred
type MultiError struct {
	Errors  []error
	Context string
}

func (m *MultiError) Error() string {
	if len(m.Errors) == 0 {
		return "no errors"
	}
	if len(m.Errors) == 1 {
		return m.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred: %v (and %d more)", len(m.Errors), m.Errors[0], len(m.Errors)-1)
}

// NewMultiError creates a new MultiError
func NewMultiError(context string) *MultiError {
	return &MultiError{
		Context: context,
		Errors:  make([]error, 0),
	}
}

// Add adds an error to the MultiError
func (m *MultiError) Add(err error) {
	if err != nil {
		m.Errors = append(m.Errors, err)
	}
}

// HasErrors returns true if there are any errors
func (m *MultiError) HasErrors() bool {
	return len(m.Errors) > 0
}

// Check handles the MultiError by exiting if there are errors
func (m *MultiError) Check() {
	if m.HasErrors() {
		FatalError(m, m.Context)
	}
}
