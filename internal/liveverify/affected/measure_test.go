package affected_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
)

// IncrementalBudget is the per-edit selection budget. It bounds the step that
// runs on every keystroke: mapping a dirty path set onto a prebuilt graph.
// Graph construction is a cold cost paid once and is deliberately outside it.
const IncrementalBudget = 100 * time.Millisecond

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func buildRepositoryGraph(t *testing.T) (*affected.Graph, time.Duration) {
	t.Helper()
	root := repositoryRoot(t)
	start := time.Now()
	graph, err := affected.Build(context.Background(), root, golang.New(), python.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return graph, time.Since(start)
}

// TestIncrementalSelectionMeetsTheLiveBudget is the gate for the sub-100ms
// incremental target. It measures the real repository, not a fixture, because
// the budget only means something at the size the product runs at.
func TestIncrementalSelectionMeetsTheLiveBudget(t *testing.T) {
	graph, cold := buildRepositoryGraph(t)
	t.Logf("cold graph build: %s units=%d languages=%v", cold.Round(time.Millisecond), len(graph.UnitIDs()), graph.Languages())

	edits := representativeEdits(t, graph)
	if len(edits) < 3 {
		t.Fatalf("too few representative edits: %v", edits)
	}
	samples := make([]time.Duration, 0, len(edits)*32)
	probes := make([]time.Duration, 0, len(edits)*32)
	var widest int
	for round := 0; round < 32; round++ {
		for _, edit := range edits {
			probes = append(probes, timeCalibrationProbe())
			start := time.Now()
			plan := affected.Select(graph, []string{edit})
			samples = append(samples, time.Since(start))
			if count := len(plan.Selected); count > widest {
				widest = count
			}
		}
	}
	sortDurations(samples)
	sortDurations(probes)
	p50 := samples[len(samples)/2]
	p95 := samples[len(samples)*95/100]
	worst := samples[len(samples)-1]
	probe95 := probes[len(probes)*95/100]
	t.Logf("incremental select over %d units: n=%d p50=%s p95=%s max=%s widest-plan=%d units; probe p50=%s p95=%s",
		len(graph.UnitIDs()), len(samples), p50, p95, worst, widest, probes[len(probes)/2], probe95)
	// Both p95s come from interleaved samples, so a contended tail inflates
	// both alike. On the reference host the ratio measures about 1.3.
	enforceSelectionBudget(t, "incremental select p95", p95, probe95, referenceProbeP95)
}

// The calibration probe's p95 and median on the reference host (Apple
// M-series, 12 cores, go1.27.2, no race detector), measured at 17.1-19.6 ms
// and 13.6-14.3 ms and rounded up, so the bounds derived from them err strict.
// The measurements and the rejected alternatives are in build log
// 2026-10-14-v1-1343-live-budget-under-load.
const (
	referenceProbeP95    = 20 * time.Millisecond
	referenceProbeMedian = 15 * time.Millisecond
)

// enforceSelectionBudget holds a measured selection time to IncrementalBudget.
// A host that is contended, or slowed by the race detector, misses the
// absolute budget without selection itself being slower; the probe, timed in
// the same goroutine alongside the selections, slows with it. So a miss is
// judged in probe units against IncrementalBudget/reference, where reference
// is the reference host's value of the same probe statistic: a selection
// above that many probes would also miss the 100 ms budget on the reference
// host. Within the bound the absolute budget is skipped by name, with the
// numbers, and never passed silently; beyond it selection itself is too slow
// and the test fails on any host.
func enforceSelectionBudget(t *testing.T, label string, measured, probe, reference time.Duration) {
	t.Helper()
	if measured <= IncrementalBudget {
		return
	}
	bound := float64(IncrementalBudget) / float64(reference)
	ratio := float64(measured) / float64(probe)
	if ratio > bound {
		t.Fatalf("%s=%s exceeds the %s budget and is %.2f calibration probes (probe=%s), above the %.2f bound: selection itself is too slow",
			label, measured, IncrementalBudget, ratio, probe, bound)
	}
	t.Skipf("absolute %s budget NOT ENFORCED on this host: %s=%s, but the calibration probe took %s, so selection is %.2f probes, within the %.2f bound (V1-1343)",
		IncrementalBudget, label, measured, probe, ratio, bound)
}

var probeSink int

// timeCalibrationProbe times one run of a fixed, deterministic,
// single-goroutine workload shaped like selection: string-keyed maps, witness
// path copies, breadth-first frontiers and sorts over a synthetic 512-node
// graph. It does not call the affected package; it shares only the Go
// runtime's allocator and collector with Select.
func timeCalibrationProbe() time.Duration {
	start := time.Now()
	const nodes = 512
	names := make([]string, nodes)
	for index := range names {
		names[index] = fmt.Sprintf("unit/%04d", index)
	}
	edges := make(map[string][]string, nodes)
	state := uint32(1)
	for index, name := range names {
		for range 6 {
			state = state*1664525 + 1013904223
			edges[name] = append(edges[name], names[(index+1+int(state>>8)%(nodes-1))%nodes])
		}
	}
	total := 0
	for seed := range 64 {
		origin := names[(seed*7919)%nodes]
		paths := map[string][]string{origin: {origin}}
		frontier := []string{origin}
		for len(frontier) != 0 {
			next := make([]string, 0, len(frontier))
			for _, id := range frontier {
				for _, dependent := range edges[id] {
					if _, seen := paths[dependent]; seen {
						continue
					}
					paths[dependent] = append(append([]string(nil), paths[id]...), dependent)
					next = append(next, dependent)
				}
			}
			sort.Strings(next)
			frontier = next
		}
		total += len(slices.Sorted(maps.Keys(paths)))
	}
	probeSink += total
	return time.Since(start)
}

func sortDurations(values []time.Duration) {
	sort.Slice(values, func(left, right int) bool { return values[left] < values[right] })
}

// representativeEdits picks units spanning the fan-out range, so the measured
// worst case is a real hub edit rather than a leaf edit that reaches nothing.
func representativeEdits(t *testing.T, graph *affected.Graph) []string {
	t.Helper()
	type candidate struct {
		path  string
		reach int
	}
	candidates := make([]candidate, 0, 64)
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if len(unit.Sources) == 0 {
			continue
		}
		candidates = append(candidates, candidate{
			path:  unit.Sources[0],
			reach: len(affected.Select(graph, []string{unit.Sources[0]}).Selected),
		})
	}
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].reach > candidates[right].reach })
	edits := make([]string, 0, 8)
	for _, index := range []int{0, 1, 2, len(candidates) / 2, len(candidates) - 1} {
		if index >= 0 && index < len(candidates) {
			edits = append(edits, candidates[index].path)
		}
	}
	return edits
}

// TestSelectionOnTheLiveDirtyWorktree is the end-to-end path the product needs:
// observe what Git says changed right now, and turn it into a plan. It runs
// against this repository's actual worktree, whatever state it is in.
func TestSelectionOnTheLiveDirtyWorktree(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := repositoryRoot(t)
	start := time.Now()
	dirty, err := affected.DirtyPaths(context.Background(), gitExecutable, root)
	statusElapsed := time.Since(start)
	if err != nil {
		if errors.Is(err, affected.ErrStatusUnavailable) {
			t.Skip("worktree status is unavailable here")
		}
		t.Fatalf("status: %v", err)
	}
	t.Logf("git status capture: %s dirty=%d paths", statusElapsed.Round(time.Millisecond), len(dirty))

	graph, cold := buildRepositoryGraph(t)
	// Five selections interleaved with five calibration probes, judged by
	// their medians: under load one sample is a spike's hostage, while a
	// slower Select slows every sample.
	var plan affected.Plan
	samples := make([]time.Duration, 0, 5)
	probes := make([]time.Duration, 0, 5)
	for range 5 {
		probes = append(probes, timeCalibrationProbe())
		start = time.Now()
		plan = affected.Select(graph, dirty)
		samples = append(samples, time.Since(start))
	}
	sortDurations(samples)
	sortDurations(probes)
	selectElapsed, probe := samples[len(samples)/2], probes[len(probes)/2]
	t.Logf("cold graph=%s select median=%s max=%s probe median=%s scope=%s selected=%d excluded=%d unknown=%d",
		cold.Round(time.Millisecond), selectElapsed, samples[len(samples)-1], probe, plan.Scope, len(plan.Selected), len(plan.Excluded), len(plan.Unknown))

	if plan.GraphDigest != graph.Digest() {
		t.Fatalf("plan is not bound to the graph it was computed from")
	}
	// Every selected unit's witness must resolve, on the live worktree, not
	// only on the fixture. A reader witness resolves to a unit whose path
	// literals name its dirty path (AFP-V0-021).
	for _, selection := range plan.Selected {
		if selection.Witness.Kind == affected.WitnessPathLiteralReader {
			if !slices.Contains(graph.Namers(selection.Witness.DirtyPath), selection.UnitID) {
				t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
			}
			continue
		}
		if selection.Witness.Kind == affected.WitnessUnboundedReader {
			if !slices.Contains(graph.UnboundedReaders(), selection.UnitID) {
				t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
			}
			continue
		}
		// A declared-scope witness resolves to a declared unit whose scope
		// holds its dirty path (AFP-V0-023).
		if selection.Witness.Kind == affected.WitnessDeclaredReadScope {
			unit, found := graph.Unit(selection.UnitID)
			files := append(append([]string(nil), unit.Sources...), unit.Tests...)
			if !found || !unit.ReadScoped || len(files) == 0 || !affected.InReadScope(path.Dir(files[0]), unit.ReadScope, selection.Witness.DirtyPath) {
				t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
			}
			continue
		}
		owner, owned := graph.OwnerOf(selection.Witness.DirtyPath)
		if !owned || owner != selection.Witness.Via[0] {
			t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
		}
	}
	enforceSelectionBudget(t, "live-worktree select median", selectElapsed, probe, referenceProbeMedian)
}

// TestOverlaySelectionNeverTouchesTheWorktree proves LPCV-V0-002's second half
// for the selection layer: admitting an unsaved buffer changes the plan and
// nothing on disk.
func TestOverlaySelectionNeverTouchesTheWorktree(t *testing.T) {
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	root := repositoryRoot(t)
	before, err := affected.DirtyPaths(context.Background(), gitExecutable, root)
	if err != nil {
		t.Skipf("worktree status is unavailable here: %v", err)
	}
	graph, _ := buildRepositoryGraph(t)
	target := ""
	for _, id := range graph.UnitIDs() {
		unit, _ := graph.Unit(id)
		if len(unit.Sources) != 0 && len(unit.Imports) == 0 {
			target = unit.Sources[0]
			break
		}
	}
	if target == "" {
		t.Skip("no candidate source file")
	}
	widened := affected.Select(graph, affected.Union(before, affected.Overlay{Paths: []string{target}}))
	if _, owned := graph.OwnerOf(target); !owned {
		t.Fatalf("%s is not owned by any unit", target)
	}
	found := false
	for _, selection := range widened.Selected {
		if selection.Witness.DirtyPath == target {
			found = true
		}
	}
	if !found && len(widened.Selected) != 0 {
		// The overlaid file's unit may already have been selected through
		// another dirty path; that is still a correct plan.
		if owner, _ := graph.OwnerOf(target); !selectedContains(widened, owner) {
			t.Fatalf("overlay path %s did not reach its own unit", target)
		}
	}
	after, err := affected.DirtyPaths(context.Background(), gitExecutable, root)
	if err != nil {
		t.Fatalf("status after: %v", err)
	}
	if len(before) != len(after) {
		t.Fatalf("selection mutated the worktree: %d dirty paths before, %d after", len(before), len(after))
	}
}

func selectedContains(plan affected.Plan, unitID string) bool {
	for _, selection := range plan.Selected {
		if selection.UnitID == unitID {
			return true
		}
	}
	return false
}

// TestPythonSelectionNarrowsARealCorpus is the non-Go end-to-end proof. The
// repository carries a real 18-module Python implementation with 41 test files;
// selection must narrow that set for a single-file edit and must still reach
// the test the specification's own traceability table pairs with the module.
func TestPythonSelectionNarrowsARealCorpus(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, python.New())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	everyTest := affected.Select(graph, nil)
	total := 0
	for _, exclusion := range everyTest.Excluded {
		unit, _ := graph.Unit(exclusion.UnitID)
		total += len(unit.Tests)
	}
	if total < 20 {
		t.Skipf("python corpus is too small to measure: %d tests", total)
	}
	const edited = "src/context_corvint_index.py"
	if _, owned := graph.OwnerOf(edited); !owned {
		t.Skipf("%s is not present", edited)
	}
	plan := affected.Select(graph, []string{edited})
	selected := plan.SelectedTests()
	if len(selected) >= total {
		t.Fatalf("selection did not narrow: %d of %d tests", len(selected), total)
	}
	if !contains(selected, "tests/test_context_corvint_cache.py") {
		t.Fatalf("selection missed the module's paired test: %v", selected)
	}
	t.Logf("python: editing %s selects %d of %d tests (%.0f%% narrowed)",
		edited, len(selected), total, 100*(1-float64(len(selected))/float64(total)))
}
