package affected

import (
	"path"
	"slices"
	"strings"
)

// WitnessImportGraphReader names a unit whose code or tests list, with `go
// list`, the import graph of a unit whose import set the change alters, or of
// the whole module when the unit lists packages it does not name
// (AFP-V0-2201). Like a binary-exec runner it is selected one edge past what
// it reads, and the traversal does not continue from it.
const WitnessImportGraphReader = "IMPORT_GRAPH_READER"

// ImportSetChanges is the golang plugin's comparison of each changed Go
// package's import set at the change's base with the tree (AFP-V0-2202).
// Changed maps each unit whose package header changed (a file added, removed
// or renamed, or a file's build constraint, package clause or import list
// changed) to the first dirty path that changed it. Module is the first dirty
// path that may change any package's import graph: a go.mod, go.sum, go.work
// or go.work.sum, a vendor/modules.txt, or a changed directory that no
// longer holds an observed package. A nil value is unknown: the selector then
// counts every dirty Go file of a Go unit as changing its package's import
// set and every unowned one as changing the module's.
type ImportSetChanges struct {
	Changed map[string]string
	Module  string
}

// WithImportSetChanges returns a copy of graph whose selection decides its
// import-graph readers by changes (AFP-V0-2202). Like the read-scope base it
// is a selection input, not part of the observed graph or its digest.
func (graph *Graph) WithImportSetChanges(changes *ImportSetChanges) *Graph {
	copied := *graph
	if changes != nil {
		cloned := &ImportSetChanges{Changed: make(map[string]string, len(changes.Changed)), Module: changes.Module}
		for id, dirty := range changes.Changed {
			cloned.Changed[id] = dirty
		}
		changes = cloned
	}
	copied.importSets = changes
	return &copied
}

// HasImportGraphReaders reports whether any unit of the graph lists an import
// graph (AFP-V0-2200), so a caller can skip the comparison when none does.
func (graph *Graph) HasImportGraphReaders() bool {
	for _, id := range graph.order {
		if unit := graph.units[id]; unit.ImportGraphReadsAll || len(unit.ImportGraphReads) != 0 {
			return true
		}
	}
	return false
}

// ModuleFile reports a dirty path whose change may alter the import graph of
// any Go package of its module (AFP-V0-2202).
func ModuleFile(relative string) bool {
	switch path.Base(relative) {
	case "go.mod", "go.sum", "go.work", "go.work.sum":
		return true
	case "modules.txt":
		return path.Base(path.Dir(relative)) == "vendor"
	}
	return false
}

// GoPackageFile reports a dirty `.go` path that can be a file of a Go package
// of the graph (AFP-V0-2202). A path under a testdata directory is fixture
// data the go tool never builds. A path below a directory whose name starts
// with "_" or "." is skipped by `./...` but still built when imported, and
// the Go plugin indexes it (AZU-V0-005), so it is a package file too unless
// no unit owns it and the repository declares it non-source output
// (AZU-V0-9921), or it lies below version-control metadata the source walk
// never descends.
func (graph *Graph) GoPackageFile(relative string) bool {
	if !strings.HasSuffix(relative, ".go") {
		return false
	}
	hidden := false
	for _, component := range strings.Split(path.Dir(relative), "/") {
		switch {
		case component == "testdata", component == ".git", component == ".hg", component == ".svn":
			return false
		case strings.HasPrefix(component, "_"), strings.HasPrefix(component, ".") && component != ".":
			hidden = true
		}
	}
	return !hidden || !graph.declaredUnindexedOutput(relative)
}

// unknownImportSets is the ImportSetChanges the selector assumes without a
// comparison: every dirty .go file a Go unit owns changes its unit's import
// set, every other dirty file below an embedding Go unit changes that unit's,
// and a module file or an unowned .go file the go tool may build, an indexed
// hidden package's included (GoPackageFile), changes the module's.
func (graph *Graph) unknownImportSets(normalized []string) *ImportSetChanges {
	changes := &ImportSetChanges{Changed: map[string]string{}}
	for _, dirty := range normalized {
		switch {
		case ModuleFile(dirty):
			if changes.Module == "" {
				changes.Module = dirty
			}
		case graph.GoPackageFile(dirty):
			owner, owned := graph.owner[dirty]
			switch {
			case owned && strings.HasPrefix(owner, "go:"):
				if _, seen := changes.Changed[owner]; !seen {
					changes.Changed[owner] = dirty
				}
			case !owned && changes.Module == "":
				changes.Module = dirty
			}
		case !strings.HasSuffix(dirty, ".go"):
			// A data file below an embedding package may be added or
			// removed, which `go list` sees as a failing embed pattern.
			for _, id := range graph.order {
				unit := graph.units[id]
				if !unit.Embeds || !strings.HasPrefix(id, "go:") {
					continue
				}
				files := append(append([]string(nil), unit.Sources...), unit.Tests...)
				if len(files) == 0 {
					continue
				}
				if directory := relativeDirectory(files[0]); directory == "." || strings.HasPrefix(dirty, directory+"/") {
					if _, seen := changes.Changed[id]; !seen {
						changes.Changed[id] = dirty
					}
				}
			}
		}
	}
	return changes
}

// importGraphReadersOf selects each unit not already reached whose import
// graph listing the change can alter (AFP-V0-2201): a reader of the whole
// module when any import set changed, and a reader of named units when one
// of them depends, through import edges, on a unit whose import set changed,
// or on any unit when the module's import graph changed. The witness chain
// runs from the changed unit through its importers to the listed unit, then
// the reader. A body-only edit changes no import set and selects no reader
// through this edge.
func (graph *Graph) importGraphReadersOf(reached map[string]Witness, normalized []string) {
	var readers []string
	for _, id := range graph.order {
		if unit := graph.units[id]; unit.ImportGraphReadsAll || len(unit.ImportGraphReads) != 0 {
			readers = append(readers, id)
		}
	}
	if len(readers) == 0 {
		return
	}
	changes := graph.importSets
	if changes == nil {
		changes = graph.unknownImportSets(normalized)
	}
	if len(changes.Changed) == 0 && changes.Module == "" {
		return
	}
	closure := graph.importClosure(changes.Changed)
	var first string
	for _, id := range graph.order {
		if _, changed := changes.Changed[id]; changed {
			first = id
			break
		}
	}
	for _, id := range readers {
		if _, seen := reached[id]; seen {
			continue
		}
		unit := graph.units[id]
		var witness Witness
		found := false
		switch {
		case changes.Module != "":
			witness, found = Witness{Kind: WitnessImportGraphReader, DirtyPath: changes.Module, Via: []string{id}}, true
		case unit.ImportGraphReadsAll:
			if first != "" {
				parent := closure[first]
				witness, found = Witness{Kind: WitnessImportGraphReader, DirtyPath: parent.DirtyPath, link: parent.extended(id)}, true
			}
		default:
			for _, target := range unit.ImportGraphReads {
				parent, hit := closure[target]
				if !hit || found && parent.ViaLength() >= witness.ViaLength()-1 {
					continue
				}
				witness, found = Witness{Kind: WitnessImportGraphReader, DirtyPath: parent.DirtyPath, link: parent.extended(id)}, true
			}
		}
		if found {
			reached[id] = witness
		}
	}
}

// importClosure walks the importer edges from each changed unit breadth
// first, in unit order, and returns every unit whose import closure holds a
// changed unit, with the shortest chain from it. Additive narrowing
// (ADN-V0-003) prunes no edge here: an import edge is in the closure whether
// or not the importer references what changed.
func (graph *Graph) importClosure(changed map[string]string) map[string]Witness {
	closure := make(map[string]Witness, len(changed))
	var queue []string
	seeds := make([]string, 0, len(changed))
	for id := range changed {
		seeds = append(seeds, id)
	}
	slices.Sort(seeds)
	for _, id := range seeds {
		if _, known := graph.units[id]; !known {
			continue
		}
		closure[id] = Witness{Kind: WitnessImportGraphReader, DirtyPath: changed[id], Via: []string{id}}
		queue = append(queue, id)
	}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		parent := closure[current]
		for _, next := range graph.dependents[current] {
			if _, seen := closure[next]; seen {
				continue
			}
			closure[next] = Witness{Kind: WitnessImportGraphReader, DirtyPath: parent.DirtyPath, link: parent.extended(next)}
			queue = append(queue, next)
		}
	}
	return closure
}
