package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBuildAllocationStaysBoundedOverAnOversizedTrackedSource is the SOP-V0-009 `memory` row: a
// tracked source 64 times larger than maxSourceBytes is excluded by its tree-recorded size, so
// the Go heap a build allocates stays far below the hostile blob instead of scaling with it.
func TestBuildAllocationStaysBoundedOverAnOversizedTrackedSource(t *testing.T) {
	root := testRepository(t)
	const oversized = 64 * maxSourceBytes
	const allocationBound = 16 << 20
	writeTestFile(t, root, "internal/huge/huge.go", "package huge\n\nvar Payload = \""+strings.Repeat("x", oversized)+"\"\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "oversized source")
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	built, err := Build(context.Background(), root)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if !exclusionIn(built.Exclusions, Exclusion{"internal/huge/huge.go", "source exceeds size bound"}) {
		t.Fatalf("oversized source not excluded: %#v", built.Exclusions)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > allocationBound {
		t.Fatalf("build allocated %d bytes over a %d-byte hostile source, bound %d", allocated, oversized, allocationBound)
	}
	t.Logf("build allocated %d bytes over a %d-byte hostile source", allocated, oversized)
}

// TestBuildPinsEachCaseFoldedTrackedPathToItsOwnBlob is the SOP-V0-009 `case-folds-context-index`
// row: two tracked paths that differ only by case share one file on a case-insensitive worktree,
// and each indexed source must still carry its own committed bytes, never the other path's.
func TestBuildPinsEachCaseFoldedTrackedPathToItsOwnBlob(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, "casefold-probe", "probe\n")
	if _, err := os.Stat(filepath.Join(root, "CASEFOLD-PROBE")); err != nil {
		t.Skip("worktree filesystem is case-sensitive; the case-fold collision cannot occur here")
	}
	if err := os.Remove(filepath.Join(root, "casefold-probe")); err != nil {
		t.Fatal(err)
	}
	upper := "package guide\n\nfunc Upper() string { return \"upper\" }\n"
	lower := "package guide\n\nfunc Lower() string { return \"lower\" }\n"
	writeTestFile(t, root, "upper.txt", upper)
	writeTestFile(t, root, "lower.txt", lower)
	upperBlob := testGit(t, root, "hash-object", "-w", "upper.txt")
	lowerBlob := testGit(t, root, "hash-object", "-w", "lower.txt")
	for _, name := range []string{"upper.txt", "lower.txt"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+upperBlob+",internal/guide/Guide.go")
	testGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+lowerBlob+",internal/guide/guide.go")
	testGit(t, root, "commit", "-qm", "case-folded paths")
	testGit(t, root, "checkout", "-f", "HEAD", "--", ".")
	if files, err := os.ReadDir(filepath.Join(root, "internal", "guide")); err != nil || len(files) != 1 {
		t.Fatalf("worktree did not fold the two paths into one file: %v %v", files, err)
	}
	built, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{"internal/guide/Guide.go": upper, "internal/guide/guide.go": lower} {
		source, ok := built.Sources[path]
		if !ok {
			t.Fatalf("case-folded path %s missing from the index", path)
		}
		if string(source.Data) != want {
			t.Fatalf("case-folded path %s carries %q, want its own blob %q", path, source.Data, want)
		}
	}
}
