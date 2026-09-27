package transaction

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// roadmapInitialized is initialized over a ROADMAP-written fixture queue.
func roadmapInitialized(t *testing.T) Input {
	t.Helper()
	inv, e := NewInventory(nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	q := fixture.QueueValue()
	q.Obj.Set("canonicalWriter", wire.String("ROADMAP"))
	r := admin(Init, "init")
	r.Queue = wire.EncodeFile(q)
	r.Policy = fixture.PolicyBytes()
	r.PrimaryWorktree = "/fixture"
	result := Model(r, Input{Inventory: inv, Premise: FixtureNoRuntime, Replay: ReplayObservation{State: "ABSENT"}, Branch: "main", RecordedAt: timestamp})
	if result.Kind != "Transaction" {
		t.Fatalf("INIT: %+v", result)
	}
	c, e := CheckCapacity(result.Plan)
	if e != nil {
		t.Fatal(e)
	}
	return Input{Inventory: c.Final, Head: result.Plan.Head(), Queue: r.Queue, Policy: r.Policy, Reservations: emptyReservations(fixture.QueueID), Premise: FixtureNoRuntime, Branch: "main", Replay: ReplayObservation{State: "ABSENT"}, RecordedAt: timestamp}
}

func imported(local, block string) *ticket.Record {
	rec := fixture.Ticket(local)
	item, sum := local, wire.Sum([]byte(block))
	rec.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "queue:beamfall:main", SourceItemID: &item, SourceRevisionSha256: &sum}
	rec.ShadowOverlay = true
	return rec
}

func importRequest(records ...*ticket.Record) Request {
	r := admin(ImportApply, "import-1")
	for _, rec := range records {
		r.Records = append(r.Records, rec.Encode())
	}
	return r
}

func refusedWith(t *testing.T, name string, r Result, code string) {
	t.Helper()
	if r.Kind == "Transaction" || len(r.Outcome.Codes) == 0 || r.Outcome.Codes[0] != code {
		t.Errorf("%s: %s %+v, want %s", name, r.Kind, r.Outcome, code)
	}
}

func TestCTSV0003_ImportApplyPostsAndChainsRevisions(t *testing.T) {
	in := roadmapInitialized(t)
	first := Model(importRequest(imported("BF-1", "one"), imported("BF-2", "two")), in)
	if first.Kind != "Transaction" || first.Plan == nil {
		t.Fatalf("import: %+v", first)
	}
	rc, e := snapshot.DecodeReceipt(first.Plan.receipt)
	if e != nil || rc.Kind != ImportApply || rc.TicketID != nil || len(rc.Post) != 3 {
		t.Fatalf("receipt: %+v %v", rc, e)
	}
	in = commitModel(t, in, first.Plan)
	for _, local := range []string{"BF-1", "BF-2"} {
		in.CanonicalTickets = append(in.CanonicalTickets, bytes.Clone(first.Plan.posts["intent/tickets/"+local+".json"]))
	}
	pre := imported("BF-1", "one")
	next := imported("BF-1", "one, changed")
	next.Revision, next.AcceptanceRevision = "2", "2"
	digest := pre.FileDigest()
	next.PreviousRecordSha256 = &digest
	r := importRequest(next)
	r.RequestID = "import-2"
	if second := Model(r, in); second.Kind != "Transaction" {
		t.Fatalf("next revision: %+v", second)
	}
	restart := imported("BF-1", "one, changed")
	r = importRequest(restart)
	r.RequestID = "import-3"
	refusedWith(t, "revision 1 over an existing record", Model(r, in), wire.CodeMalformed)
	other := imported("BF-1", "other")
	s := "BF-9"
	other.Source.SourceItemID = &s
	other.Revision, other.PreviousRecordSha256 = "2", &digest
	r = importRequest(other)
	r.RequestID = "import-4"
	refusedWith(t, "different source item", Model(r, in), wire.CodeDuplicateID)
}

func TestCTSV0003_ImportApplyRefusals(t *testing.T) {
	in := roadmapInitialized(t)
	native := fixture.Ticket("BF-1")
	held := withTicket(t, in, native, native.Encode())
	refusedWith(t, "native holder", Model(importRequest(imported("BF-1", "one")), held), wire.CodeDuplicateID)
	nativeQueue, _ := initialized(t)
	refusedWith(t, "NATIVE writer", Model(importRequest(imported("BF-1", "one")), nativeQueue), wire.CodeUnsupported)
	notShadow := imported("BF-1", "one")
	notShadow.ShadowOverlay = false
	refusedWith(t, "non-shadow record", Model(importRequest(notShadow), in), wire.CodeMalformed)
	late := imported("BF-1", "one")
	late.Revision = "2"
	refusedWith(t, "new ticket past revision 1", Model(importRequest(late), in), wire.CodeMalformed)
	if _, e := Digest(importRequest()); e == nil {
		t.Error("empty batch digested")
	}
	if _, e := Digest(importRequest(imported("BF-2", "two"), imported("BF-1", "one"))); e == nil {
		t.Error("unsorted batch digested")
	}
	stray := admin(Pause, "pause")
	stray.Records = [][]byte{imported("BF-1", "one").Encode()}
	if _, e := Digest(stray); e == nil {
		t.Error("records on a non-import request digested")
	}
}
