// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// DeclaredSelectionPath is the repository's declaration of path-glob owners
// and must-run tests (AFP-V0-2480).
const DeclaredSelectionPath = ".corvint/test-selection.json"

// DeclaredSelectionProfile is the profile of a declaration of owners and
// must-run tests alone (AFP-V0-2480).
const DeclaredSelectionProfile = "corvint-test-selection/0"

// DeclaredTreeReaderSelectionProfile is the profile of a declaration that may
// also name whole-tree readers (AZU-V0-8921); its owners and must-run tests
// mean what they mean under /0. It is a new value, not a new member of the
// /0 file, so a planner that predates treeReaders refuses the file instead
// of planning without them, and every /0 file keeps its meaning and its
// digest (AFP-V0-2480, CMP-V1-004). The value skips /1, which is the version
// of the runner's selection expectation (TRE-V0-021), a different object.
const DeclaredTreeReaderSelectionProfile = "corvint-test-selection/2"

// WitnessDeclaredTestOwner selects a unit whose test file a declared owner
// names, because an owner pattern matches a dirty path (AFP-V0-2482). Like a
// declared read scope it is terminal: the unit's dependents are not walked.
const WitnessDeclaredTestOwner = "DECLARED_TEST_OWNER"

// WitnessMustRun selects a declared must-run unit that nothing else reaches
// (AFP-V0-2484).
const WitnessMustRun = "MUST_RUN"

// ClassDeclaredTestOwner is the NonSource class of an unowned dirty path a
// declared owner pattern matches (AFP-V0-2482).
const ClassDeclaredTestOwner = "DECLARED_TEST_OWNER"

// declaredTestOwnerReason is the stated reason of every
// ClassDeclaredTestOwner entry.
const declaredTestOwnerReason = "declared test owner (" + DeclaredSelectionPath + "); the owner's tests are selected, and every unit whose path literal, enclosing directory or declared read scope reaches the path is selected too"

const (
	maxDeclaredSelectionBytes = 64 << 10
	maxDeclaredOwners         = 256
	maxDeclaredOwnerTests     = 16
	maxDeclaredMustRun        = 256
	maxOwnerPatternBytes      = 256
	maxOwnerPatternStars      = 4
	maxMustRunReasonBytes     = 200
	maxDeclaredTreeReaders    = 64
)

// TestOwner is one declared owner: the dirty paths Paths matches select the
// units holding Tests (AFP-V0-2480, AFP-V0-2482).
type TestOwner struct {
	Paths string   `json:"paths"`
	Tests []string `json:"tests"`
}

// MustRunEntry is one declared must-run test path with its reason
// (AFP-V0-2480, AFP-V0-2484).
type MustRunEntry struct {
	Tests  string `json:"tests"`
	Reason string `json:"reason"`
}

// MustRun names one must-run unit a plan holds and the entry that pins it
// (AFP-V0-2484).
type MustRun struct {
	UnitID string `json:"unitId"`
	Tests  string `json:"tests"`
	Reason string `json:"reason"`
}

// DeclaredOwner names one unit an owner pattern selects for a plan, with the
// pattern and the first dirty path in plan order it matches (AFP-V0-2489).
type DeclaredOwner struct {
	UnitID    string `json:"unitId"`
	Paths     string `json:"paths"`
	DirtyPath string `json:"dirtyPath"`
}

// TreeReader is one declared whole-tree reader (AZU-V0-8921): the repository
// file Runs is a script, tool or program that reads every tracked path, as a
// content check over `git ls-files` does, for the project's stated Reason.
// Every driver unit that runs it, directly or through units that run it, is
// a whole-tree reader driver (AZU-V0-8922).
type TreeReader struct {
	Runs   string `json:"runs"`
	Reason string `json:"reason"`
}

// testSelection is the declaration as the graph digest and cache carry it.
// TreeReaders is omitted when empty, so a /0 declaration keeps its digest.
type testSelection struct {
	Owners      []TestOwner    `json:"owners,omitempty"`
	MustRun     []MustRunEntry `json:"mustRun,omitempty"`
	TreeReaders []TreeReader   `json:"treeReaders,omitempty"`
}

func (declared *testSelection) empty() bool {
	return declared == nil || len(declared.Owners) == 0 && len(declared.MustRun) == 0 && len(declared.TreeReaders) == 0
}

type testSelectionFile struct {
	Profile     string         `json:"profile"`
	Owners      []TestOwner    `json:"owners"`
	MustRun     []MustRunEntry `json:"mustRun"`
	TreeReaders []TreeReader   `json:"treeReaders"`
}

// readTestSelection returns the declaration: nil when the repository declares
// none, an error naming the member for one that is unreadable, oversized, not
// a regular file or invalid in any member (AFP-V0-2480, AFP-V0-2481).
func readTestSelection(root *Source) (*testSelection, error) {
	if root == nil {
		return nil, nil
	}
	body, err := root.ReadBounded(DeclaredSelectionPath, maxDeclaredSelectionBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", DeclaredSelectionPath, err)
	}
	var declared testSelectionFile
	if err := json.UnmarshalRead(bytes.NewReader(body), &declared, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("%s: %w", DeclaredSelectionPath, err)
	}
	switch {
	case declared.Profile != DeclaredSelectionProfile && declared.Profile != DeclaredTreeReaderSelectionProfile:
		return nil, fmt.Errorf("%s: profile must be %s or %s", DeclaredSelectionPath, DeclaredSelectionProfile, DeclaredTreeReaderSelectionProfile)
	case declared.TreeReaders != nil && declared.Profile != DeclaredTreeReaderSelectionProfile:
		// AZU-V0-8921: treeReaders exists only under the new profile, so a
		// /0 file stays readable, byte for byte, by every 1.x planner.
		return nil, fmt.Errorf("%s: treeReaders needs profile %s", DeclaredSelectionPath, DeclaredTreeReaderSelectionProfile)
	}
	if len(declared.Owners) > maxDeclaredOwners || len(declared.MustRun) > maxDeclaredMustRun {
		return nil, fmt.Errorf("%s: at most %d owners and %d mustRun entries", DeclaredSelectionPath, maxDeclaredOwners, maxDeclaredMustRun)
	}
	previous := ""
	for _, owner := range declared.Owners {
		if !validOwnerPattern(owner.Paths) || owner.Paths <= previous {
			return nil, fmt.Errorf("%s: owners paths %q", DeclaredSelectionPath, owner.Paths)
		}
		previous = owner.Paths
		if len(owner.Tests) == 0 || len(owner.Tests) > maxDeclaredOwnerTests {
			return nil, fmt.Errorf("%s: owners %q must name 1 to %d tests", DeclaredSelectionPath, owner.Paths, maxDeclaredOwnerTests)
		}
		test := ""
		for _, entry := range owner.Tests {
			if !validDeclaredTest(entry) || entry <= test {
				return nil, fmt.Errorf("%s: owners %q tests %q", DeclaredSelectionPath, owner.Paths, entry)
			}
			test = entry
		}
	}
	previous = ""
	for _, entry := range declared.MustRun {
		if !validDeclaredTest(entry.Tests) || entry.Tests <= previous {
			return nil, fmt.Errorf("%s: mustRun tests %q", DeclaredSelectionPath, entry.Tests)
		}
		previous = entry.Tests
		if !validTestSelectionReason(entry.Reason) {
			return nil, fmt.Errorf("%s: mustRun %q reason must be 1 to %d bytes of printable text", DeclaredSelectionPath, entry.Tests, maxMustRunReasonBytes)
		}
	}
	if len(declared.TreeReaders) > maxDeclaredTreeReaders {
		return nil, fmt.Errorf("%s: at most %d treeReaders", DeclaredSelectionPath, maxDeclaredTreeReaders)
	}
	previous = ""
	for _, entry := range declared.TreeReaders {
		if !validDeclaredTest(entry.Runs) || strings.HasSuffix(entry.Runs, "/") || entry.Runs <= previous {
			return nil, fmt.Errorf("%s: treeReaders runs %q", DeclaredSelectionPath, entry.Runs)
		}
		previous = entry.Runs
		if !validTestSelectionReason(entry.Reason) {
			return nil, fmt.Errorf("%s: treeReaders %q reason must be 1 to %d bytes of printable text", DeclaredSelectionPath, entry.Runs, maxMustRunReasonBytes)
		}
	}
	selection := &testSelection{Owners: declared.Owners, MustRun: declared.MustRun, TreeReaders: declared.TreeReaders}
	if selection.empty() {
		return nil, nil
	}
	return selection, nil
}

// validDeclaredTest is the AFP-V0-160 entry grammar outside `.corvint/`.
func validDeclaredTest(entry string) bool {
	trimmed := strings.TrimSuffix(entry, "/")
	return ValidReadScopeEntry(entry) && trimmed != ".corvint" && !strings.HasPrefix(trimmed, ".corvint/")
}

func validTestSelectionReason(reason string) bool {
	if reason == "" || len(reason) > maxMustRunReasonBytes || !utf8.ValidString(reason) {
		return false
	}
	return !strings.ContainsFunc(reason, func(r rune) bool { return !unicode.IsPrint(r) && r != ' ' })
}

// validOwnerPattern is the AFP-V0-2481 grammar: `/`-separated segments, none
// empty, `.` or `..`; `**` only as a whole segment, at most four times; `*`
// and `?` within a segment; no class, alternation or escape.
func validOwnerPattern(pattern string) bool {
	if pattern == "" || len(pattern) > maxOwnerPatternBytes || !utf8.ValidString(pattern) || strings.ContainsAny(pattern, "[]{}\\") {
		return false
	}
	stars := 0
	for _, segment := range strings.Split(pattern, "/") {
		switch {
		case segment == "" || segment == "." || segment == "..":
			return false
		case segment == "**":
			stars++
		case strings.Contains(segment, "**"):
			return false
		}
		if strings.ContainsFunc(segment, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return false
		}
	}
	return stars <= maxOwnerPatternStars
}

// OwnerPatternMatches reports whether an owner pattern matches the whole
// repository-relative path (AFP-V0-2481); repository checks share the rule.
func OwnerPatternMatches(pattern, relative string) bool {
	return ownerPatternMatches(pattern, relative)
}

// ownerPatternMatches reports whether pattern matches the whole
// repository-relative path (AFP-V0-2481). No pattern matches `.git` or
// `.corvint` or a path under either.
func ownerPatternMatches(pattern, relative string) bool {
	if relative == ".git" || relative == ".corvint" || strings.HasPrefix(relative, ".git/") || strings.HasPrefix(relative, ".corvint/") {
		return false
	}
	// Most ownership declarations are exact paths or have a literal
	// directory prefix. Reject those misses before allocating the matcher
	// state (AZU-V0-8900), preserving **'s zero-component match.
	first := strings.IndexAny(pattern, "*?[\\")
	if first < 0 {
		return pattern == relative
	}
	if prefix := pattern[:first]; prefix != "" && !strings.HasPrefix(relative, prefix) {
		if !strings.HasSuffix(prefix, "/") || relative != strings.TrimSuffix(prefix, "/") {
			return false
		}
	}
	patterns, parts := strings.Split(pattern, "/"), strings.Split(relative, "/")
	// matched[j] holds whether the patterns consumed so far match parts[:j].
	matched := make([]bool, len(parts)+1)
	matched[0] = true
	for _, segment := range patterns {
		next := make([]bool, len(parts)+1)
		for j := range matched {
			if !matched[j] {
				continue
			}
			if segment == "**" {
				for k := j; k <= len(parts); k++ {
					next[k] = true
				}
				break
			}
			if j < len(parts) {
				if ok, err := path.Match(segment, parts[j]); err == nil && ok {
					next[j+1] = true
				}
			}
		}
		matched = next
	}
	return matched[len(parts)]
}

// declaredTestUnits lists, in graph order, the units holding a test file equal
// to or under entry.
func (graph *Graph) declaredTestUnits(entry string) []string {
	var units []string
	for _, id := range graph.order {
		if slices.ContainsFunc(graph.units[id].Tests, func(test string) bool { return runtimeEdgeHolds(entry, test) }) {
			units = append(units, id)
		}
	}
	return units
}

// resolveTestSelection maps each owner pattern and must-run entry to its
// units. A declared test that holds no test file contradicts the observed
// graph: a full build refuses it, naming the entry; a partial build skips it
// (AFP-V0-2483).
func (graph *Graph) resolveTestSelection(partial bool) error {
	declared := graph.testSelection
	if declared.empty() {
		return nil
	}
	graph.ownerUnits = make(map[string][]string, len(declared.Owners))
	for _, owner := range declared.Owners {
		var units []string
		for _, test := range owner.Tests {
			holders := graph.declaredTestUnits(test)
			if len(holders) == 0 && !partial {
				return fmt.Errorf("%s: owners %q tests %q holds no test file the graph observes", DeclaredSelectionPath, owner.Paths, test)
			}
			units = append(units, holders...)
		}
		// AFP-V0-10329: a pattern that matches a test file some unit holds
		// covers an exercised path. The owner selects that unit whether or
		// not it names it: a row that names only some other test in their
		// place cannot narrow the exercising tests away, and a historical
		// declaration still plans.
		units = append(units, graph.exercisingUnits(owner.Paths)...)
		slices.Sort(units)
		graph.ownerUnits[owner.Paths] = slices.Compact(units)
	}
	graph.mustRun = nil
	for _, entry := range declared.MustRun {
		holders := graph.declaredTestUnits(entry.Tests)
		if len(holders) == 0 && !partial {
			return fmt.Errorf("%s: mustRun tests %q holds no test file the graph observes", DeclaredSelectionPath, entry.Tests)
		}
		for _, id := range holders {
			graph.mustRun = append(graph.mustRun, MustRun{UnitID: id, Tests: entry.Tests, Reason: entry.Reason})
		}
	}
	slices.SortFunc(graph.mustRun, func(left, right MustRun) int {
		if left.UnitID != right.UnitID {
			return strings.Compare(left.UnitID, right.UnitID)
		}
		return strings.Compare(left.Tests, right.Tests)
	})
	return graph.resolveTreeReaders(partial)
}

// resolveTreeReaders marks every driver unit that runs a declared whole-tree
// reader as a whole-tree reader driver: it gains the driver tree-read glob,
// so it is a PATH_GLOB_READER of every dirty path and its driver dependents
// are followed, as for a recipe that enumerates the tree itself
// (AZU-V0-8922, AFP-V0-9401, AFP-V0-9402). A driver runs the file when the
// file's owning unit reaches it through dependents. A declared file no unit
// owns, or one no driver runs, contradicts the observed graph: a full build
// refuses it, naming the entry; a partial build skips it (AZU-V0-8921).
func (graph *Graph) resolveTreeReaders(partial bool) error {
	graph.treeReaderDrivers = nil
	for _, entry := range graph.testSelection.TreeReaders {
		owner, owned := graph.owner[entry.Runs]
		if !owned {
			if partial {
				continue
			}
			return fmt.Errorf("%s: treeReaders runs %q is no file a unit of the graph owns", DeclaredSelectionPath, entry.Runs)
		}
		drivers := graph.driversRunning(owner)
		if len(drivers) == 0 && !partial {
			return fmt.Errorf("%s: treeReaders runs %q is run by no driver unit the graph observes", DeclaredSelectionPath, entry.Runs)
		}
		for _, id := range drivers {
			unit := graph.units[id]
			if slices.Contains(unit.PathGlobs, driverTreeReadGlob) {
				// A recipe that enumerates the tree itself, or a driver an
				// earlier entry marked, already reads every path.
				continue
			}
			// Clone: the plugin result the graph cache stores keeps its own
			// globs, and a restore applies the declaration again.
			unit.PathGlobs = append(slices.Clone(unit.PathGlobs), driverTreeReadGlob)
			slices.Sort(unit.PathGlobs)
			graph.units[id] = unit
			if graph.treeReaderDrivers == nil {
				graph.treeReaderDrivers = map[string]string{}
			}
			graph.treeReaderDrivers[id] = entry.Runs
		}
	}
	return nil
}

// declaredTreeReaderSkips reports a dirty path a driver the treeReaders
// declaration made a whole-tree reader does not read for selection: one the
// project declares no unbounded reader reads (AFP-V0-772,
// .corvint/unbounded-reader-exclusions.json), as it does for every other
// unbounded reader. A recipe that enumerates the tree itself keeps its glob
// whole (AZU-V0-8922).
func (graph *Graph) declaredTreeReaderSkips(id, dirtyPath string) bool {
	if _, declared := graph.treeReaderDrivers[id]; !declared {
		return false
	}
	index := graph.runtimeReaderEntry(dirtyPath)
	return index >= 0 && graph.runtimeReaders[index].NoUnboundedReaders
}

// driversRunning lists, sorted, the driver units that run owner: the make
// targets and workflow jobs that reach it through dependents, directly or
// through a testless script, program or driver that runs it (AZU-V0-8922).
// An edge that exists only because a unit imports every owner under a
// package wildcard (`go vet ./...` resolves to "...") runs no script, so
// it is not followed unless the unit also names the file exactly
// (runsExplicitly). A unit with tests is a leaf: what it runs, it runs on
// the test's inputs.
func (graph *Graph) driversRunning(owner string) []string {
	seen := map[string]bool{owner: true}
	queue := []string{owner}
	var drivers []string
	for len(queue) != 0 {
		id := queue[0]
		queue = queue[1:]
		for _, dependent := range graph.dependents[id] {
			if seen[dependent] || !graph.runsExplicitly(dependent, id) {
				continue
			}
			seen[dependent] = true
			if strings.HasPrefix(dependent, DriverNamespace+":") {
				drivers = append(drivers, dependent)
			} else if len(graph.units[dependent].Tests) != 0 {
				continue
			}
			queue = append(queue, dependent)
		}
	}
	slices.Sort(drivers)
	return drivers
}

// runsExplicitly reports whether dependent's edge to id is more than a
// package wildcard: id is a driver (a prerequisite, a make call or a job's
// step), dependent has no wildcard path import, or a non-wildcard path import
// of dependent resolves to id as resolvePathImports resolves it.
func (graph *Graph) runsExplicitly(dependent, id string) bool {
	if strings.HasPrefix(id, DriverNamespace+":") {
		return true
	}
	unit := graph.units[dependent]
	wildcard := false
	for _, entry := range slices.Concat(unit.PathImports, unit.BuildPathImports) {
		if entry == "..." || strings.HasSuffix(entry, "/...") {
			wildcard = true
			continue
		}
		if graph.owner[entry] == id {
			return true
		}
		for _, file := range slices.Concat(graph.units[id].Sources, graph.units[id].Tests) {
			if rest, below := strings.CutPrefix(file, entry+"/"); below && !strings.Contains(rest, "/") {
				return true
			}
		}
	}
	return !wildcard
}

// DriversRunning lists, sorted, the driver units that run the repository
// file path by the relation a declared whole-tree reader uses (AZU-V0-8922);
// nil when no unit owns path.
func (graph *Graph) DriversRunning(path string) []string {
	owner, owned := graph.owner[path]
	if !owned {
		return nil
	}
	return graph.driversRunning(owner)
}

// TreeReaderDrivers maps each driver the declaration made a whole-tree
// reader to the first declared file it runs (AZU-V0-8922); a driver whose
// recipe already enumerates the tree is not listed. It is nil when there is
// none.
func (graph *Graph) TreeReaderDrivers() map[string]string {
	if len(graph.treeReaderDrivers) == 0 {
		return nil
	}
	out := make(map[string]string, len(graph.treeReaderDrivers))
	for id, runs := range graph.treeReaderDrivers {
		out[id] = runs
	}
	return out
}

// exercisingUnits lists, in graph order, the units holding a test file that
// pattern matches outside a fixture tree (AFP-V0-10329): a unit whose test
// files the pattern matches exercises the owned paths, so an owner selects
// it beside the tests it names.
func (graph *Graph) exercisingUnits(pattern string) []string {
	var units []string
	for _, id := range graph.order {
		if slices.ContainsFunc(graph.units[id].Tests, func(test string) bool { return ownerPatternMatches(pattern, test) && !fixtureTest(test) }) {
			units = append(units, id)
		}
	}
	return units
}

// fixtureSegments are the directory names under which a test file is fixture
// data of the test that reads the tree, not a test of the repository: a
// plugin lists such a file as a unit's test, but no runner of the repository
// exercises the owned paths through it (AFP-V0-10329; the AFP-V0-2440
// `testdata` convention, with the `fixture` and `fixtures` trees this
// repository's non-Go fixtures use).
var fixtureSegments = map[string]bool{"testdata": true, "fixture": true, "fixtures": true}

// fixtureTest reports a test file with a fixture segment in its directory.
func fixtureTest(test string) bool {
	directory, _ := path.Split(test)
	for _, segment := range strings.Split(strings.TrimSuffix(directory, "/"), "/") {
		if fixtureSegments[segment] {
			return true
		}
	}
	return false
}

// ownersOf selects, for each dirty path an owner pattern matches, every unit
// of that owner not already reached, witnessed by the first such dirty path
// (AFP-V0-2482). It is terminal: nothing is traversed from the unit.
func (graph *Graph) ownersOf(reached map[string]Witness, dirty []string) {
	if graph.testSelection.empty() {
		return
	}
	for _, dirtyPath := range dirty {
		for _, owner := range graph.testSelection.Owners {
			if !ownerPatternMatches(owner.Paths, dirtyPath) {
				continue
			}
			for _, id := range graph.ownerUnits[owner.Paths] {
				if _, seen := reached[id]; !seen {
					reached[id] = Witness{Kind: WitnessDeclaredTestOwner, DirtyPath: dirtyPath, Via: []string{id}, Glob: owner.Paths}
				}
			}
		}
	}
}

// ownedPaths removes, from the unknown entries, each UNOWNED_DIRTY_PATH whose
// path an owner pattern with at least one resolved unit matches, and returns
// those paths, in dirty-path order, as one ClassDeclaredTestOwner group
// (AFP-V0-2482). No other unknown is ever removed.
func (graph *Graph) ownedPaths(unknown []Unknown) ([]NonSource, []Unknown) {
	if graph.testSelection.empty() {
		return nil, unknown
	}
	var paths []string
	kept := make([]Unknown, 0, len(unknown))
	for _, entry := range unknown {
		if entry.Reason == UnknownUnownedDirtyPath && graph.owned(entry.Detail) {
			paths = append(paths, entry.Detail)
			continue
		}
		kept = append(kept, entry)
	}
	if len(paths) == 0 {
		return nil, unknown
	}
	return []NonSource{{Class: ClassDeclaredTestOwner, Reason: declaredTestOwnerReason, Paths: paths}}, kept
}

func (graph *Graph) owned(relative string) bool {
	for _, owner := range graph.testSelection.Owners {
		if len(graph.ownerUnits[owner.Paths]) != 0 && ownerPatternMatches(owner.Paths, relative) {
			return true
		}
	}
	return false
}

// mustRunOf selects every declared must-run unit when the plan has a dirty
// path, giving a unit nothing else reached the MUST_RUN witness, and returns
// the plan's mustRun member (AFP-V0-2484).
func (graph *Graph) mustRunOf(reached map[string]Witness, dirty []string) []MustRun {
	if len(dirty) == 0 || len(graph.mustRun) == 0 {
		return nil
	}
	for _, entry := range graph.mustRun {
		if _, seen := reached[entry.UnitID]; !seen {
			reached[entry.UnitID] = Witness{Kind: WitnessMustRun, DirtyPath: dirty[0], Via: []string{entry.UnitID}}
		}
	}
	return slices.Clone(graph.mustRun)
}

// declaredOwnersOf returns every unit an owner pattern selects for dirty,
// whatever else reaches it, once per unit and pattern with the first matching
// dirty path, sorted by unit and then pattern; nil when there is none
// (AFP-V0-2489).
func (graph *Graph) declaredOwnersOf(dirty []string) []DeclaredOwner {
	if graph.testSelection.empty() {
		return nil
	}
	var owners []DeclaredOwner
	for _, owner := range graph.testSelection.Owners {
		for _, dirtyPath := range dirty {
			if !ownerPatternMatches(owner.Paths, dirtyPath) {
				continue
			}
			for _, id := range graph.ownerUnits[owner.Paths] {
				owners = append(owners, DeclaredOwner{UnitID: id, Paths: owner.Paths, DirtyPath: dirtyPath})
			}
			break
		}
	}
	slices.SortFunc(owners, func(left, right DeclaredOwner) int {
		if left.UnitID != right.UnitID {
			return strings.Compare(left.UnitID, right.UnitID)
		}
		return strings.Compare(left.Paths, right.Paths)
	})
	return owners
}

// MustRunUnits returns every declared must-run unit with its entry, sorted by
// unit id and then entry (AFP-V0-2484), or nil when none is declared.
func (graph *Graph) MustRunUnits() []MustRun {
	return slices.Clone(graph.mustRun)
}

// DeclaredTestOwners returns, sorted, the units the declared owners select for
// dirtyPath (AFP-V0-2482), or nil when the project declares none.
func (graph *Graph) DeclaredTestOwners(dirtyPath string) []string {
	if graph.testSelection.empty() {
		return nil
	}
	var units []string
	for _, owner := range graph.testSelection.Owners {
		if ownerPatternMatches(owner.Paths, dirtyPath) {
			units = append(units, graph.ownerUnits[owner.Paths]...)
		}
	}
	slices.Sort(units)
	return slices.Compact(units)
}
