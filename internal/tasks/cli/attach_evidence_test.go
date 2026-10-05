package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTEAV0001_TicketAttachEvidenceThroughTheCLI drives `ticket
// attach-evidence` against a native store: the receipt-backed write bumps
// only the revision, `ticket show` lists the entry, the record is otherwise
// unchanged, an identical retry replays, a repeated digest refuses, and
// `receipt audit` stays CONSISTENT.
func TestTEAV0001_TicketAttachEvidenceThroughTheCLI(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")
	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	record := func() wire.Value {
		t.Helper()
		x := atm(t, r.Root, nil, "ticket", "show", id)
		if x.res.Outcome != wire.OutcomeOK || len(x.res.Items) != 1 {
			t.Fatalf("ticket show: %+v", x.res)
		}
		return x.res.Items[0]
	}
	before := record()
	if _, ok := before.Obj.Get("attachedEvidence"); ok {
		t.Fatalf("a fresh ticket shows attachedEvidence: %s", wire.Encode(before))
	}

	d := string(wire.Sum([]byte("focused go test log")))
	payload := `{"evidence":["` + d + `"],"reason":"focused go test log for the fix"}`
	args := []string{"ticket", "attach-evidence", "--target", id, "--expected-revision", "1",
		"--request-id", "ev-1", "--issued-at", "2026-10-05T12:00:00Z", "--payload", payload}
	x := atm(t, r.Root, nil, args...)
	if x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("attach-evidence: %+v\n%s", x.res, x.stdout)
	}
	item := x.res.Items[0]
	if field(item, "outcome").Str != mutation.OutcomeCompleted || field(item, "resultingRevision").Str != "2" ||
		field(item, "resultingAcceptanceRevision").Str != "1" || field(item, "receipt").Str == "" {
		t.Fatalf("result: %s", wire.Encode(item))
	}

	after := record()
	shown := field(after, "attachedEvidence")
	if len(shown.Arr) != 1 || field(shown.Arr[0], "reason").Str != "focused go test log for the fix" ||
		field(shown.Arr[0], "evidence").Arr[0].Str != d || field(shown.Arr[0], "actor").Str == "" ||
		field(shown.Arr[0], "acceptanceRevision").Str != "1" {
		t.Fatalf("ticket show attachedEvidence: %s", wire.Encode(shown))
	}
	pre, post := field(before, "record"), field(after, "record")
	for _, k := range pre.Obj.SortedKeys() {
		switch k {
		case "revision", "previousRecordSha256", "updatedAt", "updatedBy":
			continue
		}
		if string(wire.Encode(field(pre, k))) != string(wire.Encode(field(post, k))) {
			t.Errorf("record member %s changed: %s -> %s", k, wire.Encode(field(pre, k)), wire.Encode(field(post, k)))
		}
	}
	if got := len(post.Obj.SortedKeys()); got != len(pre.Obj.SortedKeys())+1 {
		t.Errorf("record gained %d members; want only attachedEvidence", got-len(pre.Obj.SortedKeys()))
	}

	again := atm(t, r.Root, nil, args...)
	if again.res.Outcome != wire.OutcomeOK || !field(again.res.Items[0], "replayed").Bool || field(again.res.Items[0], "receipt").Str != "" {
		t.Fatalf("identical retry did not replay: %+v", again.res)
	}
	dup := atm(t, r.Root, nil, "ticket", "attach-evidence", "--target", id, "--expected-revision", "2",
		"--request-id", "ev-2", "--issued-at", "2026-10-05T12:00:00Z", "--payload", payload)
	if dup.res.Outcome == wire.OutcomeOK || !hasCode(dup.res, wire.CodeDuplicateID) {
		t.Fatalf("a repeated digest was accepted: %+v", dup.res)
	}

	audit := atm(t, r.Root, nil, "receipt", "audit")
	if audit.res.Outcome != wire.OutcomeOK || field(audit.res.Items[0], "structuralConsistency").Str != "CONSISTENT" ||
		field(audit.res.Items[0], "projectionAgreement").Str != "AGREES" {
		t.Fatalf("receipt audit: %+v", audit.res)
	}

	help := atm(t, r.Root, nil, "ticket", "attach-evidence", "--help")
	for _, want := range []string{"evidence", "reason", "acceptanceRevision", "DUPLICATE_ID"} {
		if !strings.Contains(string(help.stdout), want) {
			t.Errorf("attach-evidence help lacks %q", want)
		}
	}
}
