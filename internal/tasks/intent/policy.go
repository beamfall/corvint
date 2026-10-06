package intent

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// ProfilePolicy is the policy profile (§3.1).
const ProfilePolicy = "taskman-policy/0"

// Roles is the actor role vocabulary (§3.3).
var Roles = []string{"OWNER", "OPERATOR", "WORKER", "REVIEWER", "IMPORTER", "SYSTEM"}

// Operations is the mutation operation vocabulary (§3.3).
var Operations = []string{
	"CREATE", "REFINE", "PRIORITIZE", "SET_DEPENDENCIES", "SET_GATES", "SET_EFFECTS",
	"HOLD", "RELEASE_HOLD", "REOPEN", "ARCHIVE", "RESTORE", "COMPLETE_MANUAL", "GRANT_APPROVAL", "REVOKE_APPROVAL",
	"RELEASE_CREATE", "RELEASE_UPDATE", "RELEASE_CANDIDATE", "RELEASE_EXTERNAL_ATTEST", "RELEASE_MANUAL_ATTEST", "RELEASE_PROMOTE",
	"ESCALATE", "ANSWER", "NOTE_SET", "NOTE_CLEAR", "REVIEW_RECORD", "REVIEW_RESUBMIT",
	"ATTACH_EVIDENCE",
}

// TicketKinds mirrors ticket.Kinds for policy kind lists.
var TicketKinds = []string{"FEATURE", "BUG", "CHORE", "SPIKE", "DOC", "MANUAL", "EXTERNAL"}

// LaneBudgetNames are the lane budget field names (§3.1), sorted.
var LaneBudgetNames = []string{"cacheCreationTokens", "cacheReadTokens", "inputTokens", "outputTokens", "turns", "wallClockMinutes"}

// RuntimeRoles is the runtime role vocabulary (§3.1).
var RuntimeRoles = []string{"BUILDER", "REVIEWER", "VERIFIER", "REPAIR", "DOCS"}

// DefaultRoleMatrix is the §3.2 role matrix; policy may remove rows, never
// add them.
var DefaultRoleMatrix = map[string][]string{
	"OWNER":    Operations,
	"OPERATOR": {"CREATE", "REFINE", "PRIORITIZE", "SET_DEPENDENCIES", "SET_GATES", "SET_EFFECTS", "HOLD", "RELEASE_HOLD", "REOPEN", "ARCHIVE", "RESTORE", "COMPLETE_MANUAL", "GRANT_APPROVAL", "REVOKE_APPROVAL", "RELEASE_CREATE", "RELEASE_UPDATE", "RELEASE_CANDIDATE", "RELEASE_EXTERNAL_ATTEST"},
	"IMPORTER": {"CREATE", "REFINE"},
	"WORKER":   {"REFINE"},
	"SYSTEM":   {"HOLD"},
	"REVIEWER": {},
}

// GateDefinition is one policy gate (§3.1).
type GateDefinition struct {
	GateID         string
	Kind           string
	Argv           []string
	Cwd            string
	Env            []string
	TimeoutSeconds wire.Count
	ExpectedExit   *wire.Count
	Reducer        *string
	Evidence       []string
	Inputs         []string
	SharedResource *struct{ Class, Key string }
	Reusable       bool
	Required       bool
}

// RuntimeEntry is one policy runtime (§3.1).
type RuntimeEntry struct {
	RuntimeID               string
	PathSha256              wire.Digest
	FileSha256              wire.Digest
	Mode                    string
	ArgvPrefix              []string
	CapabilityProfileSha256 wire.Digest
	ObservedBudgetFields    []string
	Roles                   []string
	MaxWorkers              wire.Count
	Enabled                 bool
}

// LaneBudget is the per-lane cap set: the four token fields are Size and
// turns and wallClockMinutes are Count (§2 and §3.1, aligned by the B1
// resolution; witness TestTMV0002_AS10_LaneBudgetPrimitives).
type LaneBudget struct {
	InputTokens         wire.Size
	CacheCreationTokens wire.Size
	CacheReadTokens     wire.Size
	OutputTokens        wire.Size
	Turns               wire.Count
	WallClockMinutes    wire.Count
}

// CapacityClass is one capacity class.
type CapacityClass struct {
	ID    string
	Units wire.Count
}

// Policy is a validated taskman-policy/0. Raw holds the exact file bytes;
// the policy identity is derived from them.
type Policy struct {
	Supervision                *SupervisionPolicy
	Pools                      []Pool
	PolicyVersion              wire.Size
	Roles                      map[string][]string
	MaxActiveAttempts          wire.Count
	MaxWorkersTotal            wire.Count
	Classes                    []CapacityClass
	Lane                       LaneBudget
	TicketMultiplier           wire.Count
	RequireEnforcedFields      []string
	AdmissionsPerRevision      wire.Count
	RepairRounds               wire.Count
	MalformedReviewRetry       wire.Count
	GateRerunOnStale           wire.Count
	ReconcileAttempts          wire.Count
	EvidenceDays               wire.Count
	Gates                      []GateDefinition
	SerialFallback             string
	IntegrationRequiredKinds   []string
	AllowEmptyObligationsKinds []string
	ReviewLaneRequired         bool
	DocsLaneRequired           bool
	CemRequired                bool
	OcmRequired                bool
	Runtimes                   []RuntimeEntry
	AllowedEnvKeys             []string
	// ExternalReviews are the optional routing-only review gate definitions
	// (ERG-V0-009); absent means no external review gate exists.
	ExternalReviews []ExternalReviewDefinition
	// LoopDetection is the optional CAL-V0-102 no-progress loop policy;
	// nil (the key absent) disables loop detection.
	LoopDetection *LoopDetection
	Raw           []byte
}

// LoopDetection bounds the CAL-V0-102 no-progress loop signals: the
// consecutive no-progress clean hand-offs and the alternating
// implement/REVIEW_RETURNED pairs one acceptance revision may record before
// a derived LOOP_DETECTED hold. Each bound is 1..MaxLoopDetectionBound.
type LoopDetection struct {
	MaxNoProgressGenerations wire.Count
	MaxAlternatingReturns    wire.Count
}

// MaxLoopDetectionBound caps both loopDetection bounds.
const MaxLoopDetectionBound = 256

// GateIDs returns the set of defined gate ids.
func (p *Policy) GateIDs() map[string]bool {
	m := map[string]bool{}
	for _, g := range p.Gates {
		m[g.GateID] = true
	}
	return m
}

// Gate returns the definition of gateId, or nil.
func (p *Policy) Gate(gateID string) *GateDefinition {
	for i := range p.Gates {
		if p.Gates[i].GateID == gateID {
			return &p.Gates[i]
		}
	}
	return nil
}

// GateDefinitionSha256 is the SHA-256 of the canonical bytes of gateId's
// entry in the policy's gates array, the §7.1 definitionSha256. It is empty
// when the policy defines no such gate.
func (p *Policy) GateDefinitionSha256(gateID string) wire.Digest {
	v, err := wire.Parse(p.Raw)
	if err != nil {
		return ""
	}
	gates, _ := v.Obj.Get("gates")
	for _, g := range gates.Arr {
		id, _ := g.Obj.Get("gateId")
		if id.Str == gateID {
			return wire.Sum(wire.EncodeFile(g))
		}
	}
	return ""
}

func boundCount(r *wire.Reader, min, max int64) wire.Count {
	c := r.Count()
	if r.Err() != nil {
		return c
	}
	if c.Int() < min || c.Int() > max {
		r.Fail(wire.CodeLimitExceeded, "value %s outside %d..%d", c, min, max)
	}
	return c
}

func subsetOf(r *wire.Reader, got []string, allowed []string, what string) {
	set := map[string]bool{}
	for _, a := range allowed {
		set[a] = true
	}
	for _, g := range got {
		if !set[g] {
			r.Fail(wire.CodeMalformed, "%s %q is not permitted", what, g)
			return
		}
	}
}

// DecodePolicy parses and validates policy.json (≤256 KiB) including the
// §1 policy caps (max columns and min columns are rejected at load).
func DecodePolicy(data []byte) (*Policy, error) {
	if len(data) > wire.MaxPolicyFileBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "policy.json larger than %d bytes", wire.MaxPolicyFileBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	if err := wire.ProfileVersion("/profile", v, ProfilePolicy); err != nil {
		return nil, err
	}
	r.Closed(wire.OptionalKeys(v, []string{"profile", "policyVersion", "roles", "capacity", "budgets", "retries", "retention", "gates",
		"serialFallback", "integrationRequiredKinds", "allowEmptyObligationsKinds", "reviewLane", "docsLane",
		"cemRequired", "ocmRequired", "runtimes", "environment"}, "pools", "supervision", "externalReviews", "loopDetection")...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfilePolicy); err != nil {
		return nil, err
	}
	p := &Policy{Roles: map[string][]string{}, Raw: append([]byte(nil), data...)}
	if wire.Has(v, "supervision") {
		p.Supervision = readSupervisionPolicy(r.Field("supervision"))
	}
	p.PolicyVersion = r.Field("policyVersion").Size()
	roles := r.Field("roles")
	if roles.Value().Kind != wire.KindObject {
		roles.Fail(wire.CodeMalformed, "roles must be an object")
	} else {
		for _, role := range roles.Value().Obj.Keys {
			known := false
			for _, k := range Roles {
				if k == role {
					known = true
				}
			}
			if !known {
				roles.Fail(wire.CodeMalformed, "unknown role %q", role)
				break
			}
			ops := roles.Field(role).Strings(-1, false, func(c *wire.Reader) string { return c.Enum(Operations...) })
			subsetOf(roles.Field(role), ops, PolicyGrantable(role), "operation for "+role)
			p.Roles[role] = ops
		}
	}
	capr := r.Field("capacity")
	capr.Closed("maxActiveAttempts", "maxWorkersTotal", "classes")
	p.MaxActiveAttempts = boundCount(capr.Field("maxActiveAttempts"), 1, wire.MaxActiveAttempts)
	p.MaxWorkersTotal = boundCount(capr.Field("maxWorkersTotal"), 1, wire.MaxWorkersTotal)
	classIDs := map[string]bool{}
	for _, c := range capr.Field("classes").Array(-1, false) {
		c.Closed("id", "units")
		id := c.Field("id").Label()
		units := c.Field("units").Count()
		if c.Err() == nil && classIDs[id] {
			c.Fail(wire.CodeDuplicateID, "duplicate capacity class %q", id)
		}
		classIDs[id] = true
		p.Classes = append(p.Classes, CapacityClass{ID: id, Units: units})
	}
	b := r.Field("budgets")
	b.Closed("lane", "ticketMultiplier", "requireEnforcedFields")
	lane := b.Field("lane")
	lane.Closed("inputTokens", "cacheCreationTokens", "cacheReadTokens", "outputTokens", "turns", "wallClockMinutes")
	p.Lane.InputTokens = lane.Field("inputTokens").Size()
	p.Lane.CacheCreationTokens = lane.Field("cacheCreationTokens").Size()
	p.Lane.CacheReadTokens = lane.Field("cacheReadTokens").Size()
	p.Lane.OutputTokens = lane.Field("outputTokens").Size()
	p.Lane.Turns = lane.Field("turns").Count()
	p.Lane.WallClockMinutes = boundCount(lane.Field("wallClockMinutes"), 1, wire.MaxLaneWallMinutes)
	p.TicketMultiplier = boundCount(b.Field("ticketMultiplier"), 1, wire.MaxCountValue)
	p.RequireEnforcedFields = b.Field("requireEnforcedFields").Strings(-1, false, (*wire.Reader).Label)
	subsetOf(b.Field("requireEnforcedFields"), p.RequireEnforcedFields, LaneBudgetNames, "requireEnforcedFields entry")
	rt := r.Field("retries")
	rt.Closed("admissionsPerRevision", "repairRounds", "malformedReviewRetry", "gateRerunOnStale", "reconcileAttempts")
	p.AdmissionsPerRevision = boundCount(rt.Field("admissionsPerRevision"), 0, wire.MaxAdmissionsPerRev)
	p.RepairRounds = boundCount(rt.Field("repairRounds"), 0, wire.MaxRepairRounds)
	p.MalformedReviewRetry = boundCount(rt.Field("malformedReviewRetry"), 0, wire.MaxMalformedRetry)
	p.GateRerunOnStale = boundCount(rt.Field("gateRerunOnStale"), 0, wire.MaxGateRerunStale)
	p.ReconcileAttempts = boundCount(rt.Field("reconcileAttempts"), 0, wire.MaxReconcileAttempt)
	ret := r.Field("retention")
	ret.Closed("evidenceDays")
	p.EvidenceDays = boundCount(ret.Field("evidenceDays"), wire.MinEvidenceDays, wire.MaxCountValue)
	gateIDs := map[string]bool{}
	envKeys := r.Field("environment")
	envKeys.Closed("allowedEnvKeys")
	p.AllowedEnvKeys = envKeys.Field("allowedEnvKeys").Strings(-1, false, (*wire.Reader).Identifier)
	for _, g := range r.Field("gates").Array(-1, true) {
		g.Closed("gateId", "kind", "argv", "cwd", "env", "timeoutSeconds", "expected", "evidence", "inputs",
			"sharedResource", "reusable", "required")
		gd := GateDefinition{}
		gd.GateID = g.Field("gateId").Label()
		if g.Err() == nil && gateIDs[gd.GateID] {
			g.Field("gateId").Fail(wire.CodeDuplicateID, "duplicate gateId %q", gd.GateID)
		}
		gateIDs[gd.GateID] = true
		gd.Kind = g.Field("kind").Enum("COMMAND", "REVIEW", "DOCS", "MANUAL", "EXTERNAL")
		gd.Argv = g.Field("argv").Strings(wire.MaxGateArgv, true, (*wire.Reader).Identifier)
		if g.Err() == nil && len(gd.Argv) < 1 {
			g.Field("argv").Fail(wire.CodeMalformed, "argv must have 1..16 literal entries")
		}
		gd.Cwd = g.Field("cwd").Enum("WORKTREE", "CANDIDATE")
		gd.Env = g.Field("env").Strings(-1, false, (*wire.Reader).Identifier)
		subsetOf(g.Field("env"), gd.Env, p.AllowedEnvKeys, "env name")
		gd.TimeoutSeconds = boundCount(g.Field("timeoutSeconds"), 1, wire.MaxGateTimeoutSecs)
		ex := g.Field("expected")
		ex.Closed("exitCode", "reducer")
		gd.ExpectedExit = ex.Field("exitCode").CountOrNull()
		gd.Reducer = ex.Field("reducer").LabelOrNull()
		gd.Evidence = g.Field("evidence").Strings(-1, false, (*wire.Reader).Label)
		gd.Inputs = g.Field("inputs").Strings(-1, false, (*wire.Reader).Path)
		sr := g.Field("sharedResource")
		if !sr.IsNull() {
			sr.Closed("class", "key")
			gd.SharedResource = &struct{ Class, Key string }{
				sr.Field("class").Enum("PATH", "SHARED_GATE", "SCHEMA", "GENERATED_OUTPUT", "PORT", "DATABASE", "WHOLE_REPOSITORY", "OTHER"),
				sr.Field("key").Identifier(),
			}
		}
		gd.Reusable = g.Field("reusable").Bool()
		gd.Required = g.Field("required").Bool()
		p.Gates = append(p.Gates, gd)
	}
	p.SerialFallback = r.Field("serialFallback").Enum("BLOCK", "WHOLE_REPOSITORY")
	p.IntegrationRequiredKinds = r.Field("integrationRequiredKinds").Strings(-1, false, func(c *wire.Reader) string { return c.Enum(TicketKinds...) })
	p.AllowEmptyObligationsKinds = r.Field("allowEmptyObligationsKinds").Strings(-1, false, func(c *wire.Reader) string { return c.Enum(TicketKinds...) })
	rl := r.Field("reviewLane")
	rl.Closed("required")
	p.ReviewLaneRequired = rl.Field("required").Bool()
	dl := r.Field("docsLane")
	dl.Closed("required")
	p.DocsLaneRequired = dl.Field("required").Bool()
	p.CemRequired = r.Field("cemRequired").Bool()
	p.OcmRequired = r.Field("ocmRequired").Bool()
	runtimeIDs := map[string]bool{}
	for _, e := range r.Field("runtimes").Array(-1, true) {
		e.Closed("runtimeId", "executable", "argvPrefix", "capabilityProfileSha256", "observedBudgetFields", "roles", "maxWorkers", "enabled")
		re := RuntimeEntry{}
		re.RuntimeID = e.Field("runtimeId").Label()
		if e.Err() == nil && runtimeIDs[re.RuntimeID] {
			e.Field("runtimeId").Fail(wire.CodeDuplicateID, "duplicate runtimeId %q", re.RuntimeID)
		}
		runtimeIDs[re.RuntimeID] = true
		ex := e.Field("executable")
		ex.Closed("pathSha256", "fileSha256", "mode")
		re.PathSha256 = ex.Field("pathSha256").Digest()
		re.FileSha256 = ex.Field("fileSha256").Digest()
		re.Mode = ex.Field("mode").String()
		if ex.Err() == nil && !isOctalMode(re.Mode) {
			ex.Field("mode").Fail(wire.CodeMalformed, "mode must be four lowercase octal digits")
		}
		re.ArgvPrefix = e.Field("argvPrefix").Strings(wire.MaxRuntimeArgv, true, (*wire.Reader).Identifier)
		re.CapabilityProfileSha256 = e.Field("capabilityProfileSha256").Digest()
		re.ObservedBudgetFields = e.Field("observedBudgetFields").Strings(-1, false, (*wire.Reader).Label)
		subsetOf(e.Field("observedBudgetFields"), re.ObservedBudgetFields, LaneBudgetNames, "observedBudgetFields entry")
		re.Roles = e.Field("roles").Strings(-1, false, func(c *wire.Reader) string { return c.Enum(RuntimeRoles...) })
		re.MaxWorkers = boundCount(e.Field("maxWorkers"), 1, wire.MaxWorkersTotal)
		re.Enabled = e.Field("enabled").Bool()
		p.Runtimes = append(p.Runtimes, re)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	if wire.Has(v, "pools") {
		p.Pools = readPools(r.Field("pools"), p.AllowedEnvKeys)
		if err := r.Err(); err != nil {
			return nil, err
		}
	}
	if wire.Has(v, "externalReviews") {
		p.ExternalReviews = readExternalReviews(r.Field("externalReviews"), gateIDs)
		if err := r.Err(); err != nil {
			return nil, err
		}
	}
	if wire.Has(v, "loopDetection") {
		l := r.Field("loopDetection")
		l.Closed("maxNoProgressGenerations", "maxAlternatingReturns")
		p.LoopDetection = &LoopDetection{MaxNoProgressGenerations: boundCount(l.Field("maxNoProgressGenerations"), 1, MaxLoopDetectionBound), MaxAlternatingReturns: boundCount(l.Field("maxAlternatingReturns"), 1, MaxLoopDetectionBound)}
		if err := r.Err(); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func isOctalMode(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if s[i] < '0' || s[i] > '7' {
			return false
		}
	}
	return true
}

// PolicySha256 is the policy identity `policy:sha256:*` digest: the WQO
// §4.3 content identity over the canonical body (file bytes minus LF).
func (p *Policy) PolicySha256() wire.Digest {
	body := p.Raw
	if len(body) > 0 && body[len(body)-1] == '\n' {
		body = body[:len(body)-1]
	}
	return wire.ContentID("policy", ProfilePolicy, body)
}

// ExplicitGrantOperations are operations a policy row may name for a role
// although that role's default row omits them: an OPERATOR gets a note verb
// (ON-V0-004) or an escalation verb (ESC-V0-001, ESC-V0-004) only through an
// explicit policy.roles.OPERATOR row, never by default; the same holds for
// ATTACH_EVIDENCE (TEA-V0-001). No other role may be granted them.
var ExplicitGrantOperations = map[string][]string{"OPERATOR": {"NOTE_SET", "NOTE_CLEAR", "ESCALATE", "ANSWER", "ATTACH_EVIDENCE"}}

// PolicyGrantable is the closed set a policy row for role may list: its
// default row plus its explicit-only grants.
func PolicyGrantable(role string) []string {
	return append(append([]string{}, DefaultRoleMatrix[role]...), ExplicitGrantOperations[role]...)
}

// MaxExternalReviewDefinitions bounds policy externalReviews (ERG-V0-009).
const MaxExternalReviewDefinitions = 16

// ExternalReviewRecorderRoles are the roles a definition may let record a
// verdict. WORKER is never a recorder (ERG-V0-001).
var ExternalReviewRecorderRoles = []string{"OWNER", "OPERATOR", "REVIEWER"}

// ExternalReviewDefinition is one routing-only external review gate
// (ERG-V0-009). It grants nothing by default: a gate absent from policy
// refuses every record, and only the listed roles and stages are admitted.
type ExternalReviewDefinition struct {
	GateID                     string
	RecorderRoles              []string
	ReviewStages, AuthorStages []string
	RequireReviewerLease       bool
	// Sha256 is the definitionSha256: the digest of the canonical bytes of
	// this entry in the policy's externalReviews array.
	Sha256 wire.Digest
}

// ExternalReview returns the definition of gateID, or nil.
func (p *Policy) ExternalReview(gateID string) *ExternalReviewDefinition {
	for i := range p.ExternalReviews {
		if p.ExternalReviews[i].GateID == gateID {
			return &p.ExternalReviews[i]
		}
	}
	return nil
}

func readExternalReviews(r *wire.Reader, executable map[string]bool) []ExternalReviewDefinition {
	var out []ExternalReviewDefinition
	seen := map[string]bool{}
	items := r.Array(-1, true)
	if r.Err() == nil && (len(items) < 1 || len(items) > MaxExternalReviewDefinitions) {
		r.Fail(wire.CodeLimitExceeded, "externalReviews must hold 1..%d definitions", MaxExternalReviewDefinitions)
		return nil
	}
	for _, e := range items {
		e.Closed("gateId", "purpose", "recorderRoles", "reviewStages", "authorStages", "requireReviewerLease")
		d := ExternalReviewDefinition{GateID: e.Field("gateId").Label()}
		if e.Err() == nil && (seen[d.GateID] || executable[d.GateID]) {
			e.Field("gateId").Fail(wire.CodeDuplicateID, "duplicate gateId %q", d.GateID)
		}
		seen[d.GateID] = true
		e.Field("purpose").Exact("ROUTING_ONLY")
		d.RecorderRoles = e.Field("recorderRoles").Strings(-1, false, func(c *wire.Reader) string { return c.Enum(ExternalReviewRecorderRoles...) })
		d.ReviewStages = e.Field("reviewStages").Strings(-1, false, func(c *wire.Reader) string { return c.Enum(StageRoles...) })
		d.AuthorStages = e.Field("authorStages").Strings(-1, false, func(c *wire.Reader) string { return c.Enum(StageRoles...) })
		d.RequireReviewerLease = e.Field("requireReviewerLease").Bool()
		if e.Err() == nil && (len(d.RecorderRoles) < 1 || len(d.AuthorStages) < 1) {
			e.Fail(wire.CodeMalformed, "recorderRoles and authorStages must be non-empty")
		}
		d.Sha256 = wire.Sum(wire.EncodeFile(e.Value()))
		out = append(out, d)
	}
	return out
}
