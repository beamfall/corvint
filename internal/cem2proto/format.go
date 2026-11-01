package cem2proto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ProtoSpec is the prototype's header label. It is deliberately not cem/2.0,
// which stays unclaimed until the wire is codified (CEM2-V0-016).
const ProtoSpec = "cem/2.0-proto"

// Reasons and Relations are the closed code tables (CEM2-V0-008): the cem/1.0
// reason and relation sets in a frozen order. A body stores the index.
var (
	Reasons   = []string{"evidence-backed", "no-evidence", "insufficient-evidence", "conflicting-evidence", "whitespace-only", "line-ending-only", "rename", "move", "import-reorder", "formatter-only"}
	Relations = []string{"specification", "decision", "test-claim", "implementation", "call-site", "dependency", "incident"}
)

// Local error codes (not cemcode registry entries).
const (
	CodeBodySha256Mismatch  = "cem2-body-sha256-mismatch"
	CodePatchSha256Mismatch = "cem2-patch-sha256-mismatch"
	CodeHunkCountMismatch   = "cem2-hunk-count-mismatch"
	CodeIndexOutOfRange     = "cem2-index-out-of-range"
	CodeCodeOutOfRange      = "cem2-code-out-of-range"
	CodeUnsupportedCritical = "cem2-unsupported-critical"
	CodeUnsupported         = "cem2-unsupported"
	CodePinnedMismatch      = "cem2-pinned-mismatch"
	CodeMalformed           = "cem2-malformed"
	CodeEvidenceUnresolved  = "cem2-evidence-unresolved"
	CodeBoundExceeded       = "cem2-bound-exceeded"
)

// Error is a typed prototype refusal or abstention.
type Error struct{ Code, Detail string }

func (e *Error) Error() string { return e.Code + ": " + e.Detail }

// Unsupported reports an abstention (UNSUPPORTED) rather than a refusal.
func (e *Error) Unsupported() bool {
	return e.Code == CodeUnsupported || e.Code == CodeUnsupportedCritical
}

func fail(code, format string, args ...any) *Error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// Header is the signed unit (CEM2-V0-003). The target commit stays external.
type Header struct {
	Spec        string
	Base        string
	PatchSha256 string
	BodySha256  string
	Role        string
	Parents     []string
	Critical    []string
}

// EvidenceRow is one cited span: a path index and a byte span, plus the blob
// and span digest when the row is pinned.
type EvidenceRow struct {
	Path       int
	Start, End int64
	BlobOid    string
	SpanSha256 string
}

// Pinned reports whether the row carries producer-asserted digests.
func (e EvidenceRow) Pinned() bool { return e.BlobOid != "" }

// BasisRow cites one evidence row under one relation code.
type BasisRow struct{ Evidence, Relation int }

// HunkRow is the claim for one canonical-patch hunk, by ordinal.
type HunkRow struct {
	Reason int
	Basis  []BasisRow
}

// Body is the index-based body (CEM2-V0-004).
type Body struct {
	Paths      []string
	Evidence   []EvidenceRow
	Hunks      []HunkRow
	Provenance *wire.StableProvenance
}

var (
	headerKeys    = []string{"spec", "base", "patchSha256", "bodySha256", "role", "parents", "critical"}
	bodyKeys      = []string{"paths", "evidence", "hunks"}
	bodyOptional  = []string{"provenance"}
	roles         = map[string]bool{wire.StableRoleChange: true, wire.StableRoleBindChore: true, wire.StableRoleSealChore: true}
	maxBodyMember = wire.MaxEvidence
)

// canonical renders v (plain Go JSON values) in the wire canonical form.
func canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	return wire.CanonicalValue(parsed), nil
}

// EncodeHeader renders a header canonically.
func EncodeHeader(h Header) ([]byte, error) {
	parents, critical := h.Parents, h.Critical
	if parents == nil {
		parents = []string{}
	}
	if critical == nil {
		critical = []string{}
	}
	return canonical(map[string]any{"spec": h.Spec, "base": h.Base, "patchSha256": h.PatchSha256, "bodySha256": h.BodySha256, "role": h.Role, "parents": parents, "critical": critical})
}

// EncodeBody renders a body canonically. Rows keep their order.
func EncodeBody(b Body) ([]byte, error) {
	paths := b.Paths
	if paths == nil {
		paths = []string{}
	}
	evidence := make([]any, 0, len(b.Evidence))
	for _, e := range b.Evidence {
		row := []any{e.Path, e.Start, e.End}
		if e.Pinned() {
			row = append(row, e.BlobOid, e.SpanSha256)
		}
		evidence = append(evidence, row)
	}
	hunks := make([]any, 0, len(b.Hunks))
	for _, h := range b.Hunks {
		row := []any{h.Reason}
		for _, basis := range h.Basis {
			row = append(row, []any{basis.Evidence, basis.Relation})
		}
		hunks = append(hunks, row)
	}
	doc := map[string]any{"paths": paths, "evidence": evidence, "hunks": hunks}
	if b.Provenance != nil {
		doc["provenance"] = b.Provenance
	}
	return canonical(doc)
}

// Sha256Hex is the lowercase hex sha256 of data.
func Sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// parseCanonical parses raw and refuses any non-canonical spelling.
func parseCanonical(raw []byte, what string) (wire.Value, error) {
	v, err := wire.Parse(raw)
	if err != nil {
		return wire.Value{}, fail(CodeMalformed, "%s: %v", what, err)
	}
	if !bytes.Equal(wire.CanonicalValue(v), raw) {
		return wire.Value{}, fail(CodeMalformed, "%s is not canonical", what)
	}
	if v.Kind != wire.KindObject {
		return wire.Value{}, fail(CodeMalformed, "%s must be an object", what)
	}
	return v, nil
}

func closedKeys(o *wire.Object, required, optional []string, what string) error {
	allowed := map[string]bool{}
	for _, k := range required {
		if _, ok := o.Get(k); !ok {
			return fail(CodeMalformed, "%s lacks member %s", what, k)
		}
		allowed[k] = true
	}
	for _, k := range optional {
		allowed[k] = true
	}
	for _, k := range o.Keys {
		if !allowed[k] {
			return fail(CodeMalformed, "%s has unknown member %s", what, k)
		}
	}
	return nil
}

func stringArray(v wire.Value, what string) ([]string, error) {
	if v.Kind != wire.KindArray {
		return nil, fail(CodeMalformed, "%s must be an array", what)
	}
	out := make([]string, 0, len(v.Arr))
	for _, item := range v.Arr {
		if item.Kind != wire.KindString {
			return nil, fail(CodeMalformed, "%s holds a non-string", what)
		}
		out = append(out, item.Str)
	}
	return out, nil
}

func str(o *wire.Object, key string) string {
	v, _ := o.Get(key)
	if v.Kind != wire.KindString {
		return "\x00"
	}
	return v.Str
}

// DecodeHeader parses and validates a header. An unknown critical entry and
// a non-empty parents list are abstentions (CEM2-V0-007, CEM2-V0-018).
func DecodeHeader(raw []byte) (Header, error) {
	v, err := parseCanonical(raw, "header")
	if err != nil {
		return Header{}, err
	}
	if err := closedKeys(v.Obj, headerKeys, nil, "header"); err != nil {
		return Header{}, err
	}
	h := Header{Spec: str(v.Obj, "spec"), Base: str(v.Obj, "base"), PatchSha256: str(v.Obj, "patchSha256"), BodySha256: str(v.Obj, "bodySha256"), Role: str(v.Obj, "role")}
	switch {
	case h.Spec != ProtoSpec:
		return Header{}, fail(CodeUnsupported, "header spec %q", h.Spec)
	case !wire.IsGitOid(h.Base):
		return Header{}, fail(CodeMalformed, "header base")
	case !wire.IsSha256(h.PatchSha256):
		return Header{}, fail(CodeMalformed, "header patchSha256")
	case !wire.IsSha256(h.BodySha256):
		return Header{}, fail(CodeMalformed, "header bodySha256")
	case !roles[h.Role]:
		return Header{}, fail(CodeMalformed, "header role")
	}
	parents, _ := v.Obj.Get("parents")
	critical, _ := v.Obj.Get("critical")
	if h.Parents, err = stringArray(parents, "header parents"); err != nil {
		return Header{}, err
	}
	if h.Critical, err = stringArray(critical, "header critical"); err != nil {
		return Header{}, err
	}
	// The extension registry is empty, so every critical entry is unknown.
	if len(h.Critical) > 0 {
		return Header{}, fail(CodeUnsupportedCritical, "critical extension %q is not implemented", h.Critical[0])
	}
	if len(h.Parents) > 0 {
		return Header{}, fail(CodeUnsupported, "delta maps (non-empty parents) are a deferral")
	}
	return h, nil
}

func intIn(v wire.Value, limit int, code, what string) (int, error) {
	if v.Kind != wire.KindInt {
		return 0, fail(CodeMalformed, "%s must be an integer", what)
	}
	if v.Int < 0 || v.Int >= int64(limit) {
		return 0, fail(code, "%s %d outside [0,%d)", what, v.Int, limit)
	}
	return int(v.Int), nil
}

// DecodeBody parses and validates a body: closed members, canonical bytes,
// bounds, and every index and code in range (CEM2-V0-004, -007, -010).
func DecodeBody(raw []byte) (Body, error) {
	if len(raw) > wire.MaxMapBytes {
		return Body{}, fail(CodeBoundExceeded, "body is %d bytes; limit %d", len(raw), wire.MaxMapBytes)
	}
	v, err := parseCanonical(raw, "body")
	if err != nil {
		return Body{}, err
	}
	if err := closedKeys(v.Obj, bodyKeys, bodyOptional, "body"); err != nil {
		return Body{}, err
	}
	var b Body
	pathsV, _ := v.Obj.Get("paths")
	if b.Paths, err = stringArray(pathsV, "body paths"); err != nil {
		return Body{}, err
	}
	if len(b.Paths) > maxBodyMember {
		return Body{}, fail(CodeBoundExceeded, "%d paths; limit %d", len(b.Paths), maxBodyMember)
	}
	seen := map[string]bool{}
	for _, p := range b.Paths {
		if len(p) > wire.MaxPathBytes {
			return Body{}, fail(CodeBoundExceeded, "path is %d bytes; limit %d", len(p), wire.MaxPathBytes)
		}
		if err := wire.ValidatePath(p); err != nil || seen[p] {
			return Body{}, fail(CodeMalformed, "body path %q invalid or repeated", p)
		}
		seen[p] = true
	}
	evidenceV, _ := v.Obj.Get("evidence")
	if evidenceV.Kind != wire.KindArray {
		return Body{}, fail(CodeMalformed, "body evidence must be an array")
	}
	if len(evidenceV.Arr) > wire.MaxEvidence {
		return Body{}, fail(CodeBoundExceeded, "%d evidence rows; limit %d", len(evidenceV.Arr), wire.MaxEvidence)
	}
	for i, row := range evidenceV.Arr {
		if row.Kind != wire.KindArray || (len(row.Arr) != 3 && len(row.Arr) != 5) {
			return Body{}, fail(CodeMalformed, "evidence row %d must have 3 or 5 items", i)
		}
		var e EvidenceRow
		if e.Path, err = intIn(row.Arr[0], len(b.Paths), CodeIndexOutOfRange, "evidence path index"); err != nil {
			return Body{}, err
		}
		if row.Arr[1].Kind != wire.KindInt || row.Arr[2].Kind != wire.KindInt || row.Arr[1].Int > row.Arr[2].Int {
			return Body{}, fail(CodeMalformed, "evidence row %d span", i)
		}
		e.Start, e.End = row.Arr[1].Int, row.Arr[2].Int
		if len(row.Arr) == 5 {
			if row.Arr[3].Kind != wire.KindString || !wire.IsGitOid(row.Arr[3].Str) || row.Arr[4].Kind != wire.KindString || !wire.IsSha256(row.Arr[4].Str) {
				return Body{}, fail(CodeMalformed, "evidence row %d pinned digests", i)
			}
			e.BlobOid, e.SpanSha256 = row.Arr[3].Str, row.Arr[4].Str
		}
		b.Evidence = append(b.Evidence, e)
	}
	hunksV, _ := v.Obj.Get("hunks")
	if hunksV.Kind != wire.KindArray {
		return Body{}, fail(CodeMalformed, "body hunks must be an array")
	}
	if len(hunksV.Arr) > wire.MaxHunks {
		return Body{}, fail(CodeBoundExceeded, "%d hunk rows; limit %d", len(hunksV.Arr), wire.MaxHunks)
	}
	for i, row := range hunksV.Arr {
		if row.Kind != wire.KindArray || len(row.Arr) == 0 {
			return Body{}, fail(CodeMalformed, "hunk row %d must be a non-empty array", i)
		}
		var h HunkRow
		if h.Reason, err = intIn(row.Arr[0], len(Reasons), CodeCodeOutOfRange, "hunk reason code"); err != nil {
			return Body{}, err
		}
		for _, pair := range row.Arr[1:] {
			if pair.Kind != wire.KindArray || len(pair.Arr) != 2 {
				return Body{}, fail(CodeMalformed, "hunk row %d basis must be [evidence, relation]", i)
			}
			var basis BasisRow
			if basis.Evidence, err = intIn(pair.Arr[0], len(b.Evidence), CodeIndexOutOfRange, "basis evidence index"); err != nil {
				return Body{}, err
			}
			if basis.Relation, err = intIn(pair.Arr[1], len(Relations), CodeCodeOutOfRange, "basis relation code"); err != nil {
				return Body{}, err
			}
			h.Basis = append(h.Basis, basis)
		}
		b.Hunks = append(b.Hunks, h)
	}
	if pv, ok := v.Obj.Get("provenance"); ok {
		if pv.Kind != wire.KindObject {
			return Body{}, fail(CodeMalformed, "body provenance must be an object")
		}
		if err := closedKeys(pv.Obj, wire.StableProvenanceKeys, nil, "body provenance"); err != nil {
			return Body{}, err
		}
		b.Provenance = &wire.StableProvenance{SourceSpec: str(pv.Obj, "sourceSpec"), SourceBlobOid: str(pv.Obj, "sourceBlobOid"), SourceSha256: str(pv.Obj, "sourceSha256"), BindCommit: str(pv.Obj, "bindCommit"), Converter: str(pv.Obj, "converter")}
	}
	return b, nil
}

// disposition is a function of the reason (cem/1.0).
func disposition(reason string) string {
	switch {
	case reason == "evidence-backed":
		return "supported"
	case wire.MechanicalReason(reason):
		return "mechanical"
	default:
		return "unknown"
	}
}
