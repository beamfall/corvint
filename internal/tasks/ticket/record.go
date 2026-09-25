// Package ticket implements the taskman-ticket/0 record of SPEC §3.1, its
// closed validation (TM-V0-002, TM-V0-003), the dependency checks of
// TM-V0-005 and the derived eligibility of §3.2 (TM-V0-004). It depends on
// internal/wire only: no journal, no filesystem, no process. Mutation
// computation (§3.3 envelopes and post records) is not part of this slice;
// it lands with TCP-02b against the real journal.
package ticket

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Profile is the ticket record profile.
const Profile = "taskman-ticket/0"

// Status values (§3.1).
const (
	StatusDraft     = "DRAFT"
	StatusOpen      = "OPEN"
	StatusHeld      = "HELD"
	StatusCompleted = "COMPLETED"
	StatusArchived  = "ARCHIVED"
)

// Kinds, priorities, execution classes, obligations, coverage and completion
// kinds (§3.1).
var (
	Kinds            = []string{"FEATURE", "BUG", "CHORE", "SPIKE", "DOC", "MANUAL", "EXTERNAL"}
	Priorities       = []string{"P0", "P1", "P2", "P3"}
	ExecutionClasses = []string{"AUTONOMOUS", "APPROVAL_REQUIRED", "MANUAL", "EXTERNAL", "NEVER"}
	Obligations      = []string{"COMPLETED", "GATE_PASSED"}
	Coverages        = []string{"QUALIFIED", "INCOMPLETE", "UNKNOWN"}
	ResourceClasses  = []string{"PATH", "SHARED_GATE", "SCHEMA", "GENERATED_OUTPUT", "PORT", "DATABASE", "WHOLE_REPOSITORY", "OTHER"}
	SourceKinds      = []string{"NATIVE", "IMPORT"}
	CompletionKinds  = []string{"VERIFIED", "MANUAL"}
	ApprovalOps      = []string{"RUN", "COMPLETE", "INTEGRATE", "ADJUDICATE"}
	Statuses         = []string{StatusDraft, StatusOpen, StatusHeld, StatusCompleted, StatusArchived}
	ArchivedFroms    = []string{StatusDraft, StatusOpen, StatusHeld, StatusCompleted}
)

// AcceptanceRelevantFields are the fields whose change bumps
// acceptanceRevision (§3.1). Exported so TCP-02b computes the bump from the
// same list.
var AcceptanceRelevantFields = []string{
	"kind", "acceptanceCriteria", "requirementRefs", "dependencies", "requiredGates",
	"effects", "capabilities", "executionClass", "supersedes", "source",
}

// Dependency is one `dependencies` entry.
type Dependency struct {
	TicketID   wire.TicketID
	Obligation string
	GateID     *string
}

// Source is the provenance object.
type Source struct {
	Kind                 string
	SourceQueueID        string
	SourceItemID         *string
	SourceRevisionSha256 *wire.Digest
}

// Resource is one declared effect resource.
type Resource struct {
	Class string
	Key   string
}

// Effects is the declared effect set.
type Effects struct {
	Coverage          string
	TouchPaths        []string
	Resources         []Resource
	ExternalUnbounded bool
}

// Hold is one `holds` entry.
type Hold struct {
	HoldID   string
	Actor    string
	Reason   string
	PlacedAt wire.Timestamp
}

// Approval is one `approvals` entry.
type Approval struct {
	GrantID        string
	Actor          string
	Operation      string
	TargetRevision wire.Count
	Scope          []string
	Revoked        bool
	GrantedAt      wire.Timestamp
}

// Completion is the completion object.
type Completion struct {
	Kind           string
	Actor          string
	Reason         *string
	Evidence       []wire.Digest
	ManifestSha256 *wire.Digest
	RecordedAt     wire.Timestamp
}

// Record is a validated taskman-ticket/0 record.
type Record struct {
	TicketID             wire.TicketID
	Revision             wire.Count
	AcceptanceRevision   wire.Count
	PreviousRecordSha256 *wire.Digest
	Status               string
	ArchivedFrom         *string
	Title                string
	Body                 *string
	Kind                 string
	Owner                *string
	Milestone            *string
	Priority             string
	Order                wire.Count
	Labels               []string
	Dependencies         []Dependency
	AcceptanceCriteria   []string
	RequirementRefs      []string
	Source               Source
	Effects              Effects
	Capabilities         []string
	RequiredGates        []string
	Holds                []Hold
	ExecutionClass       string
	Approvals            []Approval
	Completion           *Completion
	DueDate              *string
	EstimateMinutes      *wire.Count
	Supersedes           *wire.TicketID
	SupersededBy         *wire.TicketID
	ShadowOverlay        bool
	CreatedAt            wire.Timestamp
	UpdatedAt            wire.Timestamp
	UpdatedBy            string
}

var recordKeys = []string{
	"profile", "ticketId", "revision", "acceptanceRevision", "previousRecordSha256", "status",
	"archivedFrom", "title", "body", "kind", "owner", "milestone", "priority", "order", "labels",
	"dependencies", "acceptanceCriteria", "requirementRefs", "source", "effects", "capabilities",
	"requiredGates", "holds", "executionClass", "approvals", "completion", "dueDate",
	"estimateMinutes", "supersedes", "supersededBy", "shadowOverlay", "createdAt", "updatedAt",
	"updatedBy",
}

// Decode parses and validates one ticket record file (canonical bytes with
// trailing LF, ≤128 KiB).
func Decode(data []byte) (*Record, error) {
	if len(data) > wire.MaxTicketFileBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "ticket record file larger than %d bytes", wire.MaxTicketFileBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	return FromValue(v)
}

// FromValue validates a parsed value as a ticket record.
func FromValue(v wire.Value) (*Record, error) {
	r := wire.NewReader(v, "/")
	r.Closed(recordKeys...)
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), Profile); err != nil {
		return nil, err
	}
	rec := &Record{}
	rec.TicketID = r.Field("ticketId").TicketID()
	rec.Revision = r.Field("revision").Count()
	rec.AcceptanceRevision = r.Field("acceptanceRevision").Count()
	rec.PreviousRecordSha256 = r.Field("previousRecordSha256").DigestOrNull()
	rec.Status = r.Field("status").Enum(Statuses...)
	af := r.Field("archivedFrom")
	if !af.IsNull() {
		s := af.Enum(ArchivedFroms...)
		rec.ArchivedFrom = &s
	}
	rec.Title = r.Field("title").Prose(1, wire.MaxTitleBytes)
	rec.Body = r.Field("body").ProseOrNull(wire.MaxBodyBytes)
	rec.Kind = r.Field("kind").Enum(Kinds...)
	rec.Owner = r.Field("owner").LabelOrNull()
	rec.Milestone = r.Field("milestone").LabelOrNull()
	pr := r.Field("priority")
	ps := pr.String()
	if pr.Err() == nil {
		valid := false
		for _, p := range Priorities {
			if p == ps {
				valid = true
			}
		}
		if !valid {
			pr.Fail(wire.CodeInvalidPriority, "priority %q not in {P0|P1|P2|P3}", ps)
		}
	}
	rec.Priority = ps
	rec.Order = r.Field("order").Count()
	rec.Labels = r.Field("labels").Strings(wire.MaxLabels, false, (*wire.Reader).Label)
	for _, d := range r.Field("dependencies").Array(wire.MaxDependencies, true) {
		d.Closed("ticketId", "obligation", "gateId")
		dep := Dependency{}
		dep.TicketID = d.Field("ticketId").TicketID()
		dep.Obligation = d.Field("obligation").Enum(Obligations...)
		dep.GateID = d.Field("gateId").LabelOrNull()
		rec.Dependencies = append(rec.Dependencies, dep)
	}
	rec.AcceptanceCriteria = r.Field("acceptanceCriteria").Strings(wire.MaxAcceptanceCriteria, true, func(c *wire.Reader) string {
		return c.Prose(0, wire.MaxCriterionBytes)
	})
	rec.RequirementRefs = r.Field("requirementRefs").Strings(wire.MaxRequirementRefs, false, (*wire.Reader).Identifier)
	src := r.Field("source")
	src.Closed("kind", "sourceQueueId", "sourceItemId", "sourceRevisionSha256")
	rec.Source.Kind = src.Field("kind").Enum(SourceKinds...)
	rec.Source.SourceQueueID = src.Field("sourceQueueId").Identifier()
	rec.Source.SourceItemID = src.Field("sourceItemId").StringOrNull((*wire.Reader).Identifier)
	rec.Source.SourceRevisionSha256 = src.Field("sourceRevisionSha256").DigestOrNull()
	ef := r.Field("effects")
	ef.Closed("coverage", "touchPaths", "resources", "externalUnbounded")
	rec.Effects.Coverage = ef.Field("coverage").Enum(Coverages...)
	rec.Effects.TouchPaths = ef.Field("touchPaths").Strings(wire.MaxTouchPaths, false, (*wire.Reader).Path)
	for _, rs := range ef.Field("resources").Array(wire.MaxResources, false) {
		rs.Closed("class", "key")
		res := Resource{}
		res.Class = rs.Field("class").Enum(ResourceClasses...)
		res.Key = rs.Field("key").Identifier()
		if res.Class == "PATH" && rs.Err() == nil {
			if _, err := wire.ParsePath(rs.Field("key").Where(), res.Key); err != nil {
				rs.Field("key").Fail(wire.CodeOf(err), "PATH resource key: %v", err)
			}
		}
		rec.Effects.Resources = append(rec.Effects.Resources, res)
	}
	rec.Effects.ExternalUnbounded = ef.Field("externalUnbounded").Bool()
	rec.Capabilities = r.Field("capabilities").Strings(wire.MaxCapabilities, false, (*wire.Reader).Identifier)
	rec.RequiredGates = r.Field("requiredGates").Strings(wire.MaxRequiredGates, false, (*wire.Reader).Label)
	for _, h := range r.Field("holds").Array(wire.MaxHolds, true) {
		h.Closed("holdId", "actor", "reason", "placedAt")
		hold := Hold{}
		hold.HoldID = h.Field("holdId").Label()
		hold.Actor = h.Field("actor").Label()
		hold.Reason = h.Field("reason").Prose(0, wire.MaxProseBytes)
		hold.PlacedAt = h.Field("placedAt").Timestamp()
		rec.Holds = append(rec.Holds, hold)
	}
	rec.ExecutionClass = r.Field("executionClass").Enum(ExecutionClasses...)
	for _, a := range r.Field("approvals").Array(wire.MaxApprovals, true) {
		a.Closed("grantId", "actor", "operation", "targetRevision", "scope", "revoked", "grantedAt")
		ap := Approval{}
		ap.GrantID = a.Field("grantId").Label()
		ap.Actor = a.Field("actor").Label()
		ap.Operation = a.Field("operation").Enum(ApprovalOps...)
		ap.TargetRevision = a.Field("targetRevision").Count()
		ap.Scope = a.Field("scope").Strings(-1, false, (*wire.Reader).Identifier)
		ap.Revoked = a.Field("revoked").Bool()
		ap.GrantedAt = a.Field("grantedAt").Timestamp()
		rec.Approvals = append(rec.Approvals, ap)
	}
	cp := r.Field("completion")
	if !cp.IsNull() {
		cp.Closed("kind", "actor", "reason", "evidence", "manifestSha256", "recordedAt")
		c := &Completion{}
		c.Kind = cp.Field("kind").Enum(CompletionKinds...)
		c.Actor = cp.Field("actor").Label()
		c.Reason = cp.Field("reason").ProseOrNull(wire.MaxProseBytes)
		for _, e := range cp.Field("evidence").Array(-1, false) {
			c.Evidence = append(c.Evidence, e.Digest())
		}
		c.ManifestSha256 = cp.Field("manifestSha256").DigestOrNull()
		c.RecordedAt = cp.Field("recordedAt").Timestamp()
		rec.Completion = c
	}
	dd := r.Field("dueDate")
	if !dd.IsNull() {
		s := dd.String()
		if dd.Err() == nil {
			if _, err := wire.ParseDate(dd.Where(), s); err != nil {
				dd.Fail(wire.CodeMalformed, "%v", err)
			}
		}
		rec.DueDate = &s
	}
	rec.EstimateMinutes = r.Field("estimateMinutes").CountOrNull()
	sp := r.Field("supersedes")
	if !sp.IsNull() {
		id := sp.TicketID()
		rec.Supersedes = &id
	}
	sb := r.Field("supersededBy")
	if !sb.IsNull() {
		id := sb.TicketID()
		rec.SupersededBy = &id
	}
	rec.ShadowOverlay = r.Field("shadowOverlay").Bool()
	rec.CreatedAt = r.Field("createdAt").Timestamp()
	rec.UpdatedAt = r.Field("updatedAt").Timestamp()
	rec.UpdatedBy = r.Field("updatedBy").Label()
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := rec.validate(); err != nil {
		return nil, err
	}
	return rec, nil
}

// validate enforces the §3.1 field relationships that a closed type check
// cannot express. Every rule cites its clause.
func (rec *Record) validate() error {
	// §3.1 revision: starts "1"; acceptanceRevision starts "1" and bumps only
	// with a revision bump, so 1 ≤ acceptanceRevision ≤ revision.
	if rec.Revision.Int() < 1 {
		return wire.Errorf(wire.CodeMalformed, "/revision", "revision starts at 1")
	}
	if rec.AcceptanceRevision.Int() < 1 {
		return wire.Errorf(wire.CodeMalformed, "/acceptanceRevision", "acceptanceRevision starts at 1")
	}
	if rec.AcceptanceRevision.Int() > rec.Revision.Int() {
		return wire.Errorf(wire.CodeMalformed, "/acceptanceRevision", "acceptanceRevision %s exceeds revision %s", rec.AcceptanceRevision, rec.Revision)
	}
	// §3.1 previousRecordSha256: null at revision 1, a digest afterwards.
	if rec.Revision == "1" && rec.PreviousRecordSha256 != nil {
		return wire.Errorf(wire.CodeMalformed, "/previousRecordSha256", "must be null at revision 1")
	}
	if rec.Revision != "1" && rec.PreviousRecordSha256 == nil {
		return wire.Errorf(wire.CodeMalformed, "/previousRecordSha256", "must chain to the prior record after revision 1")
	}
	// §3.1 archivedFrom: null unless ARCHIVED.
	if (rec.Status == StatusArchived) != (rec.ArchivedFrom != nil) {
		return wire.Errorf(wire.CodeMalformed, "/archivedFrom", "archivedFrom is non-null iff status is ARCHIVED")
	}
	// §3.2 HELD iff holds non-empty, applied to the effective status (the
	// archived-from status for a tombstone).
	effective := rec.Status
	if rec.ArchivedFrom != nil {
		effective = *rec.ArchivedFrom
	}
	if (effective == StatusHeld) != (len(rec.Holds) > 0) {
		return wire.Errorf(wire.CodeMalformed, "/holds", "status HELD iff holds is non-empty (effective status %s, %d holds)", effective, len(rec.Holds))
	}
	// §3.1 completion: present iff the effective status is COMPLETED (a
	// tombstone archived from COMPLETED keeps it; REOPEN nulls it and the
	// prior completion lives in the chained prior record), so no reader keyed
	// on completion can report a non-completed ticket as completed or verified
	// (AS-06). MANUAL never carries a manifest; VERIFIED is the reducer's
	// output and names its manifest.
	if effective == StatusCompleted && rec.Completion == nil {
		return wire.Errorf(wire.CodeMalformed, "/completion", "status COMPLETED requires a completion record")
	}
	if effective != StatusCompleted && rec.Completion != nil {
		return wire.Errorf(wire.CodeMalformed, "/completion", "completion is non-null only while the effective status is COMPLETED (is %s)", effective)
	}
	if rec.Completion != nil {
		if rec.Completion.Kind == "MANUAL" && rec.Completion.ManifestSha256 != nil {
			return wire.Errorf(wire.CodeMalformed, "/completion/manifestSha256", "MANUAL completion never carries a manifest")
		}
		if rec.Completion.Kind == "VERIFIED" && rec.Completion.ManifestSha256 == nil {
			return wire.Errorf(wire.CodeMalformed, "/completion/manifestSha256", "VERIFIED completion must name its manifest")
		}
	}
	// §3.1 dependencies: gateId non-null iff GATE_PASSED; same queue; no
	// self-dependency; no duplicate edge.
	seen := map[string]bool{}
	for i, d := range rec.Dependencies {
		where := "/dependencies/" + idx(i)
		if (d.Obligation == "GATE_PASSED") != (d.GateID != nil) {
			return wire.Errorf(wire.CodeMalformed, where+"/gateId", "gateId is non-null iff obligation is GATE_PASSED")
		}
		if d.TicketID.QueueID() != rec.TicketID.QueueID() {
			return wire.Errorf(wire.CodeDependencyMissing, where+"/ticketId", "dependency %s is outside queue %s", d.TicketID.Raw, rec.TicketID.QueueID())
		}
		if d.TicketID.Raw == rec.TicketID.Raw {
			return wire.Errorf(wire.CodeCycle, where+"/ticketId", "ticket depends on itself")
		}
		key := d.TicketID.Raw + "\x00" + d.Obligation
		if d.GateID != nil {
			key += "\x00" + *d.GateID
		}
		if seen[key] {
			return wire.Errorf(wire.CodeDuplicateID, where, "duplicate dependency edge")
		}
		seen[key] = true
	}
	// §3.1 source: IMPORT names its source item; NATIVE has none.
	if rec.Source.Kind == "IMPORT" && rec.Source.SourceItemID == nil {
		return wire.Errorf(wire.CodeMalformed, "/source/sourceItemId", "IMPORT source must name sourceItemId")
	}
	if rec.Source.Kind == "NATIVE" && (rec.Source.SourceItemID != nil || rec.Source.SourceRevisionSha256 != nil) {
		return wire.Errorf(wire.CodeMalformed, "/source", "NATIVE source carries no sourceItemId or sourceRevisionSha256")
	}
	// §3.1 shadowOverlay: true only over an imported record.
	if rec.ShadowOverlay && rec.Source.Kind != "IMPORT" {
		return wire.Errorf(wire.CodeMalformed, "/shadowOverlay", "shadowOverlay is true only for source.kind IMPORT")
	}
	// §3.1 holds: holdId unique within the record.
	hseen := map[string]bool{}
	for i, h := range rec.Holds {
		if hseen[h.HoldID] {
			return wire.Errorf(wire.CodeDuplicateID, "/holds/"+idx(i)+"/holdId", "duplicate holdId %q", h.HoldID)
		}
		hseen[h.HoldID] = true
	}
	// §3.1 approvals: grantId unique.
	gseen := map[string]bool{}
	for i, a := range rec.Approvals {
		if gseen[a.GrantID] {
			return wire.Errorf(wire.CodeDuplicateID, "/approvals/"+idx(i)+"/grantId", "duplicate grantId %q", a.GrantID)
		}
		gseen[a.GrantID] = true
	}
	// §3.1 supersession is a reference to another ticket in the same queue.
	for name, ref := range map[string]*wire.TicketID{"supersedes": rec.Supersedes, "supersededBy": rec.SupersededBy} {
		if ref == nil {
			continue
		}
		if ref.Raw == rec.TicketID.Raw {
			return wire.Errorf(wire.CodeMalformed, "/"+name, "a ticket cannot supersede itself")
		}
		if ref.QueueID() != rec.TicketID.QueueID() {
			return wire.Errorf(wire.CodeMalformed, "/"+name, "%s is outside queue %s", ref.Raw, rec.TicketID.QueueID())
		}
	}
	return nil
}

func idx(i int) string { return string(wire.CountOf(int64(i))) }

// Value renders the record as a canonical wire value (every key present).
func (rec *Record) Value() wire.Value {
	o := wire.NewObject()
	o.Set("profile", wire.String(Profile))
	o.Set("ticketId", wire.String(rec.TicketID.Raw))
	o.Set("revision", wire.String(string(rec.Revision)))
	o.Set("acceptanceRevision", wire.String(string(rec.AcceptanceRevision)))
	o.Set("previousRecordSha256", digestOrNull(rec.PreviousRecordSha256))
	o.Set("status", wire.String(rec.Status))
	o.Set("archivedFrom", wire.StringOrNull(rec.ArchivedFrom))
	o.Set("title", wire.String(rec.Title))
	o.Set("body", wire.StringOrNull(rec.Body))
	o.Set("kind", wire.String(rec.Kind))
	o.Set("owner", wire.StringOrNull(rec.Owner))
	o.Set("milestone", wire.StringOrNull(rec.Milestone))
	o.Set("priority", wire.String(rec.Priority))
	o.Set("order", wire.String(string(rec.Order)))
	o.Set("labels", wire.Strings(rec.Labels))
	deps := make([]wire.Value, 0, len(rec.Dependencies))
	for _, d := range rec.Dependencies {
		do := wire.NewObject()
		do.Set("ticketId", wire.String(d.TicketID.Raw))
		do.Set("obligation", wire.String(d.Obligation))
		do.Set("gateId", wire.StringOrNull(d.GateID))
		deps = append(deps, wire.ObjectValue(do))
	}
	o.Set("dependencies", wire.Array(deps...))
	o.Set("acceptanceCriteria", wire.Strings(rec.AcceptanceCriteria))
	o.Set("requirementRefs", wire.Strings(rec.RequirementRefs))
	so := wire.NewObject()
	so.Set("kind", wire.String(rec.Source.Kind))
	so.Set("sourceQueueId", wire.String(rec.Source.SourceQueueID))
	so.Set("sourceItemId", wire.StringOrNull(rec.Source.SourceItemID))
	so.Set("sourceRevisionSha256", digestOrNull(rec.Source.SourceRevisionSha256))
	o.Set("source", wire.ObjectValue(so))
	eo := wire.NewObject()
	eo.Set("coverage", wire.String(rec.Effects.Coverage))
	eo.Set("touchPaths", wire.Strings(rec.Effects.TouchPaths))
	res := make([]wire.Value, 0, len(rec.Effects.Resources))
	for _, r := range rec.Effects.Resources {
		ro := wire.NewObject()
		ro.Set("class", wire.String(r.Class))
		ro.Set("key", wire.String(r.Key))
		res = append(res, wire.ObjectValue(ro))
	}
	eo.Set("resources", wire.Array(res...))
	eo.Set("externalUnbounded", wire.Bool(rec.Effects.ExternalUnbounded))
	o.Set("effects", wire.ObjectValue(eo))
	o.Set("capabilities", wire.Strings(rec.Capabilities))
	o.Set("requiredGates", wire.Strings(rec.RequiredGates))
	holds := make([]wire.Value, 0, len(rec.Holds))
	for _, h := range rec.Holds {
		ho := wire.NewObject()
		ho.Set("holdId", wire.String(h.HoldID))
		ho.Set("actor", wire.String(h.Actor))
		ho.Set("reason", wire.String(h.Reason))
		ho.Set("placedAt", wire.String(string(h.PlacedAt)))
		holds = append(holds, wire.ObjectValue(ho))
	}
	o.Set("holds", wire.Array(holds...))
	o.Set("executionClass", wire.String(rec.ExecutionClass))
	aps := make([]wire.Value, 0, len(rec.Approvals))
	for _, a := range rec.Approvals {
		ao := wire.NewObject()
		ao.Set("grantId", wire.String(a.GrantID))
		ao.Set("actor", wire.String(a.Actor))
		ao.Set("operation", wire.String(a.Operation))
		ao.Set("targetRevision", wire.String(string(a.TargetRevision)))
		ao.Set("scope", wire.Strings(a.Scope))
		ao.Set("revoked", wire.Bool(a.Revoked))
		ao.Set("grantedAt", wire.String(string(a.GrantedAt)))
		aps = append(aps, wire.ObjectValue(ao))
	}
	o.Set("approvals", wire.Array(aps...))
	if rec.Completion == nil {
		o.Set("completion", wire.Null())
	} else {
		co := wire.NewObject()
		co.Set("kind", wire.String(rec.Completion.Kind))
		co.Set("actor", wire.String(rec.Completion.Actor))
		co.Set("reason", wire.StringOrNull(rec.Completion.Reason))
		ev := make([]string, len(rec.Completion.Evidence))
		for i, d := range rec.Completion.Evidence {
			ev[i] = string(d)
		}
		co.Set("evidence", wire.Strings(ev))
		co.Set("manifestSha256", digestOrNull(rec.Completion.ManifestSha256))
		co.Set("recordedAt", wire.String(string(rec.Completion.RecordedAt)))
		o.Set("completion", wire.ObjectValue(co))
	}
	o.Set("dueDate", wire.StringOrNull(rec.DueDate))
	if rec.EstimateMinutes == nil {
		o.Set("estimateMinutes", wire.Null())
	} else {
		o.Set("estimateMinutes", wire.String(string(*rec.EstimateMinutes)))
	}
	o.Set("supersedes", ticketOrNull(rec.Supersedes))
	o.Set("supersededBy", ticketOrNull(rec.SupersededBy))
	o.Set("shadowOverlay", wire.Bool(rec.ShadowOverlay))
	o.Set("createdAt", wire.String(string(rec.CreatedAt)))
	o.Set("updatedAt", wire.String(string(rec.UpdatedAt)))
	o.Set("updatedBy", wire.String(rec.UpdatedBy))
	return wire.ObjectValue(o)
}

// Encode returns the record file bytes (canonical body plus LF).
func (rec *Record) Encode() []byte {
	return wire.EncodeFile(rec.Value())
}

// FileDigest is the chain digest of the record file (§2).
func (rec *Record) FileDigest() wire.Digest {
	return wire.Sum(rec.Encode())
}

// EffectiveStatus is the status a tombstone would return to on RESTORE, or
// the status itself for a live record.
func (rec *Record) EffectiveStatus() string {
	if rec.ArchivedFrom != nil {
		return *rec.ArchivedFrom
	}
	return rec.Status
}

func digestOrNull(d *wire.Digest) wire.Value {
	if d == nil {
		return wire.Null()
	}
	return wire.String(string(*d))
}

func ticketOrNull(t *wire.TicketID) wire.Value {
	if t == nil {
		return wire.Null()
	}
	return wire.String(t.Raw)
}
