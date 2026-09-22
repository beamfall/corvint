package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestLoadContextSnapshotHandsTheMissObservationToTheBuild pins the miss
// contract of LoadContextSnapshot: a snapshot directory with no matching file
// is a miss that still carries the loader's paired observation, and a build
// opened on that observation is the build BuildContext makes from its own.
func TestLoadContextSnapshotHandsTheMissObservationToTheBuild(t *testing.T) {
	root := taskContextFixture(t).Root
	if err := os.MkdirAll(SnapshotDirectory(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cache", "demux_test.go"), []byte("package cache\n\nfunc TestSplit() { _ = Split(\"key\") }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index, hit, opening, err := LoadContextSnapshot(context.Background(), root)
	if err != nil || hit || index != nil || opening == nil {
		t.Fatalf("miss with a snapshot directory: hit=%v index=%v opening=%v err=%v", hit, index != nil, opening != nil, err)
	}
	if opening.observed.identityErr != nil || opening.observed.statusErr != nil || len(opening.observed.dirty) != 1 {
		t.Fatalf("observation = %+v", opening.observed)
	}
	observed, err := BuildContextObserved(context.Background(), root, "cache/demux.go", opening)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := BuildContext(context.Background(), root, "cache/demux.go")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(observed, fresh) {
		t.Fatalf("a build opened on the loader's observation differs from a fresh build:\n%+v\n%+v", observed.DirtyPaths, fresh.DirtyPaths)
	}
	if _, err := os.Stat(SnapshotDirectory(root)); err != nil {
		t.Fatalf("IDX-SNAP-V0-005: the loader touched the snapshot directory: %v", err)
	}
}
