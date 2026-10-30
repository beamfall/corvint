//go:build darwin || linux

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// V1-1701: the complete journal walk's wall time on a large synthetic store.
// An opt-in measurement harness, not a timing assertion; a plain `go test`
// skips it. CORVINT_TASKS_WALK_COST names the JSON report; the store is the
// historyStore fixture built (or reused, when it already exists) at
// CORVINT_TASKS_WALK_COST_STORE with CORVINT_TASKS_WALK_COST_RECEIPTS
// receipts. Each run is one complete Audit (ModeFull) that must agree; the
// report keeps every run, the median, the host load around the runs and,
// with CORVINT_TASKS_WALK_COST_PROFILE, a CPU profile of all runs.
func TestV11701_WalkCost(t *testing.T) {
	output := os.Getenv("CORVINT_TASKS_WALK_COST")
	if output == "" {
		t.Skip("set CORVINT_TASKS_WALK_COST to a report path")
	}
	dir := os.Getenv("CORVINT_TASKS_WALK_COST_STORE")
	if dir == "" {
		t.Fatal("set CORVINT_TASKS_WALK_COST_STORE to a fixture directory")
	}
	receipts, runs := 47000, 5
	if raw := os.Getenv("CORVINT_TASKS_WALK_COST_RECEIPTS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < historyTickets+2 {
			t.Fatalf("CORVINT_TASKS_WALK_COST_RECEIPTS=%q", raw)
		}
		receipts = n
	}
	if raw := os.Getenv("CORVINT_TASKS_WALK_COST_RUNS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			t.Fatalf("CORVINT_TASKS_WALK_COST_RUNS=%q", raw)
		}
		runs = n
	}
	root := filepath.Join(dir, "repo")
	built := time.Duration(0)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		start := time.Now()
		historyStoreAt(t, root, historyTickets, receipts)
		built = time.Since(start)
	}
	repo, err := intent.Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	if head.LastSeq.Uint64() != uint64(receipts) {
		t.Fatalf("fixture head %s, want %d", head.LastSeq, receipts)
	}
	if path := os.Getenv("CORVINT_TASKS_WALK_COST_PROFILE"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			t.Fatal(err)
		}
		defer func() { pprof.StopCPUProfile(); f.Close() }()
	}
	loadBefore := hostLoad()
	var ms []float64
	for i := 0; i < runs; i++ {
		start := time.Now()
		proof, err := journalReader(repo, head).Audit()
		took := time.Since(start)
		if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
			t.Fatalf("run %d: %+v %v", i, proof, err)
		}
		ms = append(ms, float64(took.Microseconds())/1000)
		t.Logf("run %d: %v", i, took)
	}
	sorted := append([]float64(nil), ms...)
	sort.Float64s(sorted)
	report := map[string]any{"receipts": receipts, "runs": ms, "medianMs": sorted[len(sorted)/2], "fixtureBuildMs": built.Milliseconds(), "hostLoadBefore": loadBefore, "hostLoadAfter": hostLoad()}
	raw, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", raw)
}
