package transaction

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// This file uses only API that predates CAL-V0-101, so the N-1 digest below
// can be recomputed on the pre-change source.

// priorityPolicy is the fixture policy with no enforced budget fields,
// capacity 16 and one pool "lanes" over members; flag, when not empty, is
// the raw priorityAdmission member.
func priorityPolicy(t *testing.T, members []string, flag string) []byte {
	t.Helper()
	v := fixture.PolicyValue()
	b, _ := v.Obj.Get("budgets")
	b.Obj.Set("requireEnforcedFields", wire.Array())
	v.Obj.Set("capacity", object("maxActiveAttempts", s("16"), "maxWorkersTotal", s("16"), "classes", wire.Array()))
	pool := object("id", s("lanes"), "members", wire.Strings(members))
	if flag != "" {
		x, err := wire.Parse([]byte(flag + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		pool.Obj.Set("priorityAdmission", x)
	}
	v.Obj.Set("pools", wire.Array(pool))
	return wire.EncodeFile(v)
}

// priorityTicket is an OPEN fixture ticket touching its own path.
func priorityTicket(local, priority, pool string) *ticket.Record {
	rec := fixture.Ticket(local)
	rec.Priority, rec.RequiresPool = priority, pool
	rec.Effects.TouchPaths = []string{strings.ToLower(local) + "/"}
	return rec
}

// priorityFixture is pure lease state: claims apply their posts to it as a
// commit would, so later claims see their reservations and members.
type priorityFixture struct {
	st  inputState
	seq int
}

func newPriorityFixture(t *testing.T, policy []byte, recs []*ticket.Record) *priorityFixture {
	t.Helper()
	q, err := intent.DecodeQueue(fixture.QueueBytes())
	if err != nil {
		t.Fatal(err)
	}
	p, err := intent.DecodePolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	tickets, err := ticket.NewInventory(q.QueueID, recs)
	if err != nil {
		t.Fatal(err)
	}
	return &priorityFixture{seq: 2, st: inputState{queue: q, policy: p, tickets: tickets, head: &snapshot.Head{Generation: "0", LastSeq: "1"},
		pools: &snapshot.PoolState{QueueID: q.QueueID, Entries: []snapshot.PoolEntry{}}, reservations: &snapshot.ReservationSet{QueueID: q.QueueID, Entries: []snapshot.ReservationEntry{}}, attempts: map[string]*snapshot.Attempt{}}}
}

// claim builds the context of a claim of id (or claim-next when id is
// empty) on pool, as the next transaction.
func (f *priorityFixture) claim(id, pool string) leaseContext {
	verb := LeaseClaim
	if id == "" {
		verb = LeaseClaimNext
	}
	l := &LeaseRequest{Verb: verb, TicketID: id, Holder: "builder", LeaseMinutes: "60", Pool: pool}
	inv, _ := NewInventory(nil, nil)
	attempt := fmt.Sprintf("attempt:acme:main:%032x", f.seq)
	c := leaseContext{r: admin(Lease, fmt.Sprintf("claim-%d", f.seq)), l: l, seq: wire.Size(fmt.Sprint(f.seq)), in: Input{Inventory: inv, RecordedAt: timestamp, LeaseFacts: LeaseFacts{AttemptID: attempt, BaseCommit: strings.Repeat("a", 40)}}, st: f.st}
	c.r.Lease = l
	return c
}

// apply commits an admitted claim's posts into the fixture state.
func (f *priorityFixture) apply(t *testing.T, out leaseOutcome) {
	t.Helper()
	if out.result != nil {
		t.Fatalf("claim refused: %+v", *out.result)
	}
	st := f.st
	st.attempts = map[string]*snapshot.Attempt{}
	for k, v := range f.st.attempts {
		st.attempts[k] = v
	}
	for path, raw := range out.posts {
		var err error
		switch {
		case path == "pools.json":
			st.pools, err = snapshot.DecodePools(raw)
		case path == "reservations.json":
			st.reservations, err = snapshot.DecodeReservations(raw)
		case strings.HasPrefix(path, "attempts/"):
			var a *snapshot.Attempt
			if a, err = snapshot.DecodeAttempt(raw); err == nil {
				st.attempts[a.AttemptID] = a
			}
		}
		if err != nil {
			t.Fatal(path, err)
		}
	}
	f.st = st
	f.seq++
	f.st.head = &snapshot.Head{Generation: "0", LastSeq: wire.Size(fmt.Sprint(f.seq - 1))}
}

func (f *priorityFixture) planInput(pool string) PlanInput {
	return PlanInput{Pool: pool, Pools: f.st.pools, Queue: f.st.queue, Policy: f.st.policy, Tickets: f.st.tickets, Reservations: f.st.reservations, Attempts: f.st.attempts}
}

// outcomeTranscript is a claim outcome's posted bytes and refusal.
func outcomeTranscript(out leaseOutcome) string {
	var b strings.Builder
	paths := make([]string, 0, len(out.posts))
	for p := range out.posts {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(&b, "%s\x00%d\x00%s\x00", p, len(out.posts[p]), out.posts[p])
	}
	if out.result != nil {
		fmt.Fprintf(&b, "refused\x00%s\x00%v\x00%s\x00", out.result.Kind, out.result.Outcome, out.result.Detail)
	}
	return b.String()
}

func planDigest(plan TicketPlan) string {
	h := sha256.New()
	for _, e := range plan.Entries {
		fmt.Fprintf(h, "%s %s %s %v\n", e.Ticket.TicketID.Raw, e.State, e.Reason, e.Blockers)
	}
	for _, p := range plan.Pools {
		fmt.Fprintf(h, "%+v\n", p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// priorityScenario is the fixed N-1 scenario: AA-01 (P0, lanes) leads the
// plan, HI-01 (P0, lanes) waits behind it, FR-01 (P1) needs no pool and
// LO-01 (P2, lanes) comes last.
func priorityScenario() (aa, hi, fr, lo *ticket.Record) {
	return priorityTicket("AA-01", "P0", "lanes"), priorityTicket("HI-01", "P0", "lanes"), priorityTicket("FR-01", "P1", ""), priorityTicket("LO-01", "P2", "lanes")
}

// priorityN1Transcript runs the scenario under policy: AA-01 takes a lanes
// member, then LO-01 claims the last free member ahead of HI-01, claim-next
// runs with and without --pool, and both plans and every recorded
// claimability are read. It records every posted byte and decision.
func priorityN1Transcript(t *testing.T, policy []byte) string {
	t.Helper()
	aa, hi, fr, lo := priorityScenario()
	f := newPriorityFixture(t, policy, []*ticket.Record{aa, hi, fr, lo})
	f.apply(t, planClaim(f.claim(aa.TicketID.Raw, "lanes")))
	parts := []string{
		outcomeTranscript(planClaim(f.claim(lo.TicketID.Raw, "lanes"))),
		outcomeTranscript(planClaimNext(f.claim("", "lanes"))),
		outcomeTranscript(planClaimNext(f.claim("", ""))),
		planDigest(PriorityFirst(f.planInput("lanes"))),
		planDigest(PriorityFirst(f.planInput(""))),
	}
	for _, rec := range []*ticket.Record{aa, hi, fr, lo} {
		v, code := RecordedClaimability(f.planInput(""), rec)
		parts = append(parts, string(wire.Encode(v))+" "+code)
	}
	return strings.Join(parts, "\n")
}

func priorityN1Digest(t *testing.T, policy []byte) string {
	t.Helper()
	sum := sha256.Sum256([]byte(priorityN1Transcript(t, policy)))
	return hex.EncodeToString(sum[:])
}
