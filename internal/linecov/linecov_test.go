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

func Touch() int {
	return 3
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
	"os"

	"example.com/m/calc"
)

func main() {
	if os.Getenv("LINECOV_FIXTURE_TOOL") == "" {
		calc.Touch()
	}
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
	// flag passes alone and in order, and TestB reaches F only after TestA
	// set On. Every block the in-order run executes an isolated record also
	// holds (TestA's reaches F), so only the retained in-order record keeps
	// TestB when F, or the state TestA leaves, changes (GLS-V0-008).
	"flag/flag.go": `package flag

var On bool

var Want = 1

func F() int {
	return 1
}
`,
	"flag/flag_test.go": `package flag

import "testing"

func TestA(t *testing.T) {
	On = true
	F()
}

func TestB(t *testing.T) {
	if On && F() != Want {
		t.Fatal("F")
	}
}
`,
	// env reads the inherited environment: TestSkip reaches Guarded only
	// while LINECOV_FIXTURE_SKIP is unset, and TestOwn reads a variable it
	// set itself, so only the first is bound to the environment. envall's
	// TestAll reads the whole environment, which under collection holds the
	// collector's own variables, so it binds EnvModified and envall always
	// runs whole (GLS-V0-017, GLS-V0-018).
	"env/env.go": `package env

func Guarded() int {
	return 1
}

func Own() int {
	return 1
}
`,
	"env/env_test.go": `package env

import (
	"os"
	"testing"
)

func TestSkip(t *testing.T) {
	if os.Getenv("LINECOV_FIXTURE_SKIP") != "" {
		return
	}
	if Guarded() != 1 {
		t.Fatal("Guarded")
	}
}

func TestOwn(t *testing.T) {
	os.Setenv("LINECOV_FIXTURE_OWN", "1")
	if os.Getenv("LINECOV_FIXTURE_OWN") == "1" && Own() != 1 {
		t.Fatal("Own")
	}
}
`,
	"envall/envall.go": `package envall

func Everything() int {
	return 1
}
`,
	"envall/envall_test.go": `package envall

import (
	"os"
	"testing"
)

func TestAll(t *testing.T) {
	for _, kv := range os.Environ() {
		if kv == "LINECOV_FIXTURE_ALL=1" {
			return
		}
	}
	if Everything() != 1 {
		t.Fatal("Everything")
	}
}
`,
	// cov reads the variables the collector sets for every test process:
	// TestDir reaches Hidden only without GOCOVERDIR, TestFlags reaches
	// Flagged only without a coverage flag in GOFLAGS. Collection never
	// reaches either, an ordinary run reaches both, so their records bind
	// EnvModified and cov always runs whole; the coverage runtime's and the
	// exec audit's own reads of the same names bind no other test
	// (GLS-V0-017, GLS-V0-018).
	"cov/cov.go": `package cov

func Hidden() int {
	return 1
}

func Flagged() int {
	return 1
}
`,
	"cov/cov_test.go": `package cov

import (
	"os"
	"strings"
	"testing"
)

func TestDir(t *testing.T) {
	if os.Getenv("GOCOVERDIR") != "" {
		return
	}
	if Hidden() != 1 {
		t.Fatal("Hidden")
	}
}

func TestFlags(t *testing.T) {
	if v, _ := os.LookupEnv("GOFLAGS"); strings.Contains(v, "-cover") {
		return
	}
	if Flagged() != 1 {
		t.Fatal("Flagged")
	}
}
`,
	// Each test reaches its function only under a setting go test -json
	// passes every test binary: verbose mode, and a deadline from the go test
	// -timeout. Collection that ran the binary without them would record
	// neither function (GLS-V0-020).
	"rt/rt.go": `package rt

func Loud() int {
	return 1
}

func Timed() int {
	return 1
}
`,
	"rt/rt_test.go": `package rt

import "testing"

func TestLoud(t *testing.T) {
	if testing.Verbose() && Loud() != 1 {
		t.Fatal("Loud")
	}
}

func TestTimed(t *testing.T) {
	if _, ok := t.Deadline(); ok && Timed() != 1 {
		t.Fatal("Timed")
	}
}
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

	dirs := []string{"calc", "cmd/tool", "e2e", "ord", "flag", "rt", "env", "envall", "cov"}
	// Collection runs under an environment in which TestSkip skips Guarded,
	// TestAll skips Everything and the tool skips Touch; the environment
	// cases below select and run with a variable unset again, where those
	// paths run and a mutant in them fails (GLS-V0-018).
	for _, name := range []string{"LINECOV_FIXTURE_SKIP", "LINECOV_FIXTURE_ALL", "LINECOV_FIXTURE_TOOL"} {
		t.Setenv(name, "1")
	}
	if _, set := os.LookupEnv("LINECOV_FIXTURE_OWN"); set {
		t.Fatal("LINECOV_FIXTURE_OWN is set in the test environment")
	}
	outcomes, err := Collect(ctx, dir, CollectOptions{Module: "example.com/m", Packages: dirs, Work: t.TempDir(), Parallel: 4, Timeout: 5 * time.Minute, TestTimeout: DefaultTestTimeout})
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

	// The environment bindings of the fixture: TestSkip read
	// LINECOV_FIXTURE_SKIP, TestAll the whole environment, TestOwn only its
	// own setting, e2e.TestTool's child LINECOV_FIXTURE_TOOL, and cov's
	// tests the collector's GOCOVERDIR and GOFLAGS.
	if !ix.EnvBound {
		t.Fatal("the collected index binds no environment")
	}
	bindings := func(pkg, name string) string {
		id, ok := ix.byName[pkg+"\x00"+name]
		if !ok {
			t.Fatalf("%s.%s has no record", pkg, name)
		}
		var out []string
		for _, b := range ix.envBindings(id) {
			out = append(out, b.Name+"="+b.Value)
		}
		return strings.Join(out, " ")
	}
	if got := bindings("env", "TestSkip"); !strings.Contains(got, "LINECOV_FIXTURE_SKIP="+EnvHash("1")) || strings.Contains(got, "LINECOV_FIXTURE_OWN") {
		t.Errorf("TestSkip bindings %q", got)
	}
	if got := bindings("envall", "TestAll"); !strings.Contains(got, EnvAll+"="+EnvModified) {
		t.Errorf("TestAll bindings %q", got)
	}
	if got := bindings("cov", "TestDir"); !strings.Contains(got, "GOCOVERDIR="+EnvModified) || strings.Contains(got, "GOFLAGS") {
		t.Errorf("TestDir bindings %q", got)
	}
	if got := bindings("cov", "TestFlags"); !strings.Contains(got, "GOFLAGS="+EnvModified) || strings.Contains(got, "GOCOVERDIR") {
		t.Errorf("TestFlags bindings %q", got)
	}
	if got := bindings("cov", InOrder); !strings.Contains(got, "GOCOVERDIR="+EnvModified) || !strings.Contains(got, "GOFLAGS="+EnvModified) {
		t.Errorf("cov in-order bindings %q", got)
	}
	// The coverage runtime (at every instrumented exit, the e2e child's
	// included), the exec audit and the counter windows read the same names;
	// none of those reads is a test's.
	for _, r := range records {
		if r.Package == "cov" || r.Package == "envall" {
			continue
		}
		for _, b := range r.Env {
			if EnvCollectorModified(b.Name) || b.Name == EnvAll {
				t.Errorf("%s.%s binds %s: the collector's own read", r.Package, r.Name, b.Name)
			}
		}
	}
	if got := bindings("env", "TestOwn"); strings.Contains(got, "LINECOV_FIXTURE_OWN") {
		t.Errorf("TestOwn bindings %q: its own setting is not inherited", got)
	}
	if got := bindings("e2e", "TestTool"); !strings.Contains(got, "LINECOV_FIXTURE_TOOL="+EnvHash("1")) {
		t.Errorf("TestTool bindings %q: the child's read is the test's", got)
	}
	if got := bindings("env", InOrder); !strings.Contains(got, "LINECOV_FIXTURE_SKIP="+EnvHash("1")) || strings.Contains(got, EnvAll) {
		t.Errorf("in-order bindings %q", got)
	}

	cases := []struct {
		name, file, old, new string
		fallback             bool
		kept                 []string // exact kept tests, dir.Name, when not a fallback
		failsSome            bool
		// environ changes the environment of the selection and the run:
		// NAME=value sets a variable, NAME unsets one; the rest of this
		// process's environment is inherited.
		environ []string
	}{
		// Under the collection environment the guarded paths were not
		// reached, so a mutant in one fails nothing and selects nothing.
		{name: "environment: negative control, guarded body under the collection environment", file: "env/env.go", old: "func Guarded() int {", new: "func Guarded() int {\n\tpanic(\"guarded\")", kept: nil},
		{name: "environment: whole-environment reader's body under the collection environment, always kept", file: "envall/envall.go", old: "func Everything() int {", new: "func Everything() int {\n\tpanic(\"everything\")", kept: nil},
		{name: "environment: negative control, body a child reaches only under another environment", file: "calc/calc.go", old: "func Touch() int {", new: "func Touch() int {\n\tpanic(\"touch\")", kept: nil},
		// With the variable unset the test reaches the mutant and fails: the
		// package's in-order record, bound to the variable, is unbound, so
		// the package runs whole (GLS-V0-018). Excluding it would be the
		// miss the inherited environment caused before 1.0.
		{name: "environment: guarded body with the variable the test reads unset", file: "env/env.go", old: "func Guarded() int {", new: "func Guarded() int {\n\tpanic(\"guarded\")", kept: []string{"env.TestOwn", "env.TestSkip"}, environ: []string{"LINECOV_FIXTURE_SKIP"}, failsSome: true},
		{name: "environment: whole-environment reader's body under another environment", file: "envall/envall.go", old: "func Everything() int {", new: "func Everything() int {\n\tpanic(\"everything\")", kept: nil, environ: []string{"LINECOV_FIXTURE_ALL"}, failsSome: true},
		{name: "environment: body a child reaches with its variable unset", file: "calc/calc.go", old: "func Touch() int {", new: "func Touch() int {\n\tpanic(\"touch\")", kept: []string{"e2e.TestTool"}, environ: []string{"LINECOV_FIXTURE_TOOL"}, failsSome: true},
		// A test's own setting binds nothing: the in-order run reached Own,
		// so the package runs whole for order, under any environment.
		{name: "environment: own-setting body", file: "env/env.go", old: "func Own() int {", new: "func Own() int {\n\tpanic(\"own\")", kept: []string{"env.TestOwn", "env.TestSkip"}, failsSome: true},
		{name: "environment: own-setting body under a changed value", file: "env/env.go", old: "func Own() int {", new: "func Own() int {\n\tpanic(\"own\")", kept: []string{"env.TestOwn", "env.TestSkip"}, environ: []string{"LINECOV_FIXTURE_OWN=2"}, failsSome: true},
		// A body reached only without the collector's GOCOVERDIR, or without
		// its coverage flags in GOFLAGS: collection never reached it and an
		// ordinary run fails on it. cov runs whole (asserted for every case
		// below); before GLS-V0-017 dropped those reads, it was excluded.
		{name: "collector-set variable: body reached only without GOCOVERDIR", file: "cov/cov.go", old: "func Hidden() int {", new: "func Hidden() int {\n\tpanic(\"hidden\")", kept: nil, failsSome: true},
		{name: "collector-set variable: body reached only without coverage flags in GOFLAGS", file: "cov/cov.go", old: "func Flagged() int {", new: "func Flagged() int {\n\tpanic(\"flagged\")", kept: nil, failsSome: true},
		{name: "case expression outside every block", file: "calc/calc.go", old: "case n == 0:", new: "case n == 1:", kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero"}, failsSome: true},
		{name: "executed return", file: "calc/calc.go", old: `return "zero"`, new: `return "nil"`, kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero"}, failsSome: true},
		{name: "body reached only through an executed binary", file: "calc/calc.go", old: "return n * 2", new: "return n * 3", kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero", "e2e.TestTool"}, failsSome: true},
		{name: "negative control: unexecuted body", file: "calc/calc.go", old: "return 1", new: "return 2", kept: nil},
		{name: "package-level initializer", file: "calc/calc.go", old: "var Base = 10", new: "var Base = 11", fallback: true, failsSome: true},
		{name: "test function", file: "calc/calc_test.go", old: `Classify(0), "zero"`, new: `Classify(0), "pos"`, kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero"}, failsSome: true},
		{name: "path taken only in ordinary order", file: "ord/ord.go", old: `return "again"`, new: `panic("again")`, kept: []string{"ord.TestA", "ord.TestB"}, failsSome: true},
		{name: "block an isolated record shares, reached by a later test only in order", file: "flag/flag.go", old: "return 1", new: "return 2", kept: []string{"flag.TestA", "flag.TestB"}, failsSome: true},
		{name: "test function leaving state a later test reads", file: "flag/flag_test.go", old: "On = true", new: "On, Want = true, 2", kept: []string{"flag.TestA", "flag.TestB"}, failsSome: true},
		{name: "statement inserted at function entry", file: "calc/calc.go", old: "func Double(n int) int {", new: "func Double(n int) int {\n\tpanic(\"entry\")", kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero", "e2e.TestTool"}, failsSome: true},
		{name: "negative control: insertion at unexecuted entry", file: "calc/calc.go", old: "func Unused() int {", new: "func Unused() int {\n\tpanic(\"entry\")", kept: nil},
		{name: "test helper", file: "calc/calc_test.go", old: "if got != exp {", new: "if got == exp {", kept: []string{"calc.TestDouble", "calc.TestNeg", "calc.TestZero"}, failsSome: true},
		{name: "body reached only in verbose mode", file: "rt/rt.go", old: "func Loud() int {", new: "func Loud() int {\n\tpanic(\"loud\")", kept: []string{"rt.TestLoud", "rt.TestTimed"}, failsSome: true},
		{name: "body reached only under a test deadline", file: "rt/rt.go", old: "func Timed() int {", new: "func Timed() int {\n\tpanic(\"timed\")", kept: []string{"rt.TestLoud", "rt.TestTimed"}, failsSome: true},
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
			for _, kv := range c.environ {
				name, value, set := strings.Cut(kv, "=")
				if set {
					t.Setenv(name, value)
					continue
				}
				previous, had := os.LookupEnv(name)
				os.Unsetenv(name)
				t.Cleanup(func() {
					if had {
						os.Setenv(name, previous)
					}
				})
			}
			plans, reason := ix.Narrow(os.Environ(), dirs, heads, Effect{}, change)
			// envall and cov read what the collector set, so they run whole
			// under every change; their tests are left out of the exact list.
			alwaysWhole := map[string]bool{"envall": true, "cov": true}
			for pkg := range alwaysWhole {
				if plans[pkg].Mode != Whole {
					t.Errorf("%s not kept whole: %+v", pkg, plans[pkg])
				}
			}
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
				if pkg, _, _ := strings.Cut(k, "."); !alwaysWhole[pkg] {
					got = append(got, k)
				}
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, c.kept) {
				t.Errorf("kept %v, want %v (plans %+v)", got, c.kept, plans)
			}
		})
	}
}

func TestNarrowKeepsUnrecordedAndStale(t *testing.T) {
	// p's in-order record is in neither effect: a change it did not reach
	// leaves the per-test rules to decide (GLS-V0-008).
	ix := &Index{Tests: []Test{{Package: "p", Name: InOrder}, {Package: "p", Name: "TestA"}, {Package: "p", Name: "TestB"}, {Package: "p", Name: "TestC"}}}
	ix.reindex()
	heads := map[string][]string{"p": {"TestA", "TestB", "TestC", "TestNew"}, "q": {"TestQ"}}
	change := Effect{Tests: map[int32]bool{1: true}, Touched: map[string]bool{"r": true}}
	drift := Effect{Tests: map[int32]bool{2: true}}
	plans, reason := ix.Narrow(nil, []string{"p", "q", "r"}, heads, drift, change)
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
	if _, reason = ix.Narrow(nil, []string{"p"}, heads, Effect{Fallback: "x"}, change); reason != "index drift: x" {
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
	if e := ix.Analyze([]FileDiff{{Path: "a/a.go", OldOID: "old", NewOID: "2", NewSource: []byte("package a\n\nfunc F() {\n\t_ = 2\n}\n"), NewHunks: []Hunk{{4, 4}}, Hunks: []Hunk{{4, 4}}}}, read); e.Fallback != "" || len(e.Tests) != 0 || !e.Touched["a"] {
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
		{Path: "x/y.go", OldOID: strings.Repeat("a", 40), NewOID: strings.Repeat("b", 40), Hunks: []Hunk{{3, 3}, {7, 6}}, NewHunks: []Hunk{{3, 3}, {8, 9}}},
		{Path: "z.go", NewOID: strings.Repeat("c", 40), Hunks: []Hunk{{0, -1}}, NewHunks: []Hunk{{1, 1}}},
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
		{Package: "p", Name: InOrder, Profile: []byte("mode: atomic\n")},
		{Package: "q", Name: "TestQ", Profile: []byte("mode: atomic\nexample.com/m/p/p.go:4.2,5.10 2 0\n")},
		{Package: "q", Name: InOrder, Profile: []byte("mode: atomic\n")},
	}
	diff := []FileDiff{{Path: "p/p.go", OldOID: "x", NewOID: "y", NewSource: []byte(src), NewHunks: []Hunk{{4, 4}}, Hunks: []Hunk{{4, 4}}}}
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
		plans, reason := ix.Narrow(nil, []string{"p", "q"}, heads, Effect{}, ix.Analyze(diff, read))
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
	e := ix.Analyze([]FileDiff{{Path: "p/p.go", OldOID: "x", NewOID: "y", NewSource: []byte(src), NewHunks: []Hunk{{5, 5}}, Hunks: []Hunk{{5, 5}}}}, read)
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
