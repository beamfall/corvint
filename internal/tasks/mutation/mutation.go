// Package mutation implements the taskman-mutation/0 envelope of SPEC §3.3,
// its operation-specific closed payloads, the taskman-outcome/0 result, the
// pure post-record computation of every §3.3 operation (TM-V0-003, TM-V0-005),
// the request-ID replay rule of TM-V0-006 against an explicit index, and the
// §3.3 ADOPT_FILE composition (R2 F3, AS-35).
//
// The package is pure: it depends on internal/wire, internal/ticket and
// internal/intent only. It reads no file, takes no lock, consults no clock and
// draws no randomness. Every fact it needs (the canonical record, the queue
// manifest, the policy, the inventory, attempt liveness, the request index,
// the logical timestamp and the trusted actor binding) is an explicit input,
// and every result is an explicit output: a Plan carrying the outcome, the
// planned post record and, for CREATE, the planned queue manifest. A Plan is
// never a receipt. Nothing here is durable, and a Plan with outcome COMPLETED
// means "this mutation would commit as the following post record"; the
// journal commit (receipt, request index entry, intent-file projection) is
// TCP-02/TCP-02b work and is the only thing that makes a mutation real.
//
// Security boundary (owner-requested expert panel). The envelope's
// actor.role and actor.id are untrusted data supplied by whoever wrote the
// envelope; they are not authentication. Apply and Adopt therefore require a
// separate trusted Binding in the Context that names the role and id of the
// invoking principal, and refuse UNAUTHORIZED when the binding is absent or
// the envelope actor differs from it in either field. The binding is never
// derived from the envelope, from ticket text, from the policy or from any
// registration; it must be furnished by the caller's authority layer. This
// library can validate a binding it is given; it cannot authenticate the
// process that supplies it, and in particular it cannot distinguish two
// processes running under the same UID. There is no default OWNER binding and
// no privileged entry point: an empty Binding fails closed. The runtime
// enforcement profile that decides who may furnish which binding remains
// qualification work outside this package.
package mutation

import (
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Profile is the mutation envelope profile (§3.3).
const Profile = "taskman-mutation/0"

// Operation names (§3.3).
const (
	OpCreate          = "CREATE"
	OpRefine          = "REFINE"
	OpPrioritize      = "PRIORITIZE"
	OpSetDependencies = "SET_DEPENDENCIES"
	OpSetGates        = "SET_GATES"
	OpSetEffects      = "SET_EFFECTS"
	OpHold            = "HOLD"
	OpReleaseHold     = "RELEASE_HOLD"
	OpReopen          = "REOPEN"
	OpArchive         = "ARCHIVE"
	OpRestore         = "RESTORE"
	OpCompleteManual  = "COMPLETE_MANUAL"
	OpGrantApproval   = "GRANT_APPROVAL"
	OpRevokeApproval  = "REVOKE_APPROVAL"
)

// Actor is the envelope's untrusted actor claim. It is compared against the
// trusted Binding of the Context and never used on its own.
type Actor struct {
	ID   string
	Role string
}

// Envelope is a decoded taskman-mutation/0.
type Envelope struct {
	RequestID        string
	Actor            Actor
	QueueID          wire.QueueID
	TargetID         *wire.TicketID // nil only for CREATE
	ExpectedRevision *wire.Count    // nil only for CREATE
	Operation        string
	Payload          Payload
	IssuedAt         wire.Timestamp
	// Raw is the exact envelope bytes as decoded (canonical body plus LF).
	// The TM-V0-006 request digest is the SHA-256 of these bytes.
	Raw []byte
}

// Sha256 is the SHA-256 of the canonical mutation bytes (TM-V0-006).
func (e *Envelope) Sha256() wire.Digest { return wire.Sum(e.Raw) }

// Payload is one operation's closed payload (§3.3 table).
type Payload interface {
	operation() string
}

// CreatePayload is the CREATE payload: the full record minus the fields the
// tool derives (ticketId, revision, acceptanceRevision, previousRecordSha256,
// status, archivedFrom, completion, holds, approvals, createdAt, updatedAt,
// updatedBy, shadowOverlay), plus an optional localToken.
type CreatePayload struct {
	LocalToken         *string
	Title              string
	Body               *string
	Kind               string
	Owner              *string
	Milestone          *string
	Priority           string
	Order              wire.Count
	Labels             []string
	Dependencies       []ticket.Dependency
	AcceptanceCriteria []string
	RequirementRefs    []string
	Source             ticket.Source
	Effects            ticket.Effects
	Capabilities       []string
	RequiredGates      []string
	ExecutionClass     string
	DueDate            *string
	EstimateMinutes    *wire.Count
	Supersedes         *wire.TicketID
	SupersededBy       *wire.TicketID
}

func (*CreatePayload) operation() string { return OpCreate }

// RefineFields are the keys a REFINE payload may carry (§3.3), sorted.
var RefineFields = []string{
	"acceptanceCriteria", "body", "dueDate", "estimateMinutes", "kind", "labels",
	"milestone", "owner", "requirementRefs", "supersedes", "title",
}

// RefinePayload is a non-empty subset of RefineFields. Present names the
// keys carried; a typed field is meaningful only when its key is present.
type RefinePayload struct {
	Present            map[string]bool
	Title              string
	Body               *string
	Kind               string
	Owner              *string
	Milestone          *string
	Labels             []string
	AcceptanceCriteria []string
	RequirementRefs    []string
	DueDate            *string
	EstimateMinutes    *wire.Count
	Supersedes         *wire.TicketID
}

func (*RefinePayload) operation() string { return OpRefine }

// Has reports whether the payload carries the key.
func (p *RefinePayload) Has(key string) bool { return p.Present[key] }

// Keys returns the present keys in sorted order.
func (p *RefinePayload) Keys() []string {
	var out []string
	for _, k := range RefineFields {
		if p.Present[k] {
			out = append(out, k)
		}
	}
	return out
}

// PrioritizePayload is {priority, order}.
type PrioritizePayload struct {
	Priority string
	Order    wire.Count
}

func (*PrioritizePayload) operation() string { return OpPrioritize }

// SetDependenciesPayload is the full replacement of dependencies.
type SetDependenciesPayload struct {
	Dependencies []ticket.Dependency
}

func (*SetDependenciesPayload) operation() string { return OpSetDependencies }

// SetGatesPayload is the full replacement of requiredGates.
type SetGatesPayload struct {
	RequiredGates []string
}

func (*SetGatesPayload) operation() string { return OpSetGates }

// SetEffectsPayload is the full replacement of effects, capabilities and
// executionClass.
type SetEffectsPayload struct {
	Effects        ticket.Effects
	Capabilities   []string
	ExecutionClass string
}

func (*SetEffectsPayload) operation() string { return OpSetEffects }

// HoldPayload is {holdId, reason}. The hold's actor and placedAt are never
// taken from the payload: they come from the trusted Binding and the logical
// timestamp of the Context.
type HoldPayload struct {
	HoldID string
	Reason string
}

func (*HoldPayload) operation() string { return OpHold }

// ReleaseHoldPayload is {holdId}.
type ReleaseHoldPayload struct {
	HoldID string
}

func (*ReleaseHoldPayload) operation() string { return OpReleaseHold }

// ReasonPayload is {reason} for REOPEN, ARCHIVE and RESTORE.
type ReasonPayload struct {
	Op     string
	Reason string
}

func (p *ReasonPayload) operation() string { return p.Op }

// CompleteManualPayload is {reason, evidence}.
type CompleteManualPayload struct {
	Reason   string
	Evidence []wire.Digest
}

func (*CompleteManualPayload) operation() string { return OpCompleteManual }

// GrantApprovalPayload is a grant minus revoked and grantedAt. The grant's
// actor must equal the trusted Binding id.
type GrantApprovalPayload struct {
	GrantID        string
	Actor          string
	Operation      string
	TargetRevision wire.Count
	Scope          []string
}

func (*GrantApprovalPayload) operation() string { return OpGrantApproval }

// RevokeApprovalPayload is {grantId, reason}.
type RevokeApprovalPayload struct {
	GrantID string
	Reason  string
}

func (*RevokeApprovalPayload) operation() string { return OpRevokeApproval }

var envelopeKeys = []string{
	"profile", "requestId", "actor", "queueId", "targetId", "expectedRevision", "operation", "payload", "issuedAt",
}

// createKeys is the closed CREATE payload without the optional localToken.
var createKeys = []string{
	"title", "body", "kind", "owner", "milestone", "priority", "order", "labels", "dependencies",
	"acceptanceCriteria", "requirementRefs", "source", "effects", "capabilities", "requiredGates",
	"executionClass", "dueDate", "estimateMinutes", "supersedes", "supersededBy",
}

// Decode parses and validates one mutation envelope (canonical bytes with
// trailing LF, ≤256 KiB). Every key is checked against the closed schema of
// §3.3, the payload against the closed table for its operation, and the
// nullity rule for targetId/expectedRevision. Decoding trusts nothing: the
// actor claim is carried through for comparison with the Binding by Apply.
func Decode(data []byte) (*Envelope, error) {
	if len(data) > wire.MaxMutationEnvelopeBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "mutation envelope larger than %d bytes", wire.MaxMutationEnvelopeBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed(envelopeKeys...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), Profile); err != nil {
		return nil, err
	}
	env := &Envelope{Raw: append([]byte(nil), data...)}
	env.RequestID = readRequestID(r.Field("requestId"))
	actor := r.Field("actor")
	actor.Closed("id", "role")
	env.Actor.ID = actor.Field("id").Label()
	env.Actor.Role = actor.Field("role").Enum(intent.Roles...)
	env.QueueID = r.Field("queueId").QueueID()
	tid := r.Field("targetId")
	if !tid.IsNull() {
		id := tid.TicketID()
		env.TargetID = &id
	}
	env.ExpectedRevision = r.Field("expectedRevision").CountOrNull()
	env.Operation = r.Field("operation").Enum(intent.Operations...)
	env.IssuedAt = r.Field("issuedAt").Timestamp()
	if err := r.Err(); err != nil {
		return nil, err
	}
	// §3.3: targetId/expectedRevision are null only for CREATE.
	if env.Operation == OpCreate {
		if env.TargetID != nil {
			return nil, wire.Errorf(wire.CodeMalformed, "/targetId", "CREATE carries a null targetId")
		}
		if env.ExpectedRevision != nil {
			return nil, wire.Errorf(wire.CodeMalformed, "/expectedRevision", "CREATE carries a null expectedRevision")
		}
	} else {
		if env.TargetID == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "/targetId", "%s requires a targetId", env.Operation)
		}
		if env.ExpectedRevision == nil {
			return nil, wire.Errorf(wire.CodeMalformed, "/expectedRevision", "%s requires an expectedRevision", env.Operation)
		}
		if env.TargetID.QueueID() != env.QueueID.Raw {
			return nil, wire.Errorf(wire.CodeMalformed, "/targetId", "target %s is outside queue %s", env.TargetID.Raw, env.QueueID.Raw)
		}
	}
	p, err := decodePayload(env.Operation, r.Field("payload"))
	if err != nil {
		return nil, err
	}
	env.Payload = p
	return env, nil
}

// readRequestID validates an Identifier bounded to the §1 requestId limit.
func readRequestID(r *wire.Reader) string {
	s := r.Identifier()
	if r.Err() == nil && len(s) > wire.MaxRequestIDBytes {
		r.Fail(wire.CodeLimitExceeded, "requestId longer than %d bytes", wire.MaxRequestIDBytes)
	}
	return s
}

// ParseRequestID validates a request ID supplied outside an envelope (the
// ADOPT_FILE request).
func ParseRequestID(where, s string) (string, error) {
	id, err := wire.ParseIdentifier(where, s)
	if err != nil {
		return "", err
	}
	if len(id) > wire.MaxRequestIDBytes {
		return "", wire.Errorf(wire.CodeLimitExceeded, where, "requestId longer than %d bytes", wire.MaxRequestIDBytes)
	}
	return id, nil
}

func decodePayload(op string, r *wire.Reader) (Payload, error) {
	var p Payload
	switch op {
	case OpCreate:
		p = readCreate(r)
	case OpRefine:
		p = readRefine(r)
	case OpPrioritize:
		r.Closed("priority", "order")
		p = &PrioritizePayload{Priority: readPriority(r.Field("priority")), Order: r.Field("order").Count()}
	case OpSetDependencies:
		r.Closed("dependencies")
		p = &SetDependenciesPayload{Dependencies: readDependencies(r.Field("dependencies"))}
	case OpSetGates:
		r.Closed("requiredGates")
		p = &SetGatesPayload{RequiredGates: readGates(r.Field("requiredGates"))}
	case OpSetEffects:
		r.Closed("effects", "capabilities", "executionClass")
		p = &SetEffectsPayload{
			Effects:        readEffects(r.Field("effects")),
			Capabilities:   readCapabilities(r.Field("capabilities")),
			ExecutionClass: r.Field("executionClass").Enum(ticket.ExecutionClasses...),
		}
	case OpHold:
		r.Closed("holdId", "reason")
		p = &HoldPayload{HoldID: r.Field("holdId").Label(), Reason: r.Field("reason").Prose(0, wire.MaxProseBytes)}
	case OpReleaseHold:
		r.Closed("holdId")
		p = &ReleaseHoldPayload{HoldID: r.Field("holdId").Label()}
	case OpReopen, OpArchive, OpRestore:
		r.Closed("reason")
		p = &ReasonPayload{Op: op, Reason: r.Field("reason").Prose(0, wire.MaxProseBytes)}
	case OpCompleteManual:
		r.Closed("reason", "evidence")
		cm := &CompleteManualPayload{Reason: r.Field("reason").Prose(0, wire.MaxProseBytes), Evidence: []wire.Digest{}}
		for _, e := range r.Field("evidence").Array(-1, false) {
			cm.Evidence = append(cm.Evidence, e.Digest())
		}
		p = cm
	case OpGrantApproval:
		r.Closed("grantId", "actor", "operation", "targetRevision", "scope")
		g := &GrantApprovalPayload{}
		g.GrantID = r.Field("grantId").Label()
		g.Actor = r.Field("actor").Label()
		g.Operation = r.Field("operation").Enum(ticket.ApprovalOps...)
		g.TargetRevision = r.Field("targetRevision").Count()
		g.Scope = nonNil(r.Field("scope").Strings(-1, false, (*wire.Reader).Identifier))
		p = g
	case OpRevokeApproval:
		r.Closed("grantId", "reason")
		p = &RevokeApprovalPayload{GrantID: r.Field("grantId").Label(), Reason: r.Field("reason").Prose(0, wire.MaxProseBytes)}
	default:
		return nil, wire.Errorf(wire.CodeMalformed, r.Where(), "unknown operation %q", op)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	return p, nil
}

func readCreate(r *wire.Reader) *CreatePayload {
	keys := createKeys
	hasLocal := false
	if v := r.Value(); v.Kind == wire.KindObject && v.Obj != nil {
		if _, ok := v.Obj.Get("localToken"); ok {
			hasLocal = true
			keys = append(append([]string{}, createKeys...), "localToken")
		}
	}
	r.Closed(keys...)
	p := &CreatePayload{}
	if hasLocal {
		lt := r.Field("localToken")
		s := lt.String()
		if lt.Err() == nil {
			if _, err := wire.ParseToken(lt.Where(), s, wire.MaxLocalTokenBytes); err != nil {
				lt.Fail(wire.CodeOf(err), "%v", err)
			}
		}
		p.LocalToken = &s
	}
	p.Title = r.Field("title").Prose(1, wire.MaxTitleBytes)
	p.Body = r.Field("body").ProseOrNull(wire.MaxBodyBytes)
	p.Kind = r.Field("kind").Enum(ticket.Kinds...)
	p.Owner = r.Field("owner").LabelOrNull()
	p.Milestone = r.Field("milestone").LabelOrNull()
	p.Priority = readPriority(r.Field("priority"))
	p.Order = r.Field("order").Count()
	p.Labels = readLabels(r.Field("labels"))
	p.Dependencies = readDependencies(r.Field("dependencies"))
	p.AcceptanceCriteria = readCriteria(r.Field("acceptanceCriteria"))
	p.RequirementRefs = readRequirementRefs(r.Field("requirementRefs"))
	p.Source = readSource(r.Field("source"))
	p.Effects = readEffects(r.Field("effects"))
	p.Capabilities = readCapabilities(r.Field("capabilities"))
	p.RequiredGates = readGates(r.Field("requiredGates"))
	p.ExecutionClass = r.Field("executionClass").Enum(ticket.ExecutionClasses...)
	p.DueDate = readDueDate(r.Field("dueDate"))
	p.EstimateMinutes = r.Field("estimateMinutes").CountOrNull()
	p.Supersedes = readTicketOrNull(r.Field("supersedes"))
	p.SupersededBy = readTicketOrNull(r.Field("supersededBy"))
	return p
}

func readRefine(r *wire.Reader) *RefinePayload {
	v := r.Value()
	if v.Kind != wire.KindObject || v.Obj == nil {
		r.Fail(wire.CodeMalformed, "expected object, got %s", v.Kind)
		return nil
	}
	allowed := map[string]bool{}
	for _, k := range RefineFields {
		allowed[k] = true
	}
	present := []string{}
	for _, k := range v.Obj.SortedKeys() {
		if !allowed[k] {
			r.Fail(wire.CodeMalformed, "REFINE payload carries unknown key %q", k)
			return nil
		}
		present = append(present, k)
	}
	if len(present) == 0 {
		r.Fail(wire.CodeMalformed, "REFINE payload carries no field")
		return nil
	}
	r.Closed(present...)
	p := &RefinePayload{Present: map[string]bool{}}
	for _, k := range present {
		p.Present[k] = true
		f := r.Field(k)
		switch k {
		case "title":
			p.Title = f.Prose(1, wire.MaxTitleBytes)
		case "body":
			p.Body = f.ProseOrNull(wire.MaxBodyBytes)
		case "kind":
			p.Kind = f.Enum(ticket.Kinds...)
		case "owner":
			p.Owner = f.LabelOrNull()
		case "milestone":
			p.Milestone = f.LabelOrNull()
		case "labels":
			p.Labels = readLabels(f)
		case "acceptanceCriteria":
			p.AcceptanceCriteria = readCriteria(f)
		case "requirementRefs":
			p.RequirementRefs = readRequirementRefs(f)
		case "dueDate":
			p.DueDate = readDueDate(f)
		case "estimateMinutes":
			p.EstimateMinutes = f.CountOrNull()
		case "supersedes":
			p.Supersedes = readTicketOrNull(f)
		}
	}
	return p
}

func nonNil(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

func readPriority(r *wire.Reader) string {
	s := r.String()
	if r.Err() != nil {
		return ""
	}
	for _, p := range ticket.Priorities {
		if p == s {
			return s
		}
	}
	r.Fail(wire.CodeInvalidPriority, "priority %q not in {P0|P1|P2|P3}", s)
	return ""
}

func readLabels(r *wire.Reader) []string {
	return nonNil(r.Strings(wire.MaxLabels, false, (*wire.Reader).Label))
}

func readCriteria(r *wire.Reader) []string {
	return nonNil(r.Strings(wire.MaxAcceptanceCriteria, true, func(c *wire.Reader) string {
		return c.Prose(0, wire.MaxCriterionBytes)
	}))
}

func readRequirementRefs(r *wire.Reader) []string {
	return nonNil(r.Strings(wire.MaxRequirementRefs, false, (*wire.Reader).Identifier))
}

func readCapabilities(r *wire.Reader) []string {
	return nonNil(r.Strings(wire.MaxCapabilities, false, (*wire.Reader).Identifier))
}

func readGates(r *wire.Reader) []string {
	return nonNil(r.Strings(wire.MaxRequiredGates, false, (*wire.Reader).Label))
}

func readDependencies(r *wire.Reader) []ticket.Dependency {
	out := []ticket.Dependency{}
	for _, d := range r.Array(wire.MaxDependencies, true) {
		d.Closed("ticketId", "obligation", "gateId")
		out = append(out, ticket.Dependency{
			TicketID:   d.Field("ticketId").TicketID(),
			Obligation: d.Field("obligation").Enum(ticket.Obligations...),
			GateID:     d.Field("gateId").LabelOrNull(),
		})
	}
	return out
}

func readSource(r *wire.Reader) ticket.Source {
	r.Closed("kind", "sourceQueueId", "sourceItemId", "sourceRevisionSha256")
	return ticket.Source{
		Kind:                 r.Field("kind").Enum(ticket.SourceKinds...),
		SourceQueueID:        r.Field("sourceQueueId").Identifier(),
		SourceItemID:         r.Field("sourceItemId").StringOrNull((*wire.Reader).Identifier),
		SourceRevisionSha256: r.Field("sourceRevisionSha256").DigestOrNull(),
	}
}

func readEffects(r *wire.Reader) ticket.Effects {
	r.Closed("coverage", "touchPaths", "resources", "externalUnbounded")
	e := ticket.Effects{Resources: []ticket.Resource{}}
	e.Coverage = r.Field("coverage").Enum(ticket.Coverages...)
	e.TouchPaths = nonNil(r.Field("touchPaths").Strings(wire.MaxTouchPaths, false, (*wire.Reader).Path))
	for _, rs := range r.Field("resources").Array(wire.MaxResources, false) {
		rs.Closed("class", "key")
		res := ticket.Resource{Class: rs.Field("class").Enum(ticket.ResourceClasses...), Key: rs.Field("key").Identifier()}
		if res.Class == "PATH" && rs.Err() == nil {
			if _, err := wire.ParsePath(rs.Field("key").Where(), res.Key); err != nil {
				rs.Field("key").Fail(wire.CodeOf(err), "PATH resource key: %v", err)
			}
		}
		e.Resources = append(e.Resources, res)
	}
	e.ExternalUnbounded = r.Field("externalUnbounded").Bool()
	return e
}

func readDueDate(r *wire.Reader) *string {
	if r.Err() != nil || r.IsNull() {
		return nil
	}
	s := r.String()
	if r.Err() == nil {
		if _, err := wire.ParseDate(r.Where(), s); err != nil {
			r.Fail(wire.CodeMalformed, "%v", err)
			return nil
		}
	}
	return &s
}

func readTicketOrNull(r *wire.Reader) *wire.TicketID {
	if r.Err() != nil || r.IsNull() {
		return nil
	}
	id := r.TicketID()
	if r.Err() != nil {
		return nil
	}
	return &id
}

// Value renders the envelope as a canonical wire value. It is the inverse of
// Decode for a decoded envelope and lets tests and the CLI build envelopes
// from typed payloads.
func (e *Envelope) Value() wire.Value {
	o := wire.NewObject()
	o.Set("profile", wire.String(Profile))
	o.Set("requestId", wire.String(e.RequestID))
	a := wire.NewObject()
	a.Set("id", wire.String(e.Actor.ID))
	a.Set("role", wire.String(e.Actor.Role))
	o.Set("actor", wire.ObjectValue(a))
	o.Set("queueId", wire.String(e.QueueID.Raw))
	if e.TargetID == nil {
		o.Set("targetId", wire.Null())
	} else {
		o.Set("targetId", wire.String(e.TargetID.Raw))
	}
	if e.ExpectedRevision == nil {
		o.Set("expectedRevision", wire.Null())
	} else {
		o.Set("expectedRevision", wire.String(string(*e.ExpectedRevision)))
	}
	o.Set("operation", wire.String(e.Operation))
	o.Set("payload", PayloadValue(e.Payload))
	o.Set("issuedAt", wire.String(string(e.IssuedAt)))
	return wire.ObjectValue(o)
}

// PayloadValue renders a payload as its closed wire object.
func PayloadValue(p Payload) wire.Value {
	o := wire.NewObject()
	switch p := p.(type) {
	case *CreatePayload:
		if p.LocalToken != nil {
			o.Set("localToken", wire.String(*p.LocalToken))
		}
		o.Set("title", wire.String(p.Title))
		o.Set("body", wire.StringOrNull(p.Body))
		o.Set("kind", wire.String(p.Kind))
		o.Set("owner", wire.StringOrNull(p.Owner))
		o.Set("milestone", wire.StringOrNull(p.Milestone))
		o.Set("priority", wire.String(p.Priority))
		o.Set("order", wire.String(string(p.Order)))
		o.Set("labels", wire.Strings(p.Labels))
		o.Set("dependencies", dependenciesValue(p.Dependencies))
		o.Set("acceptanceCriteria", wire.Strings(p.AcceptanceCriteria))
		o.Set("requirementRefs", wire.Strings(p.RequirementRefs))
		o.Set("source", sourceValue(p.Source))
		o.Set("effects", effectsValue(p.Effects))
		o.Set("capabilities", wire.Strings(p.Capabilities))
		o.Set("requiredGates", wire.Strings(p.RequiredGates))
		o.Set("executionClass", wire.String(p.ExecutionClass))
		o.Set("dueDate", wire.StringOrNull(p.DueDate))
		o.Set("estimateMinutes", countOrNull(p.EstimateMinutes))
		o.Set("supersedes", ticketOrNull(p.Supersedes))
		o.Set("supersededBy", ticketOrNull(p.SupersededBy))
	case *RefinePayload:
		for _, k := range p.Keys() {
			switch k {
			case "title":
				o.Set(k, wire.String(p.Title))
			case "body":
				o.Set(k, wire.StringOrNull(p.Body))
			case "kind":
				o.Set(k, wire.String(p.Kind))
			case "owner":
				o.Set(k, wire.StringOrNull(p.Owner))
			case "milestone":
				o.Set(k, wire.StringOrNull(p.Milestone))
			case "labels":
				o.Set(k, wire.Strings(p.Labels))
			case "acceptanceCriteria":
				o.Set(k, wire.Strings(p.AcceptanceCriteria))
			case "requirementRefs":
				o.Set(k, wire.Strings(p.RequirementRefs))
			case "dueDate":
				o.Set(k, wire.StringOrNull(p.DueDate))
			case "estimateMinutes":
				o.Set(k, countOrNull(p.EstimateMinutes))
			case "supersedes":
				o.Set(k, ticketOrNull(p.Supersedes))
			}
		}
	case *PrioritizePayload:
		o.Set("priority", wire.String(p.Priority))
		o.Set("order", wire.String(string(p.Order)))
	case *SetDependenciesPayload:
		o.Set("dependencies", dependenciesValue(p.Dependencies))
	case *SetGatesPayload:
		o.Set("requiredGates", wire.Strings(p.RequiredGates))
	case *SetEffectsPayload:
		o.Set("effects", effectsValue(p.Effects))
		o.Set("capabilities", wire.Strings(p.Capabilities))
		o.Set("executionClass", wire.String(p.ExecutionClass))
	case *HoldPayload:
		o.Set("holdId", wire.String(p.HoldID))
		o.Set("reason", wire.String(p.Reason))
	case *ReleaseHoldPayload:
		o.Set("holdId", wire.String(p.HoldID))
	case *ReasonPayload:
		o.Set("reason", wire.String(p.Reason))
	case *CompleteManualPayload:
		o.Set("reason", wire.String(p.Reason))
		ev := make([]string, len(p.Evidence))
		for i, d := range p.Evidence {
			ev[i] = string(d)
		}
		o.Set("evidence", wire.Strings(ev))
	case *GrantApprovalPayload:
		o.Set("grantId", wire.String(p.GrantID))
		o.Set("actor", wire.String(p.Actor))
		o.Set("operation", wire.String(p.Operation))
		o.Set("targetRevision", wire.String(string(p.TargetRevision)))
		o.Set("scope", wire.Strings(p.Scope))
	case *RevokeApprovalPayload:
		o.Set("grantId", wire.String(p.GrantID))
		o.Set("reason", wire.String(p.Reason))
	}
	return wire.ObjectValue(o)
}

func dependenciesValue(deps []ticket.Dependency) wire.Value {
	out := make([]wire.Value, 0, len(deps))
	for _, d := range deps {
		do := wire.NewObject()
		do.Set("ticketId", wire.String(d.TicketID.Raw))
		do.Set("obligation", wire.String(d.Obligation))
		do.Set("gateId", wire.StringOrNull(d.GateID))
		out = append(out, wire.ObjectValue(do))
	}
	return wire.Array(out...)
}

func sourceValue(s ticket.Source) wire.Value {
	so := wire.NewObject()
	so.Set("kind", wire.String(s.Kind))
	so.Set("sourceQueueId", wire.String(s.SourceQueueID))
	so.Set("sourceItemId", wire.StringOrNull(s.SourceItemID))
	if s.SourceRevisionSha256 == nil {
		so.Set("sourceRevisionSha256", wire.Null())
	} else {
		so.Set("sourceRevisionSha256", wire.String(string(*s.SourceRevisionSha256)))
	}
	return wire.ObjectValue(so)
}

func effectsValue(e ticket.Effects) wire.Value {
	eo := wire.NewObject()
	eo.Set("coverage", wire.String(e.Coverage))
	eo.Set("touchPaths", wire.Strings(e.TouchPaths))
	res := make([]wire.Value, 0, len(e.Resources))
	for _, r := range e.Resources {
		ro := wire.NewObject()
		ro.Set("class", wire.String(r.Class))
		ro.Set("key", wire.String(r.Key))
		res = append(res, wire.ObjectValue(ro))
	}
	eo.Set("resources", wire.Array(res...))
	eo.Set("externalUnbounded", wire.Bool(e.ExternalUnbounded))
	return wire.ObjectValue(eo)
}

func countOrNull(c *wire.Count) wire.Value {
	if c == nil {
		return wire.Null()
	}
	return wire.String(string(*c))
}

func ticketOrNull(t *wire.TicketID) wire.Value {
	if t == nil {
		return wire.Null()
	}
	return wire.String(t.Raw)
}
