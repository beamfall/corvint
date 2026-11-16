// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

// twoRunnerFixture adds an RSpec run whose index keeps its one file whole
// whatever the change, so its control keeps both tests.
func twoRunnerFixture() RubyQualReport {
	rep := rubyReportFixture()
	rspec := rubyReportFixture().Runners[0]
	rspec.Family, rspec.Runner = "rspec", "ruby 3.3.8 rspec 3.13.7 coverage=lines,methods"
	rspec.Unordered = []string{"test/a_test.rb"}
	both := [][2]string{{"test/a_test.rb", "A#t1"}, {"test/a_test.rb", "A#t2"}}
	for i := range rspec.Faults {
		rspec.Faults[i].Selected = both
	}
	rep.Runners = append(rep.Runners, rspec)
	return rep
}

func rubyReportFixture() RubyQualReport {
	t1, t2 := [2]string{"test/a_test.rb", "A#t1"}, [2]string{"test/a_test.rb", "A#t2"}
	base := RubySuiteResult{Tests: 2, Failing: [][2]string{}, Inventory: [][2]string{t1, t2}}
	killed := RubySuiteResult{Exit: 1, Tests: 2, Failing: [][2]string{t1}}
	return RubyQualReport{
		Profile: RubyQualificationProfile, Seed: 20261116,
		Corpus: linecov.QualifiedCorpus{URL: "https://example.invalid/corpus", Commit: "c0", License: "MIT", LicenseSHA256: "x"},
		Runners: []RubyQualRunner{{
			Family: "minitest", Runner: "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods", IndexDigest: "sha256:ix", Baseline: base,
			Faults: []RubyQualFault{
				{Kind: "statement-delete", Path: "lib/a.rb", Line: 3, Head: "h1", Suite: killed, Selected: [][2]string{t1}},
				{Kind: "constant-change", Path: "lib/a.rb", Line: 5, Head: "h2", Suite: base, Selected: [][2]string{t1, t2}, Fallback: "the change reaches load-time code"},
				{Kind: "negative-control", Path: "lib/a.rb", Line: 9, Control: true, Head: "h3", Suite: base, Selected: [][2]string{}},
			},
		}},
	}
}

// LPCV-V0-7608: the fold restates kills, selected-set sizes, controls and
// misses, and refuses a miss, a failed control, an incomplete fault run or a
// failing baseline; with -record it writes a record CheckRubyQualification
// admits only at the qualified revision.
func TestRubyQualifySummarize(t *testing.T) {
	runs, verdicts, err := foldRubyReport(rubyReportFixture())
	if err != nil {
		t.Fatal(err)
	}
	r := runs[0]
	if r.Faults != 2 || r.Killed != 1 || r.Controls != 1 || r.ControlsFailed != 0 || r.Misses != 0 || len(r.Selected) != 2 || r.Selected[0] != 1 || r.Selected[1] != 2 || len(verdicts) != 3 || !verdicts[2].Passed || len(r.ControlsKept) != 1 || r.ControlsKept[0] != 0 {
		t.Fatalf("runs %+v verdicts %+v", runs, verdicts)
	}
	if r.Narrowed != 1 || r.Unqualified != "" || !verdicts[0].Narrowed || verdicts[1].Narrowed {
		t.Fatalf("narrowing: run %+v verdicts %+v", r, verdicts)
	}
	// The strict control rule (GLS-V0-032): a control that keeps tests of a
	// file the index keeps whole whatever the change is no control. Its run
	// is restated as unqualified while the other runner still qualifies.
	runs, verdicts, err = foldRubyReport(twoRunnerFixture())
	if err != nil {
		t.Fatal(err)
	}
	if runs[0].Unqualified != "" || !strings.Contains(runs[1].Unqualified, "no control") || runs[1].ControlsKept[0] != 2 || runs[1].KeptRegardless != 2 || runs[1].Unordered != 1 || runs[1].ControlsFailed != 0 || verdicts[5].Passed {
		t.Fatalf("kept-regardless control: %+v %+v", runs, verdicts[5])
	}
	for name, mutate := range map[string]func(*RubyQualReport){
		"miss":            func(r *RubyQualReport) { r.Runners[0].Faults[0].Selected = [][2]string{{"test/a_test.rb", "A#t2"}} },
		"control selects": func(r *RubyQualReport) { r.Runners[0].Faults[2].Selected = [][2]string{{"test/a_test.rb", "A#t2"}} },
		"control kills": func(r *RubyQualReport) {
			r.Runners[0].Faults[2].Suite.Failing = [][2]string{{"test/a_test.rb", "A#t2"}}
		},
		"incomplete suite": func(r *RubyQualReport) { r.Runners[0].Faults[1].Suite.Tests = 1 },
		"control fallback": func(r *RubyQualReport) { r.Runners[0].Faults[2].Fallback = "the change reaches load-time code" },
		"control by coverage": func(r *RubyQualReport) {
			r.Runners[0].Unordered = []string{"test/a_test.rb"}
			r.Runners[0].Faults[2].Selected = [][2]string{{"test/a_test.rb", "A#t1"}}
			r.Runners[0].Faults[2].KeptByCoverage = 1
		},
		"selected outside inventory": func(r *RubyQualReport) {
			r.Runners[0].Faults[0].Selected = [][2]string{{"test/a_test.rb", "A#t1"}, {"x", "y"}}
		},
		"no inventory":       func(r *RubyQualReport) { r.Runners[0].Baseline.Inventory = nil },
		"failing baseline":   func(r *RubyQualReport) { r.Runners[0].Baseline.Failing = [][2]string{{"test/a_test.rb", "A#t2"}} },
		"skipped baseline":   func(r *RubyQualReport) { r.Runners[0].Baseline.Skipped = 1 },
		"no runner identity": func(r *RubyQualReport) { r.Runners[0].Runner = "" },
		"wrong profile":      func(r *RubyQualReport) { r.Profile = "other" },
	} {
		rep := rubyReportFixture()
		mutate(&rep)
		if _, _, err := foldRubyReport(rep); err == nil {
			t.Errorf("%s: folded without error", name)
		}
	}

	root := t.TempDir()
	run(t, root, "git", "init", "-q")
	for _, p := range linecov.RubyQualifiedSources {
		write(t, root, p, "fixture "+p+"\n")
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "sources")
	rev, _ := output(root, "git", "rev-parse", "HEAD")
	rep := twoRunnerFixture()
	rep.SelectorRevision = rev
	data, _ := json.Marshal(rep)
	reportFile := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(reportFile, data, 0o644); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(root, linecov.RubyQualificationRecord)
	if err := os.MkdirAll(filepath.Dir(record), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := rubyQualify([]string{"-report", reportFile, "-root", root, "-record", record}); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "record")
	head, _ := output(root, "git", "rev-parse", "HEAD")
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: head}, {Key: "vcs.modified", Value: "false"}}}
	if err := linecov.CheckRubyQualification(context.Background(), root, rep.Runners[0].Runner, info); err != nil {
		t.Fatalf("written record not admitted: %v", err)
	}
	// The record names only the qualifying runner and refuses the other.
	if err := linecov.CheckRubyQualification(context.Background(), root, rep.Runners[1].Runner, info); err == nil || !strings.Contains(err.Error(), "does not qualify runner") {
		t.Fatalf("the unqualified runner was admitted: %v", err)
	}
	var written linecov.Qualified
	if data, err := os.ReadFile(record); err != nil || json.Unmarshal(data, &written) != nil || len(written.Runners) != 1 || written.Runners[0] != rep.Runners[0].Runner || len(written.Report.Runs) != 2 || written.Report.Runs[1].Unqualified == "" {
		t.Fatalf("written record %+v: %v", written, err)
	}
	// A source changed since the qualified revision refuses a record.
	write(t, root, "internal/rubylinecov/lines.go", "changed\n") //corvint:fixture-path
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qam", "change")
	if err := rubyQualify([]string{"-report", reportFile, "-root", root, "-record", filepath.Join(t.TempDir(), "r.json")}); err == nil || !strings.Contains(err.Error(), "rerun the qualification") {
		t.Fatalf("stale sources recorded: %v", err)
	}
}

// LPCV-V0-7608: the committed Ruby record, when present, is a passing record
// binding exactly RubyQualifiedSources. It qualifies Minitest with a control
// that kept no test and a killed fault that narrowed the suite, and every
// run it restates without qualifying carries its reason and is not named as
// a runner.
func TestRubyQualificationRecordPresent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "linecov", "ruby-qualification.json"))
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("no Ruby qualification record in this tree")
	}
	if err != nil {
		t.Fatal(err)
	}
	var q linecov.Qualified
	if err := json.Unmarshal(data, &q); err != nil {
		t.Fatal(err)
	}
	if q.Profile != linecov.QualifiedProfile || q.Family != "ruby" || q.Corpus == nil || q.Report.Misses != 0 || q.Report.Killed == 0 || q.Report.Controls == 0 || len(q.Report.Runs) == 0 {
		t.Fatalf("record %+v", q)
	}
	var qualified []string
	families := map[string]bool{}
	for _, r := range q.Report.Runs {
		if r.Misses != 0 || r.ControlsFailed != 0 || len(r.Selected) != r.Faults || len(r.ControlsKept) != r.Controls {
			t.Fatalf("run %+v", r)
		}
		if r.Unqualified != "" {
			continue
		}
		qualified = append(qualified, r.Runner)
		families[strings.Fields(r.Runner + " x x")[2]] = true
		if r.Controls == 0 || r.Narrowed == 0 || r.KeptRegardless != 0 || r.Unordered != 0 {
			t.Fatalf("qualifying run %+v", r)
		}
		for _, kept := range r.ControlsKept {
			if kept != 0 {
				t.Fatalf("qualifying run %q has a control keeping %d tests", r.Runner, kept)
			}
		}
	}
	if !slices.Equal(qualified, q.Runners) || !families["minitest"] {
		t.Fatalf("runners %v, qualifying runs %v", q.Runners, qualified)
	}
	if len(q.Sources) != len(linecov.RubyQualifiedSources) {
		t.Fatalf("record binds %d sources, RubyQualifiedSources lists %d", len(q.Sources), len(linecov.RubyQualifiedSources))
	}
	for _, p := range linecov.RubyQualifiedSources {
		if q.Sources[p] == "" {
			t.Fatalf("record does not bind %s", p)
		}
	}
}

// LPCV-V0-7608: the Ruby commands take nothing from a file the Ruby record
// does not bind. Every top-level name languages.go and rubyqualify.go use is
// declared in a bound file of this package.
func TestRubyCommandsUseBoundFiles(t *testing.T) {
	bound := map[string]bool{}
	for _, p := range linecov.RubyQualifiedSources {
		if dir, name := filepath.Split(p); dir == "tools/linecov/" {
			bound[name] = true
		}
	}
	for _, name := range []string{"languages.go", "main.go", "rubyqualify.go"} {
		if !bound[name] {
			t.Fatalf("%s is not a Ruby qualified source", name)
		}
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	unbound := map[string]string{} // top-level name -> the unbound file declaring it
	parsed := map[string]*ast.File{}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		parsed[name] = f
		if bound[name] {
			continue
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					unbound[d.Name.Name] = name
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						for _, n := range spec.Names {
							unbound[n.Name] = name
						}
					case *ast.TypeSpec:
						unbound[spec.Name.Name] = name
					}
				}
			}
		}
	}
	if len(unbound) == 0 {
		t.Fatal("found no declaration in an unbound file")
	}
	for _, name := range []string{"languages.go", "rubyqualify.go"} {
		var visit func(n ast.Node) bool
		visit = func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr: // pkg.Name or value.field: only the left side is this package's
				ast.Inspect(n.X, visit)
				return false
			case *ast.Ident:
				if unbound[n.Name] != "" {
					t.Errorf("%s uses %s, declared in %s, which the Ruby record does not bind", fset.Position(n.Pos()), n.Name, unbound[n.Name])
				}
			}
			return true
		}
		ast.Inspect(parsed[name], visit)
	}
}

// LPCV-V0-7608: a fault run in which no killed fault kept fewer tests than
// the suite proves nothing about narrowing, and a control that keeps any
// test, even one the index keeps whatever the change, is no control: neither
// run qualifies its runner, so the fold refuses.
func TestRubyQualifyRefusesUnprovenRunner(t *testing.T) {
	t1, t2 := [2]string{"test/a_test.rb", "A#t1"}, [2]string{"test/a_test.rb", "A#t2"}
	whole := rubyReportFixture()
	whole.Runners[0].Faults[0].Selected = [][2]string{t1, t2}
	whole.Runners[0].Faults[0].Fallback = "the change reaches load-time code"
	if _, _, err := foldRubyReport(whole); err == nil || !strings.Contains(err.Error(), "narrow") {
		t.Errorf("a run with no narrowed kill folded: %v", err)
	}
	regardless := rubyReportFixture()
	regardless.Runners[0].Unordered = []string{"test/a_test.rb"}
	regardless.Runners[0].Faults[2].Selected = [][2]string{t1, t2}
	if _, _, err := foldRubyReport(regardless); err == nil || !strings.Contains(err.Error(), "control") {
		t.Errorf("a run whose control kept two tests folded: %v", err)
	}
}

// LPCV-V0-7608: the fold refuses what admission refuses, so the command
// cannot write a record CheckRubyQualification would reject. An index that
// keeps a file whole whatever the change gives its run no control even when
// no baseline test belongs to that file, and a run with no fault at all is
// not a passing run.
func TestRubyQualifyRefusesWhatAdmissionRefuses(t *testing.T) {
	unordered := rubyReportFixture()
	unordered.Runners[0].Unordered = []string{"test/other_test.rb"}
	runs, _, err := foldRubyReport(unordered)
	if err == nil || !strings.Contains(err.Error(), "no runner qualifies") || len(runs) != 1 || !strings.Contains(runs[0].Unqualified, "no control") {
		t.Errorf("a run whose index keeps a file whole folded as qualifying: runs %+v: %v", runs, err)
	}
	noFault := rubyReportFixture()
	second := rubyReportFixture().Runners[0]
	second.Family, second.Runner = "rspec", "ruby 3.3.8 rspec 3.13.7 coverage=lines,methods"
	second.Faults = second.Faults[2:]
	noFault.Runners = append(noFault.Runners, second)
	if _, _, err := foldRubyReport(noFault); err == nil || !strings.Contains(err.Error(), "no fault") {
		t.Errorf("a run with no fault folded: %v", err)
	}

	// Through the command: neither report leaves a record behind, and a
	// report with one run of each kind writes a record admission accepts for
	// the qualifying runner and refuses for the other.
	root := t.TempDir()
	run(t, root, "git", "init", "-q")
	for _, p := range linecov.RubyQualifiedSources {
		write(t, root, p, "fixture "+p+"\n")
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "sources")
	rev, _ := output(root, "git", "rev-parse", "HEAD")
	record := filepath.Join(root, linecov.RubyQualificationRecord)
	if err := os.MkdirAll(filepath.Dir(record), 0o755); err != nil {
		t.Fatal(err)
	}
	qualify := func(rep RubyQualReport) error {
		rep.SelectorRevision = rev
		data, err := json.Marshal(rep)
		if err != nil {
			t.Fatal(err)
		}
		reportFile := filepath.Join(t.TempDir(), "report.json")
		if err := os.WriteFile(reportFile, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return rubyQualify([]string{"-report", reportFile, "-root", root, "-record", record})
	}
	for name, rep := range map[string]RubyQualReport{"unordered index": unordered, "run with no fault": noFault} {
		if err := qualify(rep); err == nil {
			t.Errorf("%s: the command wrote a record", name)
		}
		if _, err := os.Stat(record); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s: a record exists after the refusal: %v", name, err)
		}
	}
	mixed := rubyReportFixture()
	other := rubyReportFixture().Runners[0]
	other.Family, other.Runner = "rspec", "ruby 3.3.8 rspec 3.13.7 coverage=lines,methods"
	other.Unordered = []string{"test/other_test.rb"}
	mixed.Runners = append(mixed.Runners, other)
	if err := qualify(mixed); err != nil {
		t.Fatal(err)
	}
	run(t, root, "git", "add", ".")
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "record")
	head, _ := output(root, "git", "rev-parse", "HEAD")
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: head}, {Key: "vcs.modified", Value: "false"}}}
	if err := linecov.CheckRubyQualification(context.Background(), root, mixed.Runners[0].Runner, info); err != nil {
		t.Errorf("the qualifying runner of the written record is refused: %v", err)
	}
	if err := linecov.CheckRubyQualification(context.Background(), root, other.Runner, info); err == nil || !strings.Contains(err.Error(), "does not qualify runner") {
		t.Errorf("the runner with an unordered index is admitted: %v", err)
	}
}
