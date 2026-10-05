package mutation_test

import (
	"bytes"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func prereqValue(local, obligation, gate string, stages ...string) wire.Value {
	return obj("ticketId", str(fixture.TicketID(local)), "obligation", str(obligation), "gateId", strOrNull(gate), "stages", wire.Strings(stages))
}

// TestCALV0099_RefineSetsAndClearsPrerequisites: REFINE sets the key (a
// routine field: the acceptance revision is unchanged), validates each
// reference against the inventory and policy like `dependencies`, refuses
// an empty array, and clears the key with null.
func TestCALV0099_RefineSetsAndClearsPrerequisites(t *testing.T) {
	a := fixture.Ticket("AT-01")
	p := fixture.Ticket("AT-02")
	set := func(vs ...wire.Value) wire.Value { return obj("executionPrerequisites", wire.Array(vs...)) }

	// The CLI boundary sorts both the outer set and each stages set; the
	// strict envelope decoder then accepts only canonical bytes.
	raw := set(prereqValue("AT-02", "COMPLETED", "", "integrate"), prereqValue("AT-02", "GATE_PASSED", "verify", "review", "integrate"))
	if _, err := mutation.Decode(envelope("p0", owner, "AT-01", "1", mutation.OpRefine, raw)); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("unsorted prerequisites decoded strictly: %v", err)
	}
	sorted, err := mutation.CanonicalPayload(mutation.OpRefine, raw)
	if err != nil {
		t.Fatalf("canonical payload: %v", err)
	}
	cur := step(t, owner, nil, a, mutation.OpRefine, sorted, "1", p)
	// A routine field: allowed while an attempt is live.
	step(t, owner, attempts{a.TicketID.Raw: true}, a, mutation.OpRefine, sorted, "1", p)
	if len(cur.ExecutionPrerequisites) != 2 || cur.ExecutionPrerequisites[0].Obligation != "GATE_PASSED" {
		t.Fatalf("prerequisites not set canonically: %+v", cur.ExecutionPrerequisites)
	}
	if got := cur.ExecutionPrerequisites[0].Stages; len(got) != 2 || got[0] != "integrate" {
		t.Fatalf("stages not canonical: %v", got)
	}
	cleared := step(t, owner, nil, cur, mutation.OpRefine, obj("executionPrerequisites", wire.Null()), "1", p)
	if cleared.ExecutionPrerequisites != nil || bytes.Contains(cleared.Encode(), []byte("executionPrerequisites")) {
		t.Fatal("null did not clear the key")
	}

	ctx := newCtx(t, owner, nil, a, p)
	want(t, apply(t, ctx, envelope("p1", owner, "AT-01", "1", mutation.OpRefine, set(prereqValue("AT-99", "COMPLETED", "", "integrate")))), mutation.OutcomeValidationFailed, wire.CodeDependencyMissing)
	want(t, apply(t, ctx, envelope("p2", owner, "AT-01", "1", mutation.OpRefine, set(prereqValue("AT-02", "GATE_PASSED", "nope", "integrate")))), mutation.OutcomeValidationFailed, wire.CodeGateUnknown)
	want(t, apply(t, ctx, envelope("p3", owner, "AT-01", "1", mutation.OpRefine, set(prereqValue("AT-01", "COMPLETED", "", "integrate")))), mutation.OutcomeValidationFailed, wire.CodeMalformed)
	if _, err := mutation.Decode(envelope("p4", owner, "AT-01", "1", mutation.OpRefine, set())); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("empty array decoded: %v", err)
	}
	if _, err := mutation.Decode(envelope("p5", owner, "AT-01", "1", mutation.OpRefine, set(prereqValue("AT-02", "COMPLETED", "", "deploy")))); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("unknown stage decoded: %v", err)
	}
	// Mutual prerequisites are no cycle: they are excluded from cycle detection.
	b := fixture.Ticket("AT-02")
	b.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: a.TicketID, Obligation: "COMPLETED", Stages: []string{"review"}}}
	step(t, owner, nil, a, mutation.OpRefine, set(prereqValue("AT-02", "COMPLETED", "", "integrate")), "1", b)
}

// TestCALV0099_CompletingPrerequisiteChangesNoOtherRecord: completing the
// prerequisite plans exactly one post record (its own); the dependent
// record is untouched.
func TestCALV0099_CompletingPrerequisiteChangesNoOtherRecord(t *testing.T) {
	a := fixture.Ticket("AT-01")
	a.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: fixture.Ticket("AT-02").TicketID, Obligation: "COMPLETED", Stages: []string{"integrate"}}}
	before := a.Encode()
	p := fixture.Ticket("AT-02")
	plan := apply(t, newCtx(t, owner, nil, a, p), envelope("c1", owner, "AT-02", "1", mutation.OpCompleteManual, obj("reason", str("done"), "evidence", wire.Strings(nil))))
	want(t, plan, mutation.OutcomeCompleted, "")
	if plan.Post.TicketID.Raw != p.TicketID.Raw || plan.QueuePost != nil || !bytes.Equal(a.Encode(), before) {
		t.Fatal("completing a prerequisite touched another record")
	}
}

// TestCALV0099_AdoptComposesPrerequisites: ADOPT_FILE accepts the key as a
// routine field composed through REFINE, and preserves it across an
// unrelated adoption.
func TestCALV0099_AdoptComposesPrerequisites(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	p := fixture.Ticket("AT-02")
	pre := []ticket.Prerequisite{{TicketID: p.TicketID, Obligation: "GATE_PASSED", GateID: strPtr("verify"), Stages: []string{"integrate"}}}
	file := fileOf(t, canonical, func(r *ticket.Record) { r.ExecutionPrerequisites = pre })
	plan := adopt(t, newCtx(t, owner, nil, canonical, p), canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Post.ExecutionPrerequisites) != 1 || string(plan.Post.AcceptanceRevision) != "1" {
		t.Fatalf("adoption did not compose the key as a routine field: %+v", plan.Post.ExecutionPrerequisites)
	}
	carrier := plan.Post
	member := func(r *ticket.Record) string {
		v, _ := r.Value().Obj.Get("executionPrerequisites")
		return string(wire.EncodeFile(v))
	}
	plan = adopt(t, newCtx(t, owner, nil, carrier, p), carrier, fileOf(t, carrier, func(r *ticket.Record) { r.Title = "renamed" }))
	want(t, plan, mutation.OutcomeCompleted, "")
	if member(plan.Post) != member(carrier) {
		t.Fatal("unrelated adoption changed the key")
	}
	// A file reference to a missing ticket is refused like a dependency.
	bad := fileOf(t, canonical, func(r *ticket.Record) {
		r.ExecutionPrerequisites = []ticket.Prerequisite{{TicketID: fixture.Ticket("AT-99").TicketID, Obligation: "COMPLETED", Stages: []string{"integrate"}}}
	})
	want(t, adopt(t, newCtx(t, owner, nil, canonical, p), canonical, bad), mutation.OutcomeValidationFailed, wire.CodeDependencyMissing)
}
