package transaction

import (
	"sort"

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

// claimScope applies the CAL-V0-021 precedence: DECLARED, REQUESTED,
// DERIVED, then WHOLE_REPOSITORY.
func (c leaseContext) claimScope(rec *ticket.Record) (*snapshot.Scope, error) {
	if declared := Declared(rec); len(declared) > 0 {
		return &snapshot.Scope{Source: "DECLARED", Resources: pathResources(declared)}, nil
	}
	if c.l.Scope != nil {
		return &snapshot.Scope{Source: "REQUESTED", Resources: pathResources(c.l.Scope)}, nil
	}
	facts := c.in.LeaseFacts
	if facts.DerivedPaths == nil {
		return &snapshot.Scope{Source: "WHOLE_REPOSITORY", Resources: wholeRepository}, nil
	}
	if e := checkScopePaths(facts.DerivedPaths); e != nil {
		return nil, e
	}
	d, e := wire.ParseDigest("derivationSha256", string(facts.DerivationSha256))
	if e != nil {
		return nil, e
	}
	return &snapshot.Scope{Source: "DERIVED", Resources: pathResources(facts.DerivedPaths), DerivationSha256: &d}, nil
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

func (c leaseContext) lastAttempt(ticketID string) *snapshot.Attempt {
	var last *snapshot.Attempt
	for _, a := range c.st.attempts {
		if a.TicketID.Raw == ticketID && (last == nil || a.Generation.Uint64() > last.Generation.Uint64()) {
			last = a
		}
	}
	return last
}

// retryOf returns the terminal attempt this claim retries as its next
// generation (CAL-V0-013), nil for a fresh attempt, or a refusal once three
// retries at the ticket's acceptanceRevision are spent.
func (c leaseContext) retryOf(rec *ticket.Record) (*snapshot.Attempt, *leaseOutcome) {
	last := c.lastAttempt(rec.TicketID.Raw)
	if last == nil || last.Phase == "COMPLETED" || last.TicketRevision != rec.AcceptanceRevision {
		return nil, nil
	}
	if last.RetryCount.Int() >= MaxRetries {
		out := c.refuse(mutation.OutcomeBlocked, wire.CodeRetryExhausted, "three retries at acceptanceRevision "+string(rec.AcceptanceRevision)+" are spent")
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
	a.Lease = &snapshot.Lease{Holder: c.l.Holder, GrantedSeq: c.seq, ExpiresAt: addMinutes(c.in.RecordedAt, c.l.LeaseMinutes)}
	if prior == nil {
		a.AttemptID, e = c.freshID()
		return a, e
	}
	a.AttemptID = prior.AttemptID
	a.RetryCount = wire.CountOf(int64(prior.RetryCount.Int() + 1))
	a.PriorGenerations = append(append([]snapshot.PriorGeneration{}, prior.PriorGenerations...), snapshot.PriorGeneration{Generation: prior.Generation, Quiescence: prior.Quiescence, ProvedSeq: prior.PhaseSinceSeq})
	return a, nil
}

// planClaim admits one external-agent attempt by TCP-00 §4.1 steps 1, 2,
// 5, 6 and 8 (CAL-V0-007, CAL-V0-021, CAL-V0-023).
func planClaim(c leaseContext) leaseOutcome {
	if c.st.barrier != nil {
		return c.refuse(mutation.OutcomeBlocked, wire.CodePaused, "an admission barrier is present")
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
	sc, e := c.claimScope(rec)
	if e != nil {
		return c.fail(e)
	}
	return c.admit(rec, sc)
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
