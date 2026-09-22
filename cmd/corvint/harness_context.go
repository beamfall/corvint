package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/trace"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// harnessIndexedContext builds the context block for the two harness events that
// require a repository index. It is injected into the kernel rather than imported
// by it, so internal/gokernel stays dependency-free.
func harnessIndexedContext(ctx context.Context, root, event string, normalized map[string]any, budgetBytes int) (map[string]any, error) {
	if event == "user-prompt" {
		task, _ := normalized["task"].(string)
		result, queryErr := repositoryQueryContext(ctx, root, task, harnessContextLimit, &budgetBytes)
		if queryErr != nil {
			return nil, queryErr
		}
		return sanitizeHarnessQueryContext(result, task)
	}
	compact := event == "session-start"
	return overSnapshot(
		func() *contextindex.Index {
			return snapshotHit(contextindex.LoadEventSnapshotDeferred(ctx, root, compact))
		},
		func() *contextindex.Index { return snapshotHit(contextindex.LoadEventSnapshot(ctx, root, compact)) },
		func() (*contextindex.Index, error) { return contextindex.Build(ctx, root) },
		func(index *contextindex.Index) (map[string]any, error) {
			return harnessIndexedBlock(index, event, normalized, budgetBytes)
		},
	)
}

// repositoryQueryContext is the single production path for broad repository
// tasks. Both the CLI adapter and harness user-prompt route through the same
// BuildEval -> EvalQuery edge; authority-start remains a separate narrow path.
// An optional loaded index is the batch seam (SBQ-V0-003): it replaces this
// path's own acquisition and leaves every later stage, the trace read included,
// exactly where it was.
func repositoryQueryContext(ctx context.Context, root, task string, limit int, budget *int, loaded ...*contextindex.Index) (map[string]any, error) {
	if sharedQueryObservation() {
		ctx = contextindex.WithSharedQueryObservation(ctx)
		index, opening := preloadedIndex(loaded), preloadedOpening(ctx)
		if index == nil {
			index, opening = querySnapshotIndex(ctx, root)
		}
		if index == nil {
			var err error
			index, err = contextindex.BuildEval(ctx, root)
			if err != nil {
				return nil, err
			}
		}
		return repositoryQueryOver(ctx, root, index, task, limit, budget, opening)
	}
	if index := preloadedIndex(loaded); index != nil {
		return repositoryQueryOver(ctx, root, index, task, limit, budget)
	}
	return overSnapshot(
		func() *contextindex.Index { return deferredSnapshotIndex(ctx, root) },
		func() *contextindex.Index { return snapshotIndex(ctx, root) },
		func() (*contextindex.Index, error) { return contextindex.BuildEval(ctx, root) },
		func(index *contextindex.Index) (map[string]any, error) {
			return repositoryQueryOver(ctx, root, index, task, limit, budget)
		},
	)
}

// repositoryQueryOver answers a broad repository task over an acquired index.
func repositoryQueryOver(ctx context.Context, root string, index *contextindex.Index, task string, limit int, budget *int, openings ...*contextindex.LoaderObservation) (map[string]any, error) {
	var opening *contextindex.LoaderObservation
	if len(openings) > 0 {
		opening = openings[0]
	}
	if contextindex.QueryIntent(task) == "project-operations" {
		return contextindex.EvalQuery(ctx, index, task, limit, budget)
	}
	// The shared bracket (CORVINT_QUERY_SHARED_OBSERVATION=1, proposed; not
	// accepted) runs the trace read and the learn stage inside the loader's
	// own window instead of beside two more pairs of their own. It exists
	// only on a hit: a built index carries no opening pair here.
	if opening != nil && sharedQueryObservation() {
		records, state, err := tracerecordrepo.ReadObserved(ctx, root, index, opening.Observation())
		snapshot, err := queryTraceSnapshot(ctx, records, state, err)
		if err != nil {
			return nil, err
		}
		return contextindex.EvalQueryShared(ctx, index, task, limit, budget, snapshot, opening, state != "blocked-mixed-worktree")
	}
	// The trace read (ancestry log plus one stability observation) reads the
	// index and the repository, never the learn stage's observation, so it
	// runs beside that stage's opening pair and the ranking pass instead of
	// before them; EvalQueryPending joins it before the first comparison and
	// before every earlier return, keeping the error order and the receipt.
	var snapshot contextindex.QueryTraceSnapshot
	var readErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		records, state, err := tracerecordrepo.Read(ctx, root, index)
		snapshot, readErr = queryTraceSnapshot(ctx, records, state, err)
	}()
	pending := func() (contextindex.QueryTraceSnapshot, error) {
		<-done
		return snapshot, readErr
	}
	return contextindex.EvalQueryPending(ctx, index, task, limit, budget, pending)
}

// queryTraceSnapshot observes one trace read for a batch and binds it as the
// snapshot EvalQuery consumes; a read failure keeps its query error code.
func queryTraceSnapshot(ctx context.Context, records []trace.Record, state string, err error) (contextindex.QueryTraceSnapshot, error) {
	observeBatchTraces(ctx, records, state)
	if err != nil {
		return contextindex.QueryTraceSnapshot{}, mapRepositoryQueryTraceError(err)
	}
	traces := make([]contextindex.QueryTrace, len(records))
	for offset, record := range records {
		traces[offset] = contextindex.QueryTrace{
			TraceID: record.TraceID, Task: record.Task, Outcome: record.Outcome, Revision: record.Revision,
			OpenedPaths: record.OpenedPaths, ChangedPaths: record.ChangedPaths,
		}
	}
	return contextindex.NewQueryTraceSnapshot(state, traces)
}

// sharedQueryObservation is the opt-in for the shared query bracket (proposed
// GPK-V0-065; not accepted).
func sharedQueryObservation() bool {
	return runtimeenv.Value("QUERY_SHARED_OBSERVATION") == "1"
}

// querySnapshotIndex is snapshotIndex for the repository query path: the same
// hit, plus the loader's observation the shared bracket opens on. Under the
// opt-in the loader takes no closing identity read: every stage that answers
// from this hit closes a bracket that compares the tree, the project-operations
// intent through the learn stage's own pair and the rest through the shared
// window, so the bracket's closing is that read.
func querySnapshotIndex(ctx context.Context, root string) (*contextindex.Index, *contextindex.LoaderObservation) {
	load := loadQuerySnapshot
	if sharedQueryObservation() {
		load = loadSharedQuerySnapshot
	}
	index, hit, opening, err := load(ctx, root)
	if err != nil || !hit {
		return nil, nil
	}
	return index, opening
}

// batchOpeningKey carries the batch loader's observation to each query
// operation beside the loaded index the SBQ-V0-003 seam already passes.
type batchOpeningKey struct{}

func preloadedOpening(ctx context.Context) *contextindex.LoaderObservation {
	opening, _ := ctx.Value(batchOpeningKey{}).(*contextindex.LoaderObservation)
	return opening
}

func mapRepositoryQueryTraceError(err error) error {
	kind, ok := tracerecordrepo.ReadFailureOf(err)
	if !ok {
		return &contextindex.Error{Code: "unsupported-query-trace-state", Message: err.Error(), Cause: err}
	}
	codes := map[tracerecordrepo.ReadFailure]string{
		tracerecordrepo.ReadFailureTraceState:   "unsupported-query-trace-state",
		tracerecordrepo.ReadFailureReplayWindow: "unsupported-query-trace-state",
		tracerecordrepo.ReadFailureDrift:        "unsupported-query-drift",
		tracerecordrepo.ReadFailureHistory:      "unsupported-query-history",
		tracerecordrepo.ReadFailureRepository:   "unsupported-query-repository",
	}
	return &contextindex.Error{Code: codes[kind], Message: err.Error(), Cause: err}
}

const (
	// harnessContextLimit is the oracle's fixed limit for the query and impact
	// context blocks.
	harnessContextLimit = 10
	// rehydrationOverheadBytes and rehydrationHeadroomBytes reserve room for the
	// rehydration block itself and for its encoded tracked-path list.
	rehydrationOverheadBytes = 512
	rehydrationHeadroomBytes = 512
	// maxCompactionPaths and maxRehydrationPaths bound the dirty set a compaction
	// may rehydrate, matching the oracle's MAX_ITEMS and MAX_LIMIT.
	maxCompactionPaths  = 256
	maxRehydrationPaths = 50
)

// compactionContext rebuilds the working set a compaction discarded. Tracked
// dirty paths are rehydrated through impact so the agent recovers their
// evidence; untracked ones exist at no revision, so they can only be counted.
func compactionContext(index *contextindex.Index, budgetBytes int) (map[string]any, error) {
	dirty := append([]string(nil), index.DirtyPaths...)
	sort.Strings(dirty)
	if len(dirty) == 0 {
		return map[string]any{
			"availableOperations": []any{"file-change", "user-prompt"},
			"profile":             index.ProfileID,
			"state":               "READY",
		}, nil
	}
	tracked, untracked := partitionDirtyPaths(index, dirty)
	untrackedDigest, err := canonicalDigest(untracked)
	if err != nil {
		return nil, err
	}
	rehydration := map[string]any{
		"state":                     partialOrComplete(untracked),
		"trackedDirtyPathCount":     len(tracked),
		"untrackedDirtyPathCount":   len(untracked),
		"untrackedDirtyPathsSha256": untrackedDigest,
	}
	rehydrationBudget := budgetBytes - rehydrationOverheadBytes
	encodedTracked, err := gokernel.CanonicalJSON(tracked)
	if err != nil {
		return nil, err
	}
	overBudget := len(dirty) > maxCompactionPaths ||
		len(tracked) > maxRehydrationPaths ||
		len(encodedTracked) > rehydrationBudget-rehydrationHeadroomBytes
	if overBudget {
		dirtyDigest, digestErr := canonicalDigest(dirty)
		if digestErr != nil {
			return nil, digestErr
		}
		return map[string]any{
			"dirtyPathCount":   len(dirty),
			"dirtyPathsSha256": dirtyDigest,
			"rehydration":      rehydration,
			"state":            "REHYDRATION_BOUNDED",
		}, nil
	}
	if len(tracked) == 0 {
		// Reached only below the budget bound: an over-budget dirty set keeps the
		// PARTIAL/COMPLETE state, exactly as the oracle leaves it.
		rehydration["state"] = "UNTRACKED_ONLY"
		return map[string]any{"rehydration": rehydration, "state": "REHYDRATION_PARTIAL"}, nil
	}
	block, err := contextindex.EvalImpact(index, tracked, len(tracked), &rehydrationBudget)
	if err != nil {
		return nil, err
	}
	block["rehydration"] = rehydration
	return block, nil
}

// partitionDirtyPaths splits the dirty set by whether the path is tracked at the
// indexed revision. Excluded paths are tracked: the index deliberately declined
// to read them, which is not the same as their not existing.
func partitionDirtyPaths(index *contextindex.Index, dirty []string) ([]string, []string) {
	tracked := make([]string, 0, len(dirty))
	untracked := make([]string, 0, len(dirty))
	trackedAtRevision := make(map[string]struct{}, len(index.Sources)+len(index.Exclusions))
	for path := range index.Sources {
		trackedAtRevision[path] = struct{}{}
	}
	for _, exclusion := range index.Exclusions {
		trackedAtRevision[exclusion.Path] = struct{}{}
	}
	for _, path := range dirty {
		if _, ok := trackedAtRevision[path]; ok {
			tracked = append(tracked, path)
			continue
		}
		untracked = append(untracked, path)
	}
	return tracked, untracked
}

func partialOrComplete(untracked []string) string {
	if len(untracked) != 0 {
		return "PARTIAL"
	}
	return "COMPLETE"
}

func canonicalDigest(value []string) (string, error) {
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func harnessPaths(value any) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		return typed, nil
	case []any:
		paths := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, &gokernel.Error{Code: "invalid-harness-input", Message: "paths must be a bounded list"}
			}
			paths = append(paths, text)
		}
		return paths, nil
	}
	return nil, &gokernel.Error{Code: "invalid-harness-input", Message: "paths must be a bounded list"}
}

// sanitizeHarnessQueryContext drops the echoed prompt text and replaces it with
// its digest, so a receipt never carries the raw task.
func sanitizeHarnessQueryContext(value map[string]any, task string) (map[string]any, error) {
	encoded, err := gokernel.CanonicalJSON(value)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	if request, ok := result["request"].(map[string]any); ok {
		delete(request, "text")
		digest := sha256.Sum256([]byte(task))
		request["taskSha256"] = hex.EncodeToString(digest[:])
	}
	return result, nil
}

// harnessSharedIndexedContext is harnessIndexedContext inside the kernel's
// bracket (CORVINT_HARNESS_SHARED_OBSERVATION=1, proposed GPK-V0-058): the
// snapshot read consumes the bracket's opening observation and spawns no Git
// process; a miss builds as before, inside the bracket's read stage. The
// returned block is the same computation harnessIndexedContext makes, over an
// index already in memory, so it runs beside the closing observation.
func harnessSharedIndexedContext(ctx context.Context, root, event string, normalized map[string]any, budgetBytes int, observation gokernel.Observation) (gokernel.IndexedBlock, error) {
	index, hit, err := contextindex.LoadEventSnapshotObserved(root, event == "session-start", contextindex.Observation{
		ObjectFormat: observation.ObjectFormat, CommitRevision: observation.CommitRevision,
		Revision: observation.TreeRevision, DirtyPaths: observation.DirtyPaths,
		StatusSHA256: observation.StatusSHA256,
	})
	if err != nil || !hit {
		built, buildErr := contextindex.Build(ctx, root)
		if buildErr != nil {
			return nil, buildErr
		}
		index = built
	}
	return func() (map[string]any, error) {
		return harnessIndexedBlock(index, event, normalized, budgetBytes)
	}, nil
}

func harnessIndexedBlock(index *contextindex.Index, event string, normalized map[string]any, budgetBytes int) (map[string]any, error) {
	switch event {
	case "session-start":
		return compactionContext(index, budgetBytes)
	case "file-change":
		paths, pathsErr := harnessPaths(normalized["paths"])
		if pathsErr != nil {
			return nil, pathsErr
		}
		for _, value := range paths {
			if err := validateImpactPathAdmission(value); err != nil {
				return nil, err
			}
		}
		return contextindex.EvalImpact(index, paths, harnessContextLimit, &budgetBytes)
	}
	return nil, &gokernel.Error{Code: "unsupported-harness-event", Message: "unsupported harness event"}
}
