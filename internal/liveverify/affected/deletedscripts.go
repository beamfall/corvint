// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// WitnessDeletedScriptObserver names a unit selected because it may have
// observed a deleted shell script that the plan admits (deletedShellScript,
// AFP-V0-9992): its own files list or walk a directory that held the script,
// run a script by a computed name the script's name matches, read below such
// a directory by a computed name, or list, walk or run something the plugin
// cannot bound (Unit.WalkOpaque, AFP-V0-9990). Like a literal reader it is a
// terminal selection; a unit with no test of its own is carried to the units
// that run it (deletedScriptObserversOf, AFP-V0-9994).
const WitnessDeletedScriptObserver = "DELETED_SCRIPT_OBSERVER"

// deletedShellScript reports a dirty path that only the shell plugin claims,
// that has a file suffix, that is absent from the checkout the graph was
// observed from, outside every directory the shell plugin does not observe,
// in a checkout that is not sparse (AFP-V0-9992): a shell script deleted or
// moved away since the change's base. A deleted script is never run again,
// so nothing it reached needs a run; what can break is a unit that read,
// listed, walked or ran it. The path then raises no UNINDEXED_SOURCE_PATH:
//
//   - a unit whose path literal or glob names it, or whose "/dir/" fragment
//     names a directory holding it, is a rule (c) reader of the unowned path
//     (AFP-V0-021, AFP-V0-9133), and a name-listing read selects for a
//     deleted path (AFP-V0-9962);
//   - an unbounded reader is selected by rule (d) on any dirty path the
//     project does not exclude (AFP-V0-772);
//   - every other unit that may have observed it (observesDeletedScript) is
//     selected by deletedScriptObserversOf, with the units that run a
//     test-less one;
//   - the path stays unowned for frontierUnknowns (admittedDeletion,
//     AFP-V0-2749), so another plugin's path-open entry still bears on it.
//
// A graph with no disk root, a sparse checkout (absence may only mean outside
// the cone), a path another plugin also claims or no plugin claims, a path
// inside a directory the shell plugin does not observe, a path present on
// disk and a path without a file suffix (the plugin also claims extensionless
// files under script directories by location alone, and the deleted bytes may
// not have been a shell script) keep the entry.
func (graph *Graph) deletedShellScript(dirty string) bool {
	if graph.root == "" || graph.sparse || !fs.ValidPath(dirty) || path.Ext(dirty) == "" {
		return false
	}
	if claimed := graph.claimantsOf(dirty); len(claimed) != 1 || claimed[0] != "shell" {
		return false
	}
	if _, owned := graph.owner[dirty]; owned {
		return false
	}
	if _, unobserved := underDirectory(dirty, graph.unobserved["shell"]); unobserved {
		return false
	}
	_, err := os.Lstat(filepath.Join(graph.root, filepath.FromSlash(dirty)))
	return errors.Is(err, fs.ErrNotExist)
}

// observesDeletedScript reports whether unit index (in graph.order) may have
// observed the deleted script dirty by a means the reader rules do not
// select it for (AFP-V0-9993). The relation is closed: a non-Go unit whose
// plugin cannot bound what it lists, walks or runs (Unit.WalkOpaque), whose
// path tokens the plugin dropped at its bound (Unit.PathTokensBounded), one
// of whose walk directories holds the path at any depth (Unit.WalkDirectories),
// one of whose computed run names matches the path's file name
// (Unit.RunNames), or one of whose path tokens names a directory component of
// the path while naming no run of the path itself (a lone `dir/` token, which
// names only a file called dir under V1-0290, left by a read below the
// directory by a computed name such as `cat "data/$f"`). A Go unit's lone
// component names a directory at any depth already (AFP-V0-2203), so no Go
// unit is added here. A unit whose declared read scope excludes the path
// reads nothing there (AFP-V0-161).
func (graph *Graph) observesDeletedScript(index int, tokens []pathToken, dirty string) bool {
	id := graph.order[index]
	if strings.HasPrefix(id, "go:") || graph.scopeExcludes(id, dirty) {
		return false
	}
	unit := graph.units[id]
	if unit.WalkOpaque != "" || unit.PathTokensBounded || walksPath(unit.WalkDirectories, dirty) {
		return true
	}
	base := path.Base(dirty)
	for _, pattern := range unit.RunNames {
		if matchesRunName(pattern, base) {
			return true
		}
	}
	parts := strings.Split(dirty, "/")
	return len(parts) > 1 && namesAny(tokens, parts[:len(parts)-1], true) && !namesAny(tokens, parts, false)
}

// deletedScriptObserversOf selects, for each dirty path deletedShellScript
// admits, every unit not already reached that observesDeletedScript names,
// witnessed by the smallest such path (AFP-V0-9994). A selected unit with no
// test of its own is checked only through the units that run or source it,
// so those are followed as a test-less reader's users are
// (testlessReaderUsers): the units the walk reaches join followed, so a
// frontier that bears on dependents bears, and are returned as carried.
func (graph *Graph) deletedScriptObserversOf(reached map[string]Witness, dirty []string, followed map[string]bool) map[string]bool {
	var deleted []string
	for _, dirtyPath := range dirty {
		if graph.deletedShellScript(dirtyPath) {
			deleted = append(deleted, dirtyPath)
		}
	}
	if len(deleted) == 0 {
		return nil
	}
	tokens := graph.pathTokens()
	start := make(map[string]Witness)
	for index, id := range graph.order {
		if _, seen := reached[id]; seen {
			continue
		}
		for _, dirtyPath := range deleted {
			if graph.observesDeletedScript(index, tokens[index], dirtyPath) {
				start[id] = Witness{Kind: WitnessDeletedScriptObserver, DirtyPath: dirtyPath, Via: []string{id}}
				break
			}
		}
	}
	testless := make(map[string]Witness)
	for id, witness := range start {
		if len(graph.units[id].Tests) == 0 {
			testless[id] = witness
		}
	}
	mergeWitnesses(reached, start)
	if len(testless) == 0 {
		return nil
	}
	users := graph.traverse(testless, nil)
	walked := make(map[string]bool, len(users))
	for id := range users {
		followed[id] = true
		walked[id] = true
	}
	graph.testUsersOf(users, nil)
	graph.testRunnersOf(users, walked)
	carried := make(map[string]bool, len(users))
	for id, witness := range users {
		carried[id] = true
		if graph.scopeExcludes(id, witness.DirtyPath) {
			delete(users, id)
		}
	}
	mergeWitnesses(reached, users)
	return carried
}

// walksPath reports whether one of the walk directories holds dirtyPath at
// any depth: its components are a run of the path's directory components.
func walksPath(directories []string, dirtyPath string) bool {
	if len(directories) == 0 {
		return false
	}
	parts := strings.Split(dirtyPath, "/")
	parts = parts[:len(parts)-1]
	for _, directory := range directories {
		components := strings.Split(directory, "/")
		for offset := 0; offset+len(components) <= len(parts); offset++ {
			if equalComponents(parts[offset:offset+len(components)], components) {
				return true
			}
		}
	}
	return false
}

func equalComponents(parts, components []string) bool {
	for index, component := range components {
		if parts[index] != component {
			return false
		}
	}
	return true
}

// matchesRunName reports whether a RunNames pattern, "*" standing for any
// text, matches the file name base.
func matchesRunName(pattern, base string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == base
	}
	if !strings.HasPrefix(base, parts[0]) {
		return false
	}
	rest := base[len(parts[0]):]
	for _, middle := range parts[1 : len(parts)-1] {
		at := strings.Index(rest, middle)
		if at < 0 {
			return false
		}
		rest = rest[at+len(middle):]
	}
	return strings.HasSuffix(rest, parts[len(parts)-1])
}
