//go:build darwin || linux

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0171_RetryDuringRetirementKeepsItsRuns is the V1-0933 regression.
// The pass's terminal read finds three attempts terminal; a retry then revives
// one of them before the removal. The interleaving runs inside the second
// read, after the runs have left their attempt directories. The revived
// attempt's run is moved back, a run whose attempt is still terminal (at a
// later generation) is removed, and the newest run is kept. A failed second
// read keeps every run, and a run hidden by a pass that stopped is restored
// by the next pass. Before the fix the revived attempt's run was removed.
func TestATRV0171_RetryDuringRetirementKeepsItsRuns(t *testing.T) {
	retain := maxRetainedEndedRuns
	maxRetainedEndedRuns = 1
	t.Cleanup(func() { maxRetainedEndedRuns = retain })
	old := time.Now().Add(-2 * time.Hour)
	ids := []string{"attempt-0", "attempt-1", "attempt-2"}
	runDir := func(root string, i int) string {
		return filepath.Join(root, attemptRunsName(ids[i]), fmt.Sprintf("%016x", i+1))
	}
	plant := func(t *testing.T) string {
		root := filepath.Join(t.TempDir(), "taskman-runs")
		for i, id := range ids {
			plantEndedRun(t, root, id, i+1, old.Add(-time.Duration(i)*time.Minute))
		}
		return root
	}
	present := func(root string, i int) bool {
		_, err := os.Stat(filepath.Join(runDir(root, i), runRecordName))
		return err == nil
	}
	terminal := func(gen uint64) *snapshot.Attempt {
		return &snapshot.Attempt{Phase: "COMPLETED", Generation: wire.SizeOf(gen)}
	}

	t.Run("retry between the reads", func(t *testing.T) {
		root := plant(t)
		reads := 0
		records := func(_ context.Context, got []string) (map[string]*snapshot.Attempt, error) {
			reads++
			if reads == 1 {
				return map[string]*snapshot.Attempt{ids[0]: terminal(1), ids[1]: terminal(1), ids[2]: terminal(1)}, nil
			}
			// The retry lands here: attempt-2 is live at generation 2, and
			// attempt-1 was retried and ended again at generation 2.
			if present(root, 1) || present(root, 2) {
				t.Fatalf("second read began before the runs left their attempts (read names %v)", got)
			}
			return map[string]*snapshot.Attempt{ids[1]: terminal(2), ids[2]: {Phase: "RUNNING", Generation: wire.SizeOf(2)}}, nil
		}
		retireEndedRunsIn(root, records)
		if reads != 2 || !present(root, 0) || present(root, 1) || !present(root, 2) {
			t.Fatalf("reads %d; kept newest %v, retired terminal %v, kept revived %v", reads, present(root, 0), !present(root, 1), present(root, 2))
		}
		if left, _ := filepath.Glob(filepath.Join(root, ".retir[ei][dn]*-*")); len(left) != 0 {
			t.Fatalf("pass left %v", left)
		}
	})

	t.Run("failed second read", func(t *testing.T) {
		root := plant(t)
		reads := 0
		records := func(context.Context, []string) (map[string]*snapshot.Attempt, error) {
			reads++
			if reads == 1 {
				return map[string]*snapshot.Attempt{ids[0]: terminal(1), ids[1]: terminal(1), ids[2]: terminal(1)}, nil
			}
			return nil, errors.New("store unreadable")
		}
		retireEndedRunsIn(root, records)
		for i := range ids {
			if !present(root, i) {
				t.Fatalf("run of %s removed after a failed second read", ids[i])
			}
		}
	})

	t.Run("pass stopped while checking", func(t *testing.T) {
		root := plant(t)
		hidden := filepath.Join(root, retiringRunName(filepath.Dir(runDir(root, 2)), runDir(root, 2)))
		if err := os.Rename(runDir(root, 2), hidden); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Dir(runDir(root, 2))); err != nil {
			t.Fatal(err)
		}
		none := func(context.Context, []string) (map[string]*snapshot.Attempt, error) { return nil, nil }
		retireEndedRunsIn(root, none)
		if !present(root, 2) {
			t.Fatal("a hidden run was not restored")
		}
		if _, err := os.Lstat(hidden); !os.IsNotExist(err) {
			t.Fatalf("hidden name left: %v", err)
		}
	})
}
