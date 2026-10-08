package mutation_test

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// workerKnowHowPolicy renders the fixture policy with knowHow.workerAdd set,
// or without the key when enabled is nil.
func workerKnowHowPolicy(enabled *bool) []byte {
	pv := fixture.PolicyValue()
	if enabled != nil {
		pv.Obj.Set("knowHow", obj("workerAdd", wire.Bool(*enabled)))
	}
	return wire.EncodeFile(pv)
}

// workerAddPayload is a fresh KNOWHOW_ADD naming attempt and generation.
func workerAddPayload(attempt, generation string, anchors ...wire.Value) wire.Value {
	p := knowHowAdd("the generated parser must be rebuilt first", "", "", anchors...)
	p.Obj.Set("attempt", strOrNull(attempt))
	p.Obj.Set("generation", strOrNull(generation))
	return p
}

// scopedTicket is AT-01 with touchPaths a directory prefix and one file.
func scopedTicket() *ticket.Record {
	rec := fixture.Ticket("AT-01")
	rec.Effects.TouchPaths = []string{"b.go", "internal/a/"}
	return rec
}

func liveAttempt() *mutation.WorkerAttemptObservation {
	return &mutation.WorkerAttemptObservation{Live: true, TicketID: fixture.TicketID("AT-01"), Generation: "3", Holder: worker.ID}
}

func workerPlan(t *testing.T, policy []byte, obs *mutation.WorkerAttemptObservation, rec *ticket.Record, op string, payload wire.Value) *mutation.Plan {
	t.Helper()
	ctx := ctxWithPolicy(t, worker, attempts{fixture.TicketID("AT-01"): true}, policy, rec)
	ctx.WorkerAttempt = obs
	return apply(t, ctx, envelope("w1", worker, "AT-01", string(rec.Revision), op, payload))
}

func wantDetail(t *testing.T, plan *mutation.Plan, prefix string) {
	t.Helper()
	if !strings.HasPrefix(plan.Detail, prefix+":") {
		t.Fatalf("detail %q; want prefix %s", plan.Detail, prefix)
	}
}

// TestKHNV0021_WorkerAddIsPolicyOptIn: without knowHow.workerAdd, or with it
// false, a WORKER KNOWHOW_ADD is refused exactly as before, even when it names
// its own live attempt in scope; RETRACT and every other non-body write stay
// refused with the key true; a WORKER roles row still outranks the key; OWNER
// and OPERATOR behave as before.
func TestKHNV0021_WorkerAddIsPolicyOptIn(t *testing.T) {
	off, on := false, true
	good := workerAddPayload("att-1", "3", khAnchor("b.go", khBlobA))
	for name, policy := range map[string][]byte{"absent": workerKnowHowPolicy(nil), "false": workerKnowHowPolicy(&off)} {
		t.Run(name, func(t *testing.T) {
			want(t, workerPlan(t, policy, liveAttempt(), scopedTicket(), mutation.OpKnowHowAdd, good), mutation.OutcomeUnauthorized, "")
		})
	}
	enabled := workerKnowHowPolicy(&on)
	withNote := scopedTicket()
	withNote.KnowHow = []ticket.KnowHowEntry{{Seq: wire.CountOf(1), Operation: ticket.KnowHowAdd, Text: "n",
		Anchors: []ticket.KnowHowAnchor{{Path: "b.go", Blob: khBlobA}}, Routes: []string{}, Commit: khCommit, ActorID: "russell", ActorRole: "OWNER", RecordedAt: now}}
	want(t, workerPlan(t, enabled, liveAttempt(), withNote, mutation.OpKnowHowRetract, obj("note", str("1"), "reason", str("wrong"))), mutation.OutcomeUnauthorized, "")
	want(t, workerPlan(t, enabled, liveAttempt(), scopedTicket(), mutation.OpRefine, obj("title", str("renamed"))), mutation.OutcomeUnauthorized, "")

	pv := fixture.PolicyValue()
	pv.Obj.Set("knowHow", obj("workerAdd", wire.Bool(true)))
	pv.Obj.Set("roles", obj("WORKER", wire.Strings([]string{"REFINE"})))
	want(t, workerPlan(t, wire.EncodeFile(pv), liveAttempt(), scopedTicket(), mutation.OpKnowHowAdd, good), mutation.OutcomeUnauthorized, "")

	ownerCtx := ctxWithPolicy(t, owner, nil, enabled, scopedTicket())
	plan := apply(t, ownerCtx, envelope("o1", owner, "AT-01", string(scopedTicket().Revision), mutation.OpKnowHowAdd, knowHowAdd("t", "", "", khAnchor("z.go", khBlobA))))
	want(t, plan, mutation.OutcomeCompleted, "")
	opCtx := ctxWithPolicy(t, operator, nil, enabled, scopedTicket())
	want(t, apply(t, opCtx, envelope("p1", operator, "AT-01", string(scopedTicket().Revision), mutation.OpKnowHowAdd, good)), mutation.OutcomeUnauthorized, "")
}

// TestKHNV0022_WorkerAddScope: with knowHow.workerAdd true a WORKER adds a
// note on the ticket of the live attempt it holds, at that generation, with
// every anchor inside effects.touchPaths; the entry records WORKER, the
// attempt and the generation. Each scope failure has its own stable detail
// prefix, and the ordinary cap and secret screen still apply.
func TestKHNV0022_WorkerAddScope(t *testing.T) {
	on := true
	enabled := workerKnowHowPolicy(&on)
	inScope := []wire.Value{khAnchor("b.go", khBlobA), khAnchor("internal/a/x.go", khBlobB)}

	plan := workerPlan(t, enabled, liveAttempt(), scopedTicket(), mutation.OpKnowHowAdd, workerAddPayload("att-1", "3", inScope...))
	want(t, plan, mutation.OutcomeCompleted, "")
	e := plan.Post.KnowHow[0]
	if e.ActorID != worker.ID || e.ActorRole != "WORKER" || e.Attempt == nil || *e.Attempt != "att-1" || e.Generation == nil || *e.Generation != "3" {
		t.Fatalf("worker entry: %+v", e)
	}
	sameExceptKnowHow(t, scopedTicket(), plan.Post)
	if _, err := ticket.Decode(plan.Post.Encode()); err != nil {
		t.Fatalf("worker entry does not round-trip: %v", err)
	}

	other := liveAttempt()
	other.TicketID = fixture.TicketID("AT-02")
	foreign := liveAttempt()
	foreign.Holder = "lane-2"
	ended := liveAttempt()
	ended.Live = false
	withNote := scopedTicket()
	withNote.KnowHow = append(withNote.KnowHow, plan.Post.KnowHow...)
	supersede := workerAddPayload("att-1", "3", khAnchor("b.go", khBlobA))
	supersede.Obj.Set("supersedes", str("1"))
	supersede.Obj.Set("reason", str("corrected"))
	cases := []struct {
		name          string
		obs           *mutation.WorkerAttemptObservation
		rec           *ticket.Record
		payload       wire.Value
		outcome, code string
		prefix        string
	}{
		{"supersede", liveAttempt(), withNote, supersede, mutation.OutcomeUnauthorized, "", mutation.KnowHowWorkerSupersede},
		{"no attempt", liveAttempt(), scopedTicket(), workerAddPayload("", "3", inScope...), mutation.OutcomeValidationFailed, wire.CodeMalformed, mutation.KnowHowWorkerAttemptRequired},
		{"no generation", liveAttempt(), scopedTicket(), workerAddPayload("att-1", "", inScope...), mutation.OutcomeValidationFailed, wire.CodeMalformed, mutation.KnowHowWorkerAttemptRequired},
		{"unobserved attempt", nil, scopedTicket(), workerAddPayload("att-1", "3", inScope...), mutation.OutcomeRevisionConflict, wire.CodeFenced, mutation.KnowHowWorkerAttemptStale},
		{"ended attempt", ended, scopedTicket(), workerAddPayload("att-1", "3", inScope...), mutation.OutcomeRevisionConflict, wire.CodeFenced, mutation.KnowHowWorkerAttemptStale},
		{"stale generation", liveAttempt(), scopedTicket(), workerAddPayload("att-1", "2", inScope...), mutation.OutcomeRevisionConflict, wire.CodeFenced, mutation.KnowHowWorkerAttemptStale},
		{"foreign holder", foreign, scopedTicket(), workerAddPayload("att-1", "3", inScope...), mutation.OutcomeUnauthorized, "", mutation.KnowHowWorkerAttemptForeign},
		{"other ticket", other, scopedTicket(), workerAddPayload("att-1", "3", inScope...), mutation.OutcomeBlocked, wire.CodeOutOfScope, mutation.KnowHowWorkerOtherTicket},
		{"anchor out of scope", liveAttempt(), scopedTicket(), workerAddPayload("att-1", "3", khAnchor("b.go", khBlobA), khAnchor("internal/ab.go", khBlobB)), mutation.OutcomeBlocked, wire.CodeOutOfScope, mutation.KnowHowWorkerAnchorScope},
		{"no touchPaths", liveAttempt(), fixture.Ticket("AT-01"), workerAddPayload("att-1", "3", khAnchor("b.go", khBlobA)), mutation.OutcomeBlocked, wire.CodeOutOfScope, mutation.KnowHowWorkerAnchorScope},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := workerPlan(t, enabled, c.obs, c.rec, mutation.OpKnowHowAdd, c.payload)
			want(t, plan, c.outcome, c.code)
			wantDetail(t, plan, c.prefix)
		})
	}

	secret := workerAddPayload("att-1", "3", inScope...)
	secret.Obj.Set("text", str("token AKIAABCDEFGHIJKLMNOP"))
	plan = workerPlan(t, enabled, liveAttempt(), scopedTicket(), mutation.OpKnowHowAdd, secret)
	want(t, plan, mutation.OutcomeValidationFailed, wire.CodeMalformed)
	wantDetail(t, plan, mutation.KnowHowSecretDetail)

	full := scopedTicket()
	for i := 0; i < wire.KnowHowMaxEntries; i++ {
		full.KnowHow = append(full.KnowHow, ticket.KnowHowEntry{Seq: wire.CountOf(int64(i + 1)), Operation: ticket.KnowHowAdd, Text: "n",
			Anchors: []ticket.KnowHowAnchor{{Path: "b.go", Blob: khBlobA}}, Routes: []string{}, Commit: khCommit, ActorID: "russell", ActorRole: "OWNER", RecordedAt: now})
	}
	want(t, workerPlan(t, enabled, liveAttempt(), full, mutation.OpKnowHowAdd, workerAddPayload("att-1", "3", inScope...)), mutation.OutcomeValidationFailed, wire.CodeLimitExceeded)
}
