package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-064: every Git read of the flow keeps at most its output limit. A
// read past it keeps none of it: flow.git refuses git-output-over-bound and
// boundedGit reports the read unbounded so its caller records NOT_OBSERVED.
func TestDCWV0064GitReadOverItsBoundRefusesOrIsUnbounded(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "big.txt"), bytes.Repeat([]byte("x"), 4096), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "big.txt")
	testGit(t, root, "commit", "-q", "-m", "big")
	var stderr bytes.Buffer
	f := &flow{ctx: context.Background(), prefix: "dogfood-change", root: root, stderr: &stderr, gitOutputLimit: 256}

	output, status, bounded := f.boundedGit("cat-file", "blob", "HEAD:big.txt")
	if bounded || output != "" || status == 0 {
		t.Fatalf("boundedGit over the bound = (%d bytes, %d, %v), want (0 bytes, nonzero, false)", len(output), status, bounded)
	}
	if output, status, bounded := f.boundedGit("cat-file", "-s", "HEAD:big.txt"); !bounded || status != 0 || output != "4096\n" {
		t.Fatalf("boundedGit under the bound = (%q, %d, %v)", output, status, bounded)
	}

	code, err := boundary(f.ctx, func() int {
		f.git(false, "cat-file", "blob", "HEAD:big.txt")
		return 0
	}, func() {})
	if err != nil || code != 2 {
		t.Fatalf("git over the bound = (%d, %v), want exit 2", code, err)
	}
	if got := stderr.String(); !strings.Contains(got, "dogfood-change: REFUSE git-output-over-bound\n") {
		t.Fatalf("stderr = %q, want the git-output-over-bound refusal", got)
	}
}
