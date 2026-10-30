package importer_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/importer"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const issue502Now = wire.Timestamp("2026-10-04T00:00:00Z")

func obj(kv ...interface{}) wire.Value {
	o := wire.NewObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1].(wire.Value))
	}
	return wire.ObjectValue(o)
}

func str(s string) wire.Value { return wire.String(s) }

// issue502Export is a one-item export whose mapped ticket the edit may change.
func issue502Export(block string, edit func(*wire.Object)) []byte {
	t := obj(
		"acceptanceCriteria", wire.Strings([]string{"it exists"}),
		"archivedFrom", wire.Null(),
		"body", str(block),
		"capabilities", wire.Strings(nil),
		"completion", wire.Null(),
		"dependencies", wire.Array(),
		"dueDate", wire.Null(),
		"effects", obj("coverage", str("QUALIFIED"), "externalUnbounded", wire.Bool(false), "resources", wire.Array(), "touchPaths", wire.Strings(nil)),
		"estimateMinutes", wire.Null(),
		"executionClass", str("AUTONOMOUS"),
		"holds", wire.Array(),
		"kind", str("FEATURE"),
		"labels", wire.Strings([]string{"alias:bf-1"}),
		"milestone", wire.Null(),
		"order", str("1"),
		"owner", wire.Null(),
		"priority", str("P2"),
		"requiredGates", wire.Strings(nil),
		"requirementRefs", wire.Strings(nil),
		"status", str("OPEN"),
		"supersededBy", wire.Null(),
		"supersedes", wire.Null(),
		"title", str("Imported BF-1"),
	)
	if edit != nil {
		edit(t.Obj)
	}
	header := wire.Encode(obj("profile", str(importer.Profile), "sourceQueueId", str("queue:beamfall:main")))
	item := wire.Encode(obj("sourceItemId", str("BF-1"), "block", str(block), "ticket", t))
	return []byte(string(header) + "\n" + string(item) + "\n")
}

func issue502Store(t *testing.T, recs ...*ticket.Record) importer.Store {
	t.Helper()
	q := fixture.QueueValue()
	q.Obj.Set("canonicalWriter", str("ROADMAP"))
	queue, err := intent.DecodeQueue(wire.EncodeFile(q))
	if err != nil {
		t.Fatal(err)
	}
	policy, err := intent.DecodePolicy(fixture.PolicyBytes())
	if err != nil {
		t.Fatal(err)
	}
	inv, err := ticket.NewInventory(queue.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	return importer.Store{Queue: queue, Policy: policy, Tickets: inv}
}

func issue502Plan(t *testing.T, export []byte, recs ...*ticket.Record) *ticket.Record {
	t.Helper()
	exp, err := importer.Decode(export)
	if err != nil {
		t.Fatal(err)
	}
	posts, err := importer.Plan(exp, issue502Store(t, recs...), "operator", issue502Now)
	if err != nil || len(posts) != 1 {
		t.Fatalf("plan: %d posts, %v", len(posts), err)
	}
	rec, err := ticket.Decode(posts[0])
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func escalationsMember(rec *ticket.Record) string {
	v, ok := rec.Value().Obj.Get("escalations")
	if !ok {
		return ""
	}
	return string(wire.EncodeFile(v))
}

// TestIssue502_ReimportKeepsEscalations: a changed source block writes the
// next IMPORT revision and carries the tool-owned reference unchanged.
func TestIssue502_ReimportKeepsEscalations(t *testing.T) {
	first := issue502Plan(t, issue502Export("one\n", nil))
	carrier := first
	carrier.Revision = "2"
	prev := first.FileDigest()
	carrier.PreviousRecordSha256 = &prev
	carrier.Escalations = &ticket.EscalationRefs{Revision: "1", LastControlTicketRevision: "2", WorkRevision: "1", Entries: []ticket.EscalationRef{{
		RequestID: "q-1", OriginSha256: wire.Sum([]byte("origin")), HeadSha256: wire.Sum([]byte("origin")), Revision: "1", AcceptanceRevision: "1", Kind: "decision", State: "OPEN",
	}}}
	carrier, err := ticket.Decode(carrier.Encode())
	if err != nil {
		t.Fatalf("carrier: %v", err)
	}
	next := issue502Plan(t, issue502Export("one, changed\n", nil), carrier)
	if next.Revision != "3" || *next.PreviousRecordSha256 != carrier.FileDigest() {
		t.Fatalf("next revision: %+v", next)
	}
	if got := escalationsMember(next); got == "" || got != escalationsMember(carrier) {
		t.Fatalf("re-import changed the reference:\n%s\n%s", escalationsMember(carrier), got)
	}
}

// TestIssue502_ExportCannotCarryEscalations: an export item's ticket is
// closed to TicketKeys, so a foreign source cannot inject a reference.
func TestIssue502_ExportCannotCarryEscalations(t *testing.T) {
	export := issue502Export("one\n", func(o *wire.Object) { o.Set("escalations", obj("revision", str("1"))) })
	_, err := importer.Decode(export)
	if wire.CodeOf(err) != wire.CodeMalformed || !strings.Contains(err.Error(), "escalations") {
		t.Fatalf("export carrying escalations decoded: %v", err)
	}
}
