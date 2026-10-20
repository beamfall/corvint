package archive

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0908: annotating a NotRetryable wire.Error keeps the mark.
func TestPrefixWherePreservesNotRetryable(t *testing.T) {
	in := &wire.Error{Code: wire.CodeMalformed, Where: "/x", Msg: "m", NotRetryable: true}
	if !wire.RetryForbidden(prefixWhere(in, "f.json")) {
		t.Fatal("prefixWhere dropped NotRetryable")
	}
}
