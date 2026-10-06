package transaction

import (
	"sort"
	"strconv"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// admissionRank is a ticket's derived place in a pool's priority-admission
// order (CAL-V0-108). stage is the downstream stage (review or integrate)
// that the ticket's latest generation handed off to at the current
// acceptance revision, and since is that generation's terminal phase
// sequence, when the ticket waits for it; both are empty otherwise.
type admissionRank struct {
	rec   *ticket.Record
	stage string
	since uint64
}

// admissionRankOf derives rec's rank from its latest attempt generation; it
// stores nothing.
func admissionRankOf(attempts map[string]*snapshot.Attempt, rec *ticket.Record) admissionRank {
	r := admissionRank{rec: rec}
	if a := lastAttemptOf(attempts, rec.TicketID.Raw); a != nil && a.TicketRevision == rec.AcceptanceRevision {
		if stage := a.NextStage(); stage == "review" || stage == "integrate" {
			r.stage, r.since = stage, a.PhaseSinceSeq.Uint64()
		}
	}
	return r
}

// admissionLess is the CAL-V0-108 admission order: priority first; at equal
// priority a ticket waiting for a downstream stage before one that is not,
// two downstream tickets by the earlier handoff, and then plan order
// (order, ticketId). Without a downstream ticket it is plan order.
func admissionLess(a, b admissionRank) bool {
	if a.rec.Priority != b.rec.Priority {
		return a.rec.Priority < b.rec.Priority
	}
	if (a.stage != "") != (b.stage != "") {
		return a.stage != ""
	}
	if a.stage != "" && a.since != b.since {
		return a.since < b.since
	}
	return planLess(a.rec, b.rec)
}

// admissionCandidate is one OPEN ticket requiring a pool; unobserved marks a
// ticket whose only claim blockers are unknown.
type admissionCandidate struct {
	admissionRank
	unobserved bool
}

// admissionQueue lists, in admission order, the tickets that compete for
// pool under priority-yield admission (CAL-V0-101): OPEN tickets that
// require pool and have no known claim blocker, which includes a live
// attempt or no free member of pool eligible for the competitor's stage. A
// downstream ticket's blockers are read at its own stage, the stage its
// claim names; any other ticket's at in.Stage. Every competitor is read
// against pool itself, without the plan's or claim's own exclusions, so the
// default plan, the --pool plan and a claim rank the same competitors. The
// read is pure: it consults the plan input and runs no probe.
func admissionQueue(in PlanInput, pool string) []admissionCandidate {
	out := []admissionCandidate{}
	for _, t := range planTickets(in.Tickets) {
		if t.Status != ticket.StatusOpen || t.RequiresPool != pool {
			continue
		}
		r := admissionRankOf(in.Attempts, t)
		at := in
		at.Pool, at.ExcludeMembers, at.ExcludeAuthors = pool, nil, ""
		if r.stage != "" {
			at.Stage = r.stage
		}
		_, known, unknown := claimBlockerObservations(at, t)
		if len(known) == 0 {
			out = append(out, admissionCandidate{admissionRank: r, unobserved: len(unknown) > 0})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return admissionLess(out[i].admissionRank, out[j].admissionRank) })
	return out
}

// admissionAhead splits the candidates ranked ahead of r into the waiting
// tickets and those whose eligibility is NOT_OBSERVED, which never count as
// waiting. r's own ticket is never ahead of itself.
func admissionAhead(queue []admissionCandidate, r admissionRank) (waiting, unobserved []string) {
	for _, c := range queue {
		if !admissionLess(c.admissionRank, r) {
			break
		}
		if c.unobserved {
			unobserved = append(unobserved, c.rec.TicketID.Raw)
		} else {
			waiting = append(waiting, c.rec.TicketID.Raw)
		}
	}
	return waiting, unobserved
}

// priorityWaiting lists, in admission order, the tickets that compete with
// rec for pool and are ranked ahead of it (CAL-V0-101, CAL-V0-108).
func priorityWaiting(in PlanInput, rec *ticket.Record, pool string) (waiting, unobserved []string) {
	return admissionAhead(admissionQueue(in, pool), admissionRankOf(in.Attempts, rec))
}

// admissionWaits precomputes, for a plan, each opted-in pool's queue and the
// waiting tickets ahead of each queue position, so a plan entry's waiting
// list is a shared prefix rather than a fresh read per entry.
type admissionWaits map[string]admissionPrefix

type admissionPrefix struct {
	queue   []admissionCandidate
	waiting []string
	counts  []int
}

func newAdmissionWaits(in PlanInput) admissionWaits {
	out := admissionWaits{}
	for _, p := range in.Policy.Pools {
		if !p.PriorityAdmission {
			continue
		}
		pre := admissionPrefix{queue: admissionQueue(in, p.ID), counts: []int{0}}
		for _, c := range pre.queue {
			if !c.unobserved {
				pre.waiting = append(pre.waiting, c.rec.TicketID.Raw)
			}
			pre.counts = append(pre.counts, len(pre.waiting))
		}
		out[p.ID] = pre
	}
	return out
}

// of maps each opted-in pool to the waiting tickets ranked ahead of rec, as
// priorityWaiting would list them.
func (w admissionWaits) of(in PlanInput, rec *ticket.Record) map[string][]string {
	if len(w) == 0 {
		return nil
	}
	r := admissionRankOf(in.Attempts, rec)
	out := make(map[string][]string, len(w))
	for pool, pre := range w {
		i := sort.Search(len(pre.queue), func(i int) bool { return !admissionLess(pre.queue[i].admissionRank, r) })
		n := pre.counts[i]
		out[pool] = pre.waiting[:n:n]
	}
	return out
}

// admissionNote names why the yielded-to ticket ranks ahead when it waits
// for a downstream stage, or is empty.
func admissionNote(queue []admissionCandidate, to string) string {
	for _, c := range queue {
		if c.rec.TicketID.Raw == to && c.stage != "" {
			return "; " + to + " awaits " + c.stage + " since seq " + strconv.FormatUint(c.since, 10)
		}
	}
	return ""
}

// priorityYield is the ticket a claim on pool yields to, or "" when it need
// not yield: the pool opts into priority admission, it has a free eligible
// member, and the waiting competitors are at least as many as those free
// members. It names the first, highest-priority competitor.
func priorityYield(policy *intent.Policy, pool string, free int, waiting []string) string {
	p := policy.Pool(pool)
	if p == nil || !p.PriorityAdmission || free <= 0 || len(waiting) < free {
		return ""
	}
	return waiting[0]
}

// priorityYieldDetail is the refusal detail of a claim yielded to to.
func priorityYieldDetail(pool string, free int, waiting []string, to string) string {
	return "pool " + pool + " priority admission: " + strconv.Itoa(len(waiting)) + " higher-priority waiting ticket(s) for " + strconv.Itoa(free) + " free eligible member(s); yields to " + to
}

// yieldRefusal refuses an explicit pooled claim that would take a member a
// higher-priority waiting ticket needs (CAL-V0-101). It runs after the
// live-attempt checks and before the collision, capacity, health and
// allocation steps, so a yielded claim prepares no member. Without
// --pool, or on a pool that does not opt in, it never refuses.
func (c leaseContext) yieldRefusal(rec *ticket.Record) *leaseOutcome {
	if c.l.Pool == "" {
		return nil
	}
	if p := c.st.policy.Pool(c.l.Pool); p == nil || !p.PriorityAdmission {
		return nil
	}
	in := PlanInput{Pool: c.l.Pool, Stage: c.l.Stage, ExcludeMembers: c.l.ExcludeMembers, Pools: c.st.pools, Prepared: c.in.LeaseFacts.Pool.AllocationID, Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts}
	free := poolSlots(in)
	queue := admissionQueue(in, c.l.Pool)
	waiting, _ := admissionAhead(queue, admissionRankOf(in.Attempts, rec))
	to := priorityYield(c.st.policy, c.l.Pool, free, waiting)
	if to == "" {
		return nil
	}
	out := c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, priorityYieldDetail(c.l.Pool, free, waiting, to)+admissionNote(queue, to))
	return &out
}
