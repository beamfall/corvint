package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// qualifyFixture has a site of every kind: a tagless case, an else-if, a
// loop header, a closure statement and plain statements, and an unexecuted
// body for the negative control.
var qualifyFixture = map[string]string{
	"go.mod": "module example.com/m\n\ngo 1.22\n",
	"calc/calc.go": `package calc

func Classify(n int) string {
	switch {
	case n < 0:
		return "neg"
	case n == 0:
		return "zero"
	}
	if n > 100 {
		return "big"
	} else if n > 10 {
		return "mid"
	}
	return "pos"
}

func Sum(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}

func Apply(xs []int) []int {
	out := []int{}
	each(xs, func(x int) {
		out = append(out, x*2)
	})
	return out
}

func each(xs []int, f func(int)) {
	for _, x := range xs {
		f(x)
	}
}

func Double(n int) int {
	return n * 2
}

func Unused(n int) int {
	n++
	return n
}
`,
	"calc/calc_test.go": `package calc

import "testing"

func TestClassify(t *testing.T) {
	for n, want := range map[int]string{-1: "neg", 0: "zero", 5: "pos", 50: "mid", 500: "big"} {
		if got := Classify(n); got != want {
			t.Fatalf("Classify(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSum(t *testing.T) {
	if Sum(4) != 6 {
		t.Fatal("sum")
	}
}

func TestApply(t *testing.T) {
	if got := Apply([]int{1, 2}); len(got) != 2 || got[1] != 4 {
		t.Fatalf("apply %v", got)
	}
}

func TestDouble(t *testing.T) {
	if Double(2) != 4 {
		t.Fatal("double")
	}
}
`,
}

func qualifyModule(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fixture shells out to POSIX paths")
	}
	root := t.TempDir()
	for name, body := range qualifyFixture {
		write(t, root, name, body)
	}
	run(t, root, "git", "init", "-q")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	return root
}

func readReport(t *testing.T, file string) QualReport {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var r QualReport
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// GLS-V0-030..033: every site of the fixture, each kind and operator,
// selected by Plan and run in full, misses nothing, and the control on the
// unexecuted body keeps and fails nothing.
func TestQualifyFixtureHasNoMiss(t *testing.T) {
	root := qualifyModule(t)
	index := filepath.Join(t.TempDir(), "index")
	if err := collect([]string{"-root", root, "-out", index, "-parallel", "4", "-test-timeout", "20s"}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	// Mutants run only under the deadline the index's runner records.
	if err := qualify([]string{"-root", root, "-index", index, "-packages", "calc", "-test-timeout", "21s", "-report", report}); err == nil || !strings.Contains(err.Error(), "test-deadline-mismatch") {
		t.Fatalf("qualify ran under another deadline than the index records: %v", err)
	}
	if err := qualify([]string{"-root", root, "-index", index, "-packages", "calc", "-start", "0", "-count", "1000", "-controls", "1", "-test-timeout", "20s", "-report", report}); err != nil {
		t.Fatal(err)
	}
	r := readReport(t, report)
	c := r.Counts
	if c.Mutants < 10 || c.Killed == 0 || c.Misses != 0 || c.Controls != 1 || c.ControlsFailed != 0 || r.Sites != r.Order {
		t.Fatalf("counts %+v sites %d order %d", c, r.Sites, r.Order)
	}
	for _, kind := range siteKinds {
		if c.ByKind[kind] == 0 {
			t.Errorf("no valid %s mutant: %v", kind, c.ByKind)
		}
	}
	for _, op := range []string{"delete", "negate", "constant"} {
		if c.ByOperator[op] == 0 {
			t.Errorf("no valid %s mutant: %v", op, c.ByOperator)
		}
	}
	if !strings.HasPrefix(r.Controls[0].Site.Path, "calc/") || r.Controls[0].Site.Line < strings.Count(qualifyFixture["calc/calc.go"][:strings.Index(qualifyFixture["calc/calc.go"], "func Unused")], "\n") {
		t.Errorf("control outside the unexecuted body: %+v", r.Controls[0].Site)
	}
	// The same seed orders the sites the same way; another seed does not.
	again := filepath.Join(t.TempDir(), "again.json")
	if err := qualify([]string{"-root", root, "-index", index, "-packages", "calc", "-start", "0", "-count", "3", "-report", again}); err != nil {
		t.Fatal(err)
	}
	if a := readReport(t, again); a.Mutants[0].Site != r.Mutants[0].Site || a.Mutants[2].Site != r.Mutants[2].Site {
		t.Errorf("seeded order differs: %+v vs %+v", a.Mutants[0].Site, r.Mutants[0].Site)
	}
}

// GLS-V0-031: an index whose record of TestDouble lost Double's block, now
// attributed to TestSum, must report the miss of every Double mutant
// TestDouble fails, and qualify must fail.
func TestQualifyReportsASeededMiss(t *testing.T) {
	root := qualifyModule(t)
	ctx := context.Background()
	commit := strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
	const deadline = 20 * time.Second
	run, goflags, err := runner(root, deadline, "")
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := linecov.Collect(ctx, root, linecov.CollectOptions{Module: "example.com/m", Packages: []string{"calc"}, Work: t.TempDir(), Parallel: 4, Timeout: time.Minute, SuiteTimeout: time.Minute, TestTimeout: deadline, GOFLAGS: goflags})
	if err != nil {
		t.Fatal(err)
	}
	src := qualifyFixture["calc/calc.go"]
	line := strings.Count(src[:strings.Index(src, "func Double")], "\n") + 1
	// Double's one block starts at its first statement, the line after.
	block := regexp.MustCompile(fmt.Sprintf(`(?m)^(example\.com/m/calc/calc\.go:%d\.\d+,\d+\.\d+ \d+) (\d+)$`, line+1))
	var records []linecov.Record
	for _, o := range outcomes {
		r := *o.Record
		switch r.Name {
		case "TestDouble":
			r.Profile = block.ReplaceAll(r.Profile, []byte("$1 0"))
		case "TestSum":
			r.Profile = append(r.Profile, []byte(fmt.Sprintf("example.com/m/calc/calc.go:%d.2,%d.14 1 1\n", line+1, line+1))...)
		case linecov.InOrder:
			// Its Double block would keep the package whole; without the
			// record the package is unordered (GLS-V0-008).
			r.Profile = block.ReplaceAll(r.Profile, []byte("$1 0"))
		}
		records = append(records, r)
	}
	oids, err := linecov.OIDs(ctx, root, commit)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := linecov.Build("example.com/m", commit, run, oids, records)
	if err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(t.TempDir(), "index")
	var b bytes.Buffer
	if err := ix.Encode(&b); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.json")
	err = qualify([]string{"-root", root, "-index", index, "-packages", "calc", "-start", "0", "-count", "1000", "-test-timeout", "20s", "-report", report})
	if err == nil || !strings.Contains(err.Error(), "misses") {
		t.Fatalf("qualify accepted a seeded miss: %v", err)
	}
	r := readReport(t, report)
	found := false
	for _, m := range r.Mutants {
		for _, missed := range m.Missed {
			if missed == "calc.TestDouble" && m.Site.Line == line+1 {
				found = true
			}
		}
	}
	if r.Counts.Misses == 0 || !found {
		t.Fatalf("no miss of calc.TestDouble on a Double mutant: %+v", r.Counts)
	}
}

// GLS-V0-033, GLS-V0-034: summarize folds disjoint chunks of one order,
// refuses others, and writes a record only for zero misses and passing
// controls; qualified accepts exactly the recorded blobs.
func TestSummarizeAndQualified(t *testing.T) {
	root := t.TempDir()
	for _, p := range qualifiedSources {
		write(t, root, p, "package x // "+p+"\n")
	}
	run(t, root, "git", "init", "-q")
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "m")
	dir := t.TempDir()
	runnerID, _, err := runner(root, linecov.DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	report := func(name string, start, sites int, mutate func(*QualReport)) string {
		sources := map[string]string{}
		for _, p := range qualifiedSources {
			sources[p] = strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD:"+p))
		}
		r := QualReport{Sources: sources, Profile: QualificationProfile, Seed: 7, Index: ReportIndex{Commit: "c", Digest: "sha256:d", Runner: runnerID},
			Packages: []string{"p"}, Suite: []string{"p"}, Start: start, Sites: sites, Order: 10}
		for i := start; i < start+sites; i++ {
			r.Mutants = append(r.Mutants, MutantResult{Index: i, Site: Site{Path: "p/p.go", Kind: "statement", Operator: "delete"}, State: "killed", KeptCount: 1, Failing: []string{"p.TestA"}})
		}
		if start == 0 {
			r.Controls = []MutantResult{{Site: Site{Path: "p/p.go", Kind: "statement", Operator: "delete"}, State: "control-passed"}}
		}
		if mutate != nil {
			mutate(&r)
		}
		file := filepath.Join(dir, name)
		if err := writeJSON(file, r); err != nil {
			t.Fatal(err)
		}
		return file
	}
	a, b := report("a.json", 0, 4, nil), report("b.json", 4, 3, nil)
	record := filepath.Join(dir, "qualification.json")
	if err := summarizeReports(root, record, []string{a, b}); err != nil {
		t.Fatal(err)
	}
	if err := qualified([]string{"-root", root, "-record", record, "-rev", "HEAD"}); err != nil {
		t.Fatalf("qualified refuses its own record: %v", err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	var wrongRunner Qualified
	if err := json.Unmarshal(data, &wrongRunner); err != nil {
		t.Fatal(err)
	}
	wrongRunner.Runner = "another runner"
	if err := writeJSON(record, wrongRunner); err != nil {
		t.Fatal(err)
	}
	if err := qualified([]string{"-root", root, "-record", record, "-rev", "HEAD"}); err == nil {
		t.Fatal("a qualification from another runner was accepted")
	}
	if err := os.WriteFile(record, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, files := range map[string][]string{
		"overlap":                {a, report("c.json", 3, 2, nil)},
		"another seed":           {a, report("d.json", 4, 3, func(r *QualReport) { r.Seed = 8 })},
		"another implementation": {a, report("f.json", 4, 1, func(r *QualReport) { r.Sources[qualifiedSources[0]] = "other" })},
		"failed baseline":        {a, report("g.json", 4, 1, func(r *QualReport) { r.Baseline = []BaselineResult{{Package: "p", State: "failed"}} })},
		"a miss":                 {a, report("e.json", 4, 1, func(r *QualReport) { r.Mutants[0].Missed = []string{"p.TestB"} })},
	} {
		if err := summarizeReports(root, filepath.Join(dir, name+".record"), files); err == nil {
			t.Errorf("%s: summarized", name)
		}
	}
	if err := summarizeReports(root, filepath.Join(dir, "nocontrol.record"), []string{b}); err == nil {
		t.Error("a record without a control was written")
	}
	parserPath := "internal/gateselect/readscopes.go"
	write(t, root, parserPath, "package x // parser changed\n")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qam", "change parser")
	if err := qualified([]string{"-root", root, "-record", record, "-rev", "HEAD"}); err == nil || !strings.Contains(err.Error(), parserPath) {
		t.Fatalf("changed read-scope parser: got %v, want a blob mismatch for %s", err, parserPath)
	}
	run(t, root, "git", "checkout", "-q", "--detach", "HEAD~1")
	write(t, root, qualifiedSources[3], "package x // changed\n")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qam", "change")
	if err := summarizeReports(root, filepath.Join(dir, "relabeled.record"), []string{a, b}); err == nil {
		t.Fatal("summarization relabeled old evidence with changed sources")
	}
	if err := qualified([]string{"-root", root, "-record", record, "-rev", "HEAD"}); err == nil {
		t.Fatal("qualified accepted a changed selector source")
	}
	if err := qualified([]string{"-root", root, "-record", record, "-rev", "HEAD~1"}); err != nil {
		t.Fatalf("qualified refuses the qualified commit: %v", err)
	}
}

// GLS-V0-031: a pre-existing failure is never counted as a mutant kill.
func TestQualifyRefusesFailingBaseline(t *testing.T) {
	root := module(t)
	index := filepath.Join(t.TempDir(), "index")
	if err := collect([]string{"-root", root, "-out", index}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "report.json")
	args := []string{"-root", root, "-index", index, "-packages", "p", "-mutants", "1", "-controls", "1", "-report", file}
	// This suite fails its baseline. An untracked declaration added after
	// collection must be rejected before that baseline can write a report.
	write(t, root, affected.ReadScopesPath, `{"profile":"corvint-test-read-scopes/0","packages":{"p":[]}}`)
	if err := qualify(args); err == nil || !strings.Contains(err.Error(), affected.ReadScopesPath) {
		t.Fatalf("untracked declaration: got %v, want declaration mismatch", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("baseline produced a report or could not be checked: %v", err)
	}
	if err := os.Remove(filepath.Join(root, affected.ReadScopesPath)); err != nil {
		t.Fatal(err)
	}
	err := qualify(args)
	if err == nil || !strings.Contains(err.Error(), "baseline failed") {
		t.Fatalf("failed baseline admitted: %v", err)
	}
	r := readReport(t, file)
	if len(r.Baseline) != 1 || r.Baseline[0].State != "failed" || len(r.Mutants) != 0 || r.Counts.Killed != 0 {
		t.Fatalf("baseline was promoted: %+v", r)
	}
}

// GLS-V0-058: -max-rss takes a positive byte count with an optional binary
// suffix and defaults to 8 GiB.
func TestMaxRSSFlag(t *testing.T) {
	if defaultMaxRSS != 8<<30 {
		t.Fatalf("default -max-rss is %d", defaultMaxRSS)
	}
	for in, want := range map[string]int64{"256MiB": 256 << 20, "8GiB": 8 << 30, "4096": 4096, "64KiB": 64 << 10} {
		var b byteSize
		if err := b.Set(in); err != nil || int64(b) != want {
			t.Errorf("Set(%q) = %d, %v; want %d", in, b, err, want)
		}
	}
	for _, in := range []string{"", "0", "-1", "1GB", "x", "9999999999GiB"} {
		var b byteSize
		if err := b.Set(in); err == nil {
			t.Errorf("Set(%q) accepted %d", in, b)
		}
	}
}

func TestGLSV0060ControlReadScopes(t *testing.T) {
	for _, tc := range []struct {
		name, packages, excluded string
		reject                   bool
	}{
		{"directory and reader's own scope", `{"reader":["lib/","reader/"]}`, "lib", false},
		{"exact file", `{"reader":["lib/data.txt"]}`, "lib", false},
		{"root file", `{"reader":["root.go"]}`, ".", false},
		{"untracked root data", `{"reader":["local.txt"]}`, "", false},
		{"outside suite", `{"other":["lib/"]}`, "", false},
		{"missing declaration", "", "", false},
		{"invalid JSON", "{", "", true},
		{"unknown entry form", `{"reader":[{"prefix":"lib"}]}`, "", true},
		{"misspelled reader", `{"raeder":["lib/"]}`, "", true},
		{"directory without Go files", `{"assets":["lib/"]}`, "", true},
		{"nested module outside suite", `{"nested":["lib/"]}`, "", true},
		{"fixture package outside suite", `{"reader/testdata/pkg":["lib/"]}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			suite := []string{".", "lib", "reader"}
			ix := &linecov.Index{Module: "example.com/fixture"}
			write(t, root, "go.mod", "module "+ix.Module+"\n\ngo 1.22\n")
			for _, pkg := range suite {
				write(t, root, filepath.Join(pkg, "root.go"), "package fixture\n")
				write(t, root, filepath.Join(pkg, "root_test.go"), "package fixture\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\n")
				ix.Tests = append(ix.Tests, linecov.Test{Package: pkg, Name: "TestOne"})
			}
			write(t, root, "lib/data.txt", "fixture data")
			write(t, root, "other/other.go", "package other\n")
			write(t, root, "assets/data", "fixture data")
			write(t, root, "nested/go.mod", "module example.com/nested\n")
			write(t, root, "nested/nested.go", "package nested\n")
			write(t, root, "reader/testdata/pkg/fixture.go", "package fixture\n")
			if tc.packages != "" {
				write(t, root, affected.ReadScopesPath, `{"profile":"corvint-test-read-scopes/0","packages":`+tc.packages+`}`)
			}
			run(t, root, "git", "init", "-q")
			run(t, root, "git", "add", ".")
			run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "scope fixture")
			ix.Commit = strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))
			write(t, root, "local.txt", "untracked data")
			ok, excluded, err := controlEligibility(root, ix, suite)
			if tc.reject {
				if err == nil || !strings.Contains(err.Error(), affected.ReadScopesPath) || ok != nil {
					t.Fatalf("invalid declaration returned eligibility %t, error %v", ok != nil, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, pkg := range suite {
				if want := pkg != tc.excluded; ok(pkg) != want {
					t.Errorf("eligible(%q) = %t, want %t", pkg, ok(pkg), want)
				}
			}
			var want []controlExclusion
			if tc.excluded != "" {
				want = []controlExclusion{{tc.excluded, "another suite package's declared test read scope covers it (GLS-V0-060)"}}
			}
			if !slices.Equal(excluded, want) {
				t.Errorf("exclusions = %v, want %v", excluded, want)
			}
		})
	}
}

// GLS-V0-060: equality includes presence, file type and the exact bytes.
func TestGLSV0060CheckoutDeclaration(t *testing.T) {
	const declaration = `{"profile":"corvint-test-read-scopes/0","packages":{"calc":[]}}`
	for _, tc := range []struct {
		name, committed, checkout string
	}{
		{"empty untracked file", "absent", ""},
		{"empty committed blob", "", ""},
		{"missing worktree file", declaration, "absent"},
		{"same JSON with different bytes", declaration, declaration + "\n"},
		{"directory at declaration path", "absent", "directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := qualifyModule(t)
			name := filepath.Join(root, affected.ReadScopesPath)
			if tc.committed != "absent" {
				write(t, root, affected.ReadScopesPath, tc.committed)
				run(t, root, "git", "add", ".")
				run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "scope")
			}
			ix := &linecov.Index{Commit: strings.TrimSpace(gitOut(t, root, "rev-parse", "HEAD"))}
			switch tc.checkout {
			case "absent":
				if err := os.Remove(name); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.MkdirAll(name, 0755); err != nil {
					t.Fatal(err)
				}
			default:
				write(t, root, affected.ReadScopesPath, tc.checkout)
			}
			ok, _, err := controlEligibility(root, ix, []string{"calc"})
			if err == nil || !strings.Contains(err.Error(), affected.ReadScopesPath) || ok != nil {
				t.Fatalf("invalid declaration returned eligibility %t, error %v", ok != nil, err)
			}
		})
	}
}

// GLS-V0-060: the saved report and its folded chunks identify every excluded
// control package even if no control was requested.
func TestGLSV0060ControlExclusionsReport(t *testing.T) {
	root := crossPackageModule(t)
	write(t, root, affected.ReadScopesPath, `{"profile":"corvint-test-read-scopes/0","packages":{"lib":["app/"],"app":["lib/"]}}`)
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "declare scopes")
	dir := t.TempDir()
	index := filepath.Join(dir, "index")
	if err := collect([]string{"-root", root, "-out", index}); err != nil {
		t.Fatal(err)
	}
	reason := "another suite package's declared test read scope covers it (GLS-V0-060)"
	want := []controlExclusion{{"app", reason}, {"lib", reason}}
	var reports []string
	for i := range 2 {
		report := filepath.Join(dir, fmt.Sprintf("report-%d.json", i))
		if err := qualify([]string{"-root", root, "-index", index, "-packages", "lib", "-suite", "lib,app", "-start", fmt.Sprint(i), "-count", "1", "-report", report}); err != nil {
			t.Fatal(err)
		}
		if got := readReport(t, report).ExcludedControls; !slices.Equal(got, want) {
			t.Fatalf("saved exclusions = %+v, want %+v", got, want)
		}
		reports = append(reports, report)
	}
	var err error
	summary := captureStdout(t, func() { err = summarizeReports(root, "", reports) })
	if err != nil {
		t.Fatal(err)
	}
	var got QualReport
	if err := json.Unmarshal([]byte(summary), &got); err != nil || !slices.Equal(got.ExcludedControls, want) {
		t.Fatalf("summary exclusions = %+v, want %+v (decode error %v)", got.ExcludedControls, want, err)
	}
	chunk := readReport(t, reports[1])
	chunk.ExcludedControls = nil
	if err := writeJSON(reports[1], chunk); err != nil {
		t.Fatal(err)
	}
	if err := summarizeReports(root, "", reports); err == nil {
		t.Fatal("summarized chunks with different control exclusions")
	}
}
