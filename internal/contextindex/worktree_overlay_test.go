package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// overlayFixture is a committed repository whose worktree then rewrites
// cache/demux.go, adds an untracked file and leaves every other path clean.
func overlayFixture(t *testing.T) (string, *Index) {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":         "module example.test/overlay\n\ngo 1.27.0\n",
		"AGENTS.md":      "# Rules\n\nKeep it short.\n",
		"docs/notes.md":  "# Notes\n\nThe cache splits keys.\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/merge.go": "package cache\n\nfunc Merge(left, right string) string { return left + right }\n",
		"cache/gone.go":  "package cache\n\nfunc Gone() {}\n",
	})
	writeTestFile(t, root, "cache/demux.go", "package cache\n\nfunc Split(key string) string { return shardedRendezvous(key) }\n\nfunc shardedRendezvous(key string) string { return key }\n")
	writeTestFile(t, root, "cache/untracked.go", "package cache\n\nfunc shardedRendezvousUntracked() {}\n")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, index
}

// TestOverlayWorktreeDropsDeletedAndUnreadablePaths is DIRTY-CACHE-940: a
// tracked path deleted in the worktree, or replaced by a non-regular file, is
// dropped with a named exclusion instead of being served from HEAD; content
// equal to HEAD keeps the HEAD source.
func TestOverlayWorktreeDropsDeletedAndUnreadablePaths(t *testing.T) {
	t.Parallel()
	root, _ := overlayFixture(t)
	if err := os.Remove(filepath.Join(root, "cache", "gone.go")); err != nil {
		t.Fatal(err)
	}
	merge := filepath.Join(root, "cache", "merge.go")
	if err := os.Remove(merge); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(merge, 0o755); err != nil {
		t.Fatal(err)
	}
	base, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(base.DirtyPaths, "cache/gone.go") || !slices.Contains(base.DirtyPaths, "cache/merge.go") {
		t.Fatalf("fixture dirty paths = %v", base.DirtyPaths)
	}
	view, err := OverlayWorktree(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	for _, dropped := range []string{"cache/gone.go", "cache/merge.go"} {
		if _, kept := view.Sources[dropped]; kept {
			t.Fatalf("%s served from HEAD after the worktree removed it", dropped)
		}
	}
	reasons := map[string]string{}
	for _, exclusion := range view.Exclusions {
		reasons[exclusion.Path] = exclusion.Reason
	}
	if reasons["cache/gone.go"] != "deleted in the worktree" || reasons["cache/merge.go"] != worktreeOverlayUnreadReason {
		t.Fatalf("exclusions = %v", view.Exclusions)
	}
	if !slices.IsSortedFunc(view.Exclusions, func(left, right Exclusion) int { return strings.Compare(left.Path, right.Path) }) {
		t.Fatalf("exclusions not sorted: %v", view.Exclusions)
	}
	writeTestFile(t, root, "cache/demux.go", "package cache\n\nfunc Split(key string) string { return key }\n")
	clean, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	reverted, err := OverlayWorktree(context.Background(), clean)
	if err != nil {
		t.Fatal(err)
	}
	if _, overlaid := reverted.overlaidBlob("cache/demux.go"); overlaid || reverted.Sources["cache/demux.go"].BlobHash != clean.Sources["cache/demux.go"].BlobHash {
		t.Fatal("content equal to HEAD was overlaid")
	}
}

// TestOverlayWorktreeVerificationAuthorityIsCommitted is DIRTY-CACHE-942: a
// worktree edit of AGENTS.md cannot replace the committed mandatory Verify
// command; the overlay view still declares the committed one.
func TestOverlayWorktreeVerificationAuthorityIsCommitted(t *testing.T) {
	t.Parallel()
	root := impactRepositoryWithFiles(t, npmRepository(map[string]string{"AGENTS.md": "# Agents\n\n**Verify:** `npm run build`.\n"}))
	writeTestFile(t, root, "AGENTS.md", "# Agents\n\n**Verify:** `true`.\n")
	base, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	view, err := OverlayWorktree(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if _, overlaid := view.overlaidBlob("AGENTS.md"); !overlaid {
		t.Fatal("fixture: the AGENTS.md edit was not overlaid")
	}
	if got := declaredVerification(view); !slices.Equal(got, []string{"npm run build"}) {
		t.Fatalf("declared verification = %v, want the committed npm run build", got)
	}
}

// TestOverlayWorktreeRefusesSymlinkedParentDirectory is DIRTY-CACHE-940: a
// tracked directory replaced by a symlink to an ignored directory is not read
// through, so ignored bytes never stand in for the tracked path.
func TestOverlayWorktreeRefusesSymlinkedParentDirectory(t *testing.T) {
	t.Parallel()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":     "module example.test/overlay\n\ngo 1.27.0\n",
		".gitignore": "secret/\n",
		"src/a.go":   "package src\n\nfunc Tracked() {}\n",
	})
	writeTestFile(t, root, "secret/a.go", "package src\n\nfunc ignoredSecretBytes() {}\n")
	if err := os.RemoveAll(filepath.Join(root, "src")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("secret", filepath.Join(root, "src")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	base, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(base.DirtyPaths, "src/a.go") {
		t.Fatalf("fixture dirty paths = %v", base.DirtyPaths)
	}
	view, err := OverlayWorktree(context.Background(), base)
	if err != nil {
		t.Fatal(err)
	}
	if source, ok := view.Sources["src/a.go"]; ok && strings.Contains(string(source.Data), "ignoredSecretBytes") {
		t.Fatal("ignored bytes were overlaid as tracked src/a.go")
	}
	if _, overlaid := view.overlaidBlob("src/a.go"); overlaid {
		t.Fatal("src/a.go was overlaid through a symlinked parent")
	}
}
