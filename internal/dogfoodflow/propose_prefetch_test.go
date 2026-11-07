package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProposePrefetchBlobsMatchesBlob (V1-4095): the batched prefetch caches
// exactly what blob reads one path at a time, for a regular file, a file in a
// subdirectory, an executable, a symbolic link, a directory, an absent path, a
// duplicate name and a name with pathspec magic characters, and a blob over
// the output bound is left for blob to read.
func TestProposePrefetchBlobsMatchesBlob(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	big := strings.Repeat("x", 2048) + "\n"
	writeTree(t, root, map[string]string{"a.txt": "a\n", "dir/b.go": "package dir\n", "run.sh": "#!/bin/sh\n", "star*.md": "magic\n", "big.txt": big, "empty.txt": ""})
	if err := os.Chmod(filepath.Join(root, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.txt", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	commit := testGit(t, root, "rev-parse", "HEAD")
	names := []string{"a.txt", "dir/b.go", "run.sh", "link", "dir", "absent.txt", "a.txt", "star*.md", "empty.txt", "big.txt"}
	fresh := func() *proposal {
		var stderr bytes.Buffer
		return &proposal{flow: flow{ctx: context.Background(), root: root, stderr: &stderr, gitOutputLimit: 1024}, blobs: map[string][]byte{}}
	}
	batched := fresh()
	batched.prefetchBlobs(commit, names)
	if _, cached := batched.blobs[commit+":big.txt"]; cached {
		t.Error("a blob over the output bound was cached")
	}
	if _, cached := batched.blobs[commit+":star.md"]; cached {
		t.Error("a pathspec pattern matched a name that was not asked for")
	}
	single := fresh()
	single.gitOutputLimit = 1 << 20
	batched.gitOutputLimit = 1 << 20
	for _, name := range names {
		want, wantOK := single.blob(commit, name)
		got, gotOK := batched.blob(commit, name)
		if gotOK != wantOK || !bytes.Equal(got, want) || (want == nil) != (got == nil) {
			t.Errorf("%s: prefetched (%q, %v), blob (%q, %v)", name, got, gotOK, want, wantOK)
		}
	}
}
