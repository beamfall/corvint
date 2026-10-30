package sharedpath

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fixture is a throwaway repository with a fixed identity and clock.
type fixture struct {
	t    *testing.T
	root string
	tick int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, root: root}
	f.git("init", "-q", "-b", "main")
	return f
}

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Ada Author", "GIT_AUTHOR_EMAIL=ada@example.invalid",
		"GIT_COMMITTER_NAME=Cy Committer", "GIT_COMMITTER_EMAIL=cy@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05+0130", "GIT_COMMITTER_DATE=2026-02-03T04:05:06-0700")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *fixture) write(name, content string) {
	f.t.Helper()
	p := filepath.Join(f.root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// commit stages everything and commits with a distinct, fixed date.
func (f *fixture) commit(msg string, files map[string]string) string {
	f.t.Helper()
	for name, content := range files {
		if content == "" {
			f.git("rm", "-q", name)
			continue
		}
		f.write(name, content)
	}
	f.git("add", "-A")
	f.tick++
	f.git("-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.useConfigOnly=false", "commit", "-q", "--allow-empty", "-m", msg, "--date", "2026-01-0"+string(rune('0'+f.tick))+"T10:00:00+0200")
	return f.git("rev-parse", "HEAD")
}

func testConfig() Config {
	return Config{Profile: ConfigProfile, Shared: []string{"internal/testutil/**", "script/*.sh"}, Generated: []string{"docs/specs/REQUIREMENTS.tsv"},
		Marker: "shared-path:", RunTrailer: "Affected-Run", SessionMarkers: []string{".corvint/agent-session.lock"}}
}

func TestSPCV0001ConfigIsClosedAndClassifiesPaths(t *testing.T) {
	dir := t.TempDir()
	load := func(body string) error {
		p := filepath.Join(dir, "c.json")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadConfig(p)
		return err
	}
	good := `{"profile":"corvint-shared-paths/0","shared":["internal/testutil/**"],"generated":["docs/specs/REQUIREMENTS.tsv"],"marker":"shared-path:"}`
	if err := load(good); err != nil {
		t.Fatalf("good config: %v", err)
	}
	for _, bad := range []string{
		`{"profile":"corvint-shared-paths/0","shared":["a/**"],"marker":"m","extra":1}`,
		`{"profile":"corvint-shared-paths/1","shared":["a/**"],"marker":"m"}`,
		`{"profile":"corvint-shared-paths/0","shared":["../a"],"marker":"m"}`,
		`{"profile":"corvint-shared-paths/0","shared":["/abs"],"marker":"m"}`,
		`{"profile":"corvint-shared-paths/0","shared":[],"marker":"m"}`,
		`{"profile":"corvint-shared-paths/0","shared":["a/["],"marker":"m"}`,
		`{"profile":"corvint-shared-paths/0","shared":["a"],"marker":" "}`,
		`{"profile":"corvint-shared-paths/0","shared":["a"],"marker":"m","sessionMarkers":["../x"]}`,
		`{"profile":"corvint-shared-paths/0","shared":["a"],"marker":"m"} {}`,
	} {
		if err := load(bad); err == nil || err.Error() != CodeConfigUnavailable {
			t.Fatalf("%s: %v", bad, err)
		}
	}
	if _, err := LoadConfig(filepath.Join(dir, "absent.json")); err == nil || err.Error() != CodeConfigUnavailable {
		t.Fatalf("absent: %v", err)
	}
	c := testConfig()
	c.Generated = append(c.Generated, "internal/testutil/gen/**")
	for name, want := range map[string]string{
		"internal/testutil/a.go":         "shared",
		"internal/testutil/deep/x/b.go":  "shared",
		"internal/testutil/gen/z.go":     "generated",
		"script/run.sh":                  "shared",
		"script/sub/run.sh":              "other",
		"docs/specs/REQUIREMENTS.tsv":    "generated",
		"internal/sharedpath/check.go":   "other",
		"internal/testutilities/main.go": "other",
	} {
		if got := c.Class(name); got != want {
			t.Fatalf("Class(%s) = %s, want %s", name, got, want)
		}
	}
}

func TestSPCV0002CheckFindingsNameCommitPathAndMissingElement(t *testing.T) {
	f := newFixture(t)
	base := f.commit("base", map[string]string{"README": "r\n", "internal/testutil/a.go": "a\n"})
	unmarked := f.commit("feat: mixed change", map[string]string{"internal/testutil/a.go": "a2\n", "cmd/x.go": "x\n"})
	marked := f.commit("shared-path: harden helper\n\nAffected-Run: PASS run-42\n", map[string]string{"internal/testutil/a.go": "a3\n"})
	mixed := f.commit("shared-path: helper and feature\n\nAffected-Run: FAIL run-43\n", map[string]string{"script/t.sh": "t\n", "cmd/y.go": "y\n"})
	f.commit("feat: other only", map[string]string{"cmd/z.go": "z\n"})
	head := f.git("rev-parse", "HEAD")
	notes := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(notes, []byte("Shared-path commits: "+marked[:9]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := f.git("for-each-ref")
	report, err := Check(context.Background(), CheckRequest{Root: f.root, Base: base, NotesFile: notes, Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if report.Profile != CheckProfile || report.Mutates || report.OK || report.Base != base || report.Head != head || report.Commits != 4 || len(report.Shared) != 3 {
		t.Fatalf("report: %+v", report)
	}
	want := []Finding{
		{Code: "shared-path-unmarked", Commit: unmarked, Path: "internal/testutil/a.go", Missing: "marker"},
		{Code: "shared-path-unlisted", Commit: unmarked, Path: "internal/testutil/a.go", Missing: "listing"},
		{Code: "shared-path-uncited", Commit: unmarked, Path: "internal/testutil/a.go", Missing: "run-citation"},
		{Code: "shared-path-unlisted", Commit: mixed, Path: "script/t.sh", Missing: "listing"},
		{Code: "shared-path-uncited", Commit: mixed, Path: "script/t.sh", Missing: "run-citation"},
		{Code: "shared-path-mixed", Commit: mixed, Path: "cmd/y.go", Missing: "separate-commit"},
	}
	if len(report.Findings) != len(want) {
		t.Fatalf("findings: %+v", report.Findings)
	}
	for i, w := range want {
		got := report.Findings[i]
		got.Detail = ""
		if got != w {
			t.Fatalf("finding %d = %+v, want %+v", i, got, w)
		}
	}
	if report.Shared[1].Commit != marked || !report.Shared[1].Marker || report.Shared[1].Listed != "YES" || report.Shared[1].RunCitation != "PASS run-42" {
		t.Fatalf("marked commit: %+v", report.Shared[1])
	}
	if after := f.git("for-each-ref"); after != before || f.git("status", "--porcelain") != "" {
		t.Fatal("check-shared mutated refs or the worktree")
	}
	// Without notes, the listing is NOT_OBSERVED rather than a finding.
	report, err = Check(context.Background(), CheckRequest{Root: f.root, Base: base, Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report.NotObserved, []string{"notes-listing"}) || report.Shared[0].Listed != "NOT_OBSERVED" {
		t.Fatalf("no notes: %+v", report)
	}
	for _, finding := range report.Findings {
		if finding.Missing == "listing" {
			t.Fatalf("listing finding without notes: %+v", finding)
		}
	}
	if _, err := Check(context.Background(), CheckRequest{Root: f.root, Base: "no-such-rev", Config: testConfig()}); err == nil || err.Error() != CodeRangeUnavailable {
		t.Fatalf("missing base: %v", err)
	}
	if _, err := Check(context.Background(), CheckRequest{Root: f.root, Base: base, NotesFile: filepath.Join(t.TempDir(), "absent"), Config: testConfig()}); err == nil || err.Error() != CodeNotesUnavailable {
		t.Fatalf("missing notes: %v", err)
	}
}

func TestSPCV0003GeneratedConflictsAreExempt(t *testing.T) {
	f := newFixture(t)
	f.commit("base", map[string]string{"docs/specs/REQUIREMENTS.tsv": "a\n", "cmd/x.go": "x\n"})
	f.git("checkout", "-q", "-b", "feature")
	f.commit("feat: regenerate", map[string]string{"docs/specs/REQUIREMENTS.tsv": "feature\n"})
	f.git("checkout", "-q", "main")
	f.commit("main: regenerate", map[string]string{"docs/specs/REQUIREMENTS.tsv": "main\n"})
	report, err := Check(context.Background(), CheckRequest{Root: f.root, Base: "main", Head: "feature", Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || len(report.Findings) != 0 || !reflect.DeepEqual(report.ExemptConflicts, []string{"docs/specs/REQUIREMENTS.tsv"}) {
		t.Fatalf("generated-only conflict: %+v", report)
	}
	f.commit("main: edit x", map[string]string{"cmd/x.go": "main x\n"})
	f.git("checkout", "-q", "feature")
	f.commit("feat: edit x", map[string]string{"cmd/x.go": "feature x\n"})
	report, err = Check(context.Background(), CheckRequest{Root: f.root, Base: "main", Head: "feature", Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || len(report.Findings) != 1 || report.Findings[0].Code != "shared-path-merge-conflict" || report.Findings[0].Path != "cmd/x.go" || report.Findings[0].Missing != "clean-merge" {
		t.Fatalf("real conflict: %+v", report)
	}
}

// splitFixture has an unchanged prefix commit, a mixed commit with a shared
// deletion and addition, a shared-only commit and a trailing other commit.
func splitFixture(t *testing.T) (*fixture, string, string) {
	f := newFixture(t)
	base := f.commit("base", map[string]string{"README": "r\n", "internal/testutil/old.go": "old\n"})
	f.git("checkout", "-q", "-b", "work")
	f.commit("feat: prefix", map[string]string{"cmd/a.go": "a\n"})
	f.commit("feat: mixed\n\nBody line.\n\nAffected-Run: PASS run-7\n", map[string]string{"internal/testutil/old.go": "", "internal/testutil/new.go": "new\n", "cmd/b.go": "b\n"})
	f.commit("shared-path: already split\n", map[string]string{"script/t.sh": "t\n"})
	f.commit("feat: tail", map[string]string{"cmd/c.go": "c\n"})
	return f, base, f.git("rev-parse", "HEAD")
}

func TestSPCV0004SplitKeepsTreeIdentityAndMessage(t *testing.T) {
	f, base, head := splitFixture(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "clone", "-q", "--bare", f.root, bare).CombinedOutput(); err != nil {
		t.Fatalf("bare: %v %s", err, out)
	}
	f.git("remote", "add", "origin", bare)
	remoteBefore := gitIn(t, bare, "for-each-ref")
	prefix := f.git("rev-parse", "HEAD~3")
	mixed := f.git("rev-parse", "HEAD~2")
	ctx := context.Background()
	plan, err := SplitShared(ctx, SplitRequest{Root: f.root, Base: base, Branch: "work", Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Applied || plan.Mutates || plan.OldHead != head || len(plan.Splits) != 1 || plan.Splits[0].Commit != mixed ||
		!reflect.DeepEqual(plan.Splits[0].SharedPaths, []string{"internal/testutil/new.go", "internal/testutil/old.go"}) || f.git("rev-parse", "work") != head {
		t.Fatalf("plan: %+v", plan)
	}
	r, err := SplitShared(ctx, SplitRequest{Root: f.root, Base: base, Branch: "work", Apply: true, ExpectHead: head, Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	newHead := f.git("rev-parse", "work")
	if !r.Applied || !r.Mutates || r.Pushed || r.OldHead != head || r.NewHead != newHead || newHead == head || r.Rewritten != 4 {
		t.Fatalf("applied: %+v", r)
	}
	if f.git("rev-parse", newHead+"^{tree}") != f.git("rev-parse", head+"^{tree}") || r.FinalTree != f.git("rev-parse", head+"^{tree}") {
		t.Fatal("final tree changed")
	}
	if f.git("rev-list", "--count", "^"+base, newHead) != "5" || f.git("rev-parse", newHead+"~4") != prefix {
		t.Fatal("unexpected rewritten history shape or the unchanged prefix was rewritten")
	}
	shared, rest := r.Splits[0].SharedCommit, r.Splits[0].RestCommit
	if f.git("rev-parse", rest+"^") != shared || f.git("rev-parse", shared+"^") != prefix {
		t.Fatal("split commits are not chained on the prefix")
	}
	if got := f.git("diff-tree", "-r", "--no-commit-id", "--name-only", shared); got != "internal/testutil/new.go\ninternal/testutil/old.go" {
		t.Fatalf("shared commit paths: %q", got)
	}
	if got := f.git("diff-tree", "-r", "--no-commit-id", "--name-only", rest); got != "cmd/b.go" {
		t.Fatalf("rest commit paths: %q", got)
	}
	identity := "%an <%ae> %ad|%cn <%ce> %cd"
	for _, pair := range [][2]string{{mixed, rest}, {mixed, shared}, {head, newHead}} {
		if a, b := f.git("log", "-1", "--format="+identity, "--date=raw", pair[0]), f.git("log", "-1", "--format="+identity, "--date=raw", pair[1]); a != b {
			t.Fatalf("identity changed: %s vs %s", a, b)
		}
	}
	if f.git("cat-file", "-p", rest+"^{commit}") == "" || f.git("log", "-1", "--format=%B", rest) != f.git("log", "-1", "--format=%B", mixed) {
		t.Fatal("rest commit message changed")
	}
	if got := f.git("log", "-1", "--format=%s", shared); got != "shared-path: feat: mixed" {
		t.Fatalf("shared subject: %q", got)
	}
	if gitIn(t, bare, "for-each-ref") != remoteBefore {
		t.Fatal("split-shared changed the remote")
	}
	if f.git("status", "--porcelain") != "" {
		t.Fatal("split-shared left the worktree dirty")
	}
	// A second run finds nothing left to split.
	again, err := SplitShared(ctx, SplitRequest{Root: f.root, Base: base, Branch: "work", Config: testConfig()})
	if err != nil || len(again.Splits) != 0 || len(again.ManualFollowUps) != 0 {
		t.Fatalf("second plan: %+v %v", again, err)
	}
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

func TestSPCV0005SplitRefusesUnsafeStates(t *testing.T) {
	ctx := context.Background()
	refused := func(t *testing.T, f *fixture, request SplitRequest, code string) {
		t.Helper()
		before := f.git("for-each-ref")
		request.Root, request.Config = f.root, testConfig()
		if _, err := SplitShared(ctx, request); err == nil || err.Error() != code {
			t.Fatalf("want %s, got %v", code, err)
		}
		if f.git("for-each-ref") != before {
			t.Fatal("refusal changed a ref")
		}
	}
	t.Run("dirty tracked", func(t *testing.T) {
		f, base, head := splitFixture(t)
		f.write("cmd/a.go", "edited\n")
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: head}, CodeDirtyTree)
	})
	t.Run("dirty untracked", func(t *testing.T) {
		f, base, head := splitFixture(t)
		f.write("scratch.txt", "x\n")
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: head}, CodeDirtyTree)
	})
	t.Run("detached head", func(t *testing.T) {
		f, base, head := splitFixture(t)
		f.git("checkout", "-q", "--detach", head)
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: head}, CodeDetachedHead)
	})
	t.Run("branch not checked out", func(t *testing.T) {
		f, base, head := splitFixture(t)
		f.git("checkout", "-q", "main")
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: head}, CodeBranchMismatch)
	})
	t.Run("merge in range", func(t *testing.T) {
		f, base, _ := splitFixture(t)
		f.git("checkout", "-q", "-b", "side", base)
		f.commit("side", map[string]string{"side.txt": "s\n"})
		f.git("checkout", "-q", "work")
		f.git("merge", "-q", "--no-edit", "--no-ff", "side")
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: f.git("rev-parse", "HEAD")}, CodeMergeInRange)
	})
	t.Run("live session marker", func(t *testing.T) {
		f, base, head := splitFixture(t)
		f.git("config", "core.excludesFile", os.DevNull)
		f.write(".git/info/exclude", ".corvint/\n")
		f.write(".corvint/agent-session.lock", "pid\n")
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: head}, CodeLiveSession)
	})
	t.Run("git operation in progress", func(t *testing.T) {
		f, base, head := splitFixture(t)
		f.write(".git/index.lock", "")
		refused(t, f, SplitRequest{Base: base, Branch: "work", Apply: true, ExpectHead: head}, CodeLiveSession)
	})
	t.Run("invalid branch", func(t *testing.T) {
		f, base, head := splitFixture(t)
		refused(t, f, SplitRequest{Base: base, Branch: "-x", Apply: true, ExpectHead: head}, CodeInvalidArguments)
	})
}

func TestSPCV0006ManualFollowUpsAreOnlyListingAndCitation(t *testing.T) {
	f, base, head := splitFixture(t)
	r, err := SplitShared(context.Background(), SplitRequest{Root: f.root, Base: base, Branch: "work", Apply: true, ExpectHead: head, Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.ManualFollowUps, []string{"notes-listing", "run-citation"}) {
		t.Fatalf("manual follow-ups: %v", r.ManualFollowUps)
	}
	// After the split the policy findings left are exactly those two elements.
	report, err := Check(context.Background(), CheckRequest{Root: f.root, Base: base, Head: "work", NotesFile: filepath.Join(f.root, "README"), Config: testConfig()})
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Missing != "listing" && finding.Missing != "run-citation" {
			t.Fatalf("finding beyond the manual follow-ups: %+v", finding)
		}
	}
}

func TestSPCV0007HeadMismatchAbortsWithoutChanges(t *testing.T) {
	f, base, head := splitFixture(t)
	before := f.git("for-each-ref")
	objects := f.git("count-objects", "-v")
	_, err := SplitShared(context.Background(), SplitRequest{Root: f.root, Base: base, Branch: "work", Apply: true, ExpectHead: base, Config: testConfig()})
	if err == nil || err.Error() != CodeHeadMismatch {
		t.Fatalf("want head mismatch, got %v", err)
	}
	if f.git("for-each-ref") != before || f.git("count-objects", "-v") != objects || f.git("rev-parse", "work") != head {
		t.Fatal("head mismatch changed repository state")
	}
	// The compare-and-swap itself refuses when the branch moves under it.
	g := git{ctx: context.Background(), root: f.root}
	if _, code, err := g.run(nil, nil, "update-ref", "refs/heads/work", base, base); err != nil || code == 0 {
		t.Fatal("update-ref accepted a stale old value")
	}
}

// objectFiles lists every file under the repository's object store.
func (f *fixture) objectFiles() []string {
	f.t.Helper()
	var files []string
	store := filepath.Join(f.root, ".git", "objects")
	err := filepath.WalkDir(store, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return files
}

// Codex finding 10 on #1059: the merge probe of divergent branches writes no
// object into the repository, for a clean merge and for a conflict alike.
func TestSPCV0003MergeProbeWritesNoRepositoryObject(t *testing.T) {
	f := newFixture(t)
	f.commit("base", map[string]string{"cmd/x.go": "x\n", "cmd/y.go": "y\n"})
	f.git("checkout", "-q", "-b", "feature")
	f.commit("feat: edit y", map[string]string{"cmd/y.go": "feature y\n"})
	f.git("checkout", "-q", "main")
	f.commit("main: edit x", map[string]string{"cmd/x.go": "main x\n"})
	for _, conflict := range []bool{false, true} {
		if conflict {
			f.commit("main: edit y", map[string]string{"cmd/y.go": "main y\n"})
		}
		before := f.objectFiles()
		report, err := Check(context.Background(), CheckRequest{Root: f.root, Base: "main", Head: "feature", Config: testConfig()})
		if err != nil {
			t.Fatal(err)
		}
		if report.Mutates || report.OK == conflict {
			t.Fatalf("conflict=%v report: %+v", conflict, report)
		}
		if after := f.objectFiles(); !reflect.DeepEqual(before, after) {
			t.Fatalf("conflict=%v: check-shared wrote objects: %d -> %d files", conflict, len(before), len(after))
		}
	}
}

// Codex finding 9 on #1059: planning never rewrites the index, even when its
// stat data is stale and `git status` could refresh it.
func TestSPCV0004PlanningLeavesIndexBytesUnchanged(t *testing.T) {
	f, base, _ := splitFixture(t)
	index := filepath.Join(f.root, ".git", "index")
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(f.root, "cmd", "a.go"), future, future); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SplitShared(context.Background(), SplitRequest{Root: f.root, Base: base, Branch: "work", Config: testConfig()}); err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(index); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("planning rewrote the index: %v", err)
	}
}
