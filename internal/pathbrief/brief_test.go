// SPDX-License-Identifier: Apache-2.0

package pathbrief

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const (
	c1 = "1111111111111111111111111111111111111111"
	c2 = "2222222222222222222222222222222222222222"
)

func fakeTests(results ...TestsResult) func(context.Context, string, [][]string) []TestsResult {
	return func(context.Context, string, [][]string) []TestsResult { return results }
}

// links commits a requirement-link file into the repository.
func links(t *testing.T, root string, file LinkFile) {
	t.Helper()
	file.Schema = LinksSchema
	commitAll(t, root, map[string]string{LinksPath: string(mustJSON(t, file))})
}

// TBC-V1-010: three sections in fixed order; instructions come from the directory ancestry.
func TestBuildStructureAndInstructions(t *testing.T) {
	root := newRepo(t, map[string]string{"AGENTS.md": "x", "CLAUDE.md": "x", "a/AGENTS.md": "x", "a/b/x.go": "x", "z/y.go": "x"})
	brief := build(t, root, Sources{}, "a/b/x.go", "z/y.go")
	if brief.Profile != Profile || brief.Blocking || len(brief.Paths) != 2 {
		t.Fatalf("brief %+v", brief)
	}
	s := pick(t, brief, 0, "instructions")
	equal(t, "instructions", itemPaths(s.Items), []string{"AGENTS.md", "CLAUDE.md", "a/AGENTS.md"})
	equal(t, "state", s.State, StateObserved)
	equal(t, "other branch", itemPaths(pick(t, brief, 1, "instructions").Items), []string{"AGENTS.md", "CLAUDE.md"})
}

// TBC-V1-011: only an unreadable Git history is an error.
func TestBuildNoCommitIsAnError(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init", "-q")
	if _, err := Build(context.Background(), root, []string{"a"}, Sources{}); err == nil {
		t.Fatal("no error without a commit")
	}
}

// TBC-V1-010, TBC-V1-011: the tests section states which source was read.
func TestBuildTests(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x"})
	plan := []PlannedTest{{Test: "T1", Reason: "r"}}
	cases := []struct {
		name     string
		sources  Sources
		state    string
		reason   string
		items    int
		missing  int
		absentOK bool
	}{
		{"no source", Sources{}, StateNotObserved, "source-absent", 0, 2, false},
		{"both missing", Sources{Tests: fakeTests(TestsResult{PlanMissing: "p", EvidenceMissing: "e"})}, StateNotObserved, "source-absent", 0, 2, false},
		{"plan only", Sources{Tests: fakeTests(TestsResult{EvidenceMissing: "e", Planned: plan})}, StateIncomplete, "one-source-read", 1, 1, false},
		{"evidence only no items", Sources{Tests: fakeTests(TestsResult{PlanMissing: "p"})}, StateNotObserved, "one-source-read", 0, 1, false},
		{"scope unknown", Sources{Tests: fakeTests(TestsResult{Scope: "UNKNOWN", Planned: plan})}, StateIncomplete, "plan-scope-unknown", 1, 0, false},
		{"both read", Sources{Tests: fakeTests(TestsResult{Scope: "BOUNDED", Planned: plan})}, StateObserved, "plan-and-evidence-read", 1, 0, false},
	}
	for _, c := range cases {
		s := pick(t, build(t, root, c.sources, "a/x.go"), 0, "tests")
		if s.State != c.state || !strings.HasPrefix(s.Reason, c.reason) || len(s.Items) != c.items || len(s.MissingSources) != c.missing {
			t.Errorf("%s: %+v", c.name, s)
		}
	}
	// Items that differ only in Component or Reason both stay, ordered by the full key; true
	// duplicates collapse.
	result := TestsResult{
		Planned:   []PlannedTest{{"T", "b"}, {"T", "a"}, {"T", "a"}},
		Evidenced: []EvidencedTest{{"T", "y", c1}, {"T", "x", c1}, {"T", "x", c1}},
	}
	s := pick(t, build(t, root, Sources{Tests: fakeTests(result)}, "a/x.go"), 0, "tests")
	var got []string
	for _, item := range s.Items {
		got = append(got, fmt.Sprintf("%s/%s/%s", item.Link, item.Component, item.Reason))
	}
	equal(t, "order", got, []string{"affected-plan//a", "affected-plan//b", "test-evidence/x/", "test-evidence/y/"})
}

// TBC-V1-010: a section lists at most 64 items and reports the total.
func TestBuildTestsLimit(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x"})
	var planned []PlannedTest
	for i := 299; i >= 0; i-- {
		planned = append(planned, PlannedTest{Test: fmt.Sprintf("T%03d", i)})
	}
	s := pick(t, build(t, root, Sources{Tests: fakeTests(TestsResult{Planned: planned})}, "a/x.go"), 0, "tests")
	if len(s.Items) != SectionLimit || s.Total != 300 || !s.Truncated || s.Items[0].Test != "T000" || s.Items[63].Test != "T063" {
		t.Errorf("limit: %d items, total %d, truncated %v", len(s.Items), s.Total, s.Truncated)
	}
}

// TBC-V1-010, TBC-V1-011: requirements is never OBSERVED and names every missing source.
func TestBuildRequirementsStates(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x"})
	s := pick(t, build(t, root, Sources{}, "a/x.go"), 0, "requirements")
	equal(t, "no file", s.State+" "+s.Reason[:len("no-link-for-path")], "NOT_OBSERVED no-link-for-path")
	equal(t, "missing", s.MissingSources, []string{LinksPath})

	links(t, root, LinkFile{Commits: []LinkCommit{{SHA: c1, CommitterTime: 5}},
		Evidence: []LinkEvidence{{File: "s.md", End: 9, BlobOid: c1, Requirements: []string{"R-001"}}},
		Paths:    []LinkPath{{Path: "a/x.go", Records: []LinkRecord{{Evidence: []int{0}}}}}})
	s = pick(t, build(t, root, Sources{}, "a/x.go"), 0, "requirements")
	equal(t, "with link", s.State+" "+s.Reason, "INCOMPLETE links-not-exhaustive")
	equal(t, "with link, nothing missing", len(s.MissingSources), 0)
	s = pick(t, build(t, root, Sources{}, "a"), 0, "requirements")
	equal(t, "directory", len(s.Items), 1)
	s = pick(t, build(t, root, Sources{}, "AGENTS.md"), 0, "requirements")
	equal(t, "path without link", s.State, StateNotObserved)
}

// TBC-V1-012: one item per requirement; the reported entry has the latest committer time, a tie
// goes to the lower SHA; changedSince compares blobs.
func TestBuildRequirementsReportedEntry(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x"})
	blob := blobAt(t, root, "HEAD:a/x.go")
	other := strings.Repeat("a", 40)
	ev := []LinkEvidence{
		{File: "s.md", Start: 0, End: 9, BlobOid: c1, Requirements: []string{"R-001", "R-002"}},
		{File: "s.md", Start: 10, End: 20, BlobOid: c1, Requirements: []string{"R-001"}},
		{File: "s.md", Start: 30, End: 40, BlobOid: c1},
	}
	cases := []struct {
		name    string
		commits []LinkCommit
		records []LinkRecord
		commit  string
		changed bool
		entries int
	}{
		{"later time wins", []LinkCommit{{c1, 5}, {c2, 9}}, []LinkRecord{{Commit: 0, Blob: blob, Evidence: []int{0, 1}}, {Commit: 1, Blob: other, Evidence: []int{0}}}, c2, true, 3},
		{"tie goes to lower sha", []LinkCommit{{c2, 5}, {c1, 5}}, []LinkRecord{{Commit: 0, Blob: other, Evidence: []int{0}}, {Commit: 1, Blob: blob, Evidence: []int{0}}}, c1, false, 2},
	}
	for _, c := range cases {
		links(t, root, LinkFile{Commits: c.commits, Evidence: ev, Paths: []LinkPath{{Path: "a/x.go", Records: c.records}}})
		s := pick(t, build(t, root, Sources{}, "a/x.go"), 0, "requirements")
		var r1 Item
		for _, item := range s.Items {
			if item.Requirement == "R-001" {
				r1 = item
			}
		}
		if r1.Commit != c.commit || r1.ChangedSince == nil || *r1.ChangedSince != c.changed || r1.Entries != c.entries {
			t.Errorf("%s: %+v", c.name, r1)
		}
	}
	// A span that defines no requirement is its own item, listed after the requirement items.
	links(t, root, LinkFile{Commits: []LinkCommit{{c1, 1}}, Evidence: ev,
		Paths: []LinkPath{{Path: "a/x.go", Records: []LinkRecord{{Blob: blob, Evidence: []int{2, 1}}}}}})
	s := pick(t, build(t, root, Sources{}, "a/x.go"), 0, "requirements")
	if len(s.Items) != 2 || s.Items[0].Requirement != "R-001" || s.Items[1].Requirement != "" || s.Items[1].Evidence.Start != 30 {
		t.Errorf("items %+v", s.Items)
	}
	// A path moved on is not matched under its new name.
	commitAll(t, root, map[string]string{"a/y.go": "x"})
	equal(t, "new name", pick(t, build(t, root, Sources{}, "a/y.go"), 0, "requirements").State, StateNotObserved)
}

// TBC-V1-012: a damaged link file is a missing source, never a partial answer.
func TestBuildRequirementsDamagedLinkFile(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x", LinksPath: "not json"})
	s := pick(t, build(t, root, Sources{}, "a/x.go"), 0, "requirements")
	equal(t, "state", s.State, StateNotObserved)
	equal(t, "missing", s.MissingSources[0], LinksPath)
}

// TBC-V1-009: a path absent from HEAD still gets a brief.
func TestBuildAbsentPath(t *testing.T) {
	root := newRepo(t, map[string]string{"AGENTS.md": "x", "a/x.go": "x"})
	brief := build(t, root, Sources{Tests: fakeTests()}, "a/gone.go")
	p := brief.Paths[0]
	equal(t, "kind", p.Kind, "absent")
	for _, name := range []string{"tests", "requirements"} {
		s := pick(t, brief, 0, name)
		equal(t, name, s.State+" "+s.Reason, "NOT_OBSERVED path-not-at-head")
	}
	equal(t, "instructions", itemPaths(pick(t, brief, 0, "instructions").Items), []string{"AGENTS.md"})
}

// TBC-V1-011: nothing but Git is needed.
func TestBuildNeverUsedCorvint(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x"})
	brief := build(t, root, Sources{}, "a/x.go")
	equal(t, "tests", pick(t, brief, 0, "tests").State, StateNotObserved)
	equal(t, "requirements", pick(t, brief, 0, "requirements").State, StateNotObserved)
}

// TBC-V1-010: CODEOWNERS, a flows file and a Tasks store add no section; the profile is /1.
func TestBuildCutSectionsAreAbsent(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x", ".github/CODEOWNERS": "* @all\n", ".corvint/flows.json": "{}",
		".taskman/queue.json": "{}"}) //corvint:fixture-path
	brief := build(t, root, Sources{}, "a/x.go")
	equal(t, "profile", brief.Profile, "corvint-path-brief/1")
	var names []string
	for _, s := range brief.Paths[0].Sections {
		names = append(names, s.Name)
	}
	equal(t, "sections", names, []string{"instructions", "tests", "requirements"})
}

// TBC-V1-009: building the brief writes nothing to the repository.
func TestBuildLeavesTheRepositoryAlone(t *testing.T) {
	root := newRepo(t, map[string]string{"a/x.go": "x"})
	before := git(t, root, "status", "--porcelain=v1", "--ignored")
	build(t, root, Sources{}, "a/x.go", "a", ".")
	equal(t, "status", git(t, root, "status", "--porcelain=v1", "--ignored"), before)
}

// TBC-V1-009: NormalizePath refuses what lies outside the repository and accepts an absolute path
// reached through a symlinked prefix of the root.
func TestNormalizePath(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	cases := []struct {
		in   string
		want string
		fail bool
	}{
		{"a/b.go", "a/b.go", false},
		{"./a//b/../c", "a/c", false},
		{".", "", false},
		{"a/..", "", false},
		{filepath.Join(root, "a", "b.go"), "a/b.go", false},
		{root, "", false},
		{"", "", true},
		{"..", "", true},
		{"../x", "", true},
		{"a/../../x", "", true},
		{"a\nb", "", true},
		{outside, "", true},
		{filepath.Join(root, "..", "x"), "", true},
	}
	for _, c := range cases {
		got, err := NormalizePath(root, c.in)
		if c.fail != (err != nil) || (c.fail && !errors.Is(err, ErrOutsideRepository)) || got != c.want {
			t.Errorf("NormalizePath(%q) = %q, %v", c.in, got, err)
		}
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks unavailable")
		}
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, in := range []string{filepath.Join(link, "a", "b.go"), filepath.Join(link, "a"), filepath.Join(link, "not", "yet", "b.go")} {
		got, err := NormalizePath(root, in)
		want := strings.TrimPrefix(in, link+string(filepath.Separator))
		if err != nil || got != filepath.ToSlash(want) {
			t.Errorf("through link %q = %q, %v", in, got, err)
		}
	}
	if got, err := NormalizePath(link, filepath.Join(root, "a", "b.go")); err != nil || got != "a/b.go" {
		t.Errorf("root through link = %q, %v", got, err)
	}
	// A link that leaves the repository is still outside.
	escape := filepath.Join(root, "escape")
	if err := os.Symlink(outside, escape); err == nil {
		if got, err := NormalizePath(root, filepath.Join(link, "escape", "x")); err == nil {
			t.Errorf("link out of the repository accepted: %q", got)
		}
	}
}
