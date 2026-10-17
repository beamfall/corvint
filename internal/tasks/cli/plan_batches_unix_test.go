//go:build darwin || linux

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// pbFakeGit puts a git wrapper first on PATH: action runs for the named
// subcommand, every other run is the real Git.
func pbFakeGit(t *testing.T, sub, action string) {
	t.Helper()
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \" $* \" in\n*\" " + sub + " \"*) " + action + " ;;\nesac\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func pbPlanErr(t *testing.T, root string, cands []*batchCandidate) error {
	t.Helper()
	rc := &readCtx{repo: &intent.Repository{PrimaryWorktree: root, CommonDir: filepath.Join(root, ".git")}}
	_, err := batchPlan(rc, "Q", "main", cands, []string{}, 8)
	return err
}

func pbNoTemp(t *testing.T, tmp string) {
	t.Helper()
	if left, err := os.ReadDir(tmp); err != nil || len(left) != 0 {
		t.Fatalf("throw-away directory left behind: %v %v", left, err)
	}
}

// NTP-V0-024: a diff-tree failure with no missing object refuses the read
// rather than reporting the candidate ungrouped.
func TestNTPV0024_FatalDiffFailureRefuses(t *testing.T) {
	root, base := pbRepo(t)
	tree := pbTree(t, root, base, map[string]string{"a/x": "A\n"})
	pbFakeGit(t, "diff-tree", "exit 128")
	if err := pbPlanErr(t, root, []*batchCandidate{pbCand("T1", base, tree)}); err == nil {
		t.Fatal("a fatal diff-tree failure produced a plan")
	}
}

// NTP-V0-022: the deadline kills the Git process group, so a descendant that
// keeps stdout open cannot hold the read past it, and the directory goes.
func TestNTPV0022_GitDeadlineKillsProcessGroup(t *testing.T) {
	root, base := pbRepo(t)
	tree := pbTree(t, root, base, map[string]string{"a/x": "A\n"})
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	pbFakeGit(t, "merge-tree", "sleep 60")
	was := batchGitTimeout
	batchGitTimeout = time.Second
	defer func() { batchGitTimeout = was }()
	start := time.Now()
	if err := pbPlanErr(t, root, []*batchCandidate{pbCand("T1", base, tree)}); err == nil {
		t.Fatal("a hung merge produced a plan")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("the deadline did not bound the read: %v", elapsed)
	}
	pbNoTemp(t, tmp)
}

// NTP-V0-022: an interrupt cancels the running Git and removes the
// throw-away directory before the read refuses.
func TestNTPV0022_InterruptRemovesTemporaryDirectory(t *testing.T) {
	root, base := pbRepo(t)
	tree := pbTree(t, root, base, map[string]string{"a/x": "A\n"})
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	started := filepath.Join(t.TempDir(), "started")
	pbFakeGit(t, "merge-tree", "touch '"+started+"'; sleep 60")
	was := batchSignals
	batchSignals = []os.Signal{syscall.SIGUSR2}
	defer func() { batchSignals = was }()
	// The bound runs from the signal, not from the start: on a loaded host the
	// Git calls before the merge can take tens of seconds on their own.
	sent := make(chan time.Time, 1)
	go func() {
		for deadline := time.Now().Add(120 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if _, err := os.Stat(started); err == nil {
				sent <- time.Now()
				_ = syscall.Kill(os.Getpid(), syscall.SIGUSR2)
				return
			}
		}
	}()
	if err := pbPlanErr(t, root, []*batchCandidate{pbCand("T1", base, tree)}); err == nil {
		t.Fatal("an interrupted read produced a plan")
	}
	select {
	case at := <-sent:
		if elapsed := time.Since(at); elapsed > 20*time.Second {
			t.Fatalf("the interrupt did not end the read: %v", elapsed)
		}
	default:
		t.Fatal("the read ended before the merge started")
	}
	pbNoTemp(t, tmp)
}
