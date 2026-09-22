package touchsurprise

import (
	"strconv"

	"github.com/Beamfall/corvint/internal/diagnostic"
	"github.com/Beamfall/corvint/internal/gokernel"
)

// The DRC-V0 refusals of the unsupported-surprise-revision and
// unsupported-surprise-dirty-worktree sites. Each constructor keeps its site's
// code and message byte-identical and attaches one diagnostic.Refusal literal;
// this file parses no message text (DRC-V0-012).

func refused(code, message string, refusal diagnostic.Refusal) error {
	return &diagnostic.Error{Err: &gokernel.Error{Code: code, Message: message}, Refusal: refusal}
}

// revisionRefusal is shared by both resolveCommit exits: a failed rev-parse and
// an empty answer refuse the same operand with the same repair.
func revisionRefusal(revision string) error {
	return refused("unsupported-surprise-revision", "revision does not resolve to a commit: "+revision, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: revision},
		SupportedFixes: []string{"surprise.use-commit-revision"},
	})
}

// dirtyWorktreeRefusal carries the count of tracked status entries git reported.
func dirtyWorktreeRefusal(entries int) error {
	return refused("unsupported-surprise-dirty-worktree", "touch-set surprise compares committed revisions only; the worktree has uncommitted changes", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "repository-state", Value: "tracked-worktree-changes"},
		Evidence:       []diagnostic.Evidence{{Name: "tracked-status-entries", Value: strconv.Itoa(entries)}},
		SupportedFixes: []string{"git.clean-tracked-worktree"},
	})
}
