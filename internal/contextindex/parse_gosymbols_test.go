package contextindex

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func testSource() Source {
	return Source{Path: "sample.go", BlobHash: strings.Repeat("0", 40)}
}

func symbolKeys(symbols []Symbol) []string {
	result := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		result = append(result, fmt.Sprintf("%s %s:%d", symbol.Kind, symbol.Name, symbol.Line))
	}
	sort.Strings(result)
	return result
}

func containsSymbol(symbols []Symbol, key string) bool {
	for _, candidate := range symbolKeys(symbols) {
		if candidate == key {
			return true
		}
	}
	return false
}

// TestGoSymbolsExcludesFunctionLocalDeclarations pins the first of the three
// defects a line scanner cannot avoid: with no lexical state it cannot tell a
// package-level `var` from one inside a function body, so it promotes every
// local binding to a top-level symbol. Live instance:
// `beamfall/cmd/beamfall/admin.go:25`, `var password string` inside
// `runResetAdminPassword`. Function-local declarations are 9,939 of the
// scanner's 10,180 false positives over the workspace corpus.
func TestGoSymbolsExcludesFunctionLocalDeclarations(t *testing.T) {
	text := "package sample\n\nvar TopLevel = 1\n\nfunc Run() {\n\tvar password string\n\tconst localLimit = 3\n\ttype localAlias = int\n\t_, _, _ = password, localLimit, localAlias(0)\n}\n"
	symbols, refusal := goSymbols(testSource(), text)
	if refusal != "" {
		t.Fatalf("refusal = %q", refusal)
	}
	want := []string{"func Run:5", "var TopLevel:3"}
	if got := symbolKeys(symbols); !stringSlicesEqual(got, want) {
		t.Fatalf("goSymbols = %v, want %v", got, want)
	}
	// The fixture discriminates: the retained scanner does emit the locals.
	for _, key := range []string{"var password:6", "var localLimit:7", "type localAlias:8"} {
		if !containsSymbol(goSymbolsScan(testSource(), text), key) {
			t.Fatalf("fixture does not exercise the defect: scanner did not emit %q", key)
		}
	}
}

// TestGoSymbolsNamesEveryGroupMember pins the second defect: a parenthesized
// declaration group opens with a line the scanner matches and then declares its
// members on lines the scanner's pattern cannot reach, so every member is lost.
// Live instance: `beamfall/internal/migrate/migrate.go:50-58`, a nine-member
// `type (...)` group of which the scanner names none. Group members and
// multi-name specs are 4,195 of its 4,333 false negatives.
func TestGoSymbolsNamesEveryGroupMember(t *testing.T) {
	text := "package sample\n\nconst (\n\tAlpha = 1\n\tBeta  = 2\n)\n\ntype (\n\tGamma struct{}\n\tDelta = int\n)\n\nvar Epsilon, Zeta = 3, 4\n"
	symbols, refusal := goSymbols(testSource(), text)
	if refusal != "" {
		t.Fatalf("refusal = %q", refusal)
	}
	want := []string{"type Delta:10", "type Gamma:9", "var Alpha:4", "var Beta:5", "var Epsilon:13", "var Zeta:13"}
	if got := symbolKeys(symbols); !stringSlicesEqual(got, want) {
		t.Fatalf("goSymbols = %v, want %v", got, want)
	}
	// The fixture discriminates: the scanner reaches none of the six group
	// members and stops at the first name of the two-name spec.
	scanned := []string{"var Epsilon:13"}
	if got := symbolKeys(goSymbolsScan(testSource(), text)); !stringSlicesEqual(got, scanned) {
		t.Fatalf("fixture does not exercise the defect: scanner emitted %v, want %v", got, scanned)
	}
}

// TestGoSymbolsIgnoresRawStringInterior pins the third defect: a backquoted raw
// string can hold an entire Go file, and a scanner reading line by line has no
// way to know it is inside one. Live instance:
// `beamfall/cmd/ioe2e/main_test.go:102`, a `func TestSample` inside the
// `goodMarkerFile` fixture constant, which the scanner reports as a test the
// package does not have.
func TestGoSymbolsIgnoresRawStringInterior(t *testing.T) {
	text := "package sample\n\nconst goodMarkerFile = `package inner\n\nfunc TestSample(t *testing.T) {}\n\ntype Inner struct{}\n`\n"
	symbols, refusal := goSymbols(testSource(), text)
	if refusal != "" {
		t.Fatalf("refusal = %q", refusal)
	}
	want := []string{"var goodMarkerFile:3"}
	if got := symbolKeys(symbols); !stringSlicesEqual(got, want) {
		t.Fatalf("goSymbols = %v, want %v", got, want)
	}
	for _, key := range []string{"func TestSample:5", "type Inner:7"} {
		if !containsSymbol(goSymbolsScan(testSource(), text), key) {
			t.Fatalf("fixture does not exercise the defect: scanner did not emit %q", key)
		}
	}
}

// TestGoSymbolsNamesGenericDeclarations covers a false-negative class the
// scanner's suffix guards create rather than miss by accident: both require the
// character after the name to be `(` or a space, and a type parameter list
// opens with `[`.
func TestGoSymbolsNamesGenericDeclarations(t *testing.T) {
	text := "package sample\n\ntype Pair[K comparable, V any] struct{}\n\nfunc Map[T any](values []T) []T { return values }\n"
	symbols, _ := goSymbols(testSource(), text)
	want := []string{"func Map:5", "type Pair:3"}
	if got := symbolKeys(symbols); !stringSlicesEqual(got, want) {
		t.Fatalf("goSymbols = %v, want %v", got, want)
	}
}

// TestGoSymbolsReportsRefusalAndStaysNonEmpty fixes the parse-failure contract.
// The failure mode this guards is not a wrong symbol but a silent zero: an
// extractor that returns nothing on a source it could not read, while the
// source stays counted as indexed, reports success over partial data and leaves
// no counter able to observe the omission.
func TestGoSymbolsReportsRefusalAndStaysNonEmpty(t *testing.T) {
	broken := "package sample\n\nfunc Visible() {}\n\nfunc (\n"
	symbols, refusal := goSymbols(testSource(), broken)
	if refusal == "" {
		t.Fatal("unparseable source reported no refusal")
	}
	if len(symbols) == 0 {
		t.Fatal("unparseable source produced a silent zero instead of the fallback scan")
	}
	if got, want := symbolKeys(symbols), symbolKeys(goSymbolsScan(testSource(), broken)); !stringSlicesEqual(got, want) {
		t.Fatalf("fallback = %v, want the scanner's %v", got, want)
	}
}

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
