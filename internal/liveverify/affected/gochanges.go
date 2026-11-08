// SPDX-License-Identifier: Apache-2.0

package affected

import "strings"

const GoCommentOnly = "COMMENT_ONLY"

// GoChange records a proven content classification, not an inference from a
// path or graph. Unclassified paths retain full propagation (AFP-V0-3320).
type GoChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// WithGoChanges attaches the Go adapter's BASE/HEAD comparison to this plan's
// graph copy. Like importSets, this is per-selection evidence, never cached
// graph state. Callers must compare the same endpoints and dirty paths they
// plan; absent evidence keeps full propagation (AFP-V0-3323). A .go file a
// //go:embed may name has byte consumers beyond compilation, so a path a unit
// embeds, or any path below an embedding Go package (SourcePath's rule), is
// never narrowed (AFP-V0-3324).
func (graph *Graph) WithGoChanges(changes []GoChange) *Graph {
	copied := *graph
	copied.goCommentOnly = make(map[string]bool)
	var embedding []string
	if len(changes) != 0 {
		embedding = graph.embeddingDirectories()
	}
	for _, change := range changes {
		owner, owned := graph.owner[change.Path]
		if change.Kind != GoCommentOnly || !owned || !strings.HasPrefix(owner, "go:") || !ValidRelativePath(change.Path) || !strings.HasSuffix(change.Path, ".go") {
			continue
		}
		if len(graph.embedded[change.Path]) == 0 && !underEmbedding(embedding, change.Path) {
			copied.goCommentOnly[change.Path] = true
		}
	}
	return &copied
}

// commentOnlyUnit reports a seeded unit every dirty input of which is a
// proven COMMENT_ONLY file. It compiles identically, so no selectable test
// can observe a behavior change and the coverage-gap check does not apply
// (AFP-V0-3325). One code edit to the unit restores the check.
func (graph *Graph) commentOnlyUnit(id string, seed Witness, dirty []string) bool {
	if !graph.goCommentOnly[seed.DirtyPath] || graph.owner[seed.DirtyPath] != id {
		return false
	}
	for _, relative := range dirty {
		if graph.owner[relative] == id && !graph.goCommentOnly[relative] {
			return false
		}
	}
	return true
}

func (graph *Graph) compilingPaths(dirty []string) []string {
	paths := make([]string, 0, len(dirty))
	for _, relative := range dirty {
		if !graph.goCommentOnly[relative] {
			paths = append(paths, relative)
		}
	}
	return paths
}

func (graph *Graph) goChanges(dirty []string) []GoChange {
	var changes []GoChange
	for _, relative := range dirty {
		if graph.goCommentOnly[relative] {
			changes = append(changes, GoChange{Path: relative, Kind: GoCommentOnly})
		}
	}
	return changes
}

// Unproven go-list calls can observe source comments (for example .Doc),
// even when the import graph is unchanged. Keep their independent byte-read
// uncertainty without traversing the compile/import graph (AFP-V0-3322).
func (graph *Graph) commentSourceReadersOf(reached map[string]Witness, dirty []string) {
	var source string
	for _, relative := range dirty {
		if graph.goCommentOnly[relative] {
			source = relative
			break
		}
	}
	if source == "" {
		return
	}
	for _, id := range graph.order {
		unit := graph.units[id]
		if !unit.ImportGraphReadsFiles || !unit.ImportGraphReadsAll && len(unit.ImportGraphReads) == 0 {
			continue
		}
		addWitness(reached, id, Witness{
			Kind: WitnessUnboundedReader, DirtyPath: source, Via: []string{id},
			Cause: &UnboundedCause{Unit: id, Reads: "unproven go list may observe source comments or package documentation"},
		})
	}
}
