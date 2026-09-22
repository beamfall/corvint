package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	dashboardauthority "github.com/Beamfall/corvint/internal/dashboard/authority"
)

func TestAuthorityQualifiesRealRepositoryAndTerminalBlob(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	base := stableTestDirectory(t, "authority-")
	root := filepath.Join(base, "caf\u00e9")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	runTestGit(t, git, root, "config", "user.name", "Corvint Test")
	runTestGit(t, git, root, "config", "user.email", "corvint@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "tracked.go"), []byte("package tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "add", "tracked.go")
	runTestGit(t, git, root, "commit", "-q", "-m", "initial")
	budget := NewBudget(context.Background())
	defer budget.Close()
	for attempt := 0; attempt < 2; attempt++ {
		authority, snapshot, failure := NewAttempt(root, budget)
		if failure != nil {
			if failure.Code == FailureRepositoryChanged && attempt == 0 {
				continue
			}
			t.Fatalf("New failure = %#v", failure)
		}
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		interrupted := authority.QualifyTrace(cancelled, snapshot.HeadRevision, []string{"tracked.go"})
		qualification := authority.QualifyTrace(context.Background(), snapshot.HeadRevision, []string{"tracked.go"})
		finish := authority.Finish(context.Background())
		if finish.Code == dashboardauthority.FinishChanged && attempt == 0 {
			continue
		}
		if snapshot.HeadRevision == "" || snapshot.TreeRevision == "" || snapshot.WorktreeState != dashboardauthority.WorktreeClean {
			t.Fatalf("snapshot = %#v", snapshot)
		}
		if interrupted.Code != dashboardauthority.TraceInterrupted {
			t.Fatalf("cancelled qualification = %#v", interrupted)
		}
		if qualification.Code != dashboardauthority.TraceQualified || qualification.TreeRevision != snapshot.TreeRevision ||
			len(qualification.PathWitnesses) != 1 || qualification.PathWitnesses[0].Path != "tracked.go" {
			t.Fatalf("qualification = %#v", qualification)
		}
		if finish.Code != dashboardauthority.FinishStable {
			t.Fatalf("finish = %#v", finish)
		}
		return
	}
	t.Fatal("repository changed during both whole-lifecycle attempts")
}

// TestQualifyTraceIgnoresRepositoryGrafts checks that an `info/grafts` entry
// cannot splice an unrelated commit into HEAD's ancestry: graft files are not
// replace objects, so GIT_NO_REPLACE_OBJECTS alone does not disable them.
func TestQualifyTraceIgnoresRepositoryGrafts(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	root := filepath.Join(stableTestDirectory(t, "grafts-"), "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "add", "tracked")
	runTestGit(t, git, root, "-c", "user.name=Corvint", "-c", "user.email=corvint@example.invalid", "commit", "-q", "-m", "initial")
	head := strings.TrimSpace(testGitOutput(t, git, root, "rev-parse", "HEAD"))
	unrelated := strings.TrimSpace(testGitOutput(t, git, root, "-c", "user.name=Corvint", "-c", "user.email=corvint@example.invalid",
		"commit-tree", "-m", "unrelated", "HEAD^{tree}"))
	if err := os.WriteFile(filepath.Join(root, ".git", "info", "grafts"), []byte(head+" "+unrelated+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	authority, _, failure := New(context.Background(), root)
	if failure != nil {
		t.Fatalf("New failure = %#v", failure)
	}
	defer authority.Close()
	if qualification := authority.QualifyTrace(context.Background(), unrelated, []string{"tracked"}); qualification.Code != dashboardauthority.TraceAncestryBound {
		t.Fatalf("grafted unrelated commit qualification = %#v", qualification)
	}
}

func TestAuthorityCapsMalformedGitTranscriptAllocation(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	cat, err := exec.LookPath("cat")
	if err != nil {
		t.Skip("cat unavailable")
	}
	base := stableTestDirectory(t, "git-output-bound-")
	root := filepath.Join(base, "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "add", "tracked")
	runTestGit(t, git, root, "-c", "user.name=Corvint", "-c", "user.email=corvint@example.invalid", "commit", "-q", "-m", "initial")
	payload := filepath.Join(base, "malformed-output")
	if err := os.WriteFile(payload, []byte(strings.Repeat("\n", 1<<20)), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, intercepted := range []string{"--show-object-format", "cat-file"} {
		t.Run(intercepted, func(t *testing.T) {
			proxy := writeGitOutputProxy(t, git, cat, intercepted, payload)
			result := testing.Benchmark(func(benchmark *testing.B) {
				for range benchmark.N {
					before := processStartup
					processStartup = startupState{directory: root, gitPath: proxy}
					authority, _, failure := New(context.Background(), root)
					processStartup = before
					if authority != nil {
						authority.Close()
					}
					if authority != nil || failure == nil {
						benchmark.Fatalf("authority=%v failure=%v", authority, failure)
					}
				}
			})
			if allocated := result.AllocedBytesPerOp(); allocated > 20<<20 {
				t.Fatalf("New allocated %d bytes for malformed %s output", allocated, intercepted)
			}
		})
	}
}

func writeGitOutputProxy(t *testing.T, upstream, cat, intercepted, payload string) string {
	t.Helper()
	proxy := filepath.Join(stableTestDirectory(t, "git-proxy-"), "git")
	script := "#!/bin/sh\n" +
		"for argument in \"$@\"; do\n" +
		"  if [ \"$argument\" = " + shellQuote(intercepted) + " ]; then exec " + shellQuote(cat) + " " + shellQuote(payload) + "; fi\n" +
		"done\n" +
		"exec " + shellQuote(upstream) + " \"$@\"\n"
	if err := os.WriteFile(proxy, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return proxy
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestRepositoryIncludeIsRejectedBeforeFirstGitChild(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	base := stableTestDirectory(t, "include-")
	root := filepath.Join(base, "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	external := filepath.Join(t.TempDir(), "must-not-open")
	if err := os.WriteFile(external, []byte("[invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, ".git", "config")
	file, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n[include]\n\tpath = " + external + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	budget := NewBudget(context.Background())
	defer budget.Close()
	authority, _, failure := NewAttempt(root, budget)
	if authority != nil || failure == nil {
		t.Fatalf("authority=%v failure=%v", authority, failure)
	}
	budget.mu.Lock()
	children := budget.children
	budget.mu.Unlock()
	if children != 0 {
		t.Fatalf("Git started before include rejection: children=%d", children)
	}
}

func TestFinishClassifiesObservedConfigDriftAsChanged(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	base := stableTestDirectory(t, "config-drift-")
	root := filepath.Join(base, "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "add", "tracked")
	runTestGit(t, git, root, "-c", "user.name=Corvint", "-c", "user.email=corvint@example.invalid", "commit", "-q", "-m", "initial")
	authority, _, failure := New(context.Background(), root)
	if failure != nil {
		t.Fatalf("New failure = %#v", failure)
	}
	config := filepath.Join(root, ".git", "config")
	file, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n[include]\npath=/tmp/forbidden\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if finish := authority.Finish(context.Background()); finish.Code != dashboardauthority.FinishChanged {
		t.Fatalf("finish = %#v", finish)
	}
}

func TestFinishClassifiesNewCommonDirAsChanged(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("upstream Git unavailable")
	}
	base := stableTestDirectory(t, "common-drift-")
	root := filepath.Join(base, "repository")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "tracked"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, git, root, "add", "tracked")
	runTestGit(t, git, root, "-c", "user.name=Corvint", "-c", "user.email=corvint@example.invalid", "commit", "-q", "-m", "initial")
	authority, _, failure := New(context.Background(), root)
	if failure != nil {
		t.Fatalf("New failure = %#v", failure)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "commondir"), []byte("malformed without lf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if finish := authority.Finish(context.Background()); finish.Code != dashboardauthority.FinishChanged {
		t.Fatalf("finish = %#v", finish)
	}
}

func TestRepositoryRejectsSymlinkedRootComponentBeforeGit(t *testing.T) {
	base := stableTestDirectory(t, "symlink-")
	realDirectory := filepath.Join(base, "real")
	if err := os.Mkdir(realDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "linked")
	if err := os.Symlink(realDirectory, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	budget := NewBudget(context.Background())
	defer budget.Close()
	authority, _, failure := NewAttempt(link, budget)
	if authority != nil || failure == nil {
		t.Fatalf("authority=%v failure=%v", authority, failure)
	}
	budget.mu.Lock()
	children := budget.children
	budget.mu.Unlock()
	if children != 0 {
		t.Fatalf("Git started before component-symlink rejection: children=%d", children)
	}
}

func runTestGit(t *testing.T, executable, directory string, arguments ...string) {
	t.Helper()
	arguments = append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	command.Env = []string{
		"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=0",
	}
	for _, name := range []string{"SystemRoot", "TMPDIR", "TEMP", "TMP"} {
		if value, present := os.LookupEnv(name); present {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(arguments, " "), err, output)
	}
}

func testGitOutput(t *testing.T, executable, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command(executable, arguments...)
	command.Dir = directory
	command.Env = []string{"LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull}
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(arguments, " "), err)
	}
	return string(output)
}

// stableTestDirectory returns a symlink-free directory under the test's own
// temporary directory: the authority rejects symlinked root components, and a
// sandboxed run may write nowhere but TMPDIR.
func stableTestDirectory(t *testing.T, prefix string) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path, err := os.MkdirTemp(base, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return path
}
