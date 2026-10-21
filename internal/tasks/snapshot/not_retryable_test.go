package snapshot

import (
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0908: annotating a NotRetryable wire.Error keeps the mark.
func TestAnnotatorsPreserveNotRetryable(t *testing.T) {
	in := &wire.Error{Code: wire.CodeSnapshotMoved, Where: "/x", Msg: "m", NotRetryable: true}
	if !wire.RetryForbidden(prefixWhere(in, "f.json")) {
		t.Fatal("prefixWhere dropped NotRetryable")
	}
	if !wire.RetryForbidden(AfterWait(in, time.Now(), 2)) {
		t.Fatal("AfterWait dropped NotRetryable")
	}
}
