package store

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func treeBoundGit(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Stdin = strings.NewReader(stdin)
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// TestTOLV0941_BoundedListingStopsAtTheBound: the bounded tree listing
// returns the first limit kept paths in Git's tree order, reports over, and
// stops reading at the first path past the bound: a tree whose last entry
// is a path longer than the 64 KiB entry bound, which the unbounded listing
// refuses LIMIT_EXCEEDED on reading, lists cleanly under a bound that stops
// before it (TOL-V0-941, V1-1961).
func TestTOLV0941_BoundedListingStopsAtTheBound(t *testing.T) {
	dir := t.TempDir()
	treeBoundGit(t, dir, "", "init", "-q")
	blob := treeBoundGit(t, dir, "hay\n", "hash-object", "-w", "--stdin")
	long := strings.Repeat("z", maxPreflightTreeEntryBytes+16)
	tree := treeBoundGit(t, dir, "100644 blob "+blob+"\ta1\n100644 blob "+blob+"\ta2\n100644 blob "+blob+"\ta3\n100644 blob "+blob+"\tb1\n100644 blob "+blob+"\t"+long+"\n", "mktree")
	commit := treeBoundGit(t, dir, "", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit-tree", tree, "-m", "bound")

	all := func(string) bool { return true }
	if _, err := TreePathsAtCommit(dir, commit, nil, all); err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("unbounded listing read the long path without refusing: %v", err)
	}
	paths, over, err := TreePathsAtCommitUpTo(dir, commit, nil, 2, all)
	if err != nil || !over || strings.Join(paths, ",") != "a1,a2" {
		t.Fatalf("bound 2: paths %v over %v err %v", paths, over, err)
	}
	onlyB := func(p string) bool { return strings.HasPrefix(p, "b") }
	if paths, over, err = TreePathsAtCommitUpTo(dir, commit, nil, 1, onlyB); err == nil {
		t.Fatalf("a bound that reaches the long path stopped early: %v %v", paths, over)
	} else if e, ok := err.(*wire.Error); !ok || e.Code != wire.CodeLimitExceeded {
		t.Fatalf("long path code: %v", err)
	}
	if paths, over, err = TreePathsAtCommitUpTo(dir, commit, []string{"a2"}, 4, all); err != nil || over || strings.Join(paths, ",") != "a2" {
		t.Fatalf("prefix under the bound: paths %v over %v err %v", paths, over, err)
	}
}
