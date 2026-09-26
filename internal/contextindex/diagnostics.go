package contextindex

import (
	"context"
	"errors"
	"strings"

	"github.com/Beamfall/corvint/internal/diagnostic"
)

// classifyHeadFailure turns a failed identity read into the coded CCF-V1-004 refusal when HEAD
// does not resolve, as in a repository with no commit yet; `rev-parse --verify --quiet` exits 1
// with no output exactly then. Any other failure keeps its original error.
func classifyHeadFailure(ctx context.Context, root string, err error) error {
	_, probeErr := git(ctx, root, maxIdentityBytes, nil, "rev-parse", "--verify", "--quiet", "HEAD")
	var failure *Error
	if !errors.As(probeErr, &failure) || failure.gitFailure == nil || failure.gitFailure.ExitCode != 1 {
		return err
	}
	return unbornHeadRefusal()
}

func unbornHeadRefusal() error {
	return &diagnostic.Error{Err: &Error{Code: "repository-head-unborn", Message: "repository has no resolvable HEAD commit"}, Refusal: diagnostic.Refusal{
		Subject:        diagnostic.Subject{Kind: "repository-state", Value: "head-commit"},
		SupportedFixes: []string{"git.create-head-commit"},
	}}
}

// classifyMissingObjects turns a failed read into the coded CCF-V1-004 refusal when the read failed
// on an object a partial clone left on its promisor remote. Corvint's Git environment sets
// GIT_NO_LAZY_FETCH and allows no transport, so the read fails instead of fetching it. The first
// probe exits 0 only if every object missing below tips is a promisor object; the second lists them.
// Neither fetches. The refusal names the first listed object that the failed read itself named, in
// cause (Git's stderr or the object the caller rejected) or as a tip, so it never blames an object
// the read did not need. The code is the one CEM-CB-019 gives a missing promised object. Any other
// failure keeps its original error.
func classifyMissingObjects(ctx context.Context, root string, err error, cause string, tips ...string) error {
	if _, probeErr := git(ctx, root, maxIdentityBytes, nil, append([]string{"rev-list", "--objects", "--quiet", "--missing=allow-promisor"}, tips...)...); probeErr != nil {
		return err
	}
	missing, probeErr := git(ctx, root, maxStatusBytes, nil, append([]string{"rev-list", "--objects", "--quiet", "--missing=print"}, tips...)...)
	if probeErr != nil {
		return err
	}
	named := cause + "\n" + strings.Join(tips, "\n")
	for _, line := range strings.Split(string(missing), "\n") {
		object, isMissing := strings.CutPrefix(line, "?")
		if !isMissing || object == "" || !strings.Contains(named, object) {
			continue
		}
		return &diagnostic.Error{Err: &Error{Code: "repository-object-unavailable", Message: "repository object unavailable"}, Refusal: diagnostic.Refusal{
			Subject:        diagnostic.Subject{Kind: "repository-state", Value: "promisor-object"},
			Evidence:       []diagnostic.Evidence{{Name: "object", Value: object}},
			SupportedFixes: []string{"git.fetch-promisor-objects"},
		}}
	}
	return err
}

// gitStderr is the stderr a failed Git read retained, or "" for any other error.
func gitStderr(err error) string {
	details, _ := GitFailureDetails(err)
	return string(details.Stderr)
}
