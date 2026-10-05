package snapshot

import (
	"slices"

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
	AttemptPhases  = []string{"ADMITTED", "RUNNING", "BUILT", "CHECKING", "REVIEWING", "REPAIRING", "STOPPING", "QUARANTINED", "BLOCKED_RECOVERY", "FAILED", "CANCELLED", "READY_FOR_INTEGRATION", "WAITING", "RETURNED", "COMPLETED"}
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

// PriorGeneration is one closed generation. History is nil for a legacy
// entry, which records neither stage nor pool member (CAL-V0-096).
type PriorGeneration struct {
	Generation wire.Size
	Quiescence string
	ProvedSeq  wire.Size
	History    *GenerationHistory
}

// GenerationHistory is the stage and pool member a CAL-V0-096 entry copied
// from the ended generation; each is nil when that generation had none.
// PoolID and MemberID are nil together.
type GenerationHistory struct {
	Stage            *string
	PoolID, MemberID *string
	// HandoffTo and HandoffReason are the CAL-V0-082 recorded hand-off of
	// the ended generation; empty when it recorded none.
	HandoffTo, HandoffReason string
	// Loop is the CAL-V0-102 work evidence of the ended generation, recorded
	// only by a claim whose policy carries loopDetection; nil otherwise.
	Loop *LoopEvidence
}

// LoopEvidence is the optional CAL-V0-102 `loopEvidence` member of a prior
// generation: its retry-accounting disposition, candidate tree and the
// number of gate results and external reviews it recorded.
type LoopEvidence struct {
	Disposition          string
	CandidateTreeOid     *string
	GateResults, Reviews wire.Count
}

// LoopEvidenceOf copies the CAL-V0-102 work evidence of an ended
// external-agent generation; nil when it recorded no retry accounting.
func LoopEvidenceOf(a *Attempt) *LoopEvidence {
	if a == nil || a.RuntimeID != RuntimeExternalAgent || a.RetryAccounting == nil {
		return nil
	}
	e := &LoopEvidence{Disposition: a.RetryAccounting.Disposition, GateResults: wire.CountOf(int64(len(a.GateResults))), Reviews: wire.CountOf(int64(len(a.Reviews)))}
	if a.CandidateTreeOid != nil {
		tree := *a.CandidateTreeOid
		e.CandidateTreeOid = &tree
	}
	return e
}

// historyKeys are the CAL-V0-096 keys a prior generation carries together.
var historyKeys = []string{"stage", "poolId", "memberId"}

// HandoffReasons is the closed CAL-V0-083 `--handoff-reason` set.
var HandoffReasons = []string{"CHANGES_REQUESTED", "STAGE_COMPLETE", "STAGE_INCOMPLETE"}

// CheckHandoffTarget enforces CAL-V0-082/083 for a clean disposition of a
// generation that held stage: the target is a stage role, REVIEW_RETURNED
// targets implement, a reason needs a target, CHANGES_REQUESTED returns
// another stage's work to implement, STAGE_INCOMPLETE continues the same
// stage and STAGE_COMPLETE moves to another one.
func CheckHandoffTarget(where, disposition, stage, to, reason string) error {
	bad := func(detail string) error { return wire.Errorf(wire.CodeMalformed, where, "%s", detail) }
	if to == "" {
		if reason != "" {
			return bad("a handoff reason requires a handoff target")
		}
		return nil
	}
	if disposition != wire.CodeHandoff && disposition != wire.CodeReviewReturned {
		return bad("a handoff target requires a HANDOFF or REVIEW_RETURNED release")
	}
	if !slices.Contains(intent.StageRoles, to) {
		return bad("handoff target is not implement, review or integrate")
	}
	if disposition == wire.CodeReviewReturned && to != "implement" {
		return bad("REVIEW_RETURNED hands off to implement")
	}
	switch reason {
	case "":
	case "CHANGES_REQUESTED":
		if to != "implement" || stage == "implement" {
			return bad("CHANGES_REQUESTED returns review or integrate work to implement")
		}
	case "STAGE_INCOMPLETE":
		if to != stage {
			return bad("STAGE_INCOMPLETE hands off to the same stage")
		}
	case "STAGE_COMPLETE":
		if to == stage {
			return bad("STAGE_COMPLETE hands off to another stage")
		}
	default:
		return bad("unknown handoff reason")
	}
	return nil
}

// RetryAccounting records prospective generation-local observations. It is absent
// from legacy records. Supervised attempts may retain it after attachment, but
// only an external-agent terminal disposition can exempt a retry.
type RetryAccounting struct {
	FailedOrUnknown bool
	Disposition     string
}

const ProfileRetryAccounting = "taskman-retry-accounting/0"

// Attempt is a validated taskman-attempt/0.
type Attempt struct {
	DirectPoolAdmission      *DirectPoolAdmission
	LaneUntouchedAttestation *LaneUntouchedAttestation
	// OperatorNote pins the ticket's note reference at admission (ON-V0-007).
	OperatorNote *ticket.OperatorNoteReference
	// EscalationAnswers pins the same-acceptance answered escalations at
	// admission, sorted by request ID (ESC-V0-005). Absent when none.
	EscalationAnswers []EscalationAnswerRef

	// HandoffTo and HandoffReason are the CAL-V0-082/083 recorded target of
	// a clean terminal hand-off; empty when none was recorded.
	HandoffTo, HandoffReason string

	LastHeartbeatAt         *wire.Timestamp
	RetryReasons            map[string]wire.Count
	RetryAccounting         *RetryAccounting
	HandoffEvidence         string
	Supervision             *Supervision
	Stage                   string
	PoolAllocation          *PoolAllocation
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

// NextStage is the CAL-V0-084 recorded next stage of this generation: the
// recorded target of a clean terminal hand-off, implement for a
// REVIEW_RETURNED without one, and empty otherwise.
func (a *Attempt) NextStage() string {
	if a.Live() || a.RetryAccounting == nil || (a.RetryAccounting.Disposition != wire.CodeHandoff && a.RetryAccounting.Disposition != wire.CodeReviewReturned) {
		return ""
	}
	if a.HandoffTo != "" {
		return a.HandoffTo
	}
	if a.RetryAccounting.Disposition == wire.CodeReviewReturned {
		return "implement"
	}
	return ""
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

// EscalationAnswerRef names one answered escalation by its origin and head
// event digests. The blobs live in the evidence store.
type EscalationAnswerRef struct {
	RequestID    string
	OriginSha256 wire.Digest
	HeadSha256   wire.Digest
}

// MaxEscalationAnswerRefs matches the ticket's escalation entry bound.
const MaxEscalationAnswerRefs = 64

func readEscalationAnswers(r *wire.Reader) []EscalationAnswerRef {
	out := []EscalationAnswerRef{}
	for _, x := range r.Array(MaxEscalationAnswerRefs, true) {
		x.Closed("requestId", "originSha256", "headSha256")
		ref := EscalationAnswerRef{RequestID: x.Field("requestId").Identifier(), OriginSha256: x.Field("originSha256").Digest(), HeadSha256: x.Field("headSha256").Digest()}
		if r.Err() == nil && len(out) > 0 && out[len(out)-1].RequestID >= ref.RequestID {
			x.Fail(wire.CodeMalformed, "escalation answers must be strictly sorted by requestId")
		}
		out = append(out, ref)
	}
	if r.Err() == nil && len(out) == 0 {
		r.Fail(wire.CodeMalformed, "escalationAnswers must be absent when empty")
	}
	return out
}

// EscalationAnswersValue encodes pinned answer references in request order.
func EscalationAnswersValue(refs []EscalationAnswerRef) wire.Value {
	vals := make([]wire.Value, 0, len(refs))
	for _, x := range refs {
		vals = append(vals, wire.ObjectValue(wire.NewObject().Set("requestId", wire.String(x.RequestID)).Set("originSha256", wire.String(string(x.OriginSha256))).Set("headSha256", wire.String(string(x.HeadSha256)))))
	}
	return wire.Array(vals...)
}

func readPriorGenerations(r *wire.Reader) []PriorGeneration {
	out := []PriorGeneration{}
	for _, p := range r.Array(-1, true) {
		p.Closed(wire.OptionalKeys(p.Value(), []string{"generation", "quiescence", "provedSeq"}, append(append([]string{}, historyKeys...), "handoffTo", "handoffReason", "loopEvidence")...)...)
		g := PriorGeneration{Generation: p.Field("generation").Size(), Quiescence: p.Field("quiescence").Enum("PROVED", "FENCED"), ProvedSeq: p.Field("provedSeq").Size()}
		g.History = readGenerationHistory(p)
		readPriorHandoff(p, g.History)
		readPriorLoop(p, g.History)
		out = append(out, g)
	}
	return out
}

// readPriorHandoff reads the optional CAL-V0-082 hand-off keys of one prior
// generation. They need recorded history, and the decoder cannot see the
// ended generation's disposition, so only the stage rules are rechecked.
func readPriorHandoff(p *wire.Reader, h *GenerationHistory) {
	hasTo, hasReason := wire.Has(p.Value(), "handoffTo"), wire.Has(p.Value(), "handoffReason")
	if !hasTo && !hasReason {
		return
	}
	if h == nil || h.Stage == nil {
		p.Fail(wire.CodeMalformed, "a prior handoff target requires a recorded stage")
		return
	}
	if !hasTo {
		p.Fail(wire.CodeMalformed, "a handoff reason requires a handoff target")
		return
	}
	h.HandoffTo = p.Field("handoffTo").Enum(intent.StageRoles...)
	if hasReason {
		h.HandoffReason = p.Field("handoffReason").Enum(HandoffReasons...)
	}
	if p.Err() == nil {
		if err := CheckHandoffTarget(p.Where(), wire.CodeHandoff, *h.Stage, h.HandoffTo, h.HandoffReason); err != nil {
			p.Fail(wire.CodeMalformed, "%s", err.Error())
		}
	}
}

// readPriorLoop reads the optional CAL-V0-102 loopEvidence of one prior
// generation. It needs recorded history; a recorded hand-off target needs a
// clean HANDOFF or REVIEW_RETURNED disposition.
func readPriorLoop(p *wire.Reader, h *GenerationHistory) {
	if !wire.Has(p.Value(), "loopEvidence") {
		return
	}
	if h == nil {
		p.Fail(wire.CodeMalformed, "loopEvidence requires a recorded stage")
		return
	}
	x := p.Field("loopEvidence")
	x.Closed("disposition", "candidateTreeOid", "gateResults", "reviews")
	e := &LoopEvidence{Disposition: x.Field("disposition").Enum("NONE", wire.CodeHandoff, wire.CodeReviewReturned), CandidateTreeOid: x.Field("candidateTreeOid").StringOrNull((*wire.Reader).OID), GateResults: x.Field("gateResults").Count(), Reviews: x.Field("reviews").Count()}
	if p.Err() != nil {
		return
	}
	if h.HandoffTo != "" && e.Disposition == "NONE" {
		p.Fail(wire.CodeMalformed, "a prior handoff target requires a HANDOFF or REVIEW_RETURNED loopEvidence disposition")
		return
	}
	if e.Disposition == wire.CodeReviewReturned && (h.Stage == nil || *h.Stage != "review") {
		p.Fail(wire.CodeMalformed, "a REVIEW_RETURNED generation held review")
		return
	}
	h.Loop = e
}

// readGenerationHistory reads the CAL-V0-096 keys of one prior generation:
// all absent (legacy) or all present, with poolId and memberId null together.
func readGenerationHistory(p *wire.Reader) *GenerationHistory {
	present := 0
	for _, k := range historyKeys {
		if wire.Has(p.Value(), k) {
			present++
		}
	}
	if present == 0 {
		return nil
	}
	if present != len(historyKeys) {
		p.Fail(wire.CodeMalformed, "stage, poolId and memberId are present together")
		return nil
	}
	h := &GenerationHistory{Stage: p.Field("stage").StringOrNull(func(x *wire.Reader) string { return x.Enum(intent.StageRoles...) }), PoolID: p.Field("poolId").LabelOrNull(), MemberID: p.Field("memberId").LabelOrNull()}
	if p.Err() == nil && (h.PoolID == nil) != (h.MemberID == nil) {
		p.Fail(wire.CodeMalformed, "poolId and memberId are null together")
	}
	return h
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
	r.Closed(wire.OptionalKeys(v, attemptFields, "stage", "poolAllocation", "supervision", "retryAccounting", "handoffEvidence", "handoffTo", "handoffReason", "lastHeartbeatAt", "retryReasons", "directPoolAdmission", "laneUntouchedAttestation", "operatorNote", "escalationAnswers")...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileAttempt); err != nil {
		return nil, err
	}
	a := &Attempt{}
	if wire.Has(v, "directPoolAdmission") {
		a.DirectPoolAdmission = readDirectPoolAdmission(r.Field("directPoolAdmission"))
	}
	if wire.Has(v, "laneUntouchedAttestation") {
		a.LaneUntouchedAttestation = readLaneUntouched(r.Field("laneUntouchedAttestation"))
	}
	if wire.Has(v, "operatorNote") {
		note, err := ticket.OperatorNoteReferenceFromValue(r.Field("operatorNote").Value())
		if err != nil {
			return nil, err
		}
		a.OperatorNote = note
	}
	if wire.Has(v, "escalationAnswers") {
		a.EscalationAnswers = readEscalationAnswers(r.Field("escalationAnswers"))
	}
	if wire.Has(v, "lastHeartbeatAt") {
		x := r.Field("lastHeartbeatAt").Timestamp()
		a.LastHeartbeatAt = &x
	}
	if wire.Has(v, "retryReasons") {
		x := r.Field("retryReasons")
		x.Closed("EXPIRED", "RELEASED", "FAILED", "UNKNOWN")
		a.RetryReasons = map[string]wire.Count{}
		for _, k := range RetryReasonNames {
			a.RetryReasons[k] = x.Field(k).Count()
		}
	}
	if wire.Has(v, "handoffEvidence") {
		a.HandoffEvidence = r.Field("handoffEvidence").Identifier()
	}
	if wire.Has(v, "handoffTo") {
		a.HandoffTo = r.Field("handoffTo").Enum(intent.StageRoles...)
	}
	if wire.Has(v, "handoffReason") {
		a.HandoffReason = r.Field("handoffReason").Enum(HandoffReasons...)
	}
	if wire.Has(v, "retryAccounting") {
		x := r.Field("retryAccounting")
		x.Closed("profile", "failedOrUnknown", "disposition")
		x.Field("profile").Exact(ProfileRetryAccounting)
		a.RetryAccounting = &RetryAccounting{FailedOrUnknown: x.Field("failedOrUnknown").Bool(), Disposition: x.Field("disposition").Enum("NONE", "HANDOFF", "REVIEW_RETURNED")}
	}
	if wire.Has(v, "supervision") {
		a.Supervision = readSupervision(r.Field("supervision"))
	}
	if wire.Has(v, "stage") {
		a.Stage = r.Field("stage").Enum(intent.StageRoles...)
	}
	if wire.Has(v, "poolAllocation") {
		a.PoolAllocation = ReadPoolAllocation(r.Field("poolAllocation"))
	}
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
	if a.RetryReasons != nil {
		remaining := a.RetryCount.Int()
		for _, k := range RetryReasonNames {
			n := a.RetryReasons[k].Int()
			if n > remaining {
				r.Fail(wire.CodeMalformed, "retry reasons exceed retryCount")
				break
			}
			remaining -= n
		}
		if remaining != 0 {
			r.Fail(wire.CodeMalformed, "retry reasons must sum to retryCount")
		}
	}
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
	if err := a.checkLaneUntouched(); err != nil {
		return err
	}
	q, err := AttemptQueue(a.AttemptID)
	if err != nil {
		return err
	}
	if q.Raw != a.TicketID.QueueID() {
		return wire.Errorf(wire.CodeMalformed, "/attemptId", "attempt and ticket name different queues")
	}
	clean := a.RetryAccounting != nil && a.RetryAccounting.Disposition != "NONE"
	if a.HandoffEvidence != "" && !clean {
		return wire.Errorf(wire.CodeMalformed, "/handoffEvidence", "evidence requires a clean terminal handoff")
	}
	if clean {
		x := a.RetryAccounting
		candidate := a.HandoffEvidence == "" && a.CandidateTreeOid != nil && a.ScopeCheck == "WITHIN"
		externalWork := a.HandoffEvidence != "" && a.CandidateTreeOid == nil && a.ScopeCheck == "UNKNOWN" && len(a.GateResults) == 0
		if a.RuntimeID != RuntimeExternalAgent || a.Phase != "CANCELLED" || a.Quiescence != "FENCED" || x.FailedOrUnknown || (!candidate && !externalWork) || len(a.PendingEffects) != 0 || a.Cause == nil || *a.Cause != x.Disposition || (a.Stage != "implement" && a.Stage != "review" && a.Stage != "integrate") || (x.Disposition == "REVIEW_RETURNED" && a.Stage != "review") {
			return wire.Errorf(wire.CodeMalformed, "/retryAccounting", "clean disposition differs from terminal handoff facts")
		}
	}
	if a.HandoffTo != "" || a.HandoffReason != "" {
		disposition := ""
		if clean {
			disposition = a.RetryAccounting.Disposition
		}
		if err := CheckHandoffTarget("/handoffTo", disposition, a.Stage, a.HandoffTo, a.HandoffReason); err != nil {
			return err
		}
	}
	external := a.RuntimeID == RuntimeExternalAgent
	supervised := a.RuntimeID == SupervisedProfile
	if (external || supervised) != (a.Lease != nil) || (external || supervised) != (a.Scope != nil) {
		return wire.Errorf(wire.CodeMalformed, "/lease", "lease and scope are non-null exactly for %s", RuntimeExternalAgent)
	}
	if supervised != (a.Supervision != nil) {
		return wire.Errorf(wire.CodeMalformed, "supervision", "runtime binding")
	}
	if external && (a.Phase == "WAITING" || a.Phase == "RETURNED") {
		return wire.Errorf(wire.CodeMalformed, "phase", "supervised-only phase")
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
		o := wire.NewObject().Set("generation", wire.String(string(p.Generation))).Set("quiescence", wire.String(p.Quiescence)).Set("provedSeq", wire.String(string(p.ProvedSeq)))
		if h := p.History; h != nil {
			o.Set("stage", wire.StringOrNull(h.Stage)).Set("poolId", wire.StringOrNull(h.PoolID)).Set("memberId", wire.StringOrNull(h.MemberID))
			if h.HandoffTo != "" {
				o.Set("handoffTo", wire.String(h.HandoffTo))
			}
			if h.HandoffReason != "" {
				o.Set("handoffReason", wire.String(h.HandoffReason))
			}
			if e := h.Loop; e != nil {
				o.Set("loopEvidence", wire.ObjectValue(wire.NewObject().Set("disposition", wire.String(e.Disposition)).Set("candidateTreeOid", wire.StringOrNull(e.CandidateTreeOid)).Set("gateResults", wire.String(string(e.GateResults))).Set("reviews", wire.String(string(e.Reviews)))))
			}
		}
		vs = append(vs, wire.ObjectValue(o))
	}
	return wire.Array(vs...)
}

// Encode renders the attempt and proves it decodes.
func (a *Attempt) Encode() ([]byte, error) {
	if err := a.checkLaneUntouched(); err != nil {
		return nil, err
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileAttempt))
	if a.DirectPoolAdmission != nil {
		o.Set("directPoolAdmission", DirectPoolAdmissionValue(a.DirectPoolAdmission))
	}
	if a.LaneUntouchedAttestation != nil {
		o.Set("laneUntouchedAttestation", LaneUntouchedAttestationValue(a.LaneUntouchedAttestation))
	}
	if a.OperatorNote != nil {
		o.Set("operatorNote", a.OperatorNote.Value())
	}
	if len(a.EscalationAnswers) > 0 {
		o.Set("escalationAnswers", EscalationAnswersValue(a.EscalationAnswers))
	}
	if a.LastHeartbeatAt != nil {
		o.Set("lastHeartbeatAt", wire.String(string(*a.LastHeartbeatAt)))
	}
	if a.RetryReasons != nil {
		o.Set("retryReasons", RetryReasonsValue(a.RetryReasons))
	}
	if a.HandoffEvidence != "" {
		o.Set("handoffEvidence", wire.String(a.HandoffEvidence))
	}
	if a.HandoffTo != "" {
		o.Set("handoffTo", wire.String(a.HandoffTo))
	}
	if a.HandoffReason != "" {
		o.Set("handoffReason", wire.String(a.HandoffReason))
	}
	if a.RetryAccounting != nil {
		x := a.RetryAccounting
		o.Set("retryAccounting", wire.ObjectValue(wire.NewObject().Set("profile", wire.String(ProfileRetryAccounting)).Set("failedOrUnknown", wire.Bool(x.FailedOrUnknown)).Set("disposition", wire.String(x.Disposition))))
	}
	if a.Supervision != nil {
		o.Set("supervision", supervisionValue(a.Supervision))
	}
	if a.Stage != "" {
		o.Set("stage", wire.String(a.Stage))
	}
	if a.PoolAllocation != nil {
		o.Set("poolAllocation", PoolAllocationValue(a.PoolAllocation))
	}
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

// RetryReasonNames partition charged readmissions; UNKNOWN retains legacy debt.
var RetryReasonNames = []string{"EXPIRED", "RELEASED", "FAILED", "UNKNOWN"}

func RetryReasonsValue(reasons map[string]wire.Count) wire.Value {
	o := wire.NewObject()
	for _, k := range RetryReasonNames {
		n := reasons[k]
		if n == "" {
			n = "0"
		}
		o.Set(k, wire.String(string(n)))
	}
	return wire.ObjectValue(o)
}
