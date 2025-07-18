package qmp

import "fmt"

// ErrNotConnected is returned when a method is called on a disconnected client
var ErrNotConnected = fmt.Errorf("not connected to QMP socket")

// ErrCommandFailed is returned when a QMP command fails
func ErrCommandFailed(cmd string, err error) error {
	return fmt.Errorf("command %q failed: %v", cmd, err)
}

// ErrInvalidResponse is returned when an invalid response is received
func ErrInvalidResponse(detail string) error {
	return fmt.Errorf("invalid response: %s", detail)
}
