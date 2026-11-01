package verify

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/patch"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// writeGuardTree writes files under a fresh copy root.
func writeGuardTree(t *testing.T, files map[string]string) string {
	t.Helper()
	tree := t.TempDir()
	for name, data := range files {
		full := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return tree
}

const guardSource = "package p\n\n" + // 1-2
	"func helper(v int) int {\n\treturn v + 1\n}\n\n" + // 3-6
	"func Exported(v int) int {\n\treturn helper(v) * 2\n}\n\n" + // 7-10
	"func Other() int {\n\treturn 7\n}\n" // 11-13

const guardTests = "package p\n\nimport \"testing\"\n\n" + // 1-4
	"func TestExported(t *testing.T) {\n\tif Exported(1) != 4 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n\n" + // 5-10
	"func TestOther(t *testing.T) {\n\tif Other() != 7 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n\n" + // 11-16
	"func TestViaHelper(t *testing.T) {\n\tcheck(t)\n}\n\n" + // 17-20
	"func check(t *testing.T) {\n\t_ = helper(0)\n}\n\n" + // 21-24
	"func TestWrapper(t *testing.T) {\n\tTestOther(t)\n}\n" // 25-27

// TestTCQV0062CandidatesFollowThePackageClosure pins candidate naming: a
// change to an unexported helper reaches the test that calls the exported
// function using it and the test that reaches it through a test helper, a
// change to Other reaches its test and the test that calls that test, and
// neither reaches the test of an unrelated declaration. A test absent at the base cannot
// be cited, so it is never a candidate; a change outside every declaration
// names none.
func TestTCQV0062CandidatesFollowThePackageClosure(t *testing.T) {
	t.Parallel()
	tree := writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": guardTests})
	base := func(string) []byte { return []byte(guardTests) }
	pkg := loadGoPackage(tree, "p", base)
	if got := pkg.candidates("p/p.go", map[int64]bool{4: true}); !reflect.DeepEqual(got, []string{"TestExported", "TestViaHelper"}) {
		t.Fatalf("helper change candidates = %v", got)
	}
	if got := pkg.candidates("p/p.go", map[int64]bool{12: true}); !reflect.DeepEqual(got, []string{"TestOther", "TestWrapper"}) {
		t.Fatalf("Other change candidates = %v", got)
	}
	if got := pkg.candidates("p/p.go", map[int64]bool{2: true}); len(got) != 0 {
		t.Fatalf("a change outside every declaration named %v", got)
	}
	absent := loadGoPackage(tree, "p", func(string) []byte { return nil })
	if got := absent.candidates("p/p.go", map[int64]bool{12: true}); len(got) != 0 {
		t.Fatalf("a test absent at the base was named: %v", got)
	}
}

// TestTCQV0970LocalsDoNotCarryTheClosure pins that a parameter, a local
// variable, a range variable and a function literal that share a changed
// declaration's name are not mentions of it, while a call of the package
// declaration, a method selector and a reference from another file still are.
func TestTCQV0970LocalsDoNotCarryTheClosure(t *testing.T) {
	t.Parallel()
	source := "package p\n\n" + // 1-2
		"func open() int {\n\treturn 1\n}\n\n" + // 3-5
		"func Shadows(open int) int {\n\tfor _, open := range []int{open} {\n\t\t_ = open\n\t}\n\treturn open\n}\n\n" + // 7-12
		"func Local() int {\n\topen := func() int { return 2 }\n\treturn open()\n}\n\n" + // 14-17
		"func Calls() int {\n\treturn open()\n}\n\n" + // 19-21
		"type T struct{}\n\nfunc (T) Method() int {\n\treturn 3\n}\n" // 23-27
	other := "package p\n\nfunc FromOtherFile() int {\n\treturn Calls() + T{}.Method()\n}\n"
	tests := "package p\n\nimport \"testing\"\n\n" +
		"func TestShadows(t *testing.T) { _ = Shadows(1) }\n" +
		"func TestLocal(t *testing.T) { _ = Local() }\n" +
		"func TestCalls(t *testing.T) { _ = Calls() }\n" +
		"func TestOtherFile(t *testing.T) { _ = FromOtherFile() }\n" +
		"func TestMethodLocal(t *testing.T) { Method := 1; _ = Method }\n" +
		"func TestMethod(t *testing.T) { var v T; _ = v.Method() }\n"
	tree := writeGuardTree(t, map[string]string{"p/p.go": source, "p/q.go": other, "p/p_test.go": tests})
	pkg := loadGoPackage(tree, "p", func(string) []byte { return []byte(tests) })
	if got := pkg.candidates("p/p.go", map[int64]bool{4: true}); !reflect.DeepEqual(got, []string{"TestCalls", "TestOtherFile"}) {
		t.Fatalf("open change candidates = %v", got)
	}
	if got := pkg.candidates("p/p.go", map[int64]bool{26: true}); !reflect.DeepEqual(got, []string{"TestMethod", "TestOtherFile"}) {
		t.Fatalf("Method change candidates = %v", got)
	}
}

// TestTCQV0062LineDirectivesKeepPhysicalLines pins that declarations and the
// cited header are located by physical line, so a //line directive in either
// file moves neither away from the patch lines and the base span.
func TestTCQV0062LineDirectivesKeepPhysicalLines(t *testing.T) {
	t.Parallel()
	source := "package p\n\n//line gen.go:100\nfunc Gen() int {\n\treturn 1\n}\n"                                        // Gen: 4-6
	tests := "package p\n\nimport \"testing\"\n\n//line gen_test.go:500\nfunc TestGen(t *testing.T) {\n\t_ = Gen()\n}\n" // TestGen: 6-8
	tree := writeGuardTree(t, map[string]string{"p/g.go": source, "p/g_test.go": tests})
	pkg := loadGoPackage(tree, "p", func(string) []byte { return []byte(tests) })
	if got := pkg.candidates("p/g.go", map[int64]bool{5: true}); !reflect.DeepEqual(got, []string{"TestGen"}) {
		t.Fatalf("candidates = %v", got)
	}
	want := &GuardCitation{Path: "p/g_test.go", Lines: "6:6", Test: "TestGen"}
	if got := pkg.citation(tree, []string{"TestGen"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("citation = %+v", got)
	}
}

// TestTCQV0062CitationNeedsAStableBaseHeader pins the plan row's span: the
// base lines of the guard's header, cited only when the frozen drift
// classifier keeps them stable or relocated at the target, as cem cite does.
func TestTCQV0062CitationNeedsAStableBaseHeader(t *testing.T) {
	t.Parallel()
	tree := writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": guardTests})
	pkg := loadGoPackage(tree, "p", func(string) []byte { return []byte(guardTests) })
	want := &GuardCitation{Path: "p/p_test.go", Lines: "11:11", Test: "TestOther"}
	if got := pkg.citation(tree, []string{"TestOther"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("unchanged citation = %+v", got)
	}
	// Moved down two lines at the target: relocated, still the base lines.
	moved := "package p\n\n// moved\n" + guardTests[len("package p\n"):]
	tree = writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": moved})
	pkg = loadGoPackage(tree, "p", func(string) []byte { return []byte(guardTests) })
	if got := pkg.citation(tree, []string{"TestOther"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("relocated citation = %+v", got)
	}
	// The header line occurs twice at the target: ambiguous, never cited.
	twice := guardTests + "\nconst header = `\nfunc TestOther(t *testing.T) {\n`\n"
	tree = writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": twice})
	pkg = loadGoPackage(tree, "p", func(string) []byte { return []byte(guardTests) })
	if got := pkg.citation(tree, []string{"TestOther"}); got != nil {
		t.Fatalf("ambiguous header cited: %+v", got)
	}
}

// TestTCQV0063PlanRowRespectsTheCiteSpanBound pins that a guard whose header
// span exceeds the cem cite bound, wire.MaxCitationSpanBytes, is never
// printed, the next guard by name is cited instead, and a header exactly at
// the bound is still cited.
func TestTCQV0063PlanRowRespectsTheCiteSpanBound(t *testing.T) {
	t.Parallel()
	header := func(size int) string {
		prefix, suffix := "func TestBig(t *testing.T /*", "*/) {\n"
		return prefix + strings.Repeat("x", size-len(prefix)-len(suffix)) + suffix + "\t_ = Other()\n}\n"
	}
	for _, c := range []struct {
		size int
		want *GuardCitation
	}{
		{wire.MaxCitationSpanBytes, &GuardCitation{Path: "p/p_test.go", Lines: "29:29", Test: "TestBig"}},
		{wire.MaxCitationSpanBytes + 1, &GuardCitation{Path: "p/p_test.go", Lines: "11:11", Test: "TestOther"}},
	} {
		tests := guardTests + "\n" + header(c.size)
		tree := writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": tests})
		pkg := loadGoPackage(tree, "p", func(string) []byte { return []byte(tests) })
		if got := pkg.citation(tree, []string{"TestBig", "TestOther"}); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("header of %d bytes: citation = %+v, want %+v", c.size, got, c.want)
		}
	}
}

// TestTCQV0062ChangedLinesFindTheEditedDeclaration pins that an added line is
// its own target line, a pure deletion marks the lines on either side, and a
// replacement marks only its added lines, never the untouched line before it.
func TestTCQV0062ChangedLinesFindTheEditedDeclaration(t *testing.T) {
	t.Parallel()
	added := &patch.Hunk{NewRange: wire.Range{Start: 3, Count: 3}, Body: []patch.BodyLine{{Prefix: ' '}, {Prefix: '+'}, {Prefix: ' '}}}
	if got := changedLines(added); !reflect.DeepEqual(got, map[int64]bool{4: true}) {
		t.Fatalf("added = %v", got)
	}
	deleted := &patch.Hunk{NewRange: wire.Range{Start: 3, Count: 2}, Body: []patch.BodyLine{{Prefix: ' '}, {Prefix: '-'}, {Prefix: ' '}}}
	if got := changedLines(deleted); !reflect.DeepEqual(got, map[int64]bool{3: true, 4: true}) {
		t.Fatalf("deleted = %v", got)
	}
	replaced := &patch.Hunk{NewRange: wire.Range{Start: 3, Count: 3}, Body: []patch.BodyLine{{Prefix: ' '}, {Prefix: '-'}, {Prefix: '+'}, {Prefix: ' '}}}
	if got := changedLines(replaced); !reflect.DeepEqual(got, map[int64]bool{4: true}) {
		t.Fatalf("replaced = %v", got)
	}
}

// TestTCQV0062SpentBudgetKeepsNoCandidateReason pins the reason of a row
// once the total bound is spent. Spent after naming, a hunk with a candidate
// test is total-limit, but a hunk with none would run nothing, so it stays
// no-candidate-test. Spent before naming finished, an absent test may be a
// base read the bound cut short, so every row is total-limit.
func TestTCQV0062SpentBudgetKeepsNoCandidateReason(t *testing.T) {
	t.Parallel()
	tree := writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": guardTests})
	name := "p/p.go"
	at := func(line int64) guardHunk {
		body := []patch.BodyLine{{Prefix: '+', Payload: []byte("\n")}}
		if line == 12 {
			body = []patch.BodyLine{{Prefix: '-', Payload: []byte("\treturn 8\n")}, {Prefix: '+', Payload: []byte("\treturn 7\n")}}
		}
		hunk := &patch.Hunk{NewPath: &name, NewRange: wire.Range{Start: line, Count: 1}, Body: body}
		return guardHunk{hunk: hunk, kind: patch.KindModify}
	}
	base := func(string) []byte { return []byte(guardTests) }
	// The first run spends the bound, after every hunk was named.
	total, cancel := context.WithCancel(context.Background())
	defer cancel()
	runs := 0
	spend := func(context.Context, string, string, []string) runOutcome { runs++; cancel(); return runOutcome{} }
	hunks := []guardHunk{at(12), at(2), at(12)}
	rows := make([]GuardRow, len(hunks))
	judgeGuards(total, tree, hunks, rows, DefaultHunkRevertLimits, spend, base)
	if runs != 1 || rows[1].Reason != GuardNoCandidate || rows[2].Reason != RevertTotalLimit {
		t.Fatalf("spent after naming: runs=%d rows=%+v", runs, rows)
	}
	// The bound was spent before naming finished.
	spent, cancelled := context.WithCancel(context.Background())
	cancelled()
	ran := false
	run := func(context.Context, string, string, []string) runOutcome { ran = true; return runOutcome{} }
	hunks = []guardHunk{at(2), at(12)}
	rows = make([]GuardRow, len(hunks))
	judgeGuards(spent, tree, hunks, rows, DefaultHunkRevertLimits, run, base)
	if ran || rows[0].Reason != RevertTotalLimit || rows[1].Reason != RevertTotalLimit {
		t.Fatalf("spent before naming: ran=%v rows=%+v", ran, rows)
	}
}
