package contextindex

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

// TestRangeImpactRefusesUnsafePathBeforeCopyDetection pins ERI-V0-262
// (V1-2221): a range whose exact diff must end in one path refusal returns
// that refusal's bytes without running the copy-detecting diff. An unchanged
// unsafe base path that sorts below the smallest unsafe addition may be the
// copy source the exact refusal names (ERI-V0-264), so that case runs it.
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
			head: map[string]string{"señor.txt": strings.Repeat("shared line\n", 40)}, exactCalls: 1},
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

// TestRangeCertainPathRefusalProvesSmallestFailingPath pins ERI-V0-262 as
// amended by ERI-V0-264: the exact parse refuses with its smallest failing
// path's refusal, so mixed failing changed paths are proved, and with
// additions the proof defers only when an unchanged base-tree path that fails
// sorts below the smallest failing changed path, since the copy-detecting
// diff may name it as a copy source. The trees are written through the Git
// index because a path over maxPathChars is longer than the host's path limit.
func TestRangeCertainPathRefusalProvesSmallestFailingPath(t *testing.T) {
	longPath := strings.Repeat(strings.Repeat("d", 200)+"/", 6) + "long.txt"
	cases := []struct {
		name       string
		base, head []string
		proved     bool
		names      string
	}{
		{name: "mixed-changed-paths", base: []string{"keep.txt"}, head: []string{"keep.txt", "café.txt", longPath}, proved: true, names: `"café.txt"`},
		{name: "larger-unsafe-base-path", base: []string{"keep.txt", longPath}, head: []string{"keep.txt", longPath, "café.txt"}, proved: true, names: `"café.txt"`},
		{name: "smaller-unsafe-copy-source", base: []string{"keep.txt", "aé.txt"}, head: []string{"keep.txt", "aé.txt", "zé.txt"}, names: `"aé.txt"`},
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
			_, want := readRangeChanges(context.Background(), index, baseTree)
			if want == nil || !strings.HasSuffix(want.Error(), ": "+tc.names) {
				t.Fatalf("exact diff refusal %v must name %s", want, tc.names)
			}
			refusal := rangeCertainRefusal(context.Background(), index, baseTree, maxImpactPaths)
			if !tc.proved {
				if refusal != nil {
					t.Fatalf("proved %v; the exact diff must decide", refusal)
				}
				return
			}
			if refusal == nil || refusal.Error() != want.Error() {
				t.Fatalf("proved %v, want the exact refusal %v", refusal, want)
			}
		})
	}
}

// TestRangePathRefusalNamesPathAndRule pins ERI-V0-263 and ERI-V0-264: '@'
// and '+' are admitted, the refusal names the quoted path and the rule it
// fails, and the exact parse names the smallest failing path whatever the
// record order.
func TestRangePathRefusalNamesPathAndRule(t *testing.T) {
	for _, admitted := range []string{"pkg/@scope/name.ts", "docs/c++/notes.md", "a/b@2x.png", "x+y.txt"} {
		if err := validateRangePath(admitted); err != nil {
			t.Errorf("%s refused: %v", admitted, err)
		}
	}
	for _, refused := range []string{"has space.txt", "café.txt", "semi;colon", "dollar$x", "glob*x", "q?x", "pipe|x", "back`tick"} {
		err := validateRangePath(refused)
		if err == nil || !strings.Contains(err.Error(), "allowed characters are ASCII letters") || !strings.HasSuffix(err.Error(), strconv.Quote(refused)) {
			t.Errorf("%q: %v", refused, err)
		}
	}
	index := &Index{ObjectFormat: "sha1"}
	zero := strings.Repeat("0", 40)
	one := strings.Repeat("1", 40)
	record := func(status, changedPath string) string {
		return ":100644 100644 " + zero + " " + one + " " + status + "\x00" + changedPath + "\x00"
	}
	forward := record("M", "b é.txt") + record("M", "z é.txt") + record("M", "ok.txt")
	reverse := record("M", "z é.txt") + record("M", "ok.txt") + record("M", "b é.txt")
	_, first := parseRangeChanges([]byte(forward), index)
	_, second := parseRangeChanges([]byte(reverse), index)
	if first == nil || second == nil || first.Error() != second.Error() || !strings.HasSuffix(first.Error(), `"b é.txt"`) {
		t.Fatalf("forward %v, reverse %v; both must name \"b é.txt\"", first, second)
	}
}
