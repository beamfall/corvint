package store

import (
	"context"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"strings"
	"time"
)

func poolSnapshot(ctx context.Context, repo *intent.Repository) (*intent.Policy, *snapshot.PoolState, error) {
	proof, e := readLeaseProof(ctx, repo)
	if e != nil {
		return nil, nil, e
	}
	p, e := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
	if e != nil {
		return nil, nil, e
	}
	q, e := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if e != nil {
		return nil, nil, e
	}
	state := &snapshot.PoolState{QueueID: q.QueueID}
	if raw := proof.Records["pools.json"].Raw; len(raw) > 0 {
		state, e = snapshot.DecodePools(raw)
	}
	return p, state, e
}
func poolSource(root string) (string, string, error) {
	rev, e := resolveObject(root, "HEAD", "commit")
	if e != nil {
		return "", "", e
	}
	tree, e := resolveObject(root, rev, "tree")
	if e != nil {
		return "", "", e
	}
	status, e := gitOutput(root, "status", "--porcelain", "--untracked-files=all", "--", ".", ":(exclude).taskman")
	if e != nil {
		return "", "", e
	}
	if len(status) != 0 {
		return "", "", wire.Errorf(wire.CodeDirtyWorktree, "pool command", "repository command inputs must be clean outside .taskman")
	}
	return rev, tree, nil
}
func poolConfig(root string, ref *intent.ConfigRef) error {
	if ref == nil {
		return nil
	}
	raw, e := gitOutput(root, "ls-tree", "-z", ref.Revision, "--", ref.Path)
	if e != nil {
		return e
	}
	parts := strings.SplitN(strings.TrimSuffix(string(raw), "\x00"), "\t", 2)
	if len(parts) != 2 || parts[1] != ref.Path {
		return wire.Errorf(wire.CodeMissingEvidence, "configRef", "exact immutable file absent")
	}
	fields := strings.Fields(parts[0])
	if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || fields[2] != ref.Blob {
		return wire.Errorf(wire.CodeMissingEvidence, "configRef", "regular immutable blob differs")
	}
	return nil
}
func poolFacts(repo *intent.Repository, choice LeaseChoice) claimObserver {
	return func(proof *journal.Result, in *transaction.Input) (transaction.LeaseFacts, error) {
		f := transaction.LeaseFacts{Pool: choice.pool}
		l := choice.Lease
		if l.Verb == transaction.LeasePoolPrepare || l.Verb == transaction.LeasePoolCleanup {
			rev, _, e := poolSource(leaseRoot(repo, choice))
			if e != nil {
				return f, e
			}
			identity, e := poolRunnerIdentity(os.Getpid())
			if e != nil {
				return f, e
			}
			f.Pool.RunnerPID = wire.CountOf(int64(os.Getpid()))
			f.Pool.RunnerStarted = identity
			f.Pool.Revision = rev
		}
		if l.Verb == transaction.LeasePoolRecover {
			state, e := snapshot.DecodePools(in.Pools)
			if e != nil {
				return f, e
			}
			for _, en := range state.Entries {
				if en.MemberID == l.Member && string(en.AllocationID) == l.Allocation {
					identity, e := poolRunnerIdentity(int(en.RunnerPID.Int()))
					if e != nil {
						return f, e
					}
					f.Pool.RunnerGone = identity == "" || identity != en.RunnerStarted
				}
			}
		}
		return f, nil
	}
}
func poolClock() wire.Timestamp {
	return wire.Timestamp(time.Now().UTC().Format("2006-01-02T15:04:05Z"))
}
func poolChildID(id, phase string) string {
	return "pool-" + string(wire.Sum([]byte(id + "\x00" + phase)))[:48]
}
func runPool(ctx context.Context, repo *intent.Repository, choice LeaseChoice, en snapshot.PoolEntry, def *intent.PoolCommand) ([]byte, error) {
	root := leaseRoot(repo, choice)
	rev, tree, e := poolSource(root)
	if e != nil {
		return nil, e
	}
	if rev != en.CommandRevision {
		return nil, wire.Errorf(wire.CodeStaleTree, "pool", "prepared source revision changed")
	}
	if e = poolConfig(root, en.ConfigRef); e != nil {
		return nil, e
	}
	env, _ := gateEnvironment(def.Env)
	class, clean, output := executePool(ctx, def, root, env)
	after, afterTree, e := poolSource(root)
	if e != nil || after != rev || afterTree != tree {
		class = "SOURCE_CHANGED"
	}
	observation := snapshot.PoolObservation{AllocationID: en.AllocationID, DefinitionSha256: en.DefinitionSha256, Kind: en.CommandKind, Revision: rev, Tree: tree, Class: class, Passed: class == "EXIT_ZERO" && clean, GroupClean: clean, OutputSha256: output, EnvironmentSha256: wire.Sum([]byte(strings.Join(env, "\x00")))}
	return observation.Encode(), nil
}
func observePool(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, en snapshot.PoolEntry, raw []byte) (*Report, error) {
	choice.RequestID = poolChildID(choice.RequestID, "observe")
	choice.Lease = transaction.LeaseRequest{Verb: transaction.LeasePoolObserve, Member: en.MemberID, Allocation: string(en.AllocationID)}
	choice.pool.Observation = raw
	return leaseOnce(ctx, repo, actor, choice, poolClock())
}

// PoolCommand explicitly probes a free member or cleans a quarantined allocation.
// The preparation receipt precedes execution; replay never repeats execution.
func PoolCommand(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, kind string) (*Report, error) {
	if _, e := settleLease(ctx, repo); e != nil {
		return &Report{}, e
	}
	p, state, e := poolSnapshot(ctx, repo)
	if e != nil {
		return &Report{}, e
	}
	var en snapshot.PoolEntry
	var def *intent.PoolCommand
	if kind == "health" {
		for _, pool := range p.Pools {
			for _, m := range pool.Members {
				if m == choice.Lease.Member {
					choice.Lease.Pool = pool.ID
					def = pool.MemberConfig[m].Health
				}
			}
		}
		choice.Lease.Verb = transaction.LeasePoolPrepare
		choice.Lease.Holder = actor.ID
		choice.Lease.Evidence = string(wire.Sum([]byte(choice.QueueID + ":" + choice.RequestID)))
	} else {
		choice.Lease.Verb = transaction.LeasePoolCleanup
		for _, entry := range state.Entries {
			if entry.MemberID == choice.Lease.Member && string(entry.AllocationID) == choice.Lease.Allocation {
				en = entry
				def = p.Pool(entry.PoolID).MemberConfig[entry.MemberID].Cleanup
			}
		}
	}
	if def == nil {
		return &Report{}, wire.Errorf(wire.CodeUnsupported, "pool command", "no configured command")
	}
	report, e := leaseOnce(ctx, repo, actor, choice, poolClock())
	if e != nil || report.Kind != "Transaction" {
		return report, e
	}
	_, state, e = poolSnapshot(ctx, repo)
	if e != nil {
		return report, e
	}
	for _, entry := range state.Entries {
		if entry.MemberID == choice.Lease.Member {
			en = entry
		}
	}
	raw, e := runPool(ctx, repo, choice, en, def)
	if e != nil {
		return report, e
	}
	return observePool(context.WithoutCancel(ctx), repo, actor, choice, en, raw)
}

func healthClaim(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, initial *Report) (*Report, error) {
	return healthClaimWith(ctx, repo, actor, choice, initial, func(c LeaseChoice) (*Report, error) { return leaseOnce(ctx, repo, actor, c, poolClock()) })
}
func healthClaimWith(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, initial *Report, execute func(LeaseChoice) (*Report, error)) (*Report, error) {
	report := initial
	for round := 0; round < intent.MaxPoolMembers; round++ {
		p, state, e := poolSnapshot(ctx, repo)
		if e != nil {
			return report, e
		}
		pool := p.Pool(choice.Lease.Pool)
		if pool == nil {
			return report, nil
		}
		busy := map[string]bool{}
		for _, en := range state.Entries {
			busy[en.MemberID] = true
		}
		member := ""
		for _, m := range pool.Members {
			if !busy[m] && (pool.ReservedFor[m] == "" || pool.ReservedFor[m] == choice.Lease.Stage) {
				member = m
				break
			}
		}
		if member == "" {
			return report, nil
		}
		def := pool.MemberConfig[member].Health
		if def == nil {
			return execute(choice)
		}
		prep := choice
		prep.RequestID = poolChildID(choice.RequestID, member)
		prep.Lease = transaction.LeaseRequest{Verb: transaction.LeasePoolPrepare, Pool: pool.ID, Member: member, Holder: choice.Lease.Holder, Stage: choice.Lease.Stage, Evidence: string(transaction.PoolClaimBinding(&choice.Lease, state.QueueID))}
		prepared, e := leaseOnce(ctx, repo, actor, prep, poolClock())
		if e != nil {
			return prepared, e
		}
		if prepared.Kind != "Transaction" {
			report.Detail = "health preparation unavailable: " + prepared.Detail + "; replay never executes it again"
			return report, nil
		}
		_, state, e = poolSnapshot(ctx, repo)
		if e != nil {
			return prepared, e
		}
		var en snapshot.PoolEntry
		for _, x := range state.Entries {
			if x.MemberID == member {
				en = x
			}
		}
		raw, e := runPool(ctx, repo, choice, en, def)
		if e != nil {
			return prepared, e
		}
		observation, e := snapshot.DecodePoolObservation(raw)
		if e != nil {
			return prepared, e
		}
		if observation.Passed {
			choice.pool = transaction.PoolFacts{AllocationID: en.AllocationID, Observation: raw}
			claimed, e := execute(choice)
			if e == nil && claimed.Outcome.Outcome == mutation.OutcomeCompleted {
				return claimed, nil
			}
			_, cleanupErr := observePool(context.WithoutCancel(ctx), repo, actor, prep, en, raw)
			if e != nil {
				return claimed, e
			}
			if cleanupErr != nil {
				return claimed, cleanupErr
			}
			return claimed, nil
		}
		if _, e = observePool(context.WithoutCancel(ctx), repo, actor, prep, en, raw); e != nil {
			return prepared, e
		}
		if ctx.Err() != nil {
			report.Kind = "Refused"
			report.Outcome.Outcome = mutation.OutcomeBlocked
			report.Outcome.Codes = []string{wire.CodeQuiescenceUnproved}
			report.Detail = "health interrupted; bound observation recorded and member quarantined"
			return report, nil
		}
		report, e = execute(choice)
		if e != nil || !report.Outcome.HasCode(wire.CodeQuiescenceUnproved) {
			return report, e
		}
	}
	return report, wire.Errorf(wire.CodeLimitExceeded, "pool", "member probe bound")
}
