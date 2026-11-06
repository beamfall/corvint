package affected

import (
	json "encoding/json/v2"
	"fmt"
	"slices"
	"sort"
	"strconv"
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
	// UnknownUnselectedBinaryConsumer reports a file the project declares as
	// running the built binary of a Go command this change reaches, when no
	// selected unit carries it as a test (AFP-V0-762).
	UnknownUnselectedBinaryConsumer = "UNSELECTED_BINARY_CONSUMER"
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
	// pattern that matches DirtyPath (AFP-V0-401), and on a
	// DECLARED_TEST_OWNER witness: the owner pattern (AFP-V0-2482).
	Glob string `json:"glob,omitempty"`
	// link holds the chain by reference while the selector runs, in place of
	// Via (V1-2979, AFP-V0-641); see via.go.
	link *viaLink
}

// MarshalJSON labels the representative unbounded dirty path as any path (TSP-V0-7303).
func (witness Witness) MarshalJSON() ([]byte, error) {
	type wire Witness
	return json.Marshal(struct {
		wire
		AnyDirtyPath bool `json:"anyDirtyPath,omitzero"`
	}{wire: wire(witness), AnyDirtyPath: witness.Kind == WitnessUnboundedReader})
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
	// Kind is SelectionKindDriverTarget or SelectionKindCIJob for a driver
	// unit (AZU-V0-001), empty for a unit whose Tests the runner runs.
	Kind string `json:"kind,omitempty"`
	// Argv runs a selected make target from the repository root.
	Argv []string `json:"argv,omitempty"`
	// Workflow and Job name a selected workflow job, left to CI.
	Workflow string `json:"workflow,omitempty"`
	Job      string `json:"job,omitempty"`
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
	GraphDigest string   `json:"graphDigest"`
	Dirty       []string `json:"dirty"`
	// Deleted names, in Dirty order, each dirty path absent from the tree
	// the graph was observed from: a file the change deletes or renames
	// away (AFP-V0-2206). The caller fills it; it is omitted when no dirty
	// path is absent and in a sparse checkout, where an absent path may lie
	// outside the cone (AFP-V0-821).
	Deleted  []string    `json:"deleted,omitempty"`
	Scope    string      `json:"scope"`
	Selected []Selection `json:"selected"`
	Excluded []Exclusion `json:"excluded"`
	Unknown  []Unknown   `json:"unknown"`
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
	// FrontierBearers names, for each LANGUAGE_FRONTIER entry of Unknown,
	// what made it bear: the participating plugin, the reached or narrowed
	// unit with the witness that reached it, the plugin raised by a dirty
	// path no plugin owns, or the plugin left out only by the terminal rule
	// (TSP-V0-942). It restates Unknown and never changes selection or
	// scope; it is omitted when no frontier bears.
	FrontierBearers []FrontierBearer `json:"frontierBearers,omitempty"`
	// MustRun names every declared must-run unit the plan selects whatever
	// reaches it, sorted by unit then entry (AFP-V0-2484). It is omitted
	// when none is declared or nothing is dirty.
	MustRun []MustRun `json:"mustRun,omitempty"`
	// DeclaredOwners names every unit an owner pattern selects for the plan,
	// whatever reached it first, sorted by unit then pattern (AFP-V0-2489).
	// It is omitted when no owner matches a dirty path.
	DeclaredOwners []DeclaredOwner `json:"declaredOwners,omitempty"`
}

// FrontierBearer is one source that made the frontier entry Detail bear on a
// plan (TSP-V0-942). Via is one of the FrontierBearer* constants. Language
// names the plugin for a plugin-level source and Unit the unit for a
// unit-level one; Witness and DirtyPath are the reaching witness's kind and
// dirty path for a reached unit, and DirtyPath the first unowned path for an
// unowned-path source. An OMITTED entry closes a detail whose unit sources
// exceed MaxFrontierBearerUnits, with Count the number left out.
type FrontierBearer struct {
	Detail    string `json:"detail"`
	Via       string `json:"via"`
	Language  string `json:"language,omitempty"`
	Unit      string `json:"unit,omitempty"`
	Witness   string `json:"witness,omitempty"`
	DirtyPath string `json:"dirtyPath,omitempty"`
	Count     int    `json:"count,omitempty"`
}

// The FrontierBearer.Via values (TSP-V0-942).
const (
	FrontierBearerParticipating = "PARTICIPATING_PLUGIN"
	FrontierBearerReachedUnit   = "REACHED_UNIT"
	FrontierBearerNarrowedUnit  = "NARROWED_READER"
	FrontierBearerUnownedPath   = "UNOWNED_DIRTY_PATH"
	FrontierBearerTerminal      = "CROSS_LANGUAGE_ENTRY"
	FrontierBearerOmitted       = "OMITTED"
)

// MaxFrontierBearerUnits bounds the unit sources FrontierBearers lists per
// frontier entry; the rest are counted in one OMITTED entry (TSP-V0-942).
const MaxFrontierBearerUnits = 16

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
	reached, seeds, unknown, narrowed, reads, carried, followed, additive, rebuilt := graph.reach(normalized, true)
	graph.driverSites(reached, followed, normalized)
	plan.GoAdditive = additiveSummary(additive)
	through := graph.additiveExclusions(normalized, additive)
	plan.NonSource, unknown = graph.taskState(unknown)
	documentation, unknown := graph.documentationPaths(unknown)
	plan.NonSource = append(plan.NonSource, documentation...)
	runtimeReaders, unknown := graph.runtimeReaderPaths(unknown)
	plan.NonSource = append(plan.NonSource, runtimeReaders...)
	nonSource, unknown := graph.nonSourcePaths(unknown)
	owned, unknown := graph.ownedPaths(unknown)
	declaration, unknown := graph.declarationPath(unknown)
	// AFP-V0-2488: an owner-declared path stays unowned for the frontier rules,
	// so a declaration never removes a LANGUAGE_FRONTIER entry.
	frontierNonSource := append(slices.Clip(plan.NonSource), declaration...)
	// A declared non-source path, like an owned one, stays unowned for the
	// frontier rules, so it still bears the AFP-V0-511 cascade (AZU-V0-002).
	plan.NonSource = append(plan.NonSource, nonSource...)
	plan.NonSource = append(plan.NonSource, owned...)
	plan.NonSource = append(plan.NonSource, declaration...)
	frontier, excluded, bearers, involved := graph.frontierUnknowns(reached, narrowed, reads, carried, followed, normalized, frontierNonSource)
	plan.Unknown = append(frontier, unknown...)
	plan.FrontierExcluded = excluded
	plan.FrontierBearers = bearers
	plan.Unknown = append(plan.Unknown, graph.tokenBounds(reached, normalized)...)
	plan.Unknown = append(plan.Unknown, gatedLocatorsChanged(normalized)...)
	if _, closed := graph.inboundRevisionPaths(normalized); !closed {
		plan.Unknown = append(plan.Unknown, foreignCallersChanged(normalized)...)
	}
	plan.Unknown = append(plan.Unknown, shellReadScopesChanged(normalized)...)
	plan.MustRun = graph.mustRunOf(reached, normalized)
	plan.DeclaredOwners = graph.declaredOwnersOf(normalized)
	plan.Unknown = append(plan.Unknown, shellTestInputsChanged(normalized)...)
	for _, id := range graph.order {
		unit := graph.units[id]
		if _, changed := seeds[id]; changed && graph.untested(id) {
			plan.Unknown = append(plan.Unknown, Unknown{Reason: UnknownNoSelectableTest, Detail: id})
		}
		if strings.HasPrefix(id, DriverNamespace+":") {
			// A reached make target or workflow job is selected whatever
			// reached it; a driver unit is never excluded (AZU-V0-001).
			if witness, hit := reached[id]; hit {
				if selection, ok := graph.driverSelection(id, witness); ok {
					plan.Selected = append(plan.Selected, selection)
				}
			}
			continue
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
	plan.Unknown = append(plan.Unknown, graph.unselectedConsumers(reached, rebuilt, plan.Selected)...)
	graph.symlinkHints(plan.Unknown)
	instructionHints(plan.Unknown)
	driverInputHints(plan.Unknown)
	plan.FrontierSites = graph.planFrontierSites(involved, plan.Unknown)
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
//
// rebuilt holds the units whose build the change reaches: those the
// dependency traversal walks to and those an enclosed path reaches, before a
// later step replaces a unit's witness with its own test change
// (AFP-V0-762).
func (graph *Graph) reach(normalized []string, ownTests bool) (reached, seeds map[string]Witness, unknown []Unknown, narrowed, reads, carried, followed map[string]bool, additive []GoAdditive, rebuilt map[string]bool) {
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
	// A package's tests do not affect ordinary importers, but drivers run
	// those tests (AZU-V0-001). Start at their direct driver callers and follow from there.
	for _, id := range sortedKeys(contained) {
		for _, caller := range graph.dependents[id] {
			if strings.HasPrefix(caller, DriverNamespace+":") {
				addWitness(start, caller, Witness{Kind: WitnessDependency, DirtyPath: seeds[id].DirtyPath, link: seeds[id].extended(caller)})
