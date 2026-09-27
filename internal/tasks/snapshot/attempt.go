package snapshot

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Attempt and reservation profiles (TCP-00 §3.4) with the CAL-V0 A8-A12
// amendments: runtime external-agent, the lease, cause LEASE_EXPIRED,
// quiescence FENCED and the claim scope.
const (
	ProfileAttempt       = "taskman-attempt/0"
	ProfileReservations  = "taskman-reservation-set/0"
	RuntimeExternalAgent = "external-agent"
	CauseLeaseExpired    = "LEASE_EXPIRED"
	// MaxScopeResources bounds an attempt scope: every declared touch path
	// and PATH resource of one ticket, so a scoped attempt stays inline.
	MaxScopeResources = wire.MaxTouchPaths + wire.MaxResources
	// MaxEntryResources is the §3.4 reservation entry bound.
	MaxEntryResources = 4096
)

// AttemptPhases is the §6.1 phase set; TerminalPhases its terminal subset.
var (
	AttemptPhases  = []string{"ADMITTED", "RUNNING", "BUILT", "CHECKING", "REVIEWING", "REPAIRING", "STOPPING", "QUARANTINED", "BLOCKED_RECOVERY", "FAILED", "CANCELLED", "READY_FOR_INTEGRATION", "COMPLETED"}
	TerminalPhases = map[string]bool{"FAILED": true, "CANCELLED": true, "COMPLETED": true}
	ScopeSources   = []string{"DECLARED", "REQUESTED", "DERIVED", "WHOLE_REPOSITORY"}
)

// Lease is the A9 lease of an external-agent attempt.
type Lease struct {
	Holder     string
	GrantedSeq wire.Size
	ExpiresAt  wire.Timestamp
}

// Scope is the A12 claim scope.
type Scope struct {
	Source           string
	Resources        []ticket.Resource
	DerivationSha256 *wire.Digest
}

// Supervisor is the §3.4 supervisor identity.
type Supervisor struct{ Pid, StartTime wire.Size }

// Lane is the §3.4 lane identity.
type Lane struct {
	Pgid, LeaderPid, LeaderStartTime wire.Size
	SpawnEffectKey                   wire.Digest
}

// BudgetField is one BudgetUsage entry.
type BudgetField struct {
	Value *wire.Size
	State string
}

// PriorGeneration is one closed generation.
type PriorGeneration struct {
	Generation wire.Size
	Quiescence string
	ProvedSeq  wire.Size
}

// Attempt is a validated taskman-attempt/0.
type Attempt struct {
	AttemptID               string
	TicketID                wire.TicketID
	TicketRevision          wire.Count
	TicketRecordSha256      wire.Digest
	Generation              wire.Size
	Phase                   string
	PhaseSinceSeq           wire.Size
	Cause                   *string
	Mode                    string
	PlanSha256              *wire.Digest
	PolicySha256            wire.Digest
	ConfigSha256            wire.Digest
	RuntimeID               string
	CapabilityProfileSha256 wire.Digest
	BaseCommit              string
	Branch                  string
	WorktreePath            *string
	CandidateTreeOid        *string
	Supervisor              *Supervisor
	Lane                    *Lane
	Quiescence              string
	NoExec                  *string
	SpawnNoExecCount        wire.Count
	PendingEffects          []string
	RetryCount              wire.Count
	RepairRound             wire.Count
	Budget                  map[string]BudgetField
	GateResults             []string
	Reviews                 []string
	ManifestSha256          *wire.Digest
	ScopeCheck              string
	PriorGenerations        []PriorGeneration
	Lease                   *Lease
	Scope                   *Scope
}

// Live reports whether the attempt is in a non-terminal phase.
func (a *Attempt) Live() bool { return !TerminalPhases[a.Phase] }

func readCause(r *wire.Reader) string {
	s := r.String()
	if r.Err() == nil && !wire.IsCode(s) && s != CauseLeaseExpired {
		r.Fail(wire.CodeMalformed, "unknown cause %q", s)
	}
	return s
}

func readDigestString(r *wire.Reader) string { return string(r.Digest()) }

func readResources(r *wire.Reader, max int) []ticket.Resource {
	out := []ticket.Resource{}
	for _, rs := range r.Array(max, false) {
		rs.Closed("class", "key")
		res := ticket.Resource{Class: rs.Field("class").Enum(ticket.ResourceClasses...), Key: rs.Field("key").Identifier()}
		if res.Class == "PATH" && rs.Err() == nil {
			if _, err := wire.ParsePath(rs.Field("key").Where(), res.Key); err != nil {
				rs.Field("key").Fail(wire.CodeOf(err), "PATH resource key: %v", err)
			}
		}
		out = append(out, res)
	}
	return out
}

// ResourcesValue renders a resource set sorted by canonical bytes.
func ResourcesValue(where string, rs []ticket.Resource) (wire.Value, error) {
	vs := make([]wire.Value, 0, len(rs))
	for _, r := range rs {
		o := wire.NewObject()
		o.Set("class", wire.String(r.Class))
		o.Set("key", wire.String(r.Key))
		vs = append(vs, wire.ObjectValue(o))
	}
	return wire.SortedSet(where, vs)
}

func readLease(r *wire.Reader) *Lease {
	if r.IsNull() {
		return nil
	}
	r.Closed("holder", "grantedSeq", "expiresAt")
	return &Lease{Holder: r.Field("holder").Label(), GrantedSeq: r.Field("grantedSeq").Size(), ExpiresAt: r.Field("expiresAt").Timestamp()}
}

func readScope(r *wire.Reader) *Scope {
	if r.IsNull() {
		return nil
	}
	r.Closed("source", "resources", "derivationSha256")
	sc := &Scope{Source: r.Field("source").Enum(ScopeSources...), Resources: readResources(r.Field("resources"), MaxScopeResources), DerivationSha256: r.Field("derivationSha256").DigestOrNull()}
	if r.Err() == nil && (sc.Source == "DERIVED") != (sc.DerivationSha256 != nil) {
		r.Fail(wire.CodeMalformed, "derivationSha256 is non-null exactly for DERIVED")
	}
	return sc
}

func readSupervisor(r *wire.Reader) *Supervisor {
	if r.IsNull() {
		return nil
	}
	r.Closed("pid", "startTime")
	return &Supervisor{Pid: r.Field("pid").Size(), StartTime: r.Field("startTime").Size()}
}

func readLane(r *wire.Reader) *Lane {
	if r.IsNull() {
		return nil
	}
	r.Closed("pgid", "leaderPid", "leaderStartTime", "spawnEffectKey")
	return &Lane{Pgid: r.Field("pgid").Size(), LeaderPid: r.Field("leaderPid").Size(), LeaderStartTime: r.Field("leaderStartTime").Size(), SpawnEffectKey: r.Field("spawnEffectKey").Digest()}
}

func readBudget(r *wire.Reader) map[string]BudgetField {
	r.Closed(intent.LaneBudgetNames...)
	out := map[string]BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		f := r.Field(name)
		f.Closed("value", "state")
		out[name] = BudgetField{Value: f.Field("value").SizeOrNull(), State: f.Field("state").Enum("OBSERVED", "NOT_OBSERVED")}
	}
	return out
}

func readPriorGenerations(r *wire.Reader) []PriorGeneration {
	out := []PriorGeneration{}
	for _, p := range r.Array(-1, true) {
		p.Closed("generation", "quiescence", "provedSeq")
		out = append(out, PriorGeneration{Generation: p.Field("generation").Size(), Quiescence: p.Field("quiescence").Enum("PROVED", "FENCED"), ProvedSeq: p.Field("provedSeq").Size()})
	}
	return out
}

var attemptFields = []string{"profile", "attemptId", "ticketId", "ticketRevision", "ticketRecordSha256", "generation", "phase", "phaseSinceSeq", "cause", "mode", "planSha256", "policySha256", "configSha256", "runtimeId", "capabilityProfileSha256", "baseCommit", "branch", "worktreePath", "candidateTreeOid", "supervisor", "lane", "quiescence", "noExec", "spawnNoExecCount", "pendingEffects", "retryCount", "repairRound", "budget", "gateResults", "reviews", "manifestSha256", "scopeCheck", "priorGenerations", "lease", "scope"}

// DecodeAttempt parses and validates one attempts/<attemptId>.json.
func DecodeAttempt(data []byte) (*Attempt, error) {
	if len(data) > wire.MaxAttemptRecordBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "attempt larger than %d bytes", wire.MaxAttemptRecordBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed(attemptFields...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileAttempt); err != nil {
		return nil, err
	}
	a := &Attempt{}
	a.AttemptID = r.Field("attemptId").Identifier()
	a.TicketID = r.Field("ticketId").TicketID()
	a.TicketRevision = r.Field("ticketRevision").Count()
	a.TicketRecordSha256 = r.Field("ticketRecordSha256").Digest()
	a.Generation = r.Field("generation").Size()
	a.Phase = r.Field("phase").Enum(AttemptPhases...)
	a.PhaseSinceSeq = r.Field("phaseSinceSeq").Size()
	a.Cause = r.Field("cause").StringOrNull(readCause)
	a.Mode = r.Field("mode").Enum("DEVELOPMENT", "QUALIFIED")
	a.PlanSha256 = r.Field("planSha256").DigestOrNull()
	a.PolicySha256 = r.Field("policySha256").Digest()
	a.ConfigSha256 = r.Field("configSha256").Digest()
	a.RuntimeID = r.Field("runtimeId").Label()
	a.CapabilityProfileSha256 = r.Field("capabilityProfileSha256").Digest()
	a.BaseCommit = r.Field("baseCommit").OID()
	a.Branch = r.Field("branch").Label()
	a.WorktreePath = r.Field("worktreePath").StringOrNull((*wire.Reader).Identifier)
	a.CandidateTreeOid = r.Field("candidateTreeOid").StringOrNull((*wire.Reader).OID)
	a.Supervisor = readSupervisor(r.Field("supervisor"))
	a.Lane = readLane(r.Field("lane"))
	a.Quiescence = r.Field("quiescence").Enum("UNPROVED", "PROVED", "SURVIVORS", "FENCED")
	a.NoExec = r.Field("noExec").StringOrNull(func(x *wire.Reader) string { return x.Exact("PROVED") })
	a.SpawnNoExecCount = r.Field("spawnNoExecCount").Count()
	a.PendingEffects = r.Field("pendingEffects").Strings(-1, false, readDigestString)
	a.RetryCount = r.Field("retryCount").Count()
	a.RepairRound = r.Field("repairRound").Count()
	a.Budget = readBudget(r.Field("budget"))
	a.GateResults = r.Field("gateResults").Strings(-1, false, readDigestString)
	a.Reviews = r.Field("reviews").Strings(-1, false, readDigestString)
	a.ManifestSha256 = r.Field("manifestSha256").DigestOrNull()
	a.ScopeCheck = r.Field("scopeCheck").Enum("WITHIN", "OUT_OF_SCOPE", "UNKNOWN")
	a.PriorGenerations = readPriorGenerations(r.Field("priorGenerations"))
	a.Lease = readLease(r.Field("lease"))
	a.Scope = readScope(r.Field("scope"))
	if err := r.Err(); err != nil {
		return nil, err
	}
	return a, a.check()
}

// check enforces the cross-field amendment rules: A8 null supervisor/lane,
// A9 lease and A12 scope exactly for external-agent, and the attempt
// identity naming the ticket's queue.
func (a *Attempt) check() error {
	q, err := AttemptQueue(a.AttemptID)
	if err != nil {
		return err
	}
	if q.Raw != a.TicketID.QueueID() {
		return wire.Errorf(wire.CodeMalformed, "/attemptId", "attempt and ticket name different queues")
	}
	external := a.RuntimeID == RuntimeExternalAgent
	if external != (a.Lease != nil) || external != (a.Scope != nil) {
		return wire.Errorf(wire.CodeMalformed, "/lease", "lease and scope are non-null exactly for %s", RuntimeExternalAgent)
	}
	if external && (a.Supervisor != nil || a.Lane != nil) {
		return wire.Errorf(wire.CodeMalformed, "/supervisor", "%s has no supervisor or lane", RuntimeExternalAgent)
	}
	return nil
}

func digestOrNull(d *wire.Digest) wire.Value {
	if d == nil {
		return wire.Null()
	}
	return wire.String(string(*d))
}

func sizeOrNull(s *wire.Size) wire.Value {
	if s == nil {
		return wire.Null()
	}
	return wire.String(string(*s))
}

func (a *Attempt) optionalValues(o *wire.Object) error {
	o.Set("supervisor", wire.Null())
	if a.Supervisor != nil {
		so := wire.NewObject().Set("pid", wire.String(string(a.Supervisor.Pid))).Set("startTime", wire.String(string(a.Supervisor.StartTime)))
		o.Set("supervisor", wire.ObjectValue(so))
	}
	o.Set("lane", wire.Null())
	if a.Lane != nil {
		lo := wire.NewObject().Set("pgid", wire.String(string(a.Lane.Pgid))).Set("leaderPid", wire.String(string(a.Lane.LeaderPid))).Set("leaderStartTime", wire.String(string(a.Lane.LeaderStartTime))).Set("spawnEffectKey", wire.String(string(a.Lane.SpawnEffectKey)))
		o.Set("lane", wire.ObjectValue(lo))
	}
	o.Set("lease", wire.Null())
	if a.Lease != nil {
		lo := wire.NewObject().Set("holder", wire.String(a.Lease.Holder)).Set("grantedSeq", wire.String(string(a.Lease.GrantedSeq))).Set("expiresAt", wire.String(string(a.Lease.ExpiresAt)))
		o.Set("lease", wire.ObjectValue(lo))
	}
	o.Set("scope", wire.Null())
	if a.Scope == nil {
		return nil
	}
	res, err := ResourcesValue("/scope/resources", a.Scope.Resources)
	if err != nil {
		return err
	}
	so := wire.NewObject().Set("source", wire.String(a.Scope.Source)).Set("resources", res).Set("derivationSha256", digestOrNull(a.Scope.DerivationSha256))
	o.Set("scope", wire.ObjectValue(so))
	return nil
}

func budgetValue(b map[string]BudgetField) wire.Value {
	o := wire.NewObject()
	for _, name := range intent.LaneBudgetNames {
		f := b[name]
		o.Set(name, wire.ObjectValue(wire.NewObject().Set("value", sizeOrNull(f.Value)).Set("state", wire.String(f.State))))
	}
	return wire.ObjectValue(o)
}

func priorValue(ps []PriorGeneration) wire.Value {
	vs := make([]wire.Value, 0, len(ps))
	for _, p := range ps {
		vs = append(vs, wire.ObjectValue(wire.NewObject().Set("generation", wire.String(string(p.Generation))).Set("quiescence", wire.String(p.Quiescence)).Set("provedSeq", wire.String(string(p.ProvedSeq)))))
	}
	return wire.Array(vs...)
}

// Encode renders the attempt and proves it decodes.
func (a *Attempt) Encode() ([]byte, error) {
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileAttempt))
	o.Set("attemptId", wire.String(a.AttemptID))
	o.Set("ticketId", wire.String(a.TicketID.Raw))
	o.Set("ticketRevision", wire.String(string(a.TicketRevision)))
	o.Set("ticketRecordSha256", wire.String(string(a.TicketRecordSha256)))
	o.Set("generation", wire.String(string(a.Generation)))
	o.Set("phase", wire.String(a.Phase))
	o.Set("phaseSinceSeq", wire.String(string(a.PhaseSinceSeq)))
	o.Set("cause", wire.StringOrNull(a.Cause))
	o.Set("mode", wire.String(a.Mode))
	o.Set("planSha256", digestOrNull(a.PlanSha256))
	o.Set("policySha256", wire.String(string(a.PolicySha256)))
	o.Set("configSha256", wire.String(string(a.ConfigSha256)))
	o.Set("runtimeId", wire.String(a.RuntimeID))
	o.Set("capabilityProfileSha256", wire.String(string(a.CapabilityProfileSha256)))
	o.Set("baseCommit", wire.String(a.BaseCommit))
	o.Set("branch", wire.String(a.Branch))
	o.Set("worktreePath", wire.StringOrNull(a.WorktreePath))
	o.Set("candidateTreeOid", wire.StringOrNull(a.CandidateTreeOid))
	o.Set("quiescence", wire.String(a.Quiescence))
	o.Set("noExec", wire.StringOrNull(a.NoExec))
	o.Set("spawnNoExecCount", wire.String(string(a.SpawnNoExecCount)))
	o.Set("pendingEffects", wire.Strings(a.PendingEffects))
	o.Set("retryCount", wire.String(string(a.RetryCount)))
	o.Set("repairRound", wire.String(string(a.RepairRound)))
	o.Set("budget", budgetValue(a.Budget))
	o.Set("gateResults", wire.Strings(a.GateResults))
	o.Set("reviews", wire.Strings(a.Reviews))
	o.Set("manifestSha256", digestOrNull(a.ManifestSha256))
	o.Set("scopeCheck", wire.String(a.ScopeCheck))
	o.Set("priorGenerations", priorValue(a.PriorGenerations))
	if err := a.optionalValues(o); err != nil {
		return nil, err
	}
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if _, err := DecodeAttempt(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ReservationEntry is one reservation-set entry.
type ReservationEntry struct {
	AttemptID      string
	Generation     wire.Size
	TicketID       wire.TicketID
	TicketRevision wire.Count
	Resources      []ticket.Resource
	CapacityUses   []CapacityUse
	Workers        wire.Count
	State          string
	CreatedSeq     wire.Size
	Coverage       string
}

// CapacityUse is one reserved capacity class.
type CapacityUse struct {
	ClassID string
	Units   wire.Count
}

// ReservationSet is a validated taskman-reservation-set/0.
type ReservationSet struct {
	QueueID wire.QueueID
	Entries []ReservationEntry
}

func readEntry(e *wire.Reader) ReservationEntry {
	e.Closed("attemptId", "generation", "ticketId", "ticketRevision", "resources", "capacityUses", "workers", "state", "createdSeq", "coverage")
	en := ReservationEntry{AttemptID: e.Field("attemptId").Identifier(), Generation: e.Field("generation").Size(), TicketID: e.Field("ticketId").TicketID(), TicketRevision: e.Field("ticketRevision").Count()}
	en.Resources = readResources(e.Field("resources"), MaxEntryResources)
	en.CapacityUses = []CapacityUse{}
	for _, c := range e.Field("capacityUses").Array(-1, false) {
		c.Closed("classId", "units")
		en.CapacityUses = append(en.CapacityUses, CapacityUse{ClassID: c.Field("classId").Label(), Units: c.Field("units").Count()})
	}
	en.Workers = e.Field("workers").Count()
	en.State = e.Field("state").Enum("ACTIVE", "QUIESCING", "BLOCKED_RECOVERY")
	en.CreatedSeq = e.Field("createdSeq").Size()
	en.Coverage = e.Field("coverage").Enum("QUALIFIED", "WHOLE_REPOSITORY")
	return en
}

// DecodeReservations parses and validates reservations.json: at most
// MaxActiveAttempts entries, one per attempt, each naming this queue.
func DecodeReservations(data []byte) (*ReservationSet, error) {
	if len(data) > wire.MaxReservationSetBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "reservation set larger than %d bytes", wire.MaxReservationSetBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "entries")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileReservations); err != nil {
		return nil, err
	}
	s := &ReservationSet{QueueID: r.Field("queueId").QueueID(), Entries: []ReservationEntry{}}
	for _, e := range r.Field("entries").Array(wire.MaxActiveAttempts, false) {
		s.Entries = append(s.Entries, readEntry(e))
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return s, s.check()
}

func (s *ReservationSet) check() error {
	seen := map[string]bool{}
	for _, e := range s.Entries {
		q, err := AttemptQueue(e.AttemptID)
		if err != nil {
			return err
		}
		if q.Raw != s.QueueID.Raw || e.TicketID.QueueID() != s.QueueID.Raw {
			return wire.Errorf(wire.CodeMalformed, "/entries", "entry names another queue")
		}
		if seen[e.AttemptID] {
			return wire.Errorf(wire.CodeMalformed, "/entries", "two entries for %s", e.AttemptID)
		}
		seen[e.AttemptID] = true
	}
	return nil
}

func entryValue(e ReservationEntry) (wire.Value, error) {
	res, err := ResourcesValue("/entries/resources", e.Resources)
	if err != nil {
		return wire.Value{}, err
	}
	uses := make([]wire.Value, 0, len(e.CapacityUses))
	for _, c := range e.CapacityUses {
		uses = append(uses, wire.ObjectValue(wire.NewObject().Set("classId", wire.String(c.ClassID)).Set("units", wire.String(string(c.Units)))))
	}
	usesValue, err := wire.SortedSet("/entries/capacityUses", uses)
	if err != nil {
		return wire.Value{}, err
	}
	o := wire.NewObject()
	o.Set("attemptId", wire.String(e.AttemptID))
	o.Set("generation", wire.String(string(e.Generation)))
	o.Set("ticketId", wire.String(e.TicketID.Raw))
	o.Set("ticketRevision", wire.String(string(e.TicketRevision)))
	o.Set("resources", res)
	o.Set("capacityUses", usesValue)
	o.Set("workers", wire.String(string(e.Workers)))
	o.Set("state", wire.String(e.State))
	o.Set("createdSeq", wire.String(string(e.CreatedSeq)))
	o.Set("coverage", wire.String(e.Coverage))
	return wire.ObjectValue(o), nil
}

// Encode renders the reservation set, entries canonical-sorted, and proves
// it decodes.
func (s *ReservationSet) Encode() ([]byte, error) {
	vs := make([]wire.Value, 0, len(s.Entries))
	for _, e := range s.Entries {
		v, err := entryValue(e)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}
	entries, err := wire.SortedSet("/entries", vs)
	if err != nil {
		return nil, err
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileReservations))
	o.Set("queueId", wire.String(s.QueueID.Raw))
	o.Set("entries", entries)
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if _, err := DecodeReservations(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
