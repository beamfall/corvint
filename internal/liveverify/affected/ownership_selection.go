// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"slices"
	"strings"
)

const OwnershipAuditTest = "TestOwnershipAuditRatchet_AZU_V0_002"
const OwnershipAuditTestPath = "internal/liveverify/affected/ownership_audit_repo_test.go"
const WitnessOwnershipAudit = "OWNERSHIP_AUDIT"

// WithOwnershipChanges binds tracked membership changes and paths committed at
// HEAD to this plan. Neither belongs in the immutable graph cache (AZU-V0-9900).
func (graph *Graph) WithOwnershipChanges(changed, committed []string) *Graph {
	copied := *graph
	copied.ownershipChanges = NormalizePaths(changed)
	copied.committedPaths = make(map[string]bool, len(committed))
	for _, path := range committed {
		copied.committedPaths[path] = true
	}
	return &copied
}

func (graph *Graph) ownershipTrigger(dirty []string) string {
	for _, path := range dirty {
		if slices.Contains(graph.ownershipChanges, path) || path == RuntimeReadersPath || path == DocumentationPathsPath || path == NonSourcePathsPath {
			return path
		}
	}
	return ""
}

func (graph *Graph) selectOwnershipAudit(plan *Plan, reached map[string]Witness, dirty []string) {
	trigger := graph.ownershipTrigger(dirty)
	id, exists := graph.owner[OwnershipAuditTestPath]
	if trigger == "" || !exists {
		return
	}
	reason := OwnershipAuditTest + ": tracked path membership or ownership declaration changed via " + trigger + " (AZU-V0-9900)"
	if _, selected := reached[id]; selected {
		// Keep independent package coverage; the ratchet cannot narrow it.
		plan.MustRun = append(plan.MustRun, MustRun{UnitID: id, Tests: OwnershipAuditTestPath, Reason: reason})
		slices.SortFunc(plan.MustRun, func(a, b MustRun) int {
			if n := strings.Compare(a.UnitID, b.UnitID); n != 0 {
				return n
			}
			return strings.Compare(a.Tests, b.Tests)
		})
		return
	}
	reached[id] = Witness{Kind: WitnessOwnershipAudit, DirtyPath: trigger, Via: []string{id}}
}

func (graph *Graph) ownershipHints(unknown []Unknown) {
	for i := range unknown {
		entry := &unknown[i]
		if entry.Reason != UnknownUnownedDirtyPath || !graph.committedPaths[entry.Detail] {
			continue
		}
		hint := "declare runtime/test inputs in " + RuntimeReadersPath + ", documentation in " + DocumentationPathsPath + ", or evidence/output in " + NonSourcePathsPath + " (AZU-V0-9901)"
		entry.Hint = strings.TrimPrefix(entry.Hint+"; "+hint, "; ")
	}
}

// A newly tracked Go-hosted fixture is read by its selected enclosing package.
// Its missing ownership declaration remains an unknown for the ratchet to judge;
// it is not also evidence of an unrelated language's hidden edge (AZU-V0-9901).
// Resolution inputs and paths listed by another plugin remain open.
func (graph *Graph) ownershipFrontierPaths(paths []string, reached map[string]Witness) []string {
	if len(paths) == 0 || len(graph.ownershipChanges) == 0 || len(graph.committedPaths) == 0 {
		return paths
	}
	packages := graph.goPackageDirectories()
	var open []string
	for _, path := range paths {
		closed := slices.Contains(graph.ownershipChanges, path) && graph.committedPaths[path]
		directory := fixtureDirectory(path)
		closed = closed && directory != "" && graph.goUnitAbove(directory, packages)
		if closed {
			hostSelected := false
			for dir, id := range packages {
				if dir == "." || strings.HasPrefix(directory, dir+"/") {
					if _, ok := reached[id]; ok {
						hostSelected = true
					}
				}
			}
			closed = hostSelected
		}
		if closed {
			for _, name := range graph.languages {
				if name == "go" || name == DriverNamespace {
					continue
				}
				if graph.listsBelowAny(name, []string{directory}) {
					closed = false
					break
				}
				if reach, declares := graph.claimants[name].(FrontierReach); declares && reach.ResolutionInput(path) {
					closed = false
					break
				}
			}
		}
		if !closed {
			open = append(open, path)
		}
	}
	return open
}
