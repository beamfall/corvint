package affected

import (
	json "encoding/json/v2"
	"slices"
	"sort"
	"strings"
)

// Witness kinds. Every selected unit names exactly one machine-checkable
// witness, per LPCV-V0-013.
const (
	WitnessDirectSource = "DIRECT_SOURCE_CHANGE"
	WitnessDirectTest   = "DIRECT_TEST_CHANGE"
	WitnessDependency   = "DEPENDENCY_PATH"
	// WitnessPathLiteralReader names a unit whose own files carry a string
	// literal naming a dirty path no plugin owns (AFP-V0-021).
	WitnessPathLiteralReader = "PATH_LITERAL_READER"
	// WitnessPathGlobReader names a unit whose own files pass a literal glob
	// pattern, named in Witness.Glob, that matches a dirty path (AFP-V0-401).
	WitnessPathGlobReader = "PATH_GLOB_READER"
	// WitnessEnclosingPackage names a Go package whose directory encloses a
	// dirty path no Go file of it declares: a fixture, an embedded asset, or
	// other data its tests or build may read (V1-0340, AFP-V0-012 rule (b)).
	WitnessEnclosingPackage = "ENCLOSING_PACKAGE"
	// WitnessUnboundedReader names a unit whose reads no literal bounds, so it
	// is selected on any dirty path (AFP-V0-012 rule (d)).
	WitnessUnboundedReader = "UNBOUNDED_READER"
	// WitnessDeclaredReadScope names a unit whose declared read scope holds a
	// dirty path (AFP-V0-023).
	WitnessDeclaredReadScope = "DECLARED_READ_SCOPE"
	// WitnessAdvisoryConstant names a unit reached through at least one
	// advisory edge, such as a Ruby constant reference that autoloading maps
	// to the dirty unit (RUBY-AFFECTED-V0-009): the dependency is inferred
	// from a naming convention, not from an explicit load.
	WitnessAdvisoryConstant = "ADVISORY_CONSTANT_REFERENCE"
	// WitnessRouteDispatch names a unit reached through at least one
	// route-dispatch edge (Unit.RouteDispatchImports), such as a Rails
	// request spec reaching a controller it names by no constant
	// (RUBY-AFFECTED-V0-049).
	WitnessRouteDispatch = "ROUTE_DISPATCH_BOOT"
)

// Exclusion reasons. Every eligible unit that is not selected carries one,
// per LPCV-V0-014.
const (
	ExcludedNoDependencyPath = "NO_DEPENDENCY_PATH_TO_DIRTY_UNIT"
)

// Widening reasons. Any of these makes the plan's scope UNKNOWN, per
// LPCV-V0-016: uncertainty widens rather than disappearing.
const (
	// UnknownUnindexedSourcePath reports a dirty path a language plugin claims
	// as its own source text but which no unit declares. A new file, a rename,
	// or an excluded build variant produces this.
	UnknownUnindexedSourcePath = "UNINDEXED_SOURCE_PATH"
	// UnknownUnownedDirtyPath reports a dirty path no plugin claims. Build
	// configuration, generators, fixtures, and data files land here: they may
	// affect any unit, so the exclusion set cannot be justified.
	UnknownUnownedDirtyPath = "UNOWNED_DIRTY_PATH"
	// UnknownLanguageFrontier reports that a plugin could not resolve part of
	// its own graph exactly.
	UnknownLanguageFrontier = "LANGUAGE_FRONTIER"
	// UnknownNoSelectableTest reports a changed unit (one owning a dirty path)
	// that no selectable test checks under its plugin's rule (AFP-V0-020).
	UnknownNoSelectableTest = "NO_SELECTABLE_TEST"
)

// Scope axes, per the LPCV result-axis rule. Selection produces scope only;
// execution status and currency belong to the runtime provider and the caller.
const (
	ScopeBounded = "BOUNDED"
	ScopeUnknown = "UNKNOWN"
)

// Witness is the reason one unit entered the plan.
//
// Via is the dependency chain from the unit that directly contains DirtyPath to
// the selected unit, inclusive at both ends. For a direct change it holds one
// element.
type Witness struct {
	Kind      string   `json:"kind"`
	DirtyPath string   `json:"dirtyPath"`
	Via       []string `json:"via"`
	// Cause is set on an UNBOUNDED_READER witness only: the unit whose own
	// file makes the selected unit's reads unbounded, and why (AFP-V0-192).
	Cause *UnboundedCause `json:"cause,omitempty"`
	// Glob is set on a PATH_GLOB_READER witness only: the unit's literal glob
	// pattern that matches DirtyPath (AFP-V0-401).
	Glob string `json:"glob,omitempty"`
	// link holds the chain by reference while the selector runs, in place of
	// Via (V1-2979, AFP-V0-641); see via.go.
	link *viaLink
}

// UnboundedCause names why an UNBOUNDED_READER unit can read any dirty path:
// Unit is the unit itself, or the root-locating unit it depends on or whose
// dependent its tests import, and Reads is that unit's unboundedReads.
type UnboundedCause struct {
	Unit  string `json:"unit"`
	Reads string `json:"reads"`
}

// Selection is one unit whose tests the plan requires.
type Selection struct {
	UnitID  string   `json:"unitId"`
	Tests   []string `json:"tests"`
	Witness Witness  `json:"witness"`
}

// Exclusion is a bounded certificate for one eligible unit the plan did not
// select. It names the inspected universe and the condition that invalidates
// it. It is never a claim that the unit's tests cannot fail.
type Exclusion struct {
	UnitID       string `json:"unitId"`
	Reason       string `json:"reason"`
	Universe     string `json:"universe"`
	Invalidation string `json:"invalidation"`
}

// Unknown is one widening reason together with the input that raised it.
type Unknown struct {
	Reason string `json:"reason"`
	Detail string `json:"detail"`
	// Hint is optional operator guidance that never changes selection or
	// scope; Detail stays the bare value consumers key on (AFP-V0-494).
	Hint string `json:"hint,omitempty"`
}

// Plan is the selection result: what to verify, what was deliberately left out
// and why, and what could not be justified at all.
type Plan struct {
	GraphDigest string      `json:"graphDigest"`
	Dirty       []string    `json:"dirty"`
	Scope       string      `json:"scope"`
	Selected    []Selection `json:"selected"`
	Excluded    []Exclusion `json:"excluded"`
	Unknown     []Unknown   `json:"unknown"`
	// NonSource names the dirty paths classified as task state (AFP-V0-088),
	// declared documentation (AFP-V0-160) or declared runtime-readers data
	// (AFP-V0-602), not source: the TASK_STORE_PROJECTION group first, then
	// the DECLARED_DOCUMENTATION group, then one DECLARED_RUNTIME_READERS
	// group per declared entry, then the CORVINT_DECLARATION group of a
	// declaration itself (AFP-V0-443). It is omitted when empty.
	NonSource []NonSource `json:"nonSource,omitempty"`
	// FrontierSites counts the repository's frontier sites by reason,
	// whether or not this dirty set reaches them, so progress toward a
	// bounded scope can be measured (AFP-V0-381). It is omitted when the
	// repository has no frontier.
	FrontierSites map[string]int `json:"frontierSites,omitempty"`
	// FrontierExcluded names each plugin whose graph frontier this plan
	// leaves out of Unknown because the change reaches only terminal units of
	// it, with the reasons left out and those units (AFP-V0-560). It is
	// omitted when no frontier is left out.
	FrontierExcluded []FrontierExclusion `json:"frontierExcluded,omitempty"`
	// GoAdditive holds the additive-change record of each Go package the
	// golang plugin examined, NARROWED or FALLBACK with its reason
	// (ADN-V0-001). It is omitted when the graph carries no record.
	GoAdditive []GoAdditive `json:"goAdditive,omitempty"`
	// GoAdditiveExcluded holds each unit with tests the plan leaves out only
	// because a NARROWED record pruned the import edges it would be reached
	// through (ADN-V0-005). These units are in neither Selected nor Excluded:
	// plan.excluded[].reason and .invalidation are closed enumerations
	// (CCF-V1-007 (d)), so the narrowing is carried in this optional member
	// instead (CCF-V1-006). It is omitted when no unit is left out this way.
	GoAdditiveExcluded []GoAdditiveExclusion `json:"goAdditiveExcluded,omitempty"`
}

// FrontierExclusion records one plugin whose graph frontier a plan does not
// name: no dirty path is that plugin's or unowned, and every reached unit of it
// is a terminal selection, so no edge the frontier hides can carry the change
// further (AFP-V0-560).
type FrontierExclusion struct {
	Language string   `json:"language"`
	Reason   string   `json:"reason"`
	Frontier []string `json:"frontier"`
	Units    []string `json:"units"`
}

// FrontierExclusionReason is the stated reason of every FrontierExclusion.
const FrontierExclusionReason = "no dirty path is this plugin's or unowned, and the change reaches its units only as reader or binary-exec selections the traversal does not continue from, so no edge its frontier hides can change this plan"

// Select computes the plan for one dirty path set.
//
// The traversal is a breadth-first sweep over reverse dependency edges from
// every unit that directly contains a dirty path, except a Go unit whose dirty
// paths are all its own test files, which is selected alone (AFP-V0-036).
// Breadth-first order plus
// sorted expansion makes the witness for each reached unit the shortest chain,
// broken by the lexicographically smallest dirty path, so the plan is
// deterministic for fixed inputs (LPCV-V0-019).
//
// A unit is traversed whether or not it declares tests; it is selected only if
// it declares at least one, because a unit with no tests contributes no check.
// A changed unit that no selectable test checks is named as unknown scope
// instead of being omitted (AFP-V0-020); untestedRules holds each plugin's rule.
// Every dirty path also selects the units whose path literals name it
// (AFP-V0-021); an unowned path stays unknown all the same, except a Tasks
// store projection path no unit names, which is classified as task state in
// plan.NonSource instead (AFP-V0-088), and a path the repository declares as
// documentation, whose readers are selected and which is listed there too
// (AFP-V0-160), and the declaration itself when no unit names it (AFP-V0-443).
//
// Selected units are emitted in the AFP-V0-007 order: witness chain length
// ascending, shared directory prefix with the witness's dirty path descending,
// unit id ascending. Exclusions stay in unit id order.
//
// Every selected witness carries its whole chain in Via; SelectLinked is the
// same plan with the chains shared by reference (AFP-V0-641).
func Select(graph *Graph, dirty []string) Plan {
	plan := selectPlan(graph, dirty)
	plan.MaterializeVia()
	return plan
}

// selectPlan is Select with the witness chains the traversal built still held
// by reference (SelectLinked).
func selectPlan(graph *Graph, dirty []string) Plan {
	normalized := NormalizePaths(dirty)
	plan := Plan{
		GraphDigest: graph.digest,
		Dirty:       normalized,
		Scope:       ScopeBounded,
		Selected:    []Selection{},
		Excluded:    []Exclusion{},
		Unknown:     []Unknown{},
	}
	if len(graph.sites) != 0 {
		plan.FrontierSites = make(map[string]int, len(graph.sites))
		for reason, count := range graph.sites {
			plan.FrontierSites[reason] = count
		}
	}
	reached, seeds, unknown, narrowed, reads, carried, followed, additive := graph.reach(normalized, true)
	plan.GoAdditive = additiveSummary(additive)
	through := graph.additiveExclusions(normalized, additive)
	plan.NonSource, unknown = graph.taskState(unknown)
	documentation, unknown := graph.documentationPaths(unknown)
	plan.NonSource = append(plan.NonSource, documentation...)
	runtimeReaders, unknown := graph.runtimeReaderPaths(unknown)
	plan.NonSource = append(plan.NonSource, runtimeReaders...)
	declaration, unknown := graph.declarationPath(unknown)
	plan.NonSource = append(plan.NonSource, declaration...)
	frontier, excluded := graph.frontierUnknowns(reached, narrowed, reads, carried, followed, normalized, plan.NonSource)
	plan.Unknown = append(frontier, unknown...)
	plan.FrontierExcluded = excluded
	plan.Unknown = append(plan.Unknown, graph.tokenBounds(reached, normalized)...)
	for _, id := range graph.order {
		unit := graph.units[id]
		if _, changed := seeds[id]; changed && graph.untested(id) {
			plan.Unknown = append(plan.Unknown, Unknown{Reason: UnknownNoSelectableTest, Detail: id})
		}
		if len(unit.Tests) == 0 {
			continue
		}
		witness, hit := reached[id]
		if narrowedBy, additiveOnly := through[id]; !hit && additiveOnly {
			plan.GoAdditiveExcluded = append(plan.GoAdditiveExcluded, GoAdditiveExclusion{
				UnitID:       id,
				Through:      narrowedBy,
				Universe:     graph.digest,
				Invalidation: GoAdditiveInvalidation,
			})
			continue
		}
		if !hit {
			plan.Excluded = append(plan.Excluded, Exclusion{
				UnitID:       id,
				Reason:       ExcludedNoDependencyPath,
				Universe:     graph.digest,
				Invalidation: "NEW_DEPENDENCY_EDGE_OR_DIRTY_PATH",
			})
			continue
		}
		plan.Selected = append(plan.Selected, Selection{
			UnitID:  id,
			Tests:   append([]string(nil), unit.Tests...),
			Witness: witness,
		})
	}
	graph.rank(plan.Selected)
	graph.symlinkHints(plan.Unknown)
	if len(plan.Unknown) != 0 {
		plan.Scope = ScopeUnknown
	}
	return plan
}

// reach computes the reached universe once for Select and ReachedUnitIDs, so
// the delta unit denominator cannot drift from the selected plan. followed
// holds the units whose dependents the plan walks: every unit a reverse
// dependency traversal expands, from the dirty paths' owners, from a
// test-less reader of an unowned path (AFP-V0-495), and on any dirty path from
// a root locator (AFP-V0-012 rule (d)). A test user, exec user, path reader,
// own-reason unbounded reader or scoped reader is selected without its
// dependents (AFP-V0-581). carried holds the units testlessReaderUsers
// traversed (AFP-V0-560).
//
// ownTests false leaves out the step that selects a test-only Go unit for its
// own changed test files, so the caller sees only what else reached it
// (ReachedBeyondOwnTests). No later step propagates from that step's entries:
// builtCommands runs before it, and the readers and test-less reader steps
// expand only from paths and readers, so dropping it changes no other unit.
//
// additive holds the ADN-V0-001 records with their final states.
func (graph *Graph) reach(normalized []string, ownTests bool) (reached, seeds map[string]Witness, unknown []Unknown, narrowed, reads, carried, followed map[string]bool, additive []GoAdditive) {
	seeds, unknown = graph.seed(normalized)
	changed, traversed, enclosing := graph.goStructure(normalized)
	contained := graph.testOnlyGoUnits(seeds, normalized, changed, traversed)
	mergeWitnesses(seeds, changed)
	start := make(map[string]Witness, len(seeds)+len(traversed))
	mergeWitnesses(start, seeds)
	mergeWitnesses(start, traversed)
	for id := range contained {
		delete(start, id)
	}
	reached, allowed, additive := graph.additiveTraverse(start, traversed, enclosing, normalized)
	followed = make(map[string]bool, len(reached))
	for id := range reached {
		followed[id] = true
	}
	if len(normalized) != 0 {
		for id := range graph.locatorReach {
			followed[id] = true
		}
	}
	built := graph.builtCommands(reached, enclosing)
	graph.testUsersOf(reached, allowed)
	if ownTests {
		for id := range contained {
			reached[id] = seeds[id]
		}
	}
	mergeWitnesses(reached, enclosing)
	graph.execUsersOf(reached, built)
	narrowed, reads = graph.readers(reached, normalized)
	carried = graph.testlessReaderUsers(reached, normalized, followed)
	graph.unboundedReadersOf(reached, normalized)
	graph.scopedReadersOf(reached, normalized)
	return reached, seeds, unknown, narrowed, reads, carried, followed, additive
}

func relativeDirectory(relative string) string {
	index := strings.LastIndex(relative, "/")
	if index < 0 {
		return "."
	}
	return relative[:index]
}

// rank orders selections in place by the three AFP-V0-007 keys: the length of
// the witness chain (reverse-dependency distance from the dirty path), then the
// unit's proximity to the witness's dirty path (longer shared directory prefix
// first), then unit id. Every key is a function of the graph and the dirty set,
// so the order is as deterministic as the plan itself.
func (graph *Graph) rank(selected []Selection) {
	proximity := make(map[string]int, len(selected))
	for _, selection := range selected {
		proximity[selection.UnitID] = graph.proximity(selection)
	}
	sort.SliceStable(selected, func(i, j int) bool {
		left, right := selected[i], selected[j]
		if left.Witness.ViaLength() != right.Witness.ViaLength() {
			return left.Witness.ViaLength() < right.Witness.ViaLength()
		}
		if proximity[left.UnitID] != proximity[right.UnitID] {
			return proximity[left.UnitID] > proximity[right.UnitID]
		}
		return left.UnitID < right.UnitID
	})
}

// proximity is the longest run of leading directory components any of the
// unit's sources or tests shares with the witness's dirty path.
func (graph *Graph) proximity(selection Selection) int {
	dirty := directoryComponents(selection.Witness.DirtyPath)
	unit := graph.units[selection.UnitID]
	longest := 0
	for _, owned := range unit.Sources {
		longest = max(longest, sharedPrefix(directoryComponents(owned), dirty))
	}
	for _, owned := range unit.Tests {
		longest = max(longest, sharedPrefix(directoryComponents(owned), dirty))
	}
	return longest
}

// directoryComponents splits the directory part of a canonical relative path;
// a path at the repository root has none.
func directoryComponents(value string) []string {
	slash := strings.LastIndex(value, "/")
	if slash < 0 {
		return nil
	}
	return strings.Split(value[:slash], "/")
}

func sharedPrefix(left, right []string) int {
	count := 0
	for count < len(left) && count < len(right) && left[count] == right[count] {
		count++
	}
	return count
}

// seed maps every dirty path to the unit that declares it, and reports the
// paths that could not be mapped. A unit's witness is its smallest dirty
// source path, else its smallest dirty test path, so a unit that propagates to
// its dependents names a path that can reach them (AFP-V0-036).
func (graph *Graph) seed(dirty []string) (map[string]Witness, []Unknown) {
	seeds := make(map[string]Witness)
	unknown := make([]Unknown, 0)
	var packages map[string]string
	for _, path := range dirty {
		id, owned := graph.owner[path]
		if !owned {
			// A path inside a runnable unobserved directory also names that
			// directory's own test root, whether or not a plugin claims it
			// (AFP-V0-650); the observed side's selection is unchanged.
			module, inModule := graph.unobservedModule(path)
			if packages == nil {
				packages = graph.goPackageDirectories()
			}
			if graph.goReadData(path, packages) {
				// Every unit that can read it is selected (AFP-V0-600,
				// AFP-V0-601); it stays unowned for frontierUnknowns.
				if inModule {
					unknown = append(unknown, module)
				}
				continue
			}
			if graph.deletedGoTest(path, packages) {
				// Rule (a) selects its base package (AFP-V0-620). A path
				// inside a runnable unobserved directory still names that
				// directory's test root, as the read-data branch does.
				if inModule {
					unknown = append(unknown, module)
				}
				continue
			}
			entry := graph.unownedUnknown(path)
			if inModule && entry != module {
				unknown = append(unknown, module)
			}
			unknown = append(unknown, entry)
			continue
		}
		kind := graph.witnessKind(id, path)
		if existing, already := seeds[id]; already && (existing.Kind != WitnessDirectTest || kind != WitnessDirectSource) {
			continue
		}
		seeds[id] = Witness{Kind: kind, DirtyPath: path, Via: []string{id}}
	}
	return seeds, unknown
}

// testOnlyGoUnits names the Go units every dirty path of which is one of the
// unit's own declared `_test.go` files, and which no structural rule also
// changes (AFP-V0-036). Go compiles a test file only into its own package's
// test binary, and no other package can import it, so such a unit is selected
// for its own tests and is not a traversal root: neither its importers nor the
// units whose tests import it are reached through it. Every other plugin, an
// unindexed test path (rule (a)) and a data path (rule (b)) keep the
// conservative traversal; path readers are attributed per dirty path as before.
func (graph *Graph) testOnlyGoUnits(seeds map[string]Witness, dirty []string, changed, traversed map[string]Witness) map[string]bool {
	contained := make(map[string]bool)
	for id, witness := range seeds {
		if strings.HasPrefix(id, "go:") && witness.Kind == WitnessDirectTest {
			contained[id] = true
		}
	}
	for _, path := range dirty {
		id, owned := graph.owner[path]
		if owned && (!strings.HasSuffix(path, "_test.go") || graph.witnessKind(id, path) != WitnessDirectTest) {
			delete(contained, id)
		}
	}
	for id := range changed {
		delete(contained, id)
	}
	for id := range traversed {
		delete(contained, id)
	}
	return contained
}

// frontierUnknowns names, in reason order, the frontier of every participating
// plugin, the unit frontier of every reached unit (V1-0289) and of every
// narrowed reader (AFP-V0-072(4)) whose bearing condition holds
// (AFP-V0-581). A dirty path no plugin owns also
// raises, from every other plugin, each frontier entry whose hidden edge could
// bind that path, and every entry of a plugin that reads the path as an input
// to its resolution (AFP-V0-511). A clean plan names none. It also returns, in
// language order, the plugins left out only because the change reaches
// nothing of them but terminal units, with the graph frontier reasons no other
// source names (AFP-V0-560); a plan with a dirty path no plugin owns leaves no
// plugin out that way, and a unit the plan follows the dependents of
// (AFP-V0-581) is no terminal selection.
func (graph *Graph) frontierUnknowns(reached map[string]Witness, narrowed, reads, carried, followed map[string]bool, dirty []string, nonSource []NonSource) ([]Unknown, []FrontierExclusion) {
	reasons := make(map[string]bool)
	involved, unowned, terminal := graph.participants(reached, carried, followed, dirty, nonSource)
	participating := make(map[string]bool, len(involved))
	for _, name := range involved {
		participating[name] = true
		for _, reason := range graph.frontierBy[name] {
			reasons[reason] = true
		}
	}
	if len(unowned) > 0 {
		fixtures := graph.hostedFixtureDirectories(unowned)
		for _, name := range graph.languages {
			if participating[name] {
				continue
			}
			closed := fixtures
			if graph.listsBelowAny(name, fixtures) {
				closed = nil
			}
			steered := graph.readsResolutionInput(name, unowned)
			for _, entry := range graph.frontierBy[name] {
				if steered || graph.bearsOnUnowned(name, entry, closed) {
					reasons[entry] = true
				}
			}
		}
	}
	for id, witness := range reached {
		if graph.frontierBears(id, witness, followed, reads) {
			for _, reason := range graph.units[id].Frontier {
				reasons[reason] = true
			}
		}
	}
	for id := range narrowed {
		// A narrowed reader is unreached. Reached, it would be a path reader,
		// whose frontier bears whatever the plan follows (AFP-V0-581 (2)), so
		// the narrowing removes no entry (AFP-V0-072(4)).
		for _, reason := range graph.units[id].Frontier {
			reasons[reason] = true
		}
	}
	unknown := make([]Unknown, 0, len(reasons))
	for _, reason := range sortedKeys(reasons) {
		unknown = append(unknown, Unknown{Reason: UnknownLanguageFrontier, Detail: reason})
	}
	var excluded []FrontierExclusion
	names := make([]string, 0, len(terminal))
	for name := range terminal {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		left := make([]string, 0, len(graph.frontierBy[name]))
		for _, reason := range graph.frontierBy[name] {
			if !reasons[reason] {
				left = append(left, reason)
			}
		}
		if len(left) == 0 {
			continue
		}
		sort.Strings(left)
		units := terminal[name]
		sort.Strings(units)
		excluded = append(excluded, FrontierExclusion{Language: name, Reason: FrontierExclusionReason, Frontier: left, Units: units})
	}
	return unknown, excluded
}

// frontierBears reports whether unit id's Frontier bears on a plan that
// reached it with this witness and followed set: always (V1-0289), or only
// when the plan follows its dependents or the unit's own code reads a dirty
// path (AFP-V0-581). A path reader's own code may read the path, so what the
// unit does can change with it and the hidden edges from its dependents bear;
// a plan that follows no dependents of it is where the frontier is the only
// thing that names them. Whether it reads one is decided by its own literals
// and globs, not by which witness won: reads holds each unit reached first
// some other way whose own literal or glob also names a dirty path. The
// selections that concern only the unit's tests (a test user, a declared-scope
// reader, an own-reason unbounded reader that does not locate the root, an
// executable user) leave it unnamed when it reads no dirty path.
func (graph *Graph) frontierBears(id string, witness Witness, followed, reads map[string]bool) bool {
	if graph.units[id].FrontierBears != FrontierBearsOnDependents || followed[id] || reads[id] {
		return true
	}
	return witness.Kind == WitnessPathLiteralReader || witness.Kind == WitnessPathGlobReader
}

// participants lists the plugins a plan with this dirty set takes part in: one
// that owns a dirty path or a unit the change propagates through, and one whose
// units read any path. It also returns, sorted, every dirty path owned by
// none, since a path no plugin claims may be an edge any plugin could not
// resolve; frontierUnknowns asks each remaining plugin which of its frontier
// entries could bind such a path (AFP-V0-511). A path classified as task state
// or declared documentation is no such edge, so it involves no plugin
// (AFP-V0-088, AFP-V0-160). An empty dirty set changes nothing a frontier could
// hide an edge from, so no plugin takes part in it (V1-2126). A unit reached
// only as a terminal selection (terminalWitness) that no traversal carried the
// change through (carried) and whose dependents the plan does not follow
// (followed, AFP-V0-581) involves no plugin: the third result maps each
// plugin left out that way to those units (AFP-V0-560). Once a dirty path is
// owned by none, the terminal rule is off and such a plugin takes part, as a
// plugin with a reached unit does under AFP-V0-511 (3): the AFP-V0-560
// soundness argument needs an unowned path to make every reached plugin take
// part, so the two rules never compose into a narrower frontier than either.
func (graph *Graph) participants(reached map[string]Witness, carried, followed map[string]bool, dirty []string, nonSource []NonSource) (involved, unowned []string, terminal map[string][]string) {
	if len(dirty) == 0 {
		return nil, nil, nil
	}
	names := make(map[string]bool)
	terminal = make(map[string][]string)
	for id, witness := range reached {
		name, _, _ := strings.Cut(id, ":")
		// An unbounded reader keeps its plugin whatever its witness: the
		// AFP-V0-560 argument covers only readers whose reads are bounded.
		if _, unbounded := graph.causes[id]; !unbounded && !carried[id] && !followed[id] && terminalWitness(witness) {
			terminal[name] = append(terminal[name], id)
			continue
		}
		names[name] = true
	}
	for _, name := range graph.languages {
		if reader, reads := graph.claimants[name].(PathReader); reads && reader.ReadsAnyPath() {
			names[name] = true
		}
	}
	taskState := make(map[string]bool)
	for _, group := range nonSource {
		for _, path := range group.Paths {
			taskState[path] = true
		}
	}
	paths := make(map[string]bool)
	for _, path := range dirty {
		if taskState[path] {
			continue
		}
		claimed := graph.claimantsOf(path)
		if len(claimed) == 0 {
			paths[path] = true
		}
		for _, name := range claimed {
			names[name] = true
		}
	}
	if len(paths) > 0 {
		for name := range terminal {
			names[name] = true
		}
		terminal = nil
	}
	for name := range names {
		delete(terminal, name)
	}
	return sortedKeys(names), sortedKeys(paths), terminal
}

// hostedFixtureDirectories returns, for the dirty paths no plugin owns, the
// `testdata` directories they lie below when a Go unit hosts every one of them
// (AFP-V0-446's hosting rule); it returns nil as soon as one path is no such
// fixture, because the Go-fixture rule then holds for none of the plan.
func (graph *Graph) hostedFixtureDirectories(unowned []string) []string {
	packages := graph.goPackageDirectories()
	directories := make(map[string]bool, len(unowned))
	for _, path := range unowned {
		directory := fixtureDirectory(path)
		if directory == "" || !graph.goUnitAbove(directory, packages) {
			return nil
		}
		directories[directory] = true
	}
	return sortedKeys(directories)
}

// bearsOnUnowned reports whether one frontier entry of a plugin that neither
// owns a dirty path nor reaches a changed unit could still hide an edge to a
// dirty path no plugin owns (AFP-V0-511). It fails closed: an entry is dropped
// only when the plugin declares its reason source-bound, or declares it
// fixture-closed while every unowned path lies in one of the given Go-hosted
// `testdata` directories, below which the plugin lists no path, and the entry
// locates no site there. A `+N` bound hides sites the rule cannot inspect, so
// it is kept.
func (graph *Graph) bearsOnUnowned(name, entry string, fixtures []string) bool {
	fields := strings.SplitN(entry, ":", 4)
	if len(fields) < 2 {
		return true
	}
	reason := fields[0] + ":" + fields[1]
	if graph.sourceBound[name][reason] {
		return false
	}
	if !graph.fixtureClosed[name][reason] || len(fixtures) == 0 {
		return true
	}
	site := ""
	if len(fields) > 2 {
		site = fields[2]
	}
	if strings.HasPrefix(site, "+") {
		return true
	}
	for _, directory := range fixtures {
		if strings.HasPrefix(site, directory+"/") {
			return true
		}
	}
	return false
}

// readsResolutionInput reports whether the named plugin reads one of the dirty
// paths no plugin owns as an input to its own resolution (AFP-V0-511): a
// manifest, build or runner configuration it scans without owning. A change
// there may rebind any site, so the plugin's whole frontier is kept, as it is
// when the plugin declares no FrontierReach at all.
func (graph *Graph) readsResolutionInput(name string, unowned []string) bool {
	reach, declares := graph.claimants[name].(FrontierReach)
	if !declares {
		return true
	}
	for _, path := range unowned {
		if reach.ResolutionInput(path) {
			return true
		}
	}
	return false
}

// listsBelowAny reports whether any unit of the named plugin lists a path
// below one of the directories.
func (graph *Graph) listsBelowAny(name string, directories []string) bool {
	for path, id := range graph.owner {
		if !strings.HasPrefix(id, name+":") {
			continue
		}
		for _, directory := range directories {
			if strings.HasPrefix(path, directory+"/") {
				return true
			}
		}
	}
	return false
}

// terminalWitness reports whether a reached unit is a selection the traversal
// does not continue from (AFP-V0-560): a literal or glob reader (AFP-V0-021,
// AFP-V0-401), a declared read scope (AFP-V0-023) or a binary-exec runner
// (AFP-V0-037). The caller still counts one that testlessReaderUsers traversed
// (AFP-V0-495). An unbounded reader is no terminal selection: the unbounded set
// is the traversal closure of the root locators (unboundedReaders), so a hidden
// edge into a locator would widen it.
func terminalWitness(witness Witness) bool {
	switch witness.Kind {
	case WitnessPathLiteralReader, WitnessPathGlobReader, WitnessDeclaredReadScope, WitnessBinaryExec:
		return true
	}
	return false
}

// claimantsOf lists the plugins that own path: the namespace of the unit that
// declares it, and every plugin whose Owns claims it. A file a Go package
// embeds or other attributed read data (goReadData, AFP-V0-600, AFP-V0-601)
// is claimed by none: it stays an unowned path for every frontier rule.
func (graph *Graph) claimantsOf(path string) []string {
	claimed := make([]string, 0, 1)
	if id, owned := graph.owner[path]; owned {
		name, _, _ := strings.Cut(id, ":")
		claimed = append(claimed, name)
	}
	for _, name := range graph.languages {
		if graph.claimants[name].Owns(path) {
			claimed = append(claimed, name)
		}
	}
	return claimed
}

// goReadData reports whether a dirty path no unit lists is data whose every
// possible reader the plan selects, so it raises no UNOWNED_DIRTY_PATH
// (AFP-V0-600, AFP-V0-601). Two things must hold. The path is a file a Go
// package embeds (AFP-V0-600), a file below a `testdata` directory a Go unit
// hosts (AFP-V0-446), a file a Go unit's declared read scope names by an entry
// or a declared subtree, or a file in the subtree of a Go unit whose read
// scope is declared (AFP-V0-023). And no test process can read it unseen
// (confinedReaders): every unit of the graph is a Go unit, and each one with
// tests either encloses the path, so rule (b) selects it (goDataRule), or has
// a declared read scope, whose CI confinement fails it on any read the
// declaration leaves out and whose scope selects it on any read it names. A
// reader by a computed path, in any language, is then impossible; without
// that, one can exist that no rule names. A build or CI driver input
// (driverInput) is never attributed, embedded or not: an executor outside the
// graph consumes it. A task store projection or declared documentation path is
// left to its own non-source class (AFP-V0-088, AFP-V0-160), which
// reclassifies its UNOWNED entry. The path stays unowned for claimantsOf and
// every frontier rule either way.
func (graph *Graph) goReadData(path string, packages map[string]string) bool {
	if _, goGraph := graph.claimants["go"]; !goGraph || driverInput(path) || IsTaskStoreProjectionPath(path) || graph.declaredDocumentation(path) {
		return false
	}
	return graph.namedGoReadData(path, packages) && graph.confinedReaders(path, packages)
}

// namedGoReadData reports whether a rule the plan applies names a Go reader of
// path: an embed, a hosted fixture or a declared read scope (goReadData).
func (graph *Graph) namedGoReadData(path string, packages map[string]string) bool {
	if len(graph.embedded[path]) != 0 {
		return true
	}
	if directory := fixtureDirectory(path); directory != "" && graph.goUnitAbove(directory, packages) {
		return true
	}
	for _, id := range graph.order {
		unit := graph.units[id]
		if !unit.ReadScoped || !strings.HasPrefix(id, "go:") {
			continue
		}
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		if len(files) == 0 {
			continue
		}
		if directory := relativeDirectory(files[0]); directory != "." && strings.HasPrefix(path, directory+"/") {
			return true
		}
		for _, entry := range unit.ReadScope {
			if path == entry || (strings.HasSuffix(entry, "/") && strings.HasPrefix(path, entry)) {
				return true
			}
		}
	}
	return false
}

// confinedReaders reports whether every test process of the graph either is
// selected for path by rule (b) or runs confined to a declared read scope
// (goReadData): every unit is a Go unit, and each one with tests is a package
// enclosing path or has a declared read scope. One unit of another plugin, or
// one undeclared Go package elsewhere with tests, may read path through a
// computed name no rule resolves, so the answer is then false.
func (graph *Graph) confinedReaders(path string, packages map[string]string) bool {
	enclosing := make(map[string]bool)
	for directory := relativeDirectory(path); ; directory = relativeDirectory(directory) {
		if id, present := packages[directory]; present {
			enclosing[id] = true
		}
		if directory == "." {
			break
		}
	}
	for _, id := range graph.order {
		if !strings.HasPrefix(id, "go:") {
			return false
		}
		unit := graph.units[id]
		if len(unit.Tests) != 0 && !unit.ReadScoped && !enclosing[id] {
			return false
		}
	}
	return true
}

// driverInput reports a path a build or CI executor outside every plugin
// runs as its own configuration (AFP-V0-601): a make file at any depth, or a
// GitHub Actions workflow or action definition.
func driverInput(path string) bool {
	switch base := path[strings.LastIndex(path, "/")+1:]; {
	case base == "Makefile" || base == "makefile" || base == "GNUmakefile" || strings.HasSuffix(base, ".mk"):
		return true
	}
	return strings.HasPrefix(path, ".github/workflows/") || strings.HasPrefix(path, ".github/actions/")
}

// unownedUnknown distinguishes a dirty path some plugin claims as its own
// source text — an unindexed source file — from a path no plugin claims at
// all. A claimed path at or below a directory that plugin reported as
// unobserved is that plugin's frontier instead: the file is known to lie
// outside the observed graph, not merely to be missing from it (AFP-V0-191).
func (graph *Graph) unownedUnknown(path string) Unknown {
	reason := UnknownUnownedDirtyPath
	for _, name := range graph.languages {
		if graph.claimants[name] == nil || !graph.claimants[name].Owns(path) {
			continue
		}
		if directory, ok := underDirectory(path, graph.unobserved[name]); ok {
			category := FrontierUnobservedDirectory
			if slices.Contains(graph.runnable[name], directory) {
				category = FrontierUnobservedModule
			}
			return Unknown{Reason: UnknownLanguageFrontier, Detail: name + ":" + category + ":" + directory + ":" + path}
		}
		reason = UnknownUnindexedSourcePath
	}
	return Unknown{Reason: reason, Detail: path}
}

// FrontierUnobservedDirectory is the LANGUAGE_FRONTIER detail category of a
// dirty path inside a directory its plugin owns but does not observe; the
// detail is "namespace:unobserved-directory:<directory>:<path>" (AFP-V0-191).
const FrontierUnobservedDirectory = "unobserved-directory"

// FrontierUnobservedModule is the LANGUAGE_FRONTIER detail category of a
// dirty path inside an unobserved directory its plugin reported as runnable:
// "namespace:unobserved-module:<directory>:<path>" (AFP-V0-650). The plan
// stays UNKNOWN, because no unit of the directory is indexed; `corvint test`
// admits it only by running the directory's own test root.
const FrontierUnobservedModule = "unobserved-module"

// unobservedModule returns the unobserved-module frontier of path when the
// longest unobserved directory holding it, for some plugin, is one that
// plugin reported as runnable (AFP-V0-650). A deeper unobserved directory
// that is not runnable wins, so a nested module inside a runnable one keeps
// its own frontier.
func (graph *Graph) unobservedModule(path string) (Unknown, bool) {
	for _, name := range graph.languages {
		if directory, ok := underDirectory(path, graph.unobserved[name]); ok && slices.Contains(graph.runnable[name], directory) {
			return Unknown{Reason: UnknownLanguageFrontier, Detail: name + ":" + FrontierUnobservedModule + ":" + directory + ":" + path}, true
		}
	}
	return Unknown{}, false
}

// underDirectory returns the longest of the sorted directories that path lies
// below, and whether there is one.
func underDirectory(path string, directories []string) (string, bool) {
	found := ""
	for _, directory := range directories {
		if strings.HasPrefix(path, directory+"/") && len(directory) > len(found) {
			found = directory
		}
	}
	return found, found != ""
}

func (graph *Graph) witnessKind(id, path string) string {
	for _, test := range graph.units[id].Tests {
		if test == path {
			return WitnessDirectTest
		}
	}
	return WitnessDirectSource
}

// untestedRules names, per plugin, when a changed unit has no selectable test
// (AFP-V0-020). Go tests are package-scoped, so a package is untested when it
// declares none of its own. Every other plugin may keep tests in the unit itself
// or in units that depend on it, so a unit is untested when no unit it reaches,
// itself included, declares a test.
var untestedRules = map[string]func(*Graph, string) bool{
	"go": func(graph *Graph, id string) bool { return len(graph.units[id].Tests) == 0 },
}

func (graph *Graph) untested(id string) bool {
	rule, own := untestedRules[strings.SplitN(id, ":", 2)[0]]
	if own {
		return rule(graph, id)
	}
	return !graph.testReach[id]
}

// traverse walks reverse dependency edges breadth-first from the seeds. Each
// reached unit's chain is one link onto its parent's, so a chain is never
// copied (V1-2979, AFP-V0-641). A unit in allowed is a narrowed additive
// change whose import edges lead only to the importers its set holds
// (ADN-V0-003); nil follows every edge.
func (graph *Graph) traverse(seeds map[string]Witness, allowed map[string]map[string]bool) map[string]Witness {
	reached := make(map[string]Witness, len(seeds))
	frontier := make([]string, 0, len(seeds))
	for id := range seeds {
		seed := seeds[id]
		seed.link, seed.Via = seed.chainLink(), nil
		reached[id] = seed
		frontier = append(frontier, id)
	}
	sort.Strings(frontier)
	for len(frontier) != 0 {
		next := make([]string, 0, len(frontier))
		// Explicit import edges are taken before advisory ones, and those
		// before route-dispatch ones, at each depth, so a unit reached
		// several ways keeps the most specific witness. A chain with a
		// route-dispatch edge is ROUTE_DISPATCH_BOOT, else one with an
		// advisory edge is ADVISORY_CONSTANT_REFERENCE (RUBY-AFFECTED-V0-049).
		for class, edges := range []map[string][]string{graph.dependents, graph.advisory, graph.routed} {
			for _, id := range frontier {
				for _, dependent := range edges[id] {
					if _, seen := reached[dependent]; seen || (allowed != nil && !graph.follows(allowed, class, id, dependent)) {
						continue
					}
					parent := reached[id]
					kind := WitnessDependency
					switch {
					case class == 2 || parent.Kind == WitnessRouteDispatch:
						kind = WitnessRouteDispatch
					case class == 1 || parent.Kind == WitnessAdvisoryConstant:
						kind = WitnessAdvisoryConstant
					}
					reached[dependent] = Witness{
						Kind:      kind,
						DirtyPath: parent.DirtyPath,
						link:      parent.extended(dependent),
					}
					next = append(next, dependent)
				}
			}
		}
		sort.Strings(next)
		frontier = next
	}
	return reached
}

// testUsersOf adds every unit whose tests alone import a reached unit. The test
// user is selected one edge past the reached unit and is never expanded further:
// its importers do not compile its tests. Reached units are visited by witness
// length, then id, so each test user takes the shortest chain. A narrowed
// additive change's test users are limited to its allowed set (ADN-V0-003).
func (graph *Graph) testUsersOf(reached map[string]Witness, allowed map[string]map[string]bool) {
	ids := make([]string, 0, len(reached))
	for id := range reached {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := reached[ids[i]].ViaLength(), reached[ids[j]].ViaLength()
		if left != right {
			return left < right
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		parent := reached[id]
		set, narrowed := allowed[id]
		for _, user := range graph.testUsers[id] {
			if _, seen := reached[user]; seen || (narrowed && !set[user]) {
				continue
			}
			reached[user] = Witness{
				Kind:      WitnessDependency,
				DirtyPath: parent.DirtyPath,
				link:      parent.extended(user),
			}
		}
	}
}

// NormalizePaths sorts and deduplicates a dirty path set and drops anything not
// in canonical repository-relative form.
func NormalizePaths(values []string) []string {
	seen := make(map[string]bool, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if !ValidRelativePath(value) || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

// Canonical returns the deterministic JSON body of a plan. This is the receipt
// payload an agent consumes and a digest may be taken over.
func (plan Plan) Canonical() ([]byte, error) {
	return json.Marshal(plan, json.Deterministic(true))
}

// SelectedTests flattens the plan's selected tests into one sorted path list.
func (plan Plan) SelectedTests() []string {
	tests := make([]string, 0, len(plan.Selected))
	for _, selection := range plan.Selected {
		tests = append(tests, selection.Tests...)
	}
	sort.Strings(tests)
	return tests
}
