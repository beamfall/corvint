package contextindex

import (
	"context"
	"strings"
	"testing"
)

// TestRangeImpactRefusesUnsafePathBeforeCopyDetection pins ERI-V0-262
// (V1-2221): a range whose exact diff must end in one path refusal returns
// that refusal's bytes without running the copy-detecting diff, and a range
// whose paths could fail more than one way still reaches the exact diff.
func TestRangeImpactRefusesUnsafePathBeforeCopyDetection(t *testing.T) {
	longPath := strings.Repeat(strings.Repeat("d", 200)+"/", 6) + "long.txt"
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
		{name: "mixed-changed-paths", head: map[string]string{"café.txt": "new\n", longPath: "long\n"}, exactCalls: 1},
		{name: "mixed-base-path", base: map[string]string{longPath: "long\n"}, head: map[string]string{"café.txt": "new\n"}, exactCalls: 1},
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
