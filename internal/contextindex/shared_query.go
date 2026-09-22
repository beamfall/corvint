package contextindex

import "context"

// queryBracket is the shared stability bracket of one repository query
// (CORVINT_QUERY_SHARED_OBSERVATION=1, proposed amendment; not accepted). The
// concurrent path observes the repository five times on a snapshot hit: the
// loader's pair, the trace read's pair before and after its files, and the
// learn stage's pair around its history read. Every one of those pairs is
// taken against the same index fields, so one window that opens on the
// loader's pair and closes after the history read proves the same thing:
// nothing the query read moved while it read. The closing is one full
// identity read beside one status scan, and it carries the trace read's
// deferred comparison in front of the learn stage's own checks, so a
// repository whose closing observation differs from the index is refused
// with the same unsupported-query-drift code the concurrent path reports.
// Like the harness window (GPK-V0-058), this samples the repository at the
// two ends only: a change that appears and is undone between them is not
// observed, where the concurrent path's middle brackets might have seen it.
type queryBracket struct {
	opening repositoryObservation
	// traceClosing is owed when the trace read ran against the opening and
	// deferred its closing comparison here (tracerecordrepo.ReadObserved).
	traceClosing bool
}

// open is the learn stage's opening observation: a bracket answers from the
// loader's pair without a spawn; no bracket issues the concurrent path's own.
func (bracket *queryBracket) open(ctx context.Context, index *Index) *historyProbe {
	if bracket == nil {
		return startHistoryObservation(ctx, index)
	}
	probe := &historyProbe{done: make(chan struct{})}
	probe.observation = historyObservation{
		statusSHA256: bracket.opening.statusSHA256,
		head:         bracket.opening.identity.commitRevision,
		tree:         bracket.opening.identity.treeRevision,
	}
	close(probe.done)
	return probe
}

// close is the learn stage's closing observation. A bracket reads the full
// identity beside the status scan and runs the trace read's deferred
// comparison first, in the shape and words tracerecordrepo reports it.
func (bracket *queryBracket) close(ctx context.Context, index *Index) (historyObservation, error) {
	if bracket == nil {
		return observeHistoryTree(ctx, index), nil
	}
	observed := observeRepository(ctx, index.Root)
	closing := historyObservation{
		statusSHA256: observed.statusSHA256, dirtyErr: observed.statusErr,
		head: observed.identity.commitRevision, tree: observed.identity.treeRevision, identityErr: observed.identityErr,
	}
	// A Git failure at the closing is the learn stage's to report, in the
	// words and code it uses on the concurrent path; only a successful
	// observation can be compared.
	if observed.identityErr != nil || observed.statusErr != nil {
		return closing, nil
	}
	// The full identity is compared whether or not the trace read is owed a
	// closing: this observation is also the identity re-read the loader
	// made after decoding (LoadSharedQuerySnapshot), and the learn stage's
	// own checks compare the tree and the status digest but not the commit.
	drifted := observed.identity != repositoryIdentity{objectFormat: index.ObjectFormat, commitRevision: index.CommitRevision, treeRevision: index.Revision, historyCut: bracket.opening.identity.historyCut}
	if drifted && !bracket.traceClosing {
		return closing, &Error{Code: "unsupported-query-drift", Message: "repository revision changed during history learning"}
	}
	if !bracket.traceClosing {
		return closing, nil
	}
	if drifted || observed.statusSHA256 != index.StatusSHA256 || !slicesEqual(observed.dirty, index.DirtyPaths) {
		return closing, &Error{Code: "unsupported-query-drift", Message: "repository changed while reading local traces: repository identity changed"}
	}
	return closing, nil
}

// EvalQueryShared is EvalQuery opening the learn stage's window on the
// loader's observation and closing it once. traceClosing says the trace read
// ran against that same opening (tracerecordrepo.ReadObserved) and left its
// closing comparison to this bracket; a read that was blocked by a dirty
// worktree never compared anything and owes nothing.
func EvalQueryShared(ctx context.Context, index *Index, text string, limit int, budget *int, snapshot QueryTraceSnapshot, opening *LoaderObservation, traceClosing bool) (map[string]any, error) {
	if !snapshot.valid {
		return nil, &Error{Code: "unsupported-query-trace-state", Message: "native Go repository query received an invalid local trace snapshot"}
	}
	if opening == nil {
		return nil, &Error{Message: "shared query bracket requires the loader's observation"}
	}
	bracket := &queryBracket{opening: opening.observed, traceClosing: traceClosing}
	return evalQuery(ctx, index, text, limit, budget, &snapshot, bracket, nil)
}
