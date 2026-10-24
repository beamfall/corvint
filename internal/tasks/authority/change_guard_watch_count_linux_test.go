//go:build linux

package authority

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// inotifyWatchesHeld counts the inotify watches this process holds, from the
// "inotify wd:" lines of /proc/self/fdinfo, which is the count the kernel
// charges against fs.inotify.max_user_watches.
func inotifyWatchesHeld(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fdinfo")
	if err != nil {
		t.Skipf("fdinfo is not readable: %v", err)
	}
	n := 0
	for _, e := range entries {
		raw, err := os.ReadFile(filepath.Join("/proc/self/fdinfo", e.Name()))
		if err == nil {
			n += strings.Count(string(raw), "inotify wd:")
		}
	}
	return n
}

// PSR-V0-222, PSR-V0-223 (V1-2482): the watches one guard holds do not grow
// with the number of files in the store, which are tracked by stat instead. A
// store of 11 files and the same store with 4,990 more receipt-like files in
// the same directories take the same count, the directories and their
// ancestors.
func TestPSRV0222_ChangeGuardWatchCountIsBoundedByDirectories(t *testing.T) {
	repo, file := guardRepo(t)
	receipts := filepath.Join(repo.StateDir, "receipts")
	if err := os.MkdirAll(receipts, 0700); err != nil {
		t.Fatal(err)
	}
	tracked := 0
	held := func() int {
		t.Helper()
		before := inotifyWatchesHeld(t)
		g, err := WatchChanges(repo)
		if err != nil {
			t.Fatal(err)
		}
		defer g.Close()
		// Every file is tracked by its stat identity instead (PSR-V0-223),
		// and none is re-read under the writer lock.
		if w := g.watch.(*notifyWatch); len(w.uncovered) != 0 || len(w.files) < tracked {
			t.Fatalf("%d files tracked, %d uncovered, want at least %d and none", len(w.files), len(w.uncovered), tracked)
		}
		return inotifyWatchesHeld(t) - before
	}
	grow := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			for _, dir := range []string{receipts, filepath.Dir(file)} {
				path := filepath.Join(dir, fmt.Sprintf("r%06d.json", i))
				if _, err := os.Lstat(path); err == nil {
					continue
				}
				if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	grow(5)
	tracked = 11
	small := held()
	grow(2500)
	tracked = 5001
	large := held()
	if small != large {
		t.Fatalf("a guard holds %d watches over 11 files and %d over 5,001: the count grows with the files", small, large)
	}
	// ancestors of the two roots, the state directory, receipts, the intent
	// directory and tickets: far below one watch per file.
	if large > 64 {
		t.Fatalf("a guard holds %d watches over a store of 4 directories", large)
	}
	t.Logf("watches per guard: %d over 11 files, %d over 5,001 files", small, large)
}
