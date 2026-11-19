// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// GLS-V0-034: every non-test source of this package is bound, so a change to
// any of them, such as the syscall environment helper the collector injects
// into each test binary, invalidates an older qualification record.
func TestQualifiedSourcesBindEveryPackageSource(t *testing.T) {
	bound := map[string]bool{}
	for _, p := range QualifiedSources {
		bound[p] = true
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		if p := "internal/linecov/" + f; !bound[p] {
			t.Errorf("%s is not a qualified source", p)
		}
	}
}

// GLS-V0-034: the selector's sources outside this package are bound too: the
// CLI line selection, the static reader rules whose answer permits an
// exclusion (affected.Graph.Readers), and the qualification and CI helpers.
func TestQualifiedSourcesBindSelectorDependencies(t *testing.T) {
	bound := map[string]bool{}
	for _, p := range QualifiedSources {
		bound[p] = true
	}
	for _, p := range []string{
		"cmd/corvint/affected_linecov.go",
		"internal/gateselect/readscopes.go",
		"internal/liveverify/affected/readers.go",
		"tools/linecov/ci.go",
		"tools/linecov/main.go",
		"tools/linecov/qualify.go",
	} {
		if !bound[p] {
			t.Errorf("%s is not a qualified source", p)
		}
	}
}

// GLS-V0-020/034, LPCV-V0-955/963: exclusion requires qualification of the
// actual runner and executable, not just a matching coverage index.
func TestLocalQualificationRequiresMatchingEvidence(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	for _, p := range QualifiedSources {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "sources")
	rev := git(t, root, "rev-parse", "HEAD")
	oids, err := OIDs(context.Background(), root, rev)
	if err != nil {
		t.Fatal(err)
	}
	q := Qualified{Profile: "linecov-qualified/1", Family: "go", Runner: "runner", Sources: oids, Report: QualifiedReport{Digest: "sha256:fixture", Mutants: 1, Killed: 1, Controls: 1}}
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"}}}
	for _, tc := range []struct {
		name   string
		mutate func(*Qualified)
		runner string
		info   *debug.BuildInfo
		want   bool
	}{
		{"match", func(*Qualified) {}, "runner", info, true},
		{"miss", func(q *Qualified) { q.Report.Misses = 1 }, "runner", info, false},
		{"no controls", func(q *Qualified) { q.Report.Controls = 0 }, "runner", info, false},
		{"no kills", func(q *Qualified) { q.Report.Killed = 0 }, "runner", info, false},
		{"runner mismatch", func(*Qualified) {}, "other", info, false},
		{"missing provenance", func(*Qualified) {}, "runner", nil, false},
		{"dirty executable", func(*Qualified) {}, "runner", &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "true"}}}, false},
		{"wrong blob", func(q *Qualified) { q.Sources = map[string]string{} }, "runner", info, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git(t, root, "checkout", "-q", "--detach", rev)
			record := q
			tc.mutate(&record)
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			full := filepath.Join(root, ".github/linecov/qualification.json")
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, data, 0644); err != nil {
				t.Fatal(err)
			}
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "record")
			base := git(t, root, "rev-parse", "HEAD")
			err = CheckQualification(context.Background(), root, base, tc.runner, tc.info)
			if (err == nil) != tc.want {
				t.Fatalf("admitted=%v want %v: %v", err == nil, tc.want, err)
			}
			if tc.want {
				// Source drift in the running executable invalidates an older passing record.
				if err := os.WriteFile(filepath.Join(root, QualifiedSources[0]), []byte("changed\n"), 0644); err != nil {
					t.Fatal(err)
				}
				git(t, root, "commit", "-qam", "changed selector")
				changed := *info
				changed.Settings = append([]debug.BuildSetting(nil), info.Settings...)
				changed.Settings[0].Value = git(t, root, "rev-parse", "HEAD")
				if err := CheckQualification(context.Background(), root, base, "runner", &changed); err == nil {
					t.Fatal("changed executable qualified")
				}
			}
		})
	}
	if err := CheckQualification(context.Background(), root, rev, "runner", info); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing record admitted: %v", err)
	}
}

// LPCV-V0-7608: the Ruby record binds every Ruby selector and collector
// source, including each non-test source of internal/rubylinecov.
func TestRubyQualifiedSourcesBindRubySelector(t *testing.T) {
	bound := map[string]bool{}
	for _, p := range RubyQualifiedSources {
		bound[p] = true
	}
	files, err := filepath.Glob("../rubylinecov/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		if strings.HasSuffix(name, "_test.go") || !(strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".rb")) {
			continue
		}
		if p := "internal/rubylinecov/" + name; !bound[p] {
			t.Errorf("%s is not a Ruby qualified source", p)
		}
	}
	for _, p := range []string{"internal/linecov/admission.go", "internal/linecov/select.go", "tools/linecov/languages.go", "tools/linecov/rubyqualify.go", "internal/rubydialect/scanner.go"} {
		if !bound[p] {
			t.Errorf("%s is not a Ruby qualified source", p)
		}
	}
}

// unqualifyRSpec restates the second run as measured but not qualifying.
func unqualifyRSpec(q *Qualified) {
	q.Report.Runs[1].ControlsKept, q.Report.Runs[1].KeptRegardless, q.Report.Runs[1].Unordered = []int{3}, 3, 1
	q.Report.Runs[1].Unqualified = "no control: the index keeps 3 tests whatever the change"
	q.Runners = q.Runners[:1]
}

// LPCV-V0-7608: the Ruby family is admitted only by a passing record at the
// selector's clean revision whose sources match it and one of whose runs
// qualifies the index runner; a stale binding, a miss, a failed control, a
// control that kept any test, a run with no narrowed kill, a run restated as
// unqualified or an unnamed runner refuses.
func TestRubyQualificationAdmission(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	for _, p := range RubyQualifiedSources {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture "+p+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "sources")
	oids, err := OIDs(context.Background(), root, git(t, root, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	const mini, rspec = "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods", "ruby 3.3.8 rspec 3.13.7 coverage=lines,methods"
	run := func(runner string) QualifiedRun {
		return QualifiedRun{Runner: runner, IndexDigest: "sha256:ix", Tests: 23, Faults: 7, Killed: 5, Narrowed: 5, Selected: []int{1, 2, 3, 4, 5, 6, 7}, Controls: 1, ControlsKept: []int{0}}
	}
	q := Qualified{Profile: "linecov-qualified/1", Family: "ruby", IndexCommit: "corpus", Runners: []string{mini, rspec},
		Corpus:  &QualifiedCorpus{URL: "https://example.invalid/webmock", Commit: "corpus", License: "MIT", LicenseSHA256: "x"},
		Report:  QualifiedReport{Digest: "sha256:fixture", Seed: 1, Mutants: 14, Killed: 10, Controls: 2, Runs: []QualifiedRun{run(mini), run(rspec)}},
		Sources: map[string]string{}}
	for _, p := range RubyQualifiedSources {
		q.Sources[p] = oids[p]
	}
	commit := func(t *testing.T, record Qualified) *debug.BuildInfo {
		t.Helper()
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		full := filepath.Join(root, RubyQualificationRecord)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-q", "--allow-empty", "-m", "record")
		return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: git(t, root, "rev-parse", "HEAD")}, {Key: "vcs.modified", Value: "false"}}}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Qualified)
		runner string
		dirty  bool
		root   string
		want   bool
	}{
		{"match minitest", func(*Qualified) {}, mini, false, root, true},
		{"match rspec", func(*Qualified) {}, rspec, false, root, true},
		{"unnamed runner", func(*Qualified) {}, "ruby 3.4.0 minitest 5.27.0 coverage=lines,methods", false, root, false},
		{"miss", func(q *Qualified) { q.Report.Misses = 1 }, mini, false, root, false},
		{"run miss", func(q *Qualified) { q.Report.Runs[1].Misses = 1 }, mini, false, root, false},
		{"failed control", func(q *Qualified) { q.Report.Runs[0].ControlsFailed = 1 }, mini, false, root, false},
		{"no control", func(q *Qualified) { q.Report.Runs[0].Controls = 0 }, mini, false, root, false},
		{"no kill", func(q *Qualified) { q.Report.Runs[0].Killed = 0 }, mini, false, root, false},
		{"missing selected sizes", func(q *Qualified) { q.Report.Runs[0].Selected = q.Report.Runs[0].Selected[:3] }, mini, false, root, false},
		{"control kept by coverage", func(q *Qualified) { q.Report.Runs[1].ControlsKept = []int{3} }, mini, false, root, false},
		// The strict control rule: tests the index keeps whatever the change
		// are still kept tests, so the run has no control and the record
		// claiming it as qualifying is refused for every runner.
		{"control kept regardless", func(q *Qualified) { q.Report.Runs[1].ControlsKept = []int{3}; q.Report.Runs[1].KeptRegardless = 3 }, rspec, false, root, false},
		{"control kept regardless refuses the other runner too", func(q *Qualified) { q.Report.Runs[1].ControlsKept = []int{3}; q.Report.Runs[1].KeptRegardless = 3 }, mini, false, root, false},
		{"unrecorded index", func(q *Qualified) { q.Report.Runs[1].KeptRegardless, q.Report.Runs[1].Unordered = 3, 1 }, rspec, false, root, false},
		{"unqualified run refuses its runner", unqualifyRSpec, rspec, false, root, false},
		{"unqualified run leaves the other runner admitted", unqualifyRSpec, mini, false, root, true},
		{"unqualified run still listed as a runner", func(q *Qualified) { q.Report.Runs[1].Unqualified = "no control" }, mini, false, root, false},
		{"qualifying run not listed as a runner", func(q *Qualified) { q.Runners = q.Runners[:1] }, mini, false, root, false},
		{"unqualified run with a miss", func(q *Qualified) { unqualifyRSpec(q); q.Report.Runs[1].Misses = 1 }, mini, false, root, false},
		{"no narrowed kill", func(q *Qualified) { q.Report.Runs[0].Narrowed = 0 }, mini, false, root, false},
		{"narrowed beyond the kills", func(q *Qualified) { q.Report.Runs[0].Narrowed = 6 }, mini, false, root, false},
		{"narrowed beyond the smaller sets", func(q *Qualified) { q.Report.Runs[0].Selected = []int{1, 23, 23, 23, 23, 23, 23} }, mini, false, root, false},
		{"missing control sizes", func(q *Qualified) { q.Report.Runs[0].ControlsKept = nil }, mini, false, root, false},
		{"go family", func(q *Qualified) { q.Family = "go" }, mini, false, root, false},
		{"no corpus", func(q *Qualified) { q.Corpus = nil }, mini, false, root, false},
		{"stale binding", func(q *Qualified) { q.Sources["internal/rubylinecov/lines.go"] = "0000" }, mini, false, root, false},
		{"dirty executable", func(*Qualified) {}, mini, true, root, false},
		{"no selector root", func(*Qualified) {}, mini, false, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := q
			record.Runners = append([]string(nil), q.Runners...)
			record.Report.Runs = append([]QualifiedRun(nil), q.Report.Runs...)
			for i := range record.Report.Runs {
				record.Report.Runs[i].ControlsKept = append([]int(nil), q.Report.Runs[i].ControlsKept...)
			}
			record.Sources = map[string]string{}
			for k, v := range q.Sources {
				record.Sources[k] = v
			}
			tc.mutate(&record)
			info := commit(t, record)
			if tc.dirty {
				info.Settings[1].Value = "true"
			}
			err := CheckRubyQualification(context.Background(), tc.root, tc.runner, info)
			if (err == nil) != tc.want {
				t.Fatalf("admitted=%v want %v: %v", err == nil, tc.want, err)
			}
		})
	}
	// A selector source changed after qualification invalidates the record.
	commit(t, q)
	if err := os.WriteFile(filepath.Join(root, "internal/rubylinecov/lines.go"), []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "commit", "-qam", "changed selector")
	info := &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: git(t, root, "rev-parse", "HEAD")}, {Key: "vcs.modified", Value: "false"}}}
	if err := CheckRubyQualification(context.Background(), root, mini, info); err == nil || !strings.Contains(err.Error(), "source mismatch") {
		t.Fatalf("changed selector admitted: %v", err)
	}
	// A revision without the record refuses.
	bare := t.TempDir()
	git(t, bare, "init", "-q")
	if err := os.WriteFile(filepath.Join(bare, "x"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	git(t, bare, "add", ".")
	git(t, bare, "commit", "-qm", "x")
	info = &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: git(t, bare, "rev-parse", "HEAD")}, {Key: "vcs.modified", Value: "false"}}}
	if err := CheckRubyQualification(context.Background(), bare, mini, info); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing record admitted: %v", err)
	}
}

// rubyRecordRepo commits fixture RubyQualifiedSources and returns the root,
// a passing two-runner record bound to them and a function committing a
// record and returning the build info of a selector at that commit.
func rubyRecordRepo(t *testing.T, runners ...string) (string, Qualified, func(*testing.T, Qualified) *debug.BuildInfo) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	for _, p := range RubyQualifiedSources {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("fixture "+p+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "sources")
	oids, err := OIDs(context.Background(), root, git(t, root, "rev-parse", "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	q := Qualified{Profile: "linecov-qualified/1", Family: "ruby", IndexCommit: "corpus", Runners: runners,
		Corpus:  &QualifiedCorpus{URL: "https://example.invalid/webmock", Commit: "corpus", License: "MIT", LicenseSHA256: "x"},
		Report:  QualifiedReport{Digest: "sha256:fixture", Seed: 1},
		Sources: map[string]string{}}
	for _, runner := range runners {
		q.Report.Runs = append(q.Report.Runs, QualifiedRun{Runner: runner, IndexDigest: "sha256:ix", Tests: 23, Faults: 3, Killed: 2, Narrowed: 1, Selected: []int{1, 23, 4}, Controls: 1, ControlsKept: []int{0}})
		q.Report.Mutants, q.Report.Killed, q.Report.Controls = q.Report.Mutants+3, q.Report.Killed+2, q.Report.Controls+1
	}
	for _, p := range RubyQualifiedSources {
		q.Sources[p] = oids[p]
	}
	commit := func(t *testing.T, record Qualified) *debug.BuildInfo {
		t.Helper()
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		full := filepath.Join(root, RubyQualificationRecord)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-q", "--allow-empty", "-m", "record")
		return &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: git(t, root, "rev-parse", "HEAD")}, {Key: "vcs.modified", Value: "false"}}}
	}
	return root, q, commit
}

// LPCV-V0-7608: a record whose every fault kept the whole suite proves
// nothing about narrowing, so it admits no runner.
func TestRubyQualificationRefusesWithoutNarrowing(t *testing.T) {
	const mini = "ruby 3.3.8 minitest 5.27.0 coverage=lines,methods"
	root, q, commit := rubyRecordRepo(t, mini)
	q.Report.Runs[0].Selected = []int{23, 23, 23}
	err := CheckRubyQualification(context.Background(), root, mini, commit(t, q))
	if err == nil || !strings.Contains(err.Error(), "narrow") {
		t.Fatalf("a record with no narrowed fault admitted: %v", err)
	}
}

// LPCV-V0-7608: a runner whose control kept any test is not qualified, even
// when every kept test is one the index keeps whatever the change: such an
// index yields no control (GLS-V0-032).
func TestRubyQualificationRefusesKeptControl(t *testing.T) {
	const rspec = "ruby 3.3.8 rspec 3.13.7 coverage=lines,methods"
	root, q, commit := rubyRecordRepo(t, rspec)
	q.Report.Runs[0].ControlsKept = []int{3}
	q.Report.Runs[0].KeptRegardless = 3
	q.Report.Runs[0].Unordered = 1
	err := CheckRubyQualification(context.Background(), root, rspec, commit(t, q))
	if err == nil || !strings.Contains(err.Error(), "control") {
		t.Fatalf("a runner whose control kept 3 tests admitted: %v", err)
	}
}
