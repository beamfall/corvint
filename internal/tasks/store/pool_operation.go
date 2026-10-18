package store

import (
	"context"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
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
	report, e := leaseOnce(WithClock(ctx, poolClock), repo, actor, choice, poolClock())
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
		raw, e := runPool(ctx, repo, choice, en, def)
		if e != nil {
			return report, e
		}
		observed, e := observePool(context.WithoutCancel(ctx), repo, actor, choice, en, raw)
		if observed == nil {
			observed = &Report{}
		}
		observed.Unretryable = true
		if e != nil {
			return observed, e
		}
		return attachOperationReceipt(ctx, repo, choice, observed)
	}
	return report, wire.Errorf(wire.CodeFenced, "pool operate", "operation entry changed before execution")
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
	report.Detail = "operation outcome superseded by a later run of the same operation"
	return report, nil
}
