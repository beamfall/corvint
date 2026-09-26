package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// createPayloadJSON is the closed §3.3 CREATE payload as canonical JSON:
// keys sorted, no insignificant whitespace, exactly as the wire codec admits.
const createPayloadJSON = `{"acceptanceCriteria":["it exists"],"body":null,"capabilities":[],` +
	`"dependencies":[],"dueDate":null,"effects":{"coverage":"QUALIFIED","externalUnbounded":false,` +
	`"resources":[],"touchPaths":[]},"estimateMinutes":null,"executionClass":"AUTONOMOUS",` +
	`"kind":"FEATURE","labels":[],"milestone":null,"order":"0","owner":null,"priority":"P2",` +
	`"requiredGates":[],"requirementRefs":[],"source":{"kind":"NATIVE","sourceItemId":null,` +
	`"sourceQueueId":"queue:acme:main","sourceRevisionSha256":null},"supersededBy":null,` +
	`"supersedes":null,"title":"Console ticket"}`

// TestTMV0008_AS02_TicketCreateIsVisibleToTheReadVerbs is TCP-02b end to
// end: a ticket the CLI creates is committed to the journal and then listed
// by the ordinary read path, which is what a board needs in order to have
// anything to show.
func TestTMV0008_AS02_TicketCreateIsVisibleToTheReadVerbs(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	if x := atm(t, r.Root, nil, "init"); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("init: %+v", x.res)
	}

	empty := atm(t, r.Root, nil, "ticket", "list")
	if len(empty.res.Items) != 0 {
		t.Fatalf("a fresh queue already lists %d tickets", len(empty.res.Items))
	}

	created := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON)
	if created.res.Outcome != wire.OutcomeOK {
		t.Fatalf("ticket create: %+v", created.res)
	}
	id := field(created.res.Items[0], "ticketId").Str
	if !strings.HasPrefix(id, "ticket:acme:main:") {
		t.Fatalf("ticketId = %q, so the caller cannot learn its allocated id", id)
	}
	if got := field(created.res.Items[0], "receipt").Str; got != "000000000002.json" {
		t.Errorf("receipt = %q, want the second receipt", got)
	}

	listed := atm(t, r.Root, nil, "ticket", "list")
	if listed.res.Outcome != wire.OutcomeOK || len(listed.res.Items) != 1 {
		t.Fatalf("ticket list after create: %+v", listed.res)
	}
	if got := field(listed.res.Items[0], "ticketId").Str; got != id {
		t.Errorf("listed %q, created %q", got, id)
	}
}

// TestTMV0006_AS03_TicketCreateRetryReplays checks the idempotency key at
// the CLI boundary: re-running the identical command replays instead of
// creating a second ticket.
func TestTMV0006_AS03_TicketCreateRetryReplays(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())
	atm(t, r.Root, nil, "init")

	args := []string{"ticket", "create", "--request-id", "req-1",
		"--issued-at", "2026-09-07T12:00:00Z", "--payload", createPayloadJSON}
	first := atm(t, r.Root, nil, args...)
	if first.res.Outcome != wire.OutcomeOK {
		t.Fatalf("first create: %+v", first.res)
	}
	again := atm(t, r.Root, nil, args...)
	if again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("retry: %+v", again.res)
	}
	if field(first.res.Items[0], "ticketId").Str != field(again.res.Items[0], "ticketId").Str {
		t.Fatal("replay lost allocated ticket identity")
	}
	if !field(again.res.Items[0], "replayed").Bool {
		t.Error("an identical retry was not replayed")
	}
	if got := field(again.res.Items[0], "receipt").Str; got != "" {
		t.Errorf("a replay reported receipt %q, so it wrote a second one", got)
	}
	listed := atm(t, r.Root, nil, "ticket", "list")
	if len(listed.res.Items) != 1 {
		t.Errorf("the queue holds %d tickets after a replayed retry, want 1", len(listed.res.Items))
	}
}

// TestTMV0008_AS07_MutationsRefuseOnAnUninitializedStore keeps the
// uninitialized refusal honest: a mutation verb must report that there is no
// store, not create one implicitly.
func TestTMV0008_AS07_MutationsRefuseOnAnUninitializedStore(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.Write(t, filepath.Join(r.IntentDir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(r.IntentDir, "policy.json"), fixture.PolicyBytes())

	x := atm(t, r.Root, nil, "ticket", "create",
		"--request-id", "req-1", "--payload", createPayloadJSON)
	if x.res.Outcome == wire.OutcomeOK {
		t.Fatal("a mutation succeeded against an uninitialized store")
	}
	if len(x.res.Codes) == 0 || x.res.Codes[0] != wire.CodeUninitialized {
		t.Errorf("codes = %v, want %s", x.res.Codes, wire.CodeUninitialized)
	}
}
