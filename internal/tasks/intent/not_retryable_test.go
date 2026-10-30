package intent

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0908: annotating a NotRetryable wire.Error keeps the mark.
func TestPrefixPreservesNotRetryable(t *testing.T) {
	in := &wire.Error{Code: wire.CodeMalformed, Where: "/x", Msg: "m", NotRetryable: true}
	if !wire.RetryForbidden(prefix(in, "f.json")) {
		t.Fatal("prefix dropped NotRetryable")
	}
}

func TestWithIntentRepairPreservesNotRetryable(t *testing.T) {
	r := &Repository{IntentFix: "fix it"}
	in := &wire.Error{Code: wire.CodeIntentBranchMismatch, Msg: "m", NotRetryable: true}
	out := r.WithIntentRepair(in)
	if out == error(in) {
		t.Fatal("WithIntentRepair did not annotate")
	}
	if !wire.RetryForbidden(out) {
		t.Fatal("WithIntentRepair dropped NotRetryable")
	}
}
