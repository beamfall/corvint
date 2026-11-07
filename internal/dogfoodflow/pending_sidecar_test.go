package dogfoodflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// DCW-V0-2143 (V1-4443): a standalone pass is PENDING only when every failing
// row is one the bind commit clears and the sidecar is the worktree's only
// change; another failing row, another dirty path (including a .corvint/ file
// only a personal excludes file hides, which the recorder still counts), or a
// nested finish-lane pass keeps FAIL.
func TestChangeDCWV02143PendingOnlyForTheUncommittedSidecar(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"source.go": "package fixture\n"})
	testGit(t, root, "add", "source.go")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	writeTree(t, root, map[string]string{".corvint/change.cem.json": "{}\n"})
	recorder := step{"local-outcome", "NOT_PRODUCED", "record-index-failed"}
	pending := func(nested bool, rows ...step) bool {
		c := &change{flow: flow{ctx: context.Background(), root: root, base: base}, nested: nested}
		c.rows = rows
		return c.pendingSidecarCommit()
	}
	if !pending(false, recorder) {
		t.Fatal("an untracked sidecar with only the recorder row is not pending")
	}
	if pending(true, recorder) {
		t.Fatal("a nested finish-lane pass was reported pending")
	}
	if pending(false, recorder, step{"cem-cite", "NOT_PRODUCED", "citation-plan-not-provided"}) {
		t.Fatal("a missing plan was reported pending")
	}
	if pending(false, step{"cem-status", "NOT_PRODUCED", "not-ready"}) {
		t.Fatal("a pass without the recorder row was reported pending")
	}
	excludes := filepath.Join(t.TempDir(), "excludes")
	if err := os.WriteFile(excludes, []byte(".corvint/scratch.tmp\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "config", "core.excludesFile", excludes)
	writeTree(t, root, map[string]string{".corvint/scratch.tmp": "x\n"})
	if pending(false, recorder) {
		t.Fatal("a .corvint/ file only a personal excludes file hides was ignored")
	}
	if err := os.Remove(filepath.Join(root, ".corvint/scratch.tmp")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{"source.go": "package fixture // edited\n"})
	if pending(false, recorder) {
		t.Fatal("a modified tracked file beside the sidecar was reported pending")
	}
}
