package transaction

import (
	"slices"
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

// PlanReasonWorkStateHeld is the deferral reason of a ticket the
// dispatcher's work state holds (CAL-V0-105). It is derived dispatcher
// output only: plan preview, claim and claim-next never set WorkStateHeld,
// so it is never a wire code, a stored value or a claim refusal.
const PlanReasonWorkStateHeld = "WORK_STATE_HELD"

// PlanReasonBudgetHeld is the deferral reason of a ticket a dispatcher
// budget holds (CAL-V0-155). Like WORK_STATE_HELD it is derived dispatcher
// output only, never a wire code, a stored value or a claim refusal.
const PlanReasonBudgetHeld = "BUDGET_HELD"

// PlanInput is the state one taskman-priority-first/0 plan reads: the
// intent inventory, whether an admission barrier is present, the live
// reservations, and every attempt record, which decides retry exhaustion.
type PlanInput struct {
	Pool, Stage    string
	ExcludeMembers []string
	// ExcludeAuthors applies CAL-V0-098 per planned ticket.
	ExcludeAuthors string
	Pools          *snapshot.PoolState
	Prepared       wire.Digest
	Queue          *intent.Queue
	Policy         *intent.Policy
	Tickets        *ticket.Inventory
	Barrier        bool
	Reservations   *snapshot.ReservationSet
	Attempts       map[string]*snapshot.Attempt
	// ClaimablePools, when non-nil, limits the default plan to the pools its
	// consumer can claim: an entry requiring any other pool is deferred
	// naming that pool before it can use the window (CAL-V0-097). Nil, as in
	// plan preview and claim, leaves every declared pool claimable.
	ClaimablePools []string
	// WorkStateHeld names, by raw ticket ID, the tickets a dispatcher's
	// observed work state holds: an otherwise plannable entry among them is
	// deferred as WORK_STATE_HELD before it can use the window (CAL-V0-105).
	// It is derived per observation, never stored; nil plans as before.
	WorkStateHeld map[string]bool
	// BudgetHeld names, by raw ticket ID, the tickets a dispatcher budget
	// holds: an otherwise plannable entry among them is deferred as
	// BUDGET_HELD before it can use the window (CAL-V0-155).
	BudgetHeld map[string]bool
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
	poolDeferred    bool
	// prerequisites are the CAL-V0-099 blocker details naming each
	// unsatisfied execution prerequisite, so a claim-next refusal names it.
	prerequisites []string
	// Detail explains the reason, and Authors is the CAL-V0-098 derivation;
	// both are set only when the plan excludes authors.
	Detail  string
	Authors *AuthorExclusion
	// yieldTo is the waiting ticket a priority-yield deferral names (CAL-V0-101).
	yieldTo string
	// Loop is the CAL-V0-102 hold behind a LOOP_DETECTED blocker, nil
	// otherwise, so plan and a claim-next refusal name its evidence.
	Loop *ticket.LoopHold
}

// TicketPlan is a taskman-priority-first/0 plan without its snapshot header.
// Pools is the default plan's per-pool selection summary (CAL-V0-097): one
// row per policy pool, in policy order, and nil for a --pool plan.
type TicketPlan struct {
	MaxActiveAttempts, AvailableWorkers wire.Count
	Entries                             []PlanEntry
	Pools                               []PoolSelection
}

// PoolSelection is one pool's row of a default plan: whether its member
// state was observed, its free eligible members (meaningful only when
// observed), and how many entries requiring it were selected or deferred
// for want of a free member.
type PoolSelection struct {
	PoolID                   string
	Observed                 bool
	Free, Selected, Deferred int
}

// PriorityFirst plans the OPEN and HELD tickets by TCP-00 §4.3 with the
// external-agent claim predicate, so a SELECTED entry is exactly a ticket
// `claim` would admit (CAL-V0-008, CAL-V0-014). An external-agent attempt
// holds no worker, so worker capacity is reported and never decides.
func PriorityFirst(in PlanInput) TicketPlan {
	plan := TicketPlan{MaxActiveAttempts: in.Policy.MaxActiveAttempts, AvailableWorkers: availableWorkers(in), Entries: []PlanEntry{}}
	selected := []PlanEntry{}
	// waits holds each opted-in pool's admission order, so every entry's
	// waiting list is priorityWaiting (CAL-V0-101, CAL-V0-108).
	waits := newAdmissionWaits(in)
	for _, rec := range planTickets(in.Tickets) {
		e := planEntry(in, rec)
		if e.State == "" && in.WorkStateHeld[rec.TicketID.Raw] {
			e.State, e.Reason, e.Blockers = PlanDeferred, PlanReasonWorkStateHeld, []string{PlanReasonWorkStateHeld}
		} else if e.State == "" && in.BudgetHeld[rec.TicketID.Raw] {
			e.State, e.Reason, e.Blockers = PlanDeferred, PlanReasonBudgetHeld, []string{PlanReasonBudgetHeld}
		} else if e.State == "" {
			e = choose(in, e, selected, waits.of(in, rec))
		}
		if e.State == PlanSelected {
			selected = append(selected, e)
		}
		plan.Entries = append(plan.Entries, e)
	}
	plan.Pools = poolSelections(in, plan.Entries)
	return plan
}

// poolSelections summarizes a default plan by required pool (CAL-V0-097).
func poolSelections(in PlanInput, entries []PlanEntry) []PoolSelection {
	if in.Pool != "" || len(in.Policy.Pools) == 0 {
		return nil
	}
	out := make([]PoolSelection, 0, len(in.Policy.Pools))
	for _, p := range in.Policy.Pools {
		free, observed := defaultPoolFree(in, p.ID)
		row := PoolSelection{PoolID: p.ID, Observed: observed, Free: free}
		for _, e := range entries {
			if e.Ticket.RequiresPool != p.ID {
				continue
			}
			if e.State == PlanSelected {
				row.Selected++
			} else if e.poolDeferred {
				row.Deferred++
			}
		}
		out = append(out, row)
	}
	return out
}

// ClaimNext is the entry CLAIM_NEXT claims from this plan (CAL-V0-008): the
// first SELECTED entry it can admit. Without a requested pool a claim
// consumes no pool (CAL-V0-029), so it skips SELECTED entries that require
// one; those remain claimable by an explicit claim naming their pool.
func (p TicketPlan) ClaimNext(pool string) *PlanEntry {
	for i := range p.Entries {
		if e := &p.Entries[i]; e.State == PlanSelected && (pool != "" || e.Ticket.RequiresPool == "") {
			return e
		}
	}
	return nil
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
		for _, b := range blockers {
			switch b.Code {
			case wire.CodePrerequisiteUnsatisfied:
				e.prerequisites = append(e.prerequisites, b.Detail)
			case wire.CodeLoopDetected:
				e.Loop = LoopHoldOf(in.Attempts, rec, in.Policy)
			}
		}
		if in.ExcludeAuthors != "" {
			e.Detail = blockers[0].Detail
		}
	}
	if in.ExcludeAuthors != "" {
		e.Authors, _ = authorPlan(in, rec)
	}
	return e
}

// authorPlan is the CAL-V0-098 derivation for one planned ticket, or the
// INDEPENDENCE_UNVERIFIED detail.
func authorPlan(in PlanInput, rec *ticket.Record) (*AuthorExclusion, string) {
	return DeriveAuthors(in.Attempts, rec.TicketID.Raw, in.ExcludeAuthors, in.Pool, in.ExcludeMembers)
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
	if in.ExcludeAuthors != "" && in.Pool != "" && (rec.RequiresPool == "" || rec.RequiresPool == in.Pool) {
		// CAL-V0-098: as in a named claim, the derivation precedes pool
		// capacity, so its refusal and author names survive an exhausted pool.
		if x, why := authorPlan(in, rec); x == nil {
			add(ticket.Blocker{Code: wire.CodeIndependenceUnverified, Detail: why}, true)
		} else if poolSlotsExcluding(in, x.Excluded) == 0 {
			add(ticket.Blocker{Code: wire.CodeResourceCollision, Detail: "requested pool has no eligible member; " + x.AuthorsDetail()}, true)
		}
	} else if !poolAvailable(in, rec) {
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
	ctx := ticket.Context{CanonicalWriter: in.Queue.CanonicalWriter, SerialFallback: in.Policy.SerialFallback, Stage: in.Stage, Loop: LoopHoldOf(in.Attempts, rec, in.Policy)}
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
	var waiting, unobserved []string
	if pool := rec.RequiresPool; pool != "" {
		waiting, unobserved = priorityWaiting(in, rec, pool)
	}
	// Unknown blockers must not hide definite reservation/capacity conflicts.
	e = choose(in, e, nil, map[string][]string{rec.RequiresPool: waiting})
	if e.poolDeferred && in.Pools == nil {
		// Unobserved member state is never reported as a definite refusal.
		return wire.Null(), string(ticket.NotObserved)
	}
	if e.State != PlanSelected {
		return wire.Bool(false), e.Reason
	}
	if free, _ := defaultPoolFree(in, rec.RequiresPool); len(unobserved) > 0 && priorityYield(in.Policy, rec.RequiresPool, free, append(waiting, unobserved...)) != "" {
		// A competitor whose eligibility is unknown would decide the yield.
		return wire.Null(), string(ticket.NotObserved)
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
// spent. In the default plan an entry requiring pool P is also deferred,
// naming P, once the earlier selections requiring P use up P's free
// eligible members, or when P's member state was not observed
// (CAL-V0-097), or when the plan's consumer cannot claim P. A deferred entry is not a selection, so it never counts
// against maxActiveAttempts.
//
// When the claimed pool opts into priority admission, an entry that an
// explicit claim would yield is deferred naming the first waiting ticket
// before any cap applies, so the plan and the claim agree (CAL-V0-101).
// waiting maps each pool to its priorityWaiting tickets ahead of e.
func choose(in PlanInput, e PlanEntry, selected []PlanEntry, waiting map[string][]string) PlanEntry {
	e.State, e.Reason = PlanDeferred, wire.CodeResourceCollision
	if in.Pool != "" {
		slots := poolSlots(in)
		if to := priorityYield(in.Policy, in.Pool, slots, waiting[in.Pool]); to != "" {
			e.Blockers, e.yieldTo = []string{to}, to
			return e
		}
		if len(selected) >= slots {
			e.Blockers = []string{wire.CodeResourceCollision}
			return e
		}
	}
	if in.Pool == "" && e.Ticket != nil && e.Ticket.RequiresPool != "" {
		pool := e.Ticket.RequiresPool
		free, observed := defaultPoolFree(in, pool)
		using := 0
		for _, s := range selected {
			if s.Ticket.RequiresPool == pool {
				using++
			}
		}
		if !observed || (in.ClaimablePools != nil && !slices.Contains(in.ClaimablePools, pool)) {
			e.Blockers, e.poolDeferred = []string{pool}, true
			return e
		}
		if to := priorityYield(in.Policy, pool, free, waiting[pool]); to != "" {
			e.Blockers, e.poolDeferred, e.yieldTo = []string{to}, true, to
			return e
		}
		if using >= free {
			e.Blockers, e.poolDeferred = []string{pool}, true
			return e
		}
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

// poolAvailable is the pool part of the claim predicate. A --pool plan
// blocks a ticket that requires another pool, and every ticket once the
// requested pool has no eligible free member. The default plan blocks only a
// ticket whose required pool the policy does not declare; member capacity
// defers it in choose instead (CAL-V0-097).
func poolAvailable(in PlanInput, rec *ticket.Record) bool {
	if in.Pool == "" {
		return rec.RequiresPool == "" || in.Policy.Pool(rec.RequiresPool) != nil
	}
	if rec.RequiresPool != "" && rec.RequiresPool != in.Pool {
		return false
	}
	return poolSlots(in) > 0
}

// defaultPoolFree is the default plan's count of poolID's free eligible
// members for the plan's stage, with no exclusions, and whether pool member
// state was observed at all. Without that observation the count is unknown,
// never assumed free.
func defaultPoolFree(in PlanInput, poolID string) (int, bool) {
	if in.Pools == nil {
		return 0, false
	}
	return freePoolMembers(in, poolID, nil), true
}

func poolSlots(in PlanInput) int {
	return poolSlotsExcluding(in, in.ExcludeMembers)
}

// poolSlotsExcluding counts free eligible members after excluded, which is
// the explicit set or its CAL-V0-098 union with one ticket's authors.
func poolSlotsExcluding(in PlanInput, excluded []string) int {
	if CheckPoolExclusions(in.Pool, in.ExcludeMembers, in.Policy) != nil {
		return 0
	}
	return freePoolMembers(in, in.Pool, excluded)
}

// freePoolMembers counts poolID's members eligible under the claim's stage
// order and exclusions (CAL-V0-029, CAL-V0-065) that no pool state entry
// other than the prepared allocation occupies. Health probes never run.
func freePoolMembers(in PlanInput, poolID string, excluded []string) int {
	p := in.Policy.Pool(poolID)
	if p == nil {
		return 0
	}
	slots := 0
	for _, m := range OrderedPoolMembers(p, in.Stage, excluded) {
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
