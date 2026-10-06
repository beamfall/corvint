package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/authority"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Batch chunk bounds (CAL-V0-106). A chunk holds the writer lock for at most
// BatchChunkEntries entries, and starts no further entry once it has held the
// lock for BatchChunkHold; its first entry always runs. Between chunks the
// batch releases the lock and waits BatchYield, several DefaultLockPoll
// intervals, so a claim or heartbeat polling for the lock acquires it before
// the next chunk does.
const (
	BatchChunkEntries = 8
	BatchChunkHold    = 2 * time.Second
	BatchYield        = 5 * authority.DefaultLockPoll
)

// BatchEntry is one entry's result. Report is nil when the entry was not
// attempted; Err is the error that stopped the batch at this entry.
type BatchEntry struct {
	Report *Report
	Err    error
	// Chunk is the 1-based chunk that ran this entry, 0 when not attempted.
	Chunk int
}

// BatchReport is the result of MutateBatch, index-aligned with its input.
type BatchReport struct {
	Entries []BatchEntry
	Chunks  int
}

// batchStageKey carries a test-only callback that MutateBatch runs after it
// releases the lock between chunks, with the index of the next entry, so a
// test can commit another writer's transaction at that point.
type batchStageKey struct{}

// WithBatchYieldHook returns ctx with a hook MutateBatch calls after
// releasing the lock between two chunks and before it waits BatchYield.
func WithBatchYieldHook(ctx context.Context, hook func(next int)) context.Context {
	return context.WithValue(ctx, batchStageKey{}, hook)
}

// MutateBatch commits a batch of §3.3 mutation envelopes (CAL-V0-106). Every
// envelope is decoded and its actor checked before the first lock is taken,
// so a malformed batch, or one with an unadmitted actor, writes nothing. The envelopes are then applied in
// index order in chunks: each chunk acquires the writer lock, runs the writer
// guards and §5.2 redo once, and commits each of its entries exactly as
// Mutate would, with the entry's own request lookup, CAS and receipt. The
// lock is released between chunks. A refused entry (a stale expected
// revision, a conflicting request ID) is reported and the batch continues;
// an error, or a guard refusal while opening a chunk, stops the batch at that
// entry and leaves the rest not attempted.
func MutateBatch(ctx context.Context, repo *intent.Repository, actor mutation.Binding, envelopes [][]byte, now wire.Timestamp) (*BatchReport, error) {
	out := &BatchReport{Entries: make([]BatchEntry, len(envelopes))}
	if repo == nil {
		return out, wire.Errorf(wire.CodeMalformed, "", "no repository authority was resolved")
	}
	decoded := make([]*mutation.Envelope, len(envelopes))
	requests := make([]transaction.Request, len(envelopes))
	for i, raw := range envelopes {
		env, err := mutation.Decode(raw)
		if err != nil {
			return out, wire.Errorf(wire.CodeOf(err), fmt.Sprintf("entries[%d]", i), "%v", err)
		}
		request := transaction.Request{Operation: transaction.Mutate, QueueID: env.QueueID.Raw, RequestID: env.RequestID, Actor: actor, Envelope: raw}
		decoded[i], requests[i] = env, request
		if env.Actor.ID != actor.ID || env.Actor.Role != actor.Role || !transaction.ActorAdmitted(request) {
			// As in Mutate, the model refuses an unadmitted actor; a batch
			// with one such entry writes nothing and reports that entry.
			result := transaction.Model(request, transaction.Input{})
			out.Entries[i].Report = &Report{Outcome: result.Outcome, Coverage: result.Coverage, Detail: result.Detail, Kind: result.Kind}
			return out, nil
		}
	}
	if _, err := os.Stat(repo.StateDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, wire.Errorf(wire.CodeUninitialized, repo.StateDir, "no store: run `corvint-tasks init` first")
		}
		return out, err
	}
	for next := 0; next < len(envelopes); {
		if next > 0 {
			if hook, ok := ctx.Value(batchStageKey{}).(func(int)); ok {
				hook(next)
			}
			timer := time.NewTimer(BatchYield)
			select {
			case <-ctx.Done():
				timer.Stop()
				out.Entries[next].Err = ctx.Err()
				return out, nil
			case <-timer.C:
			}
		}
		out.Chunks++
		var stop bool
		next, stop = mutateChunk(ctx, repo, decoded, requests, now, next, out)
		if stop {
			break
		}
	}
	return out, nil
}

// mutateChunk runs entries from start under one writer lock and returns the
// next index and whether the batch must stop.
func mutateChunk(ctx context.Context, repo *intent.Repository, envs []*mutation.Envelope, requests []transaction.Request, now wire.Timestamp, start int, out *BatchReport) (int, bool) {
	fail := func(i int, report *Report, err error) (int, bool) {
		out.Entries[i] = BatchEntry{Chunk: out.Chunks}
		out.Entries[i].Report, out.Entries[i].Err = intentFix(repo, report, err)
		return i + 1, true
	}
	opened := func(err error) (int, bool) {
		report, err := guardFailure(&Report{}, envs[start].RequestID, err)
		return fail(start, report, err)
	}
	if _, err := authority.Qualify(repo.CommonDir); err != nil {
		return opened(err)
	}
	// Each entry's change watch closes after the lock is released, as in Mutate.
	watches := make([]*authority.ChangeGuard, 0, BatchChunkEntries)
	defer func() {
		for _, w := range watches {
			_ = w.Close()
		}
	}()
	lock, err := authority.AcquireLock(ctx, repo, authority.LockOptions{})
	if err != nil {
		return opened(err)
	}
	defer lock.Close()
	session, err := authority.NewSession(repo, lock)
	if err != nil {
		return opened(err)
	}
	defer session.Close()
	headState, err := writerGuards(repo, transaction.Mutate)
	if err != nil {
		return opened(err)
	}
	redone, err := redoPending(repo, session)
	if err != nil {
		return opened(err)
	}
	i := start
	for ; i < len(envs) && i-start < BatchChunkEntries; i++ {
		if i > start && time.Since(lock.HeldSince()) >= BatchChunkHold {
			break
		}
		report := &Report{Redone: redone}
		redone = false
		var watch *authority.ChangeGuard
		report, err := mutateLocked(ctx, repo, session, headState, requests[i], envs[i], recordedAt(ctx, now), report, &watch)
		if watch != nil {
			watches = append(watches, watch)
		}
		if err != nil {
			return fail(i, report, err)
		}
		report, _ = intentFix(repo, report, nil)
		out.Entries[i] = BatchEntry{Report: report, Chunk: out.Chunks}
	}
	return i, false
}
