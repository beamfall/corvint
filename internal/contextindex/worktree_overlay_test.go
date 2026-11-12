package contextindex

import (
	"context"
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
