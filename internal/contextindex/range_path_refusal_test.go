package contextindex

import (
	"context"
	"strings"
	"testing"
)

// TestRangeImpactRefusesUnsafePathBeforeCopyDetection pins ERI-V0-262
// (V1-2221): a range whose exact diff must end in one path refusal returns
// that refusal's bytes without running the copy-detecting diff.
func TestRangeImpactRefusesUnsafePathBeforeCopyDetection(t *testing.T) {
	cases := []struct {
		name       string
		base       map[string]string
		head       map[string]string
		exactCalls int
	}{
		{name: "modified-unsafe", base: map[string]string{"café.txt": "one\n"}, head: map[string]string{"café.txt": "two\n"}},
		{name: "added-unsafe", head: map[string]string{"café.txt": "new\n", "notes/plain.txt": "plain\n"}},
		{name: "unsafe-copy-source", base: map[string]string{"naïve.txt": strings.Repeat("shared line\n", 40)},
			head: map[string]string{"señor.txt": strings.Repeat("shared line\n", 40)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := impactRepository(t, "")
			for relative, content := range tc.base {
				writeTestFile(t, root, relative, content)
			}
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "base", "--allow-empty")
			base := testGit(t, root, "rev-parse", "HEAD")
			for relative, content := range tc.head {
				writeTestFile(t, root, relative, content)
			}
			testGit(t, root, "add", ".")
			testGit(t, root, "commit", "-qm", "head")
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			baseTree := testGit(t, root, "rev-parse", base+"^{tree}")
			_, want := readRangeChanges(context.Background(), index, baseTree)
			if want == nil {
				t.Fatal("fixture range must refuse in the exact diff")
			}
			calls := 0
			readRangeChangesFor = func(ctx context.Context, index *Index, baseTree string) ([]rangeChange, error) {
				calls++
				return readRangeChanges(ctx, index, baseTree)
			}
			t.Cleanup(func() { readRangeChangesFor = readRangeChanges })
			_, got := RangeImpact(context.Background(), index, base, 20)
			assertRangeErrorCode(t, got, "unsupported-impact-range")
			if got == nil || got.Error() != want.Error() || calls != tc.exactCalls {
				t.Fatalf("error %v (want %v), exact diff calls %d (want %d)", got, want, calls, tc.exactCalls)
			}
		})
	}
}

// TestRangeCertainPathRefusalDefersMixedRefusals pins the ERI-V0-262 limit:
// when the exact parse's first failing record could carry either path
// message, the proof answers nil and the exact diff decides. The trees are
// written through the Git index because a path over maxPathChars is longer
// than the host's path limit.
func TestRangeCertainPathRefusalDefersMixedRefusals(t *testing.T) {
	longPath := strings.Repeat(strings.Repeat("d", 200)+"/", 6) + "long.txt"
	cases := []struct {
		name       string
		base, head []string
	}{
		{name: "mixed-changed-paths", base: []string{"keep.txt"}, head: []string{"keep.txt", "café.txt", longPath}},
		{name: "mixed-base-path", base: []string{"keep.txt", longPath}, head: []string{"keep.txt", longPath, "café.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := impactRepository(t, "")
			writeTestFile(t, root, "blob.tmp", "range member\n")
			blob := testGit(t, root, "hash-object", "-w", "blob.tmp")
			tree := func(paths []string) string {
				testGit(t, root, "read-tree", "--empty")
				for _, member := range paths {
					testGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blob+","+member)
				}
				return testGit(t, root, "write-tree")
			}
			baseTree, headTree := tree(tc.base), tree(tc.head)
			index := &Index{Root: root, Revision: headTree, ObjectFormat: "sha1"}
			if _, err := readRangeChanges(context.Background(), index, baseTree); err == nil {
				t.Fatal("fixture range must refuse in the exact diff")
			}
			if refusal := rangeCertainRefusal(context.Background(), index, baseTree, maxImpactPaths); refusal != nil {
				t.Fatalf("mixed refusals proved %v; the exact diff must decide", refusal)
			}
		})
	}
}
