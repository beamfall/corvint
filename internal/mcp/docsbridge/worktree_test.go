package docsbridge

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// SDD-V0-013: a linked worktree's gitfile root is accepted through the shared
// corvint-mcp root check; a gitfile naming no directory, and a symlinked marker, are refused.
func TestSDD013LinkedWorktreeRoot(t *testing.T) {
	primary := docsRepository(t)
	linked := filepath.Join(t.TempDir(), "linked")
	command := exec.Command("git", "worktree", "add", "--detach", "-q", linked, "HEAD")
	command.Dir = primary
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, output)
	}
	registry, failed := New(linked)
	if failed != nil {
		t.Fatalf("linked worktree refused: %v", failed)
	}
	if !registry.sameRoot() {
		t.Fatal("linked worktree identity not stable")
	}

	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("gitdir: "+filepath.Join(broken, "missing")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, failed := New(broken); failed == nil {
		t.Fatal("gitfile naming a missing directory accepted")
	}

	linkedMarker := t.TempDir()
	if err := os.Symlink(filepath.Join(primary, ".git"), filepath.Join(linkedMarker, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, failed := New(linkedMarker); failed == nil {
		t.Fatal("symlinked .git accepted")
	}
}
