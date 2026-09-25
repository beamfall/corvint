package typescript

import (
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// workspaceUnitPrefix names the unit that stands for a workspace package: a
// directory whose package.json declares the name another file imports.
const workspaceUnitPrefix = "typescript:workspace:"

// workspace resolves a bare import of a workspace package by its package.json
// name (TJAA-V0-005). Each such package becomes one unit, built only once
// something imports it, that imports every source unit inside the package
// directory: a superset of what the package's entry can reach, so a change
// anywhere in the package reaches the importer.
type workspace struct {
	directories map[string][]string
	files       []affected.Unit
	units       map[string]affected.Unit
	resolved    map[string]bool
}

func newWorkspace(scopes []packageScope, files []affected.Unit) *workspace {
	directories := make(map[string][]string)
	for _, scope := range scopes {
		if scope.name != "" {
			directories[scope.name] = append(directories[scope.name], scope.directory)
		}
	}
	return &workspace{directories: directories, files: files, units: map[string]affected.Unit{}, resolved: map[string]bool{}}
}

// link adds an edge for every workspace package the bare reference names. It
// reports false when a named package has no source unit to stand for it, or
// more than one unit may import, so the caller keeps the import a frontier.
func (w *workspace) link(reference string, edges map[string]bool) bool {
	if strings.HasPrefix(reference, ".") || externalScheme(reference) {
		return true
	}
	linked := true
	for _, directory := range w.directories[importedPackage(reference)] {
		id := workspaceUnitPrefix + directory
		present := w.build(id, directory)
		linked = linked && present
		if present {
			edges[id] = true
		}
	}
	return linked
}

// build records the unit for the package in directory once and reports
// whether it exists.
func (w *workspace) build(id, directory string) bool {
	if present, built := w.resolved[id]; built {
		return present
	}
	sources := make(map[string]bool)
	for _, file := range w.files {
		if len(file.Sources) != 0 && inside(file.Sources[0], directory) {
			sources[file.ID] = true
		}
	}
	present := len(sources) != 0 && len(sources) <= affected.MaxPathsPerUnit
	w.resolved[id] = present
	if present {
		w.units[id] = affected.Unit{ID: id, Imports: sortedKeys(sources)}
	}
	return present
}

// built lists the package units built so far.
func (w *workspace) built() []affected.Unit {
	units := make([]affected.Unit, 0, len(w.units))
	for _, id := range sortedKeys(w.units) {
		units = append(units, w.units[id])
	}
	return units
}
