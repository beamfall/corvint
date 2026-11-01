package affected_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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
	enforceSelectionBudget(t, "incremental select p95", p95, probe95, referenceProbeP95, IncrementalBudget)
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

// enforceSelectionBudget holds a measured selection time to budget, which is
// IncrementalBudget for a single edit and liveWorktreeBudget for a dirty set.
// A host that is contended, or slowed by the race detector, misses the
// absolute budget without selection itself being slower; the probe, timed in
// the same goroutine alongside the selections, slows with it. So a miss is
// judged in probe units against budget/reference, where reference
// is the reference host's value of the same probe statistic: a selection
// above that many probes would also miss the budget on the reference
// host. Within the bound the absolute budget is skipped by name, with the
// numbers, and never passed silently; beyond it selection itself is too slow
// and the test fails on any host.
func enforceSelectionBudget(t *testing.T, label string, measured, probe, reference, budget time.Duration) {
	t.Helper()
	exceeded, ratio, bound := selectionBudgetExceeded(measured, probe, reference, budget)
	if measured <= budget {
		return
	}
	if exceeded {
		t.Fatalf("%s=%s exceeds the %s budget and is %.2f calibration probes (probe=%s), above the %.2f bound: selection itself is too slow",
			label, measured, budget, ratio, probe, bound)
	}
	t.Skipf("absolute %s budget NOT ENFORCED on this host: %s=%s, but the calibration probe took %s, so selection is %.2f probes, within the %.2f bound (V1-1343)",
		budget, label, measured, probe, ratio, bound)
}

// selectionBudgetExceeded is the verdict of enforceSelectionBudget: a time
// within budget never exceeds it, and a time above it exceeds it only when it
// is more than budget/reference calibration probes.
func selectionBudgetExceeded(measured, probe, reference, budget time.Duration) (exceeded bool, ratio, bound float64) {
	bound = float64(budget) / float64(reference)
	ratio = float64(measured) / float64(probe)
	return measured > budget && ratio > bound, ratio, bound
}

// liveWorktreeBudget is the selection budget for a live dirty set of the given
// number of paths (V1-1447). IncrementalBudget is the single-edit target and
// holds for up to liveWorktreePathsPerBudget paths; each further group of that
// many paths adds one IncrementalBudget, up to liveWorktreeBudgetCap of them,
// so a dirty worktree of 13 or more paths is held to 400 ms (26.67 median
// probes) and the bound still bites. Measured on this repository under load
// about 48: a 14-path set took 4.4-6.2 median probes, and V1-1447 recorded
// 7.33-14 probes at 10-14 paths under load about 30, above the 6.67 bound the
// single-edit budget gives.
func liveWorktreeBudget(paths int) time.Duration {
	steps := max(1, (paths+liveWorktreePathsPerBudget-1)/liveWorktreePathsPerBudget)
	return IncrementalBudget * time.Duration(min(steps, liveWorktreeBudgetCap))
}

const (
	liveWorktreePathsPerBudget = 4
	liveWorktreeBudgetCap      = 4
)

// V1-1447: the live dirty-worktree bound is restated by dirty-set size, and
// it bites: a selection above the bound for its size fails, whatever the size.
func TestLiveWorktreeBudgetScalesWithTheDirtySetAndBites(t *testing.T) {
	for _, tc := range []struct {
		paths int
		want  time.Duration
	}{{0, 100 * time.Millisecond}, {1, 100 * time.Millisecond}, {4, 100 * time.Millisecond}, {5, 200 * time.Millisecond}, {14, 400 * time.Millisecond}, {400, 400 * time.Millisecond}} {
		if got := liveWorktreeBudget(tc.paths); got != tc.want {
			t.Errorf("liveWorktreeBudget(%d) = %s, want %s", tc.paths, got, tc.want)
		}
	}
	probe := referenceProbeMedian
	for _, tc := range []struct {
		paths    int
		measured time.Duration
		exceeded bool
	}{
		{1, 90 * time.Millisecond, false},   // within the absolute budget
		{1, 150 * time.Millisecond, true},   // 10 probes > 6.67 for one edit
		{14, 150 * time.Millisecond, false}, // the V1-1447 ratio, within 26.67 at 14 paths
		{14, 500 * time.Millisecond, true},  // 33.3 probes > 26.67
		{400, 500 * time.Millisecond, true}, // the cap keeps the bound for any size
	} {
		exceeded, ratio, bound := selectionBudgetExceeded(tc.measured, probe, referenceProbeMedian, liveWorktreeBudget(tc.paths))
		if exceeded != tc.exceeded {
			t.Errorf("paths=%d measured=%s: exceeded=%v (ratio %.2f, bound %.2f), want %v", tc.paths, tc.measured, exceeded, ratio, bound, tc.exceeded)
		}
	}
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
	// only on the fixture.
	for _, selection := range plan.Selected {
		if !witnessResolves(graph, selection) {
			t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
		}
	}
	enforceSelectionBudget(t, fmt.Sprintf("live-worktree select median (%d dirty paths)", len(dirty)), selectElapsed, probe, referenceProbeMedian, liveWorktreeBudget(len(dirty)))
}

// TestWitnessesResolveUnderTheGoStructuralRules_V1_0564_V1_0739 is the bounded
// fixture for the live-worktree check: an unowned data file inside a package
// yields an ENCLOSING_PACKAGE witness and a DEPENDENCY_PATH chain from that
// package, and a Go file of an absent package yields a DEPENDENCY_PATH witness
// at its importer. Each must resolve under its own rule, and a witness no rule
// produced must not.
func TestWitnessesResolveUnderTheGoStructuralRules_V1_0564_V1_0739(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":            "module example.test/witness\n\ngo 1.27\n",
		"core/core.go":      "package core\n",
		"core/core_test.go": "package core\n\nimport \"testing\"\n\nfunc TestCore(*testing.T) {}\n",
		"core/fixture.txt":  "data\n",
		"user/user.go":      "package user\n\nimport _ \"example.test/witness/gone\"\n",
		"user/user_test.go": "package user\n\nimport \"testing\"\n\nfunc TestUser(*testing.T) {}\n",
		"app/app.go":        "package app\n\nimport _ \"example.test/witness/core\"\n",
		"app/app_test.go":   "package app\n\nimport \"testing\"\n\nfunc TestApp(*testing.T) {}\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, golang.New())
	if err != nil {
		t.Fatal(err)
	}
	const core, user, app = "go:example.test/witness/core", "go:example.test/witness/user", "go:example.test/witness/app"
	plan := affected.Select(graph, []string{"core/fixture.txt", "gone/gone.go"})
	got := make(map[string]affected.Witness, len(plan.Selected))
	for _, selection := range plan.Selected {
		got[selection.UnitID] = selection.Witness
		if !witnessResolves(graph, selection) {
			t.Fatalf("%s has an unresolvable witness %+v", selection.UnitID, selection.Witness)
		}
	}
	want := map[string]affected.Witness{
		core: {Kind: affected.WitnessEnclosingPackage, DirtyPath: "core/fixture.txt", Via: []string{core}},
		app:  {Kind: affected.WitnessDependency, DirtyPath: "core/fixture.txt", Via: []string{core, app}},
		user: {Kind: affected.WitnessDependency, DirtyPath: "gone/gone.go", Via: []string{user}},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("witnesses = %+v, want %+v", got, want)
	}
	for _, forged := range []affected.Selection{
		{UnitID: core, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "gone/gone.go", Via: []string{core}}},
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "gone/gone.go", Via: []string{user, core}}},
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessEnclosingPackage, DirtyPath: "core/fixture.txt", Via: []string{app}}},
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "user/fixture.txt", Via: []string{core, app}}},
		{UnitID: user, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "core/core.go", Via: []string{user}}},
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "core/fixture.txt", Via: []string{core}}},
		{UnitID: user, Witness: affected.Witness{Kind: affected.WitnessDirectSource, DirtyPath: "gone/gone.go", Via: []string{user}}},
		{UnitID: core, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "core/fixture.txt", Via: []string{core}}},
		// V1-1491: a valid origin and end joined through a forged hop; user
		// neither imports core nor is imported by app.
		{UnitID: app, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "core/fixture.txt", Via: []string{core, user, app}}},
	} {
		if witnessResolves(graph, forged) {
			t.Fatalf("%s resolved a witness no rule produced: %+v", forged.UnitID, forged.Witness)
		}
	}
}

// witnessResolves checks one selection's witness against the rule that can
// produce its kind. A reader witness resolves to a unit whose path literals
// name its dirty path (AFP-V0-021); a declared-scope witness to a declared unit
// whose scope holds it (AFP-V0-023). Every other witness is a chain ending at
// the selected unit whose first unit the dirty path reaches by a rule: it owns
// the path, or a Go structural rule of AFP-V0-012 selects it (V1-0564,
// V1-0739), and each consecutive pair of which is a traversed edge (V1-1491).
// An ENCLOSING_PACKAGE witness is selected without traversal, so its chain is
// the enclosing package alone.
func witnessResolves(graph *affected.Graph, selection affected.Selection) bool {
	witness := selection.Witness
	switch witness.Kind {
	case affected.WitnessPathLiteralReader:
		return slices.Contains(graph.Namers(witness.DirtyPath), selection.UnitID)
	case affected.WitnessUnboundedReader:
		return slices.Contains(graph.UnboundedReaders(), selection.UnitID)
	case affected.WitnessDeclaredReadScope:
		unit, found := graph.Unit(selection.UnitID)
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		return found && unit.ReadScoped && len(files) != 0 && affected.InReadScope(path.Dir(files[0]), unit.ReadScope, witness.DirtyPath)
	case affected.WitnessDeclaredExistenceReader:
		unit, found := graph.Unit(selection.UnitID)
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		return found && len(files) != 0 && slices.Contains(graph.ExistenceReaderPackages(), path.Dir(files[0]))
	}
	if len(witness.Via) == 0 || witness.Via[len(witness.Via)-1] != selection.UnitID || !viaEdges(graph, witness.Via) {
		return false
	}
	origin := witness.Via[0]
	if witness.Kind == affected.WitnessEnclosingPackage {
		return len(witness.Via) == 1 && enclosesDataPath(graph, origin, witness.DirtyPath, false)
	}
	owner, owned := graph.OwnerOf(witness.DirtyPath)
	switch {
	case owned && owner == origin:
		return true
	case golang.New().Owns(witness.DirtyPath):
		if owned {
			return false
		}
		kind, found := goSourceOrigin(graph, origin, witness.DirtyPath)
		return found && (len(witness.Via) > 1 || witness.Kind == kind)
	}
	// Rule (b) traverses from the package directly holding the data path and
	// from every enclosing package that embeds; the package itself carries the
	// ENCLOSING_PACKAGE witness, so a chain from it reaches past it.
	return len(witness.Via) > 1 && enclosesDataPath(graph, origin, witness.DirtyPath, true)
}

// viaEdges reports that every consecutive pair of a witness chain is a graph
// edge the selector traverses (V1-1491): the later unit imports, or
// advisorily imports, the earlier one; the final hop may also be a test-only
// import or a built-binary exec, which the selector takes one edge past a
// reached unit and never expands.
func viaEdges(graph *affected.Graph, via []string) bool {
	for index := 1; index < len(via); index++ {
		unit, found := graph.Unit(via[index])
		if !found {
			return false
		}
		edges := append(append([]string(nil), unit.Imports...), unit.AdvisoryImports...)
		if index == len(via)-1 {
			edges = append(append(edges, unit.TestImports...), unit.Execs...)
		}
		if !slices.Contains(edges, via[index-1]) {
			return false
		}
	}
	return true
}

// enclosesDataPath reports that id is a Go package whose directory encloses a
// dirty path that is not Go source; another language may own it. With
// traversed, the package must also sit in the path's own directory or embed,
// the rule (b) condition for reaching its dependents.
func enclosesDataPath(graph *affected.Graph, id, dirtyPath string, traversed bool) bool {
	if golang.New().Owns(dirtyPath) {
		return false
	}
	directory, found := goPackageDirectory(graph, id)
	if !found || (directory != "." && !strings.HasPrefix(dirtyPath, directory+"/")) {
		return false
	}
	unit, _ := graph.Unit(id)
	return !traversed || directory == path.Dir(dirtyPath) || unit.Embeds
}

// goSourceOrigin is rule (a) for an unindexed Go path: the package in its
// directory, a DIRECT_SOURCE_CHANGE, or, with none there, a unit importing an
// absent Go package named by the directory's last component (any absent
// package at the root), a DEPENDENCY_PATH. It returns the kind the rule gives
// id, and whether the rule selects id at all.
func goSourceOrigin(graph *affected.Graph, id, dirtyPath string) (string, bool) {
	directory := path.Dir(dirtyPath)
	for _, candidate := range graph.UnitIDs() {
		if present, found := goPackageDirectory(graph, candidate); found && present == directory {
			return affected.WitnessDirectSource, candidate == id
		}
	}
	unit, found := graph.Unit(id)
	if !found {
		return "", false
	}
	for _, target := range append(append([]string(nil), unit.Imports...), unit.TestImports...) {
		if _, known := graph.Unit(target); known || !strings.HasPrefix(target, "go:") {
			continue
		}
		if directory == "." || path.Base(target) == path.Base(directory) || target == "go:"+path.Base(directory) {
			return affected.WitnessDependency, true
		}
	}
	return "", false
}

// goPackageDirectory is the directory of a Go unit's files.
func goPackageDirectory(graph *affected.Graph, id string) (string, bool) {
	unit, found := graph.Unit(id)
	if !found || !strings.HasPrefix(id, "go:") {
		return "", false
	}
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	if len(files) == 0 {
		return "", false
	}
	return path.Dir(files[0]), true
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

// TestPythonSelectionNarrowsARealCorpus is the non-Go end-to-end proof
// (AFP-V0-186). It runs over the tracked layered corpus in
// testdata/python-corpus (10 modules, 21 test files): a one-file edit must
// narrow the full test set to exactly the tests whose imports reach the edited
// module, including the paired test of the module that depends on it. The
// 18-module corpus this test first named is not tracked, so it used to skip.
func TestPythonSelectionNarrowsARealCorpus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "python-corpus"))
	if err != nil {
		t.Fatal(err)
	}
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
	if total != 21 {
		t.Fatalf("python corpus carries %d tests, want 21", total)
	}
	const edited = "src/corpus/index.py"
	if _, owned := graph.OwnerOf(edited); !owned {
		t.Fatalf("%s is not owned by a unit", edited)
	}
	plan := affected.Select(graph, []string{edited})
	selected := plan.SelectedTests()
	sort.Strings(selected)
	want := []string{"tests/test_cache.py", "tests/test_cache_eviction.py", "tests/test_cli.py", "tests/test_cli_help.py",
		"tests/test_index.py", "tests/test_index_build.py", "tests/test_query.py", "tests/test_query_filters.py"}
	if !slices.Equal(selected, want) {
		t.Fatalf("editing %s selected %v, want %v", edited, selected, want)
	}
	t.Logf("python: editing %s selects %d of %d tests (%.0f%% narrowed)",
		edited, len(selected), total, 100*(1-float64(len(selected))/float64(total)))
}
