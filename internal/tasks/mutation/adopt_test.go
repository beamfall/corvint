package mutation_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// fileOf renders a diverged intent file: a deep copy of the canonical
// record with the given edits applied, encoded as the file would be.
func fileOf(t *testing.T, rec *ticket.Record, edit func(*ticket.Record)) []byte {
	t.Helper()
	c, err := ticket.Decode(rec.Encode())
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	edit(c)
	return c.Encode()
}

func adopt(t *testing.T, ctx mutation.Context, canonical *ticket.Record, file []byte) *mutation.Plan {
	t.Helper()
	return mutation.Adopt(ctx, "adopt-1", canonical, file)
}

// stillDiverged asserts the pure equivalent of "the ticket stays
// INTENT_DIVERGED and the state dir is byte-identical": no post record, no
// resulting revision, the canonical record unchanged.
func stillDiverged(t *testing.T, plan *mutation.Plan, canonical *ticket.Record, before string) {
	t.Helper()
	if plan.Post != nil || plan.Outcome.ResultingRevision != nil || plan.Outcome.ReceiptSeq != nil {
		t.Fatalf("refused adoption produced a post record or revision")
	}
	if string(canonical.Encode()) != before {
		t.Fatalf("canonical record changed")
	}
}

func completedManual(actor string) *ticket.Completion {
	reason := "by hand"
	return &ticket.Completion{Kind: "MANUAL", Actor: actor, Reason: &reason, Evidence: []wire.Digest{}, RecordedAt: now}
}

// TestTMV0007_AS35_N3a_StatusCompleted: file sets status COMPLETED →
// VALIDATION_FAILED/ADOPT_UNSUPPORTED_FIELD; no completion; nothing written.
func TestTMV0007_AS35_N3a_StatusCompleted(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical)
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Status = ticket.StatusCompleted
		r.Completion = completedManual("someone")
	})
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, plan, canonical, before)
	if len(plan.Composed) != 0 {
		t.Fatalf("nothing may be composed once a protected field differs: %v", plan.Composed)
	}
	if canonical.Completion != nil {
		t.Fatalf("a completion appeared")
	}
	// The same file with a routine change alongside is still refused whole.
	file = fileOf(t, canonical, func(r *ticket.Record) {
		r.Title = "renamed"
		r.Status = ticket.StatusCompleted
		r.Completion = completedManual("someone")
	})
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, plan, canonical, before)
}

// TestTMV0007_AS35_N3b_Approvals: file adds an approvals entry → refused as
// N3a; no grant exists afterwards.
func TestTMV0007_AS35_N3b_Approvals(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical)
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Approvals = []ticket.Approval{{GrantID: "g", Actor: "russell", Operation: "RUN", TargetRevision: "1", Scope: []string{}, GrantedAt: now}}
	})
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, plan, canonical, before)
	if len(canonical.Approvals) != 0 {
		t.Fatalf("a grant appeared")
	}
}

// TestTMV0007_AS35_N3c_HandBumpedRevision: file hand-bumps revision or
// acceptanceRevision → refused as N3a.
func TestTMV0007_AS35_N3c_HandBumpedRevision(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical)
	prev := wire.Sum([]byte("whatever"))
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Revision = "2"
		r.PreviousRecordSha256 = &prev
	})
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, plan, canonical, before)

	file = fileOf(t, canonical, func(r *ticket.Record) {
		r.Revision = "2"
		r.AcceptanceRevision = "2"
		r.PreviousRecordSha256 = &prev
		r.Title = "and a title"
	})
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, plan, canonical, before)
}

// TestTMV0007_AS35_N3d_PrioritizeComposed: file changes only priority and
// order → one composition of PRIORITIZE; revision +1, acceptanceRevision
// unchanged, previousRecordSha256 = the canonical record's digest.
func TestTMV0007_AS35_N3d_PrioritizeComposed(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical)
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Priority = "P0"
		r.Order = "7"
	})
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 1 || plan.Composed[0] != mutation.OpPrioritize {
		t.Fatalf("composed %v; want [PRIORITIZE]", plan.Composed)
	}
	chain(t, canonical, plan.Post, "1")
	if plan.Post.Revision != "2" || plan.Post.Priority != "P0" || plan.Post.Order != "7" {
		t.Fatalf("post %s %s %s", plan.Post.Revision, plan.Post.Priority, plan.Post.Order)
	}
	if *plan.Post.PreviousRecordSha256 == wire.Sum(file) {
		t.Fatalf("chained from the file, not the canonical record")
	}
	if plan.Post.UpdatedBy != "russell" || plan.Post.UpdatedAt != now {
		t.Fatalf("adoption attribution %s %s", plan.Post.UpdatedBy, plan.Post.UpdatedAt)
	}
	if plan.MutationSha256 == wire.Sum(file) {
		t.Fatalf("adoption digest must bind more than the file bytes")
	}
	if plan.MutationSha256 != mutation.AdoptDigest(owner, ctx.Queue.QueueID, canonical.TicketID, "adopt-1", file) {
		t.Fatalf("adoption digest is not the §3.3 ADOPT_FILE request digest")
	}
	if string(canonical.Encode()) != before {
		t.Fatalf("canonical record mutated")
	}
	// Nothing durable: the plan has no receipt and the index was not written.
	if plan.Outcome.ReceiptSeq != nil || ctx.Requests.(*mutation.MemoryIndex).Len() != 0 {
		t.Fatalf("adoption pretended to commit")
	}
}

// TestTMV0007_AS35_N3e_AttemptLive: file changes acceptanceCriteria while an
// attempt is live → BLOCKED/ATTEMPT_LIVE; the ticket stays diverged.
func TestTMV0007_AS35_N3e_AttemptLive(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, attempts{fixture.TicketID("AT-01"): true}, canonical)
	file := fileOf(t, canonical, func(r *ticket.Record) { r.AcceptanceCriteria = []string{"changed"} })
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeBlocked, wire.CodeAttemptLive)
	stillDiverged(t, plan, canonical, before)
	// A routine difference adopts despite the live attempt, revision only.
	file = fileOf(t, canonical, func(r *ticket.Record) { r.Title = "renamed while live" })
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, canonical, plan.Post, "1")
	// Unknown liveness fails closed for the relevant difference.
	ctx.Attempts = ticket.NoEvidence{}
	file = fileOf(t, canonical, func(r *ticket.Record) { r.RequiredGates = []string{"verify"} })
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeBlocked, wire.CodeAttemptLive)
	stillDiverged(t, plan, canonical, before)
}

// TestTMV0007_AS35_N3f_DependencyCycle: file changes dependencies to
// introduce a cycle → VALIDATION_FAILED/CYCLE; stays diverged.
func TestTMV0007_AS35_N3f_DependencyCycle(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	at02 := fixture.Ticket("AT-02")
	at02.Dependencies = []ticket.Dependency{fixture.Dep("AT-01")}
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical, at02)
	file := fileOf(t, canonical, func(r *ticket.Record) { r.Dependencies = []ticket.Dependency{fixture.Dep("AT-02")} })
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeCycle)
	stillDiverged(t, plan, canonical, before)
	file = fileOf(t, canonical, func(r *ticket.Record) { r.Dependencies = []ticket.Dependency{fixture.Dep("AT-99")} })
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeDependencyMissing)
	file = fileOf(t, canonical, func(r *ticket.Record) { r.RequiredGates = []string{"nope"} })
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeGateUnknown)
	stillDiverged(t, plan, canonical, before)
}

// TestTMV0007_AS35_HoldEditsRefused: an existing hold whose actor, reason or
// placedAt differs in the file has no composed operation and is refused,
// never silently ignored (owner clarification).
func TestTMV0007_AS35_HoldEditsRefused(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	canonical.Status = ticket.StatusHeld
	canonical.Holds = []ticket.Hold{
		{HoldID: "review", Actor: "russell", Reason: "needs review", PlacedAt: fixture.Timestamp},
		{HoldID: "legal", Actor: "russell", Reason: "legal", PlacedAt: fixture.Timestamp},
	}
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical)
	for name, edit := range map[string]func(*ticket.Record){
		"reason":   func(r *ticket.Record) { r.Holds[0].Reason = "changed" },
		"actor":    func(r *ticket.Record) { r.Holds[0].Actor = "mallory" },
		"placedAt": func(r *ticket.Record) { r.Holds[0].PlacedAt = now },
		"reorder":  func(r *ticket.Record) { r.Holds[0], r.Holds[1] = r.Holds[1], r.Holds[0] },
	} {
		plan := adopt(t, ctx, canonical, fileOf(t, canonical, edit))
		if plan.Outcome.Outcome != mutation.OutcomeValidationFailed || !plan.Outcome.HasCode(wire.CodeAdoptUnsupportedField) {
			t.Fatalf("%s edit: %s %v (%s)", name, plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail)
		}
		stillDiverged(t, plan, canonical, before)
	}
}

// TestTMV0007_AS35_HoldsComposedFromContext: a hold only in the file becomes
// HOLD with the reconciling actor and logical time (the file's actor and
// placedAt are ignored); a hold only in the canonical record becomes
// RELEASE_HOLD; the OPEN↔HELD status follows the holds.
func TestTMV0007_AS35_HoldsComposedFromContext(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	ctx := newCtx(t, operator, nil, canonical)
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "review", Actor: "someone-else", Reason: "please look", PlacedAt: "2020-01-01T00:00:00Z"}}
	})
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 1 || plan.Composed[0] != mutation.OpHold {
		t.Fatalf("composed %v", plan.Composed)
	}
	chain(t, canonical, plan.Post, "1")
	h := plan.Post.Holds
	if plan.Post.Status != ticket.StatusHeld || len(h) != 1 || h[0].HoldID != "review" || h[0].Reason != "please look" {
		t.Fatalf("hold: %s %+v", plan.Post.Status, h)
	}
	if h[0].Actor != "ops" || h[0].PlacedAt != now {
		t.Fatalf("hold actor/time must come from the context, got %s/%s", h[0].Actor, h[0].PlacedAt)
	}

	// Release via file: HELD [review] → OPEN [].
	held := plan.Post
	ctx = newCtx(t, operator, nil, held)
	file = fileOf(t, held, func(r *ticket.Record) {
		r.Status = ticket.StatusOpen
		r.Holds = []ticket.Hold{}
	})
	plan = adopt(t, ctx, held, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 1 || plan.Composed[0] != mutation.OpReleaseHold || plan.Post.Status != ticket.StatusOpen {
		t.Fatalf("release: %v %s", plan.Composed, plan.Post.Status)
	}
	// Swap: HELD [review] → HELD [other]: HOLD then RELEASE_HOLD, one revision.
	file = fileOf(t, held, func(r *ticket.Record) {
		r.Holds = []ticket.Hold{{HoldID: "other", Actor: "x", Reason: "swap", PlacedAt: now}}
	})
	plan = adopt(t, ctx, held, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 2 || plan.Composed[0] != mutation.OpHold || plan.Composed[1] != mutation.OpReleaseHold {
		t.Fatalf("swap composed %v", plan.Composed)
	}
	chain(t, held, plan.Post, "1")
	if len(plan.Post.Holds) != 1 || plan.Post.Holds[0].HoldID != "other" || plan.Post.Holds[0].Actor != "ops" {
		t.Fatalf("swap holds %+v", plan.Post.Holds)
	}
	// Any other status difference is protected: OPEN → DRAFT, OPEN → ARCHIVED.
	ctx = newCtx(t, operator, nil, canonical)
	from := ticket.StatusOpen
	for name, edit := range map[string]func(*ticket.Record){
		"draft":    func(r *ticket.Record) { r.Status = ticket.StatusDraft },
		"archived": func(r *ticket.Record) { r.Status = ticket.StatusArchived; r.ArchivedFrom = &from },
	} {
		plan = adopt(t, ctx, canonical, fileOf(t, canonical, edit))
		if plan.Outcome.Outcome != mutation.OutcomeValidationFailed || !plan.Outcome.HasCode(wire.CodeAdoptUnsupportedField) {
			t.Fatalf("%s: %s %v", name, plan.Outcome.Outcome, plan.Outcome.Codes)
		}
	}
	// A hold in the file that violates the record rule (HELD without holds)
	// fails file validation before any composition.
	file = fileOf(t, canonical, func(r *ticket.Record) { r.Status = ticket.StatusHeld })
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// Policy row removal applies to the composed operation too.
	pv := fixture.PolicyValue()
	pv.Obj.Set("roles", obj("OPERATOR", wire.Strings([]string{"PRIORITIZE", "REFINE"})))
	ctx = ctxWithPolicy(t, operator, nil, wire.EncodeFile(pv), canonical)
	file = fileOf(t, canonical, func(r *ticket.Record) {
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "h", Actor: "x", Reason: "r", PlacedAt: now}}
	})
	want(t, adopt(t, ctx, canonical, file), mutation.OutcomeUnauthorized, "")
}

// TestTMV0007_AS35_CompositionIsOneRevision: several differing fields
// compose several operations in the fixed order, applied as one revision
// with acceptanceRevision +1 exactly once.
func TestTMV0007_AS35_CompositionIsOneRevision(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	at03 := fixture.Ticket("AT-03")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical, at03)
	est := wire.Count("45")
	file := fileOf(t, canonical, func(r *ticket.Record) {
		r.Title = "new title"
		r.Kind = "BUG"
		r.EstimateMinutes = &est
		r.Priority = "P1"
		r.Dependencies = []ticket.Dependency{fixture.GateDep("AT-03", "verify")}
		r.RequiredGates = []string{"verify"}
		r.Effects.Coverage = "INCOMPLETE"
		r.Capabilities = []string{"git"}
		r.ExecutionClass = "APPROVAL_REQUIRED"
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "triage", Actor: "x", Reason: "triage", PlacedAt: now}}
		r.UpdatedAt = "2030-01-01T00:00:00Z"
		// order is unchanged; PRIORITIZE still takes both values from the file.
	})
	plan := adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	wantOps := []string{mutation.OpRefine, mutation.OpPrioritize, mutation.OpSetDependencies, mutation.OpSetGates, mutation.OpSetEffects, mutation.OpHold}
	if len(plan.Composed) != len(wantOps) {
		t.Fatalf("composed %v; want %v", plan.Composed, wantOps)
	}
	for i := range wantOps {
		if plan.Composed[i] != wantOps[i] {
			t.Fatalf("composed %v; want %v", plan.Composed, wantOps)
		}
	}
	chain(t, canonical, plan.Post, "2")
	p := plan.Post
	if p.Revision != "2" || p.Title != "new title" || p.Kind != "BUG" || *p.EstimateMinutes != "45" || p.Priority != "P1" ||
		len(p.Dependencies) != 1 || len(p.RequiredGates) != 1 || p.Effects.Coverage != "INCOMPLETE" || p.ExecutionClass != "APPROVAL_REQUIRED" ||
		p.Status != ticket.StatusHeld || len(p.Holds) != 1 || p.Holds[0].Actor != "russell" {
		t.Fatalf("composition not applied: %s", p.Encode())
	}
	if p.UpdatedAt != now {
		t.Fatalf("the file's updatedAt was adopted")
	}
	if string(canonical.Encode()) != before {
		t.Fatalf("canonical mutated")
	}
	// A file that differs only in updatedAt composes nothing and yields a
	// RECONCILE post at revision +1 with no acceptance bump.
	file = fileOf(t, canonical, func(r *ticket.Record) { r.UpdatedAt = "2030-01-01T00:00:00Z" })
	plan = adopt(t, ctx, canonical, file)
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 0 {
		t.Fatalf("composed %v for an updatedAt-only difference", plan.Composed)
	}
	chain(t, canonical, plan.Post, "1")
	// A COMPLETED canonical record refuses acceptance-relevant adoption but
	// takes a routine one.
	done := fixture.Ticket("AT-05")
	done.Status = ticket.StatusCompleted
	done.Completion = completedManual("russell")
	ctx = newCtx(t, owner, nil, done)
	plan = adopt(t, ctx, done, fileOf(t, done, func(r *ticket.Record) { r.AcceptanceCriteria = []string{"more"} }))
	want(t, plan, mutation.OutcomeBlocked, wire.CodeTicketState)
	plan = adopt(t, ctx, done, fileOf(t, done, func(r *ticket.Record) { r.Owner = strPtr("me") }))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, done, plan.Post, "1")
}

func strPtr(s string) *string { return &s }

// TestTMV0007_AS35_DraftOpensByAdoption: status is derived, never adopted.
// A file that adds acceptance criteria to a DRAFT composes REFINE and the
// ticket opens (§3.2), whether the editor left `status` at DRAFT or wrote
// OPEN; a hand-opened DRAFT whose criteria stay empty is refused; a DRAFT
// can be held only once the same composition opened it; a DRAFT written
// over an OPEN record is refused.
func TestTMV0007_AS35_DraftOpensByAdoption(t *testing.T) {
	draft := fixture.Ticket("AT-01")
	draft.Status = ticket.StatusDraft
	draft.AcceptanceCriteria = []string{}
	before := string(draft.Encode())
	ctx := newCtx(t, owner, nil, draft)
	for name, status := range map[string]string{"left DRAFT": ticket.StatusDraft, "written OPEN": ticket.StatusOpen} {
		file := fileOf(t, draft, func(r *ticket.Record) {
			r.AcceptanceCriteria = []string{"it works"}
			r.Status = status
		})
		plan := adopt(t, ctx, draft, file)
		if plan.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("%s: %s %v (%s)", name, plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail)
		}
		if len(plan.Composed) != 1 || plan.Composed[0] != mutation.OpRefine || plan.Post.Status != ticket.StatusOpen {
			t.Fatalf("%s: composed %v, status %s", name, plan.Composed, plan.Post.Status)
		}
		chain(t, draft, plan.Post, "2")
		if string(draft.Encode()) != before {
			t.Fatalf("canonical mutated")
		}
	}
	// Hand-opened without criteria: not derivable.
	plan := adopt(t, ctx, draft, fileOf(t, draft, func(r *ticket.Record) { r.Status = ticket.StatusOpen }))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, plan, draft, before)
	// Criteria plus a hold: REFINE opens, then HOLD derives HELD.
	plan = adopt(t, ctx, draft, fileOf(t, draft, func(r *ticket.Record) {
		r.AcceptanceCriteria = []string{"it works"}
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "triage", Actor: "x", Reason: "look", PlacedAt: now}}
	}))
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 2 || plan.Composed[1] != mutation.OpHold || plan.Post.Status != ticket.StatusHeld {
		t.Fatalf("composed %v, status %s", plan.Composed, plan.Post.Status)
	}
	// A hold on a DRAFT that nothing opens is refused by the HOLD step.
	plan = adopt(t, ctx, draft, fileOf(t, draft, func(r *ticket.Record) {
		r.Status = ticket.StatusHeld
		r.Holds = []ticket.Hold{{HoldID: "triage", Actor: "x", Reason: "look", PlacedAt: now}}
	}))
	want(t, plan, mutation.OutcomeBlocked, wire.CodeTicketState)
	stillDiverged(t, plan, draft, before)
	// Protected statuses are never derivable, even from a DRAFT.
	for name, edit := range map[string]func(*ticket.Record){
		"completed": func(r *ticket.Record) { r.Status = ticket.StatusCompleted; r.Completion = completedManual("x") },
		"archived":  func(r *ticket.Record) { s := ticket.StatusDraft; r.Status = ticket.StatusArchived; r.ArchivedFrom = &s },
	} {
		plan = adopt(t, ctx, draft, fileOf(t, draft, edit))
		if plan.Outcome.Outcome != mutation.OutcomeValidationFailed || !plan.Outcome.HasCode(wire.CodeAdoptUnsupportedField) || len(plan.Composed) != 0 {
			t.Fatalf("%s: %s %v %v", name, plan.Outcome.Outcome, plan.Outcome.Codes, plan.Composed)
		}
	}
	// OPEN → DRAFT by hand stays refused.
	open := fixture.Ticket("AT-02")
	ctx = newCtx(t, owner, nil, open)
	plan = adopt(t, ctx, open, fileOf(t, open, func(r *ticket.Record) { r.Status = ticket.StatusDraft }))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
}

// TestTMV0007_AS35_RequestDigestBindsActorAndTarget: the ADOPT_FILE request
// digest binds the operation, request ID, trusted actor, queue, target and
// file bytes. A second actor, another target or other bytes under the same
// request ID conflicts; an identical retry after the plan was committed (the
// canonical record has moved on) replays the recorded outcome.
func TestTMV0007_AS35_RequestDigestBindsActorAndTarget(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	other := fixture.Ticket("AT-02")
	file := fileOf(t, canonical, func(r *ticket.Record) { r.Body = strPtr("hand-edited body") })
	idx := mutation.NewMemoryIndex()
	ctx := newCtx(t, owner, nil, canonical, other)
	ctx.Requests = idx
	first := adopt(t, ctx, canonical, file)
	want(t, first, mutation.OutcomeCompleted, "")
	if first.MutationSha256 != mutation.AdoptDigest(owner, ctx.Queue.QueueID, canonical.TicketID, "adopt-1", file) {
		t.Fatalf("digest is not the ADOPT_FILE request digest")
	}
	if err := idx.Record(mutation.IndexEntry{RequestID: "adopt-1", MutationSha256: first.MutationSha256, Outcome: first.Outcome}); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Another actor with the same role, and the same actor id under another
	// role, both conflict rather than receive the owner's replay.
	for _, b := range []mutation.Binding{operator, {ID: "russell", Role: "OPERATOR"}} {
		c := newCtx(t, b, nil, canonical, other)
		c.Requests = idx
		p := adopt(t, c, canonical, file)
		want(t, p, mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
		if p.Outcome.Replayed {
			t.Fatalf("%s/%s received a replay of another principal's request", b.Role, b.ID)
		}
	}
	// Same owner, same bytes shape, another target.
	otherFile := fileOf(t, other, func(r *ticket.Record) { r.Body = strPtr("hand-edited body") })
	want(t, adopt(t, ctx, other, otherFile), mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
	// Same owner, same target, other bytes.
	want(t, adopt(t, ctx, canonical, fileOf(t, canonical, func(r *ticket.Record) { r.Body = strPtr("other") })), mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
	// Identical retry after commit: the canonical record is now the post
	// record, the file still carries revision 1, and the request replays
	// without decoding or composing anything.
	committed := newCtx(t, owner, nil, first.Post, other)
	committed.Requests = idx
	again := adopt(t, committed, first.Post, file)
	if again.Outcome.Outcome != mutation.OutcomeCompleted || !again.Outcome.Replayed || again.Post != nil || len(again.Composed) != 0 {
		t.Fatalf("retry after commit: %+v %v", again.Outcome, again.Composed)
	}
	if again.MutationSha256 != first.MutationSha256 {
		t.Fatalf("retry digest moved with the canonical revision")
	}
	// The digest depends on the queue and on the request ID.
	otherQueue, _ := wire.ParseQueueID("", "queue:acme:side")
	if mutation.AdoptDigest(owner, otherQueue, canonical.TicketID, "adopt-1", file) == first.MutationSha256 ||
		mutation.AdoptDigest(owner, ctx.Queue.QueueID, canonical.TicketID, "adopt-2", file) == first.MutationSha256 {
		t.Fatalf("digest ignores queue or request ID")
	}
}

// TestTMV0007_AS35_TombstoneRefusesAdoption: an ARCHIVED canonical record
// accepts only RESTORE (§3.2), which ADOPT_FILE never composes; even an
// identical or updatedAt-only file is refused BLOCKED/TICKET_STATE before
// composition, so no revision is minted on a tombstone.
func TestTMV0007_AS35_TombstoneRefusesAdoption(t *testing.T) {
	tomb := fixture.Ticket("AT-01")
	from := ticket.StatusOpen
	tomb.Status = ticket.StatusArchived
	tomb.ArchivedFrom = &from
	before := string(tomb.Encode())
	ctx := newCtx(t, owner, nil, tomb)
	for name, file := range map[string][]byte{
		"identical":      tomb.Encode(),
		"updatedAt-only": fileOf(t, tomb, func(r *ticket.Record) { r.UpdatedAt = "2030-01-01T00:00:00Z" }),
		"title":          fileOf(t, tomb, func(r *ticket.Record) { r.Title = "renamed tombstone" }),
		"restored":       fileOf(t, tomb, func(r *ticket.Record) { r.Status = ticket.StatusOpen; r.ArchivedFrom = nil }),
	} {
		plan := adopt(t, ctx, tomb, file)
		if plan.Outcome.Outcome != mutation.OutcomeBlocked || !plan.Outcome.HasCode(wire.CodeTicketState) {
			t.Fatalf("%s: %s %v (%s)", name, plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail)
		}
		if len(plan.Composed) != 0 {
			t.Fatalf("%s: composed %v on a tombstone", name, plan.Composed)
		}
		stillDiverged(t, plan, tomb, before)
	}
}

// TestTMV0007_AS35_ProtectedFieldsAndIdentity: every other protected field,
// a ticketId mismatch, a malformed file and an unrelated canonical record.
func TestTMV0007_AS35_ProtectedFieldsAndIdentity(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	other := fixture.Ticket("AT-02")
	before := string(canonical.Encode())
	ctx := newCtx(t, owner, nil, canonical, other)
	item := "X-1"
	d := wire.Sum([]byte("x"))
	protectedEdits := map[string]func(*ticket.Record){
		"source": func(r *ticket.Record) {
			r.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "queue:ext:src", SourceItemID: &item}
		},
		"shadow": func(r *ticket.Record) {
			r.Source = ticket.Source{Kind: "IMPORT", SourceQueueID: "queue:ext:src", SourceItemID: &item}
			r.ShadowOverlay = true
		},
		"supersededBy": func(r *ticket.Record) { id := other.TicketID; r.SupersededBy = &id },
		"createdAt":    func(r *ticket.Record) { r.CreatedAt = now },
		"updatedBy":    func(r *ticket.Record) { r.UpdatedBy = "mallory" },
		"prevDigest":   func(r *ticket.Record) { r.Revision = "2"; r.PreviousRecordSha256 = &d },
	}
	for name, edit := range protectedEdits {
		plan := adopt(t, ctx, canonical, fileOf(t, canonical, edit))
		if plan.Outcome.Outcome != mutation.OutcomeValidationFailed || !plan.Outcome.HasCode(wire.CodeAdoptUnsupportedField) {
			t.Fatalf("%s: %s %v (%s)", name, plan.Outcome.Outcome, plan.Outcome.Codes, plan.Detail)
		}
		stillDiverged(t, plan, canonical, before)
	}
	// An OPEN record with completion is malformed; keep that negative distinct.
	malformed := adopt(t, ctx, canonical, fileOf(t, canonical, func(r *ticket.Record) { r.Completion = completedManual("x") }))
	want(t, malformed, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	stillDiverged(t, malformed, canonical, before)
	// A valid COMPLETED record still cannot have its protected completion edited.
	done := fixture.Ticket("DONE")
	done.Status = ticket.StatusCompleted
	done.Completion = completedManual("owner")
	doneCtx := newCtx(t, owner, nil, done)
	edited := fileOf(t, done, func(r *ticket.Record) { r.Completion = completedManual("x") })
	if _, err := ticket.Decode(edited); err != nil {
		t.Fatalf("protected-field witness must be valid: %v", err)
	}
	refused := adopt(t, doneCtx, done, edited)
	want(t, refused, mutation.OutcomeValidationFailed, wire.CodeAdoptUnsupportedField)
	stillDiverged(t, refused, done, string(done.Encode()))
	// Supersedes is adoptable (REFINE) and validated.
	plan := adopt(t, ctx, canonical, fileOf(t, canonical, func(r *ticket.Record) { id := other.TicketID; r.Supersedes = &id }))
	want(t, plan, mutation.OutcomeCompleted, "")
	chain(t, canonical, plan.Post, "2")
	// Identity: the file must be this ticket.
	plan = adopt(t, ctx, canonical, other.Encode())
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// Malformed and non-canonical files.
	plan = adopt(t, ctx, canonical, []byte("{}\n"))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	plan = adopt(t, ctx, canonical, []byte("{\n"))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	plan = adopt(t, ctx, canonical, []byte(string(canonical.Encode())+"\n"))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// The canonical record must be the inventory's current record.
	stale, _ := ticket.Decode(canonical.Encode())
	stale.Title = "stale copy"
	plan = adopt(t, ctx, stale, fileOf(t, canonical, func(r *ticket.Record) { r.Title = "t" }))
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	// A file identical to the canonical record adopts as an empty
	// composition.
	plan = adopt(t, ctx, canonical, canonical.Encode())
	want(t, plan, mutation.OutcomeCompleted, "")
	if len(plan.Composed) != 0 {
		t.Fatalf("identical file composed %v", plan.Composed)
	}
}

// TestTMV0007_AS35_ActorAndReplay: ADOPT_FILE needs an OWNER/OPERATOR
// binding; a WORKER or an unbound caller is refused before the file is read;
// an identical adoption request replays and a different file conflicts.
func TestTMV0007_AS35_ActorAndReplay(t *testing.T) {
	canonical := fixture.Ticket("AT-01")
	file := fileOf(t, canonical, func(r *ticket.Record) { r.Body = strPtr("hand-edited body") })
	for _, b := range []mutation.Binding{{}, worker, reviewer, importer, system, {ID: "russell", Role: "ROOT"}} {
		ctx := newCtx(t, b, nil, canonical)
		want(t, adopt(t, ctx, canonical, file), mutation.OutcomeUnauthorized, "")
	}
	ctx := newCtx(t, owner, nil, canonical)
	if p := mutation.Adopt(ctx, "", canonical, file); p.Outcome.Outcome != mutation.OutcomeValidationFailed {
		t.Fatalf("empty requestId: %s", p.Outcome.Outcome)
	}
	idx := mutation.NewMemoryIndex()
	ctx.Requests = idx
	first := adopt(t, ctx, canonical, file)
	want(t, first, mutation.OutcomeCompleted, "")
	if err := idx.Record(mutation.IndexEntry{RequestID: "adopt-1", MutationSha256: first.MutationSha256, Outcome: first.Outcome}); err != nil {
		t.Fatalf("record: %v", err)
	}
	again := adopt(t, ctx, canonical, file)
	if again.Outcome.Outcome != mutation.OutcomeCompleted || !again.Outcome.Replayed || again.Post != nil {
		t.Fatalf("replay: %+v", again.Outcome)
	}
	altered := fileOf(t, canonical, func(r *ticket.Record) { r.Body = strPtr("another body") })
	want(t, adopt(t, ctx, canonical, altered), mutation.OutcomeRequestIDConflict, wire.CodeRequestIDConflict)
	// The composed REFINE carried exactly the differing field.
	if first.Post.Body == nil || *first.Post.Body != "hand-edited body" || len(first.Composed) != 1 || first.Composed[0] != mutation.OpRefine {
		t.Fatalf("composition %v %v", first.Composed, first.Post.Body)
	}
}
