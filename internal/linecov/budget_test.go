package linecov

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// The synthetic shape of the whole-module Corvint index (GLS-V0-003): the
// partial index of 2026-11-03 held 2,438 files, 259,929 blocks, 2,923,326
// block-test references and 5,879 tests over 236 of the module's 397 test
// packages; this is about 1.7 times that.
const (
	lbFiles        = 4000
	lbFilesPerPkg  = 10
	lbFuncs        = 110 // one block per function
	lbTests        = 10000
	lbTestsPerPkg  = 25
	lbRefsPerBlock = 20
	lbChanged      = 50
)

func lbSource() []byte {
	var b strings.Builder
	b.WriteString("package p\n\n")
	for i := 0; i < lbFuncs; i++ {
		fmt.Fprintf(&b, "func F%d() int {\n\tv := %d\n\treturn v\n}\n\n", i, i)
	}
	return []byte(b.String())
}

func lbIndex(tb testing.TB) []byte {
	ix := &Index{Format: Format, Module: "example.com/m", Commit: "c0", Runner: "go test", EnvBound: true}
	for t := 0; t < lbTests; t++ {
		ix.Tests = append(ix.Tests, Test{Package: fmt.Sprintf("p%03d", t/lbTestsPerPkg), Name: fmt.Sprintf("Test%05d", t)})
	}
	// Each package's in-order record, reaching nothing: without one the
	// package is unordered and every unit runs whole, which measures no
	// per-test decision (GLS-V0-008).
	for p := 0; p < lbTests/lbTestsPerPkg; p++ {
		ix.Tests = append(ix.Tests, Test{Package: fmt.Sprintf("p%03d", p), Name: InOrder})
	}
	sort.Slice(ix.Tests, func(i, j int) bool {
		return ix.Tests[i].Package < ix.Tests[j].Package || ix.Tests[i].Package == ix.Tests[j].Package && ix.Tests[i].Name < ix.Tests[j].Name
	})
	testID := make([]int32, lbTests) // the index of test t in the sorted table
	for i, test := range ix.Tests {
		if test.Name != InOrder {
			var t int
			fmt.Sscanf(test.Name, "Test%05d", &t)
			testID[t] = int32(i)
		}
	}
	for f := 0; f < lbFiles; f++ {
		file := File{Path: fmt.Sprintf("p%03d/f%d.go", f/lbFilesPerPkg, f), OID: "blob1"}
		for i := 0; i < lbFuncs; i++ {
			l := int32(3 + 5*i)
			b := Block{Start: l, StartCol: 15, End: l + 3, EndCol: 2}
			seen := map[int32]bool{}
			for k := 0; k < lbRefsPerBlock; k++ {
				seen[testID[(f*7919+i*104729+k*1299709)%lbTests]] = true
			}
			for id := range seen {
				b.Tests = append(b.Tests, id)
			}
			sort.Slice(b.Tests, func(x, y int) bool { return b.Tests[x] < b.Tests[y] })
			file.Blocks = append(file.Blocks, b)
		}
		ix.Files = append(ix.Files, file)
	}
	sort.Slice(ix.Files, func(i, j int) bool { return ix.Files[i].Path < ix.Files[j].Path })
	ix.reindex()
	var buf bytes.Buffer
	if err := ix.Encode(&buf); err != nil {
		tb.Fatal(err)
	}
	return buf.Bytes()
}

// lbPlan is everything GLS-V0-003 times: open the index lazily, analyse the
// change and decide every unit. Diffs, blobs and head names are read before.
func lbPlan(tb testing.TB, file string, in PlanInput) Selection {
	ix, err := Open(file)
	if err != nil {
		tb.Fatal(err)
	}
	defer ix.Close()
	sel := ix.Plan(in)
	if sel.State != StateNarrowed {
		tb.Fatalf("selection %s %s %s", sel.State, sel.Reason, sel.Detail)
	}
	return sel
}

// GLS-V0-003 (LPCV-V0-958): opening the index, analysing a 50-file change
// and deciding every unit stays within p95 <= 100 ms beyond the Git, blob and
// head-file reads. LINECOV_BENCH_INDEX names a real index to time instead,
// with LINECOV_BENCH_ROOT a checkout holding its commit; LINECOV_PLAN_P95_BUDGET
// overrides the budget on another host, and -race scales it by raceScale.
// As MCJ-V0-025 does for the segmented plan, each run is measured by process
// user plus system CPU time, so scheduler stalls on a shared runner neither
// fail nor hide the budget; total wall time is still reported, and
// reference-host wall latency stays a benchmark qualification. Process CPU
// still inflates under contention (cache, memory bandwidth and GC workers on
// busy cores: p95 110 to 122 ms on the loaded nightly gate runners, V1-5003),
// so an over-budget p95 on a host whose 1-minute load average reached the core
// count before or after the runs is reported NOT_JUDGED, as PRB-V0-028 and
// PRB-V0-039 treat a contended perf run; a quiet or unobserved host is judged.
// Contention never excuses a p95 beyond contendedPlanBudgetFactor times the
// budget, and an explicit LINECOV_PLAN_P95_BUDGET is always judged.
func TestLazyPlanBudget(t *testing.T) {
	file, in := lbInputs(t)
	sel := lbPlan(t, file, in)
	if sel.Counts.Units == 0 || sel.Counts.TestsDeclared == 0 {
		t.Fatalf("the reference change decides nothing: %+v", sel.Counts)
	}
	if _, err := planCPUTime(); errors.Is(err, errPlanCPUUnavailable) {
		t.Skipf("process CPU measurement NOT_OBSERVED: %v", err)
	} else if err != nil {
		t.Fatal(err)
	}
	const runs = 40
	loadBefore, loadedBefore := hostLoad1()
	wallStart := time.Now()
	p50, p95, err := measurePlanCPU(runs, planCPUTime, func() { lbPlan(t, file, in) })
	if err != nil {
		t.Fatal(err)
	}
	wall := time.Since(wallStart)
	loadAfter, loadedAfter := hostLoad1()
	// Under -race (raceScale > 1) the lazy path also decodes the 20 MB index on
	// every run: process CPU p95 was 0.55 s on an idle 4-core linux/amd64 host,
	// and CI's shared runners measured the segmented plan's CPU at about twice
	// that host's. Double the race factor here so the bound still catches only a
	// gross regression; the uninstrumented 100 ms budget is unchanged.
	budget := raceScale * 100 * time.Millisecond
	if raceScale > 1 {
		budget *= 2
	}
	override := os.Getenv("LINECOV_PLAN_P95_BUDGET")
	if override != "" {
		d, err := time.ParseDuration(override)
		if err != nil {
			t.Fatal(err)
		}
		budget = d
	}
	st, _ := os.Stat(file)
	t.Logf("index %d bytes; %d changed files, %d units, %d tests declared, %d kept by coverage, %d excluded; open+plan CPU p50 %s p95 %s (budget %s), wall total %s",
		st.Size(), len(in.Change), sel.Counts.Units, sel.Counts.TestsDeclared, sel.Counts.TestsCoverage, sel.Counts.TestsExcluded, p50, p95, budget, wall)
	if p95 > budget {
		cores := runtime.NumCPU()
		if override == "" && p95 <= contendedPlanBudgetFactor*budget && planBudgetContended(loadBefore, loadAfter, loadedBefore && loadedAfter, cores) {
			t.Skipf("GLS-V0-003 NOT_JUDGED: open+plan CPU p95 %s exceeds %s on a contended host: 1-minute load average %.2f before and %.2f after, threshold %d (the core count)",
				p95, budget, loadBefore, loadAfter, cores)
		}
		t.Fatalf("open+plan CPU p95 %s exceeds %s", p95, budget)
	}
}

// lbInputs returns the index file and a 50-file body change: the synthetic
// index, or LINECOV_BENCH_INDEX with 50 of its files, each changed on the
// first line inside its first block, read at the index commit from
// LINECOV_BENCH_ROOT.
func lbInputs(t *testing.T) (string, PlanInput) {
	t.Helper()
	if real := os.Getenv("LINECOV_BENCH_INDEX"); real != "" {
		return lbRealInputs(t, real, os.Getenv("LINECOV_BENCH_ROOT"))
	}
	file := filepath.Join(t.TempDir(), "linecov.idx")
	if err := os.WriteFile(file, lbIndex(t), 0o644); err != nil {
		t.Fatal(err)
	}
	src := lbSource()
	var change []FileDiff
	var units []Unit
	heads := map[string][]string{}
	for i := 0; i < lbChanged; i++ {
		f := i * (lbFiles / lbChanged)
		p := fmt.Sprintf("p%03d/f%d.go", f/lbFilesPerPkg, f)
		l := 3 + 5*(i%lbFuncs) + 1
		change = append(change, FileDiff{Path: p, OldOID: "blob1", NewOID: "blob2", NewSource: src, NewHunks: []Hunk{{Start: l, End: l}}, Hunks: []Hunk{{Start: l, End: l}}})
		dir := path.Dir(p)
		units = append(units, Unit{ID: "go:example.com/m/" + dir, Dir: dir, Witness: p})
	}
	for t := 0; t < lbTests; t++ {
		dir := fmt.Sprintf("p%03d", t/lbTestsPerPkg)
		heads[dir] = append(heads[dir], fmt.Sprintf("Test%05d", t))
	}
	return file, lbInput(change, func(string) ([]byte, error) { return src, nil }, units, heads)
}

func lbInput(change []FileDiff, read func(string) ([]byte, error), units []Unit, heads map[string][]string) PlanInput {
	return PlanInput{
		Base: "base", Head: "head", Change: change, ReadDrift: read, ReadChange: read, Units: units,
		Heads: func(dirs []string) (map[string][]string, error) {
			out := make(map[string][]string, len(dirs))
			for _, d := range dirs {
				out[d] = heads[d]
			}
			return out, nil
		},
		StaticOf: func([]string) (bool, map[string]string, error) {
			return false, nil, fmt.Errorf("the reference change has no fallback path")
		},
	}
}

func lbRealInputs(t *testing.T, file, root string) (string, PlanInput) {
	t.Helper()
	ix, err := Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer ix.Close()
	ctx := context.Background()
	var change []FileDiff
	var units []Unit
	seen := map[string]bool{}
	blobs := map[string][]byte{}
	step := max(1, len(ix.Files)/lbChanged)
	// Spread the first candidates across the index, then fill any gaps
	// left by uninstrumented or declaration-only files from the remaining
	// offsets. Skipping a candidate must not shrink the 50-file workload.
	var candidates []int
	for offset := 0; offset < step; offset++ {
		for i := offset; i < len(ix.Files); i += step {
			candidates = append(candidates, i)
		}
	}
	for _, i := range candidates {
		if len(change) == lbChanged {
			break
		}
		f := ix.Files[i]
		if !strings.HasSuffix(f.Path, ".go") || strings.HasSuffix(f.Path, "_test.go") {
			continue
		}
		if err := ix.ensureFile(i); err != nil {
			t.Fatal(err)
		}
		f = ix.Files[i]
		src, err := Reader(ctx, root, ix.Commit)(f.Path)
		if err != nil {
			t.Fatal(err)
		}
		line := 0
		for _, b := range f.Blocks {
			if b.End > b.Start {
				line = int(b.Start) + 1
				break
			}
		}
		if line == 0 {
			continue
		}
		d := FileDiff{Path: f.Path, OldOID: f.OID, NewOID: "changed", NewSource: src, NewHunks: []Hunk{{Start: line, End: line}}, Hunks: []Hunk{{Start: line, End: line}}}
		if r, _ := ix.AnalyzePaths([]FileDiff{d}, func(string) ([]byte, error) { return src, nil }); !r[0].Mapped() {
			continue // a block outside every function body
		}
		blobs[f.Path] = src
		change = append(change, d)
		dir := path.Dir(f.Path)
		if !seen[dir] {
			seen[dir] = true
			units = append(units, Unit{ID: "go:" + ix.Module + "/" + dir, Dir: dir, Witness: f.Path})
		}
	}
	if len(change) != lbChanged {
		t.Fatalf("real index provides %d eligible changed files, need %d", len(change), lbChanged)
	}
	// Every indexed package is a unit of the plan, as a module-wide static
	// plan would make it.
	for _, tt := range ix.Tests {
		if !seen[tt.Package] {
			seen[tt.Package] = true
			units = append(units, Unit{ID: "go:" + ix.Module + "/" + tt.Package, Dir: tt.Package})
		}
	}
	heads := map[string][]string{}
	for _, tt := range ix.Tests {
		if tt.Name != InOrder {
			heads[tt.Package] = append(heads[tt.Package], tt.Name)
		}
	}
	return file, lbInput(change, func(p string) ([]byte, error) { return blobs[p], nil }, units, heads)
}
