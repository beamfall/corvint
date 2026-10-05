package transaction

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var wholeRepository = []ticket.Resource{{Class: "WHOLE_REPOSITORY", Key: wholeRepositoryKey}}

func pathResources(paths []string) []ticket.Resource {
	out := make([]ticket.Resource, 0, len(paths))
	for _, p := range paths {
		out = append(out, ticket.Resource{Class: "PATH", Key: p})
	}
	return out
}

// Declared returns the sorted, duplicate-free PATH scope a QUALIFIED ticket
// declares: its touchPaths and its PATH resources (CAL-V0-021). It is empty
// for any other coverage, and for a QUALIFIED ticket that declares no path.
func Declared(rec *ticket.Record) []string {
	if rec.Effects.Coverage != "QUALIFIED" {
		return nil
	}
	set := map[string]bool{}
	for _, p := range rec.Effects.TouchPaths {
		set[p] = true
	}
	for _, r := range rec.Effects.Resources {
		if r.Class == "PATH" {
			set[r.Key] = true
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// declaredOther is every non-PATH resource the ticket declares. It joins
// each scope short of WHOLE_REPOSITORY, so two claims on one database, port
// or shared gate still collide under TCP-00 §4.2 (CAL-V0-021).
func declaredOther(rec *ticket.Record) []ticket.Resource {
	out := []ticket.Resource{}
	for _, r := range rec.Effects.Resources {
		if r.Class != "PATH" {
			out = append(out, r)
		}
	}
	return out
}

// claimScope applies the CAL-V0-021 precedence: DECLARED, REQUESTED,
// DERIVED, then WHOLE_REPOSITORY.
func (c leaseContext) claimScope(rec *ticket.Record) (*snapshot.Scope, error) {
	other := declaredOther(rec)
	if declared := Declared(rec); len(declared) > 0 {
		return &snapshot.Scope{Source: "DECLARED", Resources: append(pathResources(declared), other...)}, nil
	}
	if c.l.Scope != nil {
		return &snapshot.Scope{Source: "REQUESTED", Resources: append(pathResources(c.l.Scope), other...)}, nil
	}
	facts := c.in.LeaseFacts
	if facts.DerivedPaths == nil {
		return &snapshot.Scope{Source: "WHOLE_REPOSITORY", Resources: wholeRepository}, nil
	}
	if facts.DerivedTicketID != rec.TicketID.Raw {
		return nil, malformed("derived scope belongs to another ticket")
	}
	if e := checkScopePaths(facts.DerivedPaths); e != nil {
		return nil, e
	}
	d, e := wire.ParseDigest("derivationSha256", string(facts.DerivationSha256))
	if e != nil {
		return nil, e
	}
	return &snapshot.Scope{Source: "DERIVED", Resources: append(pathResources(facts.DerivedPaths), other...), DerivationSha256: &d}, nil
}

func entryCoverage(sc *snapshot.Scope) string {
	if sc.Source == "WHOLE_REPOSITORY" {
		return "WHOLE_REPOSITORY"
	}
	return "QUALIFIED"
}

// collision names the first other live attempt whose resources collide with
// rs under TCP-00 §4.2 (CAL-V0-023), or "".
func (c leaseContext) collision(rs []ticket.Resource, self string) string {
	for _, en := range c.st.reservations.Entries {
		if en.AttemptID != self && ticket.Collide(rs, en.Resources) {
			return en.AttemptID
		}
	}
	return ""
}

// expiredBlocking lists the expired leases whose entries would block this
// claim: the same ticket, a colliding scope, or any expired entry when the
// set is at capacity (CAL-V0-011).
func (c leaseContext) expiredBlocking(ticketID string, rs []ticket.Resource) []ExpiredLease {
	full := int64(len(c.st.reservations.Entries)) >= c.st.policy.MaxActiveAttempts.Int()
	out := []ExpiredLease{}
	for _, en := range c.st.reservations.Entries {
		if !expired(c.st.attempts[en.AttemptID], c.in.RecordedAt) {
			continue
		}
		if full || en.TicketID.Raw == ticketID || ticket.Collide(rs, en.Resources) {
			out = append(out, ExpiredLease{AttemptID: en.AttemptID, Generation: en.Generation})
		}
	}
	return out
}

func (c leaseContext) liveOn(ticketID string) string {
	for _, en := range c.st.reservations.Entries {
		if en.TicketID.Raw == ticketID {
			return en.AttemptID
		}
	}
	return ""
}

// eligibility refuses on the first §3.2 blocker or unknown, except the
// coverage blocker (a non-QUALIFIED ticket claims WHOLE_REPOSITORY) and the
// live-attempt blocker, which the claim decides after reaping.
func (c leaseContext) eligibility(id string) *leaseOutcome {
	v, _ := c.st.tickets.View(id, ticket.Context{CanonicalWriter: c.st.queue.CanonicalWriter, SerialFallback: c.st.policy.SerialFallback, Attempts: entryOracle{c.st.reservations}})
	skip := map[string]bool{wire.CodeCoverageUnknown: true, wire.CodeAttemptLive: true}
	for _, b := range append(v.Blockers, v.Unknowns...) {
		if !skip[b.Code] {
			out := c.refuse(mutation.OutcomeBlocked, b.Code, b.Detail)
			return &out
		}
	}
	return nil
}

// retryOf returns the terminal attempt this claim retries as its next
// generation (CAL-V0-013), nil for a fresh attempt, or a refusal once the
// configured retry budget at the ticket's acceptanceRevision is spent.
func (c leaseContext) retryOf(rec *ticket.Record) (*snapshot.Attempt, *leaseOutcome) {
	last := lastAttemptOf(c.st.attempts, rec.TicketID.Raw)
	if last == nil || last.Phase == "COMPLETED" || last.TicketRevision != rec.AcceptanceRevision {
		return nil, nil
	}
	if retryExhausted(c.st.attempts, rec, c.st.policy.AdmissionsPerRevision.Int()) {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeRetryExhausted, "configured retry budget at acceptanceRevision "+string(rec.AcceptanceRevision)+" is spent")
		return nil, &out
	}
	return last, nil
}

func notObservedBudget() map[string]snapshot.BudgetField {
	out := map[string]snapshot.BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		out[name] = snapshot.BudgetField{State: "NOT_OBSERVED"}
	}
	return out
}

// freshID checks the caller-minted identity of a new attempt.
func (c leaseContext) freshID() (string, error) {
	id := c.in.LeaseFacts.AttemptID
	q, e := snapshot.AttemptQueue(id)
	if e != nil {
		return "", e
	}
	if q.Raw != c.r.QueueID {
		return "", malformed("minted attempt names another queue")
	}
	if _, exists := c.in.Inventory.files[attemptPath(id)]; exists {
		return "", malformed("minted attempt already exists")
	}
	return id, nil
}

func (c leaseContext) baseCommit() (string, error) {
	base := c.in.LeaseFacts.BaseCommit
	if _, e := wire.ParseOID("baseCommit", base); e != nil {
		return "", e
	}
	if c.l.Base != "" && c.l.Base != base {
		return "", malformed("resolved base commit differs from --base")
	}
	return base, nil
}

// admitted builds the RUNNING attempt at the next queue generation
// (TM-V0-011): a retry keeps the prior identity and records the fenced
// generation it closes.
func (c leaseContext) admitted(rec *ticket.Record, prior *snapshot.Attempt, sc *snapshot.Scope) (*snapshot.Attempt, error) {
	base, e := c.baseCommit()
	if e != nil {
		return nil, e
	}
	branch := c.l.Branch
	if branch == "" {
		branch = rec.TicketID.Local
	}
	policy := wire.Sum(c.st.policy.Raw)
	a := &snapshot.Attempt{TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, TicketRecordSha256: rec.FileDigest(), Generation: wire.SizeOf(c.st.head.Generation.Uint64() + 1), Phase: "RUNNING", PhaseSinceSeq: c.seq, Mode: "DEVELOPMENT", PolicySha256: policy, ConfigSha256: policy, RuntimeID: snapshot.RuntimeExternalAgent, CapabilityProfileSha256: policy, BaseCommit: base, Branch: branch, Quiescence: "UNPROVED", SpawnNoExecCount: "0", PendingEffects: []string{}, RetryCount: "0", RepairRound: "0", Budget: notObservedBudget(), GateResults: []string{}, Reviews: []string{}, ScopeCheck: "UNKNOWN", PriorGenerations: []snapshot.PriorGeneration{}, Scope: sc}
	at := c.in.RecordedAt
	a.LastHeartbeatAt = &at
	a.RetryReasons = emptyRetryReasons()
	a.Stage = c.l.Stage
	a.RetryAccounting = &snapshot.RetryAccounting{Disposition: "NONE"}
	a.OperatorNote = rec.OperatorNote
	a.PoolAllocation, e = c.allocate(a)
	if e != nil {
		return nil, e
	}
	a.Lease = &snapshot.Lease{Holder: c.l.Holder, GrantedSeq: c.seq, ExpiresAt: addMinutes(c.in.RecordedAt, c.l.LeaseMinutes)}
	if prior == nil {
		a.AttemptID, e = c.freshID()
		if e == nil && a.PoolAllocation != nil && c.in.LeaseFacts.Pool.AllocationID == "" {
			a.DirectPoolAdmission = &snapshot.DirectPoolAdmission{AttemptID: a.AttemptID, Generation: a.Generation, OriginalAdmissionSeq: c.seq, Allocation: a.PoolAllocation, Holder: a.Lease.Holder, Stage: a.Stage}
		}
		return a, e
	}
	a.AttemptID = prior.AttemptID
	a.RetryCount = prior.RetryCount
	a.RetryReasons = retryReasons(prior)
	if !cleanHandoff(prior) {
		a.RetryCount = wire.CountOf(int64(prior.RetryCount.Int() + 1))
		reason := chargedReason(prior)
		a.RetryReasons[reason] = wire.CountOf(a.RetryReasons[reason].Int() + 1)
	}
	a.PriorGenerations = append(append([]snapshot.PriorGeneration{}, prior.PriorGenerations...), snapshot.PriorGeneration{Generation: prior.Generation, Quiescence: prior.Quiescence, ProvedSeq: prior.PhaseSinceSeq, History: endedHistory(prior)})
	return a, nil
}

// endedHistory copies the stage and pool member an ended external-agent
// generation held (CAL-V0-096). A supervised generation spans several
// stages and releases its allocation when a stage stops, so a single
// stage and member would be a guess and nothing is recorded.
func endedHistory(prior *snapshot.Attempt) *snapshot.GenerationHistory {
	if prior.RuntimeID != snapshot.RuntimeExternalAgent {
		return nil
	}
	h := &snapshot.GenerationHistory{}
	if prior.Stage != "" {
		stage := prior.Stage
		h.Stage = &stage
	}
	if x := prior.PoolAllocation; x != nil {
		pool, member := x.PoolID, x.MemberID
		h.PoolID, h.MemberID = &pool, &member
	}
	// CAL-V0-082: the ended generation's recorded hand-off moves with it.
	h.HandoffTo, h.HandoffReason = prior.HandoffTo, prior.HandoffReason
	return h
}

// noExecutionCutover names the execution cutover (CAL-V0-020), not the S2
// writer cutover that ticket views also report as CUTOVER_MISSING.
const noExecutionCutover = "a non-fixture queue admits no claim before its execution cutover"

// planClaim admits one external-agent attempt by TCP-00 §4.1 steps 1, 2,
// 5, 6 and 8 (CAL-V0-002, CAL-V0-007, CAL-V0-021, CAL-V0-023).
func planClaim(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
	}
	if !c.st.queue.Fixture && c.st.queue.ExecutionCutover == nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeCutoverMissing, noExecutionCutover)
	}
	rec, _ := c.st.tickets.Get(c.l.TicketID)
	if rec == nil {
		return c.fail(malformed("unknown ticket " + c.l.TicketID))
	}
	if len(c.st.policy.RequireEnforcedFields) != 0 {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeBudgetUnknown, "policy requires enforced budget fields an external agent cannot report")
	}
	if refusal := c.eligibility(rec.TicketID.Raw); refusal != nil {
		return *refusal
	}
	if rec.RequiresPool != "" && c.l.Pool != rec.RequiresPool {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "ticket requires explicit matching --pool")
	}
	if c.l.Pool != "" && c.st.policy.Pool(c.l.Pool) == nil {
		return c.fail(malformed("unknown pool"))
	}
	sc, e := c.claimScope(rec)
	if e != nil {
		return c.fail(e)
	}
	return c.admit(rec, sc)
}

// planClaimNext claims the first SELECTED entry of the plan computed in this
// transaction, or refuses BLOCKED with the plan's first reason (CAL-V0-008).
// Every expired lease is reaped first, since any of them can decide the plan
// through a collision or the capacity. Derived facts apply only to the
// independently selected ticket; planClaim rechecks its final scope.
func planClaimNext(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
	}
	if !c.st.queue.Fixture && c.st.queue.ExecutionCutover == nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeCutoverMissing, noExecutionCutover)
	}
	if reap := c.expiredAll(); len(reap) != 0 {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "expired leases block this claim until reaped")
		out.result.Expired = reap
		return out
	}
	plan := PriorityFirst(PlanInput{Pool: c.l.Pool, Stage: c.l.Stage, ExcludeMembers: c.l.ExcludeMembers, Pools: c.st.pools, Prepared: c.in.LeaseFacts.Pool.AllocationID, Queue: c.st.queue, Policy: c.st.policy, Tickets: c.st.tickets, Reservations: c.st.reservations, Attempts: c.st.attempts})
	chosen := plan.ClaimNext(c.l.Pool)
	if chosen == nil {
		code, detail := plan.refusal()
		return c.refuse(mutation.OutcomeBlocked, code, detail)
	}
	next := *c.l
	next.Verb, next.TicketID = LeaseClaim, chosen.Ticket.TicketID.Raw
	c.l = &next
	return planClaim(c)
}

// refusal names why CLAIM_NEXT found no entry: RESOURCE_COLLISION when every
// SELECTED entry requires an unrequested pool, otherwise the first entry's
// reason, or TICKET_STATE when no ticket is OPEN or HELD.
func (p TicketPlan) refusal() (string, string) {
	if len(p.Entries) == 0 {
		return wire.CodeTicketState, "no ticket is OPEN or HELD"
	}
	if s := p.Selected(); s != nil {
		// Every SELECTED entry requires a pool this claim did not request (CAL-V0-097).
		return wire.CodeResourceCollision, "no SELECTED ticket is claimable without --pool; the first, " + s.Ticket.TicketID.Raw + ", requires pool " + s.Ticket.RequiresPool
	}
	first := p.Entries[0]
	detail := "no ticket is SELECTED; the first of " + string(wire.CountOf(int64(len(p.Entries)))) + " planned tickets, " + first.Ticket.TicketID.Raw + ", is " + first.State + " " + first.Reason
	if first.Reason == wire.CodeEscalationPending {
		detail += " on " + strings.Join(first.Ticket.EscalationPending(), ",")
	}
	return first.Reason, detail
}

func (c leaseContext) admit(rec *ticket.Record, sc *snapshot.Scope) leaseOutcome {
	if reap := c.expiredBlocking(rec.TicketID.Raw, sc.Resources); len(reap) != 0 {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "expired leases block this claim until reaped")
		out.result.Expired = reap
		return out
	}
	if live := c.liveOn(rec.TicketID.Raw); live != "" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "attempt "+live+" is live")
	}
	if other := c.collision(sc.Resources, ""); other != "" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "scope collides with live attempt "+other)
	}
	if int64(len(c.st.reservations.Entries)) >= c.st.policy.MaxActiveAttempts.Int() {
		return c.refuse(mutation.OutcomeCapacityExhausted, wire.CodeLimitExceeded, "maxActiveAttempts "+string(c.st.policy.MaxActiveAttempts)+" attempts are live")
	}
	prior, refusal := c.retryOf(rec)
	if refusal != nil {
		return *refusal
	}
	a, e := c.admitted(rec, prior, sc)
	if e != nil {
		return c.fail(e)
	}
	entry := snapshot.ReservationEntry{AttemptID: a.AttemptID, Generation: a.Generation, TicketID: a.TicketID, TicketRevision: a.TicketRevision, Resources: sc.Resources, CapacityUses: []snapshot.CapacityUse{}, Workers: "0", State: "ACTIVE", CreatedSeq: c.seq, Coverage: entryCoverage(sc)}
	return c.write(a, append(c.entries(), entry), "ADMIT", true)
}

// rescope replaces a live attempt's scope and its entry's resources in one
// transaction (CAL-V0-025).
func (c leaseContext) rescope(a *snapshot.Attempt, sc *snapshot.Scope) leaseOutcome {
	next := *a
	next.Scope = sc
	entries := c.entries()
	for i := range entries {
		if entries[i].AttemptID == a.AttemptID {
			entries[i].Resources, entries[i].Coverage = sc.Resources, entryCoverage(sc)
		}
	}
	return c.write(&next, entries, "TRANSITION", false)
}

func (c leaseContext) addedPaths(a *snapshot.Attempt) []string {
	have := map[string]bool{}
	for _, r := range a.Scope.Resources {
		if r.Class == "PATH" {
			have[r.Key] = true
		}
	}
	out := []string{}
	for _, p := range c.l.Scope {
		if !have[p] {
			out = append(out, p)
		}
	}
	return out
}

func planWiden(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	if a.Scope.Source == "WHOLE_REPOSITORY" {
		return c.unchanged()
	}
	// An admission barrier refuses scope-expand (TCP-00 §3.4).
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
	}
	if c.l.WholeRepository {
		return c.widenWhole(a)
	}
	added := c.addedPaths(a)
	if len(added) == 0 {
		return c.unchanged()
	}
	if len(a.Scope.Resources)+len(added) > snapshot.MaxScopeResources {
		return c.fail(limit("widened scope exceeds " + string(wire.CountOf(snapshot.MaxScopeResources)) + " resources"))
	}
	if other := c.collision(pathResources(added), a.AttemptID); other != "" {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "added paths collide with live attempt "+other)
	}
	resources := append(append([]ticket.Resource{}, a.Scope.Resources...), pathResources(added)...)
	return c.rescope(a, &snapshot.Scope{Source: a.Scope.Source, Resources: resources, DerivationSha256: a.Scope.DerivationSha256})
}

func (c leaseContext) widenWhole(a *snapshot.Attempt) leaseOutcome {
	for _, en := range c.st.reservations.Entries {
		if en.AttemptID != a.AttemptID {
			return c.refuse(mutation.OutcomeBlocked, wire.CodeResourceCollision, "WHOLE_REPOSITORY collides with live attempt "+en.AttemptID)
		}
	}
	return c.rescope(a, &snapshot.Scope{Source: "WHOLE_REPOSITORY", Resources: wholeRepository})
}
