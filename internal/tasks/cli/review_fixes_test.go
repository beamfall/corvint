package cli_test

import (
	"os"
	"path/filepath"
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
func claimedRepo(t *testing.T) (string, string) {
	t.Helper()
	r := fixture.TempRepo(t)
	policy := fixture.PolicyValue()
	budgets, _ := policy.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), wire.EncodeFile(policy))
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %s", x.stdout)
	}
	git(t, r.Root, "init", "-q", "-b", "main")
	fixture.Write(t, filepath.Join(r.Root, "src", "a.go"), []byte("a"))
	git(t, r.Root, "add", "src")
	git(t, r.Root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
	created := atm(t, r.Root, nil, "ticket", "create", "--request-id", "req-c", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %s", created.stdout)
	}
	id := field(created.res.Items[0], "ticketId").Str
	if c := atm(t, r.Root, nil, "claim", id, "--holder", "agent", "--request-id", "claim-1"); c.res.Outcome != wire.OutcomeOK {
		t.Fatalf("claim: %s", c.stdout)
	}
	return r.Root, id
}

// TestTCLV0867_RoadmapRowsMatchFacets: a live claimed ticket's roadmap row
// is judged with the reservation set, as its facets are, so the row and
// the byEligibility/byNextAction counts agree.
func TestTCLV0867_RoadmapRowsMatchFacets(t *testing.T) {
	root, _ := claimedRepo(t)
	x := atm(t, root, nil, "roadmap", "--facets")
	if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 || x.res.Facets == nil {
		t.Fatalf("roadmap: %s", x.stdout)
	}
	row := x.res.Items[0]
	elig, next := field(row, "eligibility").Str, field(row, "nextAction").Str
	if got := field(field(*x.res.Facets, "byEligibility"), elig).Str; got != "1" {
		t.Fatalf("row eligibility %s, facets %s: %s", elig, got, x.stdout)
	}
	if got := field(field(*x.res.Facets, "byNextAction"), next).Str; got != "1" {
		t.Fatalf("row nextAction %s, facets %s: %s", next, got, x.stdout)
	}
}

// TestTCLV0867_CountWarnsWhenReservationsUnreadable: a count-only list or
// search whose reservation set cannot be read still carries the store-wide
// ATTEMPT_LIVE warning, as the paged read does.
func TestTCLV0867_CountWarnsWhenReservationsUnreadable(t *testing.T) {
	r, _, _ := papercutRepo(t)
	if err := os.WriteFile(filepath.Join(r.Root, ".git", "taskman", "reservations.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paged := atm(t, r.Root, nil, "ticket", "list")
	if !strings.Contains(strings.Join(paged.res.Warnings, " "), "store-wide unknown "+wire.CodeAttemptLive) {
		t.Fatalf("paged list carries no ATTEMPT_LIVE warning, so the fixture does not exercise the failure: %s", paged.stdout)
	}
	for _, args := range [][]string{{"ticket", "list", "--count"}, {"ticket", "search", "--priority", "P2", "--count"}, {"roadmap", "--count"}} {
		x := atm(t, r.Root, nil, args...)
		if x.res.Outcome != wire.OutcomeOK || !strings.Contains(strings.Join(x.res.Warnings, " "), "store-wide unknown "+wire.CodeAttemptLive) {
			t.Fatalf("%v: %s", args, x.stdout)
		}
	}
}
