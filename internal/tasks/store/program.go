package store

import (
	"context"
	"strings"
	"sync"

	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

var programWriter sync.Mutex

// ProgramTransition records the expected whole-inventory binding under the
// native writer lock. A competing owner cannot replace the observed program.
func ProgramTransition(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queue, id string, next snapshot.Program, output ...[]byte) (*Report, error) {
	programWriter.Lock()
	defer programWriter.Unlock()
	proof, e := readLeaseProof(ctx, repo)
	if e != nil {
		return nil, e
	}
	change := transaction.ProgramChange{Expected: wire.Sum(proof.Records["programs.json"].Raw), Next: next}
	if oldRaw := proof.Records["programs.json"].Raw; len(oldRaw) > 0 {
		state, e := snapshot.DecodePrograms(oldRaw)
		if e != nil {
			return nil, e
		}
		for _, old := range state.Entries {
			if old.ID == next.ID && (old.OwnerPID != next.OwnerPID || old.OwnerStarted != next.OwnerStarted) {
				identity, e := supervisor.ProcessIdentity(old.OwnerPID)
				if e != nil {
					return nil, e
				}
				if identity == old.OwnerStarted && !old.OwnerReleased {
					return nil, fmt.Errorf("program still has a live owner")
				}
				for path, record := range proof.Records {
					_ = path
					if record.Raw == nil {
						continue
					}
					if a, e := snapshot.DecodeAttempt(record.Raw); e == nil && a.Supervision != nil && a.Supervision.ProgramID == old.ID && a.Supervision.Worker {
						boot, err := supervisor.RecoveryBoot(old.EffectDirectory, old.Effect, supervisor.Boot{PID: old.LeaderPID, Started: old.LeaderStarted})
						if err != nil {
							return nil, fmt.Errorf("recovery boot unavailable: %w", err)
						}
						if !supervisor.Recover(boot) {
							return nil, fmt.Errorf("prior program worker quiescence remains uncertain")
						}
						change.PreviousWorkerClean = true
						change.Next.Phase = "FINISHED"
						change.Next.Quiescence = "PROVED"
					}
				}
				change.PreviousOwnerGone = true
			}
		}
	}
	if len(output) > 0 {
		change.Output = output[0]
	}
	raw, e := snapshot.EncodeProgramJSON(change)
	if e != nil {
		return nil, e
	}
	req := transaction.Request{Operation: transaction.Lease, QueueID: queue, RequestID: id, Actor: actor, Lease: &transaction.LeaseRequest{Verb: transaction.LeaseProgram, Evidence: string(wire.Sum(raw))}}
	facts := func(_ *journal.Result, _ *transaction.Input) (transaction.LeaseFacts, error) {
		f := transaction.LeaseFacts{Program: raw}
		f.GateResults = map[wire.Digest][]byte{}
		for path, rec := range proof.Records {
			if strings.HasPrefix(path, "evidence/") {
				f.GateResults[wire.Digest(strings.TrimPrefix(path, "evidence/"))] = rec.Raw
			}
		}
		return f, nil
	}
	report, _, e := administrativeWriteWith(WithClock(ctx, poolClock), repo, req, poolClock(), nil, facts)
	return report, e
}

func SupervisorTransition(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queue, id, attempt string, generation wire.Size, change transaction.SupervisorChange) (*Report, error) {
	proof, e := readLeaseProof(ctx, repo)
	if e != nil {
		return nil, e
	}
	change.Expected = wire.Sum(proof.Records["attempts/"+attempt+".json"].Raw)
	if change.Action == "HEARTBEAT" {
		a, ok := lockedAttempt(proof, attempt)
		if !ok || a.Lane == nil || a.Supervision == nil {
			return nil, fmt.Errorf("heartbeat lane absent")
		}
		owner, e := supervisor.ProcessIdentity(change.OwnerPID)
		if e != nil || owner != change.OwnerStarted {
			return nil, fmt.Errorf("heartbeat owner not live")
		}
		leader, e := supervisor.ProcessIdentity(int(a.Lane.LeaderPid.Uint64()))
		if e != nil || leader != a.Supervision.LeaderStarted {
			return nil, fmt.Errorf("heartbeat leader not live")
		}
	}
	raw, e := snapshot.EncodeProgramJSON(change)
	if e != nil {
		return nil, e
	}
	choice := LeaseChoice{QueueID: queue, RequestID: id, Root: repo.PrimaryWorktree, Lease: transaction.LeaseRequest{Verb: transaction.LeaseSupervisor, AttemptID: attempt, Generation: generation, Evidence: string(wire.Sum(raw))}}
	if change.Action == "DISPATCH" {
		choice.Root = change.Worktree
		a, ok := lockedAttempt(proof, attempt)
		if !ok {
			return nil, fmt.Errorf("dispatch attempt missing")
		}
		rec := claimedRecord(proof, a.TicketID.Raw)
		pool := change.Pool
		if rec != nil && rec.RequiresPool != "" {
			pool = rec.RequiresPool
		}
		choice.Lease.Pool = pool
		choice.Lease.Stage = change.Stage
		choice.Lease.Holder = a.Lease.Holder
	}
	execute := func(choice LeaseChoice) (*Report, error) {
		req := transaction.Request{Operation: transaction.Lease, QueueID: queue, RequestID: id, Actor: actor, Lease: &choice.Lease}
		facts := func(locked *journal.Result, _ *transaction.Input) (transaction.LeaseFacts, error) {
			results, e := attemptGateResults(repo, locked, attempt)
			if e != nil {
				return transaction.LeaseFacts{}, e
			}
			f := transaction.LeaseFacts{Program: raw, GateResults: results, Pool: choice.pool}
			if choice.Lease.Pool != "" {
				policy, e := intent.DecodePolicy(locked.Records["intent/policy.json"].Raw)
				if e != nil {
					return f, e
				}
				if pool := policy.Pool(choice.Lease.Pool); pool != nil {
					for _, m := range pool.Members {
						if e = poolConfig(leaseRoot(repo, choice), pool.MemberConfig[m].ConfigRef); e != nil {
							return f, e
						}
					}
				}
			}
			if len(choice.pool.Observation) > 0 {
				o, e := snapshot.DecodePoolObservation(choice.pool.Observation)
				if e != nil {
					return f, e
				}
				rev, tree, e := poolSource(leaseRoot(repo, choice))
				if e != nil {
					return f, e
				}
				if rev != o.Revision || tree != o.Tree {
					return f, fmt.Errorf("stage health source changed before allocation")
				}
			}
			return f, nil
		}
		r, _, e := administrativeWriteWith(WithClock(ctx, poolClock), repo, req, poolClock(), nil, facts)
		return r, e
	}
	report, e := execute(choice)
	if e == nil && change.Action == "DISPATCH" && choice.Lease.Pool != "" && report.Outcome.HasCode(wire.CodeQuiescenceUnproved) {
		return healthClaimWith(ctx, repo, actor, choice, report, execute)
	}
	return report, e
}
