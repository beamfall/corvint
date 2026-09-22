package contextindex

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

var canonicalFeatureID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var (
	genericFeatureSymbolNames  = stringSet("client config create delete device generate handle manager media process profile render run server session token update")
	genericFeatureRoutingTerms = stringSet("browser client device manager media profile server session settings")
	ignoredMarkerSymbolNames   = stringSet("setup teardown setupclass teardownclass beforeeach aftereach")
)

type featureCandidateKey struct {
	path string
	line int
	name string
}

type featureCandidate struct {
	score     int
	reason    string
	symbol    Symbol
	markerUse bool
}

// The dependency-link stage reserves two positions for a root declaration's
// callees. Exact marker tests get the same bounded shape: two declarations
// they use precede vocabulary-only neighbours.
const featureMarkerUseCap = 2

// Feature compiles an unbudgeted canonical-feature receipt.
func Feature(index *Index, featureID string, limit int) (map[string]any, error) {
	return FeatureBudget(index, featureID, limit, nil)
}

// FeatureBudget compiles the Go-symbol feature slice and applies the shared
// Python-compatible packet budget selector.
func FeatureBudget(index *Index, featureID string, limit int, budget *int) (map[string]any, error) {
	if err := ValidateFeature(featureID, limit); err != nil {
		return nil, err
	}
	request := map[string]any{"feature_id": featureID, "limit": limit}
	record, ok := index.Features[featureID]
	if !ok {
		contextReceipt, err := receipt(index, "feature", request, nil, limit, "OUT_OF_SCOPE")
		if err != nil {
			return nil, err
		}
		return compileReceipt(contextReceipt, budget, index)
	}
	if limit > 1 && featureHasUnsupportedSymbolSources(index) {
		return nil, &Error{
			Code:    "unsupported-feature-repository",
			Message: "native Go feature candidate ranking currently supports only Go source symbols",
		}
	}
	results := []map[string]any{
		recordResult(index, record, 1000, "exact canonical feature:"+featureID+" match"),
	}
	results = append(results, featureImplementationCandidates(index, record, limit-1)...)
	contextReceipt, err := receipt(index, "feature", request, results, limit, "READY")
	if err != nil {
		return nil, err
	}
	return compileReceipt(contextReceipt, budget, index)
}

// ValidateFeature preserves the Python feature function's validation order.
func ValidateFeature(featureID string, limit int) error {
	if limit < 1 || limit > maxLimit {
		return &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	if !canonicalFeatureID.MatchString(featureID) {
		return &Error{Message: "feature_id must be a lowercase kebab-case identifier"}
	}
	return nil
}

func featureHasUnsupportedSymbolSources(index *Index) bool {
	for sourcePath := range index.Sources {
		switch strings.ToLower(path.Ext(sourcePath)) {
		case ".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
			return true
		}
	}
	return false
}

func featureImplementationCandidates(index *Index, record Record, maximum int) []map[string]any {
	if maximum <= 0 {
		return nil
	}
	candidates := make(map[featureCandidateKey]featureCandidate)
	markerIntentTerms := make(map[string]struct{})
	markerPaths := make([]string, 0)
	markerCode := make(map[string][]string)
	for _, marker := range index.Markers[record.Kind+":"+record.ID] {
		if !isTestPath(marker.Path) {
			continue
		}
		markerPaths = append(markerPaths, marker.Path)
		nearest, ok := nearestMarkerSymbol(index.Symbols, marker)
		if !ok || contains(ignoredMarkerSymbolNames, compactText(nearest.Name)) {
			continue
		}
		markerCode[marker.Path] = append(markerCode[marker.Path], featureMarkerSymbolCode(index, nearest))
		for term := range terms(nearest.Name) {
			markerIntentTerms[term] = struct{}{}
		}
	}

	featureTerms := terms(record.ID + " " + pythonString(valueOr(record.Fields["area"], "")) + " " + pythonString(valueOr(record.Fields["summary"], "")))
	for term := range markerIntentTerms {
		featureTerms[term] = struct{}{}
	}
	// Both overlap counts below run terms() over every symbol in the repository,
	// once per feature the query considers, and featureRelevantOverlap reads the
	// resulting set only by asking whether each featureTerms member is in it. So
	// the keep-set is featureTerms and every term outside it is built to be
	// discarded -- the same shape evalSymbolChunk.collect's context window has.
	// markerIntentTerms and featureTerms themselves stay on terms(): they ARE the
	// keep-set, so there is nothing to filter them against.
	for _, markerPath := range markerPaths {
		targetDirectories := map[string]struct{}{path.Dir(markerPath): {}}
		for imported := range index.Imports[markerPath] {
			prefix := index.Module + "/"
			if strings.HasSuffix(markerPath, ".go") && index.Module != "" && strings.HasPrefix(imported, prefix) {
				targetDirectories[strings.TrimPrefix(imported, prefix)] = struct{}{}
			}
		}
		for _, symbol := range index.Symbols {
			if _, ok := targetDirectories[path.Dir(symbol.Path)]; !ok || isTestPath(symbol.Path) {
				continue
			}
			overlap := featureRelevantOverlap(featureTerms, keepSetTerms(symbol.Name, featureTerms))
			if overlap == 0 {
				continue
			}
			key := featureCandidateKey{symbol.Path, symbol.Line, symbol.Name}
			candidate := featureCandidate{
				score: 600 + overlap*20, symbol: symbol,
				reason:    "exact " + record.Kind + ":" + record.ID + " marker test shares or imports this implementation",
				markerUse: featureMarkerUsesName(markerCode[markerPath], symbol.Name),
			}
			if current, ok := candidates[key]; ok {
				candidate.markerUse = candidate.markerUse || current.markerUse
			}
			candidates[key] = candidate
		}
	}

	compactID := compactText(record.ID)
	for _, symbol := range index.Symbols {
		if isTestPath(symbol.Path) {
			continue
		}
		overlap := featureRelevantOverlap(featureTerms, keepSetTerms(symbol.Name+" "+symbol.Path, featureTerms))
		compactMatch := compactID != "" && strings.Contains(compactText(symbol.Name+" "+symbol.Path), compactID)
		if contains(genericFeatureSymbolNames, compactText(symbol.Name)) && overlap < 2 && !compactMatch {
			continue
		}
		if overlap == 0 && !compactMatch {
			continue
		}
		key := featureCandidateKey{symbol.Path, symbol.Line, symbol.Name}
		if current, ok := candidates[key]; ok {
			current.score += overlap * 20
			if compactMatch {
				current.score += 150
			}
			candidates[key] = current
			continue
		}
		score := 300 + overlap*20
		if compactMatch {
			score += 150
		}
		candidates[key] = featureCandidate{
			score: score, symbol: symbol,
			reason: "declaration/path lexically matches canonical " + record.Kind + " metadata",
		}
	}

	ranked := make([]featureCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		ranked = append(ranked, candidate)
	}
	sort.Slice(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		a, b := ranked[left].symbol, ranked[right].symbol
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
	ranked = promoteFeatureMarkerUses(ranked)
	if len(ranked) > maximum {
		ranked = ranked[:maximum]
	}
	results := make([]map[string]any, len(ranked))
	for index, candidate := range ranked {
		results[index] = featureSymbolResult(candidate.symbol, candidate.score, candidate.reason)
	}
	return results
}

func featureMarkerSymbolCode(index *Index, symbol Symbol) string {
	source, ok := index.Sources[symbol.Path]
	if !ok {
		return ""
	}
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return ""
	}
	var lines []string
	switch strings.ToLower(path.Ext(symbol.Path)) {
	case ".go":
		lines = goCodeLines(text)
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		lines = pythonSplitLines(stripWebNonCode(text))
	default:
		return ""
	}
	start, end := symbol.Line-1, len(lines)
	if start < 0 || start >= end {
		return ""
	}
	for _, candidate := range index.Symbols {
		if candidate.Path == symbol.Path && candidate.Line > symbol.Line {
			end = min(end, candidate.Line-1)
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func featureMarkerUsesName(code []string, name string) bool {
	for _, declaration := range code {
		if containsPythonWord(declaration, name) {
			return true
		}
	}
	return false
}

func promoteFeatureMarkerUses(ranked []featureCandidate) []featureCandidate {
	promoted := make([]featureCandidate, 0, min(featureMarkerUseCap, len(ranked)))
	remainder := make([]featureCandidate, 0, len(ranked))
	useNames := make(map[string]int)
	for _, candidate := range ranked {
		if candidate.markerUse {
			useNames[candidate.symbol.Name]++
		}
	}
	for _, candidate := range ranked {
		if candidate.markerUse && useNames[candidate.symbol.Name] == 1 && len(promoted) < featureMarkerUseCap {
			promoted = append(promoted, candidate)
			continue
		}
		remainder = append(remainder, candidate)
	}
	return append(promoted, remainder...)
}

func nearestMarkerSymbol(symbols []Symbol, marker Marker) (Symbol, bool) {
	var nearest Symbol
	found := false
	for _, symbol := range symbols {
		if symbol.Path != marker.Path {
			continue
		}
		if !found || markerSymbolLess(symbol, nearest, marker.Line) {
			nearest, found = symbol, true
		}
	}
	return nearest, found
}

func markerSymbolLess(left, right Symbol, markerLine int) bool {
	leftDistance, rightDistance := absolute(left.Line-markerLine), absolute(right.Line-markerLine)
	if leftDistance != rightDistance {
		return leftDistance < rightDistance
	}
	leftAfter, rightAfter := left.Line > markerLine, right.Line > markerLine
	if leftAfter != rightAfter {
		return !leftAfter
	}
	if left.Line != right.Line {
		return left.Line < right.Line
	}
	return left.Name < right.Name
}

func featureRelevantOverlap(left, right map[string]struct{}) int {
	count := 0
	for value := range left {
		if _, ok := right[value]; !ok || contains(genericFeatureRoutingTerms, value) {
			continue
		}
		count++
	}
	return count
}

func featureSymbolResult(symbol Symbol, score int, reason string) map[string]any {
	return map[string]any{
		"kind": "symbol", "id": symbol.Path + ":" + symbol.Name, "name": symbol.Name,
		"score": score, "summary": symbol.Kind + " declaration",
		"evidence": []any{evidence(symbol.Path, symbol.Line, symbol.BlobHash, reason, "medium", "syntax")},
	}
}

func absolute(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
