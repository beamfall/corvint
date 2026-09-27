package transaction

import (
	"bytes"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Lease carries one CAL-V0 lease command against one external-agent attempt
// record and the reservation set (CAL-V0-007..013, CAL-V0-021..025).
// Request.Lease names the verb and its closed arguments.
const Lease = snapshot.StageLease

// Lease verbs and bounds (CAL-V0-012, CAL-V0-013).
const (
	LeaseClaim          = "CLAIM"
	LeaseRenew          = "RENEW"
	LeaseRelease        = "RELEASE"
	LeaseReap           = "REAP"
	LeaseWiden          = "WIDEN"
	DefaultLeaseMinutes = 60
	MinLeaseMinutes     = 5
	MaxLeaseMinutes     = 1440
	MaxRetries          = 3
	wholeRepositoryKey  = "repo"
	timestampLayout     = "2006-01-02T15:04:05Z"
)

// LeaseRequest is one lease command. Fields a verb does not take stay empty;
// the closed shape table below refuses any other combination.
type LeaseRequest struct {
	Verb, TicketID, Holder string
	LeaseMinutes           wire.Size
	Branch, Base           string
	Scope                  []string
	WholeRepository        bool
	AttemptID              string
	Generation             wire.Size
	Reason                 string
}

// LeaseFacts are the caller's observations for a CLAIM: the fresh attempt
// identity it minted, the resolved base commit, and the CAL-V0-022
// derivation (nil paths when the deriver abstained).
type LeaseFacts struct {
	AttemptID, BaseCommit string
	DerivedPaths          []string
	DerivationSha256      wire.Digest
}

// ExpiredLease names a live attempt whose lease has expired.
type ExpiredLease struct {
	AttemptID  string
	Generation wire.Size
}

const (
	fieldTicket = 1 << iota
	fieldHolder
	fieldMinutes
	fieldBranch
	fieldBase
	fieldScope
	fieldWhole
	fieldAttempt
	fieldGeneration
	fieldReason
)

type leaseShape struct{ required, allowed int }

var leaseShapes = map[string]leaseShape{
	LeaseClaim:   {fieldTicket | fieldHolder | fieldMinutes, fieldTicket | fieldHolder | fieldMinutes | fieldBranch | fieldBase | fieldScope},
	LeaseRenew:   {fieldAttempt | fieldGeneration | fieldMinutes, fieldAttempt | fieldGeneration | fieldMinutes},
	LeaseRelease: {fieldAttempt | fieldGeneration, fieldAttempt | fieldGeneration | fieldReason},
	LeaseReap:    {0, fieldAttempt | fieldGeneration},
	LeaseWiden:   {fieldAttempt | fieldGeneration, fieldAttempt | fieldGeneration | fieldScope | fieldWhole},
}

func (l *LeaseRequest) present() int {
	flags := map[int]bool{fieldTicket: l.TicketID != "", fieldHolder: l.Holder != "", fieldMinutes: l.LeaseMinutes != "", fieldBranch: l.Branch != "", fieldBase: l.Base != "", fieldScope: l.Scope != nil, fieldWhole: l.WholeRepository, fieldAttempt: l.AttemptID != "", fieldGeneration: l.Generation != "", fieldReason: l.Reason != ""}
	bits := 0
	for bit, set := range flags {
		if set {
			bits |= bit
		}
	}
	return bits
}

func checkShape(l *LeaseRequest) error {
	shape, ok := leaseShapes[l.Verb]
	if !ok {
		return malformed("unknown lease verb")
	}
	bits := l.present()
	if bits&shape.required != shape.required || bits&^shape.allowed != 0 {
		return malformed("lease arguments do not fit " + l.Verb)
	}
	if l.Verb == LeaseReap && bits != 0 && bits != fieldAttempt|fieldGeneration {
		return malformed("REAP names an attempt and its generation together or neither")
	}
	if l.Verb == LeaseWiden && (bits&fieldScope != 0) == (bits&fieldWhole != 0) {
		return malformed("WIDEN takes paths or the whole repository, exactly one")
	}
	return nil
}

func checkMinutes(m wire.Size) error {
	if m == "" {
		return nil
	}
	n, e := wire.ParseSize("leaseMinutes", string(m))
	if e != nil {
		return e
	}
	if n.Uint64() < MinLeaseMinutes || n.Uint64() > MaxLeaseMinutes {
		return malformed("lease minutes outside 5..1440")
	}
	return nil
}

// checkScopePaths accepts a sorted, duplicate-free, non-empty PATH list whose
// keys are resource identifiers.
func checkScopePaths(paths []string) error {
	if paths == nil {
		return nil
	}
	if len(paths) == 0 || len(paths) > wire.MaxTouchPaths {
		return limit("scope path count")
	}
	for i, p := range paths {
		if _, e := wire.ParsePath("scope", p); e != nil {
			return e
		}
		if _, e := wire.ParseIdentifier("scope", p); e != nil {
			return e
		}
		if i > 0 && paths[i-1] >= p {
			return malformed("scope paths must be sorted and duplicate-free")
		}
	}
	return nil
}

func checkLabels(values map[string]string) error {
	for where, v := range values {
		if v == "" {
			continue
		}
		if _, e := wire.ParseLabel(where, v); e != nil {
			return e
		}
	}
	return nil
}

func checkLeaseFields(l *LeaseRequest, q wire.QueueID) error {
	if l.TicketID != "" {
		id, e := wire.ParseTicketID("ticketId", l.TicketID)
		if e != nil {
			return e
		}
		if id.QueueID() != q.Raw {
			return malformed("lease ticket names another queue")
		}
	}
	if e := checkLabels(map[string]string{"holder": l.Holder, "branch": l.Branch}); e != nil {
		return e
	}
	if l.Base != "" {
		if _, e := wire.ParseOID("base", l.Base); e != nil {
			return e
		}
	}
	if e := checkMinutes(l.LeaseMinutes); e != nil {
		return e
	}
	if e := checkScopePaths(l.Scope); e != nil {
		return e
	}
	if l.AttemptID != "" {
		aq, e := snapshot.AttemptQueue(l.AttemptID)
		if e != nil {
			return e
		}
		if aq.Raw != q.Raw {
			return malformed("lease attempt names another queue")
		}
	}
	if l.Generation != "" {
		if _, e := wire.ParseSize("generation", string(l.Generation)); e != nil {
			return e
		}
	}
	if l.Reason != "" && !wire.IsCode(l.Reason) {
		return malformed("release reason is not a closed code")
	}
	return nil
}

func optionalString(v string) wire.Value {
	if v == "" {
		return wire.Null()
	}
	return s(v)
}

// leaseValue is the digest preimage of a lease command: every field present,
// null when the verb does not take it.
func leaseValue(l *LeaseRequest, q wire.QueueID) (wire.Value, error) {
	if e := checkShape(l); e != nil {
		return wire.Value{}, e
	}
	if e := checkLeaseFields(l, q); e != nil {
		return wire.Value{}, e
	}
	scope := wire.Null()
	if l.Scope != nil {
		scope = wire.Strings(l.Scope)
	}
	return object("verb", s(l.Verb), "ticketId", optionalString(l.TicketID), "holder", optionalString(l.Holder), "leaseMinutes", optionalString(string(l.LeaseMinutes)), "branch", optionalString(l.Branch), "base", optionalString(l.Base), "scope", scope, "wholeRepository", wire.Bool(l.WholeRepository), "attemptId", optionalString(l.AttemptID), "generation", optionalString(string(l.Generation)), "reason", optionalString(l.Reason)), nil
}

// leaseEffect is what a lease transaction records: the receipt kind, the
// attempt and generation it names, whether it advances the queue-wide
// generation (an admission, TM-V0-011), and its outcome, which is a
// recorded FENCED refusal when a stale command is refused (CAL-V0-009).
type leaseEffect struct {
	kind, attemptID string
	generation      wire.Size
	bumpHead        bool
	outcome         string
	codes           []string
}

type leaseOutcome struct {
	posts  map[string][]byte
	effect *leaseEffect
	detail string
	result *Result
}

type leaseContext struct {
	r   Request
	l   *LeaseRequest
	in  Input
	st  inputState
	seq wire.Size
}

var leasePlanners = map[string]func(leaseContext) leaseOutcome{
	LeaseClaim:   planClaim,
	LeaseRenew:   planRenew,
	LeaseRelease: planRelease,
	LeaseReap:    planReap,
	LeaseWiden:   planWiden,
}

func planLease(r Request, in Input, st inputState) leaseOutcome {
	c := leaseContext{r: r, l: r.Lease, in: in, st: st, seq: wire.SizeOf(st.head.LastSeq.Uint64() + 1)}
	if e := checkClock(in, st.head); e != nil {
		return c.refuse(mutation.OutcomeStorageFailed, "", e.Error())
	}
	return leasePlanners[r.Lease.Verb](c)
}

// checkClock refuses a transaction recorded earlier than the head receipt,
// so a clock that steps backward cannot revive an expired lease
// (CAL-V0-012).
func checkClock(in Input, head *snapshot.Head) error {
	if head.LastReceiptSha256 == nil || wire.Sum(in.HeadReceipt) != *head.LastReceiptSha256 {
		return malformed("head receipt observation differs from the head")
	}
	rc, e := snapshot.DecodeReceipt(in.HeadReceipt)
	if e != nil {
		return e
	}
	if in.RecordedAt < rc.RecordedAt {
		return malformed("recordedAt " + string(in.RecordedAt) + " is earlier than the head receipt's " + string(rc.RecordedAt))
	}
	return nil
}

func (c leaseContext) refuse(outcome, code, detail string) leaseOutcome {
	res := refused(c.r.RequestID, outcome, code, detail)
	return leaseOutcome{result: &res}
}

func (c leaseContext) fail(e error) leaseOutcome {
	res := failed(c.r.RequestID, e)
	return leaseOutcome{result: &res}
}

func (c leaseContext) unchanged() leaseOutcome {
	res := noChange(c.r.RequestID)
	return leaseOutcome{result: &res}
}

func (c leaseContext) named() (*snapshot.Attempt, error) {
	a := c.st.attempts[c.l.AttemptID]
	if a == nil {
		return nil, malformed("unknown attempt " + c.l.AttemptID)
	}
	return a, nil
}

func expired(a *snapshot.Attempt, now wire.Timestamp) bool {
	return a.Lease != nil && a.Lease.ExpiresAt <= now
}

func addMinutes(t wire.Timestamp, m wire.Size) wire.Timestamp {
	at, _ := time.Parse(timestampLayout, string(t))
	return wire.Timestamp(at.Add(time.Duration(m.Uint64()) * time.Minute).UTC().Format(timestampLayout))
}

// fenced says why a command of the request's generation can no longer take
// effect: another generation, a terminal phase, or an expired lease
// (CAL-V0-009, CAL-V0-010). Empty means the command may proceed.
func (c leaseContext) fenced(a *snapshot.Attempt) string {
	switch {
	case a.Generation.Uint64() != c.l.Generation.Uint64():
		return "generation " + string(c.l.Generation) + " is not the attempt's current generation " + string(a.Generation)
	case !a.Live():
		return "the attempt is " + a.Phase
	case expired(a, c.in.RecordedAt):
		return "the lease expired at " + string(a.Lease.ExpiresAt)
	}
	return ""
}

// recordFenced records the refusal of a stale command (TM-V0-011): the
// receipt posts only the request entry.
func (c leaseContext) recordFenced(a *snapshot.Attempt, detail string) leaseOutcome {
	eff := &leaseEffect{kind: "TRANSITION", attemptID: a.AttemptID, generation: c.l.Generation, outcome: mutation.OutcomeRevisionConflict, codes: []string{wire.CodeFenced}}
	return leaseOutcome{posts: map[string][]byte{}, effect: eff, detail: detail}
}

func attemptPath(id string) string { return "attempts/" + id + ".json" }

func (c leaseContext) entries() []snapshot.ReservationEntry {
	return append([]snapshot.ReservationEntry{}, c.st.reservations.Entries...)
}

func (c leaseContext) without(id string) []snapshot.ReservationEntry {
	out := []snapshot.ReservationEntry{}
	for _, e := range c.st.reservations.Entries {
		if e.AttemptID != id {
			out = append(out, e)
		}
	}
	return out
}

// write posts the attempt and, when entries is non-nil, the reservation set.
func (c leaseContext) write(a *snapshot.Attempt, entries []snapshot.ReservationEntry, kind string, bump bool) leaseOutcome {
	raw, e := a.Encode()
	if e != nil {
		return c.fail(e)
	}
	posts := map[string][]byte{attemptPath(a.AttemptID): raw}
	if entries != nil {
		set := snapshot.ReservationSet{QueueID: c.st.reservations.QueueID, Entries: entries}
		if posts["reservations.json"], e = set.Encode(); e != nil {
			return c.fail(e)
		}
	}
	eff := &leaseEffect{kind: kind, attemptID: a.AttemptID, generation: a.Generation, bumpHead: bump, outcome: mutation.OutcomeCompleted, codes: []string{}}
	return leaseOutcome{posts: posts, effect: eff}
}

func planRenew(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	next := *a
	next.Lease = &snapshot.Lease{Holder: a.Lease.Holder, GrantedSeq: c.seq, ExpiresAt: addMinutes(c.in.RecordedAt, c.l.LeaseMinutes)}
	return c.write(&next, nil, "TRANSITION", false)
}

func planRelease(c leaseContext) leaseOutcome {
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if why := c.fenced(a); why != "" {
		return c.recordFenced(a, why)
	}
	next := *a
	next.Phase, next.PhaseSinceSeq, next.Quiescence, next.Cause = "CANCELLED", c.seq, "FENCED", nil
	if c.l.Reason != "" {
		reason := c.l.Reason
		next.Cause = &reason
	}
	return c.write(&next, c.without(a.AttemptID), "TRANSITION", false)
}

// expiredAll lists every live attempt whose lease has expired, in
// reservation-entry order.
func (c leaseContext) expiredAll() []ExpiredLease {
	out := []ExpiredLease{}
	for _, en := range c.st.reservations.Entries {
		if expired(c.st.attempts[en.AttemptID], c.in.RecordedAt) {
			out = append(out, ExpiredLease{AttemptID: en.AttemptID, Generation: en.Generation})
		}
	}
	return out
}

// planReap without an attempt surveys the expired leases and writes nothing;
// the caller reaps each one in its own transaction (CAL-V0-011).
func planReap(c leaseContext) leaseOutcome {
	if c.l.AttemptID == "" {
		out := c.unchanged()
		out.result.Expired = c.expiredAll()
		return out
	}
	a, e := c.named()
	if e != nil {
		return c.fail(e)
	}
	if !a.Live() {
		return c.unchanged()
	}
	if a.Generation.Uint64() != c.l.Generation.Uint64() {
		return c.refuse(mutation.OutcomeRevisionConflict, wire.CodeFenced, "generation "+string(c.l.Generation)+" is not the attempt's current generation "+string(a.Generation))
	}
	if !expired(a, c.in.RecordedAt) {
		return c.refuse(mutation.OutcomeBlocked, wire.CodeAttemptLive, "the lease is live until "+string(a.Lease.ExpiresAt))
	}
	cause := snapshot.CauseLeaseExpired
	next := *a
	next.Phase, next.PhaseSinceSeq, next.Quiescence, next.Cause = "FAILED", c.seq, "FENCED", &cause
	return c.write(&next, c.without(a.AttemptID), "TRANSITION", false)
}

func sameResources(a, b []ticket.Resource) bool {
	x, e1 := snapshot.ResourcesValue("", a)
	y, e2 := snapshot.ResourcesValue("", b)
	return e1 == nil && e2 == nil && bytes.Equal(wire.EncodeFile(x), wire.EncodeFile(y))
}

// entryOracle answers attempt liveness from the reservation set: an entry
// exists exactly for each live attempt (validateInput).
type entryOracle struct{ set *snapshot.ReservationSet }

func (o entryOracle) LiveAttempt(id string) ticket.Observation {
	if o.set == nil {
		return ticket.Unsatisfied
	}
	for _, e := range o.set.Entries {
		if e.TicketID.Raw == id {
			return ticket.Satisfied
		}
	}
	return ticket.Unsatisfied
}
