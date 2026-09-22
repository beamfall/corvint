// Package workflow implements the CEM producer/verifier operations —
// begin, prepare, cite, mark, status, verify, and report — over the CEM
// seams, with the frozen CEM-CB envelope table, validation precedence, and
// resume rules.
package workflow

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Default artifact locations beneath the per-worktree Git directory.
const (
	defaultPatchRelative  = "corvint/change.patch"
	defaultReportRelative = "corvint/cem-review.md"
	maxCitationSpanBytes  = 1 << 20
	maxReportBytes        = 8 << 20
)

// Session is one opened worktree root; the repository boundary is validated
// separately by openRepository at its frozen precedence position.
type Session struct {
	root       string
	workRoot   *publish.Root
	repository *gitauth.Repository
	gitRoot    *publish.Root
}

// Open validates the worktree root as a publication root only. Repository
// validation (precedence stages 5–6) is deferred to openRepository so each
// command judges argument and map validation (stages 1–4) first.
func Open(root string) (*Session, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, cemcode.New(cemcode.RepositoryObjectUnavailable, "root path cannot be made absolute")
	}
	workRoot, err := publish.OpenRoot(absolute)
	if err != nil {
		return nil, err
	}
	return &Session{root: absolute, workRoot: workRoot}, nil
}

// openRepository runs precedence stages 5–6: repository-root and reciprocal
// worktree validation, then alternate and attribute denial. Every command
// calls it before its first Git operation or publication.
func (s *Session) openRepository() error {
	if s.repository != nil {
		return nil
	}
	repository, err := gitauth.Open(s.root, gitrun.NewDefaultBudget())
	if err != nil {
		return err
	}
	gitRoot, err := publish.OpenRoot(repository.GitDir)
	if err != nil {
		return err
	}
	s.repository, s.gitRoot = repository, gitRoot
	return nil
}

func invalidArguments(format string, args ...any) *cemcode.Error {
	return cemcode.New(cemcode.InvalidArguments, format, args...)
}

// --- document model helpers ---

// buildCandidate constructs a fresh map document for the given profile with
// every hunk explicitly unknown.
func buildCandidate(spec, base string, patchBytes []byte, parsed *patch.Patch) *wire.Map {
	digest := sha256.Sum256(patchBytes)
	document := &wire.Map{Spec: spec, BaseRevision: base, PatchSha256: hex.EncodeToString(digest[:])}
	if wire.Canonical(spec) {
		document.ExcludedPath = wire.ExcludedCEMPath
	}
	for _, hunk := range parsed.Hunks {
		document.Hunks = append(document.Hunks, wire.Hunk{
			ID: hunk.ID, Path: hunk.DisplayPath, OldRange: hunk.OldRange, NewRange: hunk.NewRange,
			Disposition: "unknown", Reason: "no-evidence",
		})
	}
	return document
}

// encodeMap renders a map document exactly as the Python oracle does, which is
// json.dumps(indent=2, sort_keys=True, ensure_ascii=False) plus a trailing
// newline. The bytes matter beyond readability: OCM binds a map by mapSha256, so
// a map written by one runtime must digest identically to the other's.
func encodeMap(document *wire.Map) []byte {
	evidence := make([]any, 0, len(document.Evidence))
	for _, record := range document.Evidence {
		evidence = append(evidence, map[string]any{
			"blobOid": record.BlobOid, "id": record.ID, "path": record.Path,
			"span":       map[string]any{"end": record.Span.End, "start": record.Span.Start},
			"spanSha256": record.SpanSha256,
		})
	}
	value := map[string]any{
		"baseRevision": document.BaseRevision,
		"evidence":     evidence,
		"hunks":        documentHunks(document),
		"patchSha256":  document.PatchSha256,
		"spec":         document.Spec,
	}
	if wire.Canonical(document.Spec) {
		value["excludedPath"] = document.ExcludedPath
	}
	return indentedCanonicalJSON(value)
}

// indentedCanonicalJSON matches Python's json.dumps(indent=2, sort_keys=True,
// ensure_ascii=False) followed by a newline. encoding/json already sorts map
// keys; SetEscapeHTML(false) is what stops Go escaping <, > and & where Python
// does not. It still escapes the line- and paragraph-separator runes (code
// points 0x2028 and 0x2029) unconditionally, a JS-safety carve-out with no
// opt-out, where Python's ensure_ascii=False leaves both unescaped, so
// unescapeLineSeparators undoes exactly that.
func indentedCanonicalJSON(value any) []byte {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil
	}
	return unescapeLineSeparators(buffer.Bytes())
}

// lineSeparatorTail and paragraphSeparatorTail are the 5 ASCII bytes that
// follow the backslash in encoding/json's escape of the line- and
// paragraph-separator runes ("u2028"/"u2029"); lineSeparatorRune and
// paragraphSeparatorRune are those same runes' literal UTF-8 encoding. Built
// from byte and code-point values rather than source escape sequences so this
// file never contains one.
var (
	lineSeparatorTail      = []byte{'u', '2', '0', '2', '8'}
	paragraphSeparatorTail = []byte{'u', '2', '0', '2', '9'}
	lineSeparatorRune      = []byte(string(rune(0x2028)))
	paragraphSeparatorRune = []byte(string(rune(0x2029)))
)

// unescapeLineSeparators restores the line- and paragraph-separator runes to
// their raw UTF-8 bytes wherever encoding/json escaped them. It counts each
// run of consecutive backslashes rather than matching the escape as a fixed
// substring: a source string that itself contains a literal backslash
// immediately before the literal text "u2028" is escaped by the encoder to a
// doubled backslash followed by that text, and the trailing single backslash
// of that pair would otherwise look identical to a genuine rune escape. Only
// an odd-length run ending right before the tail is the encoder's own escape;
// an even-length run is that many literal backslashes plus unescaped literal
// text, which must pass through unchanged. internal/gokernel/canonical.go
// restoreJSONSeparators solves the identical Python-json.dumps parity problem
// this way; this mirrors that algorithm rather than a plain byte replace.
func unescapeLineSeparators(data []byte) []byte {
	backslash := byte(92)
	output := make([]byte, 0, len(data))
	for index := 0; index < len(data); {
		if data[index] != backslash {
			output = append(output, data[index])
			index++
			continue
		}
		start := index
		for index < len(data) && data[index] == backslash {
			index++
		}
		run := index - start
		var separator []byte
		if run%2 == 1 && index+5 <= len(data) {
			tail := data[index : index+5]
			switch {
			case bytes.Equal(tail, lineSeparatorTail):
				separator = lineSeparatorRune
			case bytes.Equal(tail, paragraphSeparatorTail):
				separator = paragraphSeparatorRune
			}
		}
		if separator == nil {
			output = append(output, data[start:index]...)
			continue
		}
		output = append(output, data[start:index-1]...)
		output = append(output, separator...)
		index += 5
	}
	return output
}

// --- counts, worklist, policy ---

func countsAndWorklist(document *wire.Map) (map[string]any, []any) {
	counts := map[string]any{"total": 0, "supported": 0, "unknown": 0, "mechanical": 0}
	work := []any{}
	for index, hunk := range document.Hunks {
		counts["total"] = counts["total"].(int) + 1
		if current, ok := counts[hunk.Disposition].(int); ok {
			counts[hunk.Disposition] = current + 1
		}
		next := map[string]string{
			"supported": "done", "unknown": "cite-or-mark", "mechanical": "review-mechanical",
		}[hunk.Disposition]
		work = append(work, map[string]any{
			"selector": index + 1, "id": hunk.ID, "path": hunk.Path,
			"oldRange": rangeValue(hunk.OldRange), "newRange": rangeValue(hunk.NewRange),
			"disposition": hunk.Disposition, "reason": hunk.Reason, "next": next,
		})
	}
	return counts, work
}

// documentHunks reports hunks in the shape they are stored in the map. begin
// reports the document it just wrote, not the worklist: the worklist adds
// selector/next and drops basis, which belong to status alone.
func documentHunks(document *wire.Map) []any {
	hunks := make([]any, 0, len(document.Hunks))
	for _, hunk := range document.Hunks {
		basis := make([]any, 0, len(hunk.Basis))
		for _, item := range hunk.Basis {
			basis = append(basis, map[string]any{"evidenceId": item.EvidenceID, "relation": item.Relation})
		}
		hunks = append(hunks, map[string]any{
			"basis": basis, "disposition": hunk.Disposition, "id": hunk.ID,
			"newRange": rangeValue(hunk.NewRange), "oldRange": rangeValue(hunk.OldRange),
			"path": hunk.Path, "reason": hunk.Reason,
		})
	}
	return hunks
}

func rangeValue(value wire.Range) map[string]any {
	return map[string]any{"start": value.Start, "count": value.Count}
}

func firstOpen(work []any) any {
	for _, entry := range work {
		if entry.(map[string]any)["next"] != "done" {
			return entry
		}
	}
	return nil
}

// PolicyLimits are optional disposition-count ceilings.
type PolicyLimits struct {
	MaxUnknown    *int
	MaxMechanical *int
}

func policyIssues(limits PolicyLimits, counts map[string]any) []any {
	issues := []any{}
	for _, policy := range []struct {
		limit *int
		field string
		code  string
	}{{limits.MaxUnknown, "unknown", "max-unknown-exceeded"}, {limits.MaxMechanical, "mechanical", "max-mechanical-exceeded"}} {
		if policy.limit == nil {
			continue
		}
		actual := counts[policy.field].(int)
		if actual > *policy.limit {
			issues = append(issues, map[string]any{
				"code": policy.code, "disposition": policy.field, "actual": actual,
				"maximum": *policy.limit, "message": policy.field + " hunk count exceeds policy maximum",
			})
		}
	}
	return issues
}

// --- verification envelopes ---

func successVerification(document *wire.Map, outcome *verify.Outcome, driftRejected bool) map[string]any {
	result := map[string]any{
		"valid": !driftRejected, "spec": document.Spec, "baseRevision": outcome.BaseRevision,
		"targetRevision": nullableText(outcome.TargetRevision), "patchSha256": document.PatchSha256,
		"hunksTotal": len(document.Hunks), "evidenceTotal": len(document.Evidence),
		"issues": []any{}, "drift": driftRows(document, outcome),
	}
	if driftRejected {
		result["issues"] = []any{map[string]any{"code": cemcode.EvidenceDrift, "message": "CEM evidence changed at target"}}
	}
	if wire.Canonical(document.Spec) {
		result["assurance"] = "canonical"
	}
	return result
}

// failureVerification renders a failing verification object. Per CEM-CB-014,
// "canonical" assurance may be claimed only after independent repository
// derivation and target binding completed; a failure before that point
// established structural assurance at most.
func failureVerification(spec string, err error, canonicallyBound bool) map[string]any {
	code := cemcode.CodeOf(err)
	message := ""
	if code == "" {
		code = "internal-error"
	}
	if typed, ok := err.(*cemcode.Error); ok {
		message = typed.Message
	}
	result := map[string]any{
		"valid": false, "spec": spec, "baseRevision": "", "targetRevision": nil,
		"patchSha256": "", "hunksTotal": 0, "evidenceTotal": 0,
		"issues": []any{map[string]any{"code": code, "message": message}}, "drift": []any{},
	}
	if wire.Canonical(spec) {
		result["assurance"] = "structural-only"
		if canonicallyBound {
			result["assurance"] = "canonical"
		}
	}
	return result
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// driftRows renders per-evidence drift rows sorted by evidence ID, retaining
// full context even when the target check rejects.
func driftRows(document *wire.Map, outcome *verify.Outcome) []any {
	if outcome == nil || outcome.TargetRevision == "" {
		return []any{}
	}
	byID := map[string]wire.Evidence{}
	for _, record := range document.Evidence {
		byID[record.ID] = record
	}
	items := append([]verify.DriftItem(nil), outcome.Drift...)
	sort.Slice(items, func(left, right int) bool { return items[left].EvidenceID < items[right].EvidenceID })
	rows := []any{}
	for _, item := range items {
		record := byID[item.EvidenceID]
		row := map[string]any{
			"evidenceId": item.EvidenceID, "path": record.Path, "baseBlobOid": record.BlobOid,
			"status": string(item.Status), "targetBlobOid": nullableText(item.TargetBlobOid),
			"targetSpan": nil,
		}
		if item.TargetSpan != nil {
			row["targetSpan"] = map[string]any{"start": item.TargetSpan.Start, "end": item.TargetSpan.End}
		}
		rows = append(rows, row)
	}
	return rows
}

// --- shared input helpers ---

// readMapInput reads and validates a root-relative CEM map without following
// symlinks.
func (s *Session) readMapInput(relative string) ([]byte, *wire.Map, error) {
	if relative == "" {
		return nil, nil, invalidArguments("--map is required")
	}
	if filepath.IsAbs(relative) {
		return nil, nil, invalidArguments("map path must be repository-relative")
	}
	raw, err := s.workRoot.ReadBounded(relative, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, nil, err
	}
	document, err := wire.ParseMap(raw)
	if err != nil {
		return nil, nil, err
	}
	return raw, document, nil
}

// resolveHunk resolves a one-based worklist ordinal or a full derived hunk ID.
func resolveHunk(document *wire.Map, selector string) (int, error) {
	if selector == "" {
		return 0, invalidArguments("--hunk is required")
	}
	// CEM-PILOT-003: only a canonical one-based decimal is an ordinal; "01"
	// and "+1" are not, and resolve to no hunk.
	if strings.Trim(selector, "0123456789") == "" {
		if selector[0] == '0' {
			return 0, cemcode.New(cemcode.InvalidArguments, "decimal hunk selectors must be canonical and one-based")
		}
		ordinal, err := strconv.Atoi(selector)
		if err != nil || ordinal > len(document.Hunks) {
			return 0, cemcode.New(cemcode.InvalidArguments, "hunk selector is outside the worklist")
		}
		return ordinal - 1, nil
	}
	for index, hunk := range document.Hunks {
		if hunk.ID == selector {
			return index, nil
		}
	}
	return 0, cemcode.New(cemcode.UnknownHunkID, "hunk id must identify exactly one mapped hunk")
}
