package parentverify

import "errors"

var (
	ErrInvalid     = errors.New("go-live-parent-verifier: invalid authority request")
	ErrUnavailable = errors.New("go-live-parent-verifier: authority unavailable")
	ErrDrift       = errors.New("go-live-parent-verifier: authority drift")
	ErrLimit       = errors.New("go-live-parent-verifier: bound exceeded")
	ErrCommand     = errors.New("go-live-parent-verifier: command failed")
)
