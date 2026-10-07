package contextindex

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// IDX-SNAP-V0-027 (proposed): an index write to the shared store removes the
// worktree's superseded `.corvint/index` files it recognises, names each one,
// leaves the store it wrote untouched, and removes the directory once empty.
func TestIndexWriteSweepsTheSupersededWorktreeStore(t *testing.T) {
	index := taskContextFixture(t)
	legacy := filepath.Join(index.Root, ".corvint", "index")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	tree := strings.Repeat("a", 40)
	body := bytes.Repeat([]byte{7}, 1<<20)
	stale := time.Now().Add(-2 * snapshotTemporaryStaleAfter)
	seeded := map[string][]byte{
		"sha1-" + tree + "-0123456789abcdef.gob":  body,
		"sha1-" + tree + "-0123456789abcdef.sect": body[:4096],
		"sha1-" + tree + "-fedcba9876543210.aip":  body[:2048],
		buildCostName:                             []byte(`{"format":"corvint-index-build-cost/0","buildMilliseconds":5}`),
		"snapshot-abandoned.tmp":                  body[:512],
		".gitignore":                              []byte("*\n"),
	}
	var seededBytes int64
	for name, content := range seeded {
		path := filepath.Join(legacy, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, stale, stale); err != nil {
			t.Fatal(err)
		}
		seededBytes += int64(len(content))
	}
	receipt, err := WriteSnapshot(index)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.StoreShared || receipt.LegacyStore != legacy {
		t.Fatalf("store %q shared=%t legacy %q", receipt.Store, receipt.StoreShared, receipt.LegacyStore)
	}
	if len(receipt.LegacyLeft) != 0 {
		t.Fatalf("left %+v", receipt.LegacyLeft)
	}
	var names []string
	var reclaimed int64
	for _, removed := range receipt.LegacyRemoved {
		names = append(names, filepath.Base(removed.Path)+":"+removed.Kind)
		reclaimed += removed.Bytes
	}
	sort.Strings(names)
	want := []string{
		".gitignore:ignore", "build-cost.json:build-cost", "index:directory", "sha1-" + tree + "-0123456789abcdef.gob:snapshot",
		"sha1-" + tree + "-0123456789abcdef.sect:sectioned", "sha1-" + tree + "-fedcba9876543210.aip:pack", "snapshot-abandoned.tmp:temporary",
	}
	if strings.Join(names, ",") != strings.Join(want, ",") || reclaimed != seededBytes {
		t.Fatalf("removed %v (%d bytes), want %v (%d bytes)", names, reclaimed, want, seededBytes)
	}
	t.Logf("legacy store reclaimed %d bytes in %d files", reclaimed, len(receipt.LegacyRemoved)-1)
	if _, err := os.Lstat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy directory survived: %v", err)
	}
	if _, err := os.Stat(receipt.Path); err != nil {
		t.Fatalf("the shared store's snapshot was touched: %v", err)
	}
	if loaded, hit, err := LoadSnapshot(t.Context(), index.Root); err != nil || !hit || loaded.Revision != index.Revision {
		t.Fatalf("shared snapshot no longer loads: hit=%t err=%v", hit, err)
	}
	again, err := WriteSnapshot(index)
	if err != nil || again.LegacyStore != "" || again.LegacyRemoved != nil {
		t.Fatalf("a write with no legacy store reported one: %+v %v", again, err)
	}
}

// IDX-SNAP-V0-027 (proposed): anything the sweep does not recognise, a link,
// a fresh temporary or a directory, is left and named, with the ignore file
// and the directory kept; a linked store is never followed.
func TestIndexWriteLeavesUnrecognisedLegacyEntries(t *testing.T) {
	t.Run("unexpected entries", func(t *testing.T) {
		index := taskContextFixture(t)
		legacy := filepath.Join(index.Root, ".corvint", "index")
		if err := os.MkdirAll(filepath.Join(legacy, "blobs"), 0o755); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "keep.gob")
		if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot := filepath.Join(legacy, "sha1-"+strings.Repeat("b", 40)+"-0123456789abcdef.gob")
		for path, content := range map[string]string{
			snapshot:                           "old",
			filepath.Join(legacy, "notes.txt"): "user file",
			filepath.Join(legacy, "snapshot-live.tmp"): "in flight",
			filepath.Join(legacy, ".gitignore"):        "*\n",
		} {
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		linked := filepath.Join(legacy, "sha1-"+strings.Repeat("c", 40)+"-0123456789abcdef.gob")
		if err := os.Symlink(outside, linked); err != nil {
			t.Fatal(err)
		}
		receipt, err := WriteSnapshot(index)
		if err != nil {
			t.Fatal(err)
		}
		if len(receipt.LegacyRemoved) != 1 || receipt.LegacyRemoved[0].Path != snapshot {
			t.Fatalf("removed %+v", receipt.LegacyRemoved)
		}
		reasons := map[string]string{}
		for _, entry := range receipt.LegacyLeft {
			reasons[filepath.Base(entry.Path)] = entry.Reason
		}
		want := map[string]string{
			"blobs": "not a regular file", "notes.txt": "unrecognised entry",
			"snapshot-live.tmp": "writer temporary not yet stale", filepath.Base(linked): "not a regular file",
		}
		if len(reasons) != len(want) {
			t.Fatalf("left %+v, want %+v", reasons, want)
		}
		for name, reason := range want {
			if reasons[name] != reason {
				t.Fatalf("left %+v, want %+v", reasons, want)
			}
		}
		for _, kept := range []string{outside, filepath.Join(legacy, "notes.txt"), filepath.Join(legacy, ".gitignore"), linked} {
			if _, err := os.Lstat(kept); err != nil {
				t.Fatalf("%s was removed: %v", kept, err)
			}
		}
	})
	t.Run("linked store", func(t *testing.T) {
		index := taskContextFixture(t)
		outside := t.TempDir()
		victim := filepath.Join(outside, "sha1-"+strings.Repeat("d", 40)+"-0123456789abcdef.gob")
		if err := os.WriteFile(victim, []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(index.Root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		legacy := filepath.Join(index.Root, ".corvint", "index")
		if err := os.Symlink(outside, legacy); err != nil {
			t.Fatal(err)
		}
		receipt, err := WriteSnapshot(index)
		if err != nil {
			t.Fatal(err)
		}
		if len(receipt.LegacyRemoved) != 0 || len(receipt.LegacyLeft) != 1 || receipt.LegacyLeft[0].Reason != "not a real directory" {
			t.Fatalf("linked legacy store: removed %+v left %+v", receipt.LegacyRemoved, receipt.LegacyLeft)
		}
		if _, err := os.Stat(victim); err != nil {
			t.Fatalf("a file behind the linked store was removed: %v", err)
		}
	})
}
