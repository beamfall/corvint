package snapshot

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Gate result and completion manifest profiles (TCP-00 §7.1, §7.3). Both
// live only as evidence/<sha256> blobs named by an attempt.
const (
	ProfileGateResult = "taskman-gate-result/0"
	ProfileManifest   = "taskman-completion-manifest/0"
	// MaxGateRecordBytes bounds one gate result or manifest record.
	MaxGateRecordBytes = 64 * wire.KiB
	maxGateEvidence    = 64
)

// GateStates is the §6.1 gate state set; GateOutcomes the §7.1 outcome
// classes.
var (
	GateStates   = []string{"PENDING", "RUNNING", "PASSED", "FAILED", "BLOCKED", "NOT_RUN", "STALE"}
	GateOutcomes = []string{"EXIT", "TIMEOUT", "SIGNAL", "SPAWN_FAILED", "OUTPUT_LIMIT", "UNKNOWN"}
)

// GateEvidence is one captured evidence blob of a gate run.
type GateEvidence struct {
	Label  string
	Sha256 wire.Digest
	Bytes  wire.Size
}

// GateResult is a validated taskman-gate-result/0.
type GateResult struct {
	GateID            string
	AttemptID         string
	Generation        wire.Size
	TicketRevision    wire.Count
	DefinitionSha256  wire.Digest
	CandidateTreeOid  string
	ExecutedTreeOid   *string
	ExecutedCwd       *string
	PorcelainClean    *bool
	InputsSha256      wire.Digest
	EnvironmentSha256 wire.Digest
	PolicySha256      wire.Digest
	ConfigSha256      wire.Digest
	StartedAt         wire.Timestamp
	EndedAt           wire.Timestamp
	State             string
	OutcomeClass      string
	ExitCode          *wire.Count
	Signal            *string
	Evidence          []GateEvidence
	ReusedFrom        *wire.Digest
}

var gateResultFields = []string{"profile", "gateId", "attemptId", "generation", "ticketRevision", "definitionSha256", "candidateTreeOid", "executedTreeOid", "executedCwd", "porcelainClean", "inputsSha256", "environmentSha256", "policySha256", "configSha256", "startedAt", "endedAt", "state", "outcomeClass", "exitCode", "signal", "evidence", "reusedFrom"}

func readBoolOrNull(r *wire.Reader) *bool {
	if r.IsNull() {
		return nil
	}
	b := r.Bool()
	return &b
}

func boolOrNull(b *bool) wire.Value {
	if b == nil {
		return wire.Null()
	}
	return wire.Bool(*b)
}

func countOrNull(c *wire.Count) wire.Value {
	if c == nil {
		return wire.Null()
	}
	return wire.String(string(*c))
}

// DecodeGateResult parses and validates one gate result record.
func DecodeGateResult(data []byte) (*GateResult, error) {
	if len(data) > MaxGateRecordBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "gate result larger than %d bytes", MaxGateRecordBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed(gateResultFields...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileGateResult); err != nil {
		return nil, err
	}
	g := &GateResult{}
	g.GateID = r.Field("gateId").Label()
	g.AttemptID = r.Field("attemptId").Identifier()
	g.Generation = r.Field("generation").Size()
	g.TicketRevision = r.Field("ticketRevision").Count()
	g.DefinitionSha256 = r.Field("definitionSha256").Digest()
	g.CandidateTreeOid = r.Field("candidateTreeOid").OID()
	g.ExecutedTreeOid = r.Field("executedTreeOid").StringOrNull((*wire.Reader).OID)
	g.ExecutedCwd = r.Field("executedCwd").StringOrNull(func(x *wire.Reader) string { return x.Enum("WORKTREE", "CANDIDATE") })
	g.PorcelainClean = readBoolOrNull(r.Field("porcelainClean"))
	g.InputsSha256 = r.Field("inputsSha256").Digest()
	g.EnvironmentSha256 = r.Field("environmentSha256").Digest()
	g.PolicySha256 = r.Field("policySha256").Digest()
	g.ConfigSha256 = r.Field("configSha256").Digest()
	g.StartedAt = r.Field("startedAt").Timestamp()
	g.EndedAt = r.Field("endedAt").Timestamp()
	g.State = r.Field("state").Enum(GateStates...)
	g.OutcomeClass = r.Field("outcomeClass").Enum(GateOutcomes...)
	g.ExitCode = r.Field("exitCode").CountOrNull()
	g.Signal = r.Field("signal").StringOrNull((*wire.Reader).Identifier)
	g.Evidence = []GateEvidence{}
	for _, e := range r.Field("evidence").Array(maxGateEvidence, false) {
		e.Closed("label", "sha256", "bytes")
		g.Evidence = append(g.Evidence, GateEvidence{Label: e.Field("label").Label(), Sha256: e.Field("sha256").Digest(), Bytes: e.Field("bytes").Size()})
	}
	g.ReusedFrom = r.Field("reusedFrom").DigestOrNull()
	if err := r.Err(); err != nil {
		return nil, err
	}
	return g, g.check()
}

// check enforces the §7.1 PASSED predicate's record-level half: a PASSED
// result exited, ran clean, and ran at the candidate tree.
func (g *GateResult) check() error {
	if g.State != "PASSED" {
		return nil
	}
	clean := g.PorcelainClean != nil && *g.PorcelainClean
	atCandidate := g.ExecutedTreeOid != nil && *g.ExecutedTreeOid == g.CandidateTreeOid
	if g.OutcomeClass != "EXIT" || g.ExitCode == nil || !clean || !atCandidate {
		return wire.Errorf(wire.CodeMalformed, "/state", "PASSED requires EXIT, a clean worktree and the candidate tree")
	}
	return nil
}

// Encode renders the gate result and proves it decodes.
func (g *GateResult) Encode() ([]byte, error) {
	evidence := make([]wire.Value, 0, len(g.Evidence))
	for _, e := range g.Evidence {
		evidence = append(evidence, wire.ObjectValue(wire.NewObject().Set("label", wire.String(e.Label)).Set("sha256", wire.String(string(e.Sha256))).Set("bytes", wire.String(string(e.Bytes)))))
	}
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileGateResult))
	o.Set("gateId", wire.String(g.GateID))
	o.Set("attemptId", wire.String(g.AttemptID))
	o.Set("generation", wire.String(string(g.Generation)))
	o.Set("ticketRevision", wire.String(string(g.TicketRevision)))
	o.Set("definitionSha256", wire.String(string(g.DefinitionSha256)))
	o.Set("candidateTreeOid", wire.String(g.CandidateTreeOid))
	o.Set("executedTreeOid", wire.StringOrNull(g.ExecutedTreeOid))
	o.Set("executedCwd", wire.StringOrNull(g.ExecutedCwd))
	o.Set("porcelainClean", boolOrNull(g.PorcelainClean))
	o.Set("inputsSha256", wire.String(string(g.InputsSha256)))
	o.Set("environmentSha256", wire.String(string(g.EnvironmentSha256)))
	o.Set("policySha256", wire.String(string(g.PolicySha256)))
	o.Set("configSha256", wire.String(string(g.ConfigSha256)))
	o.Set("startedAt", wire.String(string(g.StartedAt)))
	o.Set("endedAt", wire.String(string(g.EndedAt)))
	o.Set("state", wire.String(g.State))
	o.Set("outcomeClass", wire.String(g.OutcomeClass))
	o.Set("exitCode", countOrNull(g.ExitCode))
	o.Set("signal", wire.StringOrNull(g.Signal))
	o.Set("evidence", wire.Array(evidence...))
	o.Set("reusedFrom", digestOrNull(g.ReusedFrom))
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if _, err := DecodeGateResult(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// Manifest is a taskman-completion-manifest/0 (§7.3). The external-agent
// completion writes it with empty acceptance, review and finding lists and
// null docs, CEM and OCM results, because it applies only reducer rows 1-6
// and 12 (CAL amendment A13).
type Manifest struct {
	Supervision        *Supervision
	AttemptID          string
	Generation         wire.Size
	TicketID           wire.TicketID
	TicketRevision     wire.Count
	TicketRecordSha256 wire.Digest
	PolicySha256       wire.Digest
	ConfigSha256       wire.Digest
	BaseCommit         string
	CandidateTreeOid   string
	GateResults        []string
	Budget             map[string]BudgetField
	ScopeCheck         string
	Mode               string
}

// Encode renders the manifest and proves it decodes.
func (m *Manifest) Encode() ([]byte, error) {
	o := wire.NewObject()
	o.Set("profile", wire.String(ProfileManifest))
	if m.Supervision != nil {
		o.Set("supervision", supervisionValue(m.Supervision))
	}
	o.Set("attemptId", wire.String(m.AttemptID))
	o.Set("generation", wire.String(string(m.Generation)))
	o.Set("ticketId", wire.String(m.TicketID.Raw))
	o.Set("ticketRevision", wire.String(string(m.TicketRevision)))
	o.Set("ticketRecordSha256", wire.String(string(m.TicketRecordSha256)))
	o.Set("policySha256", wire.String(string(m.PolicySha256)))
	o.Set("configSha256", wire.String(string(m.ConfigSha256)))
	o.Set("baseCommit", wire.String(m.BaseCommit))
	o.Set("candidateTreeOid", wire.String(m.CandidateTreeOid))
	o.Set("acceptanceMap", wire.Array())
	o.Set("gateResults", wire.Strings(m.GateResults))
	o.Set("reviews", wire.Array())
	o.Set("docsResult", wire.Null())
	o.Set("cem", wire.Null())
	o.Set("ocm", wire.Null())
	o.Set("unresolvedFindings", wire.Array())
	o.Set("budget", budgetValue(m.Budget))
	o.Set("budgetComplete", wire.Bool(false))
	o.Set("scopeCheck", wire.String(m.ScopeCheck))
	o.Set("mode", wire.String(m.Mode))
	raw := wire.EncodeFile(wire.ObjectValue(o))
	if _, err := DecodeManifest(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

var manifestFields = []string{"profile", "attemptId", "generation", "ticketId", "ticketRevision", "ticketRecordSha256", "policySha256", "configSha256", "baseCommit", "candidateTreeOid", "acceptanceMap", "gateResults", "reviews", "docsResult", "cem", "ocm", "unresolvedFindings", "budget", "budgetComplete", "scopeCheck", "mode"}

// DecodeManifest parses and validates a completion manifest in the shape
// the external-agent completion writes.
func DecodeManifest(data []byte) (*Manifest, error) {
	if len(data) > MaxGateRecordBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "manifest larger than %d bytes", MaxGateRecordBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed(wire.OptionalKeys(v, manifestFields, "supervision")...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), ProfileManifest); err != nil {
		return nil, err
	}
	m := &Manifest{}
	if wire.Has(v, "supervision") {
		m.Supervision = readSupervision(r.Field("supervision"))
	}
	m.AttemptID = r.Field("attemptId").Identifier()
	m.Generation = r.Field("generation").Size()
	m.TicketID = r.Field("ticketId").TicketID()
	m.TicketRevision = r.Field("ticketRevision").Count()
	m.TicketRecordSha256 = r.Field("ticketRecordSha256").Digest()
	m.PolicySha256 = r.Field("policySha256").Digest()
	m.ConfigSha256 = r.Field("configSha256").Digest()
	m.BaseCommit = r.Field("baseCommit").OID()
	m.CandidateTreeOid = r.Field("candidateTreeOid").OID()
	r.Field("acceptanceMap").Array(0, false)
	m.GateResults = r.Field("gateResults").Strings(-1, false, readDigestString)
	r.Field("reviews").Array(0, false)
	for _, name := range []string{"docsResult", "cem", "ocm"} {
		if !r.Field(name).IsNull() {
			r.Field(name).Fail(wire.CodeMalformed, "must be null")
		}
	}
	r.Field("unresolvedFindings").Array(0, false)
	m.Budget = readBudget(r.Field("budget"))
	if r.Field("budgetComplete").Bool() {
		r.Field("budgetComplete").Fail(wire.CodeMalformed, "budget is not observed")
	}
	m.ScopeCheck = r.Field("scopeCheck").Exact("WITHIN")
	m.Mode = r.Field("mode").Enum("DEVELOPMENT", "QUALIFIED")
	if err := r.Err(); err != nil {
		return nil, err
	}
	return m, nil
}
