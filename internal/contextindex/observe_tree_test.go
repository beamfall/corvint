package contextindex

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestDCPV1411ObserveTreeMatchesBuildContext: the tree observation a corpus reader's freshness
// uses holds exactly the tracked and skipped paths, header fields and dirty paths a full build
// of the same state holds, and each admitted source's blob ID and mode, without reading any
// source (documentation-corpus DCP-V1-411).
func TestDCPV1411ObserveTreeMatchesBuildContext(t *testing.T) {
	t.Parallel()
	root := testRepository(t)
	writeTestFile(t, root, "docs/guide.md", "# Guide\n\nSteps.\n")
	writeTestFile(t, root, "tools/run.sh", "#!/bin/sh\necho run\n")
	if err := os.Chmod(filepath.Join(root, "tools/run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("guide.md", filepath.Join(root, "docs/link.md")); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", ".")
	sub := testGit(t, root, "rev-parse", "HEAD")
	testGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+sub+",vendor/sub")
	blob := testGit(t, root, "rev-parse", "HEAD:go.mod")
	testGit(t, root, "update-index", "--add", "--cacheinfo", "100644,"+blob+",docs/caf\xff.md")
	testGit(t, root, "commit", "-qm", "kinds")
	writeTestFile(t, root, "docs/guide.md", "# Guide\n\nChanged.\n")
	writeTestFile(t, root, "docs/new.md", "# New\n")
	ctx := context.Background()
	observed, err := ObserveTree(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	index, err := BuildContext(ctx, root, "")
	if err != nil {
		t.Fatal(err)
	}
	header := Observation{ObjectFormat: index.ObjectFormat, CommitRevision: index.CommitRevision, Revision: index.Revision, StatusSHA256: index.StatusSHA256, DirtyPaths: index.DirtyPaths}
	if !slices.Equal(observed.DirtyPaths, header.DirtyPaths) || observed.ObjectFormat != header.ObjectFormat || observed.CommitRevision != header.CommitRevision || observed.Revision != header.Revision || observed.StatusSHA256 != header.StatusSHA256 {
		t.Fatalf("header %+v, build %+v", observed.Observation, header)
	}
	if got, want := slices.Sorted(maps.Keys(observed.Tracked)), slices.Sorted(maps.Keys(index.Tracked)); !slices.Equal(got, want) || len(observed.Blobs) != len(got)-1 {
		t.Fatalf("tracked %v, build %v", got, want)
	}
	if got, want := slices.Sorted(maps.Keys(observed.Skipped)), slices.Sorted(maps.Keys(index.Skipped)); !slices.Equal(got, want) || !slices.Contains(got, "vendor/sub") {
		t.Fatalf("skipped %v, build %v", got, want)
	}
	for path, source := range index.Sources {
		if blob := observed.Blobs[path]; blob.OID != source.BlobHash || blob.Mode != source.Mode {
			t.Errorf("%s: observed %+v, build %s %s", path, blob, source.Mode, source.BlobHash)
		}
	}
	if _, ok := observed.Blobs["docs/caf\uFFFD.md"]; ok || !slices.Contains(slices.Collect(maps.Keys(observed.Tracked)), "docs/caf\uFFFD.md") {
		t.Fatalf("a non-UTF-8 path stands for its display path: %v", slices.Sorted(maps.Keys(observed.Tracked)))
	}
	if observed.Blobs["tools/run.sh"].Mode != "100755" || observed.Blobs["docs/link.md"].Mode != "120000" {
		t.Fatalf("modes not kept: %+v", observed.Blobs)
	}
}
