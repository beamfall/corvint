package contextindex

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestDCRV1411ObserveTreeMatchesBuildContext: the tree observation a corpus reader's freshness
// uses holds exactly the tracked and skipped paths, header fields and dirty paths a full build
// of the same state holds, and each admitted source's blob ID and mode, without reading any
// source (documentation-corpus DCR-V1-411).
func TestDCRV1411ObserveTreeMatchesBuildContext(t *testing.T) {
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

// TestDCRV1411ObserveTreeRetriesWhenHeadMoves: a commit that lands between the identity read and
// the status read leaves a status of the new commit beside a listing of the old tree; the closing
// identity read detects it and the observation is repeated, so the returned listing, identity
// and status all describe the new commit. HEAD moving on every attempt refuses instead of mixing
// states (documentation-corpus DCR-V1-411).
func TestDCRV1411ObserveTreeRetriesWhenHeadMoves(t *testing.T) {
	root := testRepository(t)
	writeTestFile(t, root, "docs/guide.md", "# Guide\n\nOld.\n")
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "old")
	old := testGit(t, root, "rev-parse", "HEAD")
	commits := 0
	saved := observeTreeStep
	t.Cleanup(func() { observeTreeStep = saved })
	observeTreeStep = func(attempt int) {
		if attempt == 0 {
			commits++
			writeTestFile(t, root, "docs/guide.md", fmt.Sprintf("# Guide\n\nNew %d.\n", commits))
			testGit(t, root, "commit", "-qam", "new")
		}
	}
	observed, err := ObserveTree(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	head := testGit(t, root, "rev-parse", "HEAD")
	if head == old || observed.CommitRevision != head || observed.Revision != testGit(t, root, "rev-parse", "HEAD^{tree}") {
		t.Fatalf("observed %s/%s, HEAD moved %s -> %s", observed.CommitRevision, observed.Revision, old, head)
	}
	if blob := observed.Blobs["docs/guide.md"].OID; blob != testGit(t, root, "rev-parse", "HEAD:docs/guide.md") {
		t.Fatalf("listing of the old tree: %s", blob)
	}
	settled, err := Observe(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if observed.StatusSHA256 != settled.StatusSHA256 || len(observed.DirtyPaths) != 0 {
		t.Fatalf("status %s %v, settled %s", observed.StatusSHA256, observed.DirtyPaths, settled.StatusSHA256)
	}
	observeTreeStep = func(int) {
		commits++
		writeTestFile(t, root, "docs/guide.md", fmt.Sprintf("# Guide\n\nNew %d.\n", commits))
		testGit(t, root, "commit", "-qam", "again")
	}
	if _, err := ObserveTree(context.Background(), root); err == nil || !strings.Contains(err.Error(), "HEAD changed") {
		t.Fatalf("HEAD moving on every attempt: %v", err)
	}
}
