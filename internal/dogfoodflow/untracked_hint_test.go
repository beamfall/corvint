package dogfoodflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

// DCW-V0-091 (V1-1658): an untracked file that a personal excludes file hides
// from git status still makes the recorder refuse record-index-failed in an
// otherwise clean checkout; the fix line names the path and its recovery
// instead of the uncommitted-sidecar hint, and a clean tree keeps that hint.
func TestChangeDCWV0091NamesUntrackedPathsForRecordIndexFailed(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "source.go"), []byte("package fixture\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "source.go")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	excludes := filepath.Join(t.TempDir(), "excludes")
	if err := os.WriteFile(excludes, []byte("review.txt\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "config", "core.excludesFile", excludes)
	if err := os.WriteFile(filepath.Join(root, "review.txt"), []byte("notes\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if status := testGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("git status should hide the excluded file: %q", status)
	}
	_, err = tracerecordrepo.RecordDogfood(context.Background(), root, base, base, tracerecordrepo.Input{})
	if reason := tracerecordrepo.DogfoodFailureReason(err); reason != "record-index-failed" {
		t.Fatalf("recorder reason=%q err=%v, want record-index-failed", reason, err)
	}
	row := step{"local-outcome", "NOT_PRODUCED", "record-index-failed"}
	c := &change{flow: flow{ctx: context.Background(), root: root, base: base}}
	hint := c.fixHint(row)
	if !strings.Contains(hint, "counts these untracked paths as dirty: review.txt;") || !strings.Contains(hint, "corvint dogfood change "+base) {
		t.Fatalf("hint does not name the untracked path: %q", hint)
	}
	if err := os.Remove(filepath.Join(root, "review.txt")); err != nil {
		t.Fatal(err)
	}
	if hint := c.fixHint(row); !strings.Contains(hint, "the worktree has uncommitted changes") {
		t.Fatalf("clean tree hint: %q", hint)
	}
}
