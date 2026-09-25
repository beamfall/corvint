package contextindex

import (
	"context"
	"testing"
)

// pythonImpactRepository is a two-language fixture: one Python module with a
// Python importer, and one Go package with a Go importer. The Go half is the
// control -- it proves the coverage note below is earned by the language, not
// pasted onto every impact receipt.
func pythonImpactRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if command := testGitOptional(t, root, "init", "-q"); command != "" {
		t.Skipf("git init unsupported: %s", command)
	}
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		"go.mod":                  "module example.test/fixture\n\ngo 1.27.0\n",
		"src/harness.py":          "def run() -> str:\n    return \"harness\"\n",
		"src/cli.py":              "import harness\n\n\ndef main() -> str:\n    return harness.run()\n",
		"internal/token/token.go": "package token\n\nfunc MintToken() string { return \"token\" }\n",
		"internal/api/api.go":     "package api\n\nimport \"example.test/fixture/internal/token\"\n\nvar _ = token.MintToken\n",
	}
	for relative, content := range files {
		writeTestFile(t, root, relative, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "python impact fixture")
	return root
}

// TestImpactNeverClaimsCompleteCoverageOverUncomputedReverseImports is the
// regression for DR-0006.
//
// `reverseImporters` searches `index.Imports` for one target only:
// `module + "/" + dir(changedPath)`, a slash-qualified Go import path. A Python
// import names a dotted module, so for a `.py` changed path that search is
// vacuous by construction -- `src/cli.py` imports `harness` and can never be
// found. The candidate then reported `omitted_results: 0`, `uncertainty: []`
// and `critical_missing: []` over that answer, which is a positive assertion
// that nothing was left out, made over an answer that left three results out on
// the measured subject.
//
// The invariant asserted here outlives the current shortfall: a receipt may
// resolve the reverse-import universe, or it may declare that it did not, but
// it may never assert complete coverage while silently omitting the dimension.
// Implementing Python reverse-imports would satisfy this test through its first
// branch rather than falsifying it.
func TestImpactNeverClaimsCompleteCoverageOverUncomputedReverseImports(t *testing.T) {
	root := pythonImpactRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Impact(index, []string{"src/harness.py"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	resolved := false
	for _, item := range mapsFromAny(result["results"]) {
		if item["kind"] == "reverse-import" {
			resolved = true
		}
	}
	coverage := result["coverage"].(map[string]any)
	uncertainty := anySlice(coverage["uncertainty"])
	declared := anyContains(uncertainty, "outside the native Go impact profile")
	if !resolved && !declared {
		t.Fatalf("src/cli.py imports harness and no reverse-import result resolved it, yet coverage reports omitted_results=%v uncertainty=%v critical_missing=%v: complete coverage asserted over an incomplete answer (DR-0006)",
			coverage["omitted_results"], uncertainty, coverage["critical_missing"])
	}
}

// TestImpactCoverageWithholdsTheProfileNoteForGoPaths is the other half of the
// claim. A note every receipt carries is a note every reader learns to skip, and
// a hedge attached to an answer that is not short is a false one. A Go changed
// path resolves its reverse-importers, so its coverage block must stay exactly
// as it was -- which is also what keeps a clean Go repository's receipt
// byte-identical to one built before this note existed.
func TestImpactCoverageWithholdsTheProfileNoteForGoPaths(t *testing.T) {
	root := pythonImpactRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Impact(index, []string{"internal/token/token.go"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	resolved := false
	for _, item := range mapsFromAny(result["results"]) {
		if item["kind"] == "reverse-import" && item["id"] == "internal/api/api.go" {
			resolved = true
		}
	}
	if !resolved {
		t.Fatal("fixture no longer resolves internal/api/api.go as a reverse-importer, so the control proves nothing")
	}
	uncertainty := anySlice(result["coverage"].(map[string]any)["uncertainty"])
	if len(uncertainty) != 0 {
		t.Fatalf("uncertainty = %v, want empty: a Go changed path withholds no reverse-import dimension", uncertainty)
	}
}

// workspaceImpactRepository is a two-package web workspace. The worker test in
// packages/scraper imports `@fixture/contracts`, whose barrel re-exports
// topics.ts, so it reaches topics.ts through a bare package specifier rule (c)
// does not resolve. packages/leaf is a named package nobody imports by name:
// the control that keeps the disclosure off an answer that is not short.
func workspaceImpactRepository(t *testing.T) string {
	t.Helper()
	return impactRepositoryWithFiles(t, map[string]string{
		"package.json":                        "{\"private\": true, \"workspaces\": [\"packages/*\"]}\n",
		"packages/contracts/package.json":     "{\"name\": \"@fixture/contracts\", \"main\": \"src/index.ts\"}\n",
		"packages/contracts/src/index.ts":     "export * from \"./topics\";\n",
		"packages/contracts/src/topics.ts":    "export const SCRAPER_TOPIC = \"scraper\";\n",
		"packages/scraper/package.json":       "{\"name\": \"@fixture/scraper\"}\n",
		"packages/scraper/src/worker.test.ts": "import { SCRAPER_TOPIC } from \"@fixture/contracts\";\n\nexport const topic = SCRAPER_TOPIC;\n",
		"packages/leaf/package.json":          "{\"name\": \"@fixture/leaf\"}\n",
		"packages/leaf/src/main.ts":           "export const leaf = \"leaf\";\n",
		"packages/leaf/src/uses-main.ts":      "import { leaf } from \"./main\";\n\nexport const used = leaf;\n",
	})
}

// TestImpactDisclosesWorkspacePackageImporters is the regression for panel
// blocker B3 (GPK-V0-067, proposed). Rule (c) resolves relative and alias
// specifiers only, so the worker test that imports topics.ts through
// `@fixture/contracts` was never found, and the receipt still reported
// `uncertainty: []`: complete coverage asserted over an answer missing a
// cross-package importer. As in the DR-0006 test, resolving the importer or
// disclosing the gap both satisfy it; asserting completeness does not.
func TestImpactDisclosesWorkspacePackageImporters(t *testing.T) {
	root := workspaceImpactRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Impact(index, []string{"packages/contracts/src/topics.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	resolved := reverseImportIDs(result)["packages/scraper/src/worker.test.ts"] != nil
	uncertainty := anySlice(result["coverage"].(map[string]any)["uncertainty"])
	declared := anyContains(uncertainty, "importable by workspace package name are unresolved")
	if !resolved && !declared {
		t.Fatalf("packages/scraper/src/worker.test.ts imports @fixture/contracts and no reverse-import result resolved it, yet uncertainty = %v: complete coverage asserted over an incomplete answer (B3)", uncertainty)
	}

	control, err := Impact(index, []string{"packages/leaf/src/main.ts"}, maxLimit)
	if err != nil {
		t.Fatal(err)
	}
	if reverseImportIDs(control)["packages/leaf/src/uses-main.ts"] == nil {
		t.Fatal("fixture no longer resolves packages/leaf/src/uses-main.ts as a reverse-importer, so the control proves nothing")
	}
	if controlUncertainty := anySlice(control["coverage"].(map[string]any)["uncertainty"]); anyContains(controlUncertainty, "workspace package name") {
		t.Fatalf("uncertainty = %v, want no workspace disclosure: nothing imports @fixture/leaf by name", controlUncertainty)
	}
}
