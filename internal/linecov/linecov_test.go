package linecov

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// The fixture module exercises each selection path: a case expression outside
// every coverage block, an executed and an unexecuted body, a package-level
// initializer, a test function, a test helper, and an end-to-end test that
// builds and executes a binary (LPCV-V0-954).
var fixture = map[string]string{
	"go.mod": "module example.com/m\n\ngo 1.22\n",
	"calc/calc.go": `package calc

var Base = 10

func Classify(n int) string {
	switch {
	case n < 0:
		return "neg"
	case n == 0:
		return "zero"
	}
	return "pos"
}

func Double(n int) int {
	return n * 2
}

func Unused() int {
	return 1
}
`,
	"calc/calc_test.go": `package calc

import "testing"

func want(t *testing.T, got, exp string) {
	if got != exp {
		t.Fatalf("got %q want %q", got, exp)
	}
}

func TestNeg(t *testing.T)  { want(t, Classify(-1), "neg") }
func TestZero(t *testing.T) { want(t, Classify(0), "zero") }

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("double")
	}
}
`,
	"cmd/tool/main.go": `package main

import (
	"fmt"

	"example.com/m/calc"
)

func main() {
	fmt.Println(calc.Double(calc.Base))
}
`,
	"e2e/e2e_test.go": `package e2e

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTool(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tool")
	if out, err := exec.Command("go", "build", "-o", bin, "../cmd/tool").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	out, err := exec.Command(bin).Output()
	if err != nil || strings.TrimSpace(string(out)) != "20" {
		t.Fatalf("tool: %v %q", err, out)
	}
}
`,
	// ord passes alone and in order, but only in order does TestB reach the
	// second branch of Step: no isolated record holds it.
	"ord/ord.go": `package ord

var calls int

func Step() string {
	calls++
	if calls > 1 {
		return "again"
	}
	return "first"
}
`,
	"ord/ord_test.go": `package ord

import "testing"

func TestA(t *testing.T) { Step() }
func TestB(t *testing.T) { Step() }
`,
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for p, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// failingTests runs the fixture's package dirs and returns the failing
// top-level tests as dir.Name.
func failingTests(t *testing.T, dir string, dirs []string) map[string]bool {
	t.Helper()
	args := []string{"test", "-count=1", "-json"}
	for _, d := range dirs {
		args = append(args, "./"+d)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=")
	out, _ := cmd.Output()
	failed := map[string]bool{}
	for _, line := range bytes.Split(out, []byte("\n")) {
		var ev struct{ Action, Package, Test string }
		if json.Unmarshal(line, &ev) != nil || ev.Test == "" || strings.Contains(ev.Test, "/") {
			continue
		}
		if ev.Action == "fail" {
			failed[strings.TrimPrefix(strings.TrimPrefix(ev.Package, "example.com/m"), "/")+"."+ev.Test] = true
		}
	}
	return failed
}

func requireGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	if runtime.GOOS == "windows" {
		t.Skip("fixture shells out to POSIX paths")
	}
}

// TestFaultInjection is the LPCV-V0-963 qualification in miniature: collect
// the fixture at base, mutate one line per case, and require every test the
// mutant fails to be kept by Narrow. The unexecuted-body mutant is the
// negative control: it fails nothing and selects nothing.
func TestFaultInjection(t *testing.T) {
	requireGo(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeTree(t, dir, fixture)
	git(t, dir, "init", "-q")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "base")
	base := git(t, dir, "rev-parse", "HEAD")

	dirs := []string{"calc", "cmd/tool", "e2e", "ord"}
	outcomes, err := Collect(ctx, dir, CollectOptions{Module: "example.com/m", Packages: dirs, Work: t.TempDir(), Parallel: 4, Timeout: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var records []Record
	for _, o := range outcomes {
		if o.Record == nil {
			t.Fatalf("%s.%s diverged alone: %s", o.Package, o.Name, o.Divergent)
		}
		records = append(records, *o.Record)
	}
	oids, err := OIDs(ctx, dir, base)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Build("example.com/m", base, runtime.Version()+" atomic", oids, records)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ix.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if ix, err = Decode(&buf); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name, file, old, new string
		fallback             bool
		kept                 []string // exact kept tests, dir.Name, when not a fallback
		failsSome            bool
	}{
		{name: "case expression outside every block", file: "calc/calc.go", old: "case n == 0:", new: "case n == 1:", kept: []string{"calc.TestNeg", "calc.TestZero"}, failsSome: true},
		{name: "executed return", file: "calc/calc.go", old: `return "zero"`, new: `return "nil"`, kept: []string{"calc.TestZero"}, failsSome: true},
		{name: "body reached only through an executed binary", file: "calc/calc.go", old: "return n * 2", new: "return n * 3", kept: []string{"calc.TestDouble", "e2e.TestTool"}, failsSome: true},
		{name: "negative control: unexecuted body", file: "calc/calc.go", old: "return 1", new: "return 2", kept: nil},
		{name: "package-level initializer", file: "calc/calc.go", old: "var Base = 10", new: "var Base = 11", fallback: true, failsSome: true},
		{name: "test function", file: "calc/calc_test.go", old: `Classify(0), "zero"`, new: `Classify(0), "pos"`, kept: []string{"calc.TestZero"}, failsSome: true},
		{name: "path taken only in ordinary order", file: "ord/ord.go", old: `return "again"`, new: `panic("again")`, kept: []string{"ord.TestA", "ord.TestB"}, failsSome: true},
		{name: "statement inserted at function entry", file: "calc/calc.go", old: "func Double(n int) int {", new: "func Double(n int) int {\n\tpanic(\"entry\")", kept: []string{"calc.TestDouble", "e2e.TestTool"}, failsSome: true},
		{name: "negative control: insertion at unexecuted entry", file: "calc/calc.go", old: "func Unused() int {", new: "func Unused() int {\n\tpanic(\"entry\")", kept: nil},
		{name: "test helper", file: "calc/calc_test.go", old: "if got != exp {", new: "if got == exp {", kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero"}, failsSome: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			git(t, dir, "checkout", "-q", base)
			full := filepath.Join(dir, c.file)
			src, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Count(src, []byte(c.old)) != 1 {
				t.Fatalf("mutant site %q not unique", c.old)
			}
			if err := os.WriteFile(full, bytes.Replace(src, []byte(c.old), []byte(c.new), 1), 0o644); err != nil {
				t.Fatal(err)
			}
			git(t, dir, "commit", "-qam", c.name)
			head := git(t, dir, "rev-parse", "HEAD")

			diffs, err := Diff(ctx, dir, base, head)
			if err != nil {
				t.Fatal(err)
			}
			change := ix.Analyze(diffs, Reader(ctx, dir, base))
			heads, err := HeadTests(dir, dirs)
			if err != nil {
				t.Fatal(err)
			}
			plans, reason := ix.Narrow(dirs, heads, Effect{}, change)
			kept := map[string]bool{}
			for pkg, p := range plans {
				names := p.Tests
				if p.Mode == Whole {
					names = heads[pkg]
				}
				for _, n := range names {
					kept[pkg+"."+n] = true
				}
			}
			failed := failingTests(t, dir, dirs)
			for f := range failed {
				if !kept[f] {
					t.Errorf("MISS: %s fails on the mutant but was not kept (plans %+v)", f, plans)
				}
			}
			if c.failsSome != (len(failed) > 0) {
				t.Errorf("mutant failed %v; want failures=%v", failed, c.failsSome)
			}
			if (reason != "") != c.fallback {
				t.Fatalf("fallback %q, want fallback=%v", reason, c.fallback)
			}
			if c.fallback {
				return
			}
			var got []string
			for k := range kept {
				got = append(got, k)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, c.kept) {
				t.Errorf("kept %v, want %v", got, c.kept)
			}
		})
	}
}

func TestNarrowKeepsUnrecordedAndStale(t *testing.T) {
	ix := &Index{Tests: []Test{{Package: "p", Name: "TestA"}, {Package: "p", Name: "TestB"}, {Package: "p", Name: "TestC"}}}
	ix.reindex()
	heads := map[string][]string{"p": {"TestA", "TestB", "TestC", "TestNew"}, "q": {"TestQ"}}
	change := Effect{Tests: map[int32]bool{0: true}, Touched: map[string]bool{"r": true}}
	drift := Effect{Tests: map[int32]bool{1: true}}
	plans, reason := ix.Narrow([]string{"p", "q", "r"}, heads, drift, change)
	if reason != "" {
		t.Fatal(reason)
	}
	want := map[string]PackagePlan{
		"p": {Mode: Tests, Tests: []string{"TestA", "TestB", "TestNew"}, Why: map[string]string{"TestA": "selected", "TestB": "stale", "TestNew": "unrecorded"}},
		"q": {Mode: Whole, Tests: []string{"TestQ"}, Why: map[string]string{"TestQ": "unrecorded"}},
		"r": {Mode: Compile, Why: map[string]string{}},
	}
	if !reflect.DeepEqual(plans, want) {
		t.Fatalf("plans %+v\nwant %+v", plans, want)
	}
	if _, reason = ix.Narrow([]string{"p"}, heads, Effect{Fallback: "x"}, change); reason != "index drift: x" {
		t.Fatalf("drift fallback %q", reason)
	}
}

func TestAnalyzeFallbacks(t *testing.T) {
	ix := &Index{Files: []File{{Path: "a/a.go", OID: "old"}}}
	ix.reindex()
	read := func(string) ([]byte, error) { return []byte("package a\n\nfunc F() {\n\t_ = 1\n}\n"), nil }
	for name, d := range map[string]FileDiff{
		"non-Go":         {Path: "a/data.json", OldOID: "1", NewOID: "2"},
		"added file":     {Path: "a/b.go", NewOID: "2"},
		"uninstrumented": {Path: "b/b.go", OldOID: "1", NewOID: "2"},
		"other blob":     {Path: "a/a.go", OldOID: "1", NewOID: "2", Hunks: []Hunk{{4, 4}}},
		"signature":      {Path: "a/a.go", OldOID: "old", NewOID: "2", Hunks: []Hunk{{3, 3}}},
	} {
		if e := ix.Analyze([]FileDiff{d}, read); e.Fallback == "" {
			t.Errorf("%s: no fallback", name)
		}
	}
	if e := ix.Analyze([]FileDiff{{Path: "a/a.go", OldOID: "old", NewOID: "2", Hunks: []Hunk{{4, 4}}}}, read); e.Fallback != "" || len(e.Tests) != 0 || !e.Touched["a"] {
		t.Errorf("body edit: %+v", e)
	}
}

func TestParseDiff(t *testing.T) {
	out := ":100644 100644 " + strings.Repeat("a", 40) + " " + strings.Repeat("b", 40) + " M\tx/y.go\n" +
		":000000 100644 " + zeroOID + " " + strings.Repeat("c", 40) + " A\tz.go\n" +
		"\ndiff --git a/x/y.go b/x/y.go\nindex a..b 100644\n--- a/x/y.go\n+++ b/x/y.go\n@@ -3 +3 @@ f\n-a\n+b\n@@ -7,0 +8,2 @@\n+c\n+d\n" +
		"diff --git a/z.go b/z.go\nnew file mode 100644\n--- /dev/null\n+++ b/z.go\n@@ -0,0 +1 @@\n+package z\n"
	diffs, err := parseDiff([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := []FileDiff{
		{Path: "x/y.go", OldOID: strings.Repeat("a", 40), NewOID: strings.Repeat("b", 40), Hunks: []Hunk{{3, 3}, {7, 6}}},
		{Path: "z.go", NewOID: strings.Repeat("c", 40), Hunks: []Hunk{{0, -1}}},
	}
	if !reflect.DeepEqual(diffs, want) {
		t.Fatalf("got %+v\nwant %+v", diffs, want)
	}
}

// A package whose in-order suite run did not pass has unknown order-only
// blocks, so a change reaching any of its tests keeps it whole (LPCV-V0-962).
func TestUnorderedPackageRunsWhole(t *testing.T) {
	// q's suite run did not pass, so a block only its suite order reaches may
	// have no isolated owner: the change selects nothing of q, yet q runs whole.
	src := "package p\n\nfunc F() int {\n\tx := 1\n\treturn x\n}\n"
	recs := []Record{
		{Package: "p", Name: "TestF", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,5.10 2 1\n")},
		{Package: "q", Name: "TestQ", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,5.10 2 0\n")},
	}
	diff := []FileDiff{{Path: "p/p.go", OldOID: "x", NewOID: "y", Hunks: []Hunk{{4, 4}}}}
	read := func(string) ([]byte, error) { return []byte(src), nil }
	heads := map[string][]string{"p": {"TestF"}, "q": {"TestQ"}}
	for _, unordered := range []bool{false, true} {
		ix, err := Build("example.com/m", "c0", "go test", map[string]string{"p/p.go": "x"}, recs)
		if err != nil {
			t.Fatal(err)
		}
		if unordered {
			ix.Unordered = []string{"q"}
		}
		plans, reason := ix.Narrow([]string{"p", "q"}, heads, Effect{}, ix.Analyze(diff, read))
		want := Exclude
		if unordered {
			want = Whole
		}
		if reason != "" || plans["p"].Mode != Whole || plans["q"].Mode != want {
			t.Errorf("unordered=%v: plans %+v reason %q", unordered, plans, reason)
		}
	}
}

func TestSameLineCaseConditionKeepsFunction(t *testing.T) {
	// The case condition shares line 5 with its body's block but lies outside
	// it: editing it can change the result of every test reaching the switch.
	src := "package p\n\nfunc F(n int) string {\n\tswitch {\n\tcase n < 0: return \"neg\"\n\t}\n\treturn \"pos\"\n}\n"
	profile := func(neg bool) []byte {
		n := "0"
		if neg {
			n = "1"
		}
		return []byte("mode: atomic\nexample.com/m/p/p.go:4.2,4.9 1 1\nexample.com/m/p/p.go:5.14,5.26 1 " + n + "\nexample.com/m/p/p.go:7.2,7.14 1 1\n")
	}
	recs := []Record{{Package: "p", Name: "TestNeg", Profile: profile(true)}, {Package: "p", Name: "TestZero", Profile: profile(false)}}
	ix, err := Build("example.com/m", "c0", "go test", map[string]string{"p/p.go": "x"}, recs)
	if err != nil {
		t.Fatal(err)
	}
	read := func(string) ([]byte, error) { return []byte(src), nil }
	e := ix.Analyze([]FileDiff{{Path: "p/p.go", OldOID: "x", NewOID: "y", Hunks: []Hunk{{5, 5}}}}, read)
	if e.Fallback != "" || len(e.Tests) != 2 {
		t.Fatalf("case line: effect %+v, want both tests", e)
	}
	// A comment after the return is no code: line 7 keeps only its block's tests,
	// which both are; the distinct columns keep the two line-5 tests apart.
	if got := ix.Files[0].Blocks; len(got) != 3 || got[1].StartCol != 14 || len(got[1].Tests) != 1 {
		t.Fatalf("blocks %+v", got)
	}
}

func TestHelperNamedLikeATestIsAHelper(t *testing.T) {
	for name, want := range map[string]bool{"TestX": true, "Test_x": true, "Test": true, "Testhelper": false, "Examplefoo": false, "FuzzY": true, "TestMain": false} {
		fn := &ast.FuncDecl{Name: ast.NewIdent(name)}
		if _, ok := topLevelTest(fn); ok != want {
			t.Errorf("%s: top-level %v, want %v", name, ok, want)
		}
	}
}

// Merging indexes of disjoint package shards equals one build over every
// record; parts that disagree on identity, a blob, or a test refuse.
func TestMergeEqualsWholeBuild(t *testing.T) {
	recP := Record{Package: "p", Name: "TestP", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:3.2,3.9 1 1\nexample.com/m/p/p.go:4.2,4.9 1 0\n")}
	recQ := Record{Package: "q", Name: "TestQ", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,4.9 1 1\nexample.com/m/q/q.go:3.2,3.9 1 1\n")}
	oids := map[string]string{"p/p.go": "x", "q/q.go": "y"}
	build := func(recs ...Record) *Index {
		ix, err := Build("example.com/m", "c0", "go test", oids, recs)
		if err != nil {
			t.Fatal(err)
		}
		return ix
	}
	encode := func(ix *Index) string {
		var b bytes.Buffer
		if err := ix.Encode(&b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	p, q := build(recP), build(recQ)
	q.Unordered = []string{"q"}
	whole := build(recP, recQ)
	whole.Unordered = []string{"q"}
	merged, err := Merge(q, p)
	if err != nil {
		t.Fatal(err)
	}
	if encode(merged) != encode(whole) {
		t.Fatalf("merged %+v\nwhole %+v", merged, whole)
	}
	other := build(recQ)
	other.Runner = "go test -race"
	if _, err := Merge(p, other); err == nil {
		t.Error("runner mismatch merged")
	}
	if _, err := Merge(p, build(recP)); err == nil {
		t.Error("a test recorded twice merged")
	}
	moved := build(recQ)
	moved.Files[0].OID = "z"
	if _, err := Merge(p, moved); err == nil {
		t.Error("differing blobs merged")
	}
}
