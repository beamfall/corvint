package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

const StableSpec = "cem/1.0"

// RetiredSpecFix is the recovery a retired cem/0.x map names (CEM-V1-200):
// the committed maps were rewritten in place, so a branch catches up by
// merging, never by rebasing or force-pushing.
const RetiredSpecFix = "committed 0.x maps were migrated in place by the migration commits (internal/dogfoodflow/migration.go); merge origin/main into the branch, git rm the branch's own 0.x seal under .corvint/changes/, then rerun dogfood-change, dogfood-check and dogfood-seal with BASE=<the origin/main commit merged>"

// RetiredSpec reports a cem/0.1, cem/0.2 or cem/0.3 spec. cem/1.0 is the only
// stable wire, so the stable reader refuses these with unsupported-spec and
// RetiredSpecFix (CEM-V1-200).
func RetiredSpec(spec string) bool {
	return spec == "cem/0.1" || spec == "cem/0.2" || spec == "cem/0.3"
}

// DeclaresStable reports whether data is one bounded JSON object whose spec is
// exactly StableSpec. It selects a reader only; it validates nothing else, so a
// caller still verifies the document through ParseStable (CEM-V1-019).
func DeclaresStable(data []byte) bool {
	if len(data) > MaxMapBytes {
		return false
	}
	root, err := Parse(data)
	if err != nil || root.Kind != KindObject {
		return false
	}
	spec, present := root.Obj.Get("spec")
	return present && spec.Kind == KindString && spec.Str == StableSpec
}

// StableMap owns the full stable document. Change retains the actual stable
// profile; no legacy/candidate admission or profile translation is involved.
// StableChange is intentionally not a legacy Map: reduced serializers cannot
// consume the stable document through an implicit shared field type.
type StableChange Map

type StableMap struct {
	Change    StableChange
	Criteria  []StableCriterion
	Receipts  []StableReceipt
	Links     []StableLink
	Artifacts []StableArtifact
	// Provenance and Role are the optional CEM-V1-093/095 members; nil and ""
	// mean absent. Neither carries authority.
	Provenance *StableProvenance
	Role       string
	original   []byte
}

// StableCriterion has three closed forms. The plain form (CEM-V1-083) is a Git
// blob span criterion: id, criterionIndex, criterionSha256 and evidenceId. The
// optional Corvint Tasks extension is the eight Tasks members, wholly present
// (and then evidenceId is absent) or wholly absent. The external form
// (CEM-V1-330) binds a blob span in a separately supplied repository:
// externalRepository, externalBlobOid and externalSpan beside id,
// criterionIndex and criterionSha256. Empty members are omitted, so the plain
// and Tasks forms encode and identify exactly as before.
type StableCriterion struct {
	ID                         string              `json:"id"`
	TicketID                   string              `json:"ticketId,omitempty"`
	AcceptanceRevision         string              `json:"acceptanceRevision,omitempty"`
	CriterionIndex             int64               `json:"criterionIndex"`
	AcceptanceSha256           string              `json:"acceptanceSha256,omitempty"`
	CriterionSha256            string              `json:"criterionSha256"`
	CaptureSha256              string              `json:"captureSha256,omitempty"`
	VerificationSha256         string              `json:"verificationSha256,omitempty"`
	ClaimTicketSha256          string              `json:"claimTicketSha256,omitempty"`
	SnapshotHeadReceiptSha256  string              `json:"snapshotHeadReceiptSha256,omitempty"`
	SnapshotHeadArtifactSha256 string              `json:"snapshotHeadArtifactSha256,omitempty"`
	EvidenceID                 string              `json:"evidenceId,omitempty"`
	ExternalRepository         string              `json:"externalRepository,omitempty"`
	ExternalBlobOid            string              `json:"externalBlobOid,omitempty"`
	ExternalSpan               *StableExternalSpan `json:"externalSpan,omitempty"`
}

// StableExternalSpan is the zero-based half-open byte range of an external
// criterion blob (CEM-V1-330).
type StableExternalSpan struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// Plain reports whether the criterion uses the provider-neutral form.
func (c StableCriterion) Plain() bool { return c.EvidenceID != "" }

// External reports whether the criterion uses the external form (CEM-V1-330).
func (c StableCriterion) External() bool { return c.ExternalRepository != "" }

// Tasks reports whether the criterion uses the Corvint Tasks extension form.
func (c StableCriterion) Tasks() bool { return c.TicketID != "" }

// StableExternalCriterionKeys selects the external criterion form (CEM-V1-330).
var StableExternalCriterionKeys = []string{"externalRepository", "externalBlobOid", "externalSpan"}

// IsStableExternalRepository reports whether name is a valid external
// repository name: an ASCII alphanumeric first byte, then at most 63
// alphanumeric, period, underscore or hyphen bytes (CEM-V1-330).
func IsStableExternalRepository(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		alnum := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !alnum && (i == 0 || c != '.' && c != '_' && c != '-') {
			return false
		}
	}
	return true
}

// StableTasksCriterionKeys is the optional Tasks extension member set.
var StableTasksCriterionKeys = []string{"ticketId", "acceptanceRevision", "acceptanceSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256", "snapshotHeadArtifactSha256"}

// stableCriterionKeys selects the closed key set: any Tasks member selects the
// whole Tasks form, so a partial extension fails as a missing member and a
// plain or external member beside it fails as an unknown member. Otherwise
// any external member selects the whole external form (CEM-V1-330), where
// evidenceId is unknown; otherwise the plain form applies.
func stableCriterionKeys(o *Object) []string {
	for _, k := range StableTasksCriterionKeys {
		if _, ok := o.Get(k); ok {
			return []string{"id", "ticketId", "acceptanceRevision", "criterionIndex", "acceptanceSha256", "criterionSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256", "snapshotHeadArtifactSha256"}
		}
	}
	for _, k := range StableExternalCriterionKeys {
		if _, ok := o.Get(k); ok {
			return []string{"id", "criterionIndex", "criterionSha256", "externalRepository", "externalBlobOid", "externalSpan"}
		}
	}
	return []string{"id", "criterionIndex", "criterionSha256", "evidenceId"}
}

// StableRunnerReceiptProfile labels a runner record whose receipt a producer
// decoded and admitted as a corvint-test-runner receipt (CEM-V1-280).
// StableOpaqueReceiptProfile labels one whose receipt bytes no producer
// admitted, so a reader cannot mistake an opaque file for a runner receipt
// (CEM-V1-310). A reader accepts both; neither raises a result axis.
const (
	StableRunnerReceiptProfile = "corvint-test-runner-receipt/0"
	StableOpaqueReceiptProfile = "opaque/0"
)

type StableReceipt struct {
	Sha256             string               `json:"sha256"`
	Profile            string               `json:"profile"`
	PlanSha256         string               `json:"planSha256"`
	SourceGitBinding   string               `json:"sourceGitBinding"`
	ExecutionAuthority string               `json:"executionAuthority"`
	DependencyClosure  string               `json:"dependencyClosure"`
	Authentication     StableAuthentication `json:"authentication"`
}
type StableLink struct {
	CriterionID          string   `json:"criterionId"`
	HunkIDs              []string `json:"hunkIds"`
	EvidenceIDs          []string `json:"evidenceIds"`
	RunnerReceiptSha256s []string `json:"runnerReceiptSha256s"`
}
type StableArtifact struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Sha256 string `json:"sha256"`
}

func (d *StableMap) OriginalBytes() []byte { return append([]byte(nil), d.original...) }

// ParseStable returns a nonnil document alongside a graph error only after the
// entire closed wire validated. Callers may then report completed wire checks,
// but must not publish its references/hunks as graph-validated output.
func ParseStable(raw []byte) (*StableMap, error) {
	if len(raw) > MaxMapBytes {
		return nil, cemcode.New(cemcode.MapUnavailable, "stable map bound")
	}
	v, e := Parse(raw)
	if e != nil {
		return nil, e
	}
	if !stableDepth(v, 1) {
		return nil, cemcode.New(cemcode.InvalidJSON, "stable nesting bound")
	}
	if v.Kind != KindObject {
		return nil, fieldError("stable root must be object")
	}
	spec, _ := v.Obj.Get("spec")
	if spec.Kind == KindString && RetiredSpec(spec.Str) {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "%s is retired; %s is the only stable wire; fix: %s", spec.Str, StableSpec, RetiredSpecFix)
	}
	if spec.Kind != KindString || spec.Str != StableSpec {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "stable profile required")
	}
	if e = requireStableRootKeys(v.Obj); e != nil {
		return nil, e
	}
	d := &StableMap{Change: StableChange{Spec: StableSpec}, Criteria: []StableCriterion{}, Receipts: []StableReceipt{}, Links: []StableLink{}, Artifacts: []StableArtifact{}, original: append([]byte(nil), raw...)}
	change := Map{Spec: StableSpec}
	if e = validateCommonCapabilities(v.Obj, &change, true, true); e != nil {
		return nil, stableCommonWireError(e)
	}
	d.Change = StableChange(change)
	if len(d.Change.Hunks) == 0 {
		return nil, fieldError("stable hunks must be nonempty")
	}
	if e = validateReferences(&change); e != nil {
		return nil, stableCommonWireError(e)
	}
	a, e := stableArray(v.Obj, "criterionBindings", 256)
	if e != nil {
		return nil, e
	}
	for _, x := range a {
		if x.Kind != KindObject {
			return nil, fieldError("candidate record must be object")
		}
		keys := stableCriterionKeys(x.Obj)
		if e = candidateObject(x, keys...); e != nil {
			return nil, e
		}
		for _, k := range keys {
			t, _ := x.Obj.Get(k)
			if k == "criterionIndex" {
				if t.Kind != KindInt || t.Int > 255 {
					return nil, fieldError("criterion index")
				}
				continue
			}
			if k == "externalSpan" {
				span, e := validateSpan(t)
				if e != nil {
					return nil, e
				}
				if span.Start < 0 || span.Start >= span.End {
					return nil, fieldError("external criterion span")
				}
				continue
			}
			if t.Kind != KindString {
				return nil, fieldError("criterion string required")
			}
			switch k {
			case "externalRepository":
				if !IsStableExternalRepository(t.Str) {
					return nil, fieldError("external criterion repository")
				}
			case "externalBlobOid":
				if !IsGitOid(t.Str) {
					return nil, fieldError("external criterion blob OID")
				}
			case "id":
				if !isPrefixedDigest(t.Str, CandidateCriterionPrefix) {
					return nil, fieldError("criterion ID")
				}
			case "ticketId":
				if !candidateTicketID(t.Str) {
					return nil, fieldError("ticket identity")
				}
			case "acceptanceRevision":
				if !candidateRevision(t.Str) {
					return nil, fieldError("acceptance revision")
				}
			case "evidenceId":
				if !isPrefixedDigest(t.Str, EvidencePrefix) {
					return nil, fieldError("criterion evidence ID")
				}
			default:
				if !IsSha256(t.Str) {
					return nil, fieldError("criterion digest")
				}
			}
		}
		var t StableCriterion
		if e = json.Unmarshal(CanonicalValue(x), &t); e != nil {
			return nil, e
		}
		d.Criteria = append(d.Criteria, t)
	}
	a, e = stableArray(v.Obj, "runnerReceipts", 64)
	if e != nil {
		return nil, e
	}
	for _, x := range a {
		keys := []string{"sha256", "profile", "planSha256", "sourceGitBinding", "executionAuthority", "dependencyClosure", "authentication"}
		if e = candidateObject(x, keys...); e != nil {
			return nil, e
		}
		fixed := map[string]string{"sourceGitBinding": "NOT_OBSERVED", "executionAuthority": "CALLER_OBSERVED", "dependencyClosure": "NOT_OBSERVED"}
		for _, k := range keys {
			t, _ := x.Obj.Get(k)
			if k == "authentication" {
				if _, e = parseStableAuthentication(t); e != nil {
					return nil, e
				}
				continue
			}
			if t.Kind != KindString {
				return nil, fieldError("runner string required")
			}
			if k == "profile" {
				if t.Str != StableRunnerReceiptProfile && t.Str != StableOpaqueReceiptProfile {
					return nil, fieldError("runner limitation")
				}
			} else if want, ok := fixed[k]; ok {
				if t.Str != want {
					return nil, fieldError("runner limitation")
				}
			} else if !IsSha256(t.Str) {
				return nil, fieldError("runner digest")
			}
		}
		var t StableReceipt
		if e = json.Unmarshal(CanonicalValue(x), &t); e != nil {
			return nil, e
		}
		d.Receipts = append(d.Receipts, t)
	}
	a, e = stableArray(v.Obj, "criterionLinks", 1024)
	if e != nil {
		return nil, e
	}
	for _, x := range a {
		keys := []string{"criterionId", "hunkIds", "evidenceIds", "runnerReceiptSha256s"}
		if e = candidateObject(x, keys...); e != nil {
			return nil, e
		}
		id, _ := x.Obj.Get("criterionId")
		if id.Kind != KindString || !isPrefixedDigest(id.Str, CandidateCriterionPrefix) {
			return nil, fieldError("linked criterion ID")
		}
		for i, k := range keys[1:] {
			t, _ := x.Obj.Get(k)
			if t.Kind != KindArray || len(t.Arr) == 0 || len(t.Arr) > 32 {
				return nil, fieldError("link array bound")
			}
			for _, r := range t.Arr {
				if r.Kind != KindString {
					return nil, fieldError("link string")
				}
				valid := IsSha256(r.Str)
				if i == 0 {
					valid = isPrefixedDigest(r.Str, HunkPrefix)
				}
				if i == 1 {
					valid = isPrefixedDigest(r.Str, EvidencePrefix)
				}
				if !valid {
					return nil, fieldError("link identity")
				}
			}
		}
		var t StableLink
		if e = json.Unmarshal(CanonicalValue(x), &t); e != nil {
			return nil, e
		}
		d.Links = append(d.Links, t)
	}
	a, e = stableArray(v.Obj, "artifacts", 1024)
	if e != nil {
		return nil, e
	}
	kinds := map[string]bool{"tasks-capture": true, "tasks-verification": true, "tasks-claimed-ticket": true, "tasks-snapshot-head-receipt": true, "runner-plan": true, "runner-receipt": true, ReceiptAttestationKind: true}
	for _, x := range a {
		if e = candidateObject(x, "kind", "path", "sha256"); e != nil {
			return nil, e
		}
		k, _ := x.Obj.Get("kind")
		p, _ := x.Obj.Get("path")
		h, _ := x.Obj.Get("sha256")
		if k.Kind != KindString || !kinds[k.Str] || p.Kind != KindString || h.Kind != KindString || !IsSha256(h.Str) {
			return nil, fieldError("artifact fields")
		}
		if e = ValidatePath(p.Str); e != nil {
			return nil, fieldError("artifact path")
		}
		for _, component := range strings.Split(p.Str, "/") {
			if strings.EqualFold(component, ".git") {
				return nil, fieldError("reserved artifact component")
			}
		}
		if p.Str == ExcludedCEMPath {
			return nil, fieldError("artifact cannot name sidecar")
		}
		d.Artifacts = append(d.Artifacts, StableArtifact{k.Str, p.Str, h.Str})
	}
	if e = parseStableProvenanceRole(v.Obj, d); e != nil {
		return nil, e
	}
	return d, validateStableGraph(d)
}
func stableGraphError(code string) error { return cemcode.New(code, "stable reference graph rejected") }
func stableCriterionIdentity(c StableCriterion) string {
	raw, _ := json.Marshal(c)
	v, _ := Parse(raw)
	return candidateRecordIdentity(v.Obj)
}
func validateStableGraph(d *StableMap) error {
	ids, tuples, groups, spans := map[string]bool{}, map[string]bool{}, map[string]string{}, map[string]bool{}
	for _, c := range d.Criteria {
		if c.ID != stableCriterionIdentity(c) {
			return stableGraphError("invalid-criterion-identity")
		}
		// Plain criteria share one unnamed group (empty ticket and revision),
		// which no Tasks criterion can occupy.
		tuple := c.TicketID + "\x00" + c.AcceptanceRevision + "\x00" + strconv.FormatInt(c.CriterionIndex, 10)
		// External criteria share that unnamed group (CEM-V1-332); one
		// external blob span occurs in at most one binding.
		span := c.EvidenceID
		if c.External() {
			span = "external\x00" + c.ExternalRepository + "\x00" + c.ExternalBlobOid + "\x00" + strconv.FormatInt(c.ExternalSpan.Start, 10) + "\x00" + strconv.FormatInt(c.ExternalSpan.End, 10)
		}
		if ids[c.ID] || tuples[tuple] || !c.Tasks() && spans[span] {
			return stableGraphError("duplicate-reference")
		}
		spans[span] = !c.Tasks()
		ids[c.ID] = true
		tuples[tuple] = true
		group := c.TicketID + "\x00" + c.AcceptanceRevision
		binding := strings.Join([]string{c.AcceptanceSha256, c.CaptureSha256, c.VerificationSha256, c.ClaimTicketSha256, c.SnapshotHeadReceiptSha256, c.SnapshotHeadArtifactSha256}, "/")
		if prior, ok := groups[group]; ok && prior != binding {
			return stableGraphError("reference-coherence")
		}
		groups[group] = binding
	}
	receipts := map[string]bool{}
	for _, r := range d.Receipts {
		if receipts[r.Sha256] {
			return stableGraphError("duplicate-reference")
		}
		receipts[r.Sha256] = true
	}
	hunks := map[string]Hunk{}
	evidence, spanDigests := map[string]bool{}, map[string]string{}
	for _, h := range d.Change.Hunks {
		if _, ok := hunks[h.ID]; ok {
			return stableGraphError("duplicate-reference")
		}
		hunks[h.ID] = h
	}
	for _, e := range d.Change.Evidence {
		evidence[e.ID] = true
		spanDigests[e.ID] = e.SpanSha256
	}
	// A plain criterion is the cited base blob span itself: its digest is the
	// span digest, which canonical evidence verification then binds to Git.
	for _, c := range d.Criteria {
		if !c.Plain() {
			continue
		}
		if !evidence[c.EvidenceID] {
			return stableGraphError("unresolved-reference")
		}
		if spanDigests[c.EvidenceID] != c.CriterionSha256 {
			return stableGraphError("reference-coherence")
		}
	}
	usedCriteria, usedReceipts := map[string]bool{}, map[string]bool{}
	// Resolve all links before checking basis membership, preserving graph precedence.
	for _, l := range d.Links {
		if usedCriteria[l.CriterionID] {
			return stableGraphError("duplicate-reference")
		}
		if !ids[l.CriterionID] {
			return stableGraphError("unresolved-reference")
		}
		usedCriteria[l.CriterionID] = true
		lists := [][]string{l.HunkIDs, l.EvidenceIDs, l.RunnerReceiptSha256s}
		// CEM-V1-176: uniqueness within all three arrays precedes resolution.
		for _, list := range lists {
			seen := map[string]bool{}
			for _, id := range list {
				if seen[id] {
					return stableGraphError("duplicate-reference")
				}
				seen[id] = true
			}
		}
		for i, list := range lists {
			for _, id := range list {
				valid := false
				switch i {
				case 0:
					_, valid = hunks[id]
				case 1:
					valid = evidence[id]
				case 2:
					valid = receipts[id]
					usedReceipts[id] = true
				}
				if !valid {
					return stableGraphError("unresolved-reference")
				}
			}
		}
	}
	for _, l := range d.Links {
		basis := map[string]bool{}
		for _, id := range l.HunkIDs {
			for _, b := range hunks[id].Basis {
				basis[b.EvidenceID] = true
			}
		}
		for _, id := range l.EvidenceIDs {
			if !basis[id] {
				return stableGraphError("reference-coherence")
			}
		}
	}
	if len(usedCriteria) != len(ids) || len(usedReceipts) != len(receipts) {
		return stableGraphError("reference-coherence")
	}
	required := map[string]string{}
	need := func(digest, kind string) bool {
		prior, ok := required[digest]
		if ok && prior != kind {
			return false
		}
		required[digest] = kind
		return true
	}
	for _, c := range d.Criteria {
		if !c.Tasks() {
			continue
		}
		for _, pair := range [][2]string{{c.CaptureSha256, "tasks-capture"}, {c.VerificationSha256, "tasks-verification"}, {c.ClaimTicketSha256, "tasks-claimed-ticket"}, {c.SnapshotHeadArtifactSha256, "tasks-snapshot-head-receipt"}} {
			if !need(pair[0], pair[1]) {
				return stableGraphError("artifact-role-closure")
			}
		}
	}
	envelopes := map[string]bool{}
	for _, r := range d.Receipts {
		if !need(r.Sha256, "runner-receipt") || !need(r.PlanSha256, "runner-plan") {
			return stableGraphError("artifact-role-closure")
		}
		// CEM-V1-084: each signed receipt names its own receipt-attestation
		// artifact; an attestation no receipt names fails the closure below.
		if r.Authentication.DSSE() {
			if envelopes[r.Authentication.EnvelopeSha256] {
				return stableGraphError("duplicate-reference")
			}
			envelopes[r.Authentication.EnvelopeSha256] = true
			if !need(r.Authentication.EnvelopeSha256, ReceiptAttestationKind) {
				return stableGraphError("artifact-role-closure")
			}
		}
	}
	paths, digests := map[string]bool{}, map[string]bool{}
	for _, a := range d.Artifacts {
		if paths[a.Path] || digests[a.Sha256] {
			return stableGraphError("duplicate-reference")
		}
		paths[a.Path] = true
		digests[a.Sha256] = true
		if required[a.Sha256] != a.Kind {
			return stableGraphError("artifact-role-closure")
		}
	}
	if len(required) != len(d.Artifacts) {
		return stableGraphError("artifact-role-closure")
	}
	empty := len(d.Criteria) == 0
	if (len(d.Receipts) == 0) != empty || (len(d.Links) == 0) != empty || (len(d.Artifacts) == 0) != empty {
		return stableGraphError("reference-coherence")
	}
	return nil
}

// StableOriginalDigest is the immutable raw input identity, never an encoder digest.
func (d *StableMap) StableOriginalDigest() string {
	h := sha256.Sum256(d.original)
	return hex.EncodeToString(h[:])
}

func stableArray(o *Object, key string, limit int) ([]Value, error) {
	v, _ := o.Get(key)
	if v.Kind != KindArray || len(v.Arr) > limit {
		return nil, fieldError("stable array bound")
	}
	return v.Arr, nil
}
func stableDepth(v Value, depth int) bool {
	if v.Kind != KindArray && v.Kind != KindObject {
		return true
	}
	if depth > 64 {
		return false
	}
	for _, child := range v.Arr {
		if !stableDepth(child, depth+1) {
			return false
		}
	}
	if v.Obj != nil {
		for _, child := range v.Obj.Values {
			if !stableDepth(child, depth+1) {
				return false
			}
		}
	}
	return true
}

func stableCommonWireError(err error) error {
	switch cemcode.CodeOf(err) {
	case cemcode.PathTraversal, cemcode.UnsupportedWithoutBasis, cemcode.OrphanEvidence:
		return fieldError("stable common wire relation")
	default:
		return err
	}
}
