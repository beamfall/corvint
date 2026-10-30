// SPDX-License-Identifier: Apache-2.0
package affected

import "strings"

// fixtureDirectory returns the outermost `testdata` directory a repository-
// relative path lies below, or "" when it has no `testdata` component.
func fixtureDirectory(relative string) string {
	components := strings.Split(relative, "/")
	for index, component := range components[:len(components)-1] {
		if component == "testdata" {
			return strings.Join(components[:index+1], "/")
		}
	}
	return ""
}

// promoteUnhostedFixtureFrontier raises the own frontier of every fixture unit
// that no Go unit hosts back to its plugin's frontier (AFP-V0-446). A plugin
// may keep a frontier site raised inside a `testdata` directory on the unit
// that owns it instead of on the plugin, because the Go toolchain builds and
// runs nothing below `testdata` and the enclosing package's tests read the
// fixture only as bytes, which are dirty paths of their own. That holds only
// where a Go unit encloses the directory; elsewhere `testdata` is an ordinary
// directory, so the site widens every plan the plugin takes part in, as a
// plugin-wide site does. A promoted site is listed in full: the plugin's
// per-reason `+N` bound (TJAA-V0-052) applies to the sites it names itself.
func (graph *Graph) promoteUnhostedFixtureFrontier(frontier map[string]bool) {
	packages := graph.goPackageDirectories()
	for _, id := range graph.order {
		unit := graph.units[id]
		if len(unit.Frontier) == 0 || strings.HasPrefix(id, "go:") || len(unit.Sources)+len(unit.Tests) == 0 {
			continue
		}
		first := append(append([]string(nil), unit.Sources...), unit.Tests...)[0]
		directory := fixtureDirectory(first)
		if directory == "" || graph.goUnitAbove(directory, packages) {
			continue
		}
		name, _, _ := strings.Cut(id, ":")
		merged := make(map[string]bool, len(graph.frontierBy[name])+len(unit.Frontier))
		for _, entry := range graph.frontierBy[name] {
			merged[entry] = true
		}
		for _, entry := range unit.Frontier {
			merged[entry] = true
			frontier[entry] = true
		}
		graph.frontierBy[name] = sortedKeys(merged)
		if graph.promoted == nil {
			graph.promoted = make(map[string]bool)
		}
		graph.promoted[id] = true
	}
}

// goUnitAbove reports whether a Go unit's directory encloses directory.
func (graph *Graph) goUnitAbove(directory string, packages map[string]string) bool {
	for parent := relativeDirectory(directory); ; parent = relativeDirectory(parent) {
		if _, present := packages[parent]; present {
			return true
		}
		if parent == "." {
			return false
		}
	}
}
