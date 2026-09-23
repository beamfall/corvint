package wire

import (
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
)

// Frozen wire constants.
const (
	Spec01          = "cem/0.1"
	Spec02          = "cem/0.2"
	Spec03          = "cem/0.3"
	ExcludedCEMPath = ".corvint/change.cem.json"

	MaxMapBytes    = 4 << 20
	MaxEvidence    = 4096
	MaxHunks       = 2048
	MaxBases       = 32
	MaxPathBytes   = 512
	EvidencePrefix = "evidence:sha256:"
	HunkPrefix     = "hunk:sha256:"
)

// Relations enumerates the frozen basis relations.
var Relations = map[string]bool{
	"call-site": true, "decision": true, "dependency": true, "implementation": true,
	"incident": true, "specification": true, "test-claim": true,
}

var unknownReasons = map[string]bool{
	"no-evidence": true, "insufficient-evidence": true, "conflicting-evidence": true,
}

var mechanicalReasons = map[string]bool{
	"whitespace-only": true, "line-ending-only": true,
}

// StructuralReasons enumerates the cem/0.3 mechanical reasons proved by
// Go structural comparison (CEM-SM-001); 0.1 and 0.2 documents reject them.
var StructuralReasons = map[string]bool{
	"rename": true, "move": true, "import-reorder": true, "formatter-only": true,
}

// Canonical reports whether spec is a canonical committed-change profile:
// cem/0.2, or cem/0.3 which adds the structural reason vocabulary and the
// optional hunk coverage witness.
func Canonical(spec string) bool { return spec == Spec02 || spec == Spec03 }

// MechanicalReason reports whether reason is registered for spec.
func MechanicalReason(spec, reason string) bool {
	return mechanicalReasons[reason] || (spec == Spec03 && StructuralReasons[reason])
}

// Span is a zero-based half-open raw-byte range.
type Span struct{ Start, End int64 }

// Range is a unified-diff line range.
type Range struct{ Start, Count int64 }

// Evidence is one immutable evidence record.
type Evidence struct {
	ID         string
	BlobOid    string
	Path       string
	Span       Span
	SpanSha256 string
}

// Basis is one typed evidence reference.
type Basis struct {
	EvidenceID string
	Relation   string
}

// CoverageWitness is one hunk's optional cem/0.3 patch-coverage witness: the
// added lines one identified test run executed according to one local
// coverprofile (TCQ-V0-051). Covered ranges are one-based new-side line
// ranges inside the hunk's newRange, ascending and non-adjacent; an empty
// list is State CoverageUncovered, never an omitted witness.
type CoverageWitness struct {
	ProfileSha256 string
	TestRun       string
	Mode          string
	State         string
	Covered       []Range
}

// Frozen coverage witness vocabulary.
const (
	CoverageCovered   = "covered"
	CoverageUncovered = "uncovered"
	MaxTestRunBytes   = 256
)

var coverageModes = map[string]bool{"set": true, "count": true, "atomic": true}

// DiscriminationWitness is one hunk's optional cem/0.3 mutation witness: whether
// the tests its test claims cite kill bounded mutants of the hunk's new-side
// lines on one tree revision (TCQ-V0-056). Survivors describe every mutant
// that lived; a hunk the run never judged carries State DiscriminationNotRun
// with zero counts and its Detail, never an omitted witness.
type DiscriminationWitness struct {
	TreeRevision    string
	SelectionSha256 string
	Mutants         int64
	Killed          int64
	Survived        int64
	Survivors       []SurvivingMutant
	Bounds          DiscriminationBounds
	State           string
	Detail          string
}

// SurvivingMutant is one mutant the selected tests let live.
type SurvivingMutant struct {
	Operator    string
	Line        int64
	Description string
}

// DiscriminationBounds are the caps one discriminate run declared.
type DiscriminationBounds struct {
	MaxHunks        int64
	MaxMutants      int64
	WallTimeSeconds int64
}

// Frozen discrimination witness vocabulary.
const (
	DiscriminationDiscriminates = "discriminates"
	DiscriminationSurvived      = "survived"
	DiscriminationNotRun        = "not-run"
	MaxDiscriminationTextBytes  = 512
)

// Hunk is one mapped patch hunk. Coverage and Discriminates are nil unless a
// cem/0.3 witness is recorded.
type Hunk struct {
	ID            string
	Path          string
	OldRange      Range
	NewRange      Range
	Disposition   string
	Reason        string
	Basis         []Basis
	Coverage      *CoverageWitness
	Discriminates *DiscriminationWitness
}

// Map is a validated CEM 0.1, 0.2, or 0.3 document.
type Map struct {
	Spec         string
	BaseRevision string
	PatchSha256  string
	ExcludedPath string // empty for 0.1; the frozen literal for 0.2 and 0.3
	Evidence     []Evidence
	Hunks        []Hunk
}

func fieldError(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.InvalidField, format, args...)
}

// ParseMap reads and validates one CEM wire document with the frozen stage-2
// precedence: JSON errors, root object and spec, missing required fields,
// unknown fields, then field values in normative wire order.
func ParseMap(data []byte) (*Map, error) {
	if len(data) > MaxMapBytes {
		return nil, cemcode.New(cemcode.MapUnavailable, "map exceeds %d bytes", MaxMapBytes)
	}
	root, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if root.Kind != KindObject {
		return nil, cemcode.New(cemcode.NotAnObject, "CEM root must be a JSON object")
	}
	spec, present := root.Obj.Get("spec")
	if !present {
		return nil, cemcode.New(cemcode.MissingField, "spec is required")
	}
	if spec.Kind != KindString || (spec.Str != Spec01 && !Canonical(spec.Str)) {
		return nil, cemcode.New(cemcode.UnsupportedSpec, "spec must be %q, %q, or %q", Spec01, Spec02, Spec03)
	}
	required := []string{"spec", "baseRevision", "patchSha256", "evidence", "hunks"}
	if Canonical(spec.Str) {
		required = []string{"spec", "baseRevision", "patchSha256", "excludedPath", "evidence", "hunks"}
	}
	if err := requireClosedKeys(root.Obj, required); err != nil {
		return nil, err
	}
	result := &Map{Spec: spec.Str}
	if err := validateFields(root.Obj, result); err != nil {
		return nil, err
	}
	if err := validateReferences(result); err != nil {
		return nil, err
	}
	return result, nil
}

func requireClosedKeys(object *Object, required []string) error {
	allowed := map[string]bool{}
	for _, key := range required {
		if _, present := object.Get(key); !present {
			return cemcode.New(cemcode.MissingField, "%s is required", key)
		}
		allowed[key] = true
	}
	for _, key := range object.Keys {
		if !allowed[key] {
			return cemcode.New(cemcode.UnknownField, "unknown field %q", key)
		}
	}
	return nil
}

func validateFields(object *Object, result *Map) error {
	base, _ := object.Get("baseRevision")
	if base.Kind != KindString || !IsGitOid(base.Str) {
		return fieldError("baseRevision must be a full lowercase Git commit OID")
	}
	result.BaseRevision = base.Str
	digest, _ := object.Get("patchSha256")
	if digest.Kind != KindString || !IsSha256(digest.Str) {
		return fieldError("patchSha256 must be 64 lowercase hex bytes")
	}
	result.PatchSha256 = digest.Str
	if Canonical(result.Spec) {
		excluded, _ := object.Get("excludedPath")
		if excluded.Kind != KindString || excluded.Str != ExcludedCEMPath {
			return cemcode.New(cemcode.InvalidExcludedPath, "excludedPath must be the exact string %q", ExcludedCEMPath)
		}
		result.ExcludedPath = excluded.Str
	}
	evidence, _ := object.Get("evidence")
	if evidence.Kind != KindArray || len(evidence.Arr) > MaxEvidence {
		return fieldError("evidence must be an array of at most %d items", MaxEvidence)
	}
	for index, item := range evidence.Arr {
		record, err := validateEvidence(item)
		if err != nil {
			return err
		}
		_ = index
		result.Evidence = append(result.Evidence, record)
	}
	hunks, _ := object.Get("hunks")
	// Emptiness is deliberately not rejected here: the interop manifest binds
	// patch-level expectedCodes (diff-metadata-mismatch, unsupported-tree-mode,
	// too-many-lines) to vectors whose maps carry empty hunks arrays, so the
	// patch must be judged first; an empty map always fails hunk coverage
	// against any conformant patch afterwards.
	if hunks.Kind != KindArray || len(hunks.Arr) > MaxHunks {
		return fieldError("hunks must be an array of at most %d items", MaxHunks)
	}
	for _, item := range hunks.Arr {
		record, err := validateHunk(item, result.Spec)
		if err != nil {
			return err
		}
		result.Hunks = append(result.Hunks, record)
	}
	return nil
}

func validateEvidence(item Value) (Evidence, error) {
	if item.Kind != KindObject {
		return Evidence{}, fieldError("evidence items must be objects")
	}
	if err := requireClosedKeys(item.Obj, []string{"id", "blobOid", "path", "span", "spanSha256"}); err != nil {
		return Evidence{}, err
	}
	id, _ := item.Obj.Get("id")
	if id.Kind != KindString || !isPrefixedDigest(id.Str, EvidencePrefix) {
		return Evidence{}, fieldError("evidence id must match %sDIGEST", EvidencePrefix)
	}
	blob, _ := item.Obj.Get("blobOid")
	if blob.Kind != KindString || !IsGitOid(blob.Str) {
		return Evidence{}, fieldError("evidence blobOid must be a full lowercase Git OID")
	}
	path, _ := item.Obj.Get("path")
	if path.Kind != KindString {
		return Evidence{}, fieldError("evidence path must be a string")
	}
	if err := ValidatePath(path.Str); err != nil {
		return Evidence{}, err
	}
	span, _ := item.Obj.Get("span")
	spanValue, err := validateSpan(span)
	if err != nil {
		return Evidence{}, err
	}
	digest, _ := item.Obj.Get("spanSha256")
	if digest.Kind != KindString || !IsSha256(digest.Str) {
		return Evidence{}, fieldError("evidence spanSha256 must be 64 lowercase hex bytes")
	}
	return Evidence{ID: id.Str, BlobOid: blob.Str, Path: path.Str, Span: spanValue, SpanSha256: digest.Str}, nil
}

func validateSpan(span Value) (Span, error) {
	if span.Kind != KindObject {
		return Span{}, fieldError("span must be an object")
	}
	if err := requireClosedKeys(span.Obj, []string{"start", "end"}); err != nil {
		return Span{}, err
	}
	start, _ := span.Obj.Get("start")
	end, _ := span.Obj.Get("end")
	if start.Kind != KindInt || end.Kind != KindInt || end.Int < 1 {
		return Span{}, fieldError("span start must be >= 0 and end >= 1")
	}
	return Span{Start: start.Int, End: end.Int}, nil
}

func validateRange(value Value, name string) (Range, error) {
	if value.Kind != KindObject {
		return Range{}, fieldError("%s must be an object", name)
	}
	if err := requireClosedKeys(value.Obj, []string{"start", "count"}); err != nil {
		return Range{}, err
	}
	start, _ := value.Obj.Get("start")
	count, _ := value.Obj.Get("count")
	if start.Kind != KindInt || count.Kind != KindInt {
		return Range{}, fieldError("%s start and count must be wire integers", name)
	}
	return Range{Start: start.Int, Count: count.Int}, nil
}

func validateHunk(item Value, spec string) (Hunk, error) {
	if item.Kind != KindObject {
		return Hunk{}, fieldError("hunks items must be objects")
	}
	keys := []string{"id", "path", "oldRange", "newRange", "disposition", "reason", "basis"}
	coverageValue, hasCoverage := item.Obj.Get("coverage")
	witnessed := hasCoverage && spec == Spec03
	if witnessed {
		keys = append(keys, "coverage")
	}
	discriminatesValue, hasDiscriminates := item.Obj.Get("discriminates")
	discriminated := hasDiscriminates && spec == Spec03
	if discriminated {
		keys = append(keys, "discriminates")
	}
	if err := requireClosedKeys(item.Obj, keys); err != nil {
		return Hunk{}, err
	}
	id, _ := item.Obj.Get("id")
	if id.Kind != KindString || !isPrefixedDigest(id.Str, HunkPrefix) {
		return Hunk{}, fieldError("hunk id must match %sDIGEST", HunkPrefix)
	}
	path, _ := item.Obj.Get("path")
	if path.Kind != KindString {
		return Hunk{}, fieldError("hunk path must be a string")
	}
	if err := ValidatePath(path.Str); err != nil {
		return Hunk{}, err
	}
	oldValue, _ := item.Obj.Get("oldRange")
	oldRange, err := validateRange(oldValue, "oldRange")
	if err != nil {
		return Hunk{}, err
	}
	newValue, _ := item.Obj.Get("newRange")
	newRange, err := validateRange(newValue, "newRange")
	if err != nil {
		return Hunk{}, err
	}
	disposition, _ := item.Obj.Get("disposition")
	reason, _ := item.Obj.Get("reason")
	basisValue, _ := item.Obj.Get("basis")
	if disposition.Kind != KindString || reason.Kind != KindString || basisValue.Kind != KindArray {
		return Hunk{}, fieldError("hunk disposition, reason, and basis have fixed types")
	}
	if len(basisValue.Arr) > MaxBases {
		return Hunk{}, fieldError("basis holds at most %d references", MaxBases)
	}
	var bases []Basis
	seen := map[Basis]bool{}
	for _, entry := range basisValue.Arr {
		basis, err := validateBasis(entry)
		if err != nil {
			return Hunk{}, err
		}
		if seen[basis] {
			return Hunk{}, fieldError("duplicate basis pair")
		}
		seen[basis] = true
		bases = append(bases, basis)
	}
	hunk := Hunk{ID: id.Str, Path: path.Str, OldRange: oldRange, NewRange: newRange,
		Disposition: disposition.Str, Reason: reason.Str, Basis: bases}
	if err := validateDisposition(hunk, spec); err != nil {
		return Hunk{}, err
	}
	if witnessed {
		witness, err := validateCoverage(coverageValue, newRange)
		if err != nil {
			return Hunk{}, err
		}
		hunk.Coverage = &witness
	}
	if discriminated {
		witness, err := validateDiscrimination(discriminatesValue, newRange)
		if err != nil {
			return Hunk{}, err
		}
		hunk.Discriminates = &witness
	}
	return hunk, nil
}

// ValidateTestRun checks the verbatim test run identity a coverage witness
// records: non-empty, at most MaxTestRunBytes, and free of control characters.
func ValidateTestRun(text string) error {
	if text == "" || len(text) > MaxTestRunBytes {
		return fieldError("coverage testRun must be 1..%d bytes", MaxTestRunBytes)
	}
	for _, character := range text {
		if character < 0x20 || character == 0x7f {
			return fieldError("coverage testRun must not contain control characters")
		}
	}
	return nil
}

func validateCoverage(value Value, newRange Range) (CoverageWitness, error) {
	if value.Kind != KindObject {
		return CoverageWitness{}, fieldError("hunk coverage must be an object")
	}
	if err := requireClosedKeys(value.Obj, []string{"profileSha256", "testRun", "mode", "state", "covered"}); err != nil {
		return CoverageWitness{}, err
	}
	profile, _ := value.Obj.Get("profileSha256")
	if profile.Kind != KindString || !IsSha256(profile.Str) {
		return CoverageWitness{}, fieldError("coverage profileSha256 must be a lowercase SHA-256 digest")
	}
	testRun, _ := value.Obj.Get("testRun")
	if testRun.Kind != KindString {
		return CoverageWitness{}, fieldError("coverage testRun must be a string")
	}
	if err := ValidateTestRun(testRun.Str); err != nil {
		return CoverageWitness{}, err
	}
	mode, _ := value.Obj.Get("mode")
	if mode.Kind != KindString || !coverageModes[mode.Str] {
		return CoverageWitness{}, fieldError("coverage mode must be set, count, or atomic")
	}
	state, _ := value.Obj.Get("state")
	if state.Kind != KindString || (state.Str != CoverageCovered && state.Str != CoverageUncovered) {
		return CoverageWitness{}, fieldError("coverage state must be %s or %s", CoverageCovered, CoverageUncovered)
	}
	coveredValue, _ := value.Obj.Get("covered")
	if coveredValue.Kind != KindArray {
		return CoverageWitness{}, fieldError("coverage covered must be an array")
	}
	covered, err := validateCoveredRanges(coveredValue.Arr, newRange)
	if err != nil {
		return CoverageWitness{}, err
	}
	if (len(covered) > 0) != (state.Str == CoverageCovered) {
		return CoverageWitness{}, fieldError("coverage state must agree with its covered ranges")
	}
	return CoverageWitness{ProfileSha256: profile.Str, TestRun: testRun.Str, Mode: mode.Str,
		State: state.Str, Covered: covered}, nil
}

// validateCoveredRanges requires ascending, non-adjacent, non-empty line
// ranges that lie inside the hunk's new-side range.
func validateCoveredRanges(entries []Value, newRange Range) ([]Range, error) {
	covered := []Range{}
	limit := newRange.Start + newRange.Count
	next := newRange.Start
	for _, entry := range entries {
		item, err := validateRange(entry, "coverage covered entries")
		if err != nil {
			return nil, err
		}
		if item.Count < 1 || item.Start < next || item.Start+item.Count > limit {
			return nil, fieldError("coverage covered ranges must be ascending, non-adjacent, and inside newRange")
		}
		covered = append(covered, item)
		next = item.Start + item.Count + 1
	}
	return covered, nil
}

func validateBasis(entry Value) (Basis, error) {
	if entry.Kind != KindObject {
		return Basis{}, fieldError("basis entries must be objects")
	}
	if err := requireClosedKeys(entry.Obj, []string{"evidenceId", "relation"}); err != nil {
		return Basis{}, err
	}
	id, _ := entry.Obj.Get("evidenceId")
	relation, _ := entry.Obj.Get("relation")
	if id.Kind != KindString || !isPrefixedDigest(id.Str, EvidencePrefix) {
		return Basis{}, fieldError("basis evidenceId must match %sDIGEST", EvidencePrefix)
	}
	if relation.Kind != KindString || !Relations[relation.Str] {
		return Basis{}, fieldError("basis relation must be a registered relation")
	}
	return Basis{EvidenceID: id.Str, Relation: relation.Str}, nil
}

func validateDisposition(hunk Hunk, spec string) error {
	switch hunk.Disposition {
	case "supported":
		if len(hunk.Basis) == 0 {
			return cemcode.New(cemcode.UnsupportedWithoutBasis, "supported hunks require at least one basis")
		}
		if hunk.Reason != "evidence-backed" {
			return fieldError("supported hunks require reason evidence-backed")
		}
	case "unknown":
		if len(hunk.Basis) != 0 || !unknownReasons[hunk.Reason] {
			return fieldError("unknown hunks require an empty basis and an unknown reason")
		}
	case "mechanical":
		if len(hunk.Basis) != 0 || !MechanicalReason(spec, hunk.Reason) {
			return fieldError("mechanical hunks require an empty basis and a mechanical reason")
		}
	default:
		return fieldError("disposition must be supported, unknown, or mechanical")
	}
	return nil
}

func validateReferences(result *Map) error {
	ids := map[string]bool{}
	cited := map[string]bool{}
	for _, record := range result.Evidence {
		if ids[record.ID] {
			return fieldError("duplicate evidence id %s", record.ID)
		}
		ids[record.ID] = true
	}
	for _, hunk := range result.Hunks {
		for _, basis := range hunk.Basis {
			if !ids[basis.EvidenceID] {
				return fieldError("basis references unknown evidence %s", basis.EvidenceID)
			}
			cited[basis.EvidenceID] = true
		}
	}
	for _, record := range result.Evidence {
		if !cited[record.ID] {
			return cemcode.New(cemcode.OrphanEvidence, "evidence %s is never cited", record.ID)
		}
	}
	return nil
}

// IsSha256 reports whether text is exactly 64 lowercase hex bytes.
func IsSha256(text string) bool { return isLowerHex(text) && len(text) == 64 }

// IsGitOid reports whether text is a full 40- or 64-hex lowercase Git OID.
func IsGitOid(text string) bool {
	return isLowerHex(text) && (len(text) == 40 || len(text) == 64)
}

func isLowerHex(text string) bool {
	if len(text) == 0 {
		return false
	}
	for _, c := range []byte(text) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func isPrefixedDigest(text, prefix string) bool {
	return strings.HasPrefix(text, prefix) && IsSha256(text[len(prefix):])
}

// ValidatePath enforces the frozen relative POSIX path grammar.
func ValidatePath(path string) error {
	if len(path) == 0 || len(path) > MaxPathBytes {
		return fieldError("path must be 1..%d UTF-8 bytes", MaxPathBytes)
	}
	if strings.HasPrefix(path, "/") {
		return cemcode.New(cemcode.PathTraversal, "absolute paths are invalid")
	}
	for _, c := range []byte(path) {
		if c == '\\' || c < 0x20 || c == 0x7f {
			return cemcode.New(cemcode.PathTraversal, "path contains a forbidden byte")
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return cemcode.New(cemcode.PathTraversal, "path contains an empty, dot, or dot-dot segment")
		}
	}
	return nil
}
