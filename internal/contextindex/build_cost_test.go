package contextindex

import (
	"fmt"
	"math"
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

// IDX-SNAP-V0-012: a record longer than any Duration the writer could measure is not well
// formed, and a temporary a killed writer left beside the snapshots is swept once stale.
func TestBuildCostRefusesAnOverflowingRecordAndSweepsItsTemporary(t *testing.T) {
	index := taskContextFixture(t)
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	directory := SnapshotDirectory(index.Root)
	overflowing := fmt.Sprintf(`{"format":%q,"buildMilliseconds":%d}`, buildCostFormat, int64(math.MaxInt64))
	if err := os.WriteFile(filepath.Join(directory, buildCostName), []byte(overflowing), 0o644); err != nil {
		t.Fatal(err)
	}
	if cost, ok := RecordedBuildCost(index.Root); ok {
		t.Fatalf("an overflowing build-cost record was read: %s", cost)
	}
	abandoned := filepath.Join(directory, "blob-abandoned.tmp")
	stale := time.Now().Add(-2 * snapshotTemporaryStaleAfter)
	if err := os.WriteFile(abandoned, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(abandoned, stale, stale); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSnapshot(index); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(abandoned); !os.IsNotExist(err) {
		t.Fatalf("a stale build-cost temporary survived the writer: %v", err)
	}
}
