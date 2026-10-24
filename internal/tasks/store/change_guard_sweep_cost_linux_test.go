//go:build linux

package store

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// PSR-V0-223 (V1-2482): the cost of re-reading the stat identity of every
// store file. Opt-in: CORVINT_TASKS_GUARD_SWEEP_COST lists receipt counts, for
// example "10000,45000". For each it builds the historyStoreAt fixture,
// registers one guard and logs the median and maximum of 21 Check (poll and
// sweep of every file), Sweep and CheckEvents (poll and the files with another
// link, under the writer lock) calls, with the store's regular-file count.
func TestPSRV0223_ChangeGuardSweepCost(t *testing.T) {
	spec := os.Getenv("CORVINT_TASKS_GUARD_SWEEP_COST")
	if spec == "" {
		t.Skip("set CORVINT_TASKS_GUARD_SWEEP_COST=10000,45000 to measure")
	}
	for _, field := range strings.Split(spec, ",") {
		receipts, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || receipts < historyTickets+2 {
			t.Fatalf("CORVINT_TASKS_GUARD_SWEEP_COST entry %q", field)
		}
		t.Run(field, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			repo := historyStoreAt(t, filepath.Join(base, "repo"), historyTickets, receipts)
			files := 0
			for _, root := range []string{repo.StateDir, filepath.Join(repo.IntentRoot(), intent.Dir)} {
				err := filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
					if err == nil && d.Type().IsRegular() {
						files++
					}
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			register := time.Now()
			g, err := authority.WatchChanges(repo)
			if err != nil {
				t.Fatal(err)
			}
			defer g.Close()
			registered := time.Since(register)
			measure := func(f func() error) (time.Duration, time.Duration) {
				t.Helper()
				var runs []time.Duration
				for i := 0; i < 21; i++ {
					start := time.Now()
					if err := f(); err != nil {
						t.Fatal(err)
					}
					runs = append(runs, time.Since(start))
				}
				sort.Slice(runs, func(i, j int) bool { return runs[i] < runs[j] })
				return runs[len(runs)/2], runs[len(runs)-1]
			}
			checkMed, checkMax := measure(g.Check)
			sweepMed, sweepMax := measure(func() error { g.Sweep(); return nil })
			eventsMed, eventsMax := measure(g.CheckEvents)
			t.Logf("receipts %d, regular files %d: register %v; Check median %v max %v; Sweep median %v max %v; CheckEvents median %v max %v",
				receipts, files, registered, checkMed, checkMax, sweepMed, sweepMax, eventsMed, eventsMax)
		})
	}
}
