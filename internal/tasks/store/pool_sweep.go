package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PoolSweepChoice struct {
	QueueID, RequestID, Root, Member, Allocation string
	// ExpectedDefinition is an immutable adapter observation, checked only on first admission.
	ExpectedDefinition wire.Digest
	TimeoutSeconds     wire.Count
}
type PoolSweepReport struct {
	Report   *Report
	Evidence wire.Digest
	Result   []byte
	Pending  bool
	// Unretryable reports that a fresh sweep committed its owner and then
	// failed: phase programs may have run, and a same-request retry only
	// reconciles committed receipts, so it never recovers a lost observation
	// and the result is never retryable (CAL-V0-078).
	Unretryable bool
}

// Response fault injection is private to package tests; it runs after the real writer.
type sweepResponseKey struct{}

type sweepMember struct {
	en     snapshot.PoolEntry
	config intent.MemberConfig
	env    sweepEnvironment
}

// The recorded cleanup allowance is the enforced post-wait group cleanup bound.
const poolCleanupAllowance = 5 * time.Second

var poolCleanupAllowanceMillis = wire.CountOf(poolCleanupAllowance.Milliseconds())

func sweepChildID(id string, en snapshot.PoolEntry) string {
	return poolChildID(id, string(en.AllocationID)+":"+string(en.Sweep.Attempt)+":"+en.Sweep.Phase)
}
func sweepWrite(ctx context.Context, repo *intent.Repository, actor mutation.Binding, c LeaseChoice, f transaction.PoolFacts, observe claimObserver) (*Report, error) {
	req := transaction.Request{Operation: transaction.Lease, QueueID: c.QueueID, RequestID: c.RequestID, Actor: actor, Lease: &c.Lease}
	if observe == nil {
		observe = func(*journal.Result, *transaction.Input) (transaction.LeaseFacts, error) {
			return transaction.LeaseFacts{Pool: f}, nil
		}
	}
	report, _, e := administrativeWriteWith(WithClock(ctx, poolClock), repo, req, poolClock(), nil, observe)
	if hook, ok := ctx.Value(sweepResponseKey{}).(func(LeaseChoice, *Report, error) error); ok {
		e = hook(c, report, e)
	}
	return report, e
}
func sweepCompleted(r *Report, e error) error {
	if e != nil {
		return e
	}
	if r == nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		return wire.Errorf(wire.CodeQuiescenceUnproved, "sweep", "transition refused: %v", r)
	}
	return nil
}

// PoolSweep prepares all selected allocation owners once, then runs outside every writer lock.
// Replaying the original request never reads mutable source/configuration/environment or executes.
func PoolSweep(ctx context.Context, repo *intent.Repository, actor mutation.Binding, c PoolSweepChoice) (swept *PoolSweepReport, sweptErr error) {
	committed := false
	defer func() {
		if committed && sweptErr != nil {
			if swept == nil {
				swept = &PoolSweepReport{}
			}
			swept.Unretryable = true
		}
	}()
	choice := LeaseChoice{QueueID: c.QueueID, RequestID: c.RequestID, Root: c.Root, Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolSweep, Member: c.Member, Allocation: c.Allocation, SweepSeconds: c.TimeoutSeconds}}
	request := transaction.Request{Operation: transaction.Lease, QueueID: c.QueueID, RequestID: c.RequestID, Actor: actor, Lease: &choice.Lease}
	if c.ExpectedDefinition != "" {
		if _, err := wire.ParseDigest("expectedDefinition", string(c.ExpectedDefinition)); err != nil {
			return &PoolSweepReport{}, err
		}
		if c.Member == "" || c.Allocation == "" {
			return &PoolSweepReport{}, wire.Errorf(wire.CodeMalformed, "sweep", "expected definition requires member and allocation")
		}
	}
	owner, e := transaction.Digest(request)
	if e != nil {
		return &PoolSweepReport{}, e
	}
	run, cancel := context.WithTimeout(ctx, time.Duration(c.TimeoutSeconds.Int())*time.Second)
	defer cancel()
	// Normal recording shares the invocation deadline. After execution cleanup,
	// every cancellation/finalization write shares ONE detached writer allowance.
	var finalContext context.Context
	var stopFinal context.CancelFunc
	defer func() {
		if stopFinal != nil {
			stopFinal()
		}
	}()
	writeContext := func() context.Context {
		if run.Err() == nil {
			return run
		}
		if finalContext == nil {
			finalContext, stopFinal = context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		}
		return finalContext
	}
	members := []sweepMember{}
	observer := func(proof *journal.Result, in *transaction.Input) (transaction.LeaseFacts, error) {
		members = nil
		p, e := intent.DecodePolicy(in.Policy)
		if e != nil {
			return transaction.LeaseFacts{}, e
		}
		state, e := snapshot.DecodePools(in.Pools)
		if e != nil {
			return transaction.LeaseFacts{}, e
		}
		if c.Allocation != "" {
			matched := false
			for _, en := range state.Entries {
				if en.MemberID != c.Member {
					continue
				}
				matched = string(en.AllocationID) == c.Allocation && (c.ExpectedDefinition == "" || (en.DefinitionSha256 == c.ExpectedDefinition && p.MemberDefinition(en.PoolID, en.MemberID) == c.ExpectedDefinition))
			}
			if !matched {
				return transaction.LeaseFacts{}, wire.Errorf(wire.CodeFenced, "sweep", "recorded allocation or definition changed before admission")
			}
		}
		revision, tree, e := sweepSource(run, c.Root)
		if e != nil {
			return transaction.LeaseFacts{}, e
		}
		probe, cancel := context.WithTimeout(run, time.Second)
		defer cancel()
		identity, e := poolRunnerIdentityContext(probe, os.Getpid())
		if e != nil || identity == "" {
			return transaction.LeaseFacts{}, wire.Errorf(wire.CodeQuiescenceUnproved, "sweep", "runner identity unavailable")
		}
		f := transaction.PoolFacts{RunnerPID: wire.CountOf(int64(os.Getpid())), RunnerStarted: identity, Revision: revision, Tree: tree}
		for _, pool := range p.Pools {
			for _, member := range pool.Members {
				if c.Member != "" && member != c.Member {
					continue
				}
				config := pool.MemberConfig[member]
				if config.SafeReuse == nil {
					continue
				}
				for _, en := range state.Entries {
					if en.MemberID == member && en.Sweep != nil {
						return transaction.LeaseFacts{}, wire.Errorf(wire.CodeResourceCollision, "sweep", "member already owned")
					}
					if en.MemberID != member || en.State != "QUARANTINED" {
						continue
					}
					if e = sweepConfig(run, c.Root, en.ConfigRef); e != nil {
						return transaction.LeaseFacts{}, e
					}
					keys := append([]string{}, config.SafeReuse.Reset.Env...)
					if config.Cleanup != nil {
						keys = append(keys, config.Cleanup.Env...)
					}
					env, e := loadSweepEnvironment(c.Root, keys, config.SafeReuse.EnvFile)
					if e != nil {
						return transaction.LeaseFacts{}, e
					}
					members = append(members, sweepMember{en, config, env})
					f.SweepSelections = append(f.SweepSelections, transaction.PoolSweepSelection{Pool: pool.ID, Member: member, Allocation: en.AllocationID, Definition: en.DefinitionSha256})
				}
			}
		}
		if e = transaction.CheckPoolSweepResultCapacity(owner, f.SweepSelections); e != nil {
			return transaction.LeaseFacts{}, e
		}
		return transaction.LeaseFacts{Pool: f}, nil
	}
	initial, e := sweepWrite(run, repo, actor, choice, transaction.PoolFacts{}, observer)
	// An ALL barrier forbids fresh preparation, but cannot erase an original
	// request. Only an audited exact historical match can enter reconciliation.
	if e == nil && initial != nil && initial.Outcome.HasCode(wire.CodePaused) {
		probe, err := snapshot.Probe(repo.StateDir)
		if err != nil {
			return &PoolSweepReport{Report: initial}, err
		}
		if probe.Head.QueueID.Raw != c.QueueID {
			return &PoolSweepReport{Report: initial}, wire.Errorf(wire.CodeOutOfScope, "queue", "sweep queue differs")
		}
		index := journal.RequestIndex{Reader: journalReader(repo, probe.Head)}
		entry, found, err := index.Lookup(c.RequestID)
		if err != nil {
			return &PoolSweepReport{Report: initial}, err
		}
		if found {
			setLeaseReport(initial, replayResult(request, entry))
		}
	}
	out := &PoolSweepReport{Report: initial}
	if e != nil || initial == nil || initial.Outcome.Outcome != mutation.OutcomeCompleted {
		return out, e
	}
	finishID := poolChildID(c.RequestID, "finish")
	if initial.Kind == "Replay" {
		return reconcileSweep(run, repo, actor, c, owner, initial)
	}
	committed = true
	results := []wire.Value{}
	for _, member := range members {
		en := member.en
		var last []byte
		success := false
		attemptCtx, stopAttempt := context.WithTimeout(run, time.Duration(member.config.SafeReuse.TimeoutSeconds.Int())*time.Second)
		currentAttempt := wire.Count("1")
		for round := 0; round < 7; round++ {
			_, state, e := poolSnapshot(writeContext(), repo)
			if e != nil {
				stopAttempt()
				return out, e
			}
			found := false
			for _, entry := range state.Entries {
				if entry.MemberID == en.MemberID && entry.AllocationID == en.AllocationID {
					en = entry
					found = true
				}
			}
			if !found || en.Sweep == nil {
				break
			}
			if en.State == "QUARANTINED" {
				break
			}
			if en.Sweep.Attempt != currentAttempt {
				stopAttempt()
				attemptCtx, stopAttempt = context.WithTimeout(run, time.Duration(member.config.SafeReuse.TimeoutSeconds.Int())*time.Second)
				currentAttempt = en.Sweep.Attempt
			}
			if en.Sweep.Phase == "confirm" {
				conf := LeaseChoice{QueueID: c.QueueID, RequestID: sweepChildID(c.RequestID, en), Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolSafe, Member: en.MemberID, Allocation: string(en.AllocationID), Evidence: string(owner), Reason: "operator-declared reset and verification passed"}}
				rep, e := sweepWrite(writeContext(), repo, actor, conf, transaction.PoolFacts{Observation: last}, nil)
				if e = sweepCompleted(rep, e); e != nil {
					stopAttempt()
					return reconcileSweep(writeContext(), repo, actor, c, owner, initial)
				}
				success = true
				break
			}
			phase := en.Sweep.Phase
			def := &member.config.SafeReuse.Reset
			if phase == "cleanup" {
				def = member.config.Cleanup
			}
			if phase == "verify" {
				def = &member.config.SafeReuse.Verify
			}
			env, envDigest := member.env.phase(def.Env)
			// An ALL barrier forbids launching any further phase command; the
			// owned sweep only publishes its terminal observation.
			probe, e := snapshot.Probe(repo.StateDir)
			if e != nil {
				stopAttempt()
				return out, e
			}
			result := poolCommandResult{Class: "INTERRUPTED", Clean: true}
			if probe.Barrier == nil || probe.Barrier.Scope != "ALL" {
				revision, tree, sourceErr := sweepSource(attemptCtx, c.Root)
				result.Class = sweepSourceClass(sourceErr)
				if sourceErr == nil && revision == en.CommandRevision && tree == en.Sweep.Tree {
					result = executePoolCaptured(attemptCtx, def, c.Root, env)
					after, afterTree, err := sweepSource(attemptCtx, c.Root)
					if err != nil || after != revision || afterTree != tree {
						result.Class = sweepSourceClass(err)
					}
				}
			}
			if attemptCtx.Err() != nil && result.Clean && result.Class != "OUTPUT_LIMIT" {
				result.Class = "INTERRUPTED"
				if errors.Is(attemptCtx.Err(), context.DeadlineExceeded) {
					result.Class = "TIMEOUT"
				}
			}

			passed := result.Clean && result.Exit != nil && result.Exit.Int() == 0 && (result.Class == "EXIT_ZERO")
			if phase == "verify" {
				passed = result.Clean && result.Exit != nil && result.Exit.Int() == member.config.SafeReuse.ExpectExit.Int() && (result.Class == "EXIT_ZERO" || result.Class == "EXIT_NONZERO") && strings.Contains(string(result.Stdout), member.config.SafeReuse.ExpectStdout)
				if !passed && result.Class == "EXIT_ZERO" {
					result.Class = "STDOUT_MISMATCH"
				}
			}
			log := snapshot.EncodePoolSweepLog(result.Stdout, result.Stderr)
			o := snapshot.PoolSweepObservation{AllocationID: en.AllocationID, DefinitionSha256: en.DefinitionSha256, Owner: owner, Previous: en.Sweep.Previous, Phase: phase, Attempt: en.Sweep.Attempt, Revision: en.CommandRevision, Tree: en.Sweep.Tree, Class: result.Class, Exit: result.Exit, Passed: passed, GroupClean: result.Clean, Log: wire.Sum(log), Environment: envDigest, EnvFile: member.env.file}
			if result.Timing.Deadline == "" {
				at := time.Now()
				deadline, ok := attemptCtx.Deadline()
				if !ok {
					deadline = at
				}
				result.Timing = snapshot.PoolSweepTiming{StartedAt: wire.SizeOf(uint64(at.UnixNano())), Deadline: wire.SizeOf(uint64(deadline.UnixNano())), WaitReturnedAt: wire.SizeOf(uint64(at.UnixNano())), CleanupEndedAt: wire.SizeOf(uint64(at.UnixNano())), ExecutionMillis: "0", CleanupMillis: "0", CleanupAllowanceMillis: poolCleanupAllowanceMillis}
			}
			o.Stdout = wire.Sum(result.Stdout)
			o.Stderr = wire.Sum(result.Stderr)
			o.Signal = result.Signal
			o.Timing = result.Timing
			last = o.Encode()
			rep, e := sweepWrite(writeContext(), repo, actor, LeaseChoice{QueueID: c.QueueID, RequestID: sweepChildID(c.RequestID, en), Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolObserve, Member: en.MemberID, Allocation: string(en.AllocationID), Evidence: string(owner)}}, transaction.PoolFacts{Observation: last, SweepLog: log}, nil)
			if e = sweepCompleted(rep, e); e != nil {
				stopAttempt()
				return out, e
			}
		}
		stopAttempt()
		results = append(results, wire.ObjectValue(wire.NewObject().Set("member", wire.String(en.MemberID)).Set("allocation", wire.String(string(en.AllocationID))).Set("free", wire.Bool(success)).Set("observation", wire.String(string(wire.Sum(last))))))
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-pool-sweep-result/0")).Set("owner", wire.String(string(owner))).Set("members", wire.Array(results...))))
	final, e := sweepWrite(writeContext(), repo, actor, LeaseChoice{QueueID: c.QueueID, RequestID: finishID, Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolSweepFinish, Evidence: string(owner)}}, transaction.PoolFacts{SweepResult: raw}, nil)
	if e = sweepCompleted(final, e); e != nil {
		return reconcileSweep(writeContext(), repo, actor, c, owner, initial)
	}
	out.Report = final
	out.Result = raw
	out.Evidence = wire.Sum(raw)
	return out, nil
}

// Reconcile only immutable child receipts from the original selection. Missing
// phase evidence means pending; no reset/verify or fresh confirmation is run.
// A successor in today's projection is never interpreted as the old allocation.
func reconcileSweep(ctx context.Context, repo *intent.Repository, actor mutation.Binding, c PoolSweepChoice, owner wire.Digest, initial *Report) (*PoolSweepReport, error) {
	out := &PoolSweepReport{Report: initial, Pending: true}
	finish := LeaseChoice{QueueID: c.QueueID, RequestID: poolChildID(c.RequestID, "finish"), Lease: transaction.LeaseRequest{Verb: transaction.LeasePoolSweepFinish, Evidence: string(owner)}}
	final, err := sweepWrite(ctx, repo, actor, finish, transaction.PoolFacts{}, nil)
	if err != nil {
		return out, err
	}
	if final != nil && final.Kind == "Replay" && final.Outcome.ReceiptSeq != nil {
		return sweepHistoricalResult(repo, final)
	}
	if initial == nil || initial.Outcome.ReceiptSeq == nil {
		return out, wire.Errorf(wire.CodeMissingEvidence, "sweep", "initial receipt absent")
	}
	original, err := sweepReceiptPools(repo, initial.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return out, err
	}
	rows := []wire.Value{}
	for _, selected := range original.Entries {
		if selected.Sweep == nil || selected.Sweep.RequestSha256 != owner {
			continue
		}
		en := selected
		var last wire.Digest
		complete, free := false, false
		for round := 0; round < 7; round++ {
			if en.Sweep == nil {
				return out, wire.Errorf(wire.CodeJournalForked, "sweep", "phase owner absent")
			}
			confirm := en.Sweep.Phase == "confirm"
			l := transaction.LeaseRequest{Verb: transaction.LeasePoolObserve, Member: en.MemberID, Allocation: string(en.AllocationID), Evidence: string(owner)}
			if confirm {
				l.Verb = transaction.LeasePoolSafe
				l.Reason = "operator-declared reset and verification passed"
			}
			lookup := LeaseChoice{QueueID: c.QueueID, RequestID: sweepChildID(c.RequestID, en), Lease: l}
			child, e := sweepWrite(ctx, repo, actor, lookup, transaction.PoolFacts{}, nil)
			if e != nil {
				return out, e
			}
			if !sweepReplayed(child) {
				// The phase had not committed. The original stays pending while it
				// still owns the member; once explicit proved-orphan recovery (or
				// a terminal write) has released that owner it ends quarantined,
				// never free, citing its last committed witness or the owner digest
				// when none exists.
				held, e := sweepOwnerHeld(ctx, repo, selected, owner)
				if e != nil || held {
					return out, e
				}
				// The lookup and the owner snapshot are separate reads, so a live
				// runner may have committed this phase between them. Release is
				// monotonic (observe and safe both require the owner), so one
				// repeated lookup decides whether this phase committed.
				if child, e = sweepWrite(ctx, repo, actor, lookup, transaction.PoolFacts{}, nil); e != nil {
					return out, e
				}
				if !sweepReplayed(child) {
					if last == "" {
						last = owner
					}
					complete = true
					break
				}
			}
			after, e := sweepReceiptPools(repo, child.Outcome.ReceiptSeq.Uint64())
			if e != nil {
				return out, e
			}
			var next *snapshot.PoolEntry
			for _, candidate := range after.Entries {
				if candidate.MemberID == selected.MemberID && candidate.AllocationID == selected.AllocationID {
					v := candidate
					next = &v
					break
				}
			}
			if confirm {
				if next != nil || en.Sweep.Previous == nil {
					return out, wire.Errorf(wire.CodeJournalForked, "sweep", "confirmation receipt did not remove original allocation")
				}
				last = *en.Sweep.Previous
				complete = true
				free = true
				break
			}
			if next == nil || next.ObservationSha256 == nil {
				return out, wire.Errorf(wire.CodeJournalForked, "sweep", "phase receipt allocation or witness absent")
			}
			last = *next.ObservationSha256
			if next.State == "QUARANTINED" {
				complete = true
				break
			}
			en = *next
		}
		if !complete {
			return out, nil
		}
		rows = append(rows, wire.ObjectValue(wire.NewObject().Set("member", wire.String(selected.MemberID)).Set("allocation", wire.String(string(selected.AllocationID))).Set("free", wire.Bool(free)).Set("observation", wire.String(string(last)))))
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String("taskman-pool-sweep-result/0")).Set("owner", wire.String(string(owner))).Set("members", wire.Array(rows...))))
	final, err = sweepWrite(ctx, repo, actor, finish, transaction.PoolFacts{SweepResult: raw}, nil)
	if err != nil {
		// One bounded replay resolves a lost finish response; no publication loop.
		final, err = sweepWrite(ctx, repo, actor, finish, transaction.PoolFacts{}, nil)
	}
	if err = sweepCompleted(final, err); err != nil {
		return out, err
	}
	return sweepHistoricalResult(repo, final)
}
func sweepReplayed(r *Report) bool {
	return r != nil && r.Kind == "Replay" && r.Outcome.ReceiptSeq != nil
}
func sweepOwnerHeld(ctx context.Context, repo *intent.Repository, selected snapshot.PoolEntry, owner wire.Digest) (bool, error) {
	_, state, e := poolSnapshot(ctx, repo)
	if e != nil {
		return true, e
	}
	for _, en := range state.Entries {
		if en.MemberID == selected.MemberID && en.AllocationID == selected.AllocationID && en.Sweep != nil && en.Sweep.RequestSha256 == owner {
			return true, nil
		}
	}
	return false, nil
}
func sweepHistoricalResult(repo *intent.Repository, final *Report) (*PoolSweepReport, error) {
	out := &PoolSweepReport{Report: final, Pending: true}
	if final == nil || final.Outcome.ReceiptSeq == nil {
		return out, wire.Errorf(wire.CodeMissingEvidence, "sweep", "finish receipt absent")
	}
	raw, err := sweepReceiptResult(repo, final.Outcome.ReceiptSeq.Uint64())
	if err != nil {
		return out, err
	}
	out.Pending = false
	out.Result = raw
	out.Evidence = wire.Sum(raw)
	return out, nil
}
func sweepReceiptPools(repo *intent.Repository, seq uint64) (*snapshot.PoolState, error) {
	rc, err := readReceipt(repo, seq)
	if err != nil {
		return nil, err
	}
	for _, post := range rc.Post {
		if post.Path == "pools.json" {
			var raw []byte
			if post.Record != nil {
				raw = wire.EncodeFile(*post.Record)
			} else if post.BlobSha256 != nil {
				raw, err = intent.ReadFile(filepath.Join(repo.StateDir, "evidence", string(*post.BlobSha256)), snapshot.MaxPoolStateBytes)
				if err != nil {
					return nil, err
				}
			}
			if post.Sha256 == nil || wire.Sum(raw) != *post.Sha256 {
				return nil, wire.Errorf(wire.CodeJournalForked, "sweep", "receipt pool digest differs")
			}
			return snapshot.DecodePools(raw)
		}
	}
	return nil, wire.Errorf(wire.CodeMissingEvidence, "sweep", "receipt pool post absent")
}

func sweepReceiptResult(repo *intent.Repository, seq uint64) ([]byte, error) {
	rc, e := readReceipt(repo, seq)
	if e != nil {
		return nil, e
	}
	for _, post := range rc.Post {
		if strings.HasPrefix(post.Path, "evidence/") {
			raw, e := intent.ReadFile(filepath.Join(repo.StateDir, post.Path), 65536)
			if e != nil {
				return nil, e
			}
			if post.Sha256 == nil || wire.Sum(raw) != *post.Sha256 {
				return nil, fmt.Errorf("sweep receipt evidence differs")
			}
			return raw, nil
		}
	}
	return nil, wire.Errorf(wire.CodeMissingEvidence, "sweep", "result evidence absent")
}

// Each new sweep Git observation shares the invocation deadline, uses sanitized
// local-only Git settings, bounded capture, and the same owned-group cleanup.
func sweepGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	git, e := exec.LookPath("git")
	if e != nil {
		return nil, e
	}
	argv := append([]string{git, "-c", "credential.helper=", "-c", "core.fsmonitor=false"}, args...)
	result := executePoolCaptured(ctx, &intent.PoolCommand{Argv: argv, TimeoutSeconds: "2"}, root, gitEnvironment())
	if !result.Clean || result.Class != "EXIT_ZERO" {
		e = wire.Errorf(wire.CodeUnsupported, "sweep git", "bounded observation failed: %s", result.Class)
		if result.Clean && (result.Class == "TIMEOUT" || result.Class == "INTERRUPTED") {
			return nil, sweepGitBounded{class: result.Class, err: e}
		}
		return nil, e
	}
	return result.Stdout, nil
}

// sweepGitBounded marks a Git source observation that hit its bound, so a slow
// host records TIMEOUT/INTERRUPTED rather than claiming the source changed.
type sweepGitBounded struct {
	class string
	err   error
}

func (e sweepGitBounded) Error() string { return e.err.Error() }
func (e sweepGitBounded) Unwrap() error { return e.err }
func sweepSourceClass(e error) string {
	var bounded sweepGitBounded
	if errors.As(e, &bounded) {
		return bounded.class
	}
	return "SOURCE_CHANGED"
}
func sweepSource(ctx context.Context, root string) (string, string, error) {
	raw, e := sweepGit(ctx, root, "rev-parse", "--verify", "--end-of-options", "HEAD^{commit}")
	if e != nil {
		return "", "", e
	}
	revision, e := wire.ParseOID("revision", strings.TrimSpace(string(raw)))
	if e != nil {
		return "", "", e
	}
	raw, e = sweepGit(ctx, root, "rev-parse", "--verify", "--end-of-options", revision+"^{tree}")
	if e != nil {
		return "", "", e
	}
	tree, e := wire.ParseOID("tree", strings.TrimSpace(string(raw)))
	if e != nil {
		return "", "", e
	}
	status, e := sweepGit(ctx, root, "status", "--porcelain", "--untracked-files=all", "--", ".", ":(exclude).taskman")
	if e != nil {
		return "", "", e
	}
	if len(status) != 0 {
		return "", "", wire.Errorf(wire.CodeDirtyWorktree, "sweep", "command source changed")
	}
	return revision, tree, nil
}
func sweepConfig(ctx context.Context, root string, ref *intent.ConfigRef) error {
	if ref == nil {
		return nil
	}
	raw, e := sweepGit(ctx, root, "ls-tree", "-z", ref.Revision, "--", ref.Path)
	if e != nil {
		return e
	}
	parts := strings.SplitN(strings.TrimSuffix(string(raw), "\x00"), "\t", 2)
	if len(parts) != 2 || parts[1] != ref.Path {
		return wire.Errorf(wire.CodeMissingEvidence, "configRef", "immutable file absent")
	}
	fields := strings.Fields(parts[0])
	if len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" || fields[2] != ref.Blob {
		return wire.Errorf(wire.CodeMissingEvidence, "configRef", "regular blob differs")
	}
	return nil
}
