package contextindex

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// IDX-SNAP-V0-012: the build-cost record needs an existing store, round-trips at millisecond
// resolution, survives the writer's eviction, and is not read through a symlink.
func TestBuildCostRecordRoundTripsBesideTheSnapshots(t *testing.T) {
	index := taskContextFixture(t)
	if err := RecordBuildCost(index.Root, time.Second); err == nil {
		t.Fatal("a build cost was recorded without a snapshot store")
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	if err := RecordBuildCost(index.Root, 1234*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	if cost, ok := RecordedBuildCost(index.Root); !ok || cost != 1234*time.Millisecond {
		t.Fatalf("recorded build cost = %s, %t", cost, ok)
	}
	record := filepath.Join(SnapshotDirectory(index.Root), buildCostName)
	outside := filepath.Join(t.TempDir(), buildCostName)
	if err := os.Rename(record, outside); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, record); err != nil {
		t.Fatal(err)
	}
	if cost, ok := RecordedBuildCost(index.Root); ok {
		t.Fatalf("a linked build-cost record was read: %s", cost)
	}
}
