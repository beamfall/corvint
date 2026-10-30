package transaction

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// waitPriorityPolicy is waitPolicy whose pool also opts into priority
// admission (CAL-V0-101) when priority is set.
func waitPriorityPolicy(t *testing.T, members []string, maxWait, grace string, priority bool) []byte {
	t.Helper()
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", object("maxActiveAttempts", s("16"), "maxWorkersTotal", s("16"), "classes", wire.Array()))
	pool := object("id", s("lanes"), "members", wire.Strings(members), "maxWaitSeconds", s(maxWait))
	if grace != "" {
		pool.Obj.Set("handoffGraceSeconds", s(grace))
	}
	if priority {
		pool.Obj.Set("priorityAdmission", wire.Bool(true))
	}
	v.Obj.Set("pools", wire.Array(pool))
	return wire.EncodeFile(v)
}

// graceEntry injects a live GRACE entry for rec on pool lanes.
func (f *priorityFixture) graceEntry(rec *ticket.Record) {
	st := *f.st.pools
	st.Waiters = append(append([]snapshot.PoolWaiter{}, st.Waiters...), snapshot.PoolWaiter{PoolID: "lanes", TicketID: rec.TicketID.Raw, Kind: snapshot.WaiterGrace, Holder: "builder", Priority: rec.Priority, Seq: "1", Since: timestamp, ExpiresAt: addSeconds(timestamp, 120)})
	f.st.pools = &st
}

// PSR-V0-132 (review round): the waitlist and priority admission never make
// two claimants yield to each other. With one free member, an equal-priority
// ticket earlier in plan order yields to a waiter, and the waiter is then
// admitted although priority admission ranks the other ahead; a
// higher-priority ticket without an entry yields to a GRACE entry, and the
// GRACE ticket is then admitted. Plan and claim-next agree.
func TestPSRV0132_WaitlistOutranksPriorityAdmission(t *testing.T) {
	for _, priority := range []bool{true, false} {
		t.Run(fmt.Sprintf("priorityAdmission=%v", priority), func(t *testing.T) {
			aa, bb := priorityTicket("AA-02", "P1", "lanes"), priorityTicket("BB-02", "P1", "lanes")
			f := newPriorityFixture(t, waitPriorityPolicy(t, []string{"a"}, "600", "", priority), []*ticket.Record{aa, bb})
			f.apply(t, planPoolWait(f.wait(bb, "lanes", "600")))
			waitRefused(t, planClaim(f.claim(aa.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "head waiter "+bb.TicketID.Raw)
			if out := planClaim(f.claim(bb.TicketID.Raw, "lanes")); out.result != nil {
				t.Fatalf("waiter refused: %+v", *out.result)
			}
			in := f.planInput("lanes")
			in.Now = timestamp
			if got := PriorityFirst(in).ClaimNext("lanes"); got == nil || got.Ticket.TicketID.Raw != bb.TicketID.Raw {
				t.Fatalf("plan selects %+v", got)
			}

			top, gr := priorityTicket("TP-02", "P0", "lanes"), priorityTicket("GR-02", "P1", "lanes")
			g := newPriorityFixture(t, waitPriorityPolicy(t, []string{"a"}, "600", "120", priority), []*ticket.Record{top, gr})
			g.graceEntry(gr)
			waitRefused(t, planClaim(g.claim(top.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "head waiter "+gr.TicketID.Raw+" (GRACE")
			if out := planClaim(g.claim(gr.TicketID.Raw, "lanes")); out.result != nil {
				t.Fatalf("GRACE successor refused: %+v", *out.result)
			}
		})
	}
}

// PSR-V0-132 (review round): claim --next --pool never selects a ticket the
// waitlist refuses. With one free member and an equal-priority ticket ahead
// in plan order, claim-next claims the waiter; with no waiter it claims the
// plan's first ticket as before.
func TestPSRV0132_ClaimNextSelectsTheWaiter(t *testing.T) {
	aa, bb := priorityTicket("AA-02", "P1", "lanes"), priorityTicket("BB-02", "P1", "lanes")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", ""), []*ticket.Record{aa, bb})
	if out := planClaimNext(f.claim("", "lanes")); out.result != nil || attemptOf(t, out).TicketID.Raw != aa.TicketID.Raw {
		t.Fatalf("claim-next without waiters %+v", out.result)
	}
	f.apply(t, planPoolWait(f.wait(bb, "lanes", "600")))
	out := planClaimNext(f.claim("", "lanes"))
	if out.result != nil {
		t.Fatalf("claim-next refused: %+v", *out.result)
	}
	if got := attemptOf(t, out).TicketID.Raw; got != bb.TicketID.Raw {
		t.Fatalf("claim-next chose %s, want the waiter %s", got, bb.TicketID.Raw)
	}
	in := f.planInput("lanes")
	in.Now = timestamp
	plan := PriorityFirst(in)
	if e := plan.Entries[0]; e.Ticket.TicketID.Raw != aa.TicketID.Raw || e.State != PlanDeferred || e.Reason != wire.CodeResourceCollision || !slices.Equal(e.Blockers, []string{bb.TicketID.Raw}) {
		t.Fatalf("blocked entry %+v", e)
	}
	// After expiry the waiter no longer blocks and plan order applies again.
	if out := planClaimNext(at(f.claim("", "lanes"), 601)); out.result != nil || attemptOf(t, out).TicketID.Raw != aa.TicketID.Raw {
		t.Fatalf("claim-next after expiry %+v", out.result)
	}
}

// viaPlanLease plans c through the lease entry point, as Model does.
func viaPlanLease(c leaseContext) leaseOutcome {
	return planLease(c.r, c.in, c.st)
}

// PSR-V0-133 (review round): every pools.json write drops dead entries,
// including an early pool release and a confirm-safe, so rollback can
// remove the waiters key by one ordinary write.
func TestPSRV0133_EveryPoolsWriteDropsDeadWaiters(t *testing.T) {
	xx, ww := priorityTicket("XX-02", "P1", "lanes"), priorityTicket("WW-02", "P1", "lanes")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a", "b"}, "600", ""), []*ticket.Record{xx, ww})
	x := f.admitted(t, f.claim(xx.TicketID.Raw, "lanes"))
	f.apply(t, planPoolWait(f.wait(ww, "lanes", "60")))
	noWaiters := func(name string, out leaseOutcome) {
		t.Helper()
		if out.result != nil {
			t.Fatalf("%s refused: %+v", name, *out.result)
		}
		if raw, ok := out.posts["pools.json"]; !ok || strings.Contains(string(raw), "waiters") {
			t.Fatalf("%s kept the expired waiter: %s", name, raw)
		}
	}
	early := &LeaseRequest{Verb: LeasePoolRelease, AttemptID: x.AttemptID, Generation: x.Generation, Allocation: string(x.PoolAllocation.AllocationID)}
	noWaiters("pool release", viaPlanLease(at(f.waitRequest(early), 61)))

	f.release(t, x, "", "")
	if len(f.st.pools.Waiters) != 1 {
		t.Fatalf("live waiter dropped: %+v", f.st.pools.Waiters)
	}
	safe := &LeaseRequest{Verb: LeasePoolSafe, Member: x.PoolAllocation.MemberID, Allocation: string(x.PoolAllocation.AllocationID)}
	noWaiters("confirm-safe", viaPlanLease(at(f.waitRequest(safe), 61)))
}

// PSR-V0-136 (review round): a full waitlist refuses a HANDOFF release whose
// GRACE entry it cannot hold, with LIMIT_EXCEEDED and nothing written; a
// release with another reason is unaffected.
func TestPSRV0136_FullWaitlistRefusesHandoffGrace(t *testing.T) {
	rv := priorityTicket("RV-02", "P1", "lanes")
	recs := []*ticket.Record{rv}
	waiters := []snapshot.PoolWaiter{}
	for i := 0; i < snapshot.MaxPoolWaiters; i++ {
		w := priorityTicket(fmt.Sprintf("WT-%03d", i+100), "P2", "lanes")
		recs = append(recs, w)
		waiters = append(waiters, snapshot.PoolWaiter{PoolID: "lanes", TicketID: w.TicketID.Raw, Kind: snapshot.WaiterWait, Holder: "builder", Priority: "P2", Seq: "1", Since: timestamp, ExpiresAt: addSeconds(timestamp, 600)})
	}
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", "120"), recs)
	impl := f.admitted(t, f.staged(rv.TicketID.Raw, "lanes", "implement"))
	st := *f.st.pools
	st.Waiters = waiters
	f.st.pools = &st
	release := func(reason string) leaseOutcome {
		l := &LeaseRequest{Verb: LeaseRelease, AttemptID: impl.AttemptID, Generation: impl.Generation, Reason: reason}
		if reason == wire.CodeHandoff {
			l.Evidence, l.HandoffTo = "handoff-note", "review"
		}
		return planRelease(f.waitRequest(l))
	}
	out := release(wire.CodeHandoff)
	if out.result == nil || !slices.Contains(out.result.Outcome.Codes, wire.CodeLimitExceeded) || !strings.Contains(out.result.Detail, "GRACE") || len(out.posts) != 0 {
		t.Fatalf("full waitlist HANDOFF release %+v posts %d", out.result, len(out.posts))
	}
	if out := release(""); out.result != nil {
		t.Fatalf("plain release refused at capacity: %+v", *out.result)
	}
}

// PSR-V0-135 (review round): a rendered entry carries every required field.
func TestPSRV0135_WaiterValueCarriesEveryField(t *testing.T) {
	side, hi := priorityTicket("SD-02", "P1", ""), priorityTicket("HI-02", "P1", "lanes")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", ""), []*ticket.Record{side, hi})
	a := f.admitted(t, f.claim(side.TicketID.Raw, ""))
	f.apply(t, planPoolWait(f.wait(hi, "lanes", "600")))
	f.apply(t, planPoolWait(f.waitRequest(&LeaseRequest{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "300", AttemptID: a.AttemptID, Generation: a.Generation})))
	now := addSeconds(timestamp, 10)
	got := PoolWaitlist(f.planInput(""), now)
	if len(got) != 2 {
		t.Fatalf("waitlist %+v", got)
	}
	for _, w := range got {
		v := PoolWaiterValue(w, now)
		want := map[string]string{"poolId": "lanes", "ticketId": w.TicketID, "kind": "WAIT", "holder": "builder", "priority": "P1", "seq": string(w.Seq), "since": string(timestamp), "expiresAt": string(w.ExpiresAt), "ageSeconds": "10"}
		if w.AttemptID != "" {
			want["attemptId"], want["generation"] = a.AttemptID, string(a.Generation)
		}
		if keys := v.Obj.Keys; len(keys) != len(want) {
			t.Fatalf("keys %v, want %d", keys, len(want))
		}
		for k, x := range want {
			if f, ok := v.Obj.Get(k); !ok || f.Str != x {
				t.Fatalf("%s: %s = %+v, want %q", w.TicketID, k, f, x)
			}
		}
	}
}
