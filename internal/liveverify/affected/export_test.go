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

// SetGraphCacheHook stages a replacement race at a graph cache stage
// (AFP-V0-390, AFP-V0-392) and returns the restore.
func SetGraphCacheHook(hook func(stage string)) func() {
	saved := graphCacheHook
	graphCacheHook = hook
	return func() { graphCacheHook = saved }
}

// SetReadSource replaces the disk read behind Source.Read and returns the
// restore (AFP-V0-392).
func SetReadSource(read func(root, relative string) ([]byte, error)) func() {
	saved := readSourceAt
	readSourceAt = read
	return func() { readSourceAt = saved }
}
