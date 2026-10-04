package affected

// Namers exposes the AFP-V0-021 reader match so external tests can resolve a
// PATH_LITERAL_READER witness.
func (graph *Graph) Namers(dirtyPath string) []string { return graph.namers(dirtyPath) }
