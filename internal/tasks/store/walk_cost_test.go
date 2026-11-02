//go:build darwin || linux

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
//
// V1-2186 (LGL-V0-804): each run also splits its wall time into decode CPU
// and I/O wait, because a CPU profiler can blame openat for CPU work. CPU
// is the process's user and system time and the Go runtime's mutator (user)
// and GC CPU; I/O wait is sampled: every CORVINT_TASKS_WALK_COST_IO_SAMPLE_MS
// (default 2, 0 disables) a goroutine reads each thread's state from
// /proc/self/task and counts the threads in uninterruptible (disk) sleep, so
// ioWaitMs estimates thread time blocked on I/O; on a host without
// /proc it is NOT_OBSERVED. The report names the command, the binary's
// SHA-256 and the CPU count, so an A/B of two binaries over one fixture is
// reproducible (docs/BENCHMARK-HOST.md).
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
	sampleEvery := 2 * time.Millisecond
	if raw := os.Getenv("CORVINT_TASKS_WALK_COST_IO_SAMPLE_MS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			t.Fatalf("CORVINT_TASKS_WALK_COST_IO_SAMPLE_MS=%q", raw)
		}
		sampleEvery = time.Duration(n) * time.Millisecond
	}
	loadBefore := hostLoad()
	var ms []float64
	var splits []walkSplit
	for i := 0; i < runs; i++ {
		before := probeWalk()
		sampler := startIOSampler(sampleEvery)
		start := time.Now()
		proof, err := journalReader(repo, head).Audit()
		took := time.Since(start)
		io := sampler.stop()
		if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
			t.Fatalf("run %d: %+v %v", i, proof, err)
		}
		split := probeWalk().since(before, took, io)
		ms = append(ms, split.WallMs)
		splits = append(splits, split)
		t.Logf("run %d: %v cpu=%.0fms goUser=%.0fms goGC=%.0fms ioWait=%v", i, took, split.CPUMs, split.GoUserCPUMs, split.GoGCCPUMs, split.IOWaitMs)
	}
	sorted := append([]float64(nil), ms...)
	sort.Float64s(sorted)
	report := map[string]any{"receipts": receipts, "runs": ms, "medianMs": sorted[len(sorted)/2], "fixtureBuildMs": built.Milliseconds(), "hostLoadBefore": loadBefore, "hostLoadAfter": hostLoad(),
		"split": splits, "medianSplit": medianSplit(splits), "ioSampleMs": sampleEvery.Milliseconds(), "command": walkCommand(), "binarySha256": binarySum(), "cpus": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "goVersion": runtime.Version()}
	raw, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", raw)
}
