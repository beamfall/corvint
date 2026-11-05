// SPDX-License-Identifier: Apache-2.0

package affected

import (
	json "encoding/json/v2"
	"slices"
	"strings"
)

// These are the existing shell declaration's members, read here without a
// dependency from the graph back into a language plugin.
type inboundCallerSnapshot struct {
	Profile string `json:"profile"`
	Scripts map[string]struct {
		Callers  []string `json:"callers"`
		Mentions []string `json:"mentions,omitempty"`
		Reason   string   `json:"reason"`
	} `json:"scripts"`
}

// inboundRevisionPaths requires actual base/head bytes from the CLI's bounded
// immutable base reader. A bare Select has no historical caller evidence and
// retains foreign-callers-changed. AZU-V0-004, AZU-V0-8904.
func (graph *Graph) inboundRevisionPaths(dirty []string) ([]string, bool) {
	if !slices.Contains(dirty, ShellForeignCallersPath) {
		return nil, false
	}
	revision, observed := graph.driverContent[ShellForeignCallersPath]
	if !observed || !revision.HeadExists {
		return nil, false
	}
	for _, entry := range graph.frontier {
		if strings.HasPrefix(entry, "shell:foreign-callers-invalid") {
			return nil, false
		}
	}
	var paths []string
	for index, body := range [][]byte{revision.Base, revision.Head} {
		if index == 0 && !revision.BaseExists {
			continue
		}
		if len(body) == 0 || len(body) > 64<<10 {
			return nil, false
		}
		var snapshot inboundCallerSnapshot
		if json.Unmarshal(body, &snapshot, json.RejectUnknownMembers(true)) != nil || snapshot.Profile != "corvint-shell-foreign-callers/0" || len(snapshot.Scripts) == 0 || len(snapshot.Scripts) > 64 {
			return nil, false
		}
		for script, entry := range snapshot.Scripts {
			if !ValidRelativePath(script) || len(entry.Callers)+len(entry.Mentions) > 256 || entry.Reason == "" {
				return nil, false
			}
			if index == 1 {
				unit, exists := graph.units["shell:"+script]
				// Current completeness is granted only by the shell plugin.
				if !exists || !slices.Equal(unit.Callers, entry.Callers) {
					return nil, false
				}
			}
			paths = append(paths, script)
			for _, file := range append(slices.Clone(entry.Callers), entry.Mentions...) {
				if !ValidRelativePath(file) {
					return nil, false
				}
				paths = append(paths, file)
			}
		}
	}
	paths = NormalizePaths(paths)
	_, unknown := graph.seed(paths)
	// A removed or unindexed caller cannot be silently discarded.
	if len(unknown) != 0 {
		return nil, false
	}
	return paths, true
}

func (graph *Graph) inboundRevisionCallers(reached map[string]Witness, followed map[string]bool, dirty []string) {
	paths, closed := graph.inboundRevisionPaths(dirty)
	if !closed {
		return
	}
	seeds, _ := graph.seed(paths)
	for id := range seeds {
		seeds[id] = Witness{Kind: WitnessFrontierSite, DirtyPath: ShellForeignCallersPath, Via: []string{id}}
	}
	closure := graph.traverse(seeds, nil)
	for id := range closure {
		followed[id] = true
	}
	graph.testUsersOf(closure, nil)
	graph.execUsersOf(closure, graph.builtCommands(closure, nil))
	for id, witness := range closure {
		if _, exists := reached[id]; !exists {
			reached[id] = witness
		}
	}
}
