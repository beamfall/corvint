package verify

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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
	"func check(t *testing.T) {\n\t_ = helper(0)\n}\n" // 21-23

// TestTCQV0062CandidatesFollowThePackageClosure pins candidate naming: a
// change to an unexported helper reaches the test that calls the exported
// function using it and the test that reaches it through a test helper, and
// never the test of an unrelated declaration. A test absent at the base cannot
// be cited, so it is never a candidate; a change outside every declaration
// names none.
func TestTCQV0062CandidatesFollowThePackageClosure(t *testing.T) {
	tree := writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": guardTests})
	base := func(string) []byte { return []byte(guardTests) }
	pkg := loadGoPackage(tree, "p", base)
	if got := pkg.candidates("p/p.go", map[int64]bool{4: true}); !reflect.DeepEqual(got, []string{"TestExported", "TestViaHelper"}) {
		t.Fatalf("helper change candidates = %v", got)
	}
	if got := pkg.candidates("p/p.go", map[int64]bool{12: true}); !reflect.DeepEqual(got, []string{"TestOther"}) {
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

// TestTCQV0062CitationNeedsAStableBaseHeader pins the plan row's span: the
// base lines of the guard's header, cited only when the frozen drift
// classifier keeps them stable or relocated at the target, as cem cite does.
func TestTCQV0062CitationNeedsAStableBaseHeader(t *testing.T) {
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

// TestTCQV0062ChangedLinesFindTheEditedDeclaration pins that an added line is
// its own target line and a pure deletion marks the lines on either side.
func TestTCQV0062ChangedLinesFindTheEditedDeclaration(t *testing.T) {
	added := &patch.Hunk{NewRange: wire.Range{Start: 3, Count: 3}, Body: []patch.BodyLine{{Prefix: ' '}, {Prefix: '+'}, {Prefix: ' '}}}
	if got := changedLines(added); !reflect.DeepEqual(got, map[int64]bool{4: true}) {
		t.Fatalf("added = %v", got)
	}
	deleted := &patch.Hunk{NewRange: wire.Range{Start: 3, Count: 2}, Body: []patch.BodyLine{{Prefix: ' '}, {Prefix: '-'}, {Prefix: ' '}}}
	if got := changedLines(deleted); !reflect.DeepEqual(got, map[int64]bool{3: true, 4: true}) {
		t.Fatalf("deleted = %v", got)
	}
}

// TestTCQV0062SpentBudgetKeepsNoCandidateReason pins the reason of a row
// once the total bound is spent: a hunk with a candidate test is total-limit,
// but a hunk with none would run nothing, so it stays no-candidate-test.
func TestTCQV0062SpentBudgetKeepsNoCandidateReason(t *testing.T) {
	tree := writeGuardTree(t, map[string]string{"p/p.go": guardSource, "p/p_test.go": guardTests})
	name := "p/p.go"
	at := func(line int64) guardHunk {
		hunk := &patch.Hunk{NewPath: &name, NewRange: wire.Range{Start: line, Count: 1}, Body: []patch.BodyLine{{Prefix: '+'}}}
		return guardHunk{hunk: hunk, kind: patch.KindModify}
	}
	hunks := []guardHunk{at(2), at(12)}
	rows := make([]GuardRow, len(hunks))
	spent, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	run := func(context.Context, string, string, []string) runOutcome { ran = true; return runOutcome{} }
	judgeGuards(spent, tree, hunks, rows, DefaultHunkRevertLimits, run, func(string) []byte { return []byte(guardTests) })
	if ran || rows[0].Reason != GuardNoCandidate || rows[1].Reason != RevertTotalLimit {
		t.Fatalf("ran=%v rows=%+v", ran, rows)
	}
}
