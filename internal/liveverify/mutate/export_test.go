package mutate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// exportCount counts the runner's exported copies under the temporary
// directory, so a test can see how many copies a sequence of claims cost.
func exportCount(t *testing.T) int {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "corvint-mutate-*"))
	if err != nil {
		t.Fatal(err)
	}
	return len(matches)
}

// TestOpenServesManyClaimsFromOneCopy proves two claims on one package are
// judged on one export, each on its own merits, and that Close removes it.
func TestOpenServesManyClaimsFromOneCopy(t *testing.T) {
	requireSandbox(t)
	t.Setenv("TMPDIR", t.TempDir())
	git := gitExecutable(t)
	root, revision := newFixture(t, git, map[string]string{
		"go.mod":                     fixtureGoMod,
		"pkg/calc/calc.go":           fixtureCalc,
		"pkg/calc/calc_test.go":      fixtureCalcTest,
		"pkg/calc/calc_more_test.go": strings.Replace(fixtureEmptyTest, "TestNothing", "TestNothingMore", 1),
	})
	before := treeDigest(t, root)
	copies := exportCount(t)
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	if got := exportCount(t); got != copies+1 {
		t.Fatalf("copies after Open = %d, want %d", got, copies+1)
	}
	killing, err := exported.Judge(context.Background(), Request{ChangedPath: "pkg/calc/calc.go", TestPath: "pkg/calc/calc_test.go"})
	if err != nil {
		t.Fatalf("Judge killing: %v", err)
	}
	if killing.Verdict != Killed {
		t.Fatalf("killing verdict = %s (%s), want %s", killing.Verdict, killing.Detail, Killed)
	}
	empty, err := exported.Judge(context.Background(), Request{ChangedPath: "pkg/calc/calc.go", TestPath: "pkg/calc/calc_more_test.go", Complete: true})
	if err != nil {
		t.Fatalf("Judge empty: %v", err)
	}
	if empty.Verdict != Survived {
		t.Fatalf("empty verdict = %s (%s), want %s", empty.Verdict, empty.Detail, Survived)
	}
	if got := exportCount(t); got != copies+1 {
		t.Fatalf("copies after two claims = %d, want %d", got, copies+1)
	}
	exported.Close()
	if got := exportCount(t); got != copies {
		t.Fatalf("copies after Close = %d, want %d", got, copies)
	}
	if after := treeDigest(t, root); after != before {
		t.Errorf("root tree digest changed: %s -> %s", before, after)
	}
}

// TestJudgeRejectsAClaimAgainstAnotherRevision keeps a claim bound to the copy
// it is judged on.
func TestJudgeRejectsAClaimAgainstAnotherRevision(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, revision := standardFixture(t, git, fixtureCalcTest)
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	_, err = exported.Judge(context.Background(), Request{Revision: "0000000000000000000000000000000000000000", ChangedPath: "pkg/calc/calc.go", TestPath: "pkg/calc/calc_test.go"})
	if err == nil || !strings.Contains(err.Error(), "differs from the export") {
		t.Fatalf("err = %v, want a revision mismatch", err)
	}
}

// TestExportRunsUnderItsOwnDeadline pins that the export's git calls are
// bounded in time: an already-expired deadline fails the export before git
// starts, and the failure names the git command.
func TestExportRunsUnderItsOwnDeadline(t *testing.T) {
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	configuration := exportSettings{root: t.TempDir(), git: "git", revision: "HEAD"}
	err := exportInto(expired, configuration, t.TempDir())
	if err == nil {
		t.Fatal("exportInto succeeded under an expired deadline")
	}
	if !strings.Contains(err.Error(), "git ls-tree") {
		t.Fatalf("exportInto error = %q, want it to name git ls-tree", err)
	}
}

// TestHostModuleCacheWritesNothingUnderHome pins the fix for the bug where an
// unsandboxed "go env GOMODCACHE" wrote the go command's telemetry counters
// under the host's config directory even for this read-only query
// (docs/agent-memory bugs.md, 2026-09-12): resolving the module cache must
// follow go's own GOMODCACHE/GOPATH precedence without touching HOME.
func TestHostModuleCacheWritesNothingUnderHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOENV", "off")
	t.Setenv("GOPATH", filepath.Join(home, "gopath"))
	t.Setenv("GOMODCACHE", "")
	before := treeDigest(t, home)
	cache, err := hostModuleCache()
	if err != nil {
		t.Fatalf("hostModuleCache: %v", err)
	}
	if want := filepath.Join(home, "gopath", "pkg", "mod"); cache != want {
		t.Fatalf("hostModuleCache = %q, want %q", cache, want)
	}
	if after := treeDigest(t, home); after != before {
		t.Fatalf("HOME changed while resolving the module cache: %s -> %s", before, after)
	}
}

// TestHostGOPATHRefusesTheGoRootDefault pins that, like the go command,
// $HOME/go is not the GOPATH default when it is the root of the go that runs,
// whether that root comes from GOROOT or from the go found on PATH.
func TestHostGOPATHRefusesTheGoRootDefault(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "go")
	for _, directory := range []string{"bin", filepath.Join("pkg", "tool")} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "go"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GOPATH", "")
	cases := []struct{ name, goroot, path, want string }{
		{"GOROOT names the default", root, "", ""},
		{"go on PATH lives in the default", "", filepath.Join(root, "bin"), ""},
		{"go root elsewhere", filepath.Join(home, "sdk"), "", root},
	}
	for _, c := range cases {
		t.Setenv("GOROOT", c.goroot)
		t.Setenv("PATH", c.path)
		if got := hostGOPATH(map[string]string{}); got != c.want {
			t.Errorf("%s: hostGOPATH = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestOpenRejectsCallerMistakes pins that Open validates only the copy's
// fields and refuses to export without them.
func TestOpenRejectsCallerMistakes(t *testing.T) {
	for name, request := range map[string]Request{
		"no root":        {Git: "git", Revision: "HEAD"},
		"no git":         {Root: "/repo", Revision: "HEAD"},
		"no revision":    {Root: "/repo", Git: "git"},
		"relative cache": {Root: "/repo", Git: "git", Revision: "HEAD", CacheDir: "cache"},
	} {
		if _, err := Open(context.Background(), request); err == nil {
			t.Errorf("%s: Open accepted %+v", name, request)
		}
	}
}

// TestOpenNeverReadsTheRevisionAsAnOption pins that a Revision spelled like a
// git archive option names a revision, never an option: "--output=<file>"
// once made git archive truncate that file before refusing the missing tree.
func TestOpenNeverReadsTheRevisionAsAnOption(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, _ := standardFixture(t, git, fixtureCalcTest)
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: "--output=" + victim})
	if err == nil {
		exported.Close()
		t.Fatal("Open exported an option-shaped revision")
	}
	if data, readErr := os.ReadFile(victim); readErr != nil || string(data) != "keep" {
		t.Fatalf("victim after Open = %q (%v), want it untouched", data, readErr)
	}
}

// TestOpenExportsCommittedBytesWhateverTheAttributes pins that the copy holds
// the revision's blobs byte for byte: committed export-ignore, export-subst,
// and eol attributes once dropped the cited test and shifted the changed
// file's lines under the claimed spans, and the repository's own
// info/attributes and autocrlf setting, which no archive option neutralizes,
// did the same.
func TestOpenExportsCommittedBytesWhateverTheAttributes(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	files := map[string]string{
		".gitattributes":        "pkg/calc/calc_test.go export-ignore\npkg/calc/calc.go export-subst\n*.go text eol=crlf\n",
		"go.mod":                fixtureGoMod,
		"pkg/calc/calc.go":      "// $Format:%n%n%n$\n" + fixtureCalc,
		"pkg/calc/calc_test.go": fixtureCalcTest,
	}
	root, revision := newFixture(t, git, files)
	local := "pkg/calc/calc_test.go export-ignore\npkg/calc/calc.go eol=crlf\n"
	if err := os.MkdirAll(filepath.Join(root, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "attributes"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, git, root, "config", "core.autocrlf", "true")
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: revision})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer exported.Close()
	for _, name := range []string{"pkg/calc/calc.go", "pkg/calc/calc_test.go"} {
		data, err := os.ReadFile(filepath.Join(exported.Dir(), filepath.FromSlash(name)))
		if err != nil || string(data) != files[name] {
			t.Errorf("exported %s = %q (%v), want the committed bytes", name, data, err)
		}
	}
}

// TestOpenRefusesATreeEntryItCannotReproduce pins that the copy never silently
// differs from the revision: a gitlink is reproduced as the empty directory a
// checkout without its submodule holds, and a tracked symlink, which the copy
// does not reproduce, fails the export instead of being dropped.
func TestOpenRefusesATreeEntryItCannotReproduce(t *testing.T) {
	requireSandbox(t)
	git := gitExecutable(t)
	root, _ := standardFixture(t, git, fixtureCalcTest)
	commit := func() string {
		runGit(t, git, root, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "entry")
		return strings.TrimSpace(runGit(t, git, root, "rev-parse", "HEAD"))
	}
	runGit(t, git, root, "update-index", "--add", "--cacheinfo", "160000,4b825dc642cb6eb9a060e54bf8d69288fbee4904,pkg/module")
	gitlinked := commit()
	exported, err := Open(context.Background(), Request{Root: root, Git: git, Revision: gitlinked})
	if err != nil {
		t.Fatalf("Open with a gitlink: %v", err)
	}
	info, err := os.Stat(filepath.Join(exported.Dir(), "pkg", "module"))
	exported.Close()
	if err != nil || !info.IsDir() {
		t.Fatalf("gitlink in the copy = %v (%v), want an empty directory", info, err)
	}
	if err := os.Symlink("calc.go", filepath.Join(root, "pkg", "calc", "link.go")); err != nil {
		t.Fatal(err)
	}
	runGit(t, git, root, "add", "pkg/calc/link.go")
	linked := commit()
	exported, err = Open(context.Background(), Request{Root: root, Git: git, Revision: linked})
	if err == nil {
		exported.Close()
		t.Fatal("Open exported a revision holding a symlink the copy drops")
	}
	if !strings.Contains(err.Error(), "pkg/calc/link.go") {
		t.Fatalf("Open error = %q, want it to name the symlink", err)
	}
	runGit(t, git, root, "rm", "--quiet", "--cached", "pkg/calc/link.go")
	blob := strings.TrimSpace(runGit(t, git, root, "rev-parse", gitlinked+":pkg/calc/calc.go"))
	runGit(t, git, root, "update-index", "--add", "--cacheinfo", "100644,"+blob+",pkg/calc/CALC.go")
	folded := commit()
	if _, err := os.Stat(filepath.Join(root, "pkg", "calc", "CALC.go")); err != nil {
		return // a case-sensitive volume holds both names
	}
	exported, err = Open(context.Background(), Request{Root: root, Git: git, Revision: folded})
	if err == nil {
		exported.Close()
		t.Fatal("Open merged two paths a case-insensitive volume folds into one file")
	}
}
