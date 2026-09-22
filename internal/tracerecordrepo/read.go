package tracerecordrepo

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/trace"
)

// ReadFailure classifies a failed trace read without changing its diagnostic
// bytes. The query adapter maps these categories to its stable refusal codes.
type ReadFailure string

const (
	ReadFailureTraceState   ReadFailure = "trace-state"
	ReadFailureReplayWindow ReadFailure = "replay-window"
	ReadFailureDrift        ReadFailure = "drift"
	ReadFailureHistory      ReadFailure = "history"
	ReadFailureRepository   ReadFailure = "repository"
)

type readError struct {
	kind              ReadFailure
	cause             error
	truncatedAncestry int
}

func (err *readError) Error() string {
	if err.truncatedAncestry == 0 {
		return err.cause.Error()
	}
	return fmt.Sprintf("%s (truncated ancestry: %d)", err.cause, err.truncatedAncestry)
}
func (err *readError) Unwrap() error { return err.cause }

func readFailure(kind ReadFailure, err error) error {
	if err == nil {
		return nil
	}
	var existing *readError
	if errors.As(err, &existing) {
		return err
	}
	return &readError{kind: kind, cause: err}
}

func readFailureWithTruncatedAncestry(kind ReadFailure, err error, count int) error {
	return &readError{kind: kind, cause: err, truncatedAncestry: count}
}

// ReadFailureOf returns the stable failure category attached by Read.
func ReadFailureOf(err error) (ReadFailure, bool) {
	var failure *readError
	if !errors.As(err, &failure) {
		return "", false
	}
	return failure.kind, true
}

// TruncatedAncestryOf returns the number of ancestry rows omitted from the
// bounded Git result attached to a failed read. It also reaches through a
// *contextindex.Error that a downstream mapper rebuilt from this package's
// error (via its Cause field), so the count survives that flattening without
// parsing it back out of formatted error text.
func TruncatedAncestryOf(err error) (int, bool) {
	var failure *readError
	if errors.As(err, &failure) && failure.truncatedAncestry != 0 {
		return failure.truncatedAncestry, true
	}
	var rebuilt *contextindex.Error
	if errors.As(err, &rebuilt) && rebuilt.Cause != nil {
		return TruncatedAncestryOf(rebuilt.Cause)
	}
	return 0, false
}

// Read returns the pinned local traces without creating or changing trace state.
func Read(ctx context.Context, root string, index *contextindex.Index) ([]trace.Record, string, error) {
	return read(ctx, root, index, stabilityCheck(ctx, root, index, nil))
}

// ReadObserved is Read checking stability against the observation the index
// was loaded under instead of spawning the pair before and after its files.
// The opening comparison is the concurrent path's first check verbatim; the
// closing one is deferred to the query's shared bracket
// (contextindex.EvalQueryShared), which closes the same window once.
func ReadObserved(ctx context.Context, root string, index *contextindex.Index, opening contextindex.Observation) ([]trace.Record, string, error) {
	return read(ctx, root, index, observedStabilityCheck(root, index, opening))
}

func read(ctx context.Context, root string, index *contextindex.Index, checkStable func() error) ([]trace.Record, string, error) {
	if len(index.DirtyPaths) != 0 {
		return []trace.Record{}, "blocked-mixed-worktree", nil
	}
	tracked := make([]string, 0, len(index.Sources))
	for path := range index.Sources {
		tracked = append(tracked, path)
	}
	sort.Strings(tracked)
	revisions, _, err := bindRevisions(ctx, root, index, tracked, strictRevisionBinding)
	if err != nil {
		return nil, "", err
	}
	store, err := trace.NewStore(root, revisions, checkStable)
	if err != nil {
		return nil, "", readFailure(ReadFailureTraceState, err)
	}
	records, state, err := store.Read()
	if err == nil {
		return records, state, nil
	}
	if trace.IsDrift(err) {
		return nil, "", readFailure(ReadFailureDrift, err)
	}
	var changed *repositoryChangedError
	if errors.As(err, &changed) {
		return nil, "", readFailure(ReadFailureDrift, err)
	}
	if _, ok := contextindex.GitFailureDetails(err); ok {
		return nil, "", readFailure(ReadFailureHistory, err)
	}
	var repositoryError *contextindex.Error
	if errors.As(err, &repositoryError) {
		return nil, "", readFailure(ReadFailureRepository, err)
	}
	return nil, "", readFailure(ReadFailureTraceState, err)
}
