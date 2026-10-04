package mutation_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// issue502Carrier is a canonical revision-2 record carrying one open
// question; only the escalation writer may set or change the reference.
func issue502Carrier(t *testing.T) *ticket.Record {
	t.Helper()
	rec := fixture.Ticket("AT-01")
	rec.Revision = "2"
	prev := wire.Sum([]byte("revision 1"))
	rec.PreviousRecordSha256 = &prev
	rec.Escalations = &ticket.EscalationRefs{Revision: "1", LastControlTicketRevision: "2", WorkRevision: "1", Entries: []ticket.EscalationRef{{
		RequestID: "q-1", OriginSha256: wire.Sum([]byte("origin")), HeadSha256: wire.Sum([]byte("origin")), Revision: "1", AcceptanceRevision: "1", Kind: "decision", State: "OPEN",
	}}}
	out, err := ticket.Decode(rec.Encode())
	if err != nil {
		t.Fatalf("carrier: %v", err)
	}
	return out
}

func escalationsMember(t *testing.T, rec *ticket.Record) string {
	t.Helper()
	v, ok := rec.Value().Obj.Get("escalations")
	if !ok {
		return ""
	}
	return string(wire.EncodeFile(v))
}

// TestIssue502_MutationsPreserveEscalations: every ordinary operation carries
// the tool-owned reference through unchanged, including one that bumps the
// acceptance revision (which makes the open question stale, ESC-V0-003).
func TestIssue502_MutationsPreserveEscalations(t *testing.T) {
	cases := []struct {
		name, op, acc string
		payload       wire.Value
	}{
		{"refine", mutation.OpRefine, "2", obj("acceptanceCriteria", wire.Strings([]string{"changed"}))},
		{"prioritize", mutation.OpPrioritize, "1", obj("priority", str("P0"), "order", str("3"))},
		{"hold", mutation.OpHold, "1", obj("holdId", str("h1"), "reason", str("wait"))},
		{"archive", mutation.OpArchive, "1", obj("reason", str("done"))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			canonical := issue502Carrier(t)
			before := escalationsMember(t, canonical)
			plan := apply(t, newCtx(t, owner, nil, canonical), envelope("r-"+c.name, owner, "AT-01", "2", c.op, c.payload))
			want(t, plan, mutation.OutcomeCompleted, "")
			chain(t, canonical, plan.Post, c.acc)
			if got := escalationsMember(t, plan.Post); got != before {
				t.Fatalf("%s changed the reference:\n%s\n%s", c.op, before, got)
			}
		})
	}
}

// TestIssue502_AdoptRefusesEscalationEdits: a diverged intent file cannot
// inject, delete or rewrite the reference; ADOPT_FILE refuses the whole file
// and composes nothing. A file that only changes a routine field keeps it.
func TestIssue502_AdoptRefusesEscalationEdits(t *testing.T) {
	carrier := issue502Carrier(t)
	inject := issue502Carrier(t)
	inject.Escalations = nil
	edits := []struct {
		name      string
		canonical *ticket.Record
		edit      func(*ticket.Record)
	}{
		{"inject", inject, func(r *ticket.Record) { r.Escalations = issue502Carrier(t).Escalations }},
		{"delete", carrier, func(r *ticket.Record) { r.Escalations = nil }},
		{"answer", carrier, func(r *ticket.Record) { r.Escalations.Entries[0].State = "ANSWERED" }},
	}
	for _, c := range edits {
		t.Run(c.name, func(t *testing.T) {
			before := string(c.canonical.Encode())
			file := fileOf(t, c.canonical, func(r *ticket.Record) {
				r.Title = "and a routine edit"
				c.edit(r)
			})
			plan := adopt(t, newCtx(t, owner, nil, c.canonical), c.canonical, file)
			want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
			stillDiverged(t, plan, c.canonical, before)
			if len(plan.Composed) != 0 || !strings.Contains(plan.Detail, "escalations") {
				t.Fatalf("composed %v, detail %q", plan.Composed, plan.Detail)
			}
		})
	}

	file := fileOf(t, carrier, func(r *ticket.Record) { r.Title = "renamed" })
	plan := adopt(t, newCtx(t, owner, nil, carrier), carrier, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if escalationsMember(t, plan.Post) != escalationsMember(t, carrier) {
		t.Fatal("adoption dropped or changed the reference")
	}
}

// TestIssue502_AdoptRefusesUncomposedAddedKey: an optional key present only
// in the file is a difference. ADOPT_FILE composes no operation that sets
// requiresPool, so it refuses the file instead of dropping the key and
// reporting the file adopted.
func TestIssue502_AdoptRefusesUncomposedAddedKey(t *testing.T) {
	roles := map[string][]string{"implement": {"BUILDER"}, "review": {"REVIEWER"}, "integrate": {"VERIFIER"}}
	for key, edit := range map[string]func(*ticket.Record){
		"requiresPool":  func(r *ticket.Record) { r.RequiresPool = "gpu" },
		"requiredRoles": func(r *ticket.Record) { r.RequiredRoles = roles },
	} {
		t.Run(key, func(t *testing.T) {
			canonical := fixture.Ticket("AT-01")
			before := string(canonical.Encode())
			file := fileOf(t, canonical, edit)
			plan := adopt(t, newCtx(t, owner, nil, canonical), canonical, file)
			want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
			stillDiverged(t, plan, canonical, before)
			if !strings.Contains(plan.Detail, key) {
				t.Fatalf("detail %q does not name the key", plan.Detail)
			}
		})
	}
}

// TestIssue502_CreateCannotCarryEscalations: CREATE's payload is closed, so a
// caller cannot inject a reference into a new ticket.
func TestIssue502_CreateCannotCarryEscalations(t *testing.T) {
	p := createPayload("AT-09", "FEATURE", []string{"ac"}, "NATIVE")
	p.Obj.Set("escalations", obj("revision", str("1")))
	if _, err := mutation.Decode(envelope("r", owner, "", "", mutation.OpCreate, p)); wire.CodeOf(err) != wire.CodeMalformed {
		t.Fatalf("CREATE carrying escalations decoded: %v", err)
	}
}
