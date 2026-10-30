package contextindex

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCheckpointResultsAreUnrankedAndReuseCurrentResultConstructors(t *testing.T) {
	index := &Index{Module: "example.test/checkpoint", Sources: map[string]Source{
		"pkg/code.go":  {Path: "pkg/code.go", BlobHash: "code"},
		"docs/spec.md": {Path: "docs/spec.md", BlobHash: "spec"},
	}, Documents: map[string]Record{}, Features: map[string]Record{}, Scenarios: map[string]Record{}, Markers: map[string][]Marker{}}
	record := Record{Kind: "spec", ID: "docs/spec.md", Path: "docs/spec.md", BlobHash: "spec", Line: 4,
		Fields: map[string]any{"status": "accepted", "references": []any{"pkg/code.go", "pkg/code.go"}}}
	index.Documents[record.Path] = record
	for i := 0; i < 70; i++ {
		index.Symbols = append(index.Symbols, Symbol{Kind: "function", Name: fmt.Sprintf("Symbol%02d", i), Path: "pkg/code.go", BlobHash: "code", Line: i + 1})
	}
	results := CheckpointResults(index, []string{"pkg/code.go"})
	symbols := 0
	foundDocument := false
	for _, result := range results {
		if result["kind"] == "symbol" {
			symbols++
		}
		if result["kind"] == "spec" {
			foundDocument = true
			expected := documentResult(index, record, 0, "current checkpoint document")
			if !reflect.DeepEqual(result, expected) {
				t.Fatalf("document shape diverged: %v", result)
			}
			rows := result["evidence"].([]any)
			if len(rows) != 3 || rows[1].(map[string]any)["path"] != "pkg/code.go" {
				t.Fatal(rows)
			}
		}
	}
	if symbols != 70 || !foundDocument {
		t.Fatalf("ranking cap or own-id lookup lost results: %d %v", symbols, foundDocument)
	}
	if got := CheckpointResults(index, []string{"unadmitted.bin"}); len(got) != 0 {
		t.Fatal("unadmitted selector acquired rows", got)
	}
}

func TestCheckpointMarkerResultsRequireAdmittedSourcesAndEligibleCode(t *testing.T) {
	index := &Index{Sources: map[string]Source{
		"pkg/code.go":      {Path: "pkg/code.go", BlobHash: "code"},
		"pkg/code_test.go": {Path: "pkg/code_test.go", BlobHash: "test"},
	}, Markers: map[string][]Marker{
		"feature:excluded": {{Path: "pkg/excluded_test.go", BlobHash: "excluded", Line: 3}},
		"feature:other":    {{Path: "other/code_test.go", BlobHash: "other", Line: 3}},
	}}
	for i := 0; i < 70; i++ {
		index.Markers[fmt.Sprintf("feature:marker%02d", i)] = []Marker{{Path: "pkg/code_test.go", BlobHash: "test", Line: i + 1}}
	}
	for _, test := range []struct {
		paths []string
		want  int
	}{
		{[]string{"pkg/code.go", "pkg/code_test.go"}, 70},
		{[]string{"pkg/code_test.go"}, 0},
		{[]string{"pkg/excluded.go"}, 0},
	} {
		count := 0
		for _, result := range CheckpointResults(index, test.paths) {
			for _, value := range result["evidence"].([]any) {
				row := value.(map[string]any)
				if row["authority"] != "test-marker" {
					continue
				}
				count++
				if result["kind"] != "test" || result["id"] != "pkg/code_test.go" || row["path"] != "pkg/code_test.go" || row["blob_hash"] != "test" {
					t.Fatalf("unadmitted or unrelated marker acquired rows: %v", result)
				}
			}
		}
		if count != test.want {
			t.Fatalf("paths=%v got %d marker rows, want %d", test.paths, count, test.want)
		}
	}
}
