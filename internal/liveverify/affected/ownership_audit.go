package affected

// OwnershipAudit lists, in byte order, each of the tracked paths that no
// plugin, the driver claimant or a project declaration claims (AZU-V0-002):
// plus the planner control declarations themselves. Each
// path is classified on its own, by the same seed and declaration steps a plan
// runs. A claimed declaration can still carry selection uncertainty when a
// reader names it; the audit never changes that uncertainty or any selection.
func (graph *Graph) OwnershipAudit(tracked []string) []string {
	_, unknown := graph.seed(NormalizePaths(tracked))
	_, unknown = graph.taskState(unknown)
	_, unknown = graph.documentationPaths(unknown)
	_, unknown = graph.runtimeReaderPaths(unknown)
	_, unknown = graph.nonSourcePaths(unknown)
	_, unknown = graph.ownedPaths(unknown)
	_, unknown = graph.declarationPath(unknown)
	unowned := []string{}
	for _, entry := range unknown {
		if entry.Reason == UnknownUnownedDirtyPath && !auditControlDeclaration(entry.Detail) {
			unowned = append(unowned, entry.Detail)
		}
	}
	return unowned
}

// AZU-V0-8900: these exact control files are project declarations even when
// their readers make the selection conservatively retain UNOWNED_DIRTY_PATH.
// No directory wildcard grants ownership of a new arbitrary .corvint file.
func auditControlDeclaration(path string) bool {
	switch path {
	case DocumentationPathsPath, RuntimeReadersPath, UnboundedReaderExclusionsPath,
		NonSourcePathsPath, DeclaredSelectionPath:
		return true
	}
	return false
}
