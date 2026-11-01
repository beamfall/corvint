package cli_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTCLV0860_CurrentRevisionRetryReplays: a retry of an applied
// --expected-revision current mutation with the same request ID, payload
// and --issued-at replays the recorded request instead of reading the
// advanced revision and refusing REQUEST_ID_CONFLICT.
func TestTCLV0860_CurrentRevisionRetryReplays(t *testing.T) {
	r, local, _ := papercutRepo(t)
	args := []string{"ticket", "prioritize", "--request-id", "cur-1", "--target", local, "--expected-revision", "current",
		"--issued-at", "2026-09-07T12:05:00Z", "--payload", `{"order":"1","priority":"P1"}`}
	first := atm(t, r.Root, nil, args...)
	if first.res.Outcome != wire.OutcomeOK {
		t.Fatalf("first: %s", first.stdout)
	}
	retry := atm(t, r.Root, nil, args...)
	if retry.res.Outcome != wire.OutcomeOK || hasCode(retry.res, wire.CodeRequestIDConflict) {
		t.Fatalf("retry of an applied current-revision mutation: %s", retry.stdout)
	}
	if a, b := field(first.res.Items[0], "resultingRevision").Str, field(retry.res.Items[0], "resultingRevision").Str; a != b {
		t.Fatalf("retry resultingRevision %q, first %q", b, a)
	}
	// A different payload under the same request ID still conflicts.
	other := append(append([]string{}, args[:len(args)-1]...), `{"order":"2","priority":"P1"}`)
	if x := atm(t, r.Root, nil, other...); x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("a changed payload replayed: %s", x.stdout)
	}
}

// claimedRepo is papercutRepo with the tree committed and the ticket
// claimed, so its attempt is live in the journal's reservation set.
