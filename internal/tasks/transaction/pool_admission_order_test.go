package transaction

import (
	"fmt"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// staged is f.claim with a requested stage.
func (f *priorityFixture) staged(id, pool, stage string) leaseContext {
	c := f.claim(id, pool)
	c.l.Stage = stage
	return c
}

// release builds and applies a RELEASE of a; reason HANDOFF records the
// external-evidence handoff to to.
func (f *priorityFixture) release(t *testing.T, a *snapshot.Attempt, reason, to string) *snapshot.Attempt {
	t.Helper()
	l := &LeaseRequest{Verb: LeaseRelease, AttemptID: a.AttemptID, Generation: a.Generation, Reason: reason, HandoffTo: to}
	if reason == wire.CodeHandoff {
		l.Evidence = "handoff-note"
	}
	inv, _ := NewInventory(nil, nil)
	c := leaseContext{r: admin(Lease, fmt.Sprintf("release-%d", f.seq)), l: l, seq: wire.Size(fmt.Sprint(f.seq)), in: Input{Inventory: inv, RecordedAt: timestamp}, st: f.st}
	c.r.Lease = l
	out := planRelease(c)
	f.apply(t, out)
	return attemptOf(t, out)
}

// confirmSafe frees member, as a pool confirm-safe of its quarantined
// allocation would.
func (f *priorityFixture) confirmSafe(member string) {
	st := *f.st.pools
	st.Entries = nil
	for _, en := range f.st.pools.Entries {
		if en.MemberID != member {
			st.Entries = append(st.Entries, en)
		}
	}
	f.st.pools = &st
}

// admitted applies a claim that must be admitted and returns its attempt.
func (f *priorityFixture) admitted(t *testing.T, c leaseContext) *snapshot.Attempt {
	t.Helper()
	out := planClaim(c)
	f.apply(t, out)
	return attemptOf(t, out)
}

// CAL-V0-105 (issue 626): a review handoff waits while no member is free; an
// implement claim for an equal-priority ticket earlier in plan order arrives;
// when a member frees, the implement claim yields to the waiting review, the
// plans and claim-next agree, and the review claim is admitted. Without the
// flag the implement claim takes the member, as before.
func TestCALV0105_WaitingReviewOutranksArrivingImplement(t *testing.T) {
	for _, flag := range []string{"true", ""} {
		t.Run("flag="+flag, func(t *testing.T) {
			// AA-01 and AB-01 need no pool, so they occupy members without
			// competing; FA-04 precedes FP-02 in plan order.
			aa, ab := priorityTicket("AA-01", "P0", ""), priorityTicket("AB-01", "P0", "")
			fa, fp := priorityTicket("FA-04", "P0", "lanes"), priorityTicket("FP-02", "P0", "lanes")
			f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b", "c"}, flag), []*ticket.Record{aa, ab, fa, fp})
			holder := f.admitted(t, f.claim(aa.TicketID.Raw, "lanes"))
			impl := f.admitted(t, f.staged(fp.TicketID.Raw, "lanes", "implement"))
			f.admitted(t, f.claim(ab.TicketID.Raw, "lanes"))
			handed := f.release(t, impl, wire.CodeHandoff, "review")
			since := handed.PhaseSinceSeq.Uint64()

			// No member is free: both claims meet the ordinary refusal.
			for _, c := range []leaseContext{f.staged(fa.TicketID.Raw, "lanes", "implement"), f.staged(fp.TicketID.Raw, "lanes", "review")} {
				if out := planClaim(c); out.result == nil || yieldTarget(out) != "" {
					t.Fatalf("full pool: %+v", out.result)
				}
			}
			f.release(t, holder, "", "")
			f.confirmSafe(holder.PoolAllocation.MemberID)

			out := planClaim(f.staged(fa.TicketID.Raw, "lanes", "implement"))
			if flag == "" {
				if out.result != nil {
					t.Fatalf("flag off: implement refused %+v", *out.result)
				}
				return
			}
			want := fmt.Sprintf("pool lanes priority admission: 1 higher-priority waiting ticket(s) for 1 free eligible member(s); yields to %[1]s; %[1]s awaits review since seq %[2]d", fp.TicketID.Raw, since)
			if out.result == nil || out.result.Detail != want || len(out.posts) != 0 {
				t.Fatalf("implement claim: %+v", out)
			}
			pooled := PriorityFirst(f.planInput("lanes"))
			for _, e := range pooled.Entries {
				switch e.Ticket.TicketID.Raw {
				case fa.TicketID.Raw:
					if e.yieldTo != fp.TicketID.Raw {
						t.Fatalf("--pool plan FA-04: %+v", e)
					}
				case fp.TicketID.Raw:
					if e.State != PlanSelected {
						t.Fatalf("--pool plan FP-02: %+v", e)
					}
				}
			}
			if v, code := RecordedClaimability(f.planInput(""), fa); string(wire.Encode(v)) != "false" || code != wire.CodeResourceCollision {
				t.Fatalf("FA-04 claimability %s %s", wire.Encode(v), code)
			}
			if v, code := RecordedClaimability(f.planInput(""), fp); string(wire.Encode(v)) != "true" {
				t.Fatalf("FP-02 claimability %s %s", wire.Encode(v), code)
			}
			if next := planClaimNext(f.claim("", "lanes")); next.result != nil || attemptOf(t, next).TicketID.Raw != fp.TicketID.Raw {
				t.Fatalf("claim-next: %+v", next.result)
			}
			review := f.admitted(t, f.staged(fp.TicketID.Raw, "lanes", "review"))
			if review.Stage != "review" || review.PoolAllocation == nil || review.PoolAllocation.MemberID != holder.PoolAllocation.MemberID {
				t.Fatalf("review admitted on %+v at stage %s", review.PoolAllocation, review.Stage)
			}
		})
	}
}

// CAL-V0-105: at equal priority two downstream tickets are admitted in
// handoff order, not plan order, and the plan, an explicit claim and
// claim-next agree.
func TestCALV0105_EarlierHandoffWinsAtEqualPriority(t *testing.T) {
	ra, rb := priorityTicket("RA-01", "P0", "lanes"), priorityTicket("RB-01", "P0", "lanes")
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b", "c"}, "true"), []*ticket.Record{ra, rb})
	b := f.admitted(t, f.staged(rb.TicketID.Raw, "lanes", "implement"))
	a := f.admitted(t, f.staged(ra.TicketID.Raw, "lanes", "implement"))
	first := f.release(t, b, wire.CodeHandoff, "review")
	f.release(t, a, wire.CodeHandoff, "integrate")

	out := planClaim(f.staged(ra.TicketID.Raw, "lanes", "integrate"))
	if want := fmt.Sprintf("pool lanes priority admission: 1 higher-priority waiting ticket(s) for 1 free eligible member(s); yields to %[1]s; %[1]s awaits review since seq %[2]d", rb.TicketID.Raw, first.PhaseSinceSeq.Uint64()); out.result == nil || out.result.Detail != want {
		t.Fatalf("RA-01: %+v", out.result)
	}
	if waiting, _ := priorityWaiting(f.planInput("lanes"), rb, "lanes"); len(waiting) != 0 {
		t.Fatalf("RB-01 waits behind %v", waiting)
	}
	plan := PriorityFirst(f.planInput("lanes"))
	if chosen := plan.ClaimNext("lanes"); chosen == nil || chosen.Ticket.TicketID.Raw != rb.TicketID.Raw {
		t.Fatalf("plan chose %+v", chosen)
	}
	if next := planClaimNext(f.claim("", "lanes")); next.result != nil || attemptOf(t, next).TicketID.Raw != rb.TicketID.Raw {
		t.Fatalf("claim-next: %+v", next.result)
	}
}

// CAL-V0-105: the derived rank. Only a terminal handoff to review or
// integrate at the current acceptance revision ranks downstream; priority
// still decides first.
func TestCALV0105_AdmissionRank(t *testing.T) {
	rec := priorityTicket("RV-01", "P1", "lanes")
	att := func(phase, disposition, to, revision string, since uint64) map[string]*snapshot.Attempt {
		return map[string]*snapshot.Attempt{"x": {TicketID: rec.TicketID, TicketRevision: wire.Count(revision), Generation: "1", Phase: phase, PhaseSinceSeq: wire.SizeOf(since), RetryAccounting: &snapshot.RetryAccounting{Disposition: disposition}, HandoffTo: to}}
	}
	cur := string(rec.AcceptanceRevision)
	cases := []struct {
		name     string
		attempts map[string]*snapshot.Attempt
		stage    string
	}{
		{"none", nil, ""},
		{"review", att("CANCELLED", wire.CodeHandoff, "review", cur, 9), "review"},
		{"integrate", att("CANCELLED", wire.CodeHandoff, "integrate", cur, 9), "integrate"},
		{"implement", att("CANCELLED", wire.CodeHandoff, "implement", cur, 9), ""},
		{"returned", att("CANCELLED", wire.CodeReviewReturned, "", cur, 9), ""},
		{"live", att("RUNNING", wire.CodeHandoff, "review", cur, 9), ""},
		{"stale", att("CANCELLED", wire.CodeHandoff, "review", cur+"0", 9), ""},
	}
	for _, tc := range cases {
		if r := admissionRankOf(tc.attempts, rec); r.stage != tc.stage || (r.stage != "") != (r.since == 9) {
			t.Fatalf("%s: rank %+v", tc.name, r)
		}
	}
	review := admissionRankOf(att("CANCELLED", wire.CodeHandoff, "review", cur, 9), rec)
	hi := admissionRank{rec: priorityTicket("AA-00", "P0", "lanes")}
	lo := admissionRank{rec: priorityTicket("AA-01", "P1", "lanes")}
	if !admissionLess(hi, review) || admissionLess(review, hi) {
		t.Fatal("a P1 review outranked a P0 implement")
	}
	if !admissionLess(review, lo) || admissionLess(lo, review) {
		t.Fatal("an equal-priority implement outranked a waiting review")
	}
}

// CAL-V0-105: a downstream competitor counts only when its own stage has a
// free eligible member of the pool, and every path reads it that way. With
// the only free member reserved for implement, a waiting review cannot take
// it, so the default plan, the --pool plan, claim-next, claimability and an
// explicit claim all admit the equal-priority implement ticket.
func TestCALV0105_DownstreamCompetitorNeedsItsStageMember(t *testing.T) {
	aa, ab := priorityTicket("AA-01", "P0", ""), priorityTicket("AB-01", "P0", "")
	fa, rv := priorityTicket("FA-01", "P0", "lanes"), priorityTicket("RV-02", "P0", "lanes")
	f := newPriorityFixture(t, priorityPolicy(t, []string{"a", "b", "c"}, "true"), []*ticket.Record{aa, ab, fa, rv})
	f.st.policy.Pool("lanes").ReservedFor["c"] = "implement"
	impl := f.admitted(t, f.staged(rv.TicketID.Raw, "lanes", "implement"))
	if impl.PoolAllocation == nil || impl.PoolAllocation.MemberID != "c" {
		t.Fatalf("implement on %+v", impl.PoolAllocation)
	}
	f.admitted(t, f.claim(aa.TicketID.Raw, "lanes"))
	f.admitted(t, f.claim(ab.TicketID.Raw, "lanes"))
	f.release(t, impl, wire.CodeHandoff, "review")
	f.confirmSafe("c")

	for _, pool := range []string{"", "lanes"} {
		in := f.planInput(pool)
		in.Stage = "implement"
		if waiting, unobserved := priorityWaiting(in, fa, "lanes"); len(waiting)+len(unobserved) != 0 {
			t.Fatalf("pool %q: FA-01 waits behind %v %v", pool, waiting, unobserved)
		}
		for _, e := range PriorityFirst(in).Entries {
			if e.Ticket.TicketID.Raw == fa.TicketID.Raw && (e.State != PlanSelected || e.yieldTo != "") {
				t.Fatalf("pool %q plan FA-01: %+v", pool, e)
			}
		}
	}
	in := f.planInput("")
	in.Stage = "implement"
	if v, code := RecordedClaimability(in, fa); string(wire.Encode(v)) != "true" {
		t.Fatalf("FA-01 claimability %s %s", wire.Encode(v), code)
	}
	if next := planClaimNext(f.staged("", "lanes", "implement")); next.result != nil || attemptOf(t, next).TicketID.Raw != fa.TicketID.Raw {
		t.Fatalf("claim-next: %+v", next.result)
	}
	if got := f.admitted(t, f.staged(fa.TicketID.Raw, "lanes", "implement")); got.PoolAllocation == nil || got.PoolAllocation.MemberID != "c" {
		t.Fatalf("FA-01 admitted on %+v", got.PoolAllocation)
	}
}
