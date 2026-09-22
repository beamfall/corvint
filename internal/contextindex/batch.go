package contextindex

import (
	"fmt"
	"sort"
	"strings"
)

// The SBQ-V0-008 verdicts. Every possessed entry gets exactly one, matched by
// literal repository-relative path against the loaded index's tables: no
// normalization, case folding, symlink resolution or Git process.
const (
	VerdictUnframable  = "unframable"
	VerdictUnsupported = "unsupported"
	VerdictDirty       = "dirty"
	VerdictStale       = "stale"
	VerdictRetained    = "retained"
	VerdictAbsent      = "absent"
)

// The SBQ-V0-010(d) fallback reasons. A fallback disables suppression for one
// whole operation; there is no partial fallback.
const (
	FallbackBudgetCompacted       = "budget-compacted"
	FallbackUnsupportedPossession = "unsupported-possession"
	FallbackMixedWorktree         = "mixed-worktree"
)

// PossessedEntry is one caller-supplied possession row (SBQ-V0-007(a)). The
// optional `line` is validated and ignored in V0, so it is not carried here: a
// stored line and one dropped after validation are indistinguishable on the
// wire.
type PossessedEntry struct {
	Path     string
	BlobHash string
}

// SuppressedResult is one result that left `results` for `delta.suppressed`
// (SBQ-V0-009(f)). Rows is the count of that result's evidence rows.
type SuppressedResult struct {
	Kind string
	ID   string
	Rows int
}

// InvalidatedResult is one (result, invalidation-matched path) pair
// (SBQ-V0-009(h)). It is the one delta list keyed without an operation.
type InvalidatedResult struct {
	Kind    string
	ID      string
	Path    string
	Verdict string
}

// PossessionOutcome is what one operation's possession list did to its receipt.
// A non-empty Fallback means suppression was disabled for that whole operation
// and every entry is echoed under Ignored.
type PossessionOutcome struct {
	Fallback    string
	Suppressed  []SuppressedResult
	Invalidated []InvalidatedResult
	Ignored     []PossessedEntry
}

// StabilizeReceipt re-runs the packet-byte fixed point over a receipt whose
// results changed (SBQ-V0-010(c)). It is the exported wrapper the batch verb
// uses: stabilizePacketBytes' own first statement is an unchecked type
// assertion on `coverage`, so an absent or mistyped coverage member must be a
// contextindex.Error here rather than a panic there.
func StabilizeReceipt(receipt map[string]any) error {
	if coverage, ok := receipt["coverage"].(map[string]any); !ok || coverage == nil {
		return &Error{Message: "context receipt carries no coverage object"}
	}
	return stabilizePacketBytes(receipt)
}

// PossessionVerdict verdicts one entry at the loaded snapshot (SBQ-V0-008).
// Precedence is (a) unframable, (b) unsupported, (c) dirty, (d) stale,
// (e) retained, (f) absent.
func PossessionVerdict(index *Index, entry PossessedEntry) string {
	if _, skipped := index.Skipped[entry.Path]; skipped {
		return VerdictUnframable
	}
	if directoryOfIndex(index, entry.Path) {
		return VerdictUnframable
	}
	_, tracked := index.Tracked[entry.Path]
	source, indexed := index.Sources[entry.Path]
	if tracked && !indexed {
		return VerdictUnsupported
	}
	if dirtyInIndex(index, entry.Path) {
		return VerdictDirty
	}
	if !indexed {
		return VerdictAbsent
	}
	if source.BlobHash != entry.BlobHash {
		return VerdictStale
	}
	return VerdictRetained
}

// directoryOfIndex reports whether the path is a proper slash-terminated prefix
// of a tracked path. `ls-tree -r` emits no tree entries, so a directory is only
// ever visible as a prefix of its descendants.
func directoryOfIndex(index *Index, value string) bool {
	prefix := value + "/"
	for skipped := range index.Skipped {
		if strings.HasPrefix(skipped, prefix) {
			return true
		}
	}
	for tracked := range index.Tracked {
		if len(tracked) > len(prefix) && tracked[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func dirtyInIndex(index *Index, value string) bool {
	for _, dirty := range index.DirtyPaths {
		if dirty == value {
			return true
		}
	}
	return false
}

// ApplyPossession suppresses the results whose evidence the caller already
// holds, in place, and reports what the possession list did (SBQ-V0-009,
// SBQ-V0-010). The receipt is the one the standalone verb returned, so no
// verb's code path changes.
func ApplyPossession(index *Index, receipt map[string]any, entries []PossessedEntry) (PossessionOutcome, error) {
	if index.Tracked == nil || index.Skipped == nil {
		return PossessionOutcome{}, &Error{
			Code:    "unsupported-possession-tables",
			Message: "the loaded index carries no path table: possession cannot be verdicted",
		}
	}
	verdicts := make([]string, len(entries))
	for position, entry := range entries {
		verdicts[position] = PossessionVerdict(index, entry)
	}
	results := receiptResults(receipt)
	evidencePaths := evidencePathSet(results)
	if reason := possessionFallback(index, receipt, entries, verdicts, evidencePaths); reason != "" {
		return PossessionOutcome{Fallback: reason, Ignored: append([]PossessedEntry(nil), entries...)}, nil
	}
	pairs := make(map[PossessedEntry]struct{}, len(entries))
	paths := make(map[string]string, len(entries))
	for position, entry := range entries {
		pairs[entry] = struct{}{}
		if verdicts[position] == VerdictStale {
			paths[entry.Path] = VerdictStale
		} else if _, seen := paths[entry.Path]; !seen {
			paths[entry.Path] = verdicts[position]
		}
	}
	critical := criticalSelectorSet(receipt)
	survivors := make([]map[string]any, 0, len(results))
	suppressed := make([]SuppressedResult, 0)
	suppressedResults := make([]map[string]any, 0)
	invalidated := make([]InvalidatedResult, 0)
	seenInvalid := make(map[InvalidatedResult]struct{})
	for _, result := range results {
		rows := evidenceRows(result)
		if resultSuppressible(result, rows, pairs, critical) {
			suppressed = append(suppressed, SuppressedResult{
				Kind: stringValue(result["kind"]), ID: stringValue(result["id"]), Rows: len(rows),
			})
			suppressedResults = append(suppressedResults, result)
			continue
		}
		survivors = append(survivors, result)
		for _, entry := range invalidationEntries(result, rows, pairs, paths) {
			if _, seen := seenInvalid[entry]; seen {
				continue
			}
			seenInvalid[entry] = struct{}{}
			invalidated = append(invalidated, entry)
		}
	}
	ignored := make([]PossessedEntry, 0)
	for _, entry := range entries {
		if _, matched := evidencePaths[entry.Path]; !matched {
			ignored = append(ignored, entry)
		}
	}
	outcome := PossessionOutcome{Suppressed: suppressed, Invalidated: invalidated, Ignored: ignored}
	if len(suppressed) == 0 {
		return outcome, nil
	}
	if err := narrowReceipt(index, receipt, survivors, suppressedResults, len(suppressed)); err != nil {
		return PossessionOutcome{}, err
	}
	return outcome, nil
}

// possessionFallback names the whole-operation fallback, if any
// (SBQ-V0-010(d)). unsupported-possession wins over mixed-worktree when both
// hold.
func possessionFallback(index *Index, receipt map[string]any, entries []PossessedEntry, verdicts []string, evidencePaths map[string]struct{}) string {
	for position, entry := range entries {
		if verdicts[position] != VerdictUnsupported {
			continue
		}
		if _, matched := evidencePaths[entry.Path]; matched {
			return FallbackUnsupportedPossession
		}
	}
	if request, ok := receipt["request"].(map[string]any); ok {
		if _, compacted := request["omitted_by_budget"]; compacted {
			return FallbackBudgetCompacted
		}
	}
	if len(index.DirtyPaths) != 0 {
		return FallbackMixedWorktree
	}
	return ""
}

// resultSuppressible holds when the result has at least one evidence row, every
// row is suppression-matched, and the result carries no critical selector
// (SBQ-V0-009(d,e)).
func resultSuppressible(result map[string]any, rows []map[string]any, pairs map[PossessedEntry]struct{}, critical map[string]struct{}) bool {
	if len(rows) == 0 {
		return false
	}
	if _, protected := critical[resultSelector(result)]; protected {
		return false
	}
	for _, row := range rows {
		if !suppressionMatched(row, pairs) {
			return false
		}
	}
	return true
}

// suppressionMatched holds when path and blob_hash both equal some entry's. A
// row with an empty blob_hash (an unguarded Sources miss) matches nothing,
// supplied hashes being non-empty (SBQ-V0-009(c)).
func suppressionMatched(row map[string]any, pairs map[PossessedEntry]struct{}) bool {
	hash := stringValue(row["blob_hash"])
	if hash == "" {
		return false
	}
	_, matched := pairs[PossessedEntry{Path: stringValue(row["path"]), BlobHash: hash}]
	return matched
}

// invalidationEntries lists the (result, invalidation-matched path) pairs a
// result carrying a stale row contributes (SBQ-V0-009(h)).
func invalidationEntries(result map[string]any, rows []map[string]any, pairs map[PossessedEntry]struct{}, paths map[string]string) []InvalidatedResult {
	matched := make([]InvalidatedResult, 0)
	carriesStale := false
	for _, row := range rows {
		hash := stringValue(row["blob_hash"])
		if hash == "" || suppressionMatched(row, pairs) {
			continue
		}
		verdict, possessed := paths[stringValue(row["path"])]
		if !possessed {
			continue
		}
		if verdict == VerdictStale {
			carriesStale = true
		}
		matched = append(matched, InvalidatedResult{
			Kind: stringValue(result["kind"]), ID: stringValue(result["id"]),
			Path: stringValue(row["path"]), Verdict: verdict,
		})
	}
	if !carriesStale {
		return nil
	}
	return matched
}

// narrowReceipt writes the suppression back into the receipt. Every coverage
// count, `state`, `exclusions` and `verification`'s non-path content freeze at
// their pre-suppression values (SBQ-V0-010(b)); only `suppressed_results`, the
// one appended uncertainty line, the verification path cap and the byte
// members move.
func narrowReceipt(index *Index, receipt map[string]any, survivors, suppressed []map[string]any, count int) error {
	narrowed := make([]any, len(survivors))
	for position, result := range survivors {
		narrowed[position] = result
	}
	receipt["results"] = narrowed
	coverage, ok := receipt["coverage"].(map[string]any)
	if !ok {
		return &Error{Message: "context receipt carries no coverage object"}
	}
	coverage["suppressed_results"] = count
	line := fmt.Sprintf("%d results suppressed by caller possession", count)
	coverage["uncertainty"] = append(anySlice(coverage["uncertainty"]), line)
	if _, present := receipt["verification"]; present {
		receipt["verification"] = verificationCommands(index, possessionVerificationPaths(survivors, suppressed))
	}
	if _, present := coverage["packet_bytes"]; !present {
		return nil
	}
	return restabilizeBudget(receipt, coverage)
}

// restabilizeBudget re-runs setCoverage's own order over the narrowed receipt:
// stabilize, encode, compare to budget_bytes, stabilize again. The comparison
// is reported truthfully, never assumed to stay true because suppression
// removed bytes (SBQ-V0-010(c)).
func restabilizeBudget(receipt map[string]any, coverage map[string]any) error {
	if err := StabilizeReceipt(receipt); err != nil {
		return err
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		return &Error{Message: "cannot encode context receipt"}
	}
	budget, bounded := coverage["budget_bytes"].(int)
	coverage["within_budget"] = !bounded || len(encoded) <= budget
	return StabilizeReceipt(receipt)
}

// possessionVerificationPaths recomputes the 20-path cap over the surviving
// results' evidence paths alone, with any suppressed path's contribution
// appended after them, so a retained result's own path is never displaced from
// the cap by a suppressed one (SBQ-V0-010(b)).
func possessionVerificationPaths(survivors, suppressed []map[string]any) []string {
	ordered := verificationPaths(survivors)
	seen := make(map[string]struct{}, len(ordered))
	for _, value := range ordered {
		seen[value] = struct{}{}
	}
	for _, value := range verificationPaths(suppressed) {
		if _, present := seen[value]; present {
			continue
		}
		seen[value] = struct{}{}
		ordered = append(ordered, value)
	}
	if len(ordered) > 20 {
		ordered = ordered[:20]
	}
	return ordered
}

// criticalSelectorSet renders each verb's critical selectors into one
// comparable form: `query` and `impact` key on "kind:id" strings, `context` on
// {"relation", "path"} objects (SBQ-V0-010(a)).
func criticalSelectorSet(receipt map[string]any) map[string]struct{} {
	coverage, ok := receipt["coverage"].(map[string]any)
	if !ok {
		return map[string]struct{}{}
	}
	selectors := make(map[string]struct{})
	for _, raw := range anySlice(coverage["critical"]) {
		if object, isObject := raw.(map[string]any); isObject {
			selectors[selector(object["relation"], object["path"])] = struct{}{}
			continue
		}
		selectors[stringValue(raw)] = struct{}{}
	}
	return selectors
}

func resultSelector(result map[string]any) string {
	return selector(result["kind"], result["id"])
}

func receiptResults(receipt map[string]any) []map[string]any {
	return mapsFromAny(receipt["results"])
}

func evidenceRows(result map[string]any) []map[string]any {
	rows := make([]map[string]any, 0)
	for _, raw := range anySlice(result["evidence"]) {
		if row, ok := raw.(map[string]any); ok {
			rows = append(rows, row)
		}
	}
	return rows
}

func evidencePathSet(results []map[string]any) map[string]struct{} {
	paths := make(map[string]struct{})
	for _, result := range results {
		for _, row := range evidenceRows(result) {
			paths[stringValue(row["path"])] = struct{}{}
		}
	}
	return paths
}

// SortInvalidated orders delta.invalidated by (kind, id, path) alone. It is the
// one delta list exempt from operation order (SBQ-V0-010(f)).
func SortInvalidated(entries []InvalidatedResult) {
	sort.SliceStable(entries, func(left, right int) bool {
		if entries[left].Kind != entries[right].Kind {
			return entries[left].Kind < entries[right].Kind
		}
		if entries[left].ID != entries[right].ID {
			return entries[left].ID < entries[right].ID
		}
		return entries[left].Path < entries[right].Path
	})
}
