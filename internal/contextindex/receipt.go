package contextindex

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

const (
	// MinPacketBytes is the smallest budget that can hold the worst-case
	// mandatory abstaining envelope: a mixed-worktree query packet whose
	// abstention.reason, learning block, compacted intent, and 64-hex-char
	// sha256-format revision are all at their longest mandatory shape,
	// rounded up to the next 64-byte boundary for headroom. See
	// TestMinPacketBytesIsTheWorstCaseAbstainingEnvelopeRoundedUp in
	// receipt_budget_test.go, which re-derives this value from that
	// construction and fails if the mandatory field set grows past it.
	MinPacketBytes = 1_216
	MaxPacketBytes = 1_000_000
)

func receipt(index *Index, mode string, request map[string]any, rawResults []map[string]any, limit int, requestedState string, extraUncertainty ...string) (map[string]any, error) {
	// Coverage is denominated in the universe admitted, never in the output
	// produced.  The ranking ceiling below narrows `rawResults` in place, so a
	// count taken after it makes `omitted_results` structurally zero and lets
	// the receipt assert a completeness it never measured.
	admittedResults := len(rawResults)
	if len(rawResults) > limit {
		rawResults = rawResults[:limit]
	}
	results := make([]any, len(rawResults))
	for position, result := range rawResults {
		results[position] = result
	}
	state := requestedState
	if state == "" {
		if len(results) == 0 {
			state = "OUT_OF_SCOPE"
		} else {
			state = "READY"
		}
	}
	freshnessState := "fresh"
	if len(index.DirtyPaths) != 0 {
		freshnessState = "mixed-worktree"
	}
	mixedPaths := index.DirtyPaths
	if len(mixedPaths) > maxExclusionSamples {
		mixedPaths = mixedPaths[:maxExclusionSamples]
	}
	exclusionSamples := make([]any, 0, min(len(index.Exclusions), maxExclusionSamples))
	for _, exclusion := range index.Exclusions[:min(len(index.Exclusions), maxExclusionSamples)] {
		exclusionSamples = append(exclusionSamples, map[string]any{"path": exclusion.Path, "reason": exclusion.Reason})
	}
	result := map[string]any{
		"schema_version": 1, "mode": mode, "request": request, "state": state,
		"revision": index.Revision,
		"freshness": map[string]any{
			"state": freshnessState, "scope": "git", "revision": index.Revision,
			"mixed_paths": mixedPaths, "mixed_path_count": len(index.DirtyPaths),
		},
		"results":      results,
		"exclusions":   map[string]any{"count": len(index.Exclusions) + index.UnsupportedSuffixCount, "samples": exclusionSamples},
		"verification": verification(index, rawResults),
	}
	// Both keys appear only when there is something to report, so a receipt
	// over a repository that parses and extracts cleanly stays byte-identical
	// to one built before either field existed. A structurally always-zero key
	// is the shape that trains a reader to stop looking at it.
	//
	// They are separate because they answer different questions: `unparsed`
	// means a grammar refused the source outright, `extraction` means a scanner
	// walked it and could not finish.
	if len(index.Unparsed) != 0 {
		result["unparsed"] = map[string]any{"count": len(index.Unparsed), "samples": unparsedSamples(index.Unparsed)}
	}
	if summary, noted := extractionSummary(index); noted {
		result["extraction"] = summary
	}
	if err := setCoverage(result, admittedResults, criticalResults(mode, rawResults), nil, index, extraUncertainty...); err != nil {
		return nil, err
	}
	return result, nil
}

func unparsedSamples(unparsed []Unparsed) []any {
	samples := make([]any, 0, min(len(unparsed), maxExclusionSamples))
	for _, item := range unparsed[:min(len(unparsed), maxExclusionSamples)] {
		samples = append(samples, map[string]any{"path": item.Path, "facts": item.Facts, "reason": item.Reason})
	}
	return samples
}

func compileReceipt(receipt map[string]any, budget *int, index *Index, extraUncertainty ...string) (map[string]any, error) {
	requestedResults := mapsFromAny(receipt["results"])
	// A receipt reaching here may already have been narrowed by a ranking
	// ceiling upstream, which leaves its own admitted count in coverage.
	// Re-deriving `requested` from the results in hand would measure that
	// narrowed output instead and reinstate a structurally-zero
	// `omitted_results` on every already-ranked receipt this compiles.
	admittedResults := admittedResultCount(receipt, len(requestedResults))
	critical := criticalResults(stringValue(receipt["mode"]), requestedResults)
	if budget == nil {
		if err := setCoverage(receipt, admittedResults, critical, nil, index, extraUncertainty...); err != nil {
			return nil, err
		}
		return receipt, nil
	}
	if *budget < MinPacketBytes || *budget > MaxPacketBytes {
		return nil, &Error{Message: fmt.Sprintf("budget_bytes must be between %d and %d", MinPacketBytes, MaxPacketBytes)}
	}

	compact := cloneMap(receipt)
	request := cloneMap(compact["request"].(map[string]any))
	request["budget_bytes"] = *budget
	compact["request"] = request
	if err := compactBudgetEnvelope(compact); err != nil {
		return nil, err
	}

	selected := make([]map[string]any, 0, len(requestedResults))
	criticalSet := stringSetFromAny(critical)
	ordered := criticalFirst(requestedResults, criticalSet)
	overflow := make(map[string]struct{})
	for _, item := range ordered {
		key := selector(item["kind"], item["id"])
		_, isCritical := criticalSet[key]
		if len(overflow) != 0 && !isCritical {
			continue
		}
		trial := cloneMap(compact)
		trialResults := append(cloneResults(selected), cloneMap(item))
		trial["results"] = resultsToAny(trialResults)
		trial["verification"] = verification(index, trialResults)
		if err := setCoverage(trial, admittedResults, critical, budget, index, extraUncertainty...); err != nil {
			return nil, err
		}
		encoded, err := CanonicalJSON(trial)
		if err != nil {
			return nil, &Error{Message: "cannot encode context receipt"}
		}
		if len(encoded) <= *budget {
			selected = append(selected, item)
		} else if isCritical {
			overflow[key] = struct{}{}
		}
	}

	originalState := stringValue(receipt["state"])
	coverageListsCompacted := false
	for {
		compact["results"] = resultsToAny(selected)
		compact["verification"] = verification(index, selected)
		if err := setCoverage(compact, admittedResults, critical, budget, index, extraUncertainty...); err != nil {
			return nil, err
		}
		missing := anySlice(compact["coverage"].(map[string]any)["critical_missing"])
		switch {
		case len(overflow) != 0 || len(missing) != 0:
			compact["state"] = "CRITICAL_EVIDENCE_OVERFLOW"
		case len(selected) < len(requestedResults):
			compact["state"] = "BUDGETED"
		default:
			compact["state"] = originalState
		}
		if err := setCoverage(compact, admittedResults, critical, budget, index, extraUncertainty...); err != nil {
			return nil, err
		}
		if coverageListsCompacted {
			if err := compactCoverageLists(compact, *budget); err != nil {
				return nil, err
			}
			coverage := compact["coverage"].(map[string]any)
			if len(anySlice(coverage["uncertainty"])) == 0 {
				delete(coverage, "uncertainty")
				if err := stabilizePacketBytes(compact); err != nil {
					return nil, err
				}
			}
		}
		encoded, err := CanonicalJSON(compact)
		if err != nil {
			return nil, &Error{Message: "cannot encode context receipt"}
		}
		if len(encoded) <= *budget {
			return compact, nil
		}
		if len(selected) == 0 {
			request := compact["request"].(map[string]any)
			if _, compacted := request["omitted_by_budget"]; !compacted {
				digest, err := canonicalSHA256(request)
				if err != nil {
					return nil, err
				}
				omitted := 0
				for key := range request {
					if key != "budget_bytes" {
						omitted++
					}
				}
				compact["request"] = map[string]any{
					"budget_bytes": *budget, "omitted_by_budget": omitted, "request_sha256": digest,
				}
				continue
			}
			if !coverageListsCompacted {
				coverageListsCompacted = true
				continue
			}
			return nil, &Error{Message: fmt.Sprintf("budget_bytes=%d cannot contain the structured Corvint envelope", *budget)}
		}
		dropped := selected[len(selected)-1]
		selected = selected[:len(selected)-1]
		droppedKey := selector(dropped["kind"], dropped["id"])
		if _, isCritical := criticalSet[droppedKey]; isCritical {
			overflow[droppedKey] = struct{}{}
		}
	}
}

func compactBudgetEnvelope(receipt map[string]any) error {
	freshness := receipt["freshness"].(map[string]any)
	mixedPaths := sliceLength(freshness["mixed_paths"])
	compactFreshness := map[string]any{
		"state": freshness["state"], "scope": freshness["scope"],
		"mixed_path_count": freshness["mixed_path_count"],
	}
	if mixedPaths != 0 {
		digest, err := canonicalSHA256(freshness["mixed_paths"])
		if err != nil {
			return err
		}
		compactFreshness["mixed_paths_omitted_by_budget"] = mixedPaths
		compactFreshness["mixed_paths_sha256"] = digest
	}
	receipt["freshness"] = compactFreshness
	exclusions := receipt["exclusions"].(map[string]any)
	receipt["exclusions"] = map[string]any{
		"count": exclusions["count"], "samples_omitted_by_budget": sliceLength(exclusions["samples"]),
	}
	// `unparsed` and `extraction` carry the same {count, samples} shape as
	// `exclusions` and must be compacted with it. They are Go-only members, so
	// nothing in the oracle's envelope forced the question, and their samples
	// were charged against the evidence budget: on this repository's
	// `file-change` receipt the 925-byte `unparsed` list was the whole of a
	// 901-byte envelope excess, and it evicted the highest-ranked
	// project-owned result -- a `repository-spec` document scoring 825 -- in
	// favour of a `syntax` result scoring 775 that happened to be smaller.
	// A disclosure the caller cannot act on must not outbid the evidence it
	// is disclosing about; the count and the omission stay, the samples go.
	for _, name := range []string{"unparsed", "extraction"} {
		value, ok := receipt[name]
		if !ok {
			continue
		}
		member := value.(map[string]any)
		receipt[name] = map[string]any{
			"count": member["count"], "samples_omitted_by_budget": sliceLength(member["samples"]),
		}
	}
	receipt["results"] = []any{}
	receipt["verification"] = []any{}
	if value, ok := receipt["learning"]; ok {
		learning := value.(map[string]any)
		receipt["learning"] = map[string]any{
			"history_tip": learning["history_tip"], "history_digest": learning["history_digest"],
			"local_trace_state": learning["local_trace_state"], "local_trace_count": learning["local_trace_count"],
			"advisory_candidates": learning["advisory_candidates"],
		}
	}
	if value, ok := receipt["intent"]; ok {
		intent := value.(map[string]any)
		if intent["id"] == "repository" {
			delete(receipt, "intent")
		} else {
			receipt["intent"] = map[string]any{
				"id": intent["id"], "confidence": intent["confidence"],
				"matched_terms_count": sliceLength(intent["matched_terms"]),
			}
		}
	}
	if value, ok := receipt["abstention"]; ok {
		abstention := value.(map[string]any)
		switch {
		case receipt["state"] == "NEEDS_WIDENING":
			claims := anySlice(abstention["nearest_claims"])
			nearest := make([]any, 0, min(len(claims), 3))
			for _, raw := range claims[:min(len(claims), 3)] {
				claim := raw.(map[string]any)
				nearest = append(nearest, map[string]any{"selector": valueOr(claim["selector"], "")})
			}
			receipt["abstention"] = map[string]any{
				"active": true, "reason": valueOr(abstention["reason"], "needs-widening"), "nearest_claims": nearest,
			}
		default:
			// A withdrawn packet must still say what withdrew it. Compacting the
			// abstention to its `active` flag alone -- or dropping it outright
			// under a repository intent -- left `below-relevance-floor` and
			// `unindexed-worktree-changes` observable nowhere, which makes an
			// abstention indistinguishable from an empty result set.
			active, _ := abstention["active"].(bool)
			receipt["abstention"] = map[string]any{
				"active": active, "reason": valueOr(abstention["reason"], "none"),
			}
		}
	}
	return nil
}

// withheldAuthorityUncertainty names every citation whose accepted authority was
// withheld under `CF-V0-031`, one explicit entry each.  It is derived from the
// emitted evidence rather than carried alongside it, so it survives every path
// that rebuilds coverage -- `receipt`, and `compileReceipt` with or without a
// packet budget -- and can never fall out of step with the labels it explains.
func withheldAuthorityUncertainty(results []map[string]any) []any {
	named := make([]string, 0)
	for _, result := range results {
		for _, item := range mapsFromAny(result["evidence"]) {
			if item["authority"] != UnverifiedContractAuthority {
				continue
			}
			entry := stringValue(item["reason"]) + ": no caller-independent revision is available at which to resolve the cited decision status, so accepted authority is withheld"
			if !stringIn(named, entry) {
				named = append(named, entry)
			}
		}
	}
	sort.Strings(named)
	uncertainty := make([]any, 0, len(named)+2)
	for _, entry := range named {
		uncertainty = append(uncertainty, entry)
	}
	return uncertainty
}

// SyntaxOnlyUncertainty is the single deterministic line a query packet carries
// when nothing but the host language's syntax connects its results to the task.
const SyntaxOnlyUncertainty = "all results are syntax matches; no project-owned authority corroborates the task"

const unknownLanguageVerification = "no language-specific verification command is known for this repository"

// syntaxOnlyQueryPacket reports a query packet whose every cited, non-advisory
// result rests on `authority: syntax` alone.  Product invariant 3 ranks
// project-owned authority above syntax, so a packet none of whose results any
// spec, decision, document reference, or instruction corroborates has matched
// the task's words and nothing else.  Counting such results as
// `authoritative_results` states a corroboration the packet never measured,
// which is the invented certainty invariant 2 forbids.  A result carrying no
// evidence at all cannot be read as syntax-only and disqualifies the packet.
func syntaxOnlyQueryPacket(mode string, results []map[string]any) bool {
	if mode != "query" {
		return false
	}
	cited := 0
	for _, result := range results {
		if result["kind"] == "learned-path" {
			continue
		}
		evidence := mapsFromAny(result["evidence"])
		if len(evidence) == 0 {
			return false
		}
		for _, item := range evidence {
			if item["authority"] != SyntaxAuthority {
				return false
			}
		}
		cited++
	}
	return cited != 0
}

// admittedResultCount recovers the universe an earlier compilation stage
// admitted, from the coverage that stage recorded.  A receipt with no coverage
// yet was never ranked, so the results in hand are its whole universe; a
// recorded count below them cannot be a universe at all, and is refused rather
// than allowed to drive `omitted_results` negative.
func admittedResultCount(receipt map[string]any, present int) int {
	coverage, ok := receipt["coverage"].(map[string]any)
	if !ok {
		return present
	}
	recorded, ok := coverage["requested_results"].(int)
	if !ok || recorded < present {
		return present
	}
	return recorded
}

// reverseImportProfileGap counts the requested changed paths whose
// reverse-import universe the native Go impact profile cannot compute, so the
// coverage block can say so instead of reporting a complete answer over a
// smaller universe.
//
// `reverseImporters` resolves a changed path only through a rule `GPK-V0-027`
// names for its suffix -- `.go`, `.py`, and the web set. For any other suffix
// it resolves nothing, so the search is vacuous by construction rather than
// empty by observation. Reporting the shortfall is the whole repair: the engine
// cannot count what it did not compute, so `omitted_results` stays an honest
// count of the results the ranking ceiling dropped and this names the dimension
// that count does not cover.
//
// The count is keyed on `ImpactRuleNamed`, so widening the profile retires the
// disclosure for the widened suffixes in the same edit that computes them.
//
// Test paths are excluded because the reverse-import stage never runs for them
// in either runtime, so nothing is withheld there. Paths absent from
// `index.Sources` are excluded because no ranking stage ran on them at all.
func reverseImportProfileGap(index *Index, receipt map[string]any) int {
	if index == nil || stringValue(receipt["mode"]) != "impact" {
		return 0
	}
	request, ok := receipt["request"].(map[string]any)
	if !ok {
		return 0
	}
	gaps := 0
	for _, changedPath := range stringsField(request["paths"]) {
		if ImpactRuleNamed(changedPath) || isTestPath(changedPath) {
			continue
		}
		if _, indexed := index.Sources[changedPath]; !indexed {
			continue
		}
		gaps++
	}
	return gaps
}

// webWorkspaceImportGap counts the requested web changed paths that some
// source may import through their nested package's name, a bare specifier rule
// (c) does not resolve (GPK-V0-067, proposed). Rule (c)'s answer for such a
// path is incomplete, so the receipt names the unresolved dimension rather
// than report complete coverage over it. Test and unindexed paths are excluded
// for the reasons reverseImportProfileGap gives.
func webWorkspaceImportGap(index *Index, receipt map[string]any) int {
	if index == nil || stringValue(receipt["mode"]) != "impact" {
		return 0
	}
	request, ok := receipt["request"].(map[string]any)
	if !ok {
		return 0
	}
	gaps := 0
	for _, changedPath := range stringsField(request["paths"]) {
		if !webSuffixes[strings.ToLower(pythonPathSuffix(changedPath))] || isTestPath(changedPath) {
			continue
		}
		if _, indexed := index.Sources[changedPath]; !indexed {
			continue
		}
		if webPackageNameImported(index, changedPath) {
			gaps++
		}
	}
	return gaps
}

func setCoverage(receipt map[string]any, requested int, critical []any, budget *int, index *Index, extraUncertainty ...string) error {
	results := mapsFromAny(receipt["results"])
	selectors := make(map[string]struct{}, len(results))
	advisory := 0
	for _, item := range results {
		selectors[selector(item["kind"], item["id"])] = struct{}{}
		if item["kind"] == "learned-path" {
			advisory++
		}
	}
	criticalMissing := make([]any, 0)
	for _, raw := range critical {
		if _, ok := selectors[stringValue(raw)]; !ok {
			criticalMissing = append(criticalMissing, raw)
		}
	}
	omitted := requested - len(results)
	uncertainty := withheldAuthorityUncertainty(results)
	authoritative := len(results) - advisory
	if syntaxOnlyQueryPacket(stringValue(receipt["mode"]), results) {
		authoritative = 0
		uncertainty = append(uncertainty, SyntaxOnlyUncertainty)
	}
	if lacksLanguageVerification(index, receipt) {
		uncertainty = append(uncertainty, unknownLanguageVerification)
	}
	if advisory != 0 {
		uncertainty = append(uncertainty, "learned path relationships are advisory")
	}
	if gaps := reverseImportProfileGap(index, receipt); gaps != 0 {
		uncertainty = append(uncertainty, fmt.Sprintf(
			"reverse-import results for %d changed paths with no named resolution rule are outside the native Go impact profile", gaps))
	}
	if gaps := webWorkspaceImportGap(index, receipt); gaps != 0 {
		uncertainty = append(uncertainty, fmt.Sprintf(
			"reverse-import results for %d changed paths importable by workspace package name are unresolved", gaps))
	}
	if omitted != 0 {
		// With no budget the only ceiling that can drop a ranked result is the
		// result limit, and naming a budget that was never set reads as a
		// certainty the receipt never measured.
		cause := "packet budget"
		if budget == nil {
			cause = "result limit"
		}
		uncertainty = append(uncertainty, fmt.Sprintf("%d ranked results omitted by %s", omitted, cause))
	}
	// A disclosure the caller computed before compiling is a coverage line like
	// any other, so it is appended here rather than onto a finished receipt:
	// compileReceipt fits results to the budget by re-running this over trial
	// packets, and a line added after that fitting sits outside it and can
	// carry an already-fitted packet past the budget. Appended here it competes
	// for the same bytes and displaces the last fitting result instead.
	for _, line := range extraUncertainty {
		uncertainty = append(uncertainty, line)
	}
	var budgetValue any
	if budget != nil {
		budgetValue = *budget
	}
	receipt["coverage"] = map[string]any{
		"requested_results": requested, "included_results": len(results), "omitted_results": omitted,
		"authoritative_results": authoritative, "advisory_results": advisory,
		"critical": cloneAnySlice(critical), "critical_missing": criticalMissing,
		"budget_bytes": budgetValue, "packet_bytes": 0, "within_budget": true, "uncertainty": uncertainty,
	}
	if err := stabilizePacketBytes(receipt); err != nil {
		return err
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		return &Error{Message: "cannot encode context receipt"}
	}
	receipt["coverage"].(map[string]any)["within_budget"] = budget == nil || len(encoded) <= *budget
	return stabilizePacketBytes(receipt)
}

func criticalResults(mode string, results []map[string]any) []any {
	if mode == "feature" {
		if len(results) == 0 {
			return []any{}
		}
		return []any{selector(results[0]["kind"], results[0]["id"])}
	}
	if mode == "impact" || mode == "range-impact" {
		critical := make([]any, 0)
		for _, item := range results {
			if item["kind"] == "path" {
				critical = append(critical, selector(item["kind"], item["id"]))
			}
		}
		return critical
	}
	for _, item := range results {
		if item["kind"] == "instructions" {
			return []any{selector(item["kind"], item["id"])}
		}
	}
	for _, item := range results {
		if item["kind"] == "feature" || item["kind"] == "scenario" {
			return []any{selector(item["kind"], item["id"])}
		}
	}
	return []any{}
}

func compactCoverageLists(receipt map[string]any, budget int) error {
	coverage := receipt["coverage"].(map[string]any)
	for _, field := range []string{"critical", "critical_missing"} {
		values := anySlice(coverage[field])
		if len(values) == 0 {
			continue
		}
		digest, err := canonicalSHA256(values)
		if err != nil {
			return err
		}
		replacement := map[string]any{
			field: []any{}, field + "_count": len(values),
			field + "_omitted_by_budget": len(values), field + "_sha256": digest,
		}
		replacementBytes, err := CanonicalJSON(replacement)
		if err != nil {
			return &Error{Message: "cannot encode context receipt"}
		}
		originalBytes, err := CanonicalJSON(map[string]any{field: values})
		if err != nil {
			return &Error{Message: "cannot encode context receipt"}
		}
		if len(replacementBytes) < len(originalBytes) {
			for key, value := range replacement {
				coverage[key] = value
			}
		}
	}
	coverage["within_budget"] = true
	if err := stabilizePacketBytes(receipt); err != nil {
		return err
	}
	encoded, err := CanonicalJSON(receipt)
	if err != nil {
		return &Error{Message: "cannot encode context receipt"}
	}
	coverage["within_budget"] = len(encoded) <= budget
	return stabilizePacketBytes(receipt)
}

func criticalFirst(results []map[string]any, critical map[string]struct{}) []map[string]any {
	ordered := make([]map[string]any, 0, len(results))
	for _, item := range results {
		if _, ok := critical[selector(item["kind"], item["id"])]; ok {
			ordered = append(ordered, item)
		}
	}
	for _, item := range results {
		if _, ok := critical[selector(item["kind"], item["id"])]; !ok {
			ordered = append(ordered, item)
		}
	}
	return ordered
}

func stringSetFromAny(values []any) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[stringValue(value)] = struct{}{}
	}
	return result
}

func canonicalSHA256(value any) (string, error) {
	encoded, err := CanonicalJSON(value)
	if err != nil {
		return "", &Error{Message: "cannot encode context receipt"}
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = cloneValue(item)
	}
	return result
}

func cloneValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		return cloneMap(item)
	case []any:
		return cloneAnySlice(item)
	case []map[string]any:
		return cloneResults(item)
	case []string:
		return append([]string(nil), item...)
	default:
		return value
	}
}

func cloneAnySlice(values []any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = cloneValue(value)
	}
	return result
}

func cloneResults(values []map[string]any) []map[string]any {
	result := make([]map[string]any, len(values))
	for index, value := range values {
		result[index] = cloneMap(value)
	}
	return result
}

func resultsToAny(values []map[string]any) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func sliceLength(value any) int {
	switch values := value.(type) {
	case []any:
		return len(values)
	case []string:
		return len(values)
	case []map[string]any:
		return len(values)
	default:
		return 0
	}
}

func stabilizePacketBytes(result map[string]any) error {
	coverage := result["coverage"].(map[string]any)
	for count := 0; count < 12; count++ {
		encoded, err := CanonicalJSON(result)
		if err != nil {
			return &Error{Message: "cannot encode context receipt"}
		}
		if coverage["packet_bytes"] == len(encoded) {
			return nil
		}
		coverage["packet_bytes"] = len(encoded)
	}
	// Falling out of the loop means the encoded length never matched the
	// recorded one. The receipt would then ship a packet_bytes that does not
	// describe its own bytes, so refuse rather than return a receipt whose
	// self-description is wrong (AGENTS.md invariant 2).
	return &Error{Message: "context receipt packet_bytes did not reach a fixed point"}
}

func verification(index *Index, results []map[string]any) []any {
	return verificationCommands(index, verificationPaths(results))
}

func lacksLanguageVerification(index *Index, receipt map[string]any) bool {
	commands := anySlice(receipt["verification"])
	if len(commands) != 1 || stringValue(commands[0]) != "git diff --check" {
		return false
	}
	hasLanguage := false
	for sourcePath := range index.Sources {
		if !hasSourceLanguageSuffix(sourcePath) {
			continue
		}
		hasLanguage = true
		if command, complete := toolchainCommand(sourcePath, index.Sources); complete && command != "" {
			return false
		}
	}
	return hasLanguage
}

func hasSourceLanguageSuffix(value string) bool {
	switch strings.ToLower(path.Ext(value)) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs",
		".rs", ".cs", ".swift", ".kt", ".kts", ".rb", ".sql", ".m", ".sh":
		return true
	default:
		return false
	}
}

// verificationPaths is the sorted, 20-capped evidence-path list a verification
// plan is derived from. It is separate from verificationCommands so that
// SBQ-V0-010(b) can recompute the cap over surviving results' paths alone and
// append a suppressed path's contribution after them.
func verificationPaths(results []map[string]any) []string {
	paths := make(map[string]struct{})
	for _, result := range results {
		for _, raw := range anySlice(result["evidence"]) {
			item, ok := raw.(map[string]any)
			if ok {
				paths[stringValue(item["path"])] = struct{}{}
			}
		}
	}
	orderedPaths := make([]string, 0, len(paths))
	for value := range paths {
		orderedPaths = append(orderedPaths, value)
	}
	sort.Strings(orderedPaths)
	if len(orderedPaths) > 20 {
		orderedPaths = orderedPaths[:20]
	}
	return orderedPaths
}

func verificationCommands(index *Index, orderedPaths []string) []any {
	projectFiles := index.Sources
	profile := verificationProfile(projectFiles)
	commands := make([]string, 0)
	for _, value := range orderedPaths {
		// A path's shape can match a named profile's Command table (e.g. an
		// `internal/<pkg>/` layout also matching Beamfall's) without that
		// profile actually being signalled in this repository. Only accept
		// the prescribed command when the profile that prescribes it is the
		// one this repository actually proved (AGENTS.md invariant 3):
		// project-owned authority, never syntax shape alone.
		command, prescriber, prescribed := projectprofile.Prescribe(value)
		complete := true
		if !prescribed || prescriber.ID != profile.ID {
			command, complete = toolchainCommand(value, projectFiles)
		}
		if complete && command != "" && !stringIn(commands, command) {
			commands = append(commands, command)
		}
	}
	// The gate is the plan's closing proof, so the eight-command ceiling drops
	// a package hint rather than the gate: appending the gate first and
	// truncating afterwards cut it off exactly when the plan was busiest
	// (DR-0022). Reserving its slot before the truncation keeps the ceiling and
	// keeps the gate.
	finalCommand, complete := gateCommand(profile, projectFiles)
	appendsGate := complete && finalCommand != "" && !stringIn(commands, finalCommand)
	ceiling := 8
	if appendsGate {
		ceiling--
	}
	if len(commands) > ceiling {
		commands = commands[:ceiling]
	}
	if appendsGate {
		commands = append(commands, finalCommand)
	}
	result := make([]any, len(commands))
	for index, command := range commands {
		result[index] = command
	}
	return result
}

// verificationProfile elects the profile whose gate closes a verification plan.
// An empty inventory means the caller supplied none, not that the project shows
// no signals, so it must not fall through to the signal-free profile: an absent
// inventory admits every profile.
func verificationProfile(projectFiles map[string]Source) projectprofile.Profile {
	if len(projectFiles) == 0 {
		return projectprofile.Detect(func(string) bool { return true })
	}
	return projectprofile.Detect(sourcePresence(projectFiles))
}

// toolchainCommand names the test command a path implies from the build
// toolchains present in the repository, independent of any project profile.
// Its boolean is false when deriving the answer required unavailable content.
func toolchainCommand(value string, projectFiles map[string]Source) (string, bool) {
	switch {
	case strings.HasSuffix(value, ".go"):
		if _, ok := projectFiles["go.mod"]; ok {
			parent := path.Dir(value)
			if parent == "." {
				return "go test ./...", true
			}
			return "go test ./" + parent + "/...", true
		}
	case strings.HasSuffix(value, ".py"):
		if hasAny(projectFiles, "pyproject.toml", "setup.py", "setup.cfg", "pytest.ini") ||
			hasPathPrefix(projectFiles, "tests/") {
			return "python -m pytest", true
		}
	case hasWebSuffix(value):
		if source, ok := projectFiles["package.json"]; ok {
			hasTest, loaded := packageJSONHasTestScript(source)
			if !loaded {
				return "", false
			}
			if hasTest {
				return "npm test", true
			}
		}
	}
	return "", true
}

// gateCommand names the command that closes a verification plan. A signalled
// profile's own Gate is project-owned authority and always wins; the
// signal-free fallback profile instead derives its gate from what the
// repository proves it has -- a Makefile declaring a conventional gate
// target -- rather than from a project-specific command it cannot confirm. Its
// boolean is false when that fallback scan could not inspect the Makefile.
func gateCommand(profile projectprofile.Profile, projectFiles map[string]Source) (string, bool) {
	if profile.ID != projectprofile.Fallback().ID {
		return profile.Gate, true
	}
	command, found, complete := makefileTargetCommand(projectFiles)
	if !complete {
		return "", false
	}
	if found {
		return command, true
	}
	return profile.Gate, true
}

// conventionalGateTargets are the Makefile target names treated as a
// project's closing verification step, in priority order.
var conventionalGateTargets = []string{"gate", "test", "check", "verify", "ci"}

var makefileTargetPattern = regexp.MustCompile(`(?m)^([A-Za-z0-9][A-Za-z0-9_.-]*)\s*:(?:[^=]|$)`)

// makefileTargetCommand reports the first conventional gate target actually
// declared in the repository's Makefile, so a `make <target>` command is never
// emitted unless that target's prerequisite -- the target line itself -- is
// present. Complete distinguishes a completed scan with no match from a source
// whose bytes were unavailable.
func makefileTargetCommand(projectFiles map[string]Source) (command string, found, complete bool) {
	source, ok := projectFiles["Makefile"]
	if !ok {
		source, ok = projectFiles["makefile"]
	}
	if !ok {
		return "", false, true
	}
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return "", false, false
	}
	targets := make(map[string]struct{})
	for _, match := range makefileTargetPattern.FindAllStringSubmatch(text, -1) {
		targets[match[1]] = struct{}{}
	}
	for _, name := range conventionalGateTargets {
		if _, present := targets[name]; present {
			return "make " + name, true, true
		}
	}
	return "", false, true
}

// packageJSONHasTestScript reports whether package.json declares a non-empty
// "test" script, so `npm test` is never emitted for a project that has no
// such script wired up. Its second result reports whether the scan completed.
func packageJSONHasTestScript(source Source) (bool, bool) {
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return false, false
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal([]byte(text), &manifest) != nil {
		return false, true
	}
	return strings.TrimSpace(manifest.Scripts["test"]) != "", true
}

func hasPathPrefix(projectFiles map[string]Source, prefix string) bool {
	for candidate := range projectFiles {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}

func hasAny(values map[string]Source, candidates ...string) bool {
	for _, candidate := range candidates {
		if _, ok := values[candidate]; ok {
			return true
		}
	}
	return false
}

func hasWebSuffix(value string) bool {
	switch strings.ToLower(path.Ext(value)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}

func selector(kind, identifier any) string { return fmt.Sprintf("%s:%s", kind, identifier) }

// extractionSummary reports the sources whose symbol walk did not complete. It
// mirrors the exclusions block in shape -- a total count plus a bounded sample
// -- so a reader who already understands one understands the other.
//
// The count is of notes, not of distinct paths: one file can both hit the
// symbol cap and end inside an unterminated comment, and collapsing those to a
// single path would hide one of the two reasons it is incomplete.
func extractionSummary(index *Index) (map[string]any, bool) {
	if len(index.ExtractionNotes) == 0 {
		return nil, false
	}
	sampled := index.ExtractionNotes[:min(len(index.ExtractionNotes), maxExclusionSamples)]
	samples := make([]any, 0, len(sampled))
	for _, note := range sampled {
		samples = append(samples, map[string]any{"path": note.Path, "reason": note.Reason})
	}
	return map[string]any{"count": len(index.ExtractionNotes), "samples": samples}, true
}
