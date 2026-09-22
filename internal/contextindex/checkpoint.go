package contextindex

import (
	"path"
	"strings"
)

// CheckpointResults returns current, unranked result identities for the admitted
// critical paths. It consumes no caller prose, trace, snapshot or ranking budget.
// Documents and ledger records retain references to paths outside their own id;
// symbol identities retain their declaration names while refreshing line numbers.
// Relationship results describe the declared eligible path set, as Impact does.
func CheckpointResults(index *Index, paths []string) []map[string]any {
	wanted := map[string]struct{}{}
	for _, file := range paths {
		if _, admitted := index.Sources[file]; admitted {
			wanted[file] = struct{}{}
		}
	}
	if len(wanted) == 0 {
		return nil
	}
	results := make([]map[string]any, 0)
	for _, file := range keys(wanted) {
		source := index.Sources[file]
		results = append(results, map[string]any{"kind": "path", "id": file, "evidence": []any{
			evidence(file, 1, source.BlobHash, "requested changed path", "authoritative", "git-tree"),
		}})
	}
	for _, record := range sortedRecords(index.Documents) {
		result := documentResult(index, record, 0, "current checkpoint document")
		if checkpointResultCites(result, wanted) {
			results = append(results, result)
		}
	}
	for _, records := range []map[string]Record{index.Features, index.Scenarios} {
		for _, record := range sortedRecords(records) {
			result := recordResult(index, record, 0, "current checkpoint record")
			if checkpointResultCites(result, wanted) {
				results = append(results, result)
			}
		}
	}
	for _, symbol := range index.Symbols {
		if _, selected := wanted[symbol.Path]; selected {
			results = append(results, featureSymbolResult(symbol, 0, "current checkpoint declaration"))
		}
	}
	for _, file := range keys(wanted) {
		results = append(results, checkpointRelationships(index, file)...)
	}
	return results
}

func checkpointResultCites(result map[string]any, wanted map[string]struct{}) bool {
	rows, _ := result["evidence"].([]any)
	for _, value := range rows {
		row, _ := value.(map[string]any)
		file, _ := row["path"].(string)
		if _, exists := wanted[file]; exists {
			return true
		}
	}
	return false
}

func checkpointRelationships(index *Index, changed string) []map[string]any {
	results := []map[string]any{}
	if strings.HasSuffix(changed, ".go") && !isTestPath(changed) {
		for file := range index.Sources {
			if path.Dir(file) != path.Dir(changed) || !strings.HasSuffix(file, "_test.go") {
				continue
			}
			confidence := "medium"
			if path.Base(file) == strings.TrimSuffix(path.Base(changed), ".go")+"_test.go" {
				confidence = "high"
			}
			results = append(results, map[string]any{"kind": "test", "id": file, "evidence": []any{
				evidence(file, 1, index.Sources[file].BlobHash, "same-package test for "+changed, confidence, "test-convention"),
			}})
		}
		for key, markers := range index.Markers {
			for _, marker := range markers {
				if !isTestPath(marker.Path) || path.Dir(marker.Path) != path.Dir(changed) {
					continue
				}
				if _, admitted := index.Sources[marker.Path]; !admitted {
					continue
				}
				results = append(results, map[string]any{"kind": "test", "id": marker.Path, "evidence": []any{
					evidence(marker.Path, marker.Line, marker.BlobHash, "same-package test carries "+key, "high", "test-marker"),
				}})
			}
		}
	}
	if !isTestPath(changed) {
		for _, importer := range reverseImporters(index, changed) {
			source := index.Sources[importer.path]
			line, loaded := importEvidenceLine(source, importer.imported)
			if !loaded {
				continue
			}
			results = append(results, map[string]any{"kind": "reverse-import", "id": importer.path, "evidence": []any{
				evidence(importer.path, line, source.BlobHash, "imports "+importer.imported, "high", "syntax"),
			}})
		}
	}
	for _, reference := range samePackageReferences(index, changed, false) {
		rows := []any{}
		for _, match := range reference.matches {
			rows = append(rows, evidence(reference.path, match.line, index.Sources[reference.path].BlobHash,
				"references "+match.name+" declared by "+changed, "medium", "syntax"))
		}
		results = append(results, map[string]any{"kind": "reference", "id": reference.path, "evidence": rows})
	}
	return results
}
