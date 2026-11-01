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
		derived, _, err := deriveIntentAt(ctx, repository, reader, resolved, path, "")
		if err != nil {
			return nil, err
		}
		counts = append(counts, len(derived.requirements))
	}
	return counts, nil
}

// DeriveIntentAt derives the intent at path in commit target exactly as
// PrepareOCM begins its map, and returns its obligation count or the refusal
// PrepareOCM would return for it, so a pre-flight and the bind agree on every
// intent (DCW-V0-945).
func DeriveIntentAt(ctx context.Context, root, target, path string) (int, error) {
	counts, err := IntentObligationCounts(ctx, root, target, []string{path})
	if err != nil {
		return 0, err
	}
	return counts[0], nil
}

// deriveIntentAt is the one intent derivation PrepareOCM and the obligation
// counts share: the intent must be a blob at the resolved target and derive in
// form (DCW-V0-945). It returns the derivation and the intent's blob ID.
func deriveIntentAt(ctx context.Context, repository *gitauth.Repository, reader *ocmBlobReader, target, path, form string) (intentDerivation, string, error) {
	entry, exists, err := repository.LookupTreeEntry(ctx, target, path)
	if err != nil {
		return intentDerivation{}, "", err
	}
	if !exists || entry.Type != "blob" {
		return intentDerivation{}, "", fail("repository-object-unavailable", "repository object unavailable: intent %s is not a blob at %s", path, target)
	}
	blob, err := reader.blob(target, path, entry.OID)
	if err != nil {
		return intentDerivation{}, "", err
	}
	derived, err := deriveIntent(form, path, entry.OID, blob)
	return derived, entry.OID, err
}
