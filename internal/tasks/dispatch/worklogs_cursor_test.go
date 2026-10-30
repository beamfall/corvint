//go:build darwin || linux

package dispatch

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// TestCALV0352_RetirementSweepsABacklogBeyondOneWindow plants more worker
// directories than one pass examines, a third of them inside the quiet
// window so a window can hold nothing removable. Every pass reads at most
// maxWorkerDirScan names (its window plus the carried retained ones), and
// within two passes per window every eligible directory is retired, leaving
// exactly the newest maxRetainedWorkerDirs of all plus the quiet ones.
func TestCALV0352_RetirementSweepsABacklogBeyondOneWindow(t *testing.T) {
	scan, retain := maxWorkerDirScan, maxRetainedWorkerDirs
	maxWorkerDirScan, maxRetainedWorkerDirs = 10, 3
	t.Cleanup(func() { maxWorkerDirScan, maxRetainedWorkerDirs = scan, retain })
	d, err := Open("prog", testConfig(t, "exit 0"), &fakeQueue{}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	now := time.Now()
	const old, young = 40, 20
	for i := 0; i < old; i++ {
		plantQuiet(t, d, fmt.Sprintf("old-%02d", i), "", now.Add(-2*time.Hour-time.Duration(i)*time.Minute))
	}
	for i := 0; i < young; i++ {
		plantQuiet(t, d, fmt.Sprintf("young-%02d", i), "", now)
	}
	window := maxWorkerDirScan - maxRetainedWorkerDirs
	windows := (old + young + window - 1) / window
	bound := 2*windows + 1 // stated bound: two passes per window, plus one
	left := func() []string {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join(d.dir, "workers"))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range entries {
			if e.Name() != workerRetireMarks {
				out = append(out, e.Name())
			}
		}
		sort.Strings(out)
		return out
	}
	want := []string{"old-00", "old-01", "old-02"}
	for i := 0; i < young; i++ {
		want = append(want, fmt.Sprintf("young-%02d", i))
	}
	at := now
	passes := 0
	for ; passes < bound && fmt.Sprint(left()) != fmt.Sprint(want); passes++ {
		d.Now = func() time.Time { return at }
		d.retireWorkerDirs()
		if len(d.retireWindow) > maxWorkerDirScan || len(d.retireCarry) > maxRetainedWorkerDirs {
			t.Fatalf("pass %d held window %d names, carried %d, bound %d", passes, len(d.retireWindow), len(d.retireCarry), maxWorkerDirScan)
		}
		if d.retireConfirm.IsZero() && fmt.Sprint(left()) != fmt.Sprint(want) {
			t.Fatalf("pass %d scheduled no further pass with a backlog left: %v", passes, left())
		}
		at = at.Add(workerRetireConfirm)
	}
	if got := left(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after %d passes (bound %d) left %v, want %v", passes, bound, got, want)
	}
	t.Logf("%d directories in %d-name windows retired in %d passes (bound %d)", old+young, window, passes, bound)
}
