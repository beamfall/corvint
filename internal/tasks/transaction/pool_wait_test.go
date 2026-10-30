package transaction

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// waitPolicy is priorityPolicy whose "lanes" pool keeps a waitlist of at
// most maxWait seconds and, when grace is not empty, a hand-off grace.
func waitPolicy(t *testing.T, members []string, maxWait, grace string) []byte {
	t.Helper()
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", object("maxActiveAttempts", s("16"), "maxWorkersTotal", s("16"), "classes", wire.Array()))
	pool := object("id", s("lanes"), "members", wire.Strings(members))
	if maxWait != "" {
		pool.Obj.Set("maxWaitSeconds", s(maxWait))
	}
	if grace != "" {
		pool.Obj.Set("handoffGraceSeconds", s(grace))
	}
	v.Obj.Set("pools", wire.Array(pool))
	return wire.EncodeFile(v)
}

// wait builds a ticket-form POOL_WAIT of rec on pool for seconds.
func (f *priorityFixture) wait(rec *ticket.Record, pool, seconds string) leaseContext {
	return f.waitRequest(&LeaseRequest{Verb: LeasePoolWait, Pool: pool, WaitSeconds: wire.Count(seconds), TicketID: rec.TicketID.Raw, Holder: "builder"})
}

func (f *priorityFixture) waitRequest(l *LeaseRequest) leaseContext {
	inv, _ := NewInventory(nil, nil)
	c := leaseContext{r: admin(Lease, fmt.Sprintf("wait-%d", f.seq)), l: l, seq: wire.Size(fmt.Sprint(f.seq)), in: Input{Inventory: inv, RecordedAt: timestamp}, st: f.st}
	if raw, err := f.st.pools.Encode(); err == nil {
		c.in.Pools = raw
	}
	c.r.Lease = l
	return c
}

// acquire builds a pool acquire of a on pool.
func (f *priorityFixture) acquire(a *snapshot.Attempt, pool string) leaseContext {
	l := &LeaseRequest{Verb: LeasePoolAcquire, AttemptID: a.AttemptID, Generation: a.Generation, Pool: pool}
	return f.waitRequest(l)
}

func at(c leaseContext, seconds int64) leaseContext {
	c.in.RecordedAt = addSeconds(timestamp, seconds)
	return c
}

func waitRefused(t *testing.T, out leaseOutcome, code, detail string) {
	t.Helper()
	if out.result == nil || !slices.Contains(out.result.Outcome.Codes, code) || !strings.Contains(out.result.Detail, detail) || len(out.posts) != 0 {
		t.Fatalf("want %s refusal containing %q, got %+v posts %d", code, detail, out.result, len(out.posts))
	}
}

// PSR-V0-130: the request shape. A wait names a pool and a lifetime, and
// either a ticket and holder or an attempt and generation; the lifetime is
// 1..600 s; the preimage gains waitSeconds only for POOL_WAIT.
func TestPSRV0130_PoolWaitRequestShape(t *testing.T) {
	q, _ := wire.ParseQueueID("", fixture.QueueID)
	attempt := "attempt:acme:main:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ticketID := fixture.Ticket("AA-01").TicketID.Raw
	for _, ok := range []LeaseRequest{
		{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "600", TicketID: ticketID, Holder: "builder"},
		{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "1", AttemptID: attempt, Generation: "1"},
	} {
		v, e := leaseValue(&ok, q)
		if e != nil {
			t.Fatalf("%+v: %v", ok, e)
		}
		if got := string(wire.Encode(v)); !strings.Contains(got, `"waitSeconds":"`+string(ok.WaitSeconds)+`"`) {
			t.Fatalf("waitSeconds not bound: %s", got)
		}
	}
	for name, bad := range map[string]LeaseRequest{
		"no-seconds":   {Verb: LeasePoolWait, Pool: "lanes", TicketID: ticketID, Holder: "builder"},
		"no-pool":      {Verb: LeasePoolWait, WaitSeconds: "60", TicketID: ticketID, Holder: "builder"},
		"no-holder":    {Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "60", TicketID: ticketID},
		"both-forms":   {Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "60", TicketID: ticketID, Holder: "builder", AttemptID: attempt, Generation: "1"},
		"no-gen":       {Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "60", AttemptID: attempt},
		"seconds-else": {Verb: LeasePoolAcquire, Pool: "lanes", WaitSeconds: "60", AttemptID: attempt, Generation: "1"},
	} {
		if _, e := leaseValue(&bad, q); wire.CodeOf(e) != wire.CodeMalformed {
			t.Fatalf("%s: %v", name, e)
		}
	}
	for _, n := range []wire.Count{"0", "601"} {
		bad := LeaseRequest{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: n, TicketID: ticketID, Holder: "builder"}
		if _, e := leaseValue(&bad, q); wire.CodeOf(e) != wire.CodeLimitExceeded {
			t.Fatalf("waitSeconds %s: %v", n, e)
		}
	}
	// A claim's preimage is unchanged: no waitSeconds key.
	claim := LeaseRequest{Verb: LeaseClaim, TicketID: ticketID, Holder: "builder", LeaseMinutes: "60"}
	if v, e := leaseValue(&claim, q); e != nil || strings.Contains(string(wire.Encode(v)), "waitSeconds") {
		t.Fatalf("claim preimage %s %v", wire.Encode(v), e)
	}
}

// PSR-V0-130, PSR-V0-131: a wait records one entry per ticket and pool,
// a refresh keeps the original place, the lifetime is bounded by the pool's
// maxWaitSeconds, a pool without one keeps no waitlist, and only an OPEN
// ticket of that pool may wait.
func TestPSRV0131_WaitRegistersRefreshesAndBounds(t *testing.T) {
	hi, lo, other := priorityTicket("HI-01", "P1", "lanes"), priorityTicket("LO-01", "P2", "lanes"), priorityTicket("OT-01", "P1", "elsewhere")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "300", ""), []*ticket.Record{hi, lo, other})
	first := planPoolWait(f.wait(hi, "lanes", "120"))
	f.apply(t, first)
	w := f.st.pools.Waiters
	if len(w) != 1 || w[0].TicketID != hi.TicketID.Raw || w[0].Kind != snapshot.WaiterWait || w[0].Priority != "P1" || w[0].ExpiresAt != addSeconds(timestamp, 120) || w[0].AttemptID != "" {
		t.Fatalf("registered %+v", w)
	}
	seq := w[0].Seq
	f.apply(t, planPoolWait(at(f.wait(hi, "lanes", "300"), 60)))
	if w := f.st.pools.Waiters; len(w) != 1 || w[0].Seq != seq || w[0].Since != timestamp || w[0].ExpiresAt != addSeconds(timestamp, 360) {
		t.Fatalf("refresh %+v", w)
	}
	out := planPoolWait(f.wait(lo, "lanes", "301"))
	if out.result == nil || !slices.Contains(out.result.Outcome.Codes, wire.CodeLimitExceeded) {
		t.Fatalf("over maxWaitSeconds %+v", out.result)
	}
	waitRefused(t, planPoolWait(f.wait(other, "lanes", "60")), wire.CodeResourceCollision, "requires pool elsewhere")
	g := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "", ""), []*ticket.Record{hi})
	waitRefused(t, planPoolWait(g.wait(hi, "lanes", "60")), wire.CodeUnsupported, "declares no maxWaitSeconds")
	done := priorityTicket("DN-01", "P1", "lanes")
	done.Status = ticket.StatusCompleted
	h := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "300", ""), []*ticket.Record{done})
	waitRefused(t, planPoolWait(h.wait(done, "lanes", "60")), wire.CodeTicketState, "not OPEN")
}

// PSR-V0-132, PSR-V0-134: a freed member goes to the highest-priority
// waiter before any new claim. A lower-priority claim and an equal-priority
// later arrival are refused naming the head waiter; a higher-priority claim
// is not blocked; the waiter's own claim is admitted and consumes its entry.
func TestPSRV0132_FreedMemberGoesToHeadWaiter(t *testing.T) {
	aa, bb := priorityTicket("AA-01", "P0", ""), priorityTicket("BB-01", "P0", "")
	hi, mid, lo, top := priorityTicket("HI-01", "P1", "lanes"), priorityTicket("MD-01", "P1", "lanes"), priorityTicket("LO-01", "P2", "lanes"), priorityTicket("TP-01", "P0", "lanes")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a", "b"}, "600", ""), []*ticket.Record{aa, bb, hi, mid, lo, top})
	holder := f.admitted(t, f.claim(aa.TicketID.Raw, "lanes"))
	f.admitted(t, f.claim(bb.TicketID.Raw, "lanes"))
	f.apply(t, planPoolWait(f.wait(hi, "lanes", "600")))
	f.release(t, holder, "", "")
	f.confirmSafe(holder.PoolAllocation.MemberID)

	for _, rec := range []*ticket.Record{lo, mid} {
		waitRefused(t, planClaim(f.claim(rec.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "head waiter "+hi.TicketID.Raw+" (WAIT P1")
	}
	if out := planClaim(f.claim(top.TicketID.Raw, "lanes")); out.result != nil {
		t.Fatalf("higher-priority claim blocked: %+v", *out.result)
	}
	// Claim-next takes a P0 ticket (the released AA-01 leads plan order),
	// never the lower-priority LO-01 or the later MD-01.
	if out := planClaimNext(f.claim("", "lanes")); out.result != nil || attemptOf(t, out).TicketID.Raw != aa.TicketID.Raw {
		t.Fatalf("claim-next %+v chose %s", out.result, attemptOf(t, out).TicketID.Raw)
	}
	if got := PoolWaitlist(f.planInput(""), timestamp); len(got) != 1 || got[0].TicketID != hi.TicketID.Raw {
		t.Fatalf("waitlist %+v", got)
	}
	f.admitted(t, f.claim(hi.TicketID.Raw, "lanes"))
	if len(f.st.pools.Waiters) != 0 {
		t.Fatalf("entry not consumed: %+v", f.st.pools.Waiters)
	}
}

// PSR-V0-133: a dead entry never blocks. An expired entry, an entry whose
// ticket closed and an entry whose bound attempt ended all admit the
// competing claim, and the next lease write drops them.
func TestPSRV0133_DeadWaitersNeverBlock(t *testing.T) {
	hi, lo, side := priorityTicket("HI-01", "P1", "lanes"), priorityTicket("LO-01", "P2", "lanes"), priorityTicket("SD-01", "P1", "")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", ""), []*ticket.Record{hi, lo, side})
	f.apply(t, planPoolWait(f.wait(hi, "lanes", "60")))
	waitRefused(t, planClaim(f.claim(lo.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "head waiter")
	expiredOut := planClaim(at(f.claim(lo.TicketID.Raw, "lanes"), 61))
	if expiredOut.result != nil {
		t.Fatalf("expired entry blocked: %+v", *expiredOut.result)
	}
	if raw, ok := expiredOut.posts["pools.json"]; !ok || strings.Contains(string(raw), "waiters") {
		t.Fatalf("expired entry not pruned: %s", raw)
	}

	// An attempt-bound entry dies with its attempt.
	g := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", ""), []*ticket.Record{side, lo})
	a := g.admitted(t, g.claim(side.TicketID.Raw, ""))
	g.apply(t, planPoolWait(g.waitRequest(&LeaseRequest{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "600", AttemptID: a.AttemptID, Generation: a.Generation})))
	if w := g.st.pools.Waiters; len(w) != 1 || w[0].AttemptID != a.AttemptID || w[0].Holder != "builder" {
		t.Fatalf("attempt entry %+v", w)
	}
	waitRefused(t, planClaim(g.claim(lo.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "head waiter "+side.TicketID.Raw)
	g.release(t, a, "", "")
	if len(g.st.pools.Waiters) != 0 {
		t.Fatalf("release kept the entry: %+v", g.st.pools.Waiters)
	}
	if out := planClaim(g.claim(lo.TicketID.Raw, "lanes")); out.result != nil {
		t.Fatalf("ended attempt's entry blocked: %+v", *out.result)
	}

	// A closed ticket's entry is dead.
	closed := *hi
	closed.Status = ticket.StatusCompleted
	tickets, err := ticket.NewInventory(f.st.queue.QueueID, []*ticket.Record{&closed, lo, side})
	if err != nil {
		t.Fatal(err)
	}
	f.st.tickets = tickets
	if out := planClaim(f.claim(lo.TicketID.Raw, "lanes")); out.result != nil {
		t.Fatalf("closed ticket's entry blocked: %+v", *out.result)
	}
}

// PSR-V0-134: an attempt waiting on a full pool is served by its next
// acquire once a member frees, ahead of a lower-priority claim, and the
// acquire consumes the entry.
func TestPSRV0134_WaitingAttemptAcquiresFreedMember(t *testing.T) {
	aa, wt, lo := priorityTicket("AA-01", "P0", ""), priorityTicket("WT-01", "P1", ""), priorityTicket("LO-01", "P2", "lanes")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", ""), []*ticket.Record{aa, wt, lo})
	holder := f.admitted(t, f.claim(aa.TicketID.Raw, "lanes"))
	a := f.admitted(t, f.claim(wt.TicketID.Raw, ""))
	if out := planPoolAcquire(f.acquire(a, "lanes")); out.result == nil {
		t.Fatal("acquire on a full pool admitted")
	}
	f.apply(t, planPoolWait(f.waitRequest(&LeaseRequest{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "600", AttemptID: a.AttemptID, Generation: a.Generation})))
	f.release(t, holder, "", "")
	f.confirmSafe(holder.PoolAllocation.MemberID)
	waitRefused(t, planClaim(f.claim(lo.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "head waiter "+wt.TicketID.Raw)
	out := planPoolAcquire(f.acquire(a, "lanes"))
	f.apply(t, out)
	if got := attemptOf(t, out); got.PoolAllocation == nil || got.PoolAllocation.MemberID != "a" {
		t.Fatalf("acquire %+v", got.PoolAllocation)
	}
	if len(f.st.pools.Waiters) != 0 {
		t.Fatalf("entry not consumed: %+v", f.st.pools.Waiters)
	}
}

// PSR-V0-136: a clean HANDOFF release in a pool with handoffGraceSeconds
// reserves the freed capacity for the same ticket for the grace window; an
// equal-priority claim is refused, the ticket's next-stage claim is admitted,
// and without the field nothing is reserved.
func TestPSRV0136_HandoffGraceReservesForSuccessor(t *testing.T) {
	for _, grace := range []string{"120", ""} {
		t.Run("grace="+grace, func(t *testing.T) {
			rv, other := priorityTicket("RV-01", "P1", "lanes"), priorityTicket("OT-01", "P1", "lanes")
			f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", grace), []*ticket.Record{rv, other})
			impl := f.admitted(t, f.staged(rv.TicketID.Raw, "lanes", "implement"))
			f.release(t, impl, wire.CodeHandoff, "review")
			f.confirmSafe(impl.PoolAllocation.MemberID)
			if grace == "" {
				if len(f.st.pools.Waiters) != 0 {
					t.Fatalf("grace without the field: %+v", f.st.pools.Waiters)
				}
				if out := planClaim(f.claim(other.TicketID.Raw, "lanes")); out.result != nil {
					t.Fatalf("claim blocked without grace: %+v", *out.result)
				}
				return
			}
			if w := f.st.pools.Waiters; len(w) != 1 || w[0].Kind != snapshot.WaiterGrace || w[0].TicketID != rv.TicketID.Raw || w[0].ExpiresAt != addSeconds(timestamp, 120) {
				t.Fatalf("grace entry %+v", w)
			}
			waitRefused(t, planClaim(f.claim(other.TicketID.Raw, "lanes")), wire.CodeResourceCollision, "(GRACE P1")
			if out := planClaim(at(f.claim(other.TicketID.Raw, "lanes"), 121)); out.result != nil {
				t.Fatalf("expired grace blocked: %+v", *out.result)
			}
			review := f.admitted(t, f.staged(rv.TicketID.Raw, "lanes", "review"))
			if review.PoolAllocation == nil || len(f.st.pools.Waiters) != 0 {
				t.Fatalf("successor %+v waiters %+v", review.PoolAllocation, f.st.pools.Waiters)
			}
		})
	}
}

// PSR-V0-135: the read-only view lists live entries in admission order with
// their age, and omits dead ones.
func TestPSRV0135_WaitlistView(t *testing.T) {
	hi, lo := priorityTicket("HI-01", "P1", "lanes"), priorityTicket("LO-01", "P2", "lanes")
	f := newPriorityFixture(t, waitPolicy(t, []string{"a"}, "600", ""), []*ticket.Record{hi, lo})
	f.apply(t, planPoolWait(f.wait(lo, "lanes", "600")))
	f.apply(t, planPoolWait(f.wait(hi, "lanes", "30")))
	got := PoolWaitlist(f.planInput(""), addSeconds(timestamp, 10))
	if len(got) != 2 || got[0].TicketID != hi.TicketID.Raw || got[1].TicketID != lo.TicketID.Raw {
		t.Fatalf("order %+v", got)
	}
	v := string(wire.Encode(PoolWaiterValue(got[0], addSeconds(timestamp, 10))))
	if !strings.Contains(v, `"ageSeconds":"10"`) || !strings.Contains(v, `"kind":"WAIT"`) {
		t.Fatalf("value %s", v)
	}
	if got := PoolWaitlist(f.planInput(""), addSeconds(timestamp, 31)); len(got) != 1 || got[0].TicketID != lo.TicketID.Raw {
		t.Fatalf("expired entry listed: %+v", got)
	}
	if out := planPoolWait(f.waitRequest(&LeaseRequest{Verb: LeasePoolWait, Pool: "lanes", WaitSeconds: "60", TicketID: hi.TicketID.Raw, Holder: "builder"})); out.result != nil || out.effect == nil || out.effect.outcome != mutation.OutcomeCompleted {
		t.Fatalf("refresh %+v", out.result)
	}
}
