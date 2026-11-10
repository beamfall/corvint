// SPDX-License-Identifier: Apache-2.0

// Package affectedcheck restates the selection witness rules independently of
// the affected package, so tests in the hub and in its whole-repository
// sibling package check selections against one copy.
package affectedcheck

import (
	"path"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// globMatchesPath reports whether glob matches the components of dirtyPath or
// of one of its ancestors, component by component; a leading "/" anchors the
// glob at the repository root. It restates the AFP-V0-401 match independently
// of the package's own matcher.
func globMatchesPath(glob, dirtyPath string) bool {
	parts := strings.Split(dirtyPath, "/")
	components := strings.Split(strings.TrimPrefix(glob, "/"), "/")
	last := len(parts) - len(components)
	if strings.HasPrefix(glob, "/") {
		last = min(last, 0)
	}
	for offset := 0; offset <= last; offset++ {
		matched := true
		for index, component := range components {
			if ok, err := path.Match(component, parts[offset+index]); err != nil || !ok {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// WitnessResolves checks one selection's witness against the rule that can
// produce its kind. A reader witness resolves to a unit whose path literals
// name its dirty path (AFP-V0-021); a declared-scope witness to a declared unit
// whose scope holds it (AFP-V0-023). Every other witness is a chain ending at
// the selected unit whose first unit the dirty path reaches by a rule: it owns
// the path, or a Go structural rule of AFP-V0-012 selects it (V1-0564,
// V1-0739), and each consecutive pair of which is a traversed edge (V1-1491).
// AFP-V0-495 also traverses a testless literal reader of an unowned path;
// its origin must name that exact dirty path and have no tests of its own.
// An ENCLOSING_PACKAGE witness is selected without traversal, so its chain is
// the enclosing package alone.
func WitnessResolves(graph *affected.Graph, selection affected.Selection) bool {
	witness := selection.Witness
	switch witness.Kind {
	case affected.WitnessDeclaredTestOwner:
		return slices.Contains(graph.DeclaredTestOwners(witness.DirtyPath), selection.UnitID)
	case affected.WitnessPathGlobReader:
		// AFP-V0-401: the selected unit alone passes the named literal glob, the
		// dirty path is inside its read scope, and the glob matches the dirty path
		// or one of its ancestors.
		unit, exists := graph.Unit(selection.UnitID)
		if !exists || len(witness.Via) != 1 || witness.Via[0] != selection.UnitID || !slices.Contains(unit.PathGlobs, witness.Glob) {
			return false
		}
		files := append(slices.Clone(unit.Sources), unit.Tests...)
		if unit.ReadScoped && (len(files) == 0 || !affected.InReadScope(path.Dir(files[0]), unit.ReadScope, witness.DirtyPath)) {
			return false
		}
		return globMatchesPath(witness.Glob, witness.DirtyPath)
	case "DECLARED_CONSUMER":
		// AZU-V0-007: the new final hop is a pinned consumer declaration,
		// not an import. Verify its exact argv and the preceding build chain.
		if selection.Kind != "DECLARED_CONSUMER" || len(selection.Tests) != 1 || len(witness.Via) < 2 || selection.UnitID != "declared-consumer:"+selection.Tests[0] || witness.Via[len(witness.Via)-1] != selection.UnitID {
			return false
		}
		commandID := witness.Via[len(witness.Via)-2]
		command, found := graph.Unit(commandID)
		if !found || !slices.Equal(command.ExecConsumerRuns[selection.Tests[0]], selection.Run) || len(selection.Run) == 0 {
			return false
		}
		chain := witness.Via[:len(witness.Via)-1]
		kind := affected.WitnessDependency
		if len(chain) == 1 {
			kind = affected.WitnessDirectSource
			if !golang.New().Owns(witness.DirtyPath) {
				kind = affected.WitnessEnclosingPackage
			}
		}
		return WitnessResolves(graph, affected.Selection{UnitID: commandID, Witness: affected.Witness{Kind: kind, DirtyPath: witness.DirtyPath, Via: chain}})
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
	case affected.WitnessImportGraphReader:
		return importGraphWitnessResolves(graph, selection)
	}
	if len(witness.Via) == 0 || witness.Via[len(witness.Via)-1] != selection.UnitID || !viaEdges(graph, witness.Via) {
		return false
	}
	origin := witness.Via[0]
	if witness.Kind == affected.WitnessEnclosingPackage {
		return len(witness.Via) == 1 && enclosesDataPath(graph, origin, witness.DirtyPath, false)
	}
	owner, owned := graph.OwnerOf(witness.DirtyPath)
	if witness.Kind == affected.WitnessDependency && len(witness.Via) > 1 && !owned {
		unit, found := graph.Unit(origin)
		if found && len(unit.Tests) == 0 && slices.Contains(graph.Namers(witness.DirtyPath), origin) {
			return true
		}
	}
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

// importGraphWitnessResolves checks an IMPORT_GRAPH_READER witness
// (AFP-V0-2201): a chain of the selected unit alone names a dirty path that
// may change the module's import graph; any other chain runs over import
// edges from the unit that owns the dirty path to a unit the reader lists
// (or any unit, for a reader of the whole module), then the reader.
func importGraphWitnessResolves(graph *affected.Graph, selection affected.Selection) bool {
	witness := selection.Witness
	unit, found := graph.Unit(selection.UnitID)
	if !found || len(witness.Via) == 0 || witness.Via[len(witness.Via)-1] != selection.UnitID || !unit.ImportGraphReadsAll && len(unit.ImportGraphReads) == 0 {
		return false
	}
	if len(witness.Via) == 1 {
		_, owned := graph.OwnerOf(witness.DirtyPath)
		return affected.ModuleFile(witness.DirtyPath) || !owned
	}
	chain := witness.Via[:len(witness.Via)-1]
	listed := chain[len(chain)-1]
	if !unit.ImportGraphReadsAll && !slices.Contains(unit.ImportGraphReads, listed) || !viaEdges(graph, chain) {
		return false
	}
	owner, owned := graph.OwnerOf(witness.DirtyPath)
	return !owned || owner == chain[0]
}

// viaEdges reports that every consecutive pair of a witness chain is a graph
// edge the selector traverses (V1-1491): the later unit imports, or
// advisorily imports, the earlier one, or its code may run the earlier
// command's built binary (an exec that is not test-only, AFP-V0-2993); the
// final hop may also be a test-only import or any built-binary exec, which
// the selector takes one edge past a reached unit and never expands.
func viaEdges(graph *affected.Graph, via []string) bool {
	for index := 1; index < len(via); index++ {
		unit, found := graph.Unit(via[index])
		if !found {
			return false
		}
		edges := append(append([]string(nil), unit.Imports...), unit.AdvisoryImports...)
		for _, command := range unit.Execs {
			if !slices.Contains(unit.TestOnlyExecs, command) {
				edges = append(edges, command)
			}
		}
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
