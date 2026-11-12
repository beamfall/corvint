package contextindex

import (
	"context"
	"testing"
)

// TestGoImportsReportsRefusal fixes the same contract for the import table.
// goImports already unioned the scanner in on a parse failure, but reported
// nothing about having done so; the reason now reaches the caller.
func TestGoImportsReportsRefusal(t *testing.T) {
	imports, refusal := goImports("import (\n\t\"g/one\"\n)\n")
	if refusal == "" {
		t.Fatal("unparseable source reported no refusal")
	}
	if _, ok := imports["g/one"]; !ok {
		t.Fatalf("fallback dropped the scanner edge: %v", imports)
	}
	if _, clean := goImports("package p\n\nimport \"g/one\"\n"); clean != "" {
		t.Fatalf("parseable source reported refusal %q", clean)
	}
}

// TestSourceImportsMarksScannerEdgesApproximate pins the non-Go path's
// limitation as an observable rather than a comment. genericImport has no
// comment, string or dynamic-import awareness; an audit of eleven edges it
// found that a real JavaScript analyzer did not showed every one to be
// spurious -- two inside `//` comments, two dynamic `import()`, one `vi.mock`,
// and four multi-line string bodies recorded as module names.
func TestSourceImportsMarksScannerEdgesApproximate(t *testing.T) {
	web := sourceImports("web/caller.ts", "// import \"commented/out\"\nawait import(\"dynamic/one\")\n")
	if !web.approximate {
		t.Fatal("scanner-derived edges are not marked approximate")
	}
	if len(web.imports) == 0 {
		t.Fatal("fixture does not exercise the over-approximation")
	}
	if web.refusal != "" {
		t.Fatalf("refusal = %q; the scanner refuses nothing", web.refusal)
	}
	if goSource := sourceImports("pkg/caller.go", "package p\n\nimport \"g/one\"\n"); goSource.approximate {
		t.Fatal("grammar-derived Go edges marked approximate")
	}
}

// TestForbiddenPathExcludesAgentWorktreeCopies pins the `.claude` exclusion. An
// agent worktree under `.claude/` holds copies of the repository's own files;
// indexed as first-party source they inflate the symbol population with stale
// duplicates that outrank nothing and dilute everything.
func TestForbiddenPathExcludesAgentWorktreeCopies(t *testing.T) {
	for _, excluded := range []string{".claude/worktrees/lane/internal/token/token.go", ".claude/settings.json"} {
		if forbiddenPath(excluded) == "" {
			t.Fatalf("forbiddenPath(%q) admitted an agent worktree copy", excluded)
		}
	}
	if reason := forbiddenPath("internal/claude/token.go"); reason != "" {
		t.Fatalf("forbiddenPath over-matched a first-party path: %q", reason)
	}
}

// TestIndexRecordsUnparsedSource is the countability test: a source the grammar
// refuses must leave a trace on the built index, not just a short symbol table.
func TestIndexRecordsUnparsedSource(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, "internal/token/broken.go", "package token\n\nfunc Visible() {}\n\nfunc (\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "broken")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(index.Unparsed) == 0 {
		t.Fatal("a refused source left no record on the index")
	}
	found := false
	for _, item := range index.Unparsed {
		if item.Path == "internal/token/broken.go" && item.Facts == "symbols" && item.Reason != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Unparsed = %#v", index.Unparsed)
	}
	packet, err := receipt(index, "query", map[string]any{}, nil, 1, "")
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	reported, ok := packet["unparsed"].(map[string]any)
	if !ok {
		t.Fatalf("receipt does not surface the refusal: %v", packet["unparsed"])
	}
	if reported["count"] != len(index.Unparsed) {
		t.Fatalf("receipt unparsed count = %v, want %d", reported["count"], len(index.Unparsed))
	}
}

// TestReceiptOmitsUnparsedWhenEverythingParses keeps the field from becoming a
// structurally-zero key nobody reads, and keeps a receipt over a clean
// repository byte-identical to one built before the field existed.
func TestReceiptOmitsUnparsedWhenEverythingParses(t *testing.T) {
	root := testRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(index.Unparsed) != 0 {
		t.Fatalf("clean repository reported refusals: %#v", index.Unparsed)
	}
	packet, err := receipt(index, "query", map[string]any{}, nil, 1, "")
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	if _, present := packet["unparsed"]; present {
		t.Fatal("receipt carries an always-zero unparsed key")
	}
}
