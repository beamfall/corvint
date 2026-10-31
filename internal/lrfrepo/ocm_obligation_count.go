package lrfrepo

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// IntentObligationCounts is the number of obligations `ocm prepare` derives,
// in the default intent form, from each intent path in target's tree: one per
// exact requirement, as prepare's deriveIntent counts them. It reads only Git
// objects and no CEM, so a caller can learn an aggregate bound's outcome in
// seconds, before any verification runs (OCM-V0-028). An error names the
// first path prepare itself would refuse; the caller then leaves the decision
// to prepare and the aggregate.
func IntentObligationCounts(ctx context.Context, root, target string, paths []string) ([]int, error) {
	repository, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	defer repository.BeginObjectSession()()
	resolved, err := repository.Resolve(ctx, target)
	if err != nil {
		return nil, err
	}
	reader := &ocmBlobReader{ctx: ctx, repository: repository, cache: map[string][]byte{}}
	counts := make([]int, 0, len(paths))
	for _, path := range paths {
		entry, exists, err := repository.LookupTreeEntry(ctx, resolved, path)
		if err != nil {
			return nil, err
		}
		if !exists || entry.Type != "blob" {
			return nil, fail("repository-object-unavailable", "repository object unavailable: %s is not a blob at %s", path, resolved)
		}
		blob, err := reader.blob(resolved, path, entry.OID)
		if err != nil {
			return nil, err
		}
		derived, err := deriveIntent("", path, entry.OID, blob)
		if err != nil {
			return nil, err
		}
		counts = append(counts, len(derived.requirements))
	}
	return counts, nil
}
