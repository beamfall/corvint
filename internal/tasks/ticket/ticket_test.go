package ticket_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func code(err error) string { return wire.CodeOf(err) }

func mustDecode(t *testing.T, rec *ticket.Record) *ticket.Record {
	t.Helper()
	out, err := ticket.Decode(rec.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

func inventory(t *testing.T, recs ...*ticket.Record) *ticket.Inventory {
	t.Helper()
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	inv, err := ticket.NewInventory(q, recs)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	return inv
}

func digest(s string) *wire.Digest {
	d := wire.Sum([]byte(s))
	return &d
}

// TestTMV0003_AS02_RecordRoundTrip proves a full record encodes canonically,
// decodes to the same bytes and carries every §3.1 key.
func TestTMV0003_AS02_RecordRoundTrip(t *testing.T) {
	rec := fixture.Ticket("AT-01")
	body := "line one\n\ttabbed \"quoted\" ünïcode"
	rec.Body = &body
	rec.Labels = []string{"a", "b"}
	rec.Dependencies = []ticket.Dependency{fixture.Dep("AT-00"), fixture.GateDep("AT-00", "verify")}
	rec.RequiredGates = []string{"verify"}
	est := wire.Count("30")
	rec.EstimateMinutes = &est
	due := "2026-12-31"
	rec.DueDate = &due
	raw := rec.Encode()
	back, err := ticket.Decode(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(back.Encode()) != string(raw) {
		t.Fatalf("round trip differs:\n%s\n%s", raw, back.Encode())
	}
	if back.FileDigest() != wire.Sum(raw) {
		t.Errorf("file digest is not the digest of the raw bytes")
	}
	v, _ := wire.Parse(raw)
	keys := v.Obj.SortedKeys()
	if len(keys) != 34 {
		t.Errorf("§3.1 has 34 keys, record has %d", len(keys))
	}
}

// TestTMV0002_AS01_RecordClosedSchema covers unknown, missing, duplicate
// keys, wrong types, bad enums and an unsupported profile version.
func TestTMV0002_AS01_RecordClosedSchema(t *testing.T) {
	good := string(fixture.Ticket("AT-01").Encode())
	cases := []struct {
		name string
		edit func(string) string
		want string
	}{
		{"unknown key", func(s string) string { return strings.Replace(s, `"title":`, `"extra":"x","title":`, 1) }, wire.CodeMalformed},
		{"missing key", func(s string) string { return strings.Replace(s, `"shadowOverlay":false,`, ``, 1) }, wire.CodeMalformed},
		{"duplicate key", func(s string) string { return strings.Replace(s, `"order":"0",`, `"order":"0","order":"0",`, 1) }, wire.CodeMalformed},
		{"wrong type", func(s string) string {
			return strings.Replace(s, `"shadowOverlay":false`, `"shadowOverlay":"false"`, 1)
		}, wire.CodeMalformed},
		{"bad status", func(s string) string { return strings.Replace(s, `"status":"OPEN"`, `"status":"open"`, 1) }, wire.CodeMalformed},
		{"bad kind", func(s string) string { return strings.Replace(s, `"kind":"FEATURE"`, `"kind":"EPIC"`, 1) }, wire.CodeMalformed},
		{"unsupported version", func(s string) string {
			return strings.Replace(s, `"profile":"taskman-ticket/0"`, `"profile":"taskman-ticket/1"`, 1)
		}, wire.CodeUnsupportedVersion},
		{"foreign profile", func(s string) string {
			return strings.Replace(s, `"profile":"taskman-ticket/0"`, `"profile":"taskman-queue/0"`, 1)
		}, wire.CodeMalformed},
		{"revision as Size", func(s string) string { return strings.Replace(s, `"revision":"1"`, `"revision":"4294967296"`, 1) }, wire.CodeMalformed},
		{"bad timestamp", func(s string) string {
			return strings.Replace(s, `"createdAt":"2026-09-06T12:00:00Z"`, `"createdAt":"2026-09-06 12:00:00"`, 1)
		}, wire.CodeMalformed},
		{"bad ticket id", func(s string) string {
			return strings.Replace(s, `"ticketId":"ticket:acme:main:AT-01"`, `"ticketId":"AT-01"`, 1)
		}, wire.CodeMalformed},
		{"hostile title", func(s string) string {
			return strings.Replace(s, `"title":"Fixture ticket AT-01"`, "\"title\":\"x‎y\"", 1)
		}, wire.CodeMalformed},
	}
	for _, c := range cases {
		doc := c.edit(good)
		if doc == good {
			t.Fatalf("%s: edit did not apply", c.name)
		}
		_, err := ticket.Decode([]byte(doc))
		if code(err) != c.want {
			t.Errorf("%s: code %q, want %q (%v)", c.name, code(err), c.want, err)
		}
	}
}

// TestTMV0005_AS04_InvalidPriority: an unknown priority is INVALID_PRIORITY.
func TestTMV0005_AS04_InvalidPriority(t *testing.T) {
	rec := fixture.Ticket("AT-01")
	rec.Priority = "P4"
	if _, err := ticket.Decode(rec.Encode()); code(err) != wire.CodeInvalidPriority {
		t.Errorf("code %q, want INVALID_PRIORITY", code(err))
	}
	rec.Priority = "p0"
	if _, err := ticket.Decode(rec.Encode()); code(err) != wire.CodeInvalidPriority {
		t.Errorf("lowercase priority: code %q", code(err))
	}
}

// TestTMV0003_AS02_FieldRelationships covers every §3.1 relationship rule
// the validator enforces.
func TestTMV0003_AS02_FieldRelationships(t *testing.T) {
	type tc struct {
		name string
		edit func(*ticket.Record)
		want string
	}
	cases := []tc{
		{"acceptanceRevision above revision", func(r *ticket.Record) { r.AcceptanceRevision = "2" }, wire.CodeMalformed},
		{"revision 0", func(r *ticket.Record) { r.Revision = "0"; r.AcceptanceRevision = "0" }, wire.CodeMalformed},
		{"previous digest at revision 1", func(r *ticket.Record) { r.PreviousRecordSha256 = digest("x") }, wire.CodeMalformed},
		{"no previous digest at revision 2", func(r *ticket.Record) { r.Revision = "2" }, wire.CodeMalformed},
		{"archivedFrom without ARCHIVED", func(r *ticket.Record) { s := "OPEN"; r.ArchivedFrom = &s }, wire.CodeMalformed},
		{"ARCHIVED without archivedFrom", func(r *ticket.Record) { r.Status = "ARCHIVED" }, wire.CodeMalformed},
		{"HELD without holds", func(r *ticket.Record) { r.Status = "HELD" }, wire.CodeMalformed},
		{"OPEN with holds", func(r *ticket.Record) {
			r.Holds = []ticket.Hold{{HoldID: "h", Actor: "a", Reason: "r", PlacedAt: fixture.Timestamp}}
		}, wire.CodeMalformed},
		{"COMPLETED without completion", func(r *ticket.Record) { r.Status = "COMPLETED" }, wire.CodeMalformed},
		{"OPEN with completion", func(r *ticket.Record) {
			r.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "a", Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		}, wire.CodeMalformed},
		{"ARCHIVED from OPEN with completion", func(r *ticket.Record) {
			r.Status = "ARCHIVED"
			s := "OPEN"
			r.ArchivedFrom = &s
			r.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "a", Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
		}, wire.CodeMalformed},
		{"gateId on COMPLETED obligation", func(r *ticket.Record) {
			d := fixture.Dep("AT-00")
			g := "verify"
			d.GateID = &g
			r.Dependencies = []ticket.Dependency{d}
		}, wire.CodeMalformed},
		{"GATE_PASSED without gateId", func(r *ticket.Record) {
			d := fixture.Dep("AT-00")
			d.Obligation = "GATE_PASSED"
			r.Dependencies = []ticket.Dependency{d}
		}, wire.CodeMalformed},
		{"self dependency", func(r *ticket.Record) { r.Dependencies = []ticket.Dependency{fixture.Dep("AT-01")} }, wire.CodeCycle},
		{"duplicate edge", func(r *ticket.Record) {
			r.Dependencies = []ticket.Dependency{fixture.Dep("AT-00"), fixture.Dep("AT-00")}
		}, wire.CodeDuplicateID},
		{"dependency outside queue", func(r *ticket.Record) {
			id, _ := wire.ParseTicketID("", "ticket:acme:other:X")
			r.Dependencies = []ticket.Dependency{{TicketID: id, Obligation: "COMPLETED"}}
		}, wire.CodeDependencyMissing},
		{"IMPORT without sourceItemId", func(r *ticket.Record) { r.Source.Kind = "IMPORT" }, wire.CodeMalformed},
		{"NATIVE with sourceItemId", func(r *ticket.Record) { s := "x"; r.Source.SourceItemID = &s }, wire.CodeMalformed},
		{"shadowOverlay on NATIVE", func(r *ticket.Record) { r.ShadowOverlay = true }, wire.CodeMalformed},
		{"duplicate holdId", func(r *ticket.Record) {
			r.Status = "HELD"
			r.Holds = []ticket.Hold{{HoldID: "h", Actor: "a", Reason: "", PlacedAt: fixture.Timestamp}, {HoldID: "h", Actor: "b", Reason: "", PlacedAt: fixture.Timestamp}}
		}, wire.CodeDuplicateID},
		{"duplicate grantId", func(r *ticket.Record) {
			a := ticket.Approval{GrantID: "g", Actor: "a", Operation: "RUN", TargetRevision: "1", Scope: []string{}, GrantedAt: fixture.Timestamp}
			r.Approvals = []ticket.Approval{a, a}
		}, wire.CodeDuplicateID},
		{"supersedes itself", func(r *ticket.Record) { id := r.TicketID; r.Supersedes = &id }, wire.CodeMalformed},
		{"PATH resource with bad path", func(r *ticket.Record) {
			r.Effects.Resources = []ticket.Resource{{Class: "PATH", Key: "/abs"}}
		}, wire.CodeMalformed},
		{"bad due date", func(r *ticket.Record) { d := "2026-13-01"; r.DueDate = &d }, wire.CodeMalformed},
		{"unsorted labels", func(r *ticket.Record) { r.Labels = []string{"b", "a"} }, wire.CodeMalformed},
	}
	for _, c := range cases {
		rec := fixture.Ticket("AT-01")
		c.edit(rec)
		_, err := ticket.Decode(rec.Encode())
		if code(err) != c.want {
			t.Errorf("%s: code %q, want %q (%v)", c.name, code(err), c.want, err)
		}
	}
	// Chained revision 2 is valid with a previous digest.
	rec := fixture.Ticket("AT-01")
	rec.Revision = "2"
	rec.PreviousRecordSha256 = digest("prior")
	mustDecode(t, rec)
}

// TestTMV0002_AS10_TicketLimits covers §1 bounds at boundary and boundary+1.
func TestTMV0002_AS10_TicketLimits(t *testing.T) {
	at := func(n int) string { return strings.Repeat("x", n) }
	labels := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = "l" + strings.Repeat("0", 3-len(itoa(i))) + itoa(i)
		}
		return out
	}
	deps := func(n int) []ticket.Dependency {
		out := make([]ticket.Dependency, n)
		for i := range out {
			out[i] = fixture.Dep("D" + itoa(i))
		}
		return out
	}
	cases := []struct {
		name string
		edit func(*ticket.Record)
		want string
	}{
		{"title 512", func(r *ticket.Record) { r.Title = at(512) }, ""},
		{"title 513", func(r *ticket.Record) { r.Title = at(513) }, wire.CodeLimitExceeded},
		{"title 0", func(r *ticket.Record) { r.Title = "" }, wire.CodeMalformed},
		{"body 64 KiB", func(r *ticket.Record) { b := at(64 * 1024); r.Body = &b }, ""},
		{"body 64 KiB + 1", func(r *ticket.Record) { b := at(64*1024 + 1); r.Body = &b }, wire.CodeLimitExceeded},
		{"criterion 4 KiB", func(r *ticket.Record) { r.AcceptanceCriteria = []string{at(4096)} }, ""},
		{"criterion 4 KiB + 1", func(r *ticket.Record) { r.AcceptanceCriteria = []string{at(4097)} }, wire.CodeLimitExceeded},
		{"labels 32", func(r *ticket.Record) { r.Labels = labels(32) }, ""},
		{"labels 33", func(r *ticket.Record) { r.Labels = labels(33) }, wire.CodeLimitExceeded},
		{"dependencies 64", func(r *ticket.Record) { r.Dependencies = deps(64) }, ""},
		{"dependencies 65", func(r *ticket.Record) { r.Dependencies = deps(65) }, wire.CodeLimitExceeded},
		{"holds 17", func(r *ticket.Record) {
			r.Status = "HELD"
			for i := 0; i < 17; i++ {
				r.Holds = append(r.Holds, ticket.Hold{HoldID: "h" + itoa(i), Actor: "a", Reason: "", PlacedAt: fixture.Timestamp})
			}
		}, wire.CodeLimitExceeded},
	}
	for _, c := range cases {
		rec := fixture.Ticket("AT-01")
		c.edit(rec)
		_, err := ticket.Decode(rec.Encode())
		if code(err) != c.want {
			t.Errorf("%s: code %q, want %q (%v)", c.name, code(err), c.want, err)
		}
	}
	// A record file over 128 KiB is refused before parsing.
	rec := fixture.Ticket("AT-01")
	for i := 0; i < 40; i++ {
		rec.AcceptanceCriteria = append(rec.AcceptanceCriteria, at(4096))
	}
	if len(rec.Encode()) <= wire.MaxTicketFileBytes {
		t.Fatalf("fixture did not exceed 128 KiB (%d)", len(rec.Encode()))
	}
	if _, err := ticket.Decode(rec.Encode()); code(err) != wire.CodeLimitExceeded {
		t.Errorf("oversized record file: code %q", code(err))
	}
}

func itoa(i int) string { return string(wire.CountOf(int64(i))) }

// TestTMV0005_AS04_DependencyGraph covers cycles (direct, transitive, via
// GATE_PASSED), missing dependencies and case-folded duplicate tokens.
func TestTMV0005_AS04_DependencyGraph(t *testing.T) {
	a, b, c := fixture.Ticket("A"), fixture.Ticket("B"), fixture.Ticket("C")
	a.Dependencies = []ticket.Dependency{fixture.Dep("B")}
	b.Dependencies = []ticket.Dependency{fixture.Dep("A")}
	inv := inventory(t, a, b, c)
	for _, id := range []string{fixture.TicketID("A"), fixture.TicketID("B")} {
		ps := inv.Problems(id)
		if len(ps) != 1 || ps[0].Code != wire.CodeCycle {
			t.Errorf("direct cycle not reported for %s: %+v", id, ps)
		}
	}
	if ps := inv.Problems(fixture.TicketID("C")); len(ps) != 0 {
		t.Errorf("C has no problem, got %+v", ps)
	}
	// Transitive: A→B→C→A.
	a, b, c = fixture.Ticket("A"), fixture.Ticket("B"), fixture.Ticket("C")
	a.Dependencies = []ticket.Dependency{fixture.Dep("B")}
	b.Dependencies = []ticket.Dependency{fixture.Dep("C")}
	c.Dependencies = []ticket.Dependency{fixture.Dep("A")}
	inv = inventory(t, a, b, c)
	if ps := inv.Problems(fixture.TicketID("C")); len(ps) != 1 || ps[0].Code != wire.CodeCycle {
		t.Errorf("transitive cycle not reported: %+v", ps)
	}
	// Via GATE_PASSED: A →gate B → A.
	a, b = fixture.Ticket("A"), fixture.Ticket("B")
	a.Dependencies = []ticket.Dependency{fixture.GateDep("B", "verify")}
	b.Dependencies = []ticket.Dependency{fixture.Dep("A")}
	inv = inventory(t, a, b)
	if ps := inv.Problems(fixture.TicketID("A")); len(ps) != 1 || ps[0].Code != wire.CodeCycle {
		t.Errorf("gate cycle not reported: %+v", ps)
	}
	// Missing dependency.
	a = fixture.Ticket("A")
	a.Dependencies = []ticket.Dependency{fixture.Dep("NOPE")}
	inv = inventory(t, a)
	if ps := inv.Problems(fixture.TicketID("A")); len(ps) != 1 || ps[0].Code != wire.CodeDependencyMissing || ps[0].TicketID != fixture.TicketID("NOPE") {
		t.Errorf("missing dependency not reported: %+v", ps)
	}
	// CheckDependencies on a proposed record: missing, self, and a cycle
	// through the existing graph.
	inv = inventory(t, fixture.Ticket("A"), fixture.Ticket("B"))
	p := fixture.Ticket("C")
	p.Dependencies = []ticket.Dependency{fixture.Dep("Z")}
	if err := inv.CheckDependencies(p); code(err) != wire.CodeDependencyMissing {
		t.Errorf("proposed missing dependency: %v", err)
	}
	p.Dependencies = []ticket.Dependency{fixture.Dep("C")}
	if err := inv.CheckDependencies(p); code(err) != wire.CodeCycle {
		t.Errorf("proposed self dependency: %v", err)
	}
	b = fixture.Ticket("B")
	b.Dependencies = []ticket.Dependency{fixture.Dep("A")}
	inv = inventory(t, fixture.Ticket("A"), b)
	p = fixture.Ticket("A")
	p.Dependencies = []ticket.Dependency{fixture.Dep("B")}
	if err := inv.CheckDependencies(p); code(err) != wire.CodeCycle {
		t.Errorf("proposed transitive cycle: %v", err)
	}
	p.Dependencies = []ticket.Dependency{}
	if err := inv.CheckDependencies(p); err != nil {
		t.Errorf("empty dependencies refused: %v", err)
	}
	// Case-folded duplicate local tokens.
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	if _, err := ticket.NewInventory(q, []*ticket.Record{fixture.Ticket("at-07"), fixture.Ticket("AT-07")}); code(err) != wire.CodeDuplicateID {
		t.Errorf("case-folded duplicate accepted: %v", err)
	}
	if err := inv.CheckLocalToken("a"); code(err) != wire.CodeDuplicateID {
		t.Errorf("CheckLocalToken must fold case: %v", err)
	}
	if err := inv.CheckLocalToken("AT-99"); err != nil {
		t.Errorf("free token refused: %v", err)
	}
	// Record outside the queue.
	other, _ := wire.ParseQueueID("", "queue:acme:other")
	if _, err := ticket.NewInventory(other, []*ticket.Record{fixture.Ticket("A")}); code(err) != wire.CodeMalformed {
		t.Errorf("foreign record accepted: %v", err)
	}
}

// TestTMV0004_AS05_EligibilityDerived covers §3.2 eligibility as a derived
// view: OPEN/HELD/COMPLETED/ARCHIVED, dependency obligations, approvals,
// effects, imports, and that unknown facts stay UNKNOWN, never ELIGIBLE.
func TestTMV0004_AS05_EligibilityDerived(t *testing.T) {
	ctx := ticket.Context{CanonicalWriter: "NATIVE", SerialFallback: "BLOCK"}
	view := func(inv *ticket.Inventory, local string) ticket.View {
		v, ok := inv.View(fixture.TicketID(local), ctx)
		if !ok {
			t.Fatalf("no view for %s", local)
		}
		return v
	}
	codes := func(v ticket.View) []string {
		var out []string
		for _, b := range v.Blockers {
			out = append(out, b.Code)
		}
		return out
	}
	// Intent checks pass: eligibility is UNKNOWN with the attempt fact unknown.
	v := view(inventory(t, fixture.Ticket("A")), "A")
	if v.Eligibility != ticket.EligibilityUnknown || v.IntentChecks != "PASSED" || len(v.Blockers) != 0 || v.NextAction != "admit" {
		t.Errorf("clean ticket: %+v", v)
	}
	if len(v.Unknowns) != 1 || v.Unknowns[0].Code != wire.CodeAttemptLive || v.CurrentAttempt != ticket.NotObserved {
		t.Errorf("attempt liveness must be an unknown, not a blocker: %+v", v.Unknowns)
	}
	// HELD.
	h := fixture.Ticket("H")
	h.Status = "HELD"
	h.Holds = []ticket.Hold{{HoldID: "review", Actor: "a", Reason: "wait", PlacedAt: fixture.Timestamp}}
	v = view(inventory(t, h), "H")
	if v.Eligibility != ticket.EligibilityBlocked || codes(v)[0] != wire.CodeTicketHeld || v.NextAction != "release-hold" {
		t.Errorf("held: %+v", v)
	}
	// DRAFT.
	d := fixture.Ticket("D")
	d.Status = "DRAFT"
	v = view(inventory(t, d), "D")
	if codes(v)[0] != wire.CodeTicketState || v.NextAction != "refine" {
		t.Errorf("draft: %+v", v)
	}
	// Dependency COMPLETED satisfied by COMPLETED and by ARCHIVED-from-COMPLETED only.
	done := fixture.Ticket("DONE")
	done.Status = "COMPLETED"
	done.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "a", Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
	tomb := fixture.Ticket("TOMB")
	tomb.Status = "ARCHIVED"
	from := "COMPLETED"
	tomb.ArchivedFrom = &from
	tomb.Completion = done.Completion
	open := fixture.Ticket("OPEN")
	dep := fixture.Ticket("DEP")
	dep.Dependencies = []ticket.Dependency{fixture.Dep("DONE"), fixture.Dep("TOMB")}
	dep2 := fixture.Ticket("DEP2")
	dep2.Dependencies = []ticket.Dependency{fixture.Dep("OPEN")}
	inv := inventory(t, done, tomb, open, dep, dep2)
	if v = view(inv, "DEP"); len(v.Blockers) != 0 {
		t.Errorf("satisfied COMPLETED obligations reported blockers: %+v", v.Blockers)
	}
	if v = view(inv, "DEP2"); len(v.Blockers) != 1 || v.Blockers[0].Code != wire.CodeDependencyUnsatisfied || v.NextAction != "wait-dependency" {
		t.Errorf("unsatisfied COMPLETED obligation: %+v", v.Blockers)
	}
	// GATE_PASSED with no oracle stays unknown (fail-closed, never satisfied).
	g := fixture.Ticket("G")
	g.Dependencies = []ticket.Dependency{fixture.GateDep("OPEN", "verify")}
	inv = inventory(t, open, g)
	v = view(inv, "G")
	if v.Eligibility != ticket.EligibilityUnknown || len(v.Blockers) != 0 || len(v.Unknowns) != 2 {
		t.Errorf("gate obligation without evidence must be unknown: blockers %+v unknowns %+v", v.Blockers, v.Unknowns)
	}
	// With an oracle that observes the gate as unsatisfied: BLOCKED.
	obs := ctx
	obs.Gates = gateOracle(ticket.Unsatisfied)
	if v, _ = inv.View(fixture.TicketID("G"), obs); len(v.Blockers) != 1 || v.Blockers[0].Code != wire.CodeDependencyUnsatisfied {
		t.Errorf("observed unsatisfied gate: %+v", v.Blockers)
	}
	obs.Gates = gateOracle(ticket.Satisfied)
	if v, _ = inv.View(fixture.TicketID("G"), obs); len(v.Blockers) != 0 {
		t.Errorf("observed satisfied gate: %+v", v.Blockers)
	}
	// APPROVAL_REQUIRED: grant must be RUN, non-revoked, at the current acceptanceRevision.
	ap := fixture.Ticket("AP")
	ap.ExecutionClass = "APPROVAL_REQUIRED"
	if v = view(inventory(t, ap), "AP"); codes(v)[0] != wire.CodeApprovalMissing || v.NextAction != "grant-approval" {
		t.Errorf("no grant: %+v", v.Blockers)
	}
	ap.Approvals = []ticket.Approval{{GrantID: "g1", Actor: "o", Operation: "RUN", TargetRevision: "1", Scope: []string{}, Revoked: true, GrantedAt: fixture.Timestamp}}
	if v = view(inventory(t, ap), "AP"); codes(v)[0] != wire.CodeApprovalRevoked {
		t.Errorf("revoked grant: %+v", v.Blockers)
	}
	ap.Approvals[0].Revoked = false
	ap.Revision = "2"
	ap.AcceptanceRevision = "2"
	ap.PreviousRecordSha256 = digest("p")
	if v = view(inventory(t, ap), "AP"); codes(v)[0] != wire.CodeApprovalMissing {
		t.Errorf("grant at an old acceptanceRevision must not count: %+v", v.Blockers)
	}
	ap.Approvals[0].TargetRevision = "2"
	if v = view(inventory(t, ap), "AP"); len(v.Blockers) != 0 {
		t.Errorf("current RUN grant: %+v", v.Blockers)
	}
	ap.Approvals[0].Operation = "COMPLETE"
	if v = view(inventory(t, ap), "AP"); codes(v)[0] != wire.CodeApprovalMissing {
		t.Errorf("COMPLETE grant is not a RUN grant: %+v", v.Blockers)
	}
	// Effects and coverage.
	e := fixture.Ticket("E")
	e.Effects.ExternalUnbounded = true
	if v = view(inventory(t, e), "E"); codes(v)[0] != wire.CodeExternalUnbounded || v.NextAction != "set-effects" {
		t.Errorf("externalUnbounded: %+v", v.Blockers)
	}
	e = fixture.Ticket("E")
	e.Effects.Coverage = "UNKNOWN"
	if v = view(inventory(t, e), "E"); codes(v)[0] != wire.CodeCoverageUnknown {
		t.Errorf("coverage UNKNOWN under BLOCK: %+v", v.Blockers)
	}
	whole := ctx
	whole.SerialFallback = "WHOLE_REPOSITORY"
	if v, _ = inventory(t, e).View(fixture.TicketID("E"), whole); len(v.Blockers) != 0 {
		t.Errorf("coverage UNKNOWN under WHOLE_REPOSITORY must pass: %+v", v.Blockers)
	}
	// Imported record only under a NATIVE writer.
	im := fixture.Ticket("IM")
	im.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "roadmap", SourceItemID: strPtr("R-1")}
	road := ctx
	road.CanonicalWriter = "ROADMAP"
	if v, _ = inventory(t, im).View(fixture.TicketID("IM"), road); codes(v)[0] != wire.CodeCutoverMissing || v.Tracked != "IMPORT" {
		t.Errorf("import under ROADMAP writer: %+v tracked %s", v.Blockers, v.Tracked)
	}
	im.ShadowOverlay = true
	if v = view(inventory(t, im), "IM"); v.Tracked != "SHADOW" {
		t.Errorf("shadow tracking: %s", v.Tracked)
	}
	// Execution classes that are never autonomously eligible.
	for _, cls := range []string{"MANUAL", "EXTERNAL", "NEVER"} {
		x := fixture.Ticket("X")
		x.ExecutionClass = cls
		if v = view(inventory(t, x), "X"); codes(v)[0] != wire.CodeTicketState {
			t.Errorf("%s must block: %+v", cls, v.Blockers)
		}
	}
	// A live attempt observed by an oracle blocks ATTEMPT_LIVE.
	live := ctx
	live.Attempts = attemptOracle(ticket.Satisfied)
	if v, _ = inventory(t, fixture.Ticket("A")).View(fixture.TicketID("A"), live); codes(v)[0] != wire.CodeAttemptLive || v.NextAction != "wait-attempt" {
		t.Errorf("live attempt: %+v", v.Blockers)
	}
}

type gateOracle ticket.Observation

func (o gateOracle) GatePassed(wire.TicketID, string, wire.Count) ticket.Observation {
	return ticket.Observation(o)
}

type attemptOracle ticket.Observation

func (o attemptOracle) LiveAttempt(string) ticket.Observation { return ticket.Observation(o) }

func strPtr(s string) *string { return &s }

// TestTMV0003_AS05_DueDateAndEstimateNeverAffectEligibility (TM-V0-003).
func TestTMV0003_AS05_DueDateAndEstimateNeverAffectEligibility(t *testing.T) {
	ctx := ticket.Context{CanonicalWriter: "NATIVE", SerialFallback: "BLOCK"}
	plain := fixture.Ticket("A")
	dated := fixture.Ticket("A")
	due := "2000-01-01"
	est := wire.Count("0")
	dated.DueDate = &due
	dated.EstimateMinutes = &est
	v1, _ := inventory(t, plain).View(fixture.TicketID("A"), ctx)
	v2, _ := inventory(t, dated).View(fixture.TicketID("A"), ctx)
	if v1.Eligibility != v2.Eligibility || len(v1.Blockers) != len(v2.Blockers) || v1.NextAction != v2.NextAction {
		t.Errorf("due date / estimate changed eligibility: %+v vs %+v", v1, v2)
	}
}

// TestTMV0004_AS05_ArchiveTombstoneAndReopen: a tombstone keeps its record
// and its return status; a reopened ticket carries no completion (the prior
// completion is retained in the chained prior record, §3.1) and is evaluated
// as OPEN; a record that claims a completion while not COMPLETED is refused.
func TestTMV0004_AS05_ArchiveTombstoneAndReopen(t *testing.T) {
	ctx := ticket.Context{CanonicalWriter: "NATIVE", SerialFallback: "BLOCK"}
	tomb := fixture.Ticket("T")
	tomb.Status = "ARCHIVED"
	from := "HELD"
	tomb.ArchivedFrom = &from
	tomb.Holds = []ticket.Hold{{HoldID: "h", Actor: "a", Reason: "", PlacedAt: fixture.Timestamp}}
	tomb.Revision = "3"
	tomb.PreviousRecordSha256 = digest("prev")
	back := mustDecode(t, tomb)
	if back.EffectiveStatus() != "HELD" || back.Status != "ARCHIVED" || len(back.Holds) != 1 {
		t.Errorf("tombstone lost its return status or holds: %+v", back)
	}
	v, _ := inventory(t, tomb).View(fixture.TicketID("T"), ctx)
	if v.Eligibility != ticket.EligibilityBlocked || v.Blockers[0].Code != wire.CodeTicketState || v.NextAction != "restore" {
		t.Errorf("tombstone view: %+v", v)
	}
	// A tombstone with HELD as its return status but no holds is inconsistent.
	tomb.Holds = nil
	if _, err := ticket.Decode(tomb.Encode()); code(err) != wire.CodeMalformed {
		t.Errorf("tombstone from HELD without holds accepted: %v", err)
	}
	// Reopened: OPEN at a later revision and acceptanceRevision, no completion.
	re := fixture.Ticket("R")
	re.Revision = "4"
	re.AcceptanceRevision = "2"
	re.PreviousRecordSha256 = digest("prev")
	back = mustDecode(t, re)
	if back.Completion != nil || back.Status != "OPEN" {
		t.Errorf("reopened record: %+v", back)
	}
	v, _ = inventory(t, re).View(fixture.TicketID("R"), ctx)
	if v.Eligibility != ticket.EligibilityUnknown || len(v.Blockers) != 0 {
		t.Errorf("reopened ticket must be evaluated as OPEN: %+v", v)
	}
	if got, _ := v.Value(false).Obj.Get("completion"); got.Kind != wire.KindNull {
		t.Errorf("a reopened ticket must not report a completion: %+v", got)
	}
	dep := fixture.Ticket("D")
	dep.Dependencies = []ticket.Dependency{fixture.Dep("R")}
	v, _ = inventory(t, re, dep).View(fixture.TicketID("D"), ctx)
	if len(v.Blockers) != 1 || v.Blockers[0].Code != wire.CodeDependencyUnsatisfied {
		t.Errorf("a reopened dependency must not satisfy COMPLETED: %+v", v.Blockers)
	}
	// A stale completion left on an OPEN record is a malformed record, never
	// a reopened ticket that is still "verified" (AS-06).
	re.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "a", Reason: strPtr("done by hand"), Evidence: []wire.Digest{}, RecordedAt: fixture.Timestamp}
	if _, err := ticket.Decode(re.Encode()); code(err) != wire.CodeMalformed {
		t.Errorf("OPEN record with a completion accepted: %v", err)
	}
	m := digest("manifest")
	re.Completion = &ticket.Completion{Kind: "VERIFIED", Actor: "a", Evidence: []wire.Digest{}, ManifestSha256: m, RecordedAt: fixture.Timestamp}
	if _, err := ticket.Decode(re.Encode()); code(err) != wire.CodeMalformed {
		t.Errorf("OPEN record with a VERIFIED completion accepted: %v", err)
	}
}

// TestTMV0004_AS06_ManualCompletionLabelled: MANUAL never carries a
// manifest, VERIFIED always does, and the view reports the label verbatim.
func TestTMV0004_AS06_ManualCompletionLabelled(t *testing.T) {
	ctx := ticket.Context{CanonicalWriter: "NATIVE", SerialFallback: "BLOCK"}
	m := fixture.Ticket("M")
	m.Status = "COMPLETED"
	m.Completion = &ticket.Completion{Kind: "MANUAL", Actor: "owner", Reason: strPtr("accepted by hand"), Evidence: []wire.Digest{*digest("e")}, RecordedAt: fixture.Timestamp}
	back := mustDecode(t, m)
	if back.Completion.Kind != "MANUAL" || back.Completion.ManifestSha256 != nil {
		t.Errorf("manual completion: %+v", back.Completion)
	}
	v, _ := inventory(t, m).View(fixture.TicketID("M"), ctx)
	val := v.Value(false)
	if got, _ := val.Obj.Get("completion"); got.Str != "MANUAL" {
		t.Errorf("view must label the completion MANUAL, got %q", got.Str)
	}
	if v.NextAction != "reopen" {
		t.Errorf("completed ticket next action: %s", v.NextAction)
	}
	m.Completion.ManifestSha256 = digest("manifest")
	if _, err := ticket.Decode(m.Encode()); code(err) != wire.CodeMalformed {
		t.Errorf("MANUAL with a manifest accepted: %v", err)
	}
	m.Completion.Kind = "VERIFIED"
	mustDecode(t, m)
	m.Completion.ManifestSha256 = nil
	if _, err := ticket.Decode(m.Encode()); code(err) != wire.CodeMalformed {
		t.Errorf("VERIFIED without a manifest accepted: %v", err)
	}
}

// TestTMV0008_AS08_PlanningOrder: (priority asc, order asc, ticketId bytes).
func TestTMV0008_AS08_PlanningOrder(t *testing.T) {
	a, b, c, d := fixture.Ticket("A"), fixture.Ticket("B"), fixture.Ticket("C"), fixture.Ticket("D")
	a.Priority, a.Order = "P1", "5"
	b.Priority, b.Order = "P0", "9"
	c.Priority, c.Order = "P1", "5"
	d.Priority, d.Order = "P1", "10"
	inv := inventory(t, d, c, b, a)
	got := inv.Sorted()
	want := []string{fixture.TicketID("B"), fixture.TicketID("A"), fixture.TicketID("C"), fixture.TicketID("D")}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order %v, want %v", got, want)
	}
}
