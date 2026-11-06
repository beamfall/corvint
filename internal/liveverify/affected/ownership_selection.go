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

// ownershipFixtureHosts identifies the selected hosts that keep readers terminal when every
// unowned input is a tracked membership change below a selected Go fixture
// host. frontierUnknowns must still judge every plugin's reasons against
// those paths; this only avoids bypassing that proof through participation.
// Other plans retain their existing participation and serialized output.
func (graph *Graph) ownershipFixtureHosts(unowned []string, reached map[string]Witness) []string {
	if len(graph.ownershipChanges) == 0 || len(unowned) == 0 {
		return nil
	}
	if _, exists := graph.owner[OwnershipAuditTestPath]; !exists {
		return nil
	}
	packages := graph.goPackageDirectories()
	hosts := make(map[string]bool)
	for _, path := range unowned {
		directory := fixtureDirectory(path)
		if directory == "" || !slices.Contains(graph.ownershipChanges, path) {
			return nil
		}
		for parent := relativeDirectory(directory); ; parent = relativeDirectory(parent) {
			if id, exists := packages[parent]; exists {
				if _, selected := reached[id]; !selected || graph.untested(id) {
					return nil
				}
				hosts[id] = true
				break
			}
			if parent == "." {
				return nil
			}
		}
	}
	return sortedKeys(hosts)
}

// These exclusions compose terminal traversal with the per-reason fixture
// proof; they cannot use the ordinary certificate that denies unowned paths.
func (graph *Graph) ownershipFixtureExclusionReason(name string, hosts, entries []string) string {
	classifications := make([]string, 0, len(entries))
	for _, entry := range entries {
		fields := strings.SplitN(entry, ":", 3)
		reason := fields[0] + ":" + fields[1]
		classification := "FixtureClosedFrontiers"
		if graph.sourceBound[name][reason] {
			classification = "SourceBoundFrontiers"
		}
		classifications = append(classifications, classification+" ("+entry+")")
	}
	return "unowned tracked fixture membership changes have selected Go hosts " + strings.Join(hosts, ", ") +
		"; this plugin is reached only through terminal reader or binary-exec selections; excluded entries cannot bind those fixtures under FrontierReach: " + strings.Join(classifications, "; ")
}

func (graph *Graph) ownershipHints(unknown []Unknown) {
	// Guidance belongs to the repository's ownership ratchet. Repositories
	// without that test retain their existing receipts, including frozen Core
	// output; a tracked-path observation alone does not enable the feature.
	if _, exists := graph.owner[OwnershipAuditTestPath]; !exists {
		return
	}
	for i := range unknown {
		entry := &unknown[i]
		// Content-only edits retain their frozen plan bytes (AZU-V0-9900),
		// including committed unowned paths in a repository with the ratchet.
		if entry.Reason != UnknownUnownedDirtyPath || !graph.committedPaths[entry.Detail] || !slices.Contains(graph.ownershipChanges, entry.Detail) {
			continue
		}
		hint := "declare runtime/test inputs in " + RuntimeReadersPath + ", documentation in " + DocumentationPathsPath + ", or evidence/output in " + NonSourcePathsPath + " (AZU-V0-9901)"
		entry.Hint = strings.TrimPrefix(entry.Hint+"; "+hint, "; ")
	}
}
