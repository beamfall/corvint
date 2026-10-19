package affected

import "strings"

// Namers exposes the AFP-V0-021 reader match so external tests can resolve a
// PATH_LITERAL_READER witness.
func (graph *Graph) Namers(dirtyPath string) []string { return graph.namers(dirtyPath) }

// namers lists, in id order, the units carrying a path token that names
// dirtyPath.
func (graph *Graph) namers(dirtyPath string) []string {
	parts := strings.Split(dirtyPath, "/")
	ids := make([]string, 0)
	for i, tokens := range graph.pathTokens() {
		if namesAny(tokens, parts) {
			ids = append(ids, graph.order[i])
		}
	}
	return ids
}
