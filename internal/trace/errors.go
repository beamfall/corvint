package trace

import "errors"

type driftError struct{ cause error }

func (err *driftError) Error() string { return err.cause.Error() }
func (err *driftError) Unwrap() error { return err.cause }

func markDrift(err error) error {
	if err == nil || IsDrift(err) {
		return err
	}
	return &driftError{cause: err}
}

// IsDrift reports a candidate-set, file-byte, pathname, or directory-binding
// change observed while a pinned trace read was in flight.
func IsDrift(err error) bool {
	var drift *driftError
	return errors.As(err, &drift)
}
