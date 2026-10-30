package contextindex

import (
	"context"
	"testing"
)

// TestMarkerTestImportsMatchBuildUnparsedCount pins the fix for a refused Go
// test-file import extraction being re-run once per marker key that names the
// same path. A file tagged with both a feature and a scenario marker landed in
// two index.Markers buckets; BuildEval's per-marker walk filed one Unparsed row
// for each occurrence where Build's single per-source pass files one, so the
// two builders disagreed on unparsed.count for an identical fixture.
func TestMarkerTestImportsMatchBuildUnparsedCount(t *testing.T) {
	root := testRepository(t)
	// The malformed import (no path, no quotes) makes go/parser refuse and
	// leaves the regex fallback nothing to recover, so every visit of this
	// path files a refusal and none ever populates index.Imports[path] - the
	// exact condition the old success-only guard did not dedupe against.
	writeTestFile(t, root, "internal/widget/widget_test.go",
		"package widget\n\n// feature:sample-widget\n// scenario:sample-widget-case\n\nimport !!!\n\nfunc TestWidget(t *T) {}\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "marker test file")

	built, err := Build(context.Background(), root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	evalBuilt, err := BuildEval(context.Background(), root)
	if err != nil {
		t.Fatalf("BuildEval: %v", err)
	}

	const path = "internal/widget/widget_test.go"
	buildImportRows := countUnparsed(built.Unparsed, path, "imports")
	evalImportRows := countUnparsed(evalBuilt.Unparsed, path, "imports")
	if evalImportRows != 1 {
		t.Fatalf("BuildEval filed %d 'imports' Unparsed rows for %s across its two marker keys, want 1 (no duplicate)", evalImportRows, path)
	}
	if buildImportRows != evalImportRows {
		t.Fatalf("'imports' unparsed rows diverge: Build=%d for %s, BuildEval=%d", buildImportRows, path, evalImportRows)
	}
	if len(built.Unparsed) != len(evalBuilt.Unparsed) {
		t.Fatalf("unparsed.count = %d (Build) vs %d (BuildEval), want equal", len(built.Unparsed), len(evalBuilt.Unparsed))
	}
}

func countUnparsed(rows []Unparsed, path, facts string) int {
	count := 0
	for _, row := range rows {
		if row.Path == path && row.Facts == facts {
			count++
		}
	}
	return count
}
