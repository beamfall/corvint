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

// LPCV-V0-7608: the Ruby family is admitted only by a passing record at the
// selector's clean revision whose sources match it and which names the
// index runner; a stale binding, a miss, a failed control or an unnamed
// runner refuses.
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
		return QualifiedRun{Runner: runner, IndexDigest: "sha256:ix", Tests: 23, Faults: 7, Killed: 5, Selected: []int{1, 2, 3, 4, 5, 6, 7}, Controls: 1, ControlsKept: []int{0}}
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
		{"control kept regardless", func(q *Qualified) { q.Report.Runs[1].ControlsKept = []int{3}; q.Report.Runs[1].KeptRegardless = 3 }, rspec, false, root, true},
		{"missing control sizes", func(q *Qualified) { q.Report.Runs[0].ControlsKept = nil }, mini, false, root, false},
		{"go family", func(q *Qualified) { q.Family = "go" }, mini, false, root, false},
		{"no corpus", func(q *Qualified) { q.Corpus = nil }, mini, false, root, false},
		{"stale binding", func(q *Qualified) { q.Sources["internal/rubylinecov/lines.go"] = "0000" }, mini, false, root, false},
		{"dirty executable", func(*Qualified) {}, mini, true, root, false},
		{"no selector root", func(*Qualified) {}, mini, false, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := q
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
