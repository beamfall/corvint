package affected

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func evictionDirectory(t *testing.T, entries int) (string, *os.Root) {
	t.Helper()
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	for index := range entries {
		name := filepath.Join(dir, fmt.Sprintf("affected-graph-%03d.json", index))
		if err := os.WriteFile(name, []byte("entry"), 0o600); err != nil {
			t.Fatal(err)
		}
		// Index order is age order, except two entries share a time and so
		// fall back to name order.
		stamp := base.Add(time.Duration(index) * time.Second)
		if index == 1 {
			stamp = base
		}
		if err := os.Chtimes(name, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return dir, root
}

func listNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// AFP-V0-970: past the cap, the oldest entries go first (ties by name), the
// entry just written stays even when it is the oldest, and a directory at or
// under the cap is left alone.
func TestAFPV0970EvictionKeepsTheNewestEntries(t *testing.T) {
	dir, root := evictionDirectory(t, GraphCacheMaxEntries)
	evictGraphCache(root, "affected-graph-000.json")
	if got := len(listNames(t, dir)); got != GraphCacheMaxEntries {
		t.Fatalf("a directory at the cap lost entries: %d", got)
	}
	dir, root = evictionDirectory(t, GraphCacheMaxEntries+3)
	evictGraphCache(root, "affected-graph-000.json")
	names := listNames(t, dir)
	if len(names) != GraphCacheMaxEntries {
		t.Fatalf("after eviction %d entries remain: %q", len(names), names)
	}
	for _, gone := range []string{"affected-graph-001.json", "affected-graph-002.json", "affected-graph-003.json"} {
		if slices.Contains(names, gone) {
			t.Fatalf("%s should have been evicted: %q", gone, names)
		}
	}
	if !slices.Contains(names, "affected-graph-000.json") || !slices.Contains(names, "affected-graph-004.json") {
		t.Fatalf("the kept entry or a newer one was evicted: %q", names)
	}
}

// AFP-V0-970: eviction removes only regular files named like an entry, through
// the directory handle: a symbolic link, a subdirectory and other names are
// left, and the link's target outside the directory is untouched.
func TestAFPV0970EvictionLeavesOtherFiles(t *testing.T) {
	dir, root := evictionDirectory(t, GraphCacheMaxEntries+1)
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(outside, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "affected-graph-link.json")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "affected-graph-dir.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".affected-graph-0.tmp", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	evictGraphCache(root, "affected-graph-032.json")
	names := listNames(t, dir)
	for _, kept := range []string{"affected-graph-link.json", "affected-graph-dir.json", ".affected-graph-0.tmp", "notes.txt"} {
		if !slices.Contains(names, kept) {
			t.Fatalf("eviction removed %s: %q", kept, names)
		}
	}
	if slices.Contains(names, "affected-graph-000.json") || len(names) != GraphCacheMaxEntries+4 {
		t.Fatalf("eviction removed the wrong set: %q", names)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "outside" {
		t.Fatalf("the link target outside the cache changed: %q %v", data, err)
	}
}
