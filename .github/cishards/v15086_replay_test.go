package cishards

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// measuredRun is one hosted run's measured times: per-package elapsed milliseconds and, for
// the packages the allow-list names, each top-level test's elapsed milliseconds.
type measuredRun struct {
	Run          string                      `json:"run"`
	Revision     string                      `json:"revision"`
	Milliseconds map[string]int64            `json:"milliseconds"`
	Tests        map[string]map[string]int64 `json:"tests"`
}

// run42408095814 is the V1-5086 replay run: pull-request run 42408095814 of head
// e83883d44875a00c0fa2784596d2d85ca2a0f75a, whose go-product-shard jobs ran `go test -p 4`
// under go1.27.3 (its logs report "running go1.27.3").
const (
	run42408095814Parallel  = 4
	run42408095814GoVersion = "go1.27.3"
)

func loadRun42408095814(t *testing.T) (measuredRun, []string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/run42408095814.json")
	if err != nil {
		t.Fatal(err)
	}
	var run measuredRun
	if err := json.Unmarshal(raw, &run); err != nil {
		t.Fatal(err)
	}
	if len(run.Milliseconds) != 271 {
		t.Fatalf("fixture has %d packages", len(run.Milliseconds))
	}
	universe := make([]string, 0, len(run.Milliseconds))
	for p := range run.Milliseconds {
		universe = append(universe, p)
	}
	sort.Strings(universe)
	return run, universe
}

// replayFiles are the planner inputs a refresh from the run itself produces (AFP-V0-040,
// AFP-V0-060): its measured package times as the cost table, and slices of the committed
// allow-list's packages cut by the head rule (SliceTarget, SliceCount, SliceTests) from the
// run's own top-level test names and times, both recording the run's -p as their source.
func replayFiles(t *testing.T, run measuredRun, shards, parallel int, target int64) (costs, slices []byte) {
	t.Helper()
	source := map[string]any{"revision": run.Revision, "runURL": run.Run, "goVersion": run42408095814GoVersion, "parallel": parallel}
	costs, err := json.Marshal(map[string]any{"profile": "corvint-ci-package-costs/0", "source": source, "milliseconds": run.Milliseconds})
	if err != nil {
		t.Fatal(err)
	}
	allowed, ok := Allowed(defaultAllow)
	if !ok {
		t.Fatal("committed allow-list unusable")
	}
	packages := map[string]TestSplit{}
	for p := range allowed {
		elapsed, seen := run.Milliseconds[p]
		k := SliceCount(elapsed, target, shards)
		if !seen || k < 2 {
			continue
		}
		names := make([]string, 0, len(run.Tests[p]))
		for n := range run.Tests[p] {
			names = append(names, n)
		}
		split, ok := SliceTests(names, run.Tests[p], elapsed, k)
		if !ok {
			t.Fatalf("%s: too few nameable tests for %d slices", p, k)
		}
		packages[p] = split
	}
	slices, err = json.Marshal(map[string]any{"profile": "corvint-ci-test-slices/0", "source": source, "packages": packages})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := Costs(costs); !ok || !SliceFileUsable(slices) {
		t.Fatal("replay files are not admitted")
	}
	return costs, slices
}

// TestV15086Run42408095814FullReplay replays pull-request run 42408095814 (V1-5086 criterion 1)
// through the planner: the inputs a refresh from that run produces (replayFiles), a full
// sixteen-shard plan at the run's -p 4 (PlanFromParallel), then each shard priced with the
// run's measured times. The longest shard is at most the 400 s budget and at most 1.5 times
// the shortest; a miss fails.
//
// testdata/run42408095814.json holds the go test -json elapsed times of the run's
// go-product-shard jobs for the 271 of its 458 packages that still exist at this head, since a
// deleted package is never planned (the 187 others, 8389 s of its 18396 s, include
// internal/tasks/cli and internal/tasks/store, which moved to corvint-tasks), and the
// top-level test times of each package the committed allow-list names. A package that ran as
// slices sums its slice processes. internal/testrunner (146.5 s) and internal/modelgateway
// (0.2 s) failed in that run; their measured elapsed times are kept.
//
// Test names drift: 990 of the 1506 cmd/corvint tests the run measured are not at this head,
// and the committed slice file names the tests of cost run 43766039995, so it prices the run's
// cmd/corvint almost entirely into its catch-all. The replay therefore slices by the run's own
// names, as generating from that run would, and maps no name to this head. The committed
// files' result on this run is logged for comparison and is not the criterion.
func TestV15086Run42408095814FullReplay(t *testing.T) {
	run, universe := loadRun42408095814(t)
	const shards, parallel = 16, run42408095814Parallel
	allowed, _ := Allowed(defaultAllow)
	target := SliceTarget(run.Milliseconds, allowed, shards, parallel)
	w, whole := replayWalls(t, run, universe, shards, parallel, target)
	for _, p := range whole {
		t.Errorf("%s ran %.1f s whole, above the budget, and the replay keeps it whole", p, float64(run.Milliseconds[p])/1000)
	}
	hi, lo := spread(w)
	for i, v := range w {
		t.Logf("shard %d: %.1f s", i, v/1000)
	}
	t.Logf("full -p %d replay, slice target %.1f s: wall %.1f..%.1f s (max/min %.3f), budget %.1f s", parallel, float64(target)/1000, lo/1000, hi/1000, hi/lo, float64(DefaultWallBudgetMs)/1000)
	if hi > 1.5*lo {
		t.Errorf("max/min shard wall %.3f above 1.5", hi/lo)
	}
	if hi > float64(DefaultWallBudgetMs) {
		t.Errorf("max shard wall %.1f s above the %.1f s budget", hi/1000, float64(DefaultWallBudgetMs)/1000)
	}

	// For comparison only: the committed files, whose slices name the tests of a later run.
	committed, err := PlanFromParallel(universe, shards, parallel, defaultCosts, defaultAllow, defaultSlices)
	if err != nil {
		t.Fatal(err)
	}
	chi, clo := spread(measuredWalls(t, committed, run, parallel))
	t.Logf("committed files on this run (name drift): wall %.1f..%.1f s", clo/1000, chi/1000)
}

// TestV15086ReplayMissesAtTheShardShareTarget is the replay's fault injection: slicing to the
// shard share (the total over shards, the target before V1-5086) leaves each slice a whole
// shard's wall at -p 4, and the same replay then misses criterion 1.
func TestV15086ReplayMissesAtTheShardShareTarget(t *testing.T) {
	run, universe := loadRun42408095814(t)
	const shards, parallel = 16, run42408095814Parallel
	var total int64
	for _, ms := range run.Milliseconds {
		total += ms
	}
	w, whole := replayWalls(t, run, universe, shards, parallel, total/shards)
	hi, lo := spread(w)
	t.Logf("shard-share target %.1f s: wall %.1f..%.1f s (max/min %.3f), %d packages above the budget kept whole", float64(total/shards)/1000, lo/1000, hi/1000, hi/lo, len(whole))
	if hi <= 1.5*lo && hi <= float64(DefaultWallBudgetMs) && len(whole) == 0 {
		t.Fatalf("the shard-share target meets the criterion (max/min %.3f, max %.1f s): the replay cannot tell the rules apart", hi/lo, hi/1000)
	}
}

// replayWalls plans the run from replayFiles at target and returns each shard's measured wall
// and the packages above the budget the plan keeps whole. An incomplete plan fails the test.
func replayWalls(t *testing.T, run measuredRun, universe []string, shards, parallel int, target int64) (walls []float64, whole []string) {
	t.Helper()
	costs, slicesRaw := replayFiles(t, run, shards, parallel, target)
	split := SplitPackages(universe, shards, defaultAllow, slicesRaw)
	for _, p := range universe {
		if run.Milliseconds[p] > DefaultWallBudgetMs && split[p] == nil {
			whole = append(whole, p)
		}
	}
	for p, s := range split {
		t.Logf("split %s: %.1f s into %d slices", p, float64(run.Milliseconds[p])/1000, len(s))
	}
	plan, err := PlanFromParallel(universe, shards, parallel, costs, defaultAllow, slicesRaw)
	if err != nil {
		t.Fatal(err)
	}
	requireComplete(t, universe, plan)
	if priced := PricedParallel(parallel, costs, slicesRaw); priced != parallel {
		t.Fatalf("replay files price at -p %d, want %d", priced, parallel)
	}
	return measuredWalls(t, plan, run, parallel), whole
}

// TestV15086CommittedPlanHoldsTheSingleTestSlice: the committed files' full sixteen-shard plan
// keeps the 209.6 s single-test cmd/corvint slice (one sequential scenario, which no -run
// pattern can split) as one unit and still balances. The committed table was measured at -p 1
// (push run 43766039995 records no parallel), so the -p 4 pull-request plan is priced at -p 1,
// each shard at its summed cost: that is an upper bound on its -p 4 wall, and the same bound
// holds for the -p 1 push plan.
func TestV15086CommittedPlanHoldsTheSingleTestSlice(t *testing.T) {
	costs, ok := Costs(defaultCosts)
	if !ok {
		t.Fatal("committed cost table unusable")
	}
	universe := make([]string, 0, len(costs))
	for p := range costs {
		universe = append(universe, p)
	}
	sort.Strings(universe)
	for _, parallel := range []int{1, 4} {
		plan, err := PlanParallel(universe, 16, parallel)
		if err != nil {
			t.Fatal(err)
		}
		requireComplete(t, universe, plan)
		priced := PricedParallel(parallel, defaultCosts, defaultSlices)
		if priced != 1 {
			t.Fatalf("-p %d plan of the -p 1 table priced at -p %d", parallel, priced)
		}
		w := walls(plan, costs, priced)
		hi, lo := spread(w)
		single := -1
		for i, s := range plan {
			for _, sl := range s.Slices {
				if sl.Flag == "-run" && !strings.Contains(sl.Pattern, "|") && sl.Milliseconds > 200000 {
					single = i
					t.Logf("-p %d: single-test slice %s %s (%.1f s) on shard %d, wall %.1f s", parallel, sl.Package, sl.Pattern, float64(sl.Milliseconds)/1000, i, w[i]/1000)
				}
			}
		}
		t.Logf("-p %d committed plan priced at -p %d: wall %.1f..%.1f s (max/min %.3f)", parallel, priced, lo/1000, hi/1000, hi/lo)
		if single < 0 {
			t.Fatalf("-p %d: no single-test slice above 200 s placed", parallel)
		}
		if hi > 1.5*lo || hi > float64(DefaultWallBudgetMs) {
			t.Fatalf("-p %d: max/min %.3f, max %.1f s", parallel, hi/lo, hi/1000)
		}
	}
}

// measuredWalls is each shard's wall at parallel from the run's measured times.
func measuredWalls(t *testing.T, plan []Shard, run measuredRun, parallel int) []float64 {
	t.Helper()
	out := make([]float64, len(plan))
	for i, s := range plan {
		var sum, longest int64
		for _, p := range s.Packages {
			sum += run.Milliseconds[p]
			longest = max(longest, run.Milliseconds[p])
		}
		for _, sl := range s.Slices {
			ms := measuredSlice(t, sl, run)
			sum += ms
			longest = max(longest, ms)
		}
		out[i] = float64(Wall(sum, longest, parallel)) / float64(parallel)
	}
	return out
}

// measuredSlice is the package's measured elapsed time times the share of its measured
// top-level test time the slice selects.
func measuredSlice(t *testing.T, sl Slice, run measuredRun) int64 {
	t.Helper()
	tests := run.Tests[sl.Package]
	if len(tests) == 0 {
		t.Fatalf("fixture has no test times for split package %s", sl.Package)
	}
	re := regexp.MustCompile(sl.Pattern)
	var part, all int64
	for name, ms := range tests {
		all += ms
		if re.MatchString(name) == (sl.Flag == "-run") {
			part += ms
		}
	}
	if all == 0 {
		t.Fatalf("fixture test times of %s sum to zero", sl.Package)
	}
	return int64(math.Round(float64(run.Milliseconds[sl.Package]) * float64(part) / float64(all)))
}
