package gokernel

import "fmt"

// Error is the stable machine-readable error returned by the experimental Go
// kernel. Callers must key behavior from Code, never Message.
type Error struct {
	Code    string
	Message string
}

func (err *Error) Error() string { return err.Message }

func newError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func wrapError(code, message string, cause error) *Error {
	return newError(code, fmt.Sprintf("%s: %v", message, cause))
}
