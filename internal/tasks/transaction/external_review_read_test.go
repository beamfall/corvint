package transaction

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue504Chain records PASS, RETURN, RESUBMIT and a second RETURN on G1 and
// returns the head reference, the event store, the newest-first events, the
// current binding and the head reference after each step.
func issue504Chain(t *testing.T) (snapshot.ExternalReviewRef, map[wire.Digest][]byte, [][]byte, ExternalReviewBinding, []snapshot.ExternalReviewRef) {
	t.Helper()
	q, o := issue504Fixture(t)
	blobs := map[wire.Digest][]byte{}
	var events [][]byte
	var heads []snapshot.ExternalReviewRef
	step := func() ExternalReviewTransition {
		issue504Context(t, q, &o)
		x := issue504Apply(t, q, o)
		blobs[x.Ref.Head] = x.Event
		events = append([][]byte{x.Event}, events...)
		heads = append(heads, *x.Ref)
		return x
	}
	x := step()
	issue504Next(t, &q, &o, x)
	ret := "RETURN"
	q.Verdict = &ret
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "fix", Text: "No G1 PASS is claimed"}}
	x = step()
	issue504Next(t, &q, &o, x)
	head := x.Ref.Head
	q.Action, q.Verdict, q.PriorReturn = "RESUBMIT", nil, &head
	q.AuthorLease = &snapshot.ExternalReviewLease{AttemptID: q.Subject.AttemptID, Generation: "2", Holder: "author"}
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "repair", Text: "## G1 RETURN repair dispositions"}}
	o.Actor = mutation.Binding{ID: "author", Role: "WORKER"}
	o.Author = ExternalReviewLeaseObservation{State: "LIVE", TicketID: q.TicketID, Stage: "implement", Lease: *q.AuthorLease}
	x = step()
	issue504Next(t, &q, &o, x)
	q.Action, q.Verdict, q.AuthorLease, q.PriorReturn = "RECORD", &ret, nil, nil
	q.Reasons = []snapshot.ExternalReviewReason{{Code: "fix", Text: "second return"}}
	o.Actor = mutation.Binding{ID: "owner", Role: "OWNER"}
	x = step()
	return *x.Ref, blobs, events, o.Binding, heads
}

func TestIssue504GateAdapter(t *testing.T) {
	ref, blobs, _, binding, _ := issue504Chain(t)
	lookup := func(d wire.Digest) ([]byte, bool) { b, ok := blobs[d]; return b, ok }
	views, err := ExternalReviewGates(map[string]snapshot.ExternalReviewRef{"G1": ref}, lookup, map[string]*ExternalReviewBinding{"G1": &binding})
	if err != nil {
		t.Fatal(err)
	}
	v := views["G1"]
	if v.Status != "CURRENT" || v.Verdict == nil || *v.Verdict != "RETURN" || v.Resubmitted || v.Generation != "2" || v.Revision != "4" {
		t.Fatalf("view %+v", v)
	}
	unknown := func(name string, refs map[string]snapshot.ExternalReviewRef, current map[string]*ExternalReviewBinding, lookup ExternalReviewBlob) {
		t.Helper()
		views, err := ExternalReviewGates(refs, lookup, current)
		if err != nil {
			t.Fatal(name, err)
		}
		for gate, v := range views {
			if v.Status != "UNKNOWN" || v.Verdict != nil {
				t.Fatalf("%s: %s %+v", name, gate, v)
			}
		}
	}
	unknown("absent head", map[string]snapshot.ExternalReviewRef{"G1": ref}, map[string]*ExternalReviewBinding{"G1": &binding}, func(wire.Digest) ([]byte, bool) { return nil, false })
	unknown("no binding", map[string]snapshot.ExternalReviewRef{"G1": ref}, nil, lookup)
	unknown("gate key differs from head", map[string]snapshot.ExternalReviewRef{"G2": ref}, map[string]*ExternalReviewBinding{"G2": &binding}, lookup)
	other := binding
	other.GateID = "G2"
	unknown("binding names another gate", map[string]snapshot.ExternalReviewRef{"G1": ref}, map[string]*ExternalReviewBinding{"G1": &other}, lookup)
	stale := binding
	stale.AcceptanceRevision = "2"
	views, _ = ExternalReviewGates(map[string]snapshot.ExternalReviewRef{"G1": ref}, lookup, map[string]*ExternalReviewBinding{"G1": &stale})
	if views["G1"].Status != "STALE" {
		t.Fatalf("acceptance change not stale: %+v", views["G1"])
	}
	many := map[string]snapshot.ExternalReviewRef{}
	for i := 0; i <= MaxExternalReviewGates; i++ {
		many["G"+strings.Repeat("x", i)] = ref
	}
	if _, err := ExternalReviewGates(many, lookup, nil); err == nil {
		t.Fatal("17 gates accepted")
	}
}

func TestIssue504AnchoredHistory(t *testing.T) {
	ref, blobs, events, _, _ := issue504Chain(t)
	lookup := func(d wire.Digest) ([]byte, bool) { b, ok := blobs[d]; return b, ok }
	const ticket = "ticket:acme:main:AT-0001"
	all, err := ExternalReviewHistory(ref, lookup, ticket, "G1", nil, 0)
	if err != nil || all.Next != nil || len(all.Events) != 4 {
		t.Fatalf("full history %d %v %v", len(all.Events), all.Next, err)
	}
	for i := range events {
		if !bytes.Equal(all.Events[i], events[i]) {
			t.Fatalf("event %d out of order", i)
		}
	}
	first, err := ExternalReviewHistory(ref, lookup, ticket, "G1", nil, 3)
	if err != nil || len(first.Events) != 3 || first.Next == nil || *first.Next != wire.Sum(events[3]) {
		t.Fatalf("first page %v", err)
	}
	rest, err := ExternalReviewHistory(ref, lookup, ticket, "G1", first.Next, 3)
	if err != nil || len(rest.Events) != 1 || rest.Next != nil || !bytes.Equal(rest.Events[0], events[3]) {
		t.Fatalf("second page %v", err)
	}
	refuse := func(name string, r snapshot.ExternalReviewRef, lookup ExternalReviewBlob, gate string, cursor *wire.Digest, limit int) {
		t.Helper()
		if _, err := ExternalReviewHistory(r, lookup, ticket, gate, cursor, limit); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	refuse("page 51", ref, lookup, "G1", nil, 51)
	refuse("negative page", ref, lookup, "G1", nil, -1)
	refuse("other gate", ref, lookup, "G2", nil, 0)
	foreign := wire.Sum([]byte("foreign"))
	refuse("cursor off chain", ref, lookup, "G1", &foreign, 0)
	missing := func(d wire.Digest) ([]byte, bool) {
		if d == wire.Sum(events[2]) {
			return nil, false
		}
		return lookup(d)
	}
	refuse("missing link", ref, missing, "G1", nil, 0)
	swapped := func(d wire.Digest) ([]byte, bool) {
		if d == wire.Sum(events[2]) {
			return events[1], true
		}
		return lookup(d)
	}
	refuse("bytes differ from digest", ref, swapped, "G1", nil, 0)
	moved := ref
	moved.Generation = "1"
	refuse("head generation", moved, lookup, "G1", nil, 0)
}
