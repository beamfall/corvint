// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/linecov"
)

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
	// GLS-V0-032: tests of a file the index keeps whole whatever the change
	// do not fail a control; the record restates how many it kept.
	regardless := rubyReportFixture()
	regardless.Runners[0].Unordered = []string{"test/a_test.rb"}
	regardless.Runners[0].Faults[2].Selected = [][2]string{{"test/a_test.rb", "A#t1"}, {"test/a_test.rb", "A#t2"}}
	runs, _, err = foldRubyReport(regardless)
	if err != nil || runs[0].ControlsKept[0] != 2 || runs[0].KeptRegardless != 2 || runs[0].Unordered != 1 {
		t.Fatalf("kept-regardless control: %v %+v", err, runs)
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
	rep := rubyReportFixture()
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
	// A source changed since the qualified revision refuses a record.
	write(t, root, "internal/rubylinecov/lines.go", "changed\n") //corvint:fixture-path
	run(t, root, "git", "-c", "user.name=t", "-c", "user.email=t@invalid", "-c", "commit.gpgsign=false", "commit", "-qam", "change")
	if err := rubyQualify([]string{"-report", reportFile, "-root", root, "-record", filepath.Join(t.TempDir(), "r.json")}); err == nil || !strings.Contains(err.Error(), "rerun the qualification") {
		t.Fatalf("stale sources recorded: %v", err)
	}
}

// LPCV-V0-7608: the committed Ruby record, when present, is a passing
// two-runner record binding exactly RubyQualifiedSources with a control that
// selected nothing in each run.
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
	if q.Profile != QualifiedProfile || q.Family != "ruby" || q.Corpus == nil || q.Report.Misses != 0 || q.Report.Killed == 0 || q.Report.Controls == 0 || len(q.Runners) != 2 || len(q.Report.Runs) != 2 {
		t.Fatalf("record %+v", q)
	}
	families := map[string]bool{}
	for i, r := range q.Report.Runs {
		families[strings.Fields(r.Runner + " x x")[2]] = true
		if r.Runner != q.Runners[i] || r.Misses != 0 || r.ControlsFailed != 0 || r.Controls == 0 || len(r.Selected) != r.Faults {
			t.Fatalf("run %+v", r)
		}
	}
	if !families["minitest"] || !families["rspec"] {
		t.Fatalf("runners %v", q.Runners)
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
