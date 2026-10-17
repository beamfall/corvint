package wire

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// TestATRV0050_ContextEndedIsRetryableLockTimeout: an ended write context is
// the retryable LOCK_TIMEOUT that still matches the context error; a coded
// error, an unrelated error and nil are unchanged.
func TestATRV0050_ContextEndedIsRetryableLockTimeout(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, context.Canceled, fmt.Errorf("lock wait: %w", context.DeadlineExceeded)} {
		err := ContextEnded(cause, "store write")
		if CodeOf(err) != CodeLockTimeout || !RetryOf(CodeOf(err)).Retryable || !errors.Is(err, cause) {
			t.Fatalf("%v: code %s", cause, CodeOf(err))
		}
		if CodeOf(cause) != CodeMalformed {
			t.Fatalf("a bare context error is no longer MALFORMED to CodeOf: %v", cause)
		}
	}
	coded := fmt.Errorf("%w: %w", Errorf(CodeSnapshotMoved, "x", "moved"), context.Canceled)
	if got := ContextEnded(coded, "w"); got != coded {
		t.Fatalf("a coded error changed: %v", got)
	}
	other := errors.New("other")
	if ContextEnded(other, "w") != other || ContextEnded(nil, "w") != nil {
		t.Fatal("a non-context error or nil changed")
	}
}
