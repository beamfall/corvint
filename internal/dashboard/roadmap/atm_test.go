package roadmap

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// initScratchRepo creates a throwaway git repository with one committed
// file and returns its root.
func initScratchRepo(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("scratch git fixture assumes a POSIX git")
	}
	if _, err := exec.LookPath("git"); err != nil {
		// LOD-V0-032's classification is defined in terms of real git output
		// (git rev-parse HEAD^{tree}, git status --porcelain); a git-free
		// fixture would not exercise it. This is the sole acceptance evidence
		// for that requirement (invariant 8), and this repo's own gate always
		// runs inside a git worktree, so a missing git here is a broken gate
		// environment, not a host to silently skip.
		t.Fatalf("git binary unavailable: %v", err)
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", "test")
	run("config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-q", "-m", "initial")
	return root
}

// TestReceiptClassificationRespectsWorktreeCleanliness is the acceptance
// evidence for LOD-V0-032's worktree-dirty gap fix: a receipt whose
// inputIdentity matches the current tree digest still renders NOT_OBSERVED
// with reason "worktree-dirty" once the worktree has an uncommitted edit,
// and only renders CURRENT once the worktree is clean again.
func TestReceiptClassificationRespectsWorktreeCleanliness(t *testing.T) {
	root := initScratchRepo(t)
	ctx := context.Background()

	digest, err := CurrentTreeDigest(ctx, root, 10*time.Second)
	if err != nil {
		t.Fatalf("CurrentTreeDigest: %v", err)
	}

	receiptsDir := t.TempDir()
	writeReceiptFixture(t, receiptsDir, "TCP-V0-001", digest)

	dirty, err := WorktreeDirty(ctx, root, 10*time.Second)
	if err != nil {
		t.Fatalf("WorktreeDirty (clean): %v", err)
	}
	if dirty {
		t.Fatalf("worktree reported dirty right after commit")
	}
	receipt := LoadReceipt(receiptsDir, "TCP-V0-001", digest, dirty)
	if receipt.State != receiptStateCurrent || receipt.Reason != "" {
		t.Fatalf("clean-tree receipt = %+v", receipt)
	}

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("uncommitted edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dirty, err = WorktreeDirty(ctx, root, 10*time.Second)
	if err != nil {
		t.Fatalf("WorktreeDirty (dirty): %v", err)
	}
	if !dirty {
		t.Fatalf("worktree reported clean with an uncommitted edit")
	}
	receipt = LoadReceipt(receiptsDir, "TCP-V0-001", digest, dirty)
	if receipt.State != notObserved || receipt.Reason != "worktree-dirty" {
		t.Fatalf("dirty-tree receipt = %+v", receipt)
	}
}

// TestTreeChecksIgnoreAmbientGitEnvironment pins LOD-V0-032's "no filters"
// clause: an inherited GIT_DIR cannot substitute another repository's tree,
// and a user-level excludesFile cannot hide an untracked file from the dirty
// check.
func TestTreeChecksIgnoreAmbientGitEnvironment(t *testing.T) {
	root, other := initScratchRepo(t), initScratchRepo(t)
	if err := os.WriteFile(filepath.Join(other, "tracked.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commit := exec.Command("git", "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-qam", "other")
	commit.Dir = other
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	ctx := context.Background()
	want, err := CurrentTreeDigest(ctx, root, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	excludes, config := filepath.Join(t.TempDir(), "excludes"), filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(excludes, []byte("*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("[core]\n\texcludesFile = "+excludes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	if dirty, err := WorktreeDirty(ctx, root, 10*time.Second); err != nil || !dirty {
		t.Fatalf("WorktreeDirty under a user excludesFile = %v, %v; want dirty", dirty, err)
	}
	t.Setenv("GIT_DIR", filepath.Join(other, ".git"))
	if got, err := CurrentTreeDigest(ctx, root, 10*time.Second); err != nil || got != want {
		t.Fatalf("CurrentTreeDigest under GIT_DIR = %q, %v; want %q", got, err, want)
	}
}
