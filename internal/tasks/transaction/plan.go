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
	Pool, Stage    string
	ExcludeMembers []string
	Pools          *snapshot.PoolState
	Prepared       wire.Digest
	Queue          *intent.Queue
	Policy         *intent.Policy
	Tickets        *ticket.Inventory
	Barrier        bool
	Reservations   *snapshot.ReservationSet
	Attempts       map[string]*snapshot.Attempt
}

// PlanEntry is one planned ticket. Resources are what a claim of it would
// reserve: its DECLARED scope, or WHOLE_REPOSITORY (CAL-V0-021).
type PlanEntry struct {
	Retries         wire.Value
	NextStage       wire.Value // advisory CAL-V0-084 derived next stage
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
	e := PlanEntry{Ticket: rec, Resources: wholeRepository, Retries: RetryObservation(in.Attempts, rec, in.Policy.AdmissionsPerRevision.Int()), NextStage: NextStage(in.Attempts, rec)}
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
	all, _, _ := claimBlockerObservations(in, rec)
	return all
}

// ObservedBlocker is one planner claim blocker; Observed is false for a
// NOT_OBSERVED unknown.
type ObservedBlocker struct {
	ticket.Blocker
	Observed bool
}

// ClaimBlockerObservations is the planner's claim-blocker derivation for
// rec in planClaim's order, keeping unknown provenance. Read-only views such
// as critical-path (CAL-V0-081) reuse it so they never diverge from `plan`.
// A nil Reservations set leaves attempt liveness NOT_OBSERVED.
func ClaimBlockerObservations(in PlanInput, rec *ticket.Record) []ObservedBlocker {
	var out []ObservedBlocker
	add := func(b ticket.Blocker, observed bool) {
		out = append(out, ObservedBlocker{Blocker: b, Observed: observed})
	}
	if !poolAvailable(in, rec) {
		add(ticket.Blocker{Code: wire.CodeResourceCollision, Detail: "required or requested pool has no eligible member"}, true)
	}
	if in.Barrier {
		add(ticket.Blocker{Code: wire.CodePaused}, true)
	}
	if !in.Queue.Fixture && in.Queue.ExecutionCutover == nil {
		add(ticket.Blocker{Code: wire.CodeCutoverMissing}, true)
	}
	if len(in.Policy.RequireEnforcedFields) != 0 {
		add(ticket.Blocker{Code: wire.CodeBudgetUnknown}, true)
	}
	ctx := ticket.Context{CanonicalWriter: in.Queue.CanonicalWriter, SerialFallback: in.Policy.SerialFallback}
	if in.Reservations != nil {
		ctx.Attempts = entryOracle{in.Reservations}
	}
	v, _ := in.Tickets.View(rec.TicketID.Raw, ctx)
	for _, b := range v.Blockers {
		if b.Code != wire.CodeCoverageUnknown {
			add(b, true)
		}
	}
	for _, b := range v.Unknowns {
		if b.Code != wire.CodeCoverageUnknown {
			add(b, false)
		}
	}
	if retryExhausted(in.Attempts, rec, in.Policy.AdmissionsPerRevision.Int()) {
		add(ticket.Blocker{Code: wire.CodeRetryExhausted}, true)
	}
	return out
}

// Preserve unknown provenance while keeping the planner's original blocker order.
func claimBlockerObservations(in PlanInput, rec *ticket.Record) (all, known, unknown []ticket.Blocker) {
	for _, b := range ClaimBlockerObservations(in, rec) {
		all = append(all, b.Blocker)
		if b.Observed {
			known = append(known, b.Blocker)
		} else {
			unknown = append(unknown, b.Blocker)
		}
	}
	return all, known, unknown
}

// RecordedClaimability describes the default external-agent plan at this read
// snapshot, before reap. It does not reserve resources or validate caller inputs.
func RecordedClaimability(in PlanInput, rec *ticket.Record) (wire.Value, string) {
	if rec.Status != ticket.StatusOpen && rec.Status != ticket.StatusHeld {
		return wire.Bool(false), wire.CodeTicketState
	}
	_, known, unknown := claimBlockerObservations(in, rec)
	if len(known) > 0 {
		return wire.Bool(false), known[0].Code
	}
	e := planEntry(in, rec)
	// Unknown blockers must not hide definite reservation/capacity conflicts.
	e = choose(in, e, nil)
	if e.State != PlanSelected {
		return wire.Bool(false), e.Reason
	}
	if len(unknown) > 0 {
		return wire.Null(), unknown[0].Code
	}
	return wire.Bool(true), PlanSelected
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
	if in.Pool != "" && len(selected) >= poolSlots(in) {
		e.Blockers = []string{wire.CodeResourceCollision}
		return e
	}
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

// retryExhausted applies the current policy to charged retries at this acceptance revision.
func retryExhausted(attempts map[string]*snapshot.Attempt, rec *ticket.Record, limit int64) bool {
	last := lastAttemptOf(attempts, rec.TicketID.Raw)
	return last != nil && last.TicketRevision == rec.AcceptanceRevision && exhaustedAttempt(last, limit)
}

func poolAvailable(in PlanInput, rec *ticket.Record) bool {
	if rec.RequiresPool != "" && rec.RequiresPool != in.Pool {
		return false
	}
	if in.Pool == "" {
		return true
	}
	return poolSlots(in) > 0
}
func poolSlots(in PlanInput) int {
	p := in.Policy.Pool(in.Pool)
	if CheckPoolExclusions(in.Pool, in.ExcludeMembers, in.Policy) != nil || p == nil {
		return 0
	}
	slots := 0
	for _, m := range OrderedPoolMembers(p, in.Stage, in.ExcludeMembers) {
		busy := false
		if in.Pools != nil {
			for _, en := range in.Pools.Entries {
				if en.MemberID == m && en.AllocationID != in.Prepared {
					busy = true
				}
			}
		}
		if !busy {
			slots++
		}
	}
	return slots
}
