package doccompiler

import "fmt"

type Error struct {
	Code    string
	Message string
}

func (problem *Error) Error() string { return problem.Code + ": " + problem.Message }

func failure(code, format string, values ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, values...)}
}
