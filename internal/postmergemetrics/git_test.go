//go:build darwin || linux

package postmergemetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	root, e := filepath.EvalSymlinks(root)
	if e != nil {
		t.Fatal(e)
	}
	gitRun(t, root, "init", "-q")
	return root
}
func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	argv := []string{"-c", "user.name=Metrics Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}
	argv = append(argv, args...)
	cmd := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, argv...)...)
	cmd.Dir = root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + root, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "LC_ALL=C"}
	b, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %v %s", args, e, b)
	}
	return strings.TrimSuffix(string(b), "\n")
}
func put(t *testing.T, root, path string, b []byte) {
	t.Helper()
	if e := os.WriteFile(filepath.Join(root, path), b, 0600); e != nil {
		t.Fatal(e)
	}
}
func commit(t *testing.T, root string) string {
	t.Helper()
	gitRun(t, root, "add", "--all")
	gitRun(t, root, "commit", "-qm", "fixture")
	return gitRun(t, root, "rev-parse", "HEAD")
}
func measure(t *testing.T, g *GitMeasurer, p GitPair) Measurement {
	t.Helper()
	m, e := g.Measure(context.Background(), p)
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestMetricsGit(t *testing.T) {
	t.Run("PMM-V0-002 actual-immutable-counts", func(t *testing.T) {
		root := gitFixture(t)
		put(t, root, "mod", []byte("old\n"))
		put(t, root, "del", []byte("gone\n"))
		put(t, root, "rename", []byte("same\n"))
		bot := commit(t, root)
		put(t, root, "mod", []byte("new\nsecond\n"))
		if e := os.Remove(filepath.Join(root, "del")); e != nil {
			t.Fatal(e)
		}
		if e := os.Rename(filepath.Join(root, "rename"), filepath.Join(root, "renamed")); e != nil {
			t.Fatal(e)
		}
		put(t, root, "tab\tline\n.txt", []byte("added\n"))
		approved := commit(t, root)
		g, e := NewGitMeasurer()
		if e != nil {
			t.Fatal(e)
		}
		pair := GitPair{root, bot, approved}
		m := measure(t, g, pair)
		if m.Files != 5 || m.BinaryFiles != 0 || m.Added == nil || *m.Added != 4 || *m.Deleted != 3 {
			t.Fatalf("%+v", m)
		}
		a, _ := json.Marshal(m)
		put(t, root, "mod", []byte("dirty\n"))
		put(t, root, ".gitattributes", []byte("* -diff\n"))
		b, _ := json.Marshal(measure(t, g, pair))
		if !bytes.Equal(a, b) {
			t.Fatalf("dirty worktree changed immutable evidence %s %s", a, b)
		}
		zero := measure(t, g, GitPair{root, approved, approved})
		if zero.Files != 0 || zero.Added == nil || *zero.Added != 0 {
			t.Fatal(zero)
		}
		// Different repositories/ancestors must not be silently measured as approval history.
		gitRun(t, root, "checkout", "--orphan", "other")
		gitRun(t, root, "rm", "-rf", "--ignore-unmatch", ".")
		put(t, root, "other", []byte("unrelated\n"))
		other := commit(t, root)
		if _, e := g.Measure(context.Background(), GitPair{root, bot, other}); e != ErrGit {
			t.Fatalf("nonancestor: %v", e)
		}
	})
	t.Run("PMM-V0-002 binary-and-rename-edit", func(t *testing.T) {
		root := gitFixture(t)
		put(t, root, "old", []byte("a\nb\n"))
		bot := commit(t, root)
		if e := os.Rename(filepath.Join(root, "old"), filepath.Join(root, "new")); e != nil {
			t.Fatal(e)
		}
		put(t, root, "new", []byte("a\nchanged\n"))
		put(t, root, "binary", []byte{0, 1, 2, 3})
		approved := commit(t, root)
		g, _ := NewGitMeasurer()
		m := measure(t, g, GitPair{root, bot, approved})
		if m.Files != 3 || m.BinaryFiles != 1 || m.Added != nil || m.Deleted != nil || m.TextAdded != 2 || m.TextDeleted != 2 {
			t.Fatal(m)
		}
		p, h := fixture(1)
		h.Runs[0].Git = &GitPair{root, bot, approved}
		r, e := Build(context.Background(), p, h, hour(0), hour(2), g)
		if e != nil || r.Classes[0].Recommendation != "review" || r.Classes[0].UnknownEditRuns != 1 {
			t.Fatalf("%v %+v", e, r)
		}
	})
	t.Run("PMM-V0-002 attribute-isolation", func(t *testing.T) {
		root := gitFixture(t)
		put(t, root, "text", []byte("old\n"))
		bot := commit(t, root)
		put(t, root, "text", []byte("new\n"))
		approved := commit(t, root)
		g, _ := NewGitMeasurer()
		pair := GitPair{root, bot, approved}
		global := filepath.Join(t.TempDir(), "attributes")
		if e := os.WriteFile(global, []byte("* -diff\n"), 0600); e != nil {
			t.Fatal(e)
		}
		gitRun(t, root, "config", "core.attributesFile", global)
		m := measure(t, g, pair)
		if m.BinaryFiles != 0 {
			t.Fatal(m)
		}
		put(t, root, ".git/info/attributes", []byte("* -diff\n"))
		if _, e := g.Measure(context.Background(), pair); e != ErrGit {
			t.Fatalf("mutable attributes: %v", e)
		}
		if e := os.Remove(filepath.Join(root, ".git/info/attributes")); e != nil {
			t.Fatal(e)
		}
		put(t, root, ".gitattributes", []byte("text diff=custom\n"))
		approved = commit(t, root)
		gitRun(t, root, "config", "diff.custom.command", "touch executed")
		if _, e := g.Measure(context.Background(), GitPair{root, bot, approved}); e != ErrGit {
			t.Fatalf("named driver: %v", e)
		}
		if _, e := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(e) {
			t.Fatal("driver executed")
		}
	})
}
func TestMetricsNumstat(t *testing.T) {
	t.Run("PMM-V0-002 bounded-nul-parser", func(t *testing.T) {
		for _, raw := range [][]byte{[]byte("1\t2\tx"), []byte("1\t2\t../x\x00"), []byte("1\t2\tx\x001\t2\tx\x00"), []byte("-\t2\tx\x00"), []byte("1\t2\t/abs\x00"), []byte("1\t2\t\xff\x00"), bytes.Repeat([]byte("a"), (4<<20)+1)} {
			if _, _, e := parseNumstat(raw); e != ErrGit {
				t.Fatalf("accepted malformed %q", raw[:min(len(raw), 50)])
			}
		}
		m, paths, e := parseNumstat([]byte("1\t2\ttab\tnewline\n\x00"))
		if e != nil || len(paths) != 1 || m.TextAdded != 1 {
			t.Fatal(e, m, paths)
		}
	})
}
func TestMetricsGitProcessBounds(t *testing.T) {
	t.Run("PMM-V0-007 output-overflow", func(t *testing.T) {
		script := filepath.Join(t.TempDir(), "fake-git")
		if e := os.WriteFile(script, []byte("#!/bin/sh\n/usr/bin/yes payload\n"), 0700); e != nil {
			t.Fatal(e)
		}
		g := &GitMeasurer{executable: script}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, e := g.command(ctx, GitPair{Root: t.TempDir(), Approved: strings.Repeat("a", 40)}, nil, "--version")
		if e != ErrGit {
			t.Fatal(e)
		}
	})
	t.Run("PMM-V0-007 cancel-owned-descendant", func(t *testing.T) {
		root := t.TempDir()
		script := filepath.Join(root, "fake-git")
		pidFile := filepath.Join(root, "child-pid")
		body := "#!/bin/sh\n/bin/sleep 30 &\nprintf '%s' $! > '" + pidFile + "'\nwait\n"
		if e := os.WriteFile(script, []byte(body), 0700); e != nil {
			t.Fatal(e)
		}
		g := &GitMeasurer{executable: script}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, e := g.command(ctx, GitPair{Root: root, Approved: strings.Repeat("a", 40)}, nil, "--version")
			done <- e
		}()
		deadline := time.Now().Add(3 * time.Second)
		var pid string
		for time.Now().Before(deadline) {
			if b, e := os.ReadFile(pidFile); e == nil && len(b) > 0 {
				pid = string(b)
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		if pid == "" {
			t.Fatal("child not started")
		}
		select {
		case e := <-done:
			if e != ErrCancelled {
				t.Fatal(e)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cancellation failed")
		}
		if e := exec.Command("/bin/kill", "-0", pid).Run(); e == nil {
			t.Fatalf("child %s survived cancellation", pid)
		}
	})
}

func TestMetricsGitlinks(t *testing.T) {
	t.Run("PMM-V0-002 gitlink-lines-unknown", func(t *testing.T) {
		root := gitFixture(t)
		put(t, root, "text", []byte("base\n"))
		first := commit(t, root)
		gitRun(t, root, "update-index", "--add", "--cacheinfo", "160000,"+first+",sub")
		gitRun(t, root, "commit", "-qm", "gitlink")
		bot := gitRun(t, root, "rev-parse", "HEAD")
		gitRun(t, root, "update-index", "--cacheinfo", "160000,"+bot+",sub")
		gitRun(t, root, "commit", "-qm", "gitlink update")
		approved := gitRun(t, root, "rev-parse", "HEAD")
		g, _ := NewGitMeasurer()
		m := measure(t, g, GitPair{root, bot, approved})
		if m.Files != 1 || m.GitlinkFiles != 1 || m.BinaryFiles != 0 || m.Added != nil || m.Deleted != nil || m.TextAdded != 0 || m.TextDeleted != 0 {
			t.Fatal(m)
		}
	})
}
