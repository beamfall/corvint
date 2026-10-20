package main

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// affectedMixedWarning is the AFP-V0-044 warning: a --base range path outside
// the worktree dirty set is planned together with uncommitted worktree content
// that the receipt's revision does not pin. It is empty, and the member is
// omitted, for a clean worktree, the worktree-only form, or a range inside the
// dirty set.
func affectedMixedWarning(revision string, dirty, committed []string) string {
	if len(dirty) == 0 || len(committed) == 0 {
		return ""
	}
	dirtySet := make(map[string]struct{}, len(dirty))
	for _, path := range dirty {
		dirtySet[path] = struct{}{}
	}
	for _, path := range committed {
		if _, ok := dirtySet[path]; !ok {
			return fmt.Sprintf("MIXED worktree: the --base range is planned at commit %s together with %d dirty worktree paths whose mutable content no commit pins.", revision, len(dirty))
		}
	}
	return ""
}

// affectedMixedWorktreeRefusal is the AFP-V0-045 --require-clean refusal,
// returned before the range diff, the graph walk or any provider runs.
func affectedMixedWorktreeRefusal(revision string, dirty int) error {
	return &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --require-clean refuses to plan; HEAD is commit %s with %d dirty worktree paths", revision, dirty)}
}

// affectedPathListMixedRefusal is the AFP-V0-094 refusal: `--path` plans at
// HEAD, and a worktree with any status entry would let uncommitted bytes into
// a graph the receipt binds to that commit.
func affectedPathListMixedRefusal(revision string, dirty int) error {
	return &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --path plans at HEAD and refuses a dirty worktree; HEAD is commit %s with %d dirty worktree paths", revision, dirty)}
}

// affectedPathListIgnoredRefusal names an ignored file HEAD does not track that
// a --path plan would read from disk (AFP-V0-094).
func affectedPathListIgnoredRefusal(revision, path string) error {
	return &gokernel.Error{Code: "unsupported-affected-mixed-worktree", Message: fmt.Sprintf("MIXED worktree: --path plans at HEAD and refuses ignored file %s, which HEAD does not track and the graph reads; HEAD is commit %s", path, revision)}
}
