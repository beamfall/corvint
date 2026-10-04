package wire

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// CandidateSpec is deliberately outside legacy ParseMap and Canonical dispatch.
// It binds references, never native authority, execution or criterion adequacy.
const CandidateSpec = "cem/1.0-experimental.1"
const CandidateCriterionPrefix = "criterion:sha256:"
const CandidateMaxArtifacts = 1024
const CandidateMaxArtifactBytes = 4 << 20
const CandidateMaxTotalArtifactBytes = 16 << 20

// Candidate preserves the exact profile on the inherited change mechanics.
// Its references require separate, bounded artifact-byte verification.
type Candidate struct {
	Change    Map
	Criteria  []CriterionReference
	Receipts  []RunnerReceiptReference
	Links     []CriterionLink
	Artifacts []CandidateArtifact
}
type CriterionReference struct {
	ID, TicketID, AcceptanceRevision, AcceptanceSha256                                               string
	CriterionIndex                                                                                   int64
	CriterionSha256, CaptureSha256, VerificationSha256, ClaimTicketSha256, SnapshotHeadReceiptSha256 string
}
type RunnerReceiptReference struct{ Sha256, PlanSha256 string }
type CriterionLink struct {
	CriterionID                                string
	HunkIDs, EvidenceIDs, RunnerReceiptSha256s []string
}
type CandidateArtifact struct{ Kind, Path, Sha256 string }

// ParseCandidate validates the closed experimental wire and reference graph.
// It never interprets native Tasks records or native execution receipts.
func ParseCandidate(raw []byte) (*Candidate, error) {
	if len(raw) > MaxMapBytes {
		return nil, cemcode.New(cemcode.MapUnavailable, "candidate map byte bound")
	}
	root, err := Parse(raw)
	if err != nil {
		return nil, err
	}
	if root.Kind != KindObject {
		return nil, fieldError("candidate root must be object")
	}
	spec, _ := root.Obj.Get("spec")
	if spec.Kind != KindString || spec.Str != CandidateSpec {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "candidate interface requires %s", CandidateSpec)
	}
	if err = requireClosedKeys(root.Obj, []string{"spec", "baseRevision", "patchSha256", "excludedPath", "evidence", "hunks", "criterionBindings", "runnerReceipts", "criterionLinks", "artifacts"}); err != nil {
		return nil, err
	}
	c := &Candidate{Change: Map{Spec: CandidateSpec}}
	if err = validateCommonFields(root.Obj, &c.Change, true); err != nil {
		return nil, err
	}
	if err = validateReferences(&c.Change); err != nil {
		return nil, err
	}
	// Unlike historical stage ordering, candidate references need an unambiguous
	// hunk index before any artifact can be opened.
	hunkIDs := map[string]bool{}
	for _, h := range c.Change.Hunks {
		if hunkIDs[h.ID] {
			return nil, fieldError("duplicate candidate hunk")
		}
		hunkIDs[h.ID] = true
	}
	items, err := candidateArray(root.Obj, "criterionBindings", 256)
	if err != nil {
		return nil, err
	}
	ids, tuples, groups := map[string]bool{}, map[string]bool{}, map[string]string{}
	for _, item := range items {
		keys := []string{"id", "ticketId", "acceptanceRevision", "acceptanceSha256", "criterionIndex", "criterionSha256", "captureSha256", "verificationSha256", "claimTicketSha256", "snapshotHeadReceiptSha256"}
		if err = candidateObject(item, keys...); err != nil {
			return nil, err
		}
		text := func(k string) string {
			v, _ := item.Obj.Get(k)
			if v.Kind != KindString {
				return ""
			}
			return v.Str
		}
		x := CriterionReference{ID: text("id"), TicketID: text("ticketId"), AcceptanceRevision: text("acceptanceRevision"), AcceptanceSha256: text("acceptanceSha256"), CriterionSha256: text("criterionSha256"), CaptureSha256: text("captureSha256"), VerificationSha256: text("verificationSha256"), ClaimTicketSha256: text("claimTicketSha256"), SnapshotHeadReceiptSha256: text("snapshotHeadReceiptSha256")}
		index, _ := item.Obj.Get("criterionIndex")
		if index.Kind != KindInt || index.Int >= 256 {
			return nil, fieldError("criterion index bound")
		}
		x.CriterionIndex = index.Int
		if !candidateTicketID(x.TicketID) || !candidateRevision(x.AcceptanceRevision) {
			return nil, fieldError("canonical native criterion reference required")
		}
		for _, d := range []string{x.AcceptanceSha256, x.CriterionSha256, x.CaptureSha256, x.VerificationSha256, x.ClaimTicketSha256, x.SnapshotHeadReceiptSha256} {
			if !IsSha256(d) {
				return nil, fieldError("criterion digest required")
			}
		}
		if x.ID != candidateRecordIdentity(item.Obj) || ids[x.ID] {
			return nil, fieldError("criterion identity mismatch or duplicate")
		}
		ids[x.ID] = true
		tuple := x.TicketID + "\x00" + x.AcceptanceRevision + "\x00" + strconv.FormatInt(x.CriterionIndex, 10)
		if tuples[tuple] {
			return nil, fieldError("duplicate native criterion tuple")
		}
		tuples[tuple] = true
		group := x.TicketID + "\x00" + x.AcceptanceRevision
		binding := strings.Join([]string{x.AcceptanceSha256, x.CaptureSha256, x.VerificationSha256, x.ClaimTicketSha256, x.SnapshotHeadReceiptSha256}, "/")
		if old, ok := groups[group]; ok && old != binding {
			return nil, fieldError("incoherent criterion capture group")
		}
		groups[group] = binding
		c.Criteria = append(c.Criteria, x)
	}
	items, err = candidateArray(root.Obj, "runnerReceipts", 64)
	if err != nil {
		return nil, err
	}
	receipts := map[string]bool{}
	for _, item := range items {
		if err = candidateObject(item, "sha256", "profile", "planSha256", "sourceGitBinding", "executionAuthority", "dependencyClosure", "authentication"); err != nil {
			return nil, err
		}
		for k, want := range map[string]string{"profile": "corvint-test-runner-receipt/0", "sourceGitBinding": "NOT_OBSERVED", "executionAuthority": "CALLER_OBSERVED", "dependencyClosure": "NOT_OBSERVED", "authentication": "NOT_OBSERVED"} {
			v, _ := item.Obj.Get(k)
			if v.Kind != KindString || v.Str != want {
				return nil, fieldError("unqualified runner receipt field %s", k)
			}
		}
		digest, _ := item.Obj.Get("sha256")
		plan, _ := item.Obj.Get("planSha256")
		if digest.Kind != KindString || plan.Kind != KindString || !IsSha256(digest.Str) || !IsSha256(plan.Str) || receipts[digest.Str] {
			return nil, fieldError("invalid or duplicate runner receipt")
		}
		receipts[digest.Str] = true
		c.Receipts = append(c.Receipts, RunnerReceiptReference{digest.Str, plan.Str})
	}
	items, err = candidateArray(root.Obj, "criterionLinks", 1024)
	if err != nil {
		return nil, err
	}
	usedCriteria, usedReceipts := map[string]bool{}, map[string]bool{}
	for _, item := range items {
		if err = candidateObject(item, "criterionId", "hunkIds", "evidenceIds", "runnerReceiptSha256s"); err != nil {
			return nil, err
		}
		id, _ := item.Obj.Get("criterionId")
		if id.Kind != KindString || !ids[id.Str] || usedCriteria[id.Str] {
			return nil, fieldError("unknown or repeated linked criterion")
		}
		usedCriteria[id.Str] = true
		link := CriterionLink{CriterionID: id.Str}
		if link.HunkIDs, err = candidateRefs(item.Obj, "hunkIds", hunkIDs); err != nil {
			return nil, err
		}
		basis := map[string]bool{}
		for _, h := range c.Change.Hunks {
			for _, id := range link.HunkIDs {
				if h.ID == id {
					for _, b := range h.Basis {
						basis[b.EvidenceID] = true
					}
				}
			}
		}
		if link.EvidenceIDs, err = candidateRefs(item.Obj, "evidenceIds", basis); err != nil {
			return nil, err
		}
		if link.RunnerReceiptSha256s, err = candidateRefs(item.Obj, "runnerReceiptSha256s", receipts); err != nil {
			return nil, err
		}
		for _, id := range link.RunnerReceiptSha256s {
			usedReceipts[id] = true
		}
		c.Links = append(c.Links, link)
	}
	if len(usedCriteria) != len(ids) || len(usedReceipts) != len(receipts) {
		return nil, fieldError("unreferenced candidate criterion or receipt")
	}
	required := map[string]string{}
	need := func(d, kind string) error {
		if old, ok := required[d]; ok && old != kind {
			return fieldError("artifact digest used in conflicting roles")
		}
		required[d] = kind
		return nil
	}
	for _, x := range c.Criteria {
		for kind, d := range map[string]string{"tasks-capture": x.CaptureSha256, "tasks-verification": x.VerificationSha256, "tasks-claimed-ticket": x.ClaimTicketSha256} {
			if err = need(d, kind); err != nil {
				return nil, err
			}
		}
	}
	for _, x := range c.Receipts {
		if err = need(x.Sha256, "runner-receipt"); err != nil {
			return nil, err
		}
		if err = need(x.PlanSha256, "runner-plan"); err != nil {
			return nil, err
		}
	}
	items, err = candidateArray(root.Obj, "artifacts", CandidateMaxArtifacts)
	if err != nil {
		return nil, err
	}
	seenPaths, seenDigests := map[string]bool{}, map[string]bool{}
	for _, item := range items {
		if err = candidateObject(item, "kind", "path", "sha256"); err != nil {
			return nil, err
		}
		kind, _ := item.Obj.Get("kind")
		path, _ := item.Obj.Get("path")
		digest, _ := item.Obj.Get("sha256")
		if kind.Kind != KindString || path.Kind != KindString || digest.Kind != KindString || !IsSha256(digest.Str) || required[digest.Str] != kind.Str || kind.Str == "" {
			return nil, fieldError("unknown, unreferenced or wrong-kind artifact")
		}
		if err = ValidatePath(path.Str); err != nil {
			return nil, err
		}
		for _, part := range strings.Split(path.Str, "/") {
			if strings.EqualFold(part, ".git") {
				return nil, fieldError("artifact Git segment refused")
			}
		}
		if seenPaths[path.Str] || seenDigests[digest.Str] {
			return nil, fieldError("duplicate artifact path or digest")
		}
		seenPaths[path.Str] = true
		seenDigests[digest.Str] = true
		c.Artifacts = append(c.Artifacts, CandidateArtifact{kind.Str, path.Str, digest.Str})
	}
	if len(required) != len(seenDigests) {
		return nil, fieldError("missing declared artifact")
	}
	return c, nil
}
func candidateObject(v Value, keys ...string) error {
	if v.Kind != KindObject {
		return fieldError("candidate record must be object")
	}
	return requireClosedKeys(v.Obj, keys)
}
func candidateArray(o *Object, key string, limit int) ([]Value, error) {
	v, _ := o.Get(key)
	if v.Kind != KindArray || len(v.Arr) < 1 || len(v.Arr) > limit {
		return nil, fieldError("candidate %s array bound", key)
	}
	return v.Arr, nil
}
func candidateRefs(o *Object, key string, available map[string]bool) ([]string, error) {
	items, e := candidateArray(o, key, 32)
	if e != nil {
		return nil, e
	}
	out := []string{}
	seen := map[string]bool{}
	for _, v := range items {
		if v.Kind != KindString || !available[v.Str] || seen[v.Str] {
			return nil, fieldError("invalid or duplicate candidate %s reference", key)
		}
		seen[v.Str] = true
		out = append(out, v.Str)
	}
	return out, nil
}
func candidateRecordIdentity(o *Object) string {
	p := &Object{Values: map[string]Value{}}
	for _, k := range o.Keys {
		if k != "id" {
			p.Keys = append(p.Keys, k)
			p.Values[k] = o.Values[k]
		}
	}
	d := sha256.Sum256(CanonicalValue(Value{Kind: KindObject, Obj: p}))
	return CandidateCriterionPrefix + hex.EncodeToString(d[:])
}
func candidateRevision(s string) bool {
	if s == "" || len(s) > 16 || s[0] == '0' {
		return false
	}
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, e := strconv.ParseUint(s, 10, 64)
	return e == nil && n > 0 && n <= MaxWireInteger
}
func candidateTicketID(s string) bool {
	if len(s) > 128 {
		return false
	}
	p := strings.Split(s, ":")
	if len(p) != 4 || p[0] != "ticket" || len(p[3]) > 64 {
		return false
	}
	for _, t := range p[1:] {
		if t == "" {
			return false
		}
		for i, c := range []byte(t) {
			if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				continue
			}
			if i == 0 || c != '.' && c != '_' && c != '-' {
				return false
			}
		}
	}
	return true
}
