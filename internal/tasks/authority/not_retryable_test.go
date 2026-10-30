package authority

import (
	"errors"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0908: annotating a NotRetryable wire.Error keeps the mark.
func TestWithCleanupPreservesNotRetryable(t *testing.T) {
	in := &wire.Error{Code: wire.CodeLockTimeout, Msg: "m", NotRetryable: true}
	if !wire.RetryForbidden(withCleanup(in, errors.New("cleanup"))) {
		t.Fatal("withCleanup dropped NotRetryable")
	}
}
