package wire

import (
	"context"
	"errors"
	"fmt"
)

// ContextEnded maps a store write whose context expired or was cancelled (a
// write budget, a detached ceiling or an interrupt) to the retryable
// LOCK_TIMEOUT (ATR-V0-050): every context check in a write runs before that
// write commits, and the same command with the same --request-id replays any
// step that committed earlier, so a retry can succeed. A context error never reaches CodeOf as
// a bare error, which would report MALFORMED. errors.Is still reports the
// context error. An error that already carries a code, or that is not a
// context error, is returned unchanged.
func ContextEnded(err error, where string) error {
	if err == nil || !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
		return err
	}
	var e *Error
	if errors.As(err, &e) {
		return err
	}
	return fmt.Errorf("%w: %w", Errorf(CodeLockTimeout, where, "the store write's context ended (write budget, ceiling or interrupt) before the request completed; retry with the same --request-id"), err)
}
