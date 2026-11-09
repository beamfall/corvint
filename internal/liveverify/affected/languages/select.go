// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"context"
	"io/fs"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
)

// SelectAt plans dirty over graph, built from the tree under head, as
// `corvint affected --base` does: it first compares each dirty Go package's
// import set at commit, read from the repository under root, with head
// (AFP-V0-2202), so an import-graph reader is selected only when an import
// set changed (AFP-V0-9011). An unpinned or unreadable commit leaves the
// comparison unknown, and the plan then counts every dirty Go file as an
// import-set change, as a `--path` plan does.
func SelectAt(ctx context.Context, root, head, commit string, graph *affected.Graph, dirty []string) affected.Plan {
	graph = graph.WithImportSetChanges(golang.ImportSetChangesAt(ctx, root, head, commit, graph, dirty))
	return affected.Select(graph.WithGoChanges(golang.GoChangesAt(ctx, root, head, commit, dirty)), dirty)
}

// SelectBetween is SelectAt over two source trees: base, the tree the change
// is measured from, and head, the tree graph was built from (AFP-V0-9011). A
// nil base is unknown, as an unreadable commit is for SelectAt.
func SelectBetween(base, head fs.FS, graph *affected.Graph, dirty []string) affected.Plan {
	var changes *affected.ImportSetChanges
	switch {
	case !graph.HasImportGraphReaders() || len(dirty) == 0:
		changes = &affected.ImportSetChanges{Changed: map[string]string{}}
	case base != nil && head != nil:
		changes = golang.ImportSetRecords(base, head, graph, dirty)
	}
	return affected.Select(graph.WithImportSetChanges(changes).WithGoChanges(golang.GoChangeRecords(base, head, dirty)), dirty)
}
