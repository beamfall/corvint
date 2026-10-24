package store

import (
	"context"
	"strings"
	"sync"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// programWriters holds one in-process program writer mutex per store root
// (the canonical StateDir), so transitions on one store serialize while
// independent stores in one process proceed concurrently (V1-1108). An entry
// counts its holder and waiters and is removed when the count reaches zero,
// so the map holds only stores with a transition in flight and one store
// never has two mutexes.
var programWriters = struct {
	sync.Mutex
	byRoot map[string]*programWriterEntry
}{byRoot: map[string]*programWriterEntry{}}

type programWriterEntry struct {
	sync.Mutex
	refs int
}

// lockProgramWriter locks the store's program writer and returns its unlock.
func lockProgramWriter(repo *intent.Repository) func() {
	root := repo.StateDir
	programWriters.Lock()
	e := programWriters.byRoot[root]
	if e == nil {
		e = &programWriterEntry{}
		programWriters.byRoot[root] = e
	}
	e.refs++
	programWriters.Unlock()
	e.Lock()
	return func() {
		e.Unlock()
		programWriters.Lock()
		if e.refs--; e.refs == 0 {
			delete(programWriters.byRoot, root)
		}
		programWriters.Unlock()
	}
}

// recoveryBootRefusal codes a RecoveryBoot failure. A retryable operational
// condition, such as descriptor exhaustion while opening the boot capsule,
// keeps its code; missing, invalid or mismatched boot evidence leaves the
// prior worker's quiescence unproved (CAL-V0-037).
func recoveryBootRefusal(err error) error {
	if wire.RetryOf(wire.CodeOf(err)).Retryable {
		return wire.Annotatef(err, "program", "recovery boot unavailable: %v", err)
	}
	return wire.Errorf(wire.CodeQuiescenceUnproved, "program", "recovery boot unavailable: %v", err)
}

// ProgramTransition records the expected whole-inventory binding under the
// native writer lock. A competing owner cannot replace the observed program.
func ProgramTransition(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queue, id string, next snapshot.Program, output ...[]byte) (*Report, error) {
	return programTransition(ctx, repo, actor, queue, id, next, false, false, output...)
}

// programTransition is ProgramTransition that, when fenced, refuses while the
// record it replaces carries a pending control. The check reads the same
// programs.json revision the write binds as Expected, so a control recorded
// before the write refuses it and one recorded after it is a later revision
// (CAL-V0-089). When keepControl is set, as for a phase write by the
// program's own workflow or runner, the written control is the one that same
// programs.json revision records, never the caller's earlier read, so a DRAIN
// or CANCEL recorded meanwhile is kept, and one recorded after the read moves
// the inventory and refuses the write (ATR-V0-240, V1-0831).
func programTransition(ctx context.Context, repo *intent.Repository, actor mutation.Binding, queue, id string, next snapshot.Program, fenced, keepControl bool, output ...[]byte) (*Report, error) {
	defer lockProgramWriter(repo)()
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
			if fenced && old.ID == next.ID && old.Control != "" {
				return nil, wire.Errorf(wire.CodeFenced, "program", "program control %s is pending", old.Control)
			}
			if keepControl && old.ID == next.ID {
				change.Next.Control = old.Control
			}
			// The same owner, its btime read one second apart, is not a
			// change of owner (PSR-V0-227).
			if old.ID == next.ID && (old.OwnerPID != next.OwnerPID || recordedMatch(next.OwnerStarted, old.OwnerStarted) != supervisor.IdentitySame) {
				identity, e := recordedProcessIdentity(old.OwnerPID)
				if e != nil {
					return nil, e
				}
				if e := ownerNotLive(identity, old); e != nil {
					return nil, e
				}
				// A dead owner's removal child may outlive it (CAL-V0-368).
				if e := removerStopped(old); e != nil {
					return nil, e
				}
				for path, record := range proof.Records {
					_ = path
					if record.Raw == nil {
						continue
					}
					if a, e := snapshot.DecodeAttempt(record.Raw); e == nil && a.Supervision != nil && a.Supervision.ProgramID == old.ID && a.Supervision.Worker {
						boot, err := supervisor.RecoveryBoot(old.EffectDirectory, old.Effect, supervisor.Boot{PID: old.LeaderPID, Started: old.LeaderStarted})
						if err != nil {
							return nil, recoveryBootRefusal(err)
						}
						if !supervisor.RecoverHost(old.EffectDirectory, boot) {
							return nil, wire.Errorf(wire.CodeQuiescenceUnproved, "program", "prior program worker quiescence remains uncertain")
						}
						change.PreviousWorkerClean = true
						change.Next.Phase = "FINISHED"
						change.Next.Quiescence = "PROVED"
						// The recovered stage's host output was lost with
						// the owner, so a stop from STOPPING records its
						// usage unobserved (CAL-V0-074, V1-1063).
						if old.Phase == "STOPPING" && change.Next.ResultClass != "NO_EXEC" {
							change.Next.UsageKnown = false
						}
					}
				}
				// An owner that died between dispatch and FINISHED after the
				// attempt recorded a proved stop leaves nothing to recover: the
				// replacement settles the program FINISHED. A stage stopped
				// from STOPPING lost its host output, so its usage is
				// unobserved (CAL-V0-210).
				if !change.PreviousWorkerClean && (old.Phase == "SPAWNING" || old.Phase == "STOPPING") {
					if rec, ok := proof.Records["attempts/"+old.CurrentAttempt+".json"]; ok && rec.Raw != nil {
						if a, e := snapshot.DecodeAttempt(rec.Raw); e == nil && transaction.BoundStoppedAttempt(a, old) {
							change.Next.Phase = "FINISHED"
							change.Next.Quiescence = "PROVED"
							if old.Phase == "STOPPING" && change.Next.ResultClass != "NO_EXEC" {
								change.Next.UsageKnown = false
							}
						}
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
			return nil, wire.Errorf(wire.CodeFenced, "supervisor transition", "heartbeat lane absent")
		}
		owner, e := recordedProcessIdentity(change.OwnerPID)
		if e != nil || recordedMatch(owner, change.OwnerStarted) != supervisor.IdentitySame {
			return nil, wire.Errorf(wire.CodeSupervisorLost, "supervisor transition", "heartbeat owner not live")
		}
		leader, e := recordedProcessIdentity(int(a.Lane.LeaderPid.Uint64()))
		if e != nil || recordedMatch(leader, a.Supervision.LeaderStarted) != supervisor.IdentitySame {
			return nil, wire.Errorf(wire.CodeSupervisorLost, "supervisor transition", "heartbeat leader not live")
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
			return nil, wire.Errorf(wire.CodeFenced, "supervisor transition", "dispatch attempt missing")
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
					return f, wire.Errorf(wire.CodeSnapshotMoved, "supervisor transition", "stage health source changed before allocation")
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
