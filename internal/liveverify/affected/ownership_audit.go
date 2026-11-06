package affected

import "context"

// AuditOwnership is the common repository ownership classifier for the CLI
// and its maintained ratchet. The caller supplies its immutable tracked list
// (or the index list during development); no baseline alters ownership.
func AuditOwnership(ctx context.Context, root string, tracked []string, languages ...Language) ([]string, error) {
	graph, err := Build(ctx, root, languages...)
	if err != nil {
		return nil, err
	}
	return graph.OwnershipAudit(tracked), nil
}

// OwnershipAudit lists, in byte order, each of the tracked paths that no
// plugin, the driver claimant or a project declaration claims (AZU-V0-002):
// the paths a plan with that path dirty reports as UNOWNED_DIRTY_PATH. Each
// path is classified on its own, by the same seed and declaration steps a plan
// runs, so a path the audit omits raises no such entry in any plan over this
// graph. It reads only the graph.
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
		if entry.Reason == UnknownUnownedDirtyPath {
			unowned = append(unowned, entry.Detail)
		}
	}
	return unowned
}
