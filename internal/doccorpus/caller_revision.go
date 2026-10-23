package doccorpus

import (
	"context"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// CallerRevision relates the caller's checkout commit to the corpus source
// revision (DCP-V1-332). Relation is equal, descendant (the caller's commit
// descends from the source revision), ancestor (the source revision descends
// from the caller's commit), diverged (neither descends from the other) or
// unknown when the reader cannot observe the ancestry; Reason always says why.
type CallerRevision struct {
	Revision       string `json:"revision"`
	SourceRevision string `json:"source_revision"`
	Relation       string `json:"relation"`
	Reason         string `json:"reason"`
}

// Fixed caller-revision reasons. None echoes Git output.
const (
	callerEqualReason      = "caller revision is the corpus source revision"
	callerDescendantReason = "caller revision descends from the corpus source revision; changes after the source revision are not in the corpus"
	callerAncestorReason   = "caller revision is an ancestor of the corpus source revision; the corpus describes later commits"
	callerDivergedReason   = "caller revision and the corpus source revision have diverged; neither descends from the other"
	callerNoGitReason      = "reader has no Git repository access; caller revision ancestry NOT_OBSERVED"
	callerRepositoryReason = "reader repository could not be opened; caller revision ancestry NOT_OBSERVED"
	callerAbsentReason     = "caller or source revision is not present in the reader repository, or its history is shallow; caller revision ancestry NOT_OBSERVED"
	callerReadReason       = "Git ancestry read failed; caller revision ancestry NOT_OBSERVED"
)

// ValidCallerRevision admits an absent caller revision or a full lowercase
// Git object ID.
func ValidCallerRevision(revision string) bool {
	return revision == "" || wire.IsGitOid(revision)
}

// relateCallerRevision compares caller with source in the primary checkout at
// root. An empty root is a reader without Git access. Every failure to observe
// the ancestry is unknown with a fixed reason, never a guessed relation; only
// a cancelled context is returned as an error.
func relateCallerRevision(ctx context.Context, root, source, caller string) (*CallerRevision, error) {
	out := &CallerRevision{Revision: caller, SourceRevision: source}
	answer := func(relation, reason string) (*CallerRevision, error) {
		out.Relation, out.Reason = relation, reason
		return out, nil
	}
	if caller == source {
		return answer("equal", callerEqualReason)
	}
	if root == "" {
		return answer("unknown", callerNoGitReason)
	}
	auth, err := gitauth.Open(root, newGitBudget(ctx))
	if err != nil {
		return answer("unknown", callerRepositoryReason)
	}
	unobserved := func(err error) (*CallerRevision, error) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if cemcode.CodeOf(err) == cemcode.RepositoryObjectUnavailable {
			return answer("unknown", callerAbsentReason)
		}
		return answer("unknown", callerReadReason)
	}
	descends, err := auth.IsAncestor(ctx, source, caller)
	if err != nil {
		return unobserved(err)
	}
	if descends {
		return answer("descendant", callerDescendantReason)
	}
	precedes, err := auth.IsAncestor(ctx, caller, source)
	if err != nil {
		return unobserved(err)
	}
	if precedes {
		return answer("ancestor", callerAncestorReason)
	}
	return answer("diverged", callerDivergedReason)
}
