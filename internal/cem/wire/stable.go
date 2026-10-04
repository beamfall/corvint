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
	original  []byte
}
type StableCriterion struct {
	ID                         string `json:"id"`
	TicketID                   string `json:"ticketId"`
	AcceptanceRevision         string `json:"acceptanceRevision"`
	CriterionIndex             int64  `json:"criterionIndex"`
	AcceptanceSha256           string `json:"acceptanceSha256"`
	CriterionSha256            string `json:"criterionSha256"`
	CaptureSha256              string `json:"captureSha256"`
	VerificationSha256         string `json:"verificationSha256"`
	ClaimTicketSha256          string `json:"claimTicketSha256"`
	SnapshotHeadReceiptSha256  string `json:"snapshotHeadReceiptSha256"`
	SnapshotHeadArtifactSha256 string `json:"snapshotHeadArtifactSha256"`
}
type StableReceipt struct {
	Sha256             string `json:"sha256"`
	Profile            string `json:"profile"`
	PlanSha256         string `json:"planSha256"`
	SourceGitBinding   string `json:"sourceGitBinding"`
	ExecutionAuthority string `json:"executionAuthority"`
	DependencyClosure  string `json:"dependencyClosure"`
	Authentication     string `json:"authentication"`
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
	if spec.Kind != KindString || spec.Str != StableSpec {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "stable profile required")
	}
	if e = requireClosedKeys(v.Obj, []string{"spec", "baseRevision", "patchSha256", "excludedPath", "evidence", "hunks", "criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"}); e != nil {
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
		keys := []string{"id", "ticketId", "acceptanceRevision", "criterionIndex", "acceptanceSha256", "criterionSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256", "snapshotHeadArtifactSha256"}
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
			if t.Kind != KindString {
				return nil, fieldError("criterion string required")
			}
			switch k {
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
		fixed := map[string]string{"profile": "corvint-test-runner-receipt/0", "sourceGitBinding": "NOT_OBSERVED", "executionAuthority": "CALLER_OBSERVED", "dependencyClosure": "NOT_OBSERVED", "authentication": "NOT_OBSERVED"}
		for _, k := range keys {
			t, _ := x.Obj.Get(k)
			if t.Kind != KindString {
				return nil, fieldError("runner string required")
			}
			if want, ok := fixed[k]; ok {
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
	kinds := map[string]bool{"tasks-capture": true, "tasks-verification": true, "tasks-claimed-ticket": true, "tasks-snapshot-head-receipt": true, "runner-plan": true, "runner-receipt": true}
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
	return d, validateStableGraph(d)
}
func stableGraphError(code string) error { return cemcode.New(code, "stable reference graph rejected") }
func stableCriterionIdentity(c StableCriterion) string {
	raw, _ := json.Marshal(c)
	v, _ := Parse(raw)
	return candidateRecordIdentity(v.Obj)
}
func validateStableGraph(d *StableMap) error {
	ids, tuples, groups := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, c := range d.Criteria {
		if c.ID != stableCriterionIdentity(c) {
			return stableGraphError("invalid-criterion-identity")
		}
		tuple := c.TicketID + "\x00" + c.AcceptanceRevision + "\x00" + strconv.FormatInt(c.CriterionIndex, 10)
		if ids[c.ID] || tuples[tuple] {
			return stableGraphError("duplicate-reference")
		}
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
	evidence := map[string]bool{}
	for _, h := range d.Change.Hunks {
		if _, ok := hunks[h.ID]; ok {
			return stableGraphError("duplicate-reference")
		}
		hunks[h.ID] = h
	}
	for _, e := range d.Change.Evidence {
		evidence[e.ID] = true
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
		for i, list := range [][]string{l.HunkIDs, l.EvidenceIDs, l.RunnerReceiptSha256s} {
			seen := map[string]bool{}
			for _, id := range list {
				if seen[id] {
					return stableGraphError("duplicate-reference")
				}
				seen[id] = true
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
		for _, pair := range [][2]string{{c.CaptureSha256, "tasks-capture"}, {c.VerificationSha256, "tasks-verification"}, {c.ClaimTicketSha256, "tasks-claimed-ticket"}, {c.SnapshotHeadArtifactSha256, "tasks-snapshot-head-receipt"}} {
			if !need(pair[0], pair[1]) {
				return stableGraphError("artifact-role-closure")
			}
		}
	}
	for _, r := range d.Receipts {
		if !need(r.Sha256, "runner-receipt") || !need(r.PlanSha256, "runner-plan") {
			return stableGraphError("artifact-role-closure")
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
