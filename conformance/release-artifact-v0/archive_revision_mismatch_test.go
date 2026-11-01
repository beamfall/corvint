package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveRevisionNotHeadRefusesAlone: --revision naming a commit other
// than the checked-out HEAD prints one refusal naming the mismatch and the
// detached-worktree fix, with no misleading witness note before it (V1-3491).
func TestArchiveRevisionNotHeadRefusesAlone(t *testing.T) {
	root := initArchiveTestRepository(t)
	head, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(head))
	mustWrite(t, filepath.Join(root, "tracked.txt"), "moved\n")
	runGitTest(t, root, "-c", "user.name=Corvint Test", "-c", "user.email=corvint@example.invalid", "commit", "-qam", "move head")
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer stdout.Close()
	defer stderr.Close()
	manifest, err := filepath.Abs("manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	code := runArchiveCLI(context.Background(), []string{"--root", root, "--manifest", manifest, "--revision", base, "--output", filepath.Join(parent, "core")}, stdout, stderr)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	got, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	want := "release-artifact-v0 archive: selected revision " + base + " is not the checked-out HEAD; check it out in a detached worktree (git worktree add --detach DIR " + base + ") and run archive there\n"
	if string(got) != want {
		t.Fatalf("stderr =\n%q\nwant\n%q", got, want)
	}
}
