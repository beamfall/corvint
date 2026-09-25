package contextindex

import (
	"context"
	"errors"

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
