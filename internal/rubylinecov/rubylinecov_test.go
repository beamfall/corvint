// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// rspecImage is the owner-approved disposable qualification image the
// rubytestprovider live tests also use (Ruby 3.4, rspec-core 3.13).
const rspecImage = "corvint-rspec-qual:2026-10-20"

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// project copies testdata/<fixture> into dir as a one-commit repository and
// returns its HEAD.
func project(t *testing.T, dir, fixture string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("live Ruby fixtures need a POSIX host")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	src := filepath.Join("testdata", fixture)
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(rel)), 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "config", "user.email", "test@example.invalid")
	git(t, dir, "config", "user.name", "test")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "fixture")
	return git(t, dir, "rev-parse", "HEAD")
}

// requireRuby skips unless ruby on PATH can activate the gem.
func requireRuby(t *testing.T, gem string) {
	t.Helper()
	ruby, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("live collection needs ruby on PATH")
	}
	if out, err := exec.Command(ruby, "-e", "gem '"+gem+"'").CombinedOutput(); err != nil {
		t.Skipf("live collection needs the %s gem: %s", gem, out)
	}
}

func collect(t *testing.T, root, head string, o Options) (*Run, *linecov.Index) {
	t.Helper()
	o.Root = root
	if o.Work == "" {
		o.Work = t.TempDir()
	}
	o.Timeout = 2 * time.Minute
	run, err := Collect(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	oids, err := linecov.OIDs(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Index(".", head, run, oids)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ix.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if ix, err = linecov.Decode(&buf); err != nil {
		t.Fatal(err)
	}
	return run, ix
}

// edit returns the diff changing (or, with insert, inserting after) line of p.
func edit(ix *linecov.Index, p string, line int, insert bool) []linecov.FileDiff {
	oid := ""
	for _, f := range ix.Files {
		if f.Path == p {
			oid = f.OID
		}
	}
	h := linecov.Hunk{Start: line, End: line}
	if insert {
		h.End = line - 1
	}
	return []linecov.FileDiff{{Path: p, OldOID: oid, NewOID: "0123", Hunks: []linecov.Hunk{h}}}
}

// selected names the tests an effect selects, or the fallback.
func selected(ix *linecov.Index, e linecov.Effect) string {
	if e.Fallback != "" {
		return "fallback: " + e.Fallback
	}
	var names []string
	for id := range e.Tests {
		names = append(names, ix.Tests[id].Name)
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

// checkCalc checks the mapping of lib/calc.rb every fixture shares: an edit
// selects every test of the method it is in, so lines 11 and 13 (the two diff
// branches) and 12 (else) select both diff tests, line 6 the add test, and
// line 18 (unused) none.
func checkCalc(t *testing.T, ix *linecov.Index, add, down, up string) {
	t.Helper()
	for _, c := range []struct {
		line   int
		insert bool
		want   string
	}{
		{6, false, add},
		{11, false, sorted(down, up)},
		{13, false, sorted(down, up)},
		{12, false, sorted(down, up)},
		{11, true, sorted(down, up)},
		{18, false, "fallback: edit outside a method body lib/calc.rb:18"}, // uncalled method may contain a syntax edit
		{5, false, "fallback: edit outside a method body lib/calc.rb:5"},   // the def line
		{7, false, "fallback: edit outside a method body lib/calc.rb:7"},   // the end line
		{3, false, "fallback: edit outside a method body lib/calc.rb:3"},   // class-level comment
		{8, true, "fallback: edit outside a method body lib/calc.rb:8"},    // between methods
	} {
		if got := selected(ix, Analyze(ix, edit(ix, "lib/calc.rb", c.line, c.insert))); got != c.want {
			t.Errorf("lib/calc.rb:%d insert=%v selects %q, want %q", c.line, c.insert, got, c.want)
		}
	}
}

// lineTests names the tests the line block of path:line records.
func lineTests(ix *linecov.Index, path string, line int) string {
	var names []string
	for _, f := range ix.Files {
		for _, b := range f.Blocks {
			if f.Path == path && b.StartCol == 0 && int(b.Start) == line {
				for _, id := range b.Tests {
					names = append(names, ix.Tests[id].Name)
				}
			}
		}
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}

func sorted(names ...string) string {
	var kept []string
	for _, n := range names {
		if n != "" {
			kept = append(kept, n)
		}
	}
	sort.Strings(kept)
	return strings.Join(kept, " ")
}

// LPCV-V0-950, LPCV-V0-952, LPCV-V0-955: one Minitest process records each
// test's own lines, joins a subprocess's coverage to the test that started
// it, keeps collecting after an exec that fails, and gives a failing test no
// record. Work is relative to a directory other than Root.
func TestLPCVV0955LiveMinitestPerTest(t *testing.T) {
	requireRuby(t, "minitest")
	root := t.TempDir()
	head := project(t, root, "minitest")
	t.Chdir(t.TempDir())
	run, ix := collect(t, root, head, Options{Work: "work", Argv: []string{"ruby", "-Itest", "test/calc_test.rb"}})
	if run.ExitCode() == 0 {
		t.Fatal("the failing test should fail the suite")
	}
	if !strings.Contains(ix.Runner, " minitest ") || !strings.HasSuffix(ix.Runner, "coverage=lines,methods") {
		t.Fatalf("runner %q", ix.Runner)
	}
	recorded := map[string]bool{}
	for _, o := range run.Outcomes {
		recorded[o.Name] = o.Recorded
	}
	if len(recorded) != 6 || recorded["CalcTest#test_fails"] || !recorded["CalcTest#test_add"] || !recorded["CalcTest#test_tool"] ||
		!recorded["CalcTest#test_failed_exec"] {
		t.Fatalf("outcomes %+v", run.Outcomes)
	}
	if strings.Join(ix.Unordered, ",") != "test/calc_test.rb" {
		t.Fatalf("unordered %v: a file with a failing test runs whole", ix.Unordered)
	}
	checkCalc(t, ix, "CalcTest#test_add", "CalcTest#test_diff_down", "CalcTest#test_diff_up")
	if got := selected(ix, Analyze(ix, edit(ix, "lib/tool.rb", 6, false))); got != "CalcTest#test_tool" {
		t.Fatalf("a subprocess-only line selects %q", got)
	}
	// A real diff through linecov.Diff maps the same way.
	b, _ := os.ReadFile(filepath.Join(root, "lib", "calc.rb"))
	if err := os.WriteFile(filepath.Join(root, "lib", "calc.rb"), bytes.Replace(b, []byte("a + b"), []byte("b + a"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "commit", "-q", "-am", "swap")
	diffs, err := linecov.Diff(context.Background(), root, head, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if got := selected(ix, Analyze(ix, diffs)); got != "CalcTest#test_add" {
		t.Fatalf("the committed edit selects %q", got)
	}
}

// LPCV-V0-955: test-unit tests of one process each get their own lines.
func TestLPCVV0955LiveTestUnitPerTest(t *testing.T) {
	requireRuby(t, "test-unit")
	root := t.TempDir()
	head := project(t, root, "testunit")
	run, ix := collect(t, root, head, Options{Argv: []string{"ruby", "-Itest", "-e", "require './test/test_calc'"}})
	if run.ExitCode() != 0 || len(ix.Unordered) != 0 {
		t.Fatalf("exit %d unordered %v", run.ExitCode(), ix.Unordered)
	}
	if !strings.Contains(ix.Runner, " test-unit ") {
		t.Fatalf("runner %q", ix.Runner)
	}
	checkCalc(t, ix, "TestCalc#test_add", "TestCalc#test_diff_down", "")
}

// LPCV-V0-955, LPCV-V0-951: real RSpec through a docker prefix. Each example
// gets its own lines, and Plan runs every recorded spec file whole until the
// isolated pass (V1-4415) supports narrowing.
func TestLPCVV0955LiveRSpecThroughDocker(t *testing.T) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Skip("live RSpec collection needs docker on PATH")
	}
	if err := exec.Command(docker, "image", "inspect", rspecImage).Run(); err != nil {
		t.Skipf("live RSpec collection needs the %s image", rspecImage)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("home directory unavailable")
	}
	// Desktop docker VMs share the home directory, not the system temporary
	// directory.
	if err := os.MkdirAll(filepath.Join(home, ".cache"), 0o700); err != nil {
		t.Skipf("home cache unavailable: %v", err)
	}
	root, err := os.MkdirTemp(filepath.Join(home, ".cache"), "corvint-rubylinecov-")
	if err != nil {
		t.Skipf("home cache unavailable: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if root, err = filepath.EvalSymlinks(root); err != nil {
		t.Fatal(err)
	}
	head := project(t, root, "rspec")
	o := Options{
		Work: filepath.Join(root, ".corvint-linecov"), // untracked, inside the mount
		Argv: []string{"rspec"},
		Prefix: func(env []string) []string {
			argv := []string{docker, "run", "--rm", "--network", "none", "-v", root + ":/work", "-w", "/work"}
			for _, kv := range env {
				argv = append(argv, "-e", kv)
			}
			return append(argv, rspecImage)
		},
		PathIn: func(host string) string { return "/work" + strings.TrimPrefix(host, root) },
	}
	run, ix := collect(t, root, head, o)
	if !strings.HasPrefix(ix.Runner, "ruby 3.") || !strings.Contains(ix.Runner, " rspec ") {
		t.Fatalf("runner %q", ix.Runner)
	}
	if len(run.Outcomes) != 5 || strings.Join(ix.Unordered, ",") != "spec/calc_spec.rb" {
		t.Fatalf("outcomes %+v unordered %v", run.Outcomes, ix.Unordered)
	}
	checkCalc(t, ix, sorted("./spec/calc_spec.rb[1:1]", "./spec/other_spec.rb[1:1]"),
		"./spec/calc_spec.rb[1:2]", "./spec/calc_spec.rb[1:3]")
	static := []string{"spec/calc_spec.rb", "spec/other_spec.rb", "spec/new_spec.rb"}
	plans, reason := Plan(ix, static, linecov.Effect{}, Analyze(ix, edit(ix, "lib/calc.rb", 11, false)))
	if reason != "" || plans["spec/other_spec.rb"].Mode != linecov.Exclude ||
		plans["spec/calc_spec.rb"].Mode != linecov.Whole || plans["spec/new_spec.rb"].Mode != linecov.Whole {
		t.Fatalf("plans %+v reason %q", plans, reason)
	}
}

// writeDump writes one synthetic collector dump.
func writeDump(t *testing.T, dir, name string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	dumpStart = `{"kind":"start","ruby":"3.4.1","test":"load"}`
	dumpFiles = `{"kind":"files","files":{"lib/a.rb":{"lines":[1,2,3,6,7],"methods":[[2,2,4,5],[5,2,8,5]]},"vendor/x.rb":{"lines":[1],"methods":[]}}}`
	dumpDone  = `{"kind":"done"}`
)

func suiteDump(t *testing.T, dir string, extra ...string) {
	t.Helper()
	lines := append([]string{dumpStart, `{"kind":"load","lines":{"lib/a.rb":[1,2,5]},"methods":{}}`,
		`{"kind":"test","seq":1,"package":"spec/a_spec.rb","name":"one","status":"passed","lines":{"lib/a.rb":[3]},"methods":{"lib/a.rb":[[2,2,4,5]]}}`,
		`{"kind":"test","seq":2,"package":"spec/a_spec.rb","name":"two","status":"passed","lines":{},"methods":{}}`,
		`{"kind":"test","seq":3,"package":"spec/b_spec.rb","name":"three","status":"passed","loaded":true,"lines":{"lib/a.rb":[7]},"methods":{"lib/a.rb":[[5,2,8,5]]}}`,
		`{"kind":"framework","name":"rspec","version":"3.13.6"}`, dumpFiles}, extra...)
	writeDump(t, dir, "suite.jsonl", append(lines, dumpDone)...)
}

// LPCV-V0-954, LPCV-V0-962: a subprocess's dump joins its test by sequence; a
// test whose subprocess dump is incomplete has no record and runs its file
// whole; files outside Git are left out; a suite without its completion
// marker publishes nothing.
func TestLPCVV0954DumpJoinAndIncompleteness(t *testing.T) {
	dir := t.TempDir()
	suiteDump(t, dir)
	writeDump(t, dir, "child-1-1.jsonl", `{"kind":"start","ruby":"3.4.1","test":"2"}`,
		`{"kind":"child","test":"2","lines":{"lib/a.rb":[6]},"methods":{"lib/a.rb":[[5,2,8,5]]}}`, dumpFiles, dumpDone)
	run, err := read(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Index("m", "c0", run, map[string]string{"lib/a.rb": "oid-a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Files) != 1 || ix.Files[0].Path != "lib/a.rb" || len(ix.Tests) != 4 {
		t.Fatalf("files %+v tests %+v", ix.Files, ix.Tests)
	}
	// Line 6 ran only in test two's subprocess.
	if got := lineTests(ix, "lib/a.rb", 6); got != "two" {
		t.Fatalf("subprocess line records %q", got)
	}
	if got := selected(ix, Analyze(ix, edit(ix, "lib/a.rb", 3, false))); got != "one" {
		t.Fatalf("line 3 selects %q", got)
	}
	// Test three required a file, so its window joins the load bucket, and
	// lines 6 and 7 share its method.
	for _, line := range []int{6, 7} {
		if got := selected(ix, Analyze(ix, edit(ix, "lib/a.rb", line, false))); got != "fallback: the change reaches load-time code" {
			t.Fatalf("line %d selects %q", line, got)
		}
	}

	dir = t.TempDir()
	suiteDump(t, dir)
	writeDump(t, dir, "child-1-1.jsonl", `{"kind":"start","ruby":"3.4.1","test":"2"}`, `{"kind":"child","test":"2","lines":{},"methods":{}}`)
	if run, err = read(dir); err != nil {
		t.Fatal(err)
	}
	for _, o := range run.Outcomes {
		if o.Name == "two" && (o.Recorded || o.Reason != "a subprocess wrote no completion marker") {
			t.Fatalf("outcome %+v", o)
		}
	}
	if ix, err = Index("m", "c0", run, map[string]string{"lib/a.rb": "oid-a"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(ix.Unordered, ",") != "spec/a_spec.rb,spec/b_spec.rb" {
		t.Fatalf("unordered %v", ix.Unordered)
	}
	plans, _ := Plan(ix, []string{"spec/a_spec.rb", "spec/b_spec.rb"}, linecov.Effect{}, Analyze(ix, edit(ix, "lib/a.rb", 3, false)))
	if plans["spec/a_spec.rb"].Mode != linecov.Whole || plans["spec/b_spec.rb"].Mode != linecov.Whole {
		t.Fatalf("plans %+v", plans)
	}

	dir = t.TempDir()
	writeDump(t, dir, "suite.jsonl", dumpStart, `{"kind":"load","lines":{},"methods":{}}`)
	if _, err := read(dir); err == nil || !strings.Contains(err.Error(), "completion marker") {
		t.Fatalf("an unfinished suite reads: %v", err)
	}
	dir = t.TempDir()
	suiteDump(t, dir)
	writeDump(t, dir, "child-1-1.jsonl", `{"kind":"start","ruby":"3.4.1","test":"load"}`)
	if _, err := read(dir); err == nil {
		t.Fatal("incomplete coverage outside every test reads")
	}
}

// LPCV-V0-953: changes the index cannot map by line keep the static plan.
func TestLPCVV0953RubyFallbacks(t *testing.T) {
	dir := t.TempDir()
	suiteDump(t, dir)
	run, err := read(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Index("m", "c0", run, map[string]string{"lib/a.rb": "oid-a", "lib/b.rb": "oid-b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		diff linecov.FileDiff
		want string
	}{
		{linecov.FileDiff{Path: "Gemfile", OldOID: "1", NewOID: "2"}, "non-Ruby input Gemfile"},
		{linecov.FileDiff{Path: "spec/spec_helper.rb", OldOID: "1", NewOID: "2"}, "test helper spec/spec_helper.rb"},
		{linecov.FileDiff{Path: "lib/new.rb", NewOID: "2"}, "added or deleted file lib/new.rb"},
		{linecov.FileDiff{Path: "lib/b.rb", OldOID: "oid-b", NewOID: "2"}, "uninstrumented file lib/b.rb"},
		{linecov.FileDiff{Path: "lib/a.rb", OldOID: "other", NewOID: "2"}, "index records lib/a.rb at another blob"},
	} {
		if e := Analyze(ix, []linecov.FileDiff{c.diff}); e.Fallback != c.want {
			t.Errorf("%s: fallback %q, want %q", c.diff.Path, e.Fallback, c.want)
		}
	}
	e := Analyze(ix, []linecov.FileDiff{{Path: "spec/b_spec.rb", OldOID: "1", NewOID: "2"}, {Path: "test/test_new.rb", NewOID: "2"}})
	if e.Fallback != "" || !e.Whole["spec/b_spec.rb"] || !e.Whole["test/test_new.rb"] {
		t.Fatalf("test files: %+v", e)
	}
}
