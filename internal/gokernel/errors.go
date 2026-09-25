package gokernel

import "fmt"

// Error is the stable machine-readable error returned by the experimental Go
// kernel. Callers must key behavior from Code, never Message. ReasonClass is
// the closed status refusal class of decision 0383, set only on a status
// refusal.
type Error struct {
	Code        string
	Message     string
	ReasonClass string
}

func (err *Error) Error() string { return err.Message }

func newError(code, message string) *Error {
	return &Error{Code: code, Message: message}
}

func wrapError(code, message string, cause error) *Error {
	return newError(code, fmt.Sprintf("%s: %v", message, cause))
}
