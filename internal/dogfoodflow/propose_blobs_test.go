package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestProposeCitationsDCWV0752FastBlobReadsMatchTheSlowPath (V1-2360): the
// draft read through one tree listing per commit and one cat-file --batch
// process is byte-identical to the draft read with an ls-tree and a cat-file
// per blob, and each blob, a symlink, a directory and a missing path read the
// same both ways.
func TestProposeCitationsDCWV0752FastBlobReadsMatchTheSlowPath(t *testing.T) {
	root, base := selfDevelopmentRepository(t)
	draft := func(fast bool) (string, string) {
		t.Helper()
		previous := blobFastPath
		blobFastPath = fast
		defer func() { blobFastPath = previous }()
		var stdout, stderr bytes.Buffer
		if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
			t.Fatalf("ProposeCitations(fast=%v) = %d, %v\n%s", fast, code, err, stderr.String())
		}
		return stdout.String(), stderr.String()
	}
	fastOut, fastErr := draft(true)
	slowOut, slowErr := draft(false)
	if fastOut != slowOut || fastErr != slowErr {
		t.Fatalf("the fast draft differs from the slow one:\nfast:\n%s%s\nslow:\n%s%s", fastOut, fastErr, slowOut, slowErr)
	}
	// A symbolic link refuses the draft's patch, so it is added after the
	// drafts, for the blob reads alone.
	if err := os.Symlink("x.go", filepath.Join(root, "internal/x/link.go")); err != nil {
		t.Skipf("symlink: %v", err)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "link")
	head := testGit(t, root, "rev-parse", "HEAD")
	reader := func() *proposal {
		return &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &bytes.Buffer{}}, blobs: map[string][]byte{}, lines: map[string][][]byte{}}
	}
	for _, commit := range []string{base, head} {
		for _, name := range []string{"internal/x/x.go", "internal/x/link.go", "internal/x", "docs/specs/x-v0.md", "no/such/file"} {
			fast, slow := reader(), reader()
			blobFastPath = false
			slowData, slowFound := slow.blob(commit, name)
			blobFastPath = true
			fastData, fastFound := fast.blob(commit, name)
			fast.closeBlobs()
			if fastFound != slowFound || !bytes.Equal(fastData, slowData) {
				t.Errorf("blob(%s, %s): fast = %v %q, slow = %v %q", commit[:7], name, fastFound, fastData, slowFound, slowData)
			}
		}
	}
}
