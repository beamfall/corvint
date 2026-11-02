package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0250_DefaultedIssuedAtRetryNamesTheDifference (name kept: the CI
// shard slice pins it; it now proves TCL-V0-800 and TCL-V0-735) reproduces
// V1-1384 and V1-3958: `ticket create` without --issued-at defaults issuedAt
// to the current second, and issuedAt is part of the request digest. A
// byte-identical retry in a later second must replay the original ticketId
// with replayed:true, name the reused issuedAt, and write no second ticket; a
// retry at that named issuedAt replays too; a different payload under the same
// request ID still refuses REQUEST_ID_CONFLICT and is never attributed to
// issuedAt.
func TestCALV0250_DefaultedIssuedAtRetryNamesTheDifference(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	args := []string{"ticket", "create", "--request-id", "retry-1384", "--payload-stdin"}
	stdin := []byte(createPayloadJSON + "\n")
	first := atm(t, r.Root, stdin, args...)
	if first.res.Outcome != wire.OutcomeOK {
		t.Fatalf("first create: %s", first.stdout)
	}
	id := field(first.res.Items[0], "ticketId").Str
	start := time.Now().UTC().Truncate(time.Second)
	for time.Now().UTC().Truncate(time.Second).Equal(start) {
		time.Sleep(20 * time.Millisecond)
	}

	again := atm(t, r.Root, stdin, args...)
	if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "ticketId").Str != id {
		t.Fatalf("defaulted-issuedAt retry did not replay %s: %s", id, again.stdout)
	}
	if w := strings.Join(again.res.Warnings, " "); !strings.Contains(w, "reused the recorded issuedAt") {
		t.Fatalf("replay does not name the reused issuedAt: %s", again.stdout)
	}
	text := strings.Join(again.res.Warnings, " ")
	const marker = "is recorded with issuedAt "
	at := strings.Index(text, marker)
	if at < 0 || !strings.Contains(text, "replayed the recorded result") {
		t.Fatalf("replay does not name the recovered issuedAt: %s", text)
	}
	original := text[at+len(marker) : at+len(marker)+len("2006-01-02T15:04:05Z")]

	replay := atm(t, r.Root, stdin, append(args, "--issued-at", original)...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool || field(replay.res.Items[0], "ticketId").Str != id {
		t.Fatalf("retry at the named issuedAt did not replay %s: %s", id, replay.stdout)
	}
	if listed := atm(t, r.Root, nil, "ticket", "list"); len(listed.res.Items) != 1 {
		t.Fatalf("a retry wrote another ticket: %s", listed.stdout)
	}

	// A different payload under the same request ID is a real conflict: it is
	// never attributed to issuedAt alone and never replayed.
	changed := atm(t, r.Root, []byte(strings.Replace(createPayloadJSON, "Console ticket", "Other ticket", 1)), args...)
	if w := strings.Join(changed.res.Warnings, " "); changed.res.Outcome != wire.OutcomeRefused || !hasCode(changed.res, wire.CodeRequestIDConflict) ||
		strings.Contains(w, "only in issuedAt") || strings.Contains(w, "reused the recorded issuedAt") || !strings.Contains(w, "a retry replays only with the original --issued-at") {
		t.Fatalf("changed payload replayed or attributed to issuedAt: %s", changed.stdout)
	}
}

// TestTCLV0800_RefineRetryReplays: the same holds for another submitMutation
// verb, a refine retried without --issued-at.
func TestTCLV0800_RefineRetryReplays(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	created := atm(t, r.Root, []byte(createPayloadJSON+"\n"), "ticket", "create", "--request-id", "create-3958", "--payload-stdin")
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %s", created.stdout)
	}
	id := field(created.res.Items[0], "ticketId").Str
	refine := []string{"ticket", "refine", "--request-id", "refine-3958", "--target", id, "--expected-revision", "1", "--payload", `{"body":"Refined once"}`}
	if x := atm(t, r.Root, nil, refine...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("refine: %s", x.stdout)
	}
	start := time.Now().UTC().Truncate(time.Second)
	for time.Now().UTC().Truncate(time.Second).Equal(start) {
		time.Sleep(20 * time.Millisecond)
	}
	if x := atm(t, r.Root, nil, refine...); x.res.Outcome != wire.OutcomeOK || !field(x.res.Items[0], "replayed").Bool {
		t.Fatalf("refine retry did not replay: %s", x.stdout)
	}
}

// TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers reproduces V1-1438: a
// refine whose output was lost defaulted issuedAt; a later refine then moved
// the ticket's updatedAt more than the two-second lead past it, so neither
// createdAt nor updatedAt rebuilds the recorded digest. The writer recovers
// the original issuedAt from the recordedAt of the receipt the request-index
// entry names, and the plain retry replays (TCL-V0-800).
func TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers(t *testing.T) {
	t.Parallel() // idle-bound on fixed waits and independent of process state (V1-2366)
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}
	created := atm(t, r.Root, []byte(createPayloadJSON+"\n"), "ticket", "create", "--request-id", "create-1438", "--payload-stdin")
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("create: %s", created.stdout)
	}
	id := field(created.res.Items[0], "ticketId").Str
	waitSeconds := func(n int) {
		end := time.Now().UTC().Truncate(time.Second).Add(time.Duration(n) * time.Second)
		for time.Now().UTC().Before(end) {
			time.Sleep(20 * time.Millisecond)
		}
	}
	waitSeconds(3)
	lost := []string{"ticket", "refine", "--request-id", "refine-lost-1438", "--target", id, "--expected-revision", "1", "--payload", `{"body":"First refine whose output was lost"}`}
	if x := atm(t, r.Root, nil, lost...); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("first refine: %s", x.stdout)
	}
	waitSeconds(4)
	if x := atm(t, r.Root, nil, "ticket", "refine", "--request-id", "refine-later-1438", "--target", id, "--expected-revision", "2", "--payload", `{"body":"A later refine"}`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("later refine: %s", x.stdout)
	}
	again := atm(t, r.Root, nil, lost...)
	text := strings.Join(again.res.Warnings, " ")
	const marker = "is recorded with issuedAt "
	at := strings.Index(text, marker)
	if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || at < 0 || !strings.Contains(text, "reused the recorded issuedAt") {
		t.Fatalf("lost-output retry after a later update does not recover issuedAt and replay (TCL-V0-735): %s", again.stdout)
	}
	original := text[at+len(marker) : at+len(marker)+len("2006-01-02T15:04:05Z")]
	replay := atm(t, r.Root, nil, append(lost, "--issued-at", original)...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool {
		t.Fatalf("retry with the recovered --issued-at %s did not replay: %s", original, replay.stdout)
	}
}
