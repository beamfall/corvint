package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0250_DefaultedIssuedAtRetryNamesTheDifference reproduces
// V1-1384: `ticket create` without --issued-at defaults issuedAt to the
// current second, and issuedAt is part of the request digest, so a byte-
// identical retry in a later second is REQUEST_ID_CONFLICT (TM-V0-006). The
// refusal must name issuedAt as the differing non-payload input, the original
// issuedAt and ticket, and the retry that replays; that retry must replay the
// original ticketId with replayed:true and write no second ticket.
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
	if again.res.Outcome != wire.OutcomeRefused || len(again.res.Codes) != 1 || again.res.Codes[0] != wire.CodeRequestIDConflict {
		t.Fatalf("defaulted-issuedAt retry: %s", again.stdout)
	}
	text := strings.Join(again.res.Warnings, " ")
	const marker = "differs from the recorded request only in issuedAt: the original was "
	at := strings.Index(text, marker)
	if at < 0 || !strings.Contains(text, "request retry-1384 is recorded for ticket "+id) {
		t.Fatalf("refusal does not name the differing input: %s", text)
	}
	original := text[at+len(marker) : at+len(marker)+len("2006-01-02T15:04:05Z")]
	if !strings.Contains(text, "retry with --issued-at "+original+" to replay") {
		t.Fatalf("refusal does not name the replaying retry: %s", text)
	}

	replay := atm(t, r.Root, stdin, append(args, "--issued-at", original)...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool || field(replay.res.Items[0], "ticketId").Str != id {
		t.Fatalf("named retry did not replay %s: %s", id, replay.stdout)
	}
	if listed := atm(t, r.Root, nil, "ticket", "list"); len(listed.res.Items) != 1 {
		t.Fatalf("a retry wrote another ticket: %s", listed.stdout)
	}

	// A different payload under the same request ID is a real conflict: it is
	// never attributed to issuedAt alone.
	changed := atm(t, r.Root, []byte(strings.Replace(createPayloadJSON, "Console ticket", "Other ticket", 1)), args...)
	if w := strings.Join(changed.res.Warnings, " "); changed.res.Outcome != wire.OutcomeRefused || strings.Contains(w, marker) || !strings.Contains(w, "a retry replays only with the original --issued-at") {
		t.Fatalf("changed payload attributed to issuedAt: %s", changed.stdout)
	}
}

// TestATRV0275_LostOutputRetryAfterLaterUpdateRecovers reproduces V1-1438: a
// refine whose output was lost defaulted issuedAt; a later refine then moved
// the ticket's updatedAt more than the two-second lead past it, so neither
// createdAt nor updatedAt rebuilds the recorded digest. The refusal recovers
// the original issuedAt from the recordedAt of the receipt the request-index
// entry names, and the retry with that --issued-at replays.
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
	const marker = "differs from the recorded request only in issuedAt: the original was "
	at := strings.Index(text, marker)
	if !hasCode(again.res, wire.CodeRequestIDConflict) || at < 0 {
		t.Fatalf("lost-output retry after a later update does not recover issuedAt: %s", again.stdout)
	}
	original := text[at+len(marker) : at+len(marker)+len("2006-01-02T15:04:05Z")]
	replay := atm(t, r.Root, nil, append(lost, "--issued-at", original)...)
	if replay.res.Outcome != wire.OutcomeOK || !field(replay.res.Items[0], "replayed").Bool {
		t.Fatalf("retry with the recovered --issued-at %s did not replay: %s", original, replay.stdout)
	}
}
