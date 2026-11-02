package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// TestProposeCitationsDCWV0752OversizedBlobClosesWithoutHanging (DCW-V0-752,
// review): a blob over the read bound leaves its body unread in the cat-file
// stream; the read reports it (or refuses the flow), and closing the reader
// kills the blocked process instead of waiting on it forever.
func TestProposeCitationsDCWV0752OversizedBlobClosesWithoutHanging(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	writeTree(t, root, map[string]string{"big.txt": strings.Repeat("0123456789abcdef\n", 1<<16)})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "big")
	head := testGit(t, root, "rev-parse", "HEAD")
	for _, refuse := range []bool{false, true} {
		p := &proposal{flow: flow{ctx: context.Background(), prefix: "test", root: root, stderr: &bytes.Buffer{}, gitOutputLimit: 1024}, blobs: map[string][]byte{}, lines: map[string][][]byte{}}
		entries, ok := p.commitTree(head)
		if !ok {
			t.Fatal("no tree listing")
		}
		var read bool
		code, _ := boundary(context.Background(), func() int {
			_, read = p.batchRead(entries["big.txt"].object, 1024, refuse)
			return 0
		}, func() {})
		if read || refuse != (code == 2) {
			t.Fatalf("refuse=%v: read=%v code=%d, want no read and a refusal only when asked", refuse, read, code)
		}
		closed := make(chan struct{})
		go func() { p.closeBlobs(); close(closed) }()
		select {
		case <-closed:
		case <-time.After(10 * time.Second):
			t.Fatalf("refuse=%v: closeBlobs hangs on the unread oversized blob", refuse)
		}
	}
}
