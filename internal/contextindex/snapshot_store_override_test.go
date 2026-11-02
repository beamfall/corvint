package contextindex

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/gitstatus"
)

// TestIndexStoreOverride_IDXSNAPV0808: unset or empty CORVINT_INDEX_STORE keeps
// the shared store under the Git common directory; set, every write and read
// uses that directory and the shared store stays empty; a symlinked override
// is refused like a symlinked store (IDX-SNAP-V0-808).
func TestIndexStoreOverride_IDXSNAPV0808(t *testing.T) {
	index := taskContextFixture(t)
	common, err := gitstatus.CommonDirectory(index.Root)
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(common, filepath.FromSlash(sharedSnapshotSubpath))
	for _, value := range []string{"", "unset"} {
		if value == "unset" {
			os.Unsetenv("CORVINT_" + indexStoreSuffix)
		} else {
			t.Setenv("CORVINT_"+indexStoreSuffix, value)
		}
		if got := SnapshotDirectory(index.Root); got != shared {
			t.Fatalf("CORVINT_INDEX_STORE %q: store %s, want the shared %s", value, got, shared)
		}
	}

	scratch := filepath.Join(t.TempDir(), "store")
	t.Setenv("CORVINT_"+indexStoreSuffix, scratch)
	if got := SnapshotDirectory(index.Root); got != scratch {
		t.Fatalf("override store %s, want %s", got, scratch)
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(receipt.Path) != scratch || receipt.StoreShared {
		t.Fatalf("snapshot written to %s (shared=%v), want %s", receipt.Path, receipt.StoreShared, scratch)
	}
	if entries, _ := os.ReadDir(shared); len(entries) != 0 {
		t.Fatalf("the shared store holds %d entries after an override write", len(entries))
	}
	if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || !hit {
		t.Fatalf("override read: hit=%v err=%v", hit, err)
	}
	t.Setenv("CORVINT_"+indexStoreSuffix, "")
	if _, hit, err := LoadSnapshot(context.Background(), index.Root); err != nil || hit {
		t.Fatalf("the shared store served an override write: hit=%v err=%v", hit, err)
	}

	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(scratch, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	t.Setenv("CORVINT_"+indexStoreSuffix, link)
	if _, err := WriteSnapshot(index); err == nil {
		t.Fatal("a symlinked override store was written")
	}
}
