package worktreeimpact

import "github.com/Beamfall/corvint/internal/diagnostic"

// The DRC-V0 refusals of the seven unsupported-working-tree-impact-* sites (decision 0201 (j)).
// Each constructor keeps its site's code and message byte-identical and attaches one
// diagnostic.Refusal literal; the coverage ratchet counts one literal per site, so the two
// sites that share a message still keep one constructor each.

const (
	suffixMessage  = "untracked-path impact is implemented for `.go` files only; commit or `git add -N` the file to use tracked-path impact"
	pathCode       = "unsupported-working-tree-impact-path"
	repositoryCode = "unsupported-working-tree-impact-repository"
)

func refused(code, message string, refusal diagnostic.Refusal) error {
	return &diagnostic.Error{Err: failure(code, message), Refusal: refusal}
}

// missingIndexRefusal: no captured revision index exists, so no caller-side repair is known.
func missingIndexRefusal() error {
	return refused(repositoryCode, "working-tree impact requires a captured revision index", diagnostic.Refusal{
		Subject:  diagnostic.Subject{Kind: "repository-state", Value: "captured-revision-index"},
		Terminal: "absent-evidence",
	})
}

func moduleRefusal(module string) error {
	return refused(repositoryCode, "working-tree impact requires a slash-qualified Go module", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "repository-state", Value: "go-module-path"},
		Evidence:       []diagnostic.Evidence{{Name: "module", Value: module}},
		SupportedFixes: []string{"go-mod.qualify-module-path"},
	})
}

func trackedPathRefusal(cleaned, revision string) error {
	return refused(pathCode, "working-tree impact accepts only paths absent from the captured revision: "+cleaned, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: cleaned},
		Evidence:       []diagnostic.Evidence{{Name: "revision", Value: revision}},
		SupportedFixes: []string{"worktree-impact.remove-path", "impact.use-tracked-path-profile"},
	})
}

func unobservedPathRefusal(cleaned, revision string) error {
	return refused(pathCode, "working-tree impact path is not present in the captured mixed-worktree status: "+cleaned, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: cleaned},
		Evidence:       []diagnostic.Evidence{{Name: "revision", Value: revision}},
		SupportedFixes: []string{"worktree-impact.remove-path", "worktree-impact.make-path-untracked"},
	})
}

// rawSuffixRefusal refuses a raw operand before normalization.
func rawSuffixRefusal(value string) error {
	return refused(pathCode, suffixMessage, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: value},
		SupportedFixes: []string{"worktree-impact.remove-path", "git.add-intent-to-add", "impact.use-tracked-path-profile"},
	})
}

// cleanSuffixRefusal refuses a normalized operand.
func cleanSuffixRefusal(value string) error {
	return refused(pathCode, suffixMessage, diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: value},
		SupportedFixes: []string{"worktree-impact.remove-path", "git.add-intent-to-add", "impact.use-tracked-path-profile"},
	})
}

func rootPathRefusal(value string) error {
	return refused(pathCode, "working-tree impact requires nested .go paths", diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "value", Value: value},
		SupportedFixes: []string{"worktree-impact.remove-path", "worktree-impact.nest-path"},
	})
}
