package contextindex

import (
	"context"
	"fmt"
	"os"
	"testing"
)

// TestERIV0261RangeBoundRefusesWithoutDecodingTheSnapshot pins ERI-V0-261
// (V1-1881): with a snapshot file present for HEAD's tree, an over-bound range
// refuses with the loaded path's exact error while the snapshot body is
// truncated to nothing, so no snapshot byte was decoded; every case it cannot
// prove answers nil and leaves the loaded path to decide.
func TestERIV0261RangeBoundRefusesWithoutDecodingTheSnapshot(t *testing.T) {
	t.Parallel()
	commitAdded := func(t *testing.T, root string, count int, prefix string) {
		t.Helper()
		for item := 0; item < count; item++ {
			writeTestFile(t, root, fmt.Sprintf("%s/%03d.txt", prefix, item), fmt.Sprintf("range member %s %d\n", prefix, item))
		}
		testGit(t, root, "add", ".")
		testGit(t, root, "commit", "-qm", "range "+prefix)
	}
	// indexed builds the index, writes its snapshot, and empties the snapshot
	// file so any decode of it would fail.
	indexed := func(t *testing.T, root string) *Index {
		t.Helper()
		index, err := Build(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := WriteSnapshot(index)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Path == "" {
			t.Skip("snapshots are disabled for this test binary")
		}
		if err := os.Truncate(receipt.Path, 0); err != nil {
			t.Fatal(err)
		}
		return index
	}
	for _, objectFormat := range []string{"sha1", "sha256"} {
		t.Run("over-bound/"+objectFormat, func(t *testing.T) {
			root := impactRepository(t, objectFormat)
			base := testGit(t, root, "rev-parse", "HEAD")
			commitAdded(t, root, 101, "notes")
			index := indexed(t, root)
			_, want := RangeImpact(context.Background(), index, base, 20)
			got := RangeBoundRefusalBeforeLoad(context.Background(), root, base, 20, false)
			assertRangeErrorCode(t, got, "unsupported-impact-range")
			if want == nil || got.Error() != want.Error() {
				t.Fatalf("precheck error %v, loaded path error %v", got, want)
			}
		})
	}
	t.Run("expanded-over-bound", func(t *testing.T) {
		root := impactRepository(t, "")
		base := testGit(t, root, "rev-parse", "HEAD")
		commitAdded(t, root, 257, "notes")
		index := indexed(t, root)
		_, want := ExpandedRangeImpact(context.Background(), index, base, 20)
		got := RangeBoundRefusalBeforeLoad(context.Background(), root, base, 20, true)
		if want == nil || got == nil || got.Error() != want.Error() {
			t.Fatalf("expanded precheck error %v, loaded path error %v", got, want)
		}
		if RangeBoundRefusalBeforeLoad(context.Background(), root, base, 20, false) == nil {
			t.Fatal("default profile did not refuse a 257-path range")
		}
	})
	undecided := map[string]func(t *testing.T) (root, base string){
		"in-bound": func(t *testing.T) (string, string) {
			root := impactRepository(t, "")
			base := testGit(t, root, "rev-parse", "HEAD")
			commitAdded(t, root, 3, "notes")
			indexed(t, root)
			return root, base
		},
		"no-snapshot": func(t *testing.T) (string, string) {
			root := impactRepository(t, "")
			base := testGit(t, root, "rev-parse", "HEAD")
			commitAdded(t, root, 101, "notes")
			return root, base
		},
		"dirty-worktree": func(t *testing.T) (string, string) {
			root := impactRepository(t, "")
			base := testGit(t, root, "rev-parse", "HEAD")
			commitAdded(t, root, 101, "notes")
			indexed(t, root)
			writeTestFile(t, root, "scratch.txt", "untracked\n")
			return root, base
		},
		"unqualified-module": func(t *testing.T) (string, string) {
			root := impactRepository(t, "")
			writeTestFile(t, root, "go.mod", "module fixture\n\ngo 1.27.0\n")
			testGit(t, root, "commit", "-qam", "unqualified module")
			base := testGit(t, root, "rev-parse", "HEAD")
			commitAdded(t, root, 101, "notes")
			indexed(t, root)
			return root, base
		},
		"not-an-ancestor": func(t *testing.T) (string, string) {
			root := impactRepository(t, "")
			testGit(t, root, "checkout", "-qb", "side")
			commitAdded(t, root, 2, "side")
			base := testGit(t, root, "rev-parse", "HEAD")
			testGit(t, root, "checkout", "-q", "-")
			commitAdded(t, root, 101, "notes")
			indexed(t, root)
			return root, base
		},
	}
	for name, setup := range undecided {
		t.Run(name, func(t *testing.T) {
			root, base := setup(t)
			if err := RangeBoundRefusalBeforeLoad(context.Background(), root, base, 20, false); err != nil {
				t.Fatalf("precheck decided an unproved case: %v", err)
			}
		})
	}
	t.Run("invalid-limit", func(t *testing.T) {
		root := impactRepository(t, "")
		base := testGit(t, root, "rev-parse", "HEAD")
		commitAdded(t, root, 101, "notes")
		indexed(t, root)
		if err := RangeBoundRefusalBeforeLoad(context.Background(), root, base, 0, false); err != nil {
			t.Fatalf("limit 0 precheck: %v", err)
		}
	})
}

// TestERIV0285PerPathPrecheckKeepsTheRecordBound pins ERI-V0-285 against the
// ERI-V0-261 precheck: with a snapshot present, a per-path range just over the
// 100-path bound is left to the compiler, which analyses the first 100 paths
// and names the rest PATH_BOUND, instead of being refused whole before load.
func TestERIV0285PerPathPrecheckKeepsTheRecordBound(t *testing.T) {
	t.Parallel()
	root := impactRepository(t, "")
	base := testGit(t, root, "rev-parse", "HEAD")
	for item := 0; item <= maxImpactPaths; item++ {
		writeTestFile(t, root, fmt.Sprintf("notes/%03d.txt", item), fmt.Sprintf("range member %d\n", item))
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "range just over the path bound")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Path == "" {
		t.Skip("snapshots are disabled for this test binary")
	}
	if got := PerPathRangeBoundRefusalBeforeLoad(context.Background(), root, base, 20, false); got != nil {
		t.Fatalf("per-path precheck refused a %d-path range whole: %v", maxImpactPaths+1, got)
	}
	if RangeBoundRefusalBeforeLoad(context.Background(), root, base, 20, false) == nil {
		t.Fatal("strict precheck did not refuse the over-bound range")
	}
}
