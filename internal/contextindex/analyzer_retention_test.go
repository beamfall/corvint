package contextindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAnalyzerPacksStayBoundedAcrossTwelveTrees(t *testing.T) {
	t.Run("IDX-SNAP-V0-017", func(t *testing.T) {
		t.Setenv(snapshotFormatEnv, packFormatValue)
		root := taskContextFixture(t).Root
		var latest SnapshotReceipt
		for i := 0; i < 12; i++ {
			writeTestFile(t, root, "revision.txt", fmt.Sprintf("tree %d\n", i))
			testGit(t, root, "add", "revision.txt")
			testGit(t, root, "commit", "-qm", fmt.Sprintf("tree %d", i))
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			latest, err = WriteSnapshot(index)
			if err != nil {
				t.Fatal(err)
			}
			packs, err := filepath.Glob(filepath.Join(SnapshotDirectory(root), "*.aip"))
			if err != nil || len(packs) != min(i+1, snapshotKeep) {
				t.Fatalf("tree %d: %d packs, err=%v", i, len(packs), err)
			}
		}
		if _, err := os.Stat(latest.PackPath); err != nil {
			t.Fatal("current pack was evicted:", err)
		}
		if _, hit, _, err := LoadContextSnapshot(context.Background(), root); err != nil || !hit {
			t.Fatalf("current pack no longer loads: hit=%v err=%v", hit, err)
		}
	})
}

func TestAnalyzerPackEvictionReservesCurrentAndLeavesOtherFormats(t *testing.T) {
	t.Run("IDX-SNAP-V0-017", func(t *testing.T) {
		directory := t.TempDir()
		var current string
		for i := 0; i < 12; i++ {
			path := filepath.Join(directory, fmt.Sprintf("sha1-tree%02d-schema%02d.aip", i, i))
			if err := os.WriteFile(path, []byte("pack"), 0o644); err != nil {
				t.Fatal(err)
			}
			when := time.Unix(int64(i), 0)
			if err := os.Chtimes(path, when, when); err != nil {
				t.Fatal(err)
			}
			if i == 0 {
				current = path // Deliberately older than all other packs.
			}
		}
		for _, name := range []string{"legacy.gob", "legacy.sect", "snapshot-young.tmp"} {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if removed := evictAnalyzerPacks(directory, current, snapshotKeep); removed != 4 {
			t.Fatalf("evicted %d, want 4", removed)
		}
		packs, err := filepath.Glob(filepath.Join(directory, "*.aip"))
		if err != nil || len(packs) != snapshotKeep {
			t.Fatalf("pack count %d, err=%v", len(packs), err)
		}
		if _, err := os.Stat(current); err != nil {
			t.Fatal("old-clock current pack removed:", err)
		}
		for _, name := range []string{"legacy.gob", "legacy.sect", "snapshot-young.tmp"} {
			data, err := os.ReadFile(filepath.Join(directory, name))
			if err != nil || string(data) != name {
				t.Fatalf("unrelated %s modified: %v", name, err)
			}
		}
	})
}
