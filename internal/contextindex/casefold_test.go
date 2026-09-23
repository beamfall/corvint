package contextindex

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// caseFoldContents are two tracked paths that differ only in case. A
// case-insensitive filesystem can hold only one of them, so reading the other
// through the worktree yields the wrong file's bytes.
var caseFoldContents = map[string]string{
	"internal/token/token.go": "package token\n\nfunc MintToken() string { return \"lower\" }\n",
	"internal/token/Token.go": "package token\n\nfunc MintTokenUpper() string { return \"upper\" }\n",
}

// caseFoldRepository commits caseFoldContents through Git plumbing, which
// works on a case-insensitive filesystem where `git add` could not stage both.
// It returns the root and the pre-collision base commit.
func caseFoldRepository(t *testing.T) (string, string) {
	t.Helper()
	root := testRepository(t)
	base := testGit(t, root, "rev-parse", "HEAD")
	for _, path := range slices.Sorted(maps.Keys(caseFoldContents)) {
		blob := filepath.Join(t.TempDir(), "blob")
		if err := os.WriteFile(blob, []byte(caseFoldContents[path]), 0o644); err != nil {
			t.Fatal(err)
		}
		oid := testGit(t, root, "hash-object", "-w", blob)
		testGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+oid+","+path)
	}
	tree := testGit(t, root, "write-tree")
	commit := testGit(t, root, "commit-tree", tree, "-p", "HEAD", "-m", "case-fold collision")
	testGit(t, root, "update-ref", "HEAD", commit)
	testGit(t, root, "checkout", "-q", "--", ".")
	return root, base
}

// porcelainDirtyPaths is what `git status --porcelain` reports dirty, sorted:
// the colliding path a case-insensitive worktree cannot hold, or nothing.
func porcelainDirtyPaths(t *testing.T, root string) []string {
	t.Helper()
	dirty := make([]string, 0)
	for _, line := range strings.Split(testGit(t, root, "status", "--porcelain"), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 {
			dirty = append(dirty, fields[1])
		}
	}
	slices.Sort(dirty)
	return dirty
}

// TestBuildPinsCaseFoldCollidingPathsToTheirOwnBlobs checks GPK-V0-006 over
// a tree carrying two paths that differ only in case. Whatever the filesystem
// holds, each path must carry its own committed blob (never the other case's
// bytes), the divergence must be reported dirty exactly as Git reports it, a
// second build must agree byte for byte, and range impact must refuse by name
// over the dirty worktree instead of combining the two files.
func TestBuildPinsCaseFoldCollidingPathsToTheirOwnBlobs(t *testing.T) {
	root, base := caseFoldRepository(t)
	dirty := porcelainDirtyPaths(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range caseFoldContents {
		source, ok := index.Sources[path]
		if !ok {
			t.Fatalf("%s missing from the index; sources %v", path, slices.Sorted(maps.Keys(caseFoldContents)))
		}
		if string(source.Data) != content {
			t.Fatalf("%s carries %q, want its own committed blob %q", path, source.Data, content)
		}
		if want := gitBlobHash([]byte(content), "sha1"); source.BlobHash != want {
			t.Fatalf("%s blob = %s, want %s", path, source.BlobHash, want)
		}
	}
	if !slices.Equal(index.DirtyPaths, dirty) {
		t.Fatalf("dirty paths = %v, want Git's %v", index.DirtyPaths, dirty)
	}
	again, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path := range caseFoldContents {
		if again.Sources[path].BlobHash != index.Sources[path].BlobHash {
			t.Fatalf("%s blob differs between builds", path)
		}
	}
	if !slices.Equal(again.DirtyPaths, index.DirtyPaths) {
		t.Fatalf("dirty paths differ between builds: %v vs %v", again.DirtyPaths, index.DirtyPaths)
	}
	_, err = RangeImpact(context.Background(), index, base, 10)
	if len(dirty) == 0 {
		if err != nil {
			t.Fatalf("range impact over a clean case-sensitive worktree: %v", err)
		}
		return
	}
	assertRangeErrorCode(t, err, "unsupported-impact-worktree")
}
