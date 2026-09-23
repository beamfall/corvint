package affected

import (
	json "encoding/json/v2"
	"sort"
	"strings"
)

// Witness kinds. Every selected unit names exactly one machine-checkable
// witness, per LPCV-V0-013.
const (
	WitnessDirectSource = "DIRECT_SOURCE_CHANGE"
	WitnessDirectTest   = "DIRECT_TEST_CHANGE"
	WitnessDependency   = "DEPENDENCY_PATH"
)

// Exclusion reasons. Every eligible unit that is not selected carries one,
// per LPCV-V0-014.
const (
	ExcludedNoDependencyPath                 = "NO_DEPENDENCY_PATH_TO_DIRTY_UNIT"
	ExcludedDirtyGoPathMayBeDeletedOrRenamed = "UNINDEXED_DIRTY_GO_PATH_MAY_BE_DELETED_OR_RENAMED"
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
}

// Select computes the plan for one dirty path set.
//
// The traversal is a breadth-first sweep over reverse dependency edges from
// every unit that directly contains a dirty path. Breadth-first order plus
// sorted expansion makes the witness for each reached unit the shortest chain,
// broken by the lexicographically smallest dirty path, so the plan is
// deterministic for fixed inputs (LPCV-V0-019).
//
// A unit is traversed whether or not it declares tests; it is selected only if
// it declares at least one, because a unit with no tests contributes no check.
// A changed unit that no selectable test checks is named as unknown scope
// instead of being omitted (AFP-V0-020); untestedRules holds each plugin's rule.
//
// Selected units are emitted in the AFP-V0-007 order: witness chain length
// ascending, shared directory prefix with the witness's dirty path descending,
// unit id ascending. Exclusions stay in unit id order.
func Select(graph *Graph, dirty []string) Plan {
	normalized := NormalizePaths(dirty)
	plan := Plan{
		GraphDigest: graph.digest,
		Dirty:       normalized,
		Scope:       ScopeBounded,
		Selected:    []Selection{},
		Excluded:    []Exclusion{},
		Unknown:     []Unknown{},
	}
	for _, reason := range graph.frontier {
		plan.Unknown = append(plan.Unknown, Unknown{Reason: UnknownLanguageFrontier, Detail: reason})
	}
	seeds, unknown := graph.seed(normalized)
	plan.Unknown = append(plan.Unknown, unknown...)
	reached := graph.traverse(seeds)
	for _, id := range graph.order {
		unit := graph.units[id]
		if _, changed := seeds[id]; changed && graph.untested(id, seeds[id]) {
			plan.Unknown = append(plan.Unknown, Unknown{Reason: UnknownNoSelectableTest, Detail: id})
		}
		if len(unit.Tests) == 0 {
			continue
		}
		witness, hit := reached[id]
		if !hit {
			plan.Excluded = append(plan.Excluded, Exclusion{
				UnitID:       id,
				Reason:       graph.exclusionReason(id, normalized),
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
	if len(plan.Unknown) != 0 {
		plan.Scope = ScopeUnknown
	}
	return plan
}

// exclusionReason names a possible removed-path condition for a Go package
// instead of claiming its own now-unindexed source has no dependency path.
func (graph *Graph) exclusionReason(id string, dirty []string) string {
	goLanguage, goGraph := graph.claimants["go"]
	if !goGraph || !strings.HasPrefix(id, "go:") {
		return ExcludedNoDependencyPath
	}
	unit := graph.units[id]
	for _, dirtyPath := range dirty {
		if _, indexed := graph.owner[dirtyPath]; indexed {
			continue
		}
		if !goLanguage.Owns(dirtyPath) {
			continue
		}
		if unitOwnsDirectory(unit, relativeDirectory(dirtyPath)) {
			return ExcludedDirtyGoPathMayBeDeletedOrRenamed
		}
	}
	return ExcludedNoDependencyPath
}

func unitOwnsDirectory(unit Unit, directory string) bool {
	for _, relative := range unit.Sources {
		if relativeDirectory(relative) == directory {
			return true
		}
	}
	for _, relative := range unit.Tests {
		if relativeDirectory(relative) == directory {
			return true
		}
	}
	return false
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
		if len(left.Witness.Via) != len(right.Witness.Via) {
			return len(left.Witness.Via) < len(right.Witness.Via)
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
// paths that could not be mapped.
func (graph *Graph) seed(dirty []string) (map[string]Witness, []Unknown) {
	seeds := make(map[string]Witness)
	unknown := make([]Unknown, 0)
	for _, path := range dirty {
		id, owned := graph.owner[path]
		if !owned {
			unknown = append(unknown, Unknown{Reason: graph.unownedReason(path), Detail: path})
			continue
		}
		if _, already := seeds[id]; already {
			continue
		}
		seeds[id] = Witness{Kind: graph.witnessKind(id, path), DirtyPath: path, Via: []string{id}}
	}
	return seeds, unknown
}

// unownedReason distinguishes a dirty path some plugin claims as its own source
// text — an unindexed source file — from a path no plugin claims at all.
func (graph *Graph) unownedReason(path string) string {
	for _, name := range graph.languages {
		if graph.claimants[name] != nil && graph.claimants[name].Owns(path) {
			return UnknownUnindexedSourcePath
		}
	}
	return UnknownUnownedDirtyPath
}

func (graph *Graph) witnessKind(id, path string) string {
	for _, test := range graph.units[id].Tests {
		if test == path {
			return WitnessDirectTest
		}
	}
	return WitnessDirectSource
}

// traverse walks reverse dependency edges breadth-first from the seeds.
// untestedRules names, per plugin, when a changed unit has no selectable test
// (AFP-V0-020). Go tests are package-scoped, so a package is untested when it
// declares none of its own. Every other plugin keeps tests in units that
// depend on sources, so a unit is untested when no unit it reaches, itself
// included, declares a test.
var untestedRules = map[string]func(*Graph, string, Witness) bool{
	"go": func(graph *Graph, id string, _ Witness) bool { return len(graph.units[id].Tests) == 0 },
}

func (graph *Graph) untested(id string, seed Witness) bool {
	rule, own := untestedRules[strings.SplitN(id, ":", 2)[0]]
	if own {
		return rule(graph, id, seed)
	}
	return !graph.reachesTest(id, seed)
}

func (graph *Graph) reachesTest(id string, seed Witness) bool {
	for reached := range graph.traverse(map[string]Witness{id: seed}) {
		if len(graph.units[reached].Tests) != 0 {
			return true
		}
	}
	return false
}

func (graph *Graph) traverse(seeds map[string]Witness) map[string]Witness {
	reached := make(map[string]Witness, len(seeds))
	frontier := make([]string, 0, len(seeds))
	for id := range seeds {
		reached[id] = seeds[id]
		frontier = append(frontier, id)
	}
	sort.Strings(frontier)
	for len(frontier) != 0 {
		next := make([]string, 0, len(frontier))
		for _, id := range frontier {
			for _, dependent := range graph.dependents[id] {
				if _, seen := reached[dependent]; seen {
					continue
				}
				parent := reached[id]
				reached[dependent] = Witness{
					Kind:      WitnessDependency,
					DirtyPath: parent.DirtyPath,
					Via:       append(append([]string(nil), parent.Via...), dependent),
				}
				next = append(next, dependent)
			}
		}
		sort.Strings(next)
		frontier = next
	}
	return reached
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
