package contextindex

import (
	"context"
	"testing"
)

// TestCommitPinnedRangeImpactMatchesTheCheckedOutHead pins ERI-V0-265: the
// receipt over an index built at an explicit commit equals the per-path
// receipt compiled while that commit was checked out, except that the range
// binding says COMMIT-PINNED and carries no status digest. It still holds
// after HEAD has moved on and the worktree is dirty, because the pinned path
// reads only the object store.
func TestCommitPinnedRangeImpactMatchesTheCheckedOutHead(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	checkedOut, err := PerPathRangeImpact(context.Background(), index, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	head := index.CommitRevision
	writeTestFile(t, root, "notes.txt", "a later commit\n")
	testGit(t, root, "commit", "-qam", "later")
	writeTestFile(t, root, "internal/token/token.go", "package token\n\nfunc dirty() {}\n")

	pinnedIndex, err := BuildRevisionContext(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := CommitPinnedPerPathRangeImpact(context.Background(), pinnedIndex, base, 50)
	if err != nil {
		t.Fatal(err)
	}
	binding := pinned["range"].(map[string]any)
	if binding["status"] != "COMMIT-PINNED" || binding["statusSha256"] != nil || binding["headCommit"] != head {
		t.Fatalf("pinned range binding = %v", binding)
	}
	// The packet measures its own bytes, so it shrinks by exactly the bytes
	// the binding sheds; nothing else may differ.
	want := checkedOut["range"].(map[string]any)
	before, _ := CanonicalJSON(want)
	want["status"] = "COMMIT-PINNED"
	delete(want, "statusSha256")
	after, _ := CanonicalJSON(want)
	coverage := checkedOut["coverage"].(map[string]any)
	coverage["packet_bytes"] = coverage["packet_bytes"].(int) - (len(before) - len(after))
	left, _ := CanonicalJSON(checkedOut)
	right, _ := CanonicalJSON(pinned)
	if string(left) != string(right) {
		t.Fatalf("commit-pinned receipt differs from the checked-out receipt beyond its binding:\n%s\n%s", left, right)
	}

	// The checked-out path still refuses the same historical index: the
	// pinned variant is the only one that skips the worktree checks.
	if _, err := PerPathRangeImpact(context.Background(), pinnedIndex, base, 50); err == nil {
		t.Fatal("the worktree-bound range impact must refuse an index that is not the checked-out HEAD")
	}
}

// TestCommitPinnedRangeImpactFailsClosed pins ERI-V0-265: a head whose commit
// no longer names the tree the index read, or whose identity is not a full
// object ID, refuses rather than compiling.
func TestCommitPinnedRangeImpactFailsClosed(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := commitRangeFixture(t, root)
	baseTree := testGit(t, root, "rev-parse", base+"^{tree}")
	for name, mutate := range map[string]func(*Index){
		"tree drift":     func(index *Index) { index.Revision = baseTree },
		"short identity": func(index *Index) { index.CommitRevision = index.CommitRevision[:12] },
		"absent commit":  func(index *Index) { index.CommitRevision = "1234567890123456789012345678901234567890" },
	} {
		t.Run(name, func(t *testing.T) {
			index, err := BuildRevisionContext(context.Background(), root, testGit(t, root, "rev-parse", "HEAD"))
			if err != nil {
				t.Fatal(err)
			}
			mutate(index)
			if receipt, err := CommitPinnedPerPathRangeImpact(context.Background(), index, base, 50); err == nil {
				t.Fatalf("a %s compiled %v", name, receipt["range"])
			}
		})
	}
}
