package appflows

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/extevidence"
	"github.com/Beamfall/corvint/internal/gitancestry"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// CodeRepositoryUnbound is the closed fallback for a declared application repository that is not
// bound to immutable Git content, or a bound one the provider does not declare (AFU-V1-069).
const CodeRepositoryUnbound = "e2e-repository-unbound"

// The binding states of one application repository in the result (AFU-V1-068).
const (
	RepositoryBound   = "bound"
	RepositoryUnbound = "unbound"
)

// SelectionRepository is one application repository the provider declares (AFU-V1-067): only a
// declared ID contributes changed paths or evidence, and only a checkout whose HEAD reaches the
// declared parentless origin commit binds it.
type SelectionRepository struct {
	ID     string `json:"id"`
	Origin string `json:"origin"`
}

// E2ERepositoryInput is one operator binding: `--repository ID=DIR` with its `--repository-base`.
type E2ERepositoryInput struct {
	ID     string
	Source string
	Base   string
}

// E2ERepository is one application repository's binding in the result (AFU-V1-068).
type E2ERepository struct {
	ID            string `json:"id"`
	Origin        string `json:"origin"`
	State         string `json:"state"`
	Reason        string `json:"reason,omitempty"`
	Base          string `json:"base"`
	Revision      string `json:"revision"`
	Tree          string `json:"tree"`
	WorktreePaths int    `json:"worktree_paths"`
}

// RepositoryPin is a bound application repository as link review reads it: its canonical checkout
// directory and the commit a qualified target is evaluated at (AFU-V1-071).
type RepositoryPin struct {
	Dir      string
	Revision string
}

// boundRepository is one bound repository: its pin and its qualified changed paths.
type boundRepository struct {
	pin     RepositoryPin
	changed []string
}

// splitQualified splits an `ID:PATH` qualified path; a root path in a flow document never holds a colon.
func splitQualified(p string) (id, rel string, ok bool) {
	id, rel, ok = strings.Cut(p, ":")
	return id, rel, ok && extevidence.ValidRepositoryID(id) && safePath(rel)
}

// validQualified reports a qualified path naming a declared repository.
func validQualified(p string, declared map[string]bool) bool {
	id, _, ok := splitQualified(p)
	return ok && declared[id]
}

// bindRepositories binds each declared repository to its operator checkout and reads its changed
// paths base..HEAD plus its worktree status, qualified with its ID (AFU-V1-068). A declared ID
// without a binding, a binding the provider does not declare, and any unreadable or drifting
// checkout is unbound and yields e2e-repository-unbound (AFU-V1-069).
func bindRepositories(ctx context.Context, root string, declared []SelectionRepository, inputs []E2ERepositoryInput) ([]E2ERepository, map[string]boundRepository, []E2EFallback) {
	byID := map[string]E2ERepositoryInput{}
	for _, in := range inputs {
		byID[in.ID] = in
	}
	rows, bound, codes := []E2ERepository{}, map[string]boundRepository{}, []E2EFallback{}
	known := map[string]bool{}
	for _, repo := range declared {
		known[repo.ID] = true
		row := E2ERepository{ID: repo.ID, Origin: repo.Origin, State: RepositoryUnbound, Reason: "not bound by --repository"}
		if in, ok := byID[repo.ID]; ok {
			var b boundRepository
			if b, row.Reason = bindRepository(ctx, root, repo, in, &row); row.Reason == "" {
				row.State, bound[repo.ID] = RepositoryBound, b
			}
		}
		if row.State != RepositoryBound {
			codes = append(codes, E2EFallback{Code: CodeRepositoryUnbound, Subject: repo.ID})
		}
		rows = append(rows, row)
	}
	for _, in := range inputs {
		if !known[in.ID] {
			rows = append(rows, E2ERepository{ID: in.ID, State: RepositoryUnbound, Reason: "not declared by the provider"})
			codes = append(codes, E2EFallback{Code: CodeRepositoryUnbound, Subject: in.ID})
		}
	}
	slices.SortFunc(rows, func(a, b E2ERepository) int { return strings.Compare(a.ID, b.ID) })
	return rows, bound, codes
}

// bindRepository resolves one checkout, filling row with what it observed; a nonempty reason unbinds it.
func bindRepository(ctx context.Context, root string, repo SelectionRepository, in E2ERepositoryInput, row *E2ERepository) (boundRepository, string) {
	canonical, head, reason := OpenCheckout(ctx, root, in.Source)
	if reason != "" {
		return boundRepository{}, reason
	}
	row.Revision = head
	tree, err := git(ctx, canonical, "rev-parse", "--verify", "--end-of-options", head+"^{tree}")
	if err != nil {
		return boundRepository{}, "checkout HEAD tree unreadable"
	}
	row.Tree = strings.TrimSpace(string(tree))
	if base, err := ResolveRevision(ctx, canonical, in.Base); err != nil || base != in.Base {
		return boundRepository{}, "repository base is not a commit of the checkout"
	}
	row.Base = in.Base
	roots, err := git(ctx, canonical, "rev-list", "--max-parents=0", head)
	if err != nil || !slices.Contains(strings.Fields(string(roots)), repo.Origin) {
		return boundRepository{}, "checkout HEAD does not reach the declared origin"
	}
	if _, err := git(ctx, canonical, "merge-base", "--is-ancestor", repo.Origin, in.Base); err != nil {
		return boundRepository{}, "repository base does not reach the declared origin"
	}
	changed, dirty, err := repositoryChanges(ctx, canonical, in.Base, head)
	if err != nil {
		return boundRepository{}, err.Error()
	}
	if reason := ancestryRewritten(ctx, canonical); reason != "" {
		return boundRepository{}, reason
	}
	row.WorktreePaths = len(dirty)
	qualified := make([]string, 0, len(changed))
	for _, p := range changed {
		// A path a qualified path cannot name (a colon, say) would be parsed back as a root path and
		// gain root impact-graph ownership; the repository is unbound instead (AFU-V1-069).
		if !safePath(p) {
			return boundRepository{}, "checkout changed a path a qualified path cannot name"
		}
		qualified = append(qualified, repo.ID+":"+p)
	}
	return boundRepository{pin: RepositoryPin{Dir: canonical, Revision: in.Base}, changed: qualified}, ""
}

// OpenCheckout resolves a --repository source (relative to root unless absolute) to the canonical
// top level of a Git checkout with trusted ancestry and returns it with its HEAD commit, or names
// why it cannot (AFP-V0-156, AFU-V1-068, ANL-V0-060).
func OpenCheckout(ctx context.Context, root, source string) (dir, head, reason string) {
	dir = source
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, filepath.FromSlash(dir))
	}
	canonical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", "", "cannot read checkout"
	}
	// Git walks up to an enclosing repository, so the directory must be the top level itself: an
	// empty prefix inside a worktree proves that without naming the top as a path (AFP-V0-156).
	where, err := git(ctx, canonical, "rev-parse", "--is-inside-work-tree", "--show-prefix")
	if errors.Is(err, errPlatformUnsupported) {
		return "", "", err.Error()
	}
	if err != nil || !strings.HasPrefix(string(where), "true\n") {
		return "", "", "not a Git checkout"
	}
	if string(where) != "true\n\n" {
		return "", "", "not the top level of a Git checkout"
	}
	if reason := ancestryRewritten(ctx, canonical); reason != "" {
		return "", "", reason
	}
	head, err = ResolveRevision(ctx, canonical, "HEAD")
	if err != nil {
		return "", "", "checkout has no HEAD commit"
	}
	return canonical, head, ""
}

// ancestryRewritten names why the checkout's commit ancestry cannot be trusted, or returns ""
// (AFU-V1-068; the shared guard is internal/gitancestry).
func ancestryRewritten(ctx context.Context, dir string) string {
	return gitancestry.Rewritten(ctx, func(ctx context.Context, args ...string) ([]byte, error) {
		return git(ctx, dir, args...)
	})
}

// repositoryChanges reads the tree diff base..HEAD and the worktree status between two status and
// HEAD reads; a change between them is drift, never a short list.
func repositoryChanges(ctx context.Context, dir, base, head string) (changed, dirty []string, err error) {
	gitPath := gitstatus.Executable()
	unreadable := errors.New("checkout changes unreadable")
	if dirty, err = affected.DirtyPaths(ctx, gitPath, dir); err != nil {
		return nil, nil, unreadable
	}
	committed, err := affected.RangePaths(ctx, gitPath, dir, base)
	if err != nil {
		return nil, nil, unreadable
	}
	recheck, err := affected.DirtyPaths(ctx, gitPath, dir)
	if err != nil {
		return nil, nil, unreadable
	}
	if after, err := ResolveRevision(ctx, dir, "HEAD"); err != nil || after != head || !slices.Equal(dirty, recheck) {
		return nil, nil, errors.New("checkout changed while it was read")
	}
	return affected.NormalizePaths(append(append([]string{}, committed...), dirty...)), dirty, nil
}
