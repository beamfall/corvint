package witnesscollapse

import (
	"errors"
	"fmt"
)

// Error is one stable package failure without source or cause-preimage text.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func failure(code, format string, values ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, values...)}
}

// CodeOf returns a witness-collapse error code or the empty string.
func CodeOf(err error) string {
	var target *Error
	if errors.As(err, &target) {
		return target.Code
	}
	return ""
}
