//go:build darwin || linux

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0154_RetirementSweepsABacklogBeyondOneWindow plants more attempt
// directories than one pass examines, a quarter of them empty so a window can
// hold nothing removable. Every pass examines at most maxRunAttemptScan
// attempts and makes at most one terminal lookup, and within the stated
// number of passes every ended run of a terminal attempt is retired except
// the newest maxRetainedEndedRuns of all.
func TestATRV0154_RetirementSweepsABacklogBeyondOneWindow(t *testing.T) {
	scan, retain := maxRunAttemptScan, maxRetainedEndedRuns
	maxRunAttemptScan, maxRetainedEndedRuns = 6, 2
	t.Cleanup(func() { maxRunAttemptScan, maxRetainedEndedRuns = scan, retain })
	root := filepath.Join(t.TempDir(), "taskman-runs")
	old := time.Now().Add(-2 * time.Hour)
	sum, status, pid := strings.Repeat("0", 64), 0, os.Getpid()
	ended := "2026-10-20T00:00:01Z"
	const runs, empty = 30, 10
	terminal := map[string]*snapshot.Attempt{}
	for i := 0; i < runs; i++ {
		id := fmt.Sprintf("attempt-%02d", i)
		terminal[id] = &snapshot.Attempt{Phase: "COMPLETED", Generation: wire.SizeOf(1)}
		runID := fmt.Sprintf("%016x", i+1)
		dir := filepath.Join(root, attemptRunsName(id), runID)
		raw, err := encodeRunRecord(&runRecord{Profile: "taskman-attempt-run-record/0", RunID: runID, AttemptID: id, Generation: "1",
			ArgvSha256: sum, TimeoutSeconds: 60, State: runFinished, LaunchedAt: "2026-10-20T00:00:00Z", SupervisorPid: pid,
			SupervisorIdentity: "gone", CommandPid: &pid, EndedAt: &ended, ExitStatus: &status, ResultSha256: &sum})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, runRecordName), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		at := old.Add(-time.Duration(i) * time.Minute)
		for _, p := range []string{filepath.Join(dir, runRecordName), dir} {
			if err := os.Chtimes(p, at, at); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < empty; i++ {
		if err := os.MkdirAll(filepath.Join(root, attemptRunsName(fmt.Sprintf("empty-%02d", i))), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	lookups := 0
	records := func(_ context.Context, ids []string) (map[string]*snapshot.Attempt, error) {
		lookups++
		if len(ids) > maxRunAttemptScan {
			t.Fatalf("one lookup named %d attempts", len(ids))
		}
		return terminal, nil
	}
	left := func() []string {
		t.Helper()
		var out []string
		for i := 0; i < runs; i++ {
			if _, err := os.Stat(filepath.Join(root, attemptRunsName(fmt.Sprintf("attempt-%02d", i)))); err == nil {
				out = append(out, fmt.Sprintf("attempt-%02d", i))
			}
		}
		sort.Strings(out)
		return out
	}
	want := []string{"attempt-00", "attempt-01"}
	// Stated bound: each pass reads at least maxRunAttemptScan less the
	// carried attempts of names it has not examined this sweep.
	bound := (runs+empty+maxRunAttemptScan-maxRetainedEndedRuns-1)/(maxRunAttemptScan-maxRetainedEndedRuns) + 1
	passes := 0
	for ; passes < bound && fmt.Sprint(left()) != fmt.Sprint(want); passes++ {
		window, _, ok := runRetireWindow(root)
		if !ok || len(window) > maxRunAttemptScan {
			t.Fatalf("pass %d window %d names (ok %v), bound %d", passes, len(window), ok, maxRunAttemptScan)
		}
		before := lookups
		retireEndedRunsIn(root, records)
		if lookups-before > 1 {
			t.Fatalf("pass %d made %d lookups", passes, lookups-before)
		}
	}
	if got := left(); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("after %d passes (bound %d) left %v, want %v", passes, bound, got, want)
	}
	for i := 0; i < empty; i++ {
		if _, err := os.Stat(filepath.Join(root, attemptRunsName(fmt.Sprintf("empty-%02d", i)))); err != nil {
			t.Fatalf("an attempt directory without ended runs was removed: %v", err)
		}
	}
	t.Logf("%d attempt directories in %d-name windows retired in %d passes (bound %d)", runs+empty, maxRunAttemptScan, passes, bound)

	// A cursor that does not read back as attempt names starts over.
	if err := os.WriteFile(filepath.Join(root, runRetireCursorName), []byte(`{"after":"../x","carry":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if cur := readRunRetireCursor(root); cur.After != "" || cur.Carry != nil {
		t.Fatalf("malformed cursor read as %+v", cur)
	}
}
