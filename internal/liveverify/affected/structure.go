package affected

import (
	"path"
	"sort"
	"strings"
)

// goStructure mirrors the structural rules (a) and (b) of
// tools/gate-affected-select (AFP-V0-012) for dirty paths no Go unit declares.
// Rule (a): an unindexed Go path is a source change of the package in its
// directory; with no package there it reaches the importers of the absent
// package. Rule (b): any other path selects every enclosing package, and the
// nearest one when the path sits directly in its directory, and every
// embedding ancestor, also reach their dependents. It returns the changed
// units, the further units to traverse from, and the enclosing units that are
// selected without traversal. Only a graph with a Go plugin has these rules.
func (graph *Graph) goStructure(dirty []string) (changed, traversed, enclosing map[string]Witness) {
	changed, traversed, enclosing = map[string]Witness{}, map[string]Witness{}, map[string]Witness{}
	goLanguage, goGraph := graph.claimants["go"]
	if !goGraph {
		return changed, traversed, enclosing
	}
	packages := graph.goPackageDirectories()
	for _, dirtyPath := range dirty {
		if _, indexed := graph.owner[dirtyPath]; indexed && goLanguage.Owns(dirtyPath) {
			continue
		}
		if goLanguage.Owns(dirtyPath) {
			graph.goSourceRule(dirtyPath, packages, changed)
			continue
		}
		graph.goDataRule(dirtyPath, packages, traversed, enclosing)
	}
	return changed, traversed, enclosing
}

// goPackageDirectories maps each Go unit's directory to its identity.
func (graph *Graph) goPackageDirectories() map[string]string {
	packages := make(map[string]string)
	for _, id := range graph.order {
		if !strings.HasPrefix(id, "go:") {
			continue
		}
		unit := graph.units[id]
		for _, relative := range append(append([]string(nil), unit.Sources...), unit.Tests...) {
			packages[relativeDirectory(relative)] = id
		}
	}
	return packages
}

// goSourceRule is rule (a) for an unindexed Go path: a deleted or renamed-away
// file of the package in its directory, or a file of a package that is gone,
// whose importers still name it by an edge to an absent unit.
func (graph *Graph) goSourceRule(dirtyPath string, packages map[string]string, changed map[string]Witness) {
	directory := relativeDirectory(dirtyPath)
	if id, present := packages[directory]; present {
		addWitness(changed, id, Witness{Kind: WitnessDirectSource, DirtyPath: dirtyPath, Via: []string{id}})
		return
	}
	for _, id := range graph.order {
		if graph.importsAbsentPackage(graph.units[id], directory) {
			addWitness(changed, id, Witness{Kind: WitnessDependency, DirtyPath: dirtyPath, Via: []string{id}})
		}
	}
}

// importsAbsentPackage reports an edge to an absent Go unit whose import path
// ends in directory's last component. The graph does not map an absent import
// path to a directory, so this is a superset of the absent package's importers.
func (graph *Graph) importsAbsentPackage(unit Unit, directory string) bool {
	name := path.Base(directory)
	for _, target := range append(append([]string(nil), unit.Imports...), unit.TestImports...) {
		if _, known := graph.units[target]; known || !strings.HasPrefix(target, "go:") {
			continue
		}
		if directory == "." || strings.HasSuffix(target, "/"+name) || target == "go:"+name {
			return true
		}
	}
	return false
}

// goDataRule is rule (b): every package enclosing dataPath, nearest first.
func (graph *Graph) goDataRule(dataPath string, packages map[string]string, traversed, enclosing map[string]Witness) {
	parent := relativeDirectory(dataPath)
	for directory := parent; ; directory = relativeDirectory(directory) {
		id, present := packages[directory]
		if present {
			witness := Witness{Kind: WitnessEnclosingPackage, DirtyPath: dataPath, Via: []string{id}}
			addWitness(enclosing, id, witness)
			if directory == parent || graph.units[id].Embeds {
				addWitness(traversed, id, witness)
			}
			parent = ""
		}
		if directory == "." {
			return
		}
	}
}

// addWitness keeps the first witness per unit; dirty paths arrive sorted, so
// that is the smallest dirty path.
func addWitness(witnesses map[string]Witness, id string, witness Witness) {
	if _, seen := witnesses[id]; !seen {
		witnesses[id] = witness
	}
}

// mergeWitnesses adds every witness of extra that start lacks, in id order.
func mergeWitnesses(start, extra map[string]Witness) {
	ids := make([]string, 0, len(extra))
	for id := range extra {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		addWitness(start, id, extra[id])
	}
}
