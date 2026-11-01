package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// PoolTargetSourceEnv names the REVISION:TREE pair a retarget operation's
// command receives (PSR-V0-163).
const PoolTargetSourceEnv = "CORVINT_POOL_TARGET_SOURCE"

// PoolOperate runs one declared typed member lifecycle operation
// (PSR-V0-160..167). The operation receipt precedes execution, so a
// same-request retry replays the recorded outcome and never runs the
// command again. A pending receipt it settles first is reported as Redone.
func PoolOperate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice) (*Report, error) {
	redone, e := settleLease(ctx, repo)
	if e != nil {
		return &Report{}, e
	}
	report, e := poolOperate(ctx, repo, actor, choice)
	if redone && report != nil {
		report.Redone = true
	}
	return report, e
}

func poolOperate(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice) (*Report, error) {
	choice.Lease.Verb = transaction.LeasePoolOperate
	report, e := leaseOnce(WithClock(ctx, storeClock(repo)), repo, actor, choice, storeClock(repo)())
	if report.Receipt != "" {
		// The receipt may commit before a post/head write fails (CAL-V0-215).
		report.Unretryable = true
	}
	if e != nil {
		return report, e
	}
	if report.Kind == "Replay" {
		// PSR-V0-164: a replay reports the recorded outcome and runs nothing.
		return attachOperationReceipt(ctx, repo, choice, report)
	}
	if report.Kind != "Transaction" {
		return report, nil
	}
	// The operation receipt is committed; from here no result is retryable
	// (CAL-V0-078).
	report.Unretryable = true
	if poolPreparedFault != nil {
		if e = poolPreparedFault(); e != nil {
			return report, e
		}
	}
	p, state, e := poolSnapshot(ctx, repo)
	if e != nil {
		return report, e
	}
	for _, en := range state.Entries {
		if en.MemberID != choice.Lease.Member || en.Operation == nil || en.Operation.RequestID != choice.RequestID {
			continue
		}
		def := poolMemberCommand(p, en.MemberID, en.Operation.Name)
		if def == nil {
			return report, wire.Errorf(wire.CodeUnsupported, "pool operate", "operation no longer declared")
		}
		raw, cause := runPool(ctx, repo, choice, en, def)
		if cause != nil {
			// PSR-V0-165: a refusal before execution is settled in this
			// request as a failed receipt, so the member is quarantined and
			// never left pending.
			if raw, e = operationNotRun(leaseRoot(repo, choice), en, cause); e != nil {
				return report, e
			}
		}
		observed, e := observePool(context.WithoutCancel(ctx), repo, actor, choice, en, raw)
		if observed == nil {
			observed = &Report{}
		}
		observed.Unretryable = true
		if e != nil {
			return observed, e
		}
		observed, e = attachOperationReceipt(ctx, repo, choice, observed)
		if cause != nil && e == nil {
			observed.Detail = "operation not run: " + cause.Error()
		}
		return observed, e
	}
	return report, wire.Errorf(wire.CodeFenced, "pool operate", "operation entry changed before execution")
}

// operationNotRun is the failed observation of an operation whose runner
// refused before execution (PSR-V0-165): SOURCE_CHANGED when the prepared
// source moved or became dirty, otherwise SPAWN_FAILED. It is bound to the
// prepared revision; when even that cannot be read, cause is returned and
// the entry stays pending for recovery.
func operationNotRun(root string, en snapshot.PoolEntry, cause error) ([]byte, error) {
	tree, e := resolveObject(root, en.CommandRevision, "tree")
	if e != nil {
		return nil, cause
	}
	class := "SPAWN_FAILED"
	if c := wire.CodeOf(cause); c == wire.CodeStaleTree || c == wire.CodeDirtyWorktree {
		class = "SOURCE_CHANGED"
	}
	o := snapshot.PoolObservation{AllocationID: en.AllocationID, DefinitionSha256: en.DefinitionSha256, Kind: en.CommandKind, Revision: en.CommandRevision, Tree: tree, Class: class, GroupClean: true, OutputSha256: wire.Sum(nil), EnvironmentSha256: wire.Sum(nil)}
	return o.Encode(), nil
}

// attachOperationReceipt reports the member's recorded receipt for this
// request, or says why none is recorded.
func attachOperationReceipt(ctx context.Context, repo *intent.Repository, choice LeaseChoice, report *Report) (*Report, error) {
	_, state, e := poolSnapshot(ctx, repo)
	if e != nil {
		return report, e
	}
	if rec := state.PoolOperationReceiptFor(choice.Lease.Member, choice.Lease.Operation); rec != nil && rec.RequestID == choice.RequestID {
		x := *rec
		report.PoolOperation = &x
		return report, nil
	}
	for _, en := range state.Entries {
		if en.MemberID == choice.Lease.Member && en.Operation != nil && en.Operation.RequestID == choice.RequestID {
			report.Detail = "operation pending; its runner has not recorded an outcome and replay never runs it again"
			return report, nil
		}
	}
	rec, e := recordedOperation(repo, choice)
	if e != nil {
		return report, e
	}
	if rec != nil {
		report.PoolOperation = rec
		return report, nil
	}
	report.Detail = "operation outcome not recorded; its runner never settled it and replay never runs it again"
	return report, nil
}

// recordedOperation reads this request's receipt from the pools.json its own
// observe transaction committed (PSR-V0-164). A later run replaces the
// member's current receipt, so the request's journal record is the durable
// answer; the read is bounded by the pool state limit and adds nothing to
// pools.json. nil means that transaction never completed.
func recordedOperation(repo *intent.Repository, choice LeaseChoice) (*snapshot.PoolOperationReceipt, error) {
	probe, e := snapshot.Probe(repo.StateDir)
	if e != nil {
		return nil, e
	}
	index := journal.RequestIndex{Reader: journalReader(repo, probe.Head)}
	entry, found, e := index.Lookup(poolChildID(choice.RequestID, "observe"))
	if e != nil || !found || entry.Outcome.Outcome != mutation.OutcomeCompleted || entry.Outcome.ReceiptSeq == nil {
		return nil, e
	}
	state, e := sweepReceiptPools(repo, entry.Outcome.ReceiptSeq.Uint64())
	if e != nil {
		return nil, e
	}
	if rec := state.PoolOperationReceiptFor(choice.Lease.Member, choice.Lease.Operation); rec != nil && rec.RequestID == choice.RequestID {
		x := *rec
		return &x, nil
	}
	return nil, wire.Errorf(wire.CodeJournalForked, "pool operate", "observe receipt does not record this request")
}
