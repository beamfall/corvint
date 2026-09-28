package transaction

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Plan entry states (TCP-00 §4.3).
const (
	PlanSelected = "SELECTED"
	PlanDeferred = "DEFERRED"
	PlanBlocked  = "BLOCKED"
)

// PlanInput is the state one taskman-priority-first/0 plan reads: the
// intent inventory, whether an admission barrier is present, the live
// reservations, and every attempt record, which decides retry exhaustion.
type PlanInput struct {
	Queue        *intent.Queue
	Policy       *intent.Policy
	Tickets      *ticket.Inventory
	Barrier      bool
	Reservations *snapshot.ReservationSet
	Attempts     map[string]*snapshot.Attempt
}

// PlanEntry is one planned ticket. Resources are what a claim of it would
// reserve: its DECLARED scope, or WHOLE_REPOSITORY (CAL-V0-021).
type PlanEntry struct {
	Ticket          *ticket.Record
	Resources       []ticket.Resource
	ClosureComplete bool
	State, Reason   string
	Blockers        []string
}

// TicketPlan is a taskman-priority-first/0 plan without its snapshot header.
type TicketPlan struct {
	MaxActiveAttempts, AvailableWorkers wire.Count
	Entries                             []PlanEntry
}

// PriorityFirst plans the OPEN and HELD tickets by TCP-00 §4.3 with the
// external-agent claim predicate, so a SELECTED entry is exactly a ticket
// `claim` would admit (CAL-V0-008, CAL-V0-014). An external-agent attempt
// holds no worker, so worker capacity is reported and never decides.
func PriorityFirst(in PlanInput) TicketPlan {
	plan := TicketPlan{MaxActiveAttempts: in.Policy.MaxActiveAttempts, AvailableWorkers: availableWorkers(in), Entries: []PlanEntry{}}
	selected := []PlanEntry{}
	for _, rec := range planTickets(in.Tickets) {
		e := planEntry(in, rec)
		if e.State == "" {
			e = choose(in, e, selected)
		}
		if e.State == PlanSelected {
			selected = append(selected, e)
		}
		plan.Entries = append(plan.Entries, e)
	}
	return plan
}

// Selected is the first SELECTED entry, or nil.
func (p TicketPlan) Selected() *PlanEntry {
	for i := range p.Entries {
		if p.Entries[i].State == PlanSelected {
			return &p.Entries[i]
		}
	}
	return nil
}

func availableWorkers(in PlanInput) wire.Count {
	used := int64(0)
	for _, en := range in.Reservations.Entries {
		used += en.Workers.Int()
	}
	return wire.CountOf(max(in.Policy.MaxWorkersTotal.Int()-used, 0))
}

// planTickets orders the OPEN and HELD tickets by (priority, order,
// ticketId bytes).
func planTickets(inv *ticket.Inventory) []*ticket.Record {
	out := []*ticket.Record{}
	for _, id := range inv.IDs() {
		rec, _ := inv.Get(id)
		if rec.Status == ticket.StatusOpen || rec.Status == ticket.StatusHeld {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return planLess(out[i], out[j]) })
	return out
}

func planLess(a, b *ticket.Record) bool {
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	if a.Order.Int() != b.Order.Int() {
		return a.Order.Int() < b.Order.Int()
	}
	return a.TicketID.Raw < b.TicketID.Raw
}

func planEntry(in PlanInput, rec *ticket.Record) PlanEntry {
	e := PlanEntry{Ticket: rec, Resources: wholeRepository}
	if declared := Declared(rec); len(declared) > 0 {
		e.Resources, e.ClosureComplete = append(pathResources(declared), declaredOther(rec)...), true
	}
	blockers := claimBlockers(in, rec)
	if len(blockers) > 0 {
		e.State, e.Reason, e.Blockers = PlanBlocked, blockers[0].Code, blockerRefs(blockers)
	}
	return e
}

// claimBlockers are the facts that refuse a claim of rec whatever the
// reservations: planClaim's barrier, execution cutover, budget, eligibility
// and retry checks in its order. The coverage blocker is not one, because an
// undeclared ticket claims WHOLE_REPOSITORY.
func claimBlockers(in PlanInput, rec *ticket.Record) []ticket.Blocker {
	out := []ticket.Blocker{}
	if in.Barrier {
		out = append(out, ticket.Blocker{Code: wire.CodePaused})
	}
	if !in.Queue.Fixture {
		out = append(out, ticket.Blocker{Code: wire.CodeCutoverMissing})
	}
	if len(in.Policy.RequireEnforcedFields) != 0 {
		out = append(out, ticket.Blocker{Code: wire.CodeBudgetUnknown})
	}
	v, _ := in.Tickets.View(rec.TicketID.Raw, ticket.Context{CanonicalWriter: in.Queue.CanonicalWriter, SerialFallback: in.Policy.SerialFallback, Attempts: entryOracle{in.Reservations}})
	for _, b := range append(v.Blockers, v.Unknowns...) {
		if b.Code != wire.CodeCoverageUnknown {
			out = append(out, b)
		}
	}
	if retryExhausted(in.Attempts, rec) {
		out = append(out, ticket.Blocker{Code: wire.CodeRetryExhausted})
	}
	return out
}

// blockerRefs is the sorted, duplicate-free set of blocker codes and the
// tickets they name.
func blockerRefs(blockers []ticket.Blocker) []string {
	set := map[string]bool{}
	for _, b := range blockers {
		set[b.Code] = true
		if b.TicketID != "" {
			set[b.TicketID] = true
		}
	}
	out := make([]string, 0, len(set))
	for ref := range set {
		out = append(out, ref)
	}
	sort.Strings(out)
	return out
}

// choose selects an eligible entry unless it collides with a live
// reservation or an earlier selection, or the active-attempt capacity is
// spent.
func choose(in PlanInput, e PlanEntry, selected []PlanEntry) PlanEntry {
	e.State, e.Reason = PlanDeferred, wire.CodeResourceCollision
	for _, en := range in.Reservations.Entries {
		if ticket.Collide(e.Resources, en.Resources) {
			e.Blockers = []string{en.TicketID.Raw}
			return e
		}
	}
	for _, s := range selected {
		if ticket.Collide(e.Resources, s.Resources) {
			e.Blockers = []string{s.Ticket.TicketID.Raw}
			return e
		}
	}
	if int64(len(in.Reservations.Entries)+len(selected)) >= in.Policy.MaxActiveAttempts.Int() {
		e.Reason, e.Blockers = wire.CodeLimitExceeded, []string{wire.CodeLimitExceeded}
		return e
	}
	e.State, e.Reason, e.Blockers = PlanSelected, wire.CodeDevelopmentMode, []string{}
	return e
}

// lastAttemptOf is the ticket's attempt at the highest generation, or nil.
func lastAttemptOf(attempts map[string]*snapshot.Attempt, ticketID string) *snapshot.Attempt {
	var last *snapshot.Attempt
	for _, a := range attempts {
		if a.TicketID.Raw == ticketID && (last == nil || a.Generation.Uint64() > last.Generation.Uint64()) {
			last = a
		}
	}
	return last
}

// retryExhausted says three retries at the ticket's acceptanceRevision are
// spent (CAL-V0-013).
func retryExhausted(attempts map[string]*snapshot.Attempt, rec *ticket.Record) bool {
	last := lastAttemptOf(attempts, rec.TicketID.Raw)
	return last != nil && last.Phase != "COMPLETED" && last.TicketRevision == rec.AcceptanceRevision && last.RetryCount.Int() >= MaxRetries
}
