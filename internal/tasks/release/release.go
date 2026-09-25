// Package release implements the pure taskman-release/0 release-control
// model. It performs no I/O and grants no publication or execution authority.
package release

import (
	"sort"

	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const Profile = "taskman-release/0"
const AttestationProfile = "taskman-release-attestation/0"

const (
	Blocked       = "BLOCKED"
	Unknown       = "UNKNOWN"
	ReadyAttested = "READY_ATTESTED"
	Manual        = "MANUAL_ATTESTATION"
	External      = "EXTERNAL_ATTESTATION"
)

type TicketBinding struct {
	TicketID           wire.TicketID
	RecordSha256       wire.Digest
	AcceptanceRevision wire.Count
}

type PredecessorBinding struct {
	ReleaseID       string
	PromotionSha256 wire.Digest
}

type Candidate struct {
	RepositoryIdentity string
	HeadCommit         string
	HeadTree           string
	SourceSha256       wire.Digest
	DefinitionSha256   wire.Digest
	PolicySha256       wire.Digest
	Tickets            []TicketBinding
	Predecessors       []PredecessorBinding
}

type Attestation struct {
	AttestationID   string
	CandidateSha256 wire.Digest
	GateID          string
	Provenance      string
	Actor           string
	RecordedAt      wire.Timestamp
	Result          string
	Criteria        []wire.Count
	Evidence        []wire.Digest
	SourceIdentity  string
}

type Promotion struct {
	CandidateSha256    wire.Digest
	AttestationSha256s []wire.Digest
	Predecessors       []PredecessorBinding
	Actor              string
	RecordedAt         wire.Timestamp
}

type Record struct {
	QueueID            wire.QueueID
	ReleaseID          string
	Revision           wire.Count
	PreviousSha256     *wire.Digest
	Version            string
	Title              string
	PredecessorIDs     []string
	TicketIDs          []wire.TicketID
	AcceptanceCriteria []string
	RequiredGates      []string
	Candidate          *Candidate
	Attestations       []Attestation
	Promotion          *Promotion
}

func Decode(data []byte) (*Record, error) {
	if len(data) > wire.MaxReleaseFileBytes {
		return nil, wire.Errorf(wire.CodeLimitExceeded, "/", "release record larger than %d bytes", wire.MaxReleaseFileBytes)
	}
	v, err := wire.Parse(data)
	if err != nil {
		return nil, err
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "queueId", "releaseId", "revision", "previousRecordSha256", "version", "title", "predecessorReleaseIds", "ticketIds", "acceptanceCriteria", "requiredGates", "candidate", "attestations", "promotion")
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := wire.CheckProfile("/profile", r.Field("profile").String(), Profile); err != nil {
		return nil, err
	}
	x := &Record{}
	x.QueueID = r.Field("queueId").QueueID()
	x.ReleaseID = r.Field("releaseId").Label()
	x.Revision = r.Field("revision").Count()
	x.PreviousSha256 = r.Field("previousRecordSha256").DigestOrNull()
	x.Version = r.Field("version").Label()
	x.Title = r.Field("title").Prose(1, wire.MaxTitleBytes)
	x.PredecessorIDs = r.Field("predecessorReleaseIds").Strings(wire.MaxReleasesPerQueue, false, (*wire.Reader).Label)
	for _, q := range r.Field("ticketIds").Array(wire.MaxTicketsPerQueue, false) {
		x.TicketIDs = append(x.TicketIDs, q.TicketID())
	}
	x.AcceptanceCriteria = r.Field("acceptanceCriteria").Strings(wire.MaxAcceptanceCriteria, true, func(q *wire.Reader) string { return q.Prose(0, wire.MaxCriterionBytes) })
	x.RequiredGates = r.Field("requiredGates").Strings(wire.MaxRequiredGates, false, (*wire.Reader).Label)
	if c := r.Field("candidate"); !c.IsNull() {
		x.Candidate = decodeCandidate(c)
	}
	for _, a := range r.Field("attestations").Array(wire.MaxJSONArrayElements, false) {
		x.Attestations = append(x.Attestations, decodeAttestation(a))
	}
	if p := r.Field("promotion"); !p.IsNull() {
		x.Promotion = decodePromotion(p)
	}
	if err := r.Err(); err != nil {
		return nil, err
	}
	if err := validate(x); err != nil {
		return nil, err
	}
	return x, nil
}

func decodeCandidate(r *wire.Reader) *Candidate {
	r.Closed("repositoryIdentity", "headCommit", "headTree", "sourceSha256", "definitionSha256", "policySha256", "tickets", "predecessors")
	c := &Candidate{RepositoryIdentity: r.Field("repositoryIdentity").Identifier()}
	c.HeadCommit, _ = wire.ParseOID(r.Field("headCommit").Where(), r.Field("headCommit").String())
	c.HeadTree, _ = wire.ParseOID(r.Field("headTree").Where(), r.Field("headTree").String())
	c.SourceSha256 = r.Field("sourceSha256").Digest()
	c.DefinitionSha256 = r.Field("definitionSha256").Digest()
	c.PolicySha256 = r.Field("policySha256").Digest()
	for _, q := range r.Field("tickets").Array(wire.MaxTicketsPerQueue, true) {
		q.Closed("ticketId", "recordSha256", "acceptanceRevision")
		c.Tickets = append(c.Tickets, TicketBinding{q.Field("ticketId").TicketID(), q.Field("recordSha256").Digest(), q.Field("acceptanceRevision").Count()})
	}
	for _, q := range r.Field("predecessors").Array(wire.MaxReleasesPerQueue, true) {
		c.Predecessors = append(c.Predecessors, decodePredecessor(q))
	}
	return c
}

func decodePredecessor(r *wire.Reader) PredecessorBinding {
	r.Closed("releaseId", "promotionSha256")
	return PredecessorBinding{r.Field("releaseId").Label(), r.Field("promotionSha256").Digest()}
}

func decodeAttestation(r *wire.Reader) Attestation {
	r.Closed("profile", "attestationId", "candidateSha256", "gateId", "provenance", "actor", "recordedAt", "result", "criteria", "evidence", "sourceIdentity")
	if r.Field("profile").String() != AttestationProfile {
		r.Fail(wire.CodeMalformed, "invalid attestation profile")
	}
	a := Attestation{AttestationID: r.Field("attestationId").Label(), CandidateSha256: r.Field("candidateSha256").Digest(), GateID: r.Field("gateId").Label(), Provenance: r.Field("provenance").Enum(Manual, External), Actor: r.Field("actor").Label(), RecordedAt: r.Field("recordedAt").Timestamp(), Result: r.Field("result").Enum("PASS", "FAIL"), SourceIdentity: r.Field("sourceIdentity").Identifier()}
	for _, q := range r.Field("criteria").Array(wire.MaxAcceptanceCriteria, false) {
		a.Criteria = append(a.Criteria, q.Count())
	}
	for _, q := range r.Field("evidence").Array(wire.MaxJSONArrayElements, false) {
		a.Evidence = append(a.Evidence, q.Digest())
	}
	return a
}

func decodePromotion(r *wire.Reader) *Promotion {
	r.Closed("candidateSha256", "attestationSha256s", "predecessors", "actor", "recordedAt")
	p := &Promotion{CandidateSha256: r.Field("candidateSha256").Digest(), Actor: r.Field("actor").Label(), RecordedAt: r.Field("recordedAt").Timestamp()}
	for _, q := range r.Field("attestationSha256s").Array(wire.MaxJSONArrayElements, false) {
		p.AttestationSha256s = append(p.AttestationSha256s, q.Digest())
	}
	for _, q := range r.Field("predecessors").Array(wire.MaxReleasesPerQueue, true) {
		p.Predecessors = append(p.Predecessors, decodePredecessor(q))
	}
	return p
}

func validate(r *Record) error {
	if r.Revision.Int() < 1 {
		return wire.Errorf(wire.CodeMalformed, "revision", "release revision must be positive")
	}
	if err := sortedStrings("predecessorReleaseIds", r.PredecessorIDs); err != nil {
		return err
	}
	ids := make([]string, len(r.TicketIDs))
	for i := range r.TicketIDs {
		ids[i] = r.TicketIDs[i].Raw
	}
	if err := sortedStrings("ticketIds", ids); err != nil {
		return err
	}
	if err := sortedStrings("requiredGates", r.RequiredGates); err != nil {
		return err
	}
	if r.Promotion != nil && r.Candidate == nil {
		return wire.Errorf(wire.CodeMalformed, "promotion", "promotion requires candidate")
	}
	if r.Candidate != nil {
		if err := validateCandidate(r, r.Candidate); err != nil {
			return err
		}
	}
	seen := map[string]bool{}
	for _, a := range r.Attestations {
		if seen[a.AttestationID] {
			return wire.Errorf(wire.CodeMalformed, "attestations", "duplicate attestationId %q", a.AttestationID)
		}
		seen[a.AttestationID] = true
		if len(a.Evidence) == 0 || a.SourceIdentity == "" {
			return wire.Errorf(wire.CodeMalformed, "attestations", "attestation evidence and source identity are required")
		}
		criteria := make([]string, len(a.Criteria))
		for i := range a.Criteria {
			criteria[i] = string(a.Criteria[i])
		}
		if err := sortedStrings("attestations/criteria", criteria); err != nil {
			return err
		}
		evidence := make([]string, len(a.Evidence))
		for i := range a.Evidence {
			evidence[i] = string(a.Evidence[i])
		}
		if err := sortedStrings("attestations/evidence", evidence); err != nil {
			return err
		}
		for _, i := range a.Criteria {
			if i.Int() >= int64(len(r.AcceptanceCriteria)) {
				return wire.Errorf(wire.CodeMalformed, "attestations", "criterion index %s is outside the release definition", i)
			}
		}
	}
	if r.Promotion != nil {
		if r.Promotion.CandidateSha256 != CandidateDigest(r.Candidate) {
			return wire.Errorf(wire.CodeMalformed, "promotion", "promotion does not bind the current candidate")
		}
		if !equalPredecessors(r.Promotion.Predecessors, r.Candidate.Predecessors) {
			return wire.Errorf(wire.CodeMalformed, "promotion", "promotion predecessor bindings differ from the candidate")
		}
		attestations := map[wire.Digest]bool{}
		for _, a := range r.Attestations {
			if a.CandidateSha256 == r.Promotion.CandidateSha256 && a.Result == "PASS" {
				attestations[AttestationDigest(a)] = true
			}
		}
		promotionDigests := make([]string, len(r.Promotion.AttestationSha256s))
		for i, d := range r.Promotion.AttestationSha256s {
			promotionDigests[i] = string(d)
			if !attestations[d] {
				return wire.Errorf(wire.CodeMalformed, "promotion", "promotion names a missing, failed, or stale attestation")
			}
		}
		if len(promotionDigests) == 0 {
			return wire.Errorf(wire.CodeMalformed, "promotion", "promotion must bind passing attestations")
		}
		if err := sortedStrings("promotion/attestationSha256s", promotionDigests); err != nil {
			return err
		}
	}
	return nil
}

// ValidateGraph checks the complete queue-scoped release DAG. It performs no
// I/O and does not treat graph validity as evidence that a release is ready.
func ValidateGraph(records []*Record) error {
	byID := map[string]*Record{}
	queue := ""
	for _, r := range records {
		if r == nil {
			return wire.Errorf(wire.CodeMalformed, "releases", "nil release record")
		}
		if queue == "" {
			queue = r.QueueID.Raw
		}
		if r.QueueID.Raw != queue {
			return wire.Errorf(wire.CodeMalformed, "releases", "release records span queues")
		}
		if byID[r.ReleaseID] != nil {
			return wire.Errorf(wire.CodeMalformed, "releases", "duplicate releaseId %q", r.ReleaseID)
		}
		byID[r.ReleaseID] = r
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(id string) error {
		if state[id] == 1 {
			return wire.Errorf(wire.CodeMalformed, "releases", "release predecessor cycle at %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, predecessor := range byID[id].PredecessorIDs {
			if byID[predecessor] == nil {
				return wire.Errorf(wire.CodeMalformed, "releases", "unknown predecessor %q", predecessor)
			}
			if err := visit(predecessor); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for id := range byID {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func validateCandidate(r *Record, c *Candidate) error {
	if len(c.Tickets) != len(r.TicketIDs) || len(c.Predecessors) != len(r.PredecessorIDs) {
		return wire.Errorf(wire.CodeMalformed, "candidate", "candidate scope differs from the release definition")
	}
	for i := range r.TicketIDs {
		if c.Tickets[i].TicketID != r.TicketIDs[i] {
			return wire.Errorf(wire.CodeMalformed, "candidate/tickets", "candidate ticket order differs from the release definition")
		}
	}
	for i := range r.PredecessorIDs {
		if c.Predecessors[i].ReleaseID != r.PredecessorIDs[i] {
			return wire.Errorf(wire.CodeMalformed, "candidate/predecessors", "candidate predecessor order differs from the release definition")
		}
	}
	return nil
}

func equalPredecessors(a, b []PredecessorBinding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedStrings(where string, xs []string) error {
	for i := 1; i < len(xs); i++ {
		if xs[i] <= xs[i-1] {
			return wire.Errorf(wire.CodeMalformed, where, "entries must be sorted and unique")
		}
	}
	return nil
}

func Encode(r *Record) []byte {
	o := wire.NewObject().Set("profile", wire.String(Profile)).Set("queueId", wire.String(r.QueueID.Raw)).Set("releaseId", wire.String(r.ReleaseID)).Set("revision", wire.String(string(r.Revision))).Set("previousRecordSha256", digestOrNull(r.PreviousSha256)).Set("version", wire.String(r.Version)).Set("title", wire.String(r.Title)).Set("predecessorReleaseIds", wire.Strings(r.PredecessorIDs)).Set("acceptanceCriteria", wire.Strings(r.AcceptanceCriteria)).Set("requiredGates", wire.Strings(r.RequiredGates))
	ids := make([]string, len(r.TicketIDs))
	for i := range r.TicketIDs {
		ids[i] = r.TicketIDs[i].Raw
	}
	o.Set("ticketIds", wire.Strings(ids))
	if r.Candidate == nil {
		o.Set("candidate", wire.Null())
	} else {
		o.Set("candidate", candidateValue(r.Candidate))
	}
	avs := make([]wire.Value, len(r.Attestations))
	for i := range r.Attestations {
		avs[i] = attestationValue(r.Attestations[i])
	}
	o.Set("attestations", wire.Array(avs...))
	if r.Promotion == nil {
		o.Set("promotion", wire.Null())
	} else {
		o.Set("promotion", promotionValue(r.Promotion))
	}
	return wire.EncodeFile(wire.ObjectValue(o))
}

func digestOrNull(d *wire.Digest) wire.Value {
	if d == nil {
		return wire.Null()
	}
	return wire.String(string(*d))
}
func predecessorValues(ps []PredecessorBinding) wire.Value {
	vs := make([]wire.Value, len(ps))
	for i, p := range ps {
		vs[i] = wire.ObjectValue(wire.NewObject().Set("releaseId", wire.String(p.ReleaseID)).Set("promotionSha256", wire.String(string(p.PromotionSha256))))
	}
	return wire.Array(vs...)
}
func candidateValue(c *Candidate) wire.Value {
	ts := make([]wire.Value, len(c.Tickets))
	for i, t := range c.Tickets {
		ts[i] = wire.ObjectValue(wire.NewObject().Set("ticketId", wire.String(t.TicketID.Raw)).Set("recordSha256", wire.String(string(t.RecordSha256))).Set("acceptanceRevision", wire.String(string(t.AcceptanceRevision))))
	}
	return wire.ObjectValue(wire.NewObject().Set("repositoryIdentity", wire.String(c.RepositoryIdentity)).Set("headCommit", wire.String(c.HeadCommit)).Set("headTree", wire.String(c.HeadTree)).Set("sourceSha256", wire.String(string(c.SourceSha256))).Set("definitionSha256", wire.String(string(c.DefinitionSha256))).Set("policySha256", wire.String(string(c.PolicySha256))).Set("tickets", wire.Array(ts...)).Set("predecessors", predecessorValues(c.Predecessors)))
}
func attestationValue(a Attestation) wire.Value {
	cs := make([]string, len(a.Criteria))
	for i, c := range a.Criteria {
		cs[i] = string(c)
	}
	es := make([]string, len(a.Evidence))
	for i, e := range a.Evidence {
		es[i] = string(e)
	}
	return wire.ObjectValue(wire.NewObject().Set("profile", wire.String(AttestationProfile)).Set("attestationId", wire.String(a.AttestationID)).Set("candidateSha256", wire.String(string(a.CandidateSha256))).Set("gateId", wire.String(a.GateID)).Set("provenance", wire.String(a.Provenance)).Set("actor", wire.String(a.Actor)).Set("recordedAt", wire.String(string(a.RecordedAt))).Set("result", wire.String(a.Result)).Set("criteria", wire.Strings(cs)).Set("evidence", wire.Strings(es)).Set("sourceIdentity", wire.String(a.SourceIdentity)))
}
func promotionValue(p *Promotion) wire.Value {
	ds := make([]string, len(p.AttestationSha256s))
	for i, d := range p.AttestationSha256s {
		ds[i] = string(d)
	}
	return wire.ObjectValue(wire.NewObject().Set("candidateSha256", wire.String(string(p.CandidateSha256))).Set("attestationSha256s", wire.Strings(ds)).Set("predecessors", predecessorValues(p.Predecessors)).Set("actor", wire.String(p.Actor)).Set("recordedAt", wire.String(string(p.RecordedAt))))
}

func DefinitionDigest(r *Record) wire.Digest {
	x := *r
	x.Candidate = nil
	x.Attestations = nil
	x.Promotion = nil
	x.Revision = "0"
	x.PreviousSha256 = nil
	return wire.Sum(Encode(&x))
}
func CandidateDigest(c *Candidate) wire.Digest { return wire.Sum(wire.EncodeFile(candidateValue(c))) }
func AttestationDigest(a Attestation) wire.Digest {
	return wire.Sum(wire.EncodeFile(attestationValue(a)))
}
func PromotionDigest(p *Promotion) wire.Digest { return wire.Sum(wire.EncodeFile(promotionValue(p))) }

type Observation struct {
	HeadCommit                 string
	SourceSha256, PolicySha256 wire.Digest
	Tickets                    map[string]*ticket.Record
	TicketDigests              map[string]wire.Digest
	PredecessorPromotions      map[string]wire.Digest
}
type Readiness struct {
	State   string
	Missing []string
}

type Gate struct {
	GateID, Kind string
	Required     bool
}

func Assess(r *Record, gates []Gate, o Observation) Readiness {
	missing := []string{}
	if r.Candidate == nil {
		return Readiness{Blocked, []string{"candidate"}}
	}
	c := r.Candidate
	cd := CandidateDigest(c)
	if err := validateCandidate(r, c); err != nil {
		missing = append(missing, "candidate-scope")
	}
	if c.DefinitionSha256 != DefinitionDigest(r) {
		missing = append(missing, "definition")
	}
	if c.PolicySha256 != o.PolicySha256 || c.HeadCommit != o.HeadCommit || c.SourceSha256 != o.SourceSha256 {
		missing = append(missing, "candidate-source-or-policy")
	}
	for _, b := range c.Tickets {
		t := o.Tickets[b.TicketID.Raw]
		if t == nil || t.Status != ticket.StatusCompleted || o.TicketDigests[b.TicketID.Raw] != b.RecordSha256 || t.AcceptanceRevision != b.AcceptanceRevision {
			missing = append(missing, "ticket:"+b.TicketID.Raw)
		}
	}
	for _, b := range c.Predecessors {
		if o.PredecessorPromotions[b.ReleaseID] != b.PromotionSha256 {
			missing = append(missing, "predecessor:"+b.ReleaseID)
		}
	}
	defined := map[string]string{}
	required := map[string]string{}
	for _, g := range gates {
		defined[g.GateID] = g.Kind
		if g.Required {
			required[g.GateID] = g.Kind
		}
	}
	for _, id := range r.RequiredGates {
		for _, g := range gates {
			if g.GateID == id {
				required[id] = g.Kind
			}
		}
		if _, ok := required[id]; !ok {
			missing = append(missing, "gate-definition:"+id)
		}
	}
	covered := make([]bool, len(r.AcceptanceCriteria))
	satisfied := map[string]bool{}
	for _, a := range r.Attestations {
		if a.CandidateSha256 != cd || a.Result != "PASS" {
			continue
		}
		kind, known := defined[a.GateID]
		_, requiredGate := required[a.GateID]
		compatible := a.Provenance == Manual && kind == "MANUAL" || a.Provenance == External && externalKind(kind)
		if requiredGate && compatible {
			satisfied[a.GateID] = true
		}
		if !known || !compatible {
			continue
		}
		for _, i := range a.Criteria {
			if i.Int() >= 0 && i.Int() < int64(len(covered)) {
				covered[i.Int()] = true
			}
		}
	}
	for id := range required {
		if !satisfied[id] {
			missing = append(missing, "gate:"+id)
		}
	}
	for i, ok := range covered {
		if !ok {
			missing = append(missing, "criterion:"+string(wire.CountOf(int64(i))))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return Readiness{Blocked, missing}
	}
	return Readiness{ReadyAttested, nil}
}

func externalKind(kind string) bool {
	switch kind {
	case "COMMAND", "REVIEW", "DOCS", "EXTERNAL":
		return true
	}
	return false
}

func passingAttestation(a Attestation, candidate wire.Digest, gates []Gate) bool {
	if a.CandidateSha256 != candidate || a.Result != "PASS" {
		return false
	}
	for _, g := range gates {
		if g.GateID == a.GateID {
			return a.Provenance == Manual && g.Kind == "MANUAL" || a.Provenance == External && externalKind(g.Kind)
		}
	}
	return false
}
