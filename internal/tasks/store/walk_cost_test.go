//go:build darwin || linux

package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"syscall"
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
// with CORVINT_TASKS_WALK_COST_PROFILE, a CPU profile of all runs. Beside
// each wall time it records the run's process CPU (user plus system, every
// thread, so the read-ahead decode counts wherever it ran) and, apart from it,
// its I/O: the blocking input operations rusage counts (0 for a run served
// from the page cache) and, on Linux with kernel.task_delayacct=1, the time
// the process waited on block I/O; a host without that accounting records
// NOT_OBSERVED, never 0 (JRT-V0-532, V1-2186). A CPU profile alone cannot
// separate them: it attributes a blocking openat or read to Syscall6.
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
	var ms, cpuMs []float64
	var inBlocks []int64
	var ioDelayMs []any
	for i := 0; i < runs; i++ {
		cpuBefore, ok := processCPU()
		if !ok {
			t.Fatal("process CPU time is not observable on this host")
		}
		blocksBefore, delayBefore := walkInBlocks(t), walkIODelay()
		start := time.Now()
		proof, err := journalReader(repo, head).Audit()
		took := time.Since(start)
		cpuAfter, _ := processCPU()
		cpu := cpuAfter - cpuBefore
		inBlocks = append(inBlocks, walkInBlocks(t)-blocksBefore)
		if delayAfter := walkIODelay(); delayBefore >= 0 && delayAfter >= 0 {
			ioDelayMs = append(ioDelayMs, float64((delayAfter-delayBefore).Microseconds())/1000)
		} else {
			ioDelayMs = append(ioDelayMs, "NOT_OBSERVED")
		}
		if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
			t.Fatalf("run %d: %+v %v", i, proof, err)
		}
		ms = append(ms, float64(took.Microseconds())/1000)
		cpuMs = append(cpuMs, float64(cpu.Microseconds())/1000)
		t.Logf("run %d: wall %v cpu %v in-blocks %d io-delay %v", i, took, cpu, inBlocks[i], ioDelayMs[i])
	}
	report := map[string]any{"receipts": receipts, "runs": ms, "medianMs": walkMedian(ms), "cpuMs": cpuMs, "medianCpuMs": walkMedian(cpuMs), "inBlocks": inBlocks, "ioDelayMs": ioDelayMs, "fixtureBuildMs": built.Milliseconds(), "hostLoadBefore": loadBefore, "hostLoadAfter": hostLoad()}
	raw, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(output, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", raw)
}

func walkMedian(v []float64) float64 {
	sorted := append([]float64(nil), v...)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

// walkInBlocks is the process's count of blocking input operations so far.
func walkInBlocks(t *testing.T) int64 {
	t.Helper()
	var u syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &u); err != nil {
		t.Fatal(err)
	}
	return int64(u.Inblock)
}

// walkIODelay is the process's aggregate block I/O delay (field 42 of
// /proc/self/stat, in USER_HZ = 100 ticks per second), or -1 where it is not
// observable: not Linux, or delay accounting is off so the field stays 0.
func walkIODelay() time.Duration {
	if on, err := os.ReadFile("/proc/sys/kernel/task_delayacct"); err != nil || strings.TrimSpace(string(on)) != "1" {
		return -1
	}
	raw, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return -1
	}
	i := strings.LastIndexByte(string(raw), ')')
	fields := strings.Fields(string(raw[i+1:]))
	if i < 0 || len(fields) < 40 {
		return -1
	}
	ticks, err := strconv.ParseInt(fields[39], 10, 64)
	if err != nil {
		return -1
	}
	return time.Duration(ticks) * 10 * time.Millisecond
}
