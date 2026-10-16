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
