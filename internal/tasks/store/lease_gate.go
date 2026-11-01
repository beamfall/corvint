package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateWaitDelay bounds how long a finished or killed gate may keep its
// output pipe open through a descendant outside its process group.
const gateWaitDelay = 5 * time.Second

// gateInterruptRecordBound bounds the store write that records an
// interrupted gate run's result (CAL-V0-319).
const gateInterruptRecordBound = 30 * time.Second

// gateRecordFault, when set by a package test, fails GateRun after the gate
// program ran and before its result is recorded; it is nil in production.
var gateRecordFault func() error

// gateObserveFault, when set by a package test, runs just before the
// post-run observation under the run's own context; it is nil in production.
var gateObserveFault func()

// gateRun is one finished gate run: its encoded result and captured output,
// or a reuse: the rebound record and the source result it names, with no
// output (CAL-V0-240).
type gateRun struct {
	record, output, source []byte
	// pending is a descendant PASS awaiting integration (TCD-V0-806).
	pending *GateReuse
}

// GateRun runs one policy command gate in worktree at the attempt's
// candidate tree, then records it (CAL-V0-016). The gate runs before the
// store lock is taken, so a long gate never holds it; the prepared model
// rechecks the attempt, tree and definition, then binds them under lock. A repeated
// request id reruns the gate before the replay is found. A stale or
// unsubmitted attempt is not run: the model answers it with no gate facts.
// Before reading, it settles the journal through guarded recovery, so a gate run
// retried after a crash recovers as any other writer does (CAL-V0-019).
func GateRun(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, worktree string, clock func() time.Time) (report *Report, err error) {
	// Once the program has started, every return reports it (CAL-V0-078).
	executed, interrupted := false, false
	var pending *GateReuse
	defer func() {
		if executed {
			if report == nil {
				report = &Report{}
			}
			report.Unretryable = true
			report.GateInterrupted = interrupted
		}
	}()
	redone, err := settleLease(ctx, repo)
	if err != nil {
		return &Report{}, err
	}
	a, policy, proof, err := unlockedAttemptProof(ctx, repo, choice.Lease.AttemptID)
	if err != nil {
		return &Report{}, err
	}
	admitted, err := gateAdmitted(ctx, repo, actor, choice)
	if err != nil {
		return &Report{}, err
	}
	if admitted && runnable(a, choice.Lease, clock()) {
		def, err := commandGate(policy, choice.Lease.Gate)
		if err != nil {
			return &Report{}, err
		}
		repos, err := attemptRepositories(proof, a)
		if err != nil {
			return &Report{}, err
		}
		observe, cancel := gateObservation(ctx, def.TimeoutSeconds.Int())
		err = atCandidate(observe, worktree, *a.CandidateTreeOid, repos)
		cancel()
		if err != nil {
			return &Report{}, err
		}
		argv, substituted, err := gateArgv(def, proof, a, worktree, repos)
		if err != nil {
			return &Report{}, err
		}
		run, reused, err := reusableRun(repo, leaseRoot(repo, choice), proof, policy, def, a, argv, substituted)
		pending = run.pending
		if err != nil {
			return &Report{}, err
		}
		if !reused {
			executed = true
			stopBeats := gateHeartbeat(ctx, repo, actor, choice, policy, clock)
			run, err = runGate(ctx, def, argv, substituted, policy, a, worktree, repos, clock)
			stopBeats()
			if err != nil {
				return &Report{}, err
			}
		}
		if err = ctx.Err(); err != nil {
			if reused {
				return &Report{}, err
			}
			// CAL-V0-319: the run's context ended while or after the gate
			// ran; the process group is already reaped. The run's result and
			// captured output are recorded under a bounded write instead of
			// being discarded.
			interrupted = true
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), gateInterruptRecordBound)
			defer cancel()
		}
		choice.gate = &run
		if gateRecordFault != nil {
			if err = gateRecordFault(); err != nil {
				return &Report{}, err
			}
		}
	}
	now, err := wire.ParseTimestamp("recordedAt", clock().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return &Report{}, err
	}
	report, err = Lease(ctx, repo, actor, choice, now)
	if report != nil {
		report.Redone = report.Redone || redone
		if report.GateReusedFrom == nil && pending != nil && err == nil {
			report.GateReusePending = pending
		}
		switch {
		case err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted:
		case report.Kind == "Transaction" && choice.gate != nil && choice.gate.source != nil:
			report.GateReusedFrom = gateReuseOf(choice.gate.source)
		case report.Kind == "Transaction" && choice.gate != nil:
			report.GateExecuted = &GateExecution{Record: choice.gate.record, Output: choice.gate.output}
			if def := policy.Gate(choice.Lease.Gate); def != nil {
				report.GateExecuted.FailurePattern = def.FailurePattern
			}
		case report.Kind == "Replay" && report.Outcome.ReceiptSeq != nil:
			report.GateReusedFrom = recordedReuse(repo, report.Outcome.ReceiptSeq.Uint64(), choice.Lease.AttemptID, choice.Lease.Gate)
		}
	}
	return report, err
}

// GateExecution is the encoded gate result record a gate run executed and
// committed in this call, and the output it captured, retained as evidence
// under the record's "output" digest (TCD-V0-165, TCD-V0-166).
type GateExecution struct {
	Record, Output []byte
	// FailurePattern is the gate's policy failurePattern, nil when it has
	// none (TCD-V0-801).
	FailurePattern *regexp.Regexp
}

// GateReuse names the earlier result a gate run reused: its digest and the
// attempt that executed it (CAL-V0-240).
// A CI-backed result names the retained CI run record and its run id
// instead, with no attempt (TCD-V0-155).
type GateReuse struct {
	Result    wire.Digest
	AttemptID string
	CIRunID   string
	// Tree, Gate and Branch are set only on Report.GateReusePending: the
	// pending result's tree, its gate and the protected intent branch it is
	// not yet on (TCD-V0-806).
	Tree, Gate, Branch string
}

func gateReuseOf(source []byte) *GateReuse {
	if ci, err := snapshot.DecodeCIRun(source); err == nil {
		return &GateReuse{Result: wire.Sum(source), CIRunID: string(ci.RunID)}
	}
	src, err := snapshot.DecodeGateResult(source)
	if err != nil {
		return nil
	}
	return &GateReuse{Result: wire.Sum(source), AttemptID: src.AttemptID}
}

// recordedReuse is the source of the result that GATE_RESULT receipt seq
// recorded for the attempt's gate, nil when that result was executed or
// cannot be read. A replayed gate run reports what the original receipt
// recorded, not what this run would have chosen.
func recordedReuse(repo *intent.Repository, seq uint64, attemptID, gate string) *GateReuse {
	rc, err := readReceipt(repo, seq)
	if err != nil || rc.Kind != "GATE_RESULT" {
		return nil
	}
	blob := ExternalReviewBlob(repo)
	for _, p := range rc.Post {
		if p.Path != "attempts/"+attemptID+".json" || p.Sha256 == nil {
			continue
		}
		var raw []byte
		switch {
		case p.Record != nil:
			raw = wire.EncodeFile(*p.Record)
		case p.BlobSha256 != nil:
			raw, _ = blob(*p.BlobSha256)
		}
		at, err := snapshot.DecodeAttempt(raw)
		if err != nil {
			return nil
		}
		for _, d := range at.GateResults {
			raw, ok := blob(wire.Digest(d))
			g, err := snapshot.DecodeGateResult(raw)
			if !ok || err != nil || g.GateID != gate {
				continue
			}
			if g.ReusedFrom == nil {
				return nil
			}
			if src, ok := blob(*g.ReusedFrom); ok {
				return gateReuseOf(src)
			}
			return nil
		}
	}
	return nil
}

// reusableRun finds an earlier result that satisfies this run of a
// reusable gate without executing it (CAL-V0-240): a PASSED, unreused
// result of the same gate that another attempt's record names, at the same
// candidate tree, under the current definition digest, whose inputs and
// environment digests equal the ones this run would record. The lowest
// qualifying digest is chosen, so the choice does not depend on read order.
// With no same-tree result, a qualifying result at another tree is chosen
// when its commit on the protected intent branch descends from a commit
// carrying the candidate tree (TCD-V0-151). A gate not declared reusable, or no qualifying result, runs as before; an
// unreadable or undecodable record is skipped, never trusted. The model
// rechecks the choice under the lock.
func reusableRun(repo *intent.Repository, root string, proof *journal.Result, policy *intent.Policy, def *intent.GateDefinition, a *snapshot.Attempt, argv []string, substituted bool) (gateRun, bool, error) {
	if !def.Reusable {
		return gateRun{}, false, nil
	}
	candidate := *a.CandidateTreeOid
	_, env, keyed := gateEnvironmentKey(def, argv)
	if !keyed {
		return gateRun{}, false, nil
	}
	definition := policy.GateDefinitionSha256(def.GateID)
	inputs := map[string]wire.Digest{}
	var best wire.Digest
	var source []byte
	var chosen *snapshot.GateResult
	// elsewhere holds qualifying results at other trees, a descendant
	// reuse's candidates when none is at the candidate (TCD-V0-151).
	elsewhere := map[wire.Digest][]byte{}
	for path, record := range proof.Records {
		if !strings.HasPrefix(path, "attempts/") || record.Sha256 == nil {
			continue
		}
		owner, err := snapshot.DecodeAttempt(record.Raw)
		if err != nil || owner.AttemptID == a.AttemptID || owner.CandidateTreeOid == nil {
			continue
		}
		tree := *owner.CandidateTreeOid
		if _, ok := inputs[tree]; !ok {
			inputs[tree] = gateInputs(tree, argv, substituted)
		}
		for _, d := range owner.GateResults {
			if tree == candidate && source != nil && wire.Digest(d) >= best {
				continue
			}
			raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", d), snapshot.MaxGateRecordBytes)
			if err != nil || wire.Sum(raw) != wire.Digest(d) {
				continue
			}
			g, err := snapshot.DecodeGateResult(raw)
			if err != nil || g.GateID != def.GateID || g.ReusedFrom != nil || g.State != "PASSED" || g.AttemptID != owner.AttemptID || g.CandidateTreeOid != tree || g.DefinitionSha256 != definition || g.InputsSha256 != inputs[tree] || g.EnvironmentSha256 != env {
				continue
			}
			if tree != candidate {
				elsewhere[wire.Digest(d)] = raw
				continue
			}
			best, source, chosen = wire.Digest(d), raw, g
		}
	}
	var pending *GateReuse
	if source == nil && len(elsewhere) > 0 {
		var err error
		if best, source, chosen, pending, err = descendantSource(root, intentBranch(proof), candidate, elsewhere); err != nil {
			return gateRun{}, false, err
		}
	}
	if source == nil {
		return gateRun{pending: pending}, false, nil
	}
	g := transaction.ReusedGateResult(chosen, best, a)
	record, err := g.Encode()
	return gateRun{record: record, source: source}, err == nil, err
}

// descendantSource is the lowest-digest result among results at other trees
// whose tree is on the protected intent branch as a descendant of candidate
// (TCD-V0-151), or none. With none, pending names the lowest-digest result
// refused only because its tree is not yet on that branch (TCD-V0-806).
func descendantSource(root, branch, candidate string, results map[wire.Digest][]byte) (wire.Digest, []byte, *snapshot.GateResult, *GateReuse, error) {
	digests := make([]wire.Digest, 0, len(results))
	for d := range results {
		digests = append(digests, d)
	}
	slices.Sort(digests)
	h, err := loadProtectedHistory(root, branch)
	if err != nil {
		return "", nil, nil, nil, err
	}
	var pending *GateReuse
	for _, d := range digests {
		g, err := snapshot.DecodeGateResult(results[d])
		if err != nil {
			continue
		}
		why, err := h.descent(candidate, g.CandidateTreeOid)
		if err != nil {
			return "", nil, nil, nil, err
		}
		if why == "" {
			return d, results[d], g, nil, nil
		}
		if pending == nil && len(h.trees[g.CandidateTreeOid]) == 0 {
			pending = &GateReuse{Result: d, AttemptID: g.AttemptID, Tree: g.CandidateTreeOid, Gate: g.GateID, Branch: branch}
		}
	}
	return "", nil, nil, pending, nil
}

// gateInputs digests the candidate tree, the (empty) declared inputs and,
// when any placeholder was substituted, the argv the gate runs with, so a
// result binds the attempt facts it ran with; an argv run as declared keeps
// the old digest.
func gateInputs(candidate string, argv []string, substituted bool) wire.Digest {
	inputObject := wire.NewObject().Set("tree", wire.String(candidate)).Set("inputs", wire.Array())
	if substituted {
		inputObject.Set("argv", wire.Strings(argv))
	}
	return wire.Sum(wire.EncodeFile(wire.ObjectValue(inputObject)))
}

// unlockedAttempt reads the attempt and policy without the store lock; the
// prepared model and locked guard recheck both before committing.
func unlockedAttempt(ctx context.Context, repo *intent.Repository, attemptID string) (*snapshot.Attempt, *intent.Policy, error) {
	a, policy, _, err := unlockedAttemptProof(ctx, repo, attemptID)
	return a, policy, err
}

// gateAdmitted reports whether the lease write that would record a gate run
// admits its actor's role and passes the writer guards, checked before the
// program starts so a refused role or an ALL barrier never runs it
// (V1-3008). A refusal is not reported here: the run proceeds with no gate
// facts, and the lease write reports its canonical refusal under the lock,
// where both are checked again after the program would have run. A read a
// concurrent commit tore is retried, as leaseWrite's pre-round guard read is
// (CAL-V0-372).
func gateAdmitted(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice) (bool, error) {
	if actor.Role != "OWNER" && actor.Role != "OPERATOR" {
		return false, nil
	}
	lease := choice.Lease
	operation := guardOperation(transaction.Request{Operation: transaction.Lease, Lease: &lease})
	for round := 0; round < 16; round++ {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		_, err := unlockedWriterGuards(ctx, repo, operation)
		if wire.CodeOf(err) == wire.CodeSnapshotMoved {
			continue
		}
		return err == nil, nil
	}
	return false, nil
}

// unlockedAttemptProof is unlockedAttempt that also returns the read it
// decoded both from.
func unlockedAttemptProof(ctx context.Context, repo *intent.Repository, attemptID string) (*snapshot.Attempt, *intent.Policy, *journal.Result, error) {
	path := "attempts/" + attemptID + ".json"
	proof, err := readLeaseProof(ctx, repo)
	if err != nil {
		return nil, nil, nil, err
	}
	record, ok := proof.Records[path]
	if !ok || record.Sha256 == nil {
		return nil, nil, nil, wire.Errorf(wire.CodeMalformed, "attemptId", "attempt does not exist")
	}
	a, err := snapshot.DecodeAttempt(record.Raw)
	if err != nil {
		return nil, nil, nil, err
	}
	policy, err := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
	return a, policy, proof, err
}

// attemptRepositories returns the extra repositories of the supervised
// program whose current attempt is a, or nil for an unsupervised or
// single-repository attempt (CAL-V0-087). An attempt its program no longer
// names gets nil, so a composite candidate fails closed as a stale tree.
func attemptRepositories(proof *journal.Result, a *snapshot.Attempt) ([]snapshot.RepositoryRecord, error) {
	if a.Supervision == nil || a.Supervision.ProgramID == "" {
		return nil, nil
	}
	record, ok := proof.Records["programs.json"]
	if !ok || record.Sha256 == nil {
		return nil, nil
	}
	ps, err := snapshot.DecodePrograms(record.Raw)
	if err != nil {
		return nil, err
	}
	for _, p := range ps.Entries {
		if p.ID == a.Supervision.ProgramID && p.CurrentAttempt == a.AttemptID && p.CurrentGeneration == string(a.Generation) {
			return p.Repositories, nil
		}
	}
	return nil, nil
}

// runnable says the model could record a run of this attempt: the command's
// generation, a live unexpired lease, and a submitted tree.
func runnable(a *snapshot.Attempt, l transaction.LeaseRequest, now time.Time) bool {
	if a.Lease == nil {
		return false
	}
	expires, err := time.Parse("2006-01-02T15:04:05Z", string(a.Lease.ExpiresAt))
	return err == nil && a.Live() && a.Generation.Uint64() == l.Generation.Uint64() && now.Before(expires) && (a.Phase == "BUILT" || a.Phase == "CHECKING") && a.CandidateTreeOid != nil
}

// commandGate is the policy gate this runner supports: a COMMAND gate run in
// the worktree with an expected exit code and no reducer, no shared resource, no declared inputs, and at most
// the captured output as its evidence.
func commandGate(policy *intent.Policy, gateID string) (*intent.GateDefinition, error) {
	def := policy.Gate(gateID)
	if def == nil {
		return nil, wire.Errorf(wire.CodeGateUnknown, "gate", "policy declares no gate %q", gateID)
	}
	if def.Kind != "COMMAND" || def.ExpectedExit == nil || def.Reducer != nil {
		return nil, wire.Errorf(wire.CodeUnsupported, "gate", "gate %s is not a COMMAND gate with an expected exit code and no reducer", gateID)
	}
	if def.Cwd != "WORKTREE" {
		return nil, wire.Errorf(wire.CodeUnsupported, "gate", "gate %s runs in %s, not the worktree", gateID, def.Cwd)
	}
	if def.SharedResource != nil || len(def.Inputs) != 0 {
		return nil, wire.Errorf(wire.CodeUnsupported, "gate", "gate %s declares a shared resource or inputs", gateID)
	}
	for _, label := range def.Evidence {
		if label != "output" {
			return nil, wire.Errorf(wire.CodeUnsupported, "gate", "gate %s declares evidence %q beyond the captured output", gateID, label)
		}
	}
	return def, nil
}

// worktreeTree observes the worktree's HEAD tree and whether it is clean.
func worktreeTree(worktree string) (string, bool, error) {
	return observeWorktreeTree(context.Background(), worktree)
}

// observeWorktreeTree is worktreeTree under ctx: each Git observation ends
// when ctx does, runs no fsmonitor hook and keeps bounded output (V1-3009).
func observeWorktreeTree(ctx context.Context, worktree string) (string, bool, error) {
	tree, err := resolveObjectWith(observedGitRunner(ctx), worktree, "HEAD", "tree")
	if err != nil {
		return "", false, err
	}
	status, truncated, err := observeGit(ctx, worktree, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", false, err
	}
	return tree, len(status) == 0 && !truncated, nil
}

// observedCandidate is the candidate tree a gate observes in worktree and
// whether it is clean. With extra repositories it is the composite of the
// worktree's HEAD tree and each sibling worktree's HEAD commit, clean only
// when every worktree is, and each sibling must belong to its declared
// repository (CAL-V0-087).
func observedCandidate(ctx context.Context, worktree string, repos []snapshot.RepositoryRecord) (string, bool, error) {
	tree, clean, err := observeWorktreeTree(ctx, worktree)
	if err != nil || len(repos) == 0 {
		return tree, clean, err
	}
	commits := map[string]string{}
	for _, r := range repos {
		at := extraPath(filepath.Clean(worktree), r.Name)
		if _, _, shared, err := gitIdentities(at); err != nil || shared != r.CommonIdentity {
			return "", false, wire.Errorf(wire.CodeStaleTree, "worktree", "repository %s worktree %s is absent or not that repository", r.Name, at)
		}
		head, err := resolveObject(at, "HEAD", "commit")
		if err != nil {
			return "", false, err
		}
		_, ok, err := observeWorktreeTree(ctx, at)
		if err != nil {
			return "", false, err
		}
		clean = clean && ok
		commits[r.Name] = head
	}
	composite, err := compositeTree(worktree, tree, commits)
	return composite, clean, err
}

// atCandidate refuses a worktree whose HEAD tree (or, with extra
// repositories, composite candidate) is not the candidate or that has any
// change or untracked file.
func atCandidate(ctx context.Context, worktree, candidate string, repos []snapshot.RepositoryRecord) error {
	tree, clean, err := observedCandidate(ctx, worktree, repos)
	if err != nil {
		return err
	}
	if tree != candidate {
		return wire.Errorf(wire.CodeStaleTree, "worktree", "worktree HEAD tree %s is not the candidate %s", tree, candidate)
	}
	if !clean {
		return wire.Errorf(wire.CodeDirtyWorktree, "worktree", "worktree has uncommitted or untracked changes")
	}
	return nil
}

// The closed gate argv placeholders (TCB-V0-015, TCB-V0-037). An argv entry
// exactly equal to one is replaced, just before the gate runs, with the
// native attempt fact it names; any other entry, including one that merely
// contains a placeholder, runs as declared.
const (
	GateArgBaseCommit      = "{attempt.baseCommit}"
	GateArgCandidateCommit = "{attempt.candidateCommit}"
	GateArgTouchPaths      = "{claim.touchPaths}"
	GateArgVerdictPaths    = "{claim.verdictPaths}"
)

// gateArgv is the argv GateRun executes: def.Argv with each whole-entry
// placeholder replaced by its attempt fact, and whether any entry was
// (TCB-V0-015). The base is the attempt's recorded base commit; the candidate
// commit is the worktree HEAD, whose tree atCandidate has just proven is the
// candidate; the touch paths are the claim-era ticket's effects.touchPaths as
// one canonical JSON array, read only while the current ticket record is
// byte for byte the one the claim pinned, and the verdict paths its
// effects.verdictPaths, or its touchPaths when it declares none. A composite
// candidate has no single base or commit, so placeholders on one refuse
// before the gate runs.
func gateArgv(def *intent.GateDefinition, proof *journal.Result, a *snapshot.Attempt, worktree string, repos []snapshot.RepositoryRecord) ([]string, bool, error) {
	argv := append([]string{}, def.Argv...)
	substituted := false
	for i, arg := range argv {
		if arg != GateArgBaseCommit && arg != GateArgCandidateCommit && arg != GateArgTouchPaths && arg != GateArgVerdictPaths {
			continue
		}
		if len(repos) > 0 {
			return nil, false, wire.Errorf(wire.CodeUnsupported, "gate", "gate %s substitutes attempt facts, which a multi-repository candidate does not have", def.GateID)
		}
		var fact string
		var err error
		switch arg {
		case GateArgBaseCommit:
			fact = a.BaseCommit
		case GateArgCandidateCommit:
			fact, err = resolveObject(worktree, "HEAD", "commit")
		default:
			fact, err = claimPaths(proof, a, arg == GateArgVerdictPaths)
		}
		if err != nil {
			return nil, false, err
		}
		argv[i], substituted = fact, true
	}
	return argv, substituted, nil
}

// claimPaths is the claim-era ticket's effects.touchPaths as a canonical JSON
// array, or with verdict its effects.verdictPaths when declared (TCB-V0-037).
// A ticket edited since the claim refuses STALE_TICKET: the claim pinned the
// record digest, not its bytes, so a re-claim restores the fact.
func claimPaths(proof *journal.Result, a *snapshot.Attempt, verdict bool) (string, error) {
	record, ok := proof.Records["intent/tickets/"+a.TicketID.Local+".json"]
	if !ok || record.Sha256 == nil || wire.Sum(record.Raw) != a.TicketRecordSha256 {
		return "", wire.Errorf(wire.CodeStaleTicket, "gate", "ticket %s changed since its claim; re-claim to gate on its touch paths", a.TicketID.Raw)
	}
	rec, err := ticket.Decode(record.Raw)
	if err != nil {
		return "", err
	}
	paths := rec.Effects.TouchPaths
	if verdict && rec.Effects.VerdictPaths != nil {
		paths = rec.Effects.VerdictPaths
	}
	return string(wire.Encode(wire.Strings(paths))), nil
}

// cappedOutput keeps the first max bytes of combined output and kills the
// process group on overflow. Zero max retains the executable gate's 16 MiB cap.
type cappedOutput struct {
	mu       sync.Mutex
	max      int
	buf      bytes.Buffer
	overflow bool
	kill     func()
}

func (c *cappedOutput) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	max := c.max
	if max == 0 {
		max = wire.MaxGateOutputBytes
	}
	room := max - c.buf.Len()
	if len(p) > room {
		c.buf.Write(p[:room])
		if !c.overflow {
			c.overflow = true
			c.kill()
		}
		return len(p), nil
	}
	return c.buf.Write(p)
}

// gateEnvironment passes exactly the declared names that are set, and
// digests the names with their presence, never their values.
func gateEnvironment(names []string) ([]string, wire.Digest) {
	return environmentOf(names, false)
}

// gateEnvironmentKey is gateEnvironment for a gate result's
// environmentSha256. A gate declared reusable also digests each set
// variable's value, so a PASS under another PATH, GOTOOLCHAIN or other
// declared value is never reused (TCD-V0-158); any other gate keeps the
// names-and-presence digest, so a non-reusable gate records nothing derived
// from its values. A reusable gate's key also binds the program a bare
// argv[0] resolves to (gateExecutableEntry, V1-3010). keyed is false when
// that program resolved but its bytes could not be hashed: the run is then
// not reusable, so it neither reuses nor is matched by a later run's key.
func gateEnvironmentKey(def *intent.GateDefinition, argv []string) ([]string, wire.Digest, bool) {
	if !def.Reusable {
		env, digest := environmentOf(def.Env, false)
		return env, digest, true
	}
	entry, keyed := gateExecutableEntry(argv)
	env, digest := environmentOf(def.Env, true, entry...)
	return env, digest, keyed
}

// gateExecutableEntry is the reuse-key entry of a reusable gate whose argv[0]
// is a bare name (V1-3010). exec.Command resolves such a name through this
// process's PATH whether or not the gate declares PATH, so two agents whose
// PATHs resolve it to different programs must not share a PASS. The entry
// names the resolved path and the SHA-256 of its content, or that it did not
// resolve. A path argv[0] adds nothing, so its key is unchanged. A program
// that resolves but cannot be hashed (unreadable, or not a regular file)
// names its path with no digest and reports false: distinct such programs
// must not share a key, so the run is not reusable (fail closed).
func gateExecutableEntry(argv []string) ([]wire.Value, bool) {
	if len(argv) == 0 || filepath.Base(argv[0]) != argv[0] {
		return nil, true
	}
	entry := wire.NewObject().Set("executable", wire.String(argv[0]))
	path, err := exec.LookPath(argv[0])
	if err != nil {
		entry.Set("resolved", wire.Bool(false))
		return []wire.Value{wire.ObjectValue(entry)}, true
	}
	entry.Set("resolved", wire.Bool(true)).Set("path", wire.String(path))
	digest, err := executableSha256(path)
	if err != nil {
		entry.Set("hashed", wire.Bool(false))
		return []wire.Value{wire.ObjectValue(entry)}, false
	}
	entry.Set("sha256", wire.String(string(digest)))
	return []wire.Value{wire.ObjectValue(entry)}, true
}

// executableSha256 is the SHA-256 of the whole regular file at path. A
// path that is not a regular file (a FIFO or device on PATH could block the
// read indefinitely) is refused before it is opened, and the opened file
// must be that same regular file.
func executableSha256(path string) (wire.Digest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New(path + " is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return "", errors.New(path + " changed while it was opened")
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return wire.Digest(hex.EncodeToString(h.Sum(nil))), nil
}

func environmentOf(names []string, values bool, extra ...wire.Value) ([]string, wire.Digest) {
	env := []string{}
	entries := []wire.Value{}
	for _, name := range names {
		value, set := os.LookupEnv(name)
		if set {
			env = append(env, name+"="+value)
		}
		entry := wire.NewObject().Set("name", wire.String(name)).Set("present", wire.Bool(set))
		if values && set {
			entry.Set("valueSha256", wire.String(string(wire.Sum([]byte(value)))))
		}
		entries = append(entries, wire.ObjectValue(entry))
	}
	entries = append(entries, extra...)
	return env, wire.Sum(wire.EncodeFile(wire.Array(entries...)))
}

// execution is what one process run observed. It carries no process group
// id: execute has reaped the gate, so that id confers no kill authority
// (PGO-V0-009).
type execution struct {
	class    string
	exitCode *wire.Count
	signal   *string
}

func execute(ctx context.Context, def *intent.GateDefinition, worktree string, env []string, out *cappedOutput) execution {
	return executeArgv(ctx, def, def.Argv, worktree, env, out)
}

// executeArgv is execute with the argv GateRun substituted (TCB-V0-015).
func executeArgv(ctx context.Context, def *intent.GateDefinition, argv []string, worktree string, env []string, out *cappedOutput) execution {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin = worktree, env, nil
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = gateWaitDelay // a descendant that escaped the group cannot hold the pipe open
	containGate(cmd)
	var timedOut bool
	var mu sync.Mutex
	// Early kills stop only the unreaped leader; on Darwin and Linux reapGate
	// SIGKILLs the group while the exited leader is unreaped, so a kill that
	// races the reap never names a reused group. Other Unix platforms send no
	// group signal (V1-1047).
	out.kill = func() { stopGate(cmd) }
	if err := cmd.Start(); err != nil {
		return execution{class: "SPAWN_FAILED"}
	}
	timer := time.AfterFunc(time.Duration(def.TimeoutSeconds.Int())*time.Second, func() {
		mu.Lock()
		timedOut = true
		mu.Unlock()
		stopGate(cmd)
	})
	stop := context.AfterFunc(ctx, func() { stopGate(cmd) })
	err := reapGate(cmd)
	timer.Stop()
	stop()
	mu.Lock()
	defer mu.Unlock()
	switch {
	case timedOut:
		return execution{class: "TIMEOUT"}
	case out.overflow:
		return execution{class: "OUTPUT_LIMIT"}
	}
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		return execution{class: "UNKNOWN"}
	}
	if name := gateSignal(cmd.ProcessState); name != "" {
		return execution{class: "SIGNAL", signal: &name}
	}
	code := wire.CountOf(int64(cmd.ProcessState.ExitCode()))
	return execution{class: "EXIT", exitCode: &code}
}

// gateState is the §7.1 state of one run: PASSED only for the expected
// exit at the candidate tree in a clean worktree, BLOCKED when the run left
// the worktree dirty or moved it, FAILED otherwise.
func gateState(run execution, expected wire.Count, clean, atCandidate bool) string {
	switch {
	case run.class != "EXIT" || run.exitCode.Int() != expected.Int():
		return "FAILED"
	case !clean || !atCandidate:
		return "BLOCKED"
	}
	return "PASSED"
}

func runGate(ctx context.Context, def *intent.GateDefinition, argv []string, substituted bool, policy *intent.Policy, a *snapshot.Attempt, worktree string, repos []snapshot.RepositoryRecord, clock func() time.Time) (gateRun, error) {
	env, envDigest, _ := gateEnvironmentKey(def, argv)
	out := &cappedOutput{}
	started := clock()
	run := executeArgv(ctx, def, argv, worktree, env, out)
	ended := clock()
	tree, clean, err := observeAfterRun(ctx, def.TimeoutSeconds.Int(), worktree, repos)
	if err != nil {
		return gateRun{}, err
	}
	output := out.buf.Bytes()
	cwd := "WORKTREE"
	candidate := *a.CandidateTreeOid
	inputs := gateInputs(candidate, argv, substituted)
	g := snapshot.GateResult{GateID: def.GateID, AttemptID: a.AttemptID, Generation: a.Generation, TicketRevision: a.TicketRevision, DefinitionSha256: policy.GateDefinitionSha256(def.GateID), CandidateTreeOid: candidate, ExecutedTreeOid: &tree, ExecutedCwd: &cwd, PorcelainClean: &clean, InputsSha256: inputs, EnvironmentSha256: envDigest, PolicySha256: a.PolicySha256, ConfigSha256: a.ConfigSha256, StartedAt: stamp(started), EndedAt: stamp(ended), State: gateState(run, *def.ExpectedExit, clean, tree == candidate), OutcomeClass: run.class, ExitCode: run.exitCode, Signal: run.signal, Evidence: []snapshot.GateEvidence{{Label: "output", Sha256: wire.Sum(output), Bytes: wire.SizeOf(uint64(len(output)))}}}
	record, err := g.Encode()
	return gateRun{record: record, output: output}, err
}

// observeAfterRun is the post-run worktree observation, bounded by the
// gate's timeout. A run whose context ended before or during it observes
// again under the interrupted-record bound, so the run's result and output
// reach the interrupted-result record instead of being discarded
// (CAL-V0-319, V1-3009).
func observeAfterRun(ctx context.Context, timeoutSeconds int64, worktree string, repos []snapshot.RepositoryRecord) (string, bool, error) {
	if ctx.Err() == nil {
		if gateObserveFault != nil {
			gateObserveFault()
		}
		observe, cancel := gateObservation(ctx, timeoutSeconds)
		tree, clean, err := observedCandidate(observe, worktree, repos)
		cancel()
		if err == nil || ctx.Err() == nil {
			return tree, clean, err
		}
	}
	base, stop := context.WithTimeout(context.WithoutCancel(ctx), gateInterruptRecordBound)
	defer stop()
	observe, cancel := gateObservation(base, timeoutSeconds)
	defer cancel()
	return observedCandidate(observe, worktree, repos)
}

func stamp(t time.Time) wire.Timestamp {
	return wire.Timestamp(t.UTC().Format("2006-01-02T15:04:05Z"))
}

// gateFacts builds the observer for a lease verb that needs git or gate
// observations, or nil.
func gateFacts(repo *intent.Repository, choice LeaseChoice) claimObserver {
	l := choice.Lease
	switch l.Verb {
	case transaction.LeaseSubmit:
		return func(proof *journal.Result, _ *transaction.Input) (transaction.LeaseFacts, error) {
			return submitFacts(leaseRoot(repo, choice), proof, l)
		}
	case transaction.LeaseGateRun:
		return func(proof *journal.Result, _ *transaction.Input) (transaction.LeaseFacts, error) {
			results, err := attemptGateResults(repo, proof, l.AttemptID)
			if choice.gate == nil || err != nil {
				return transaction.GateFacts(nil, nil, nil, "", false, results), err
			}
			if choice.gate.source != nil {
				return gateReuseFacts(leaseRoot(repo, choice), proof, l.AttemptID, choice.gate, results)
			}
			return transaction.GateFacts(nil, choice.gate.record, choice.gate.output, "", false, results), nil
		}
	case transaction.LeaseComplete:
		return func(proof *journal.Result, _ *transaction.Input) (transaction.LeaseFacts, error) {
			return completeFacts(repo, leaseRoot(repo, choice), proof, l)
		}
	}
	return nil
}

func lockedAttempt(proof *journal.Result, attemptID string) (*snapshot.Attempt, bool) {
	record, ok := proof.Records["attempts/"+attemptID+".json"]
	if !ok || record.Sha256 == nil {
		return nil, false
	}
	a, err := snapshot.DecodeAttempt(record.Raw)
	return a, err == nil
}

// submitFacts names the paths the submitted tree changes against both the
// attempt's base tree and the intent branch's tip tree, rename-free
// (CAL-V0-024, amendment A13): a rebase onto work merged since the claim
// does not count that work's paths.
func submitFacts(root string, proof *journal.Result, l transaction.LeaseRequest) (transaction.LeaseFacts, error) {
	a, ok := lockedAttempt(proof, l.AttemptID)
	if !ok {
		return transaction.LeaseFacts{}, nil
	}
	if _, err := resolveObject(root, l.Tree, "tree"); err != nil {
		return transaction.LeaseFacts{}, err
	}
	q, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	fromBase, err := changedPaths(root, a.BaseCommit+"^{tree}", l.Tree)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	fromTip, err := changedPaths(root, "refs/heads/"+q.IntentBranch+"^{tree}", l.Tree)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	tip := map[string]bool{}
	for _, p := range fromTip {
		tip[p] = true
	}
	changed := []string{}
	for _, p := range fromBase {
		if tip[p] {
			changed = append(changed, p)
		}
	}
	return transaction.GateFacts(changed, nil, nil, "", false, nil), nil
}

// changedPaths is the rename-free set of paths that differ between two
// trees, in diff order.
func changedPaths(root, from, to string) ([]string, error) {
	out, err := gitOutput(root, "diff-tree", "-r", "--name-only", "--no-renames", "-z", "--end-of-options", from, to)
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

// completeFacts observes the completing commit's tree and whether the
// intent branch contains it, plus the attempt's gate results. For a
// multi-repository attempt the tree is the composite of that commit's tree
// and each extra repository's candidate, and the candidate is reachable only
// when every changed extra repository's designated integration branch also
// contains its candidate (CAL-V0-087).
func completeFacts(repo *intent.Repository, root string, proof *journal.Result, l transaction.LeaseRequest) (transaction.LeaseFacts, error) {
	results, err := attemptGateResults(repo, proof, l.AttemptID)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	q, err := intent.DecodeQueue(proof.Records["intent/queue.json"].Raw)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	if _, err = resolveObject(root, l.Commit, "commit"); err != nil {
		return transaction.LeaseFacts{}, err
	}
	tree, err := resolveObject(root, l.Commit, "tree")
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	reachable, upstream, err := intentIntegrated(root, l.Commit, q.IntentBranch)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	unintegrated := ""
	var repos []snapshot.RepositoryRecord
	if a, ok := lockedAttempt(proof, l.AttemptID); ok {
		if repos, err = attemptRepositories(proof, a); err != nil {
			return transaction.LeaseFacts{}, err
		}
	}
	if len(repos) > 0 {
		if tree, err = compositeTree(root, tree, candidateCommits(repos)); err != nil {
			return transaction.LeaseFacts{}, err
		}
		for _, r := range repos {
			if !reachable {
				break
			}
			if reachable, err = repositoryIntegrated(r); err != nil {
				return transaction.LeaseFacts{}, err
			}
			if !reachable {
				unintegrated = r.Name
			}
		}
	}
	a, _ := lockedAttempt(proof, l.AttemptID)
	descendants, err := descendantResults(root, proof, a, results)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	return transaction.CompleteFacts(tree, reachable, upstream, unintegrated, results).WithDescendantResults(descendants), nil
}

// gateReuseFacts are a reused gate run's facts under the lock; a source at
// another tree than the attempt's candidate carries the ancestry verdict
// rechecked from git now (TCD-V0-152).
func gateReuseFacts(root string, proof *journal.Result, attemptID string, run *gateRun, results map[wire.Digest][]byte) (transaction.LeaseFacts, error) {
	a, ok := lockedAttempt(proof, attemptID)
	if ci, err := snapshot.DecodeCIRun(run.source); err == nil {
		return ciRunFacts(root, proof, a, ci, run, results)
	}
	src, err := snapshot.DecodeGateResult(run.source)
	if err != nil || !ok || a.CandidateTreeOid == nil || src.CandidateTreeOid == *a.CandidateTreeOid {
		return transaction.GateReuseFacts(run.record, run.source, results), nil
	}
	h, err := loadProtectedHistory(root, intentBranch(proof))
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	why, err := h.descent(*a.CandidateTreeOid, src.CandidateTreeOid)
	if err != nil {
		return transaction.LeaseFacts{}, err
	}
	return transaction.GateDescendantReuseFacts(run.record, run.source, why, results), nil
}

// intentIntegrated reports whether the intent branch contains commit, or,
// when it does not, whether the branch's configured upstream does, provided
// that upstream is an existing remote-tracking ref (CAL-V0-017, V1-1081). A
// checked-out intent branch can lag a remote main that already merged the
// commit, and moving it under unrelated staged work is unsafe. It never
// fetches and never writes a ref; it returns the upstream ref it checked, or
// "" when none is configured, resolvable or remote-tracking.
func intentIntegrated(root, commit, branch string) (bool, string, error) {
	local := "refs/heads/" + branch
	reachable, err := isAncestor(root, commit, local)
	if err != nil || reachable {
		return reachable, "", err
	}
	upstream := configuredUpstream(root, local)
	if upstream == "" {
		return false, "", nil
	}
	reachable, err = isAncestor(root, commit, upstream)
	return reachable, upstream, err
}

// configuredUpstream names local's configured upstream when it is an
// existing refs/remotes/ ref, else "". A missing, unresolvable or local
// upstream fails closed: the commit then stays unreachable.
func configuredUpstream(root, local string) string {
	out, err := gitOutput(root, "for-each-ref", "--format=%(refname)%00%(upstream)", "--end-of-options", local)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		ref, upstream, ok := strings.Cut(line, "\x00")
		if !ok || ref != local || !strings.HasPrefix(upstream, "refs/remotes/") {
			continue
		}
		if _, err := gitOutput(root, "show-ref", "--verify", "--quiet", "--end-of-options", upstream); err != nil {
			return ""
		}
		return upstream
	}
	return ""
}

// candidateCommits maps each extra repository to its candidate commit.
func candidateCommits(repos []snapshot.RepositoryRecord) map[string]string {
	out := map[string]string{}
	for _, r := range repos {
		out[r.Name] = r.Candidate
	}
	return out
}

// repositoryIntegrated reports whether an extra repository's candidate is
// integrated: trivially for an unchanged repository, otherwise only in the
// admitted designated checkout whose integration branch contains it.
func repositoryIntegrated(r snapshot.RepositoryRecord) (bool, error) {
	if r.Candidate == "" {
		return false, wire.Errorf(wire.CodeMissingEvidence, "repositories", "repository %s candidate absent", r.Name)
	}
	if r.Candidate == r.Base {
		return true, nil
	}
	if r.IntegrationBranch == "" {
		return false, nil
	}
	if id, err := supervisor.DirectoryIdentity(r.Checkout); err != nil || id != r.IntegrationIdentity {
		return false, nil
	}
	if _, _, shared, err := gitIdentities(r.Checkout); err != nil || shared != r.CommonIdentity {
		return false, nil
	}
	return isAncestor(r.Checkout, r.Candidate, "refs/heads/"+r.IntegrationBranch)
}

// isAncestor runs git merge-base --is-ancestor, whose exit status 1 means
// no and any other failure is an unobservable answer.
func isAncestor(root, commit, ref string) (bool, error) {
	c := exec.Command("git", "-c", "credential.helper=", "merge-base", "--is-ancestor", "--end-of-options", commit, ref)
	c.Dir, c.Env = root, gitEnvironment()
	err := c.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	return false, wire.Errorf(wire.CodeUnsupported, "git", "git merge-base failed: %v", err)
}

// attemptGateResults reads the evidence file of every gate result the
// attempt names; the model checks each against the inventory.
func attemptGateResults(repo *intent.Repository, proof *journal.Result, attemptID string) (map[wire.Digest][]byte, error) {
	out := map[wire.Digest][]byte{}
	a, ok := lockedAttempt(proof, attemptID)
	if !ok {
		return out, nil
	}
	for _, d := range a.GateResults {
		raw, err := intent.ReadFile(filepath.Join(repo.StateDir, "evidence", d), snapshot.MaxGateRecordBytes)
		if err != nil {
			return nil, err
		}
		out[wire.Digest(d)] = raw
	}
	return out, nil
}
