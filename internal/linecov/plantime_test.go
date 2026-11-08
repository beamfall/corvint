package linecov

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// Plan-time reference shape (MCJ-V0-019): an E2E-heavy index with segments
// present. Sizes are per the build-log entry of 2026-11-03.
const (
	ptFiles        = 300
	ptFuncs        = 60 // one block per function
	ptTests        = 2000
	ptSegsPerTest  = 8
	ptReachPerSeg  = 12
	ptTemplates    = 40 // distinct segment sequences, so classes are shared
	ptHunksChanged = 6
)

func ptPath(f int) string { return fmt.Sprintf("app/f%03d.go", f) }

func ptSource() []byte {
	var b strings.Builder
	b.WriteString("package app\n\n")
	for i := 0; i < ptFuncs; i++ {
		fmt.Fprintf(&b, "func F%d() int {\n\tv := %d\n\treturn v\n}\n\n", i, i)
	}
	return []byte(b.String())
}

// ptIndex builds the reference index directly: test t follows template
// t%ptTemplates, so every template's segments form classes of about
// ptTests/ptTemplates members with identical reaches.
func ptIndex() *Index {
	ix := &Index{Format: Format, Module: "example.com/m", Commit: "c0", Runner: "go test"}
	// The in-order record reaches no block: otherwise every change keeps e2e
	// whole for order (GLS-V0-008). Test t is ix.Tests[t+1].
	ix.Tests = append(ix.Tests, Test{Package: "e2e", Name: InOrder})
	ix.Segments = append(ix.Segments, Segment{Test: 0, Kind: KindTest})
	for t := 0; t < ptTests; t++ {
		ix.Tests = append(ix.Tests, Test{Package: "e2e", Name: fmt.Sprintf("Test%05d", t), Elapsed: time.Duration(1+t%97) * time.Millisecond, Level: "e2e"})
	}
	ix.Files = make([]File, ptFiles)
	for f := range ix.Files {
		ix.Files[f] = File{Path: ptPath(f), OID: "blob1"}
		for i := 0; i < ptFuncs; i++ {
			l := int32(3 + 5*i)
			ix.Files[f].Blocks = append(ix.Files[f].Blocks, Block{Start: l, StartCol: 15, End: l + 3, EndCol: 2})
		}
	}
	type ref struct{ f, b int }
	reach := func(tpl, seg int) []ref {
		var out []ref
		for k := 0; k < ptReachPerSeg; k++ {
			n := (tpl*ptSegsPerTest+seg)*ptReachPerSeg + k
			out = append(out, ref{(n * 7) % ptFiles, (n * 13) % ptFuncs})
		}
		return out
	}
	tests := map[ref]map[int32]bool{}
	segs := map[ref][]int32{}
	for t := 0; t < ptTests; t++ {
		tpl := t % ptTemplates
		for s := 0; s < ptSegsPerTest; s++ {
			id := int32(len(ix.Segments))
			ix.Segments = append(ix.Segments, Segment{Test: int32(t + 1), Kind: KindStep, ID: fmt.Sprintf("step:%d:%d", tpl, s), Bound: true, Units: []string{fmt.Sprintf("screen:%d", s)}, Elapsed: time.Millisecond})
			for _, r := range reach(tpl, s) {
				if tests[r] == nil {
					tests[r] = map[int32]bool{}
				}
				tests[r][int32(t+1)] = true
				segs[r] = append(segs[r], id)
			}
		}
	}
	for r, set := range tests {
		b := &ix.Files[r.f].Blocks[r.b]
		for t := range set {
			b.Tests = append(b.Tests, t)
		}
		sort.Slice(b.Tests, func(i, j int) bool { return b.Tests[i] < b.Tests[j] })
		b.Segs = uniqueSorted(segs[r])
	}
	ix.reindex()
	return ix
}

func ptChange() []FileDiff {
	var out []FileDiff
	for h := 0; h < ptHunksChanged; h++ {
		f, fn := (h*37)%ptFiles, (h*11)%ptFuncs
		l := 3 + 5*fn + 1
		out = append(out, FileDiff{Path: ptPath(f), OldOID: "blob1", NewOID: "blob2", NewSource: ptSource(), NewHunks: []Hunk{{Start: l, End: l}}, Hunks: []Hunk{{Start: l, End: l}}})
	}
	return out
}

// planOnce is everything a plan does beyond git diff and blob reads: decode
// the index, analyse the change, join it, run the shared-step shadow, narrow
// and report.
func planOnce(tb testing.TB, encoded []byte, diffs []FileDiff, read func(string) ([]byte, error), heads map[string][]string) PlanReport {
	ix, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		tb.Fatal(err)
	}
	e := ix.Analyze(diffs, read)
	if e.Fallback != "" {
		tb.Fatalf("fallback %s", e.Fallback)
	}
	plans, _ := ix.Narrow(nil, []string{"e2e"}, heads, Effect{}, e)
	j := ix.Join(e)
	return ix.Report(plans, heads, j, ix.Shadow(j, nil))
}

func ptInputs(tb testing.TB) ([]byte, []FileDiff, func(string) ([]byte, error), map[string][]string) {
	ix := ptIndex()
	var b bytes.Buffer
	if err := ix.Encode(&b); err != nil {
		tb.Fatal(err)
	}
	src := ptSource() // read once: blob reads are outside the budget
	heads := map[string][]string{}
	for _, t := range ix.Tests[1:] {
		heads["e2e"] = append(heads["e2e"], t.Name)
	}
	return b.Bytes(), ptChange(), func(string) ([]byte, error) { return src, nil }, heads
}

// MCJ-V0-019, MCJ-V0-025: retain the reference workload and p95 budget, measuring
// process CPU to exclude scheduler stalls on shared runners. Wall time is still reported.
func TestPlanTimeBudgetWithSegments(t *testing.T) {
	encoded, diffs, read, heads := ptInputs(t)
	r := planOnce(t, encoded, diffs, read, heads)
	if len(r.Shadow.Exclusions) == 0 || r.Counts["e2e"]["selected"] == 0 {
		t.Fatalf("reference change exercises no shadow exclusion: %v", r.Counts)
	}
	if _, err := planCPUTime(); errors.Is(err, errPlanCPUUnavailable) {
		t.Skipf("process CPU measurement NOT_OBSERVED: %v", err)
	} else if err != nil {
		t.Fatal(err)
	}
	const runs = 40
	wallStart := time.Now()
	p50, p95, err := measurePlanCPU(runs, planCPUTime, func() {
		planOnce(t, encoded, diffs, read, heads)
	})
	if err != nil {
		t.Fatal(err)
	}
	budget := raceScale * 100 * time.Millisecond
	if v := os.Getenv("LINECOV_PLAN_P95_BUDGET"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			t.Fatal(err)
		}
		budget = d
	}
	t.Logf("index %d bytes, %d tests, %d segments; selected %d, shadow would skip %d; plan CPU p50 %s p95 %s (budget %s), wall total %s",
		len(encoded), ptTests, ptTests*ptSegsPerTest, r.Counts["e2e"]["selected"], len(r.Shadow.Exclusions), p50, p95, budget, time.Since(wallStart))
	if err := checkPlanBudget(p95, budget); err != nil {
		t.Fatal(err)
	}
}

var errPlanCPUUnavailable = errors.New("process CPU clock unavailable on this platform")

func measurePlanCPU(runs int, clock func() (time.Duration, error), plan func()) (time.Duration, time.Duration, error) {
	times := make([]time.Duration, runs)
	for i := range times {
		start, err := clock()
		if err != nil {
			return 0, 0, err
		}
		plan()
		end, err := clock()
		if err != nil {
			return 0, 0, err
		}
		if end < start {
			return 0, 0, fmt.Errorf("process CPU clock went backwards")
		}
		times[i] = end - start
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times[runs/2], times[runs*95/100], nil
}

func checkPlanBudget(p95, budget time.Duration) error {
	if p95 > budget {
		return fmt.Errorf("plan p95 %s exceeds %s", p95, budget)
	}
	return nil
}

func TestMCJV0025PlanClockFailure(t *testing.T) {
	sentinel := errors.New("clock failed")
	if _, _, err := measurePlanCPU(40, func() (time.Duration, error) { return 0, sentinel }, func() { t.Fatal("ran without clock") }); !errors.Is(err, sentinel) {
		t.Fatalf("lost clock failure: %v", err)
	}
	calls := 0
	_, _, err := measurePlanCPU(40, func() (time.Duration, error) {
		calls++
		if calls == 1 {
			return time.Second, nil
		}
		return 0, nil
	}, func() {})
	if err == nil {
		t.Fatal("backwards CPU clock admitted")
	}
}

func TestMCJV0025PlanBudgetDoubleCostFails(t *testing.T) {
	// Inject a CPU clock, not sleeps: scheduler delay cannot hide or create the regression.
	for _, cost := range []time.Duration{60 * time.Millisecond, 120 * time.Millisecond} {
		var cpu time.Duration
		clock := func() (time.Duration, error) { return cpu, nil }
		p50, p95, err := measurePlanCPU(40, clock, func() { cpu += cost })
		if err != nil || p50 != cost || p95 != cost {
			t.Fatalf("measurement p50=%s p95=%s err=%v", p50, p95, err)
		}
		if failed := checkPlanBudget(p95, 100*time.Millisecond) != nil; failed != (cost > 100*time.Millisecond) {
			t.Fatalf("cost %s budget failure=%v", cost, failed)
		}
	}
	// A p95 regression must not be averaged away by the other 37 cheap runs.
	var cpu time.Duration
	i := 0
	_, p95, err := measurePlanCPU(40, func() (time.Duration, error) { return cpu, nil }, func() {
		i++
		cpu += 60 * time.Millisecond
		if i > 37 {
			cpu += 60 * time.Millisecond
		}
	})
	if err != nil || checkPlanBudget(p95, 100*time.Millisecond) == nil {
		t.Fatalf("tail regression escaped: p95=%s err=%v", p95, err)
	}
}

func BenchmarkPlanWithSegments(b *testing.B) {
	encoded, diffs, read, heads := ptInputs(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planOnce(b, encoded, diffs, read, heads)
	}
}
