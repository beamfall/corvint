package contextindex

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/projectprofile"
)

type rankedResult struct {
	score, order          int
	key                   string
	result                map[string]any
	testConvention        bool
	broadModuleRootImport bool
	caller                bool
}

func Impact(index *Index, paths []string, limit int) (map[string]any, error) {
	result, _, err := impact(index, paths, limit)
	return result, err
}

// impact compiles the path impact receipt and also returns its omitted-caller
// disclosures, so EvalImpact can carry them through its budget compilation.
func impact(index *Index, paths []string, limit int) (map[string]any, []string, error) {
	if limit < 1 || limit > maxLimit {
		return nil, nil, &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	if len(paths) == 0 {
		return nil, nil, &Error{Message: "impact paths must be a non-empty list"}
	}
	if len(paths) > maxImpactPaths {
		return nil, nil, &Error{Message: fmt.Sprintf("impact paths exceed %d-path bound", maxImpactPaths)}
	}
	cleanedSet := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		cleaned, err := cleanImpactPath(value)
		if err != nil {
			return nil, nil, err
		}
		cleanedSet[cleaned] = struct{}{}
	}
	cleaned := keys(cleanedSet)
	// Decision 0015: only rule (a) resolves through the Go module path, so
	// the slash-qualified module is a precondition for `.go` paths alone;
	// `.py` and web paths are admitted in any Git repository, as the oracle
	// admits them.
	if needsGoModule(cleaned) && (index.Module == "" || !strings.Contains(index.Module, "/")) {
		return nil, nil, &Error{Code: "unsupported-impact-repository", Message: "native Go impact requires a slash-qualified Go module for .go paths"}
	}
	tracked := make(map[string]struct{}, len(index.Sources)+len(index.Exclusions))
	for value := range index.Sources {
		tracked[value] = struct{}{}
	}
	for _, exclusion := range index.Exclusions {
		tracked[exclusion.Path] = struct{}{}
	}
	missing := make([]string, 0)
	for _, value := range cleaned {
		if !contains(tracked, value) {
			missing = append(missing, value)
		}
	}
	if len(missing) != 0 {
		return nil, nil, &Error{Message: fmt.Sprintf("impact paths are not tracked at revision %s: %s", index.Revision, strings.Join(missing, ", "))}
	}
	forbidden := make([]string, 0)
	for _, value := range cleaned {
		if _, ok := index.Sources[value]; !ok {
			forbidden = append(forbidden, value)
		}
	}
	if len(forbidden) != 0 {
		result, err := receipt(index, "impact", map[string]any{"paths": cleaned, "limit": limit}, nil, limit, "OUT_OF_SCOPE")
		return result, nil, err
	}

	ranked := make([]rankedResult, 0)
	related := make(map[string]struct{})
	// carried holds the keys a changed path's own markers carry;
	// importerCarriers names, for each key, the first importing test that
	// carried it, so a key only such a test carries says so (V1-0263).
	carried := make(map[string]struct{})
	importerCarriers := make(map[string]string)
	// V1-0054: built once per call rather than rescanned per changed path (up to
	// maxImpactPaths) or per ranked record. dirIndex answers "which tracked paths
	// share this directory" and sortedPaths lets an ADR-document prefix lookup
	// binary-search instead of scanning every source.
	dirIndex := dirPathIndex(index)
	sortedPaths := sortedSourcePaths(index)
	goCallers := make(map[string]goCaller)
	for _, changedPath := range cleaned {
		source := index.Sources[changedPath]
		ranked = append(ranked, rankedResult{score: 1000, order: 0, key: changedPath, result: map[string]any{
			"kind": "path", "id": changedPath, "score": 1000,
			"summary": "direct changed path", "evidence": []any{evidence(
				changedPath, 1, source.BlobHash, "requested changed path", "authoritative", "git-tree",
			)},
		}})
		for _, document := range index.Documents {
			references := stringsField(document.Fields["references"])
			if document.Path == changedPath || stringIn(references, changedPath) {
				relation := "is the changed document"
				if document.Path != changedPath {
					relation = "explicitly references changed path " + changedPath
				}
				ranked = append(ranked, rankedResult{score: 825, order: 1, key: document.Path, result: documentResult(index, document, 825, relation)})
			}
		}
		for key, markers := range index.Markers {
			for _, marker := range markers {
				if marker.Path == changedPath {
					related[key] = struct{}{}
					carried[key] = struct{}{}
					break
				}
			}
		}
		packageReferences := samePackageReferencesWithDirIndex(index, changedPath, true, dirIndex)
		testReferences := make(map[string]packageReference)
		for _, reference := range packageReferences {
			if isTestPath(reference.path) {
				testReferences[reference.path] = reference
			}
		}
		packageMarkers := make(map[string][]markerRelation)
		if strings.HasSuffix(changedPath, ".go") && !isTestPath(changedPath) {
			parent, stem := path.Dir(changedPath), strings.TrimSuffix(path.Base(changedPath), ".go")
			tests := make([]string, 0)
			for _, candidate := range dirIndex[parent] {
				if strings.HasSuffix(candidate, "_test.go") {
					tests = append(tests, candidate)
				}
			}
			sort.Strings(tests)
			for _, testPath := range tests {
				exact := path.Base(testPath) == stem+"_test.go"
				score, confidence := 550, "medium"
				if exact {
					score, confidence = 825, "high"
				}
				testSource := index.Sources[testPath]
				testEvidence := []any{evidence(testPath, 1, testSource.BlobHash,
					"same-package test for "+changedPath, confidence, "test-convention")}
				if reference, ok := testReferences[testPath]; ok {
					if !exact {
						score += min(reference.count, 274)
					}
					for _, item := range packageReferenceEvidence(index, changedPath, reference) {
						if len(testEvidence) == maxEvidence {
							break
						}
						testEvidence = append(testEvidence, item)
					}
				}
				ranked = append(ranked, rankedResult{score: score, order: 1, key: testPath, testConvention: true, result: map[string]any{
					"kind": "test", "id": testPath, "score": score,
					"summary":  "same-package test affected by " + changedPath,
					"evidence": testEvidence,
				}})
			}
			changedKeys := make(map[string]struct{}, len(related))
			for key := range related {
				changedKeys[key] = struct{}{}
			}
			for key, markers := range index.Markers {
				for _, marker := range markers {
					if isTestPath(marker.Path) && path.Dir(marker.Path) == parent && markerCredited(marker.Path, key, stem+"_test.go", changedKeys, testReferences) {
						packageMarkers[marker.Path] = append(packageMarkers[marker.Path], markerRelation{key, marker})
						related[key] = struct{}{}
					}
				}
			}
		}
		markerPaths := make([]string, 0, len(packageMarkers))
		for markerPath := range packageMarkers {
			markerPaths = append(markerPaths, markerPath)
		}
		sort.Strings(markerPaths)
		for _, testPath := range markerPaths {
			items := packageMarkers[testPath]
			sort.Slice(items, func(left, right int) bool {
				return markerRelationLess(index, items[left], items[right])
			})
			if len(items) > maxEvidence {
				items = items[:maxEvidence]
			}
			testEvidence := make([]any, 0, len(items))
			for _, item := range items {
				kind, id := splitRelation(item.key)
				testEvidence = append(testEvidence, evidence(item.marker.Path, item.marker.Line, item.marker.BlobHash,
					"same-package test carries "+kind+":"+id, "high", "test-marker"))
			}
			ranked = append(ranked, rankedResult{score: 850, order: 1, key: testPath, result: map[string]any{
				"kind": "test", "id": testPath, "score": 850,
				"summary":  "same-package marked test affected by " + changedPath,
				"evidence": testEvidence,
			}})
		}
		if !isTestPath(changedPath) {
			exported := exportedGoNames(index, changedPath)
			for _, importer := range reverseImporters(index, changedPath) {
				importerMarkers := markerKeysForPath(index, importer.path)
				if isTestPath(importer.path) && countKind(importerMarkers, "feature") <= 1 && countKind(importerMarkers, "scenario") <= 1 {
					for _, key := range importerMarkers {
						related[key] = struct{}{}
						if _, named := importerCarriers[key]; !named {
							importerCarriers[key] = "test " + importer.path + " importing changed " + changedPath
						}
					}
				}
				line, loaded := importEvidenceLine(index.Sources[importer.path], importer.imported)
				if !loaded {
					continue
				}
				recordGoCaller(goCallers, index, changedPath, importer, line)
				score := 700
				if isTestPath(importer.path) {
					score = 650
				}
				importerSource := index.Sources[importer.path]
				ranked = append(ranked, rankedResult{score: score, order: 1, key: importer.path,
					broadModuleRootImport: broadModuleRootImporter(index, changedPath, importer),
					caller:                score == 700 && namesAny(importerSource, exported), result: map[string]any{
						"kind": "reverse-import", "id": importer.path, "score": score,
						"summary": "directly imports package/module containing " + changedPath,
						"evidence": []any{evidence(importer.path, line, importerSource.BlobHash,
							"imports "+importer.imported, "high", "syntax")},
					}})
			}
		}
		for _, reference := range packageReferences {
			if isTestPath(reference.path) {
				continue
			}
			ranked = append(ranked, rankedResult{score: 775, order: 1, key: reference.path, result: map[string]any{
				"kind": "reference", "id": reference.path, "score": 775,
				"summary":  "same-package code references declarations from " + changedPath,
				"evidence": packageReferenceEvidence(index, changedPath, reference),
			}})
		}
	}
	// A changed path that calls another changed path is already a direct row.
	for _, changedPath := range cleaned {
		delete(goCallers, changedPath)
	}
	relatedKeys := keys(related)
	for _, key := range relatedKeys {
		kind, identifier := splitRelation(key)
		records := index.Features
		if kind == "scenario" {
			records = index.Scenarios
		}
		if record, ok := records[identifier]; ok {
			carrier := "changed path"
			if importerCarrier, ok := importerCarriers[key]; ok && !contains(carried, key) {
				carrier = importerCarrier
			}
			result := recordResultWithSortedPaths(index, record, 800, carrier+" carries "+kind+":"+identifier, sortedPaths)
			withholdUnverifiedADRAuthority(result)
			ranked = append(ranked, rankedResult{score: 800, order: 1, key: key, result: result})
		}
	}
	sort.SliceStable(ranked, func(left, right int) bool {
		if ranked[left].score != ranked[right].score {
			return ranked[left].score > ranked[right].score
		}
		if ranked[left].order != ranked[right].order {
			return ranked[left].order < ranked[right].order
		}
		return ranked[left].key < ranked[right].key
	})
	ranked = reserveTestConventionTail(ranked)
	deduplicated := make([]map[string]any, 0, len(ranked))
	seen := make(map[string]map[string]any)
	callers := make(map[string]bool)
	for _, item := range ranked {
		key := fmt.Sprintf("%s:%s", item.result["kind"], item.result["id"])
		callers[key] = callers[key] || item.caller
		if existing, ok := seen[key]; ok {
			mergeEvidence(existing, item.result)
			continue
		}
		seen[key] = item.result
		deduplicated = append(deduplicated, item.result)
	}
	deduplicated = reserveCallerRows(deduplicated, callers, limit)
	disclosures := omittedCallerDisclosures(deduplicated, goCallers, limit)
	result, err := receipt(index, "impact", map[string]any{"paths": cleaned, "limit": limit}, deduplicated, limit, "", disclosures...)
	return result, disclosures, err
}

// reserveTestConventionTail keeps same-package convention tests ahead of broad
// consumers of a module's root package. Those consumers remain ranked normally
// when they name the changed file or one of its declarations.
func reserveTestConventionTail(ranked []rankedResult) []rankedResult {
	firstBroad := len(ranked)
	for index, item := range ranked {
		if item.broadModuleRootImport {
			firstBroad = index
			break
		}
	}
	if firstBroad == len(ranked) {
		return ranked
	}
	broad := make([]rankedResult, 0)
	remainder := make([]rankedResult, 0, len(ranked))
	lastTest := -1
	testAfterBroad := false
	for index, item := range ranked {
		if item.broadModuleRootImport {
			broad = append(broad, item)
			continue
		}
		remainder = append(remainder, item)
		if item.testConvention {
			lastTest = len(remainder) - 1
			testAfterBroad = testAfterBroad || index > firstBroad
		}
	}
	if !testAfterBroad {
		return ranked
	}
	result := make([]rankedResult, 0, len(ranked))
	result = append(result, remainder[:lastTest+1]...)
	result = append(result, broad...)
	return append(result, remainder[lastTest+1:]...)
}

// reserveCallerRows keeps limit/10 rows inside the limit for cross-package
// callers: non-test reverse importers whose code names an exported
// declaration of a changed Go file (GPK-V0-070). Callers already inside the
// limit count toward the quota; a moved caller displaces the lowest included
// rows, never a requested path row. Scores are unchanged.
func reserveCallerRows(results []map[string]any, callers map[string]bool, limit int) []map[string]any {
	quota := limit / 10
	if len(results) <= limit || quota == 0 {
		return results
	}
	for _, result := range results[:limit] {
		if callers[fmt.Sprintf("%s:%s", result["kind"], result["id"])] {
			quota--
		}
	}
	paths := 0
	for paths < limit && results[paths]["kind"] == "path" {
		paths++
	}
	quota = min(quota, limit-paths)
	moved := make([]map[string]any, 0, max(quota, 0))
	rest := make([]map[string]any, 0, len(results)-limit)
	for _, result := range results[limit:] {
		if len(moved) < quota && callers[fmt.Sprintf("%s:%s", result["kind"], result["id"])] {
			moved = append(moved, result)
			continue
		}
		rest = append(rest, result)
	}
	if len(moved) == 0 {
		return results
	}
	cut := limit - len(moved)
	reordered := make([]map[string]any, 0, len(results))
	reordered = append(reordered, results[:cut]...)
	reordered = append(reordered, moved...)
	reordered = append(reordered, results[cut:limit]...)
	return append(reordered, rest...)
}

// exportedGoNames lists the exported declarations of a non-test Go file.
func exportedGoNames(index *Index, changedPath string) []string {
	if !strings.HasSuffix(changedPath, ".go") {
		return nil
	}
	declared := make(map[string]struct{})
	for _, symbol := range index.Symbols {
		first, _ := utf8.DecodeRuneInString(symbol.Name)
		if symbol.Path == changedPath && unicode.IsUpper(first) {
			declared[symbol.Name] = struct{}{}
		}
	}
	return keys(declared)
}

// namesAny reports whether a Go source's code, outside comments and
// strings, names one of the given identifiers as a whole word.
func namesAny(source Source, names []string) bool {
	text, valid, loaded := source.Text()
	if !loaded || !valid || len(names) == 0 {
		return false
	}
	for _, line := range goCodeLines(text) {
		for _, name := range names {
			if strings.Contains(line, "."+name) && containsPythonWord(line, name) {
				return true
			}
		}
	}
	return false
}

func broadModuleRootImporter(index *Index, changedPath string, importer importer) bool {
	if !strings.HasSuffix(changedPath, goImpactSuffix) || path.Dir(changedPath) != "." ||
		index.Module == "" || importer.imported != index.Module {
		return false
	}
	text, valid, loaded := index.Sources[importer.path].Text()
	if !loaded || !valid {
		return true
	}
	if strings.Contains(text, changedPath) {
		return false
	}
	for _, symbol := range index.Symbols {
		if symbol.Path == changedPath && containsPythonWord(text, symbol.Name) {
			return false
		}
	}
	return true
}

type markerRelation struct {
	key    string
	marker Marker
}

func markerRelationLess(index *Index, left, right markerRelation) bool {
	leftFirst, rightFirst := index.Markers[left.key][0], index.Markers[right.key][0]
	if leftFirst.Path != rightFirst.Path {
		return leftFirst.Path < rightFirst.Path
	}
	if leftFirst.Line != rightFirst.Line {
		return leftFirst.Line < rightFirst.Line
	}
	if leftFirst.Column != rightFirst.Column {
		return leftFirst.Column < rightFirst.Column
	}
	if left.key != right.key {
		return left.key < right.key
	}
	if left.marker.Line != right.marker.Line {
		return left.marker.Line < right.marker.Line
	}
	return left.marker.Column < right.marker.Column
}

// needsGoModule reports whether any changed path resolves by the Go rule.
func needsGoModule(paths []string) bool {
	for _, value := range paths {
		if strings.HasSuffix(value, ".go") {
			return true
		}
	}
	return false
}

func cleanImpactPath(value string) (string, error) {
	if value == "" {
		return "", &Error{Message: "impact paths must be non-empty strings"}
	}
	if utf8.RuneCountInString(value) > maxPathChars {
		return "", &Error{Message: fmt.Sprintf("impact path exceeds %d characters", maxPathChars)}
	}
	if !repositoryRelative(value) || path.Clean(value) != value || strings.HasPrefix(value, "../") {
		return "", &Error{Message: fmt.Sprintf("impact path must be normalized and repository-relative: %q", value)}
	}
	return value, nil
}

func evidence(file string, line int, blob, reason, confidence, authority string) map[string]any {
	return map[string]any{"path": file, "line": line, "blob_hash": blob, "reason": reason,
		"confidence": confidence, "authority": authority}
}

// documentAuthority maps a document record to its AGENTS.md invariant-3 authority
// label and the confidence that label carries. `impact` and the authority trigger
// index share it so that one document cannot rank one way inside a packet and
// another way inside a trigger.
func documentAuthority(record Record) (authority, confidence string) {
	status, _ := record.Fields["status"].(string)
	binding := status == "accepted" || status == "approved" || status == "current" || status == "active" || strings.HasPrefix(status, "partially-superseded-by:")
	authority = "repository-spec"
	switch {
	case record.Kind == "instructions":
		authority, binding = "project-instructions", true
	case record.Kind == "decision" && binding:
		authority = "accepted-decision"
	case record.Kind == "decision":
		authority = "non-binding-decision"
	case binding:
		authority = "accepted-spec"
	}
	confidence = "medium"
	if binding {
		confidence = "authoritative"
	}
	return authority, confidence
}

func documentResult(index *Index, record Record, score int, reason string) map[string]any {
	status, _ := record.Fields["status"].(string)
	authority, confidence := documentAuthority(record)
	resultEvidence := []any{evidence(record.Path, record.Line, record.BlobHash, reason, confidence, authority)}
	references := stringsField(record.Fields["references"])
	for _, referenced := range references {
		if len(resultEvidence) >= maxEvidence {
			break
		}
		source := index.Sources[referenced]
		resultEvidence = append(resultEvidence, evidence(referenced, 1, source.BlobHash,
			"explicitly referenced by "+record.Path, "high", "document-reference"))
	}
	return map[string]any{"kind": record.Kind, "id": record.ID, "score": score,
		"title": stringValue(record.Fields["title"]), "summary": stringValue(record.Fields["summary"]),
		"status": status, "references": references, "evidence": resultEvidence}
}

// UnverifiedContractAuthority is the authority label `impact` emits for a
// governing decision it cannot resolve from a revision the caller did not
// author.  `CF-V0-031` (ratified 2026-08-29) permits exactly two dispositions
// for such a surface: gain a caller-independent revision, or report its
// accepted-authority labels as unverified.  `Impact(index, paths, limit)` has
// no base revision and no diff -- `paths` is the change set the caller
// *declares* -- so it takes the second, unconditionally.  A guard keyed on
// `paths` would close nothing, which `CF-V0-025` forbids asserting: the cited
// decision reaches `recordResult` through a ledger record's `adr:` field, so an
// actor that authors the decision in its own change set need only omit the
// path.  Withholding for every citation is the only disposition that does not
// depend on the caller's own declaration.
const UnverifiedContractAuthority = "unverified-contract"

// UnverifiedLedgerAuthority is the counterpart label for a canonical ledger
// record the caller's own change set declares.  `range impact` does have a
// caller-independent base, so it withholds only where the ledger file is
// genuinely inside the computed change set -- a scoped refusal, not a blanket
// one.
const UnverifiedLedgerAuthority = "unverified-ledger"

// SyntaxAuthority is the weakest evidence label a citation can carry: the host
// language's grammar found the symbol, and no project-owned document says it is
// the answer.  Product invariant 3 ranks it below every project-owned
// authority, so `setCoverage` treats a packet made only of it as uncorroborated.
const SyntaxAuthority = "syntax"

// withholdUnverifiedADRAuthority downgrades the accepted-authority labels an
// ADR citation would otherwise confer on an `impact` result, per
// `CF-V0-031`.  `setCoverage` names each withheld citation in
// `coverage.uncertainty`, so the abstention is explicit rather than a silent
// drop.  The citation itself, its path, and its blob hash are unchanged: the
// evidence is still reported, only its authority is withheld.
func withholdUnverifiedADRAuthority(result map[string]any) {
	for _, item := range mapsFromAny(result["evidence"]) {
		switch item["authority"] {
		case "accepted-contract", "partially-superseded-contract":
			item["authority"], item["confidence"] = UnverifiedContractAuthority, "low"
		}
	}
}

// recordResult renders a feature/scenario as a ranked evidence record.
// recordResultWithSortedPaths is the same computation with the index's source
// paths supplied pre-sorted, so a caller ranking many records (Impact, up to
// maxImpactPaths changed paths) builds that slice once instead of once per ADR
// ID per record (V1-0054).
func recordResult(index *Index, record Record, score int, reason string) map[string]any {
	return recordResultWithSortedPaths(index, record, score, reason, nil)
}

func recordResultWithSortedPaths(index *Index, record Record, score int, reason string, sortedPaths []string) map[string]any {
	resultEvidence := []any{evidence(record.Path, record.Line, record.BlobHash, reason, "authoritative", "canonical-ledger")}
	adrOutput, adrIDs := recordSequence(record.Fields["adr"], true)
	for _, adrValue := range adrIDs {
		adr := pythonString(adrValue)
		prefix := ""
		if adrPrefix := projectprofile.ByID(index.ProfileID).ADRPrefix; adrPrefix != "" {
			prefix = adrPrefix + adr + "-"
		}
		matches := adrDocumentMatches(index, sortedPaths, prefix)
		sort.Strings(matches)
		if len(matches) != 0 && len(resultEvidence) < maxEvidence {
			source := index.Sources[matches[0]]
			status := ""
			if document, ok := index.Documents[source.Path]; ok {
				status = stringValue(document.Fields["status"])
			}
			partial := strings.HasPrefix(status, "partially-superseded-by:")
			binding := status == "accepted" || partial
			authority, confidence := "non-binding-adr", "low"
			if partial {
				authority, confidence = "partially-superseded-contract", "authoritative"
			} else if binding {
				authority, confidence = "accepted-contract", "authoritative"
			}
			resultEvidence = append(resultEvidence, evidence(source.Path, 1, source.BlobHash,
				"ADR-"+adr+" governs "+record.Kind+":"+record.ID, confidence, authority))
		}
	}
	markers := index.Markers[record.Kind+":"+record.ID]
	markerLimit := maxEvidence - 1
	if len(markers) < markerLimit {
		markerLimit = len(markers)
	}
	for _, marker := range markers[:markerLimit] {
		confidence, authority := "medium", "source-marker"
		if isTestPath(marker.Path) {
			confidence, authority = "high", "test-marker"
		}
		resultEvidence = append(resultEvidence, evidence(marker.Path, marker.Line, marker.BlobHash,
			"exact "+record.Kind+":"+record.ID+" marker", confidence, authority))
	}
	if record.Kind == "feature" {
		for _, scenario := range sortedRecords(index.Scenarios) {
			if pythonContains(scenario.Fields["features"], record.ID) {
				resultEvidence = append(resultEvidence, evidence(scenario.Path, scenario.Line, scenario.BlobHash,
					"scenario:"+scenario.ID+" composes feature:"+record.ID, "high", "canonical-ledger"))
				if len(resultEvidence) >= maxEvidence {
					break
				}
			}
		}
	}
	return map[string]any{"kind": record.Kind, "id": record.ID, "score": score,
		"area": valueOr(record.Fields["area"], ""), "summary": truncateRunes(pythonString(valueOr(record.Fields["summary"], "")), 500),
		"status": valueOr(record.Fields["status"], ""), "adr": adrOutput,
		"applies": recordSequenceValue(record.Fields["applies"]), "evidence": resultEvidence[:min(len(resultEvidence), maxEvidence)]}
}

// adrDocumentMatches returns the tracked `.md` paths naming the ADR prefix, in
// no particular order (the caller sorts). With sortedPaths supplied it binary
// searches the prefix range instead of scanning every source path.
func adrDocumentMatches(index *Index, sortedPaths []string, prefix string) []string {
	if prefix == "" {
		return nil
	}
	matches := make([]string, 0)
	if sortedPaths == nil {
		for sourcePath := range index.Sources {
			if strings.HasPrefix(sourcePath, prefix) && strings.HasSuffix(sourcePath, ".md") {
				matches = append(matches, sourcePath)
			}
		}
		return matches
	}
	for i := sort.SearchStrings(sortedPaths, prefix); i < len(sortedPaths) && strings.HasPrefix(sortedPaths[i], prefix); i++ {
		if strings.HasSuffix(sortedPaths[i], ".md") {
			matches = append(matches, sortedPaths[i])
		}
	}
	return matches
}

// sortedSourcePaths returns every index.Sources path, sorted, so a caller that
// needs several prefix lookups over the same index (Impact, once per call)
// builds the slice once instead of once per lookup.
func sortedSourcePaths(index *Index) []string {
	paths := make([]string, 0, len(index.Sources))
	for sourcePath := range index.Sources {
		paths = append(paths, sourcePath)
	}
	sort.Strings(paths)
	return paths
}

// dirPathIndex maps each directory to the index.Sources paths under it, so a
// caller that needs several same-directory lookups over the same index
// (Impact, once per call) builds the map once instead of scanning every
// source per lookup.
func dirPathIndex(index *Index) map[string][]string {
	dirs := make(map[string][]string)
	for sourcePath := range index.Sources {
		dir := path.Dir(sourcePath)
		dirs[dir] = append(dirs[dir], sourcePath)
	}
	return dirs
}

func recordSequence(value any, wrapString bool) (any, []any) {
	if pythonFalse(value) {
		return []any{}, nil
	}
	if text, ok := value.(string); ok && wrapString {
		items := []any{text}
		return items, items
	}
	switch typed := value.(type) {
	case []any:
		return typed, typed
	case []string:
		items := make([]any, len(typed))
		for index, item := range typed {
			items[index] = item
		}
		return typed, items
	default:
		return value, []any{value}
	}
}

func recordSequenceValue(value any) any {
	if pythonFalse(value) {
		return []any{}
	}
	return value
}

func pythonFalse(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case bool:
		return !typed
	case int:
		return typed == 0
	case int64:
		return typed == 0
	case []any:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	default:
		return false
	}
}

func pythonString(value any) string {
	switch typed := value.(type) {
	case nil:
		return "None"
	case string:
		return typed
	case bool:
		if typed {
			return "True"
		}
		return "False"
	case pythonInteger:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func pythonContains(value any, wanted string) bool {
	switch typed := value.(type) {
	case string:
		return strings.Contains(typed, wanted)
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && text == wanted {
				return true
			}
		}
	case []string:
		return stringIn(typed, wanted)
	}
	return false
}

func isTestPath(value string) bool {
	name := strings.ToLower(path.Base(value))
	wrapped := "/" + strings.ToLower(value) + "/"
	return strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, "_test.py") || strings.HasSuffix(name, "_test.sh") ||
		strings.Contains(name, ".test.") || strings.Contains(name, ".spec.") || strings.Contains(wrapped, "/test/") || strings.Contains(wrapped, "/tests/")
}

// importer is one resolved reverse-import edge: the source that imports the
// changed path, and the specifier it named it by. The resolution rules that
// produce these live in reverseimports.go.
type importer struct{ path, imported string }

// importEvidenceLine reports the 1-based line of the statement that imports
// the specifier. It reports false when the source is unreadable or no line can
// be shown to import it, so a caller never cites a line the source does not
// support (invariant 2). A Python relative import rarely spells its resolved
// module, so Python lines come from the grammar's own import facts.
func importEvidenceLine(source Source, imported string) (int, bool) {
	text, valid, loaded := source.Text()
	if !loaded || !valid {
		return 0, false
	}
	if strings.HasSuffix(source.Path, ".py") {
		line := pythonImportLine(source.Path, text, imported)
		return line, line > 0
	}
	leaf := imported
	if index := strings.LastIndexAny(leaf, "./"); index >= 0 {
		leaf = leaf[index+1:]
	}
	for index, line := range strings.Split(text, "\n") {
		if strings.Contains(line, imported) || (strings.Contains(line, "import") && containsPythonWord(line, leaf)) {
			return index + 1, true
		}
	}
	return 0, false
}

type nameMatch struct {
	line int
	name string
}
type packageReference struct {
	path    string
	matches []nameMatch
	count   int
}

// samePackageReferences scans every source in changedPath's directory for
// lines that reference a name changedPath declares. samePackageReferencesWithDirIndex
// is the same computation with the index's directory->paths map supplied, so
// a caller scanning many changed paths (Impact, up to maxImpactPaths) builds
// that map once instead of once per changed path (V1-0054).
func samePackageReferences(index *Index, changedPath string, includeTests bool) []packageReference {
	return samePackageReferencesWithDirIndex(index, changedPath, includeTests, nil)
}

func samePackageReferencesWithDirIndex(index *Index, changedPath string, includeTests bool, dirIndex map[string][]string) []packageReference {
	if !strings.HasSuffix(changedPath, ".go") || isTestPath(changedPath) {
		return nil
	}
	declared := make(map[string]struct{})
	for _, symbol := range index.Symbols {
		if symbol.Path == changedPath && utf8.RuneCountInString(symbol.Name) >= 4 {
			declared[symbol.Name] = struct{}{}
		}
	}
	// A name declared twice (two methods named String, repeated init) is one
	// name, as the oracle's set is: each matching code line is one pair.
	names := keys(declared)
	parent := path.Dir(changedPath)
	candidates := dirIndex[parent]
	if dirIndex == nil {
		for candidate := range index.Sources {
			if path.Dir(candidate) == parent {
				candidates = append(candidates, candidate)
			}
		}
	}
	result := make([]packageReference, 0)
	for _, candidate := range candidates {
		if candidate == changedPath || (!includeTests && isTestPath(candidate)) || !strings.HasSuffix(candidate, ".go") {
			continue
		}
		source := index.Sources[candidate]
		text, valid, loaded := source.Text()
		if !loaded || !valid {
			continue
		}
		matches := make([]nameMatch, 0)
		for lineIndex, line := range goCodeLines(text) {
			for _, name := range names {
				if containsPythonWord(line, name) {
					matches = append(matches, nameMatch{lineIndex + 1, name})
				}
			}
		}
		if len(matches) != 0 {
			count := len(matches)
			if len(matches) > maxEvidence {
				matches = matches[:maxEvidence]
			}
			result = append(result, packageReference{candidate, matches, count})
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].path < result[right].path })
	return result
}

func packageReferenceEvidence(index *Index, changedPath string, reference packageReference) []any {
	source := index.Sources[reference.path]
	result := make([]any, 0, len(reference.matches))
	for _, match := range reference.matches {
		result = append(result, evidence(reference.path, match.line, source.BlobHash,
			"references "+match.name+" declared by "+changedPath, "medium", "syntax"))
	}
	return result
}

func containsPythonWord(value, word string) bool {
	if word == "" {
		return false
	}
	for offset := 0; offset <= len(value)-len(word); {
		relative := strings.Index(value[offset:], word)
		if relative < 0 {
			return false
		}
		start := offset + relative
		end := start + len(word)
		leftWord := false
		if start != 0 {
			character, _ := utf8.DecodeLastRuneInString(value[:start])
			leftWord = isPythonWordRune(character)
		}
		rightWord := false
		if end != len(value) {
			character, _ := utf8.DecodeRuneInString(value[end:])
			rightWord = isPythonWordRune(character)
		}
		if !leftWord && !rightWord {
			return true
		}
		offset = start + 1
	}
	return false
}

func goCodeLines(text string) []string {
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	inBlock, inRaw := false, false
	for _, line := range lines {
		var output strings.Builder
		for index := 0; index < len(line); {
			if inBlock {
				end := strings.Index(line[index:], "*/")
				if end < 0 {
					index = len(line)
					continue
				}
				inBlock, index = false, index+end+2
				continue
			}
			if inRaw {
				end := strings.IndexByte(line[index:], '`')
				if end < 0 {
					index = len(line)
					continue
				}
				inRaw, index = false, index+end+1
				continue
			}
			if strings.HasPrefix(line[index:], "//") {
				break
			}
			if strings.HasPrefix(line[index:], "/*") {
				inBlock, index = true, index+2
				continue
			}
			character := line[index]
			if character == '`' {
				inRaw, index = true, index+1
				continue
			}
			if character == '"' || character == '\'' {
				quote := character
				index++
				for index < len(line) {
					if line[index] == '\\' {
						index += 2
					} else if line[index] == quote {
						index++
						break
					} else {
						index++
					}
				}
				continue
			}
			output.WriteByte(character)
			index++
		}
		result = append(result, output.String())
	}
	return result
}

// markerCredited reports whether a same-package test marker relates to the
// change (GPK-V0-070): the test is the changed file's exact twin, references
// a name the changed file declares, or the changed file carries the same key.
func markerCredited(testPath, key, twin string, changedKeys map[string]struct{}, testReferences map[string]packageReference) bool {
	if path.Base(testPath) == twin {
		return true
	}
	if _, ok := testReferences[testPath]; ok {
		return true
	}
	_, ok := changedKeys[key]
	return ok
}

func markerKeysForPath(index *Index, file string) []string {
	result := make([]string, 0)
	for key, markers := range index.Markers {
		for _, marker := range markers {
			if marker.Path == file {
				result = append(result, key)
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

func countKind(keys []string, kind string) int {
	count := 0
	for _, key := range keys {
		if strings.HasPrefix(key, kind+":") {
			count++
		}
	}
	return count
}

func splitRelation(value string) (string, string) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func mergeEvidence(existing, candidate map[string]any) {
	existingEvidence := anySlice(existing["evidence"])
	for _, raw := range anySlice(candidate["evidence"]) {
		item := raw.(map[string]any)
		key := evidenceIdentity(item)
		if !containsEvidence(existingEvidence, key) && len(existingEvidence) < maxEvidence {
			existingEvidence = append(existingEvidence, item)
		}
	}
	existing["evidence"] = existingEvidence
}

type evidenceKey struct {
	path   string
	line   int
	reason string
}

func evidenceIdentity(item map[string]any) evidenceKey {
	line, _ := item["line"].(int)
	return evidenceKey{path: stringValue(item["path"]), line: line, reason: stringValue(item["reason"])}
}

func containsEvidence(items []any, wanted evidenceKey) bool {
	for _, raw := range items {
		if evidenceIdentity(raw.(map[string]any)) == wanted {
			return true
		}
	}
	return false
}

func anySlice(value any) []any {
	if result, ok := value.([]any); ok {
		return result
	}
	return nil
}

func stringIn(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func stringValue(value any) string { result, _ := value.(string); return result }
func valueOr(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}
