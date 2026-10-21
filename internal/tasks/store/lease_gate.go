package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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

// gateRun is one finished gate run: its encoded result and captured output,
// or a reuse: the rebound record and the source result it names, with no
// output (CAL-V0-240).
type gateRun struct{ record, output, source []byte }

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
	if runnable(a, choice.Lease, clock()) {
		def, err := commandGate(policy, choice.Lease.Gate)
		if err != nil {
			return &Report{}, err
		}
		repos, err := attemptRepositories(proof, a)
		if err != nil {
			return &Report{}, err
		}
		if err = atCandidate(worktree, *a.CandidateTreeOid, repos); err != nil {
			return &Report{}, err
		}
		argv, substituted, err := gateArgv(def, proof, a, worktree, repos)
		if err != nil {
			return &Report{}, err
		}
		run, reused, err := reusableRun(repo, leaseRoot(repo, choice), proof, policy, def, a, argv, substituted)
		if err != nil {
			return &Report{}, err
		}
		if !reused {
			executed = true
			if run, err = runGate(ctx, def, argv, substituted, policy, a, worktree, repos, clock); err != nil {
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
		switch {
		case err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted:
		case report.Kind == "Transaction" && choice.gate != nil && choice.gate.source != nil:
			report.GateReusedFrom = gateReuseOf(choice.gate.source)
		case report.Kind == "Replay" && report.Outcome.ReceiptSeq != nil:
			report.GateReusedFrom = recordedReuse(repo, report.Outcome.ReceiptSeq.Uint64(), choice.Lease.AttemptID, choice.Lease.Gate)
		}
	}
	return report, err
}

// GateReuse names the earlier result a gate run reused: its digest and the
// attempt that executed it (CAL-V0-240).
type GateReuse struct {
	Result    wire.Digest
	AttemptID string
}

func gateReuseOf(source []byte) *GateReuse {
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
	_, env := gateEnvironment(def.Env)
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
	if source == nil && len(elsewhere) > 0 {
		var err error
		if best, source, chosen, err = descendantSource(root, intentBranch(proof), candidate, elsewhere); err != nil {
			return gateRun{}, false, err
		}
	}
	if source == nil {
		return gateRun{}, false, nil
	}
	g := transaction.ReusedGateResult(chosen, best, a)
	record, err := g.Encode()
	return gateRun{record: record, source: source}, err == nil, err
}

// descendantSource is the lowest-digest result among results at other trees
// whose tree is on the protected intent branch as a descendant of candidate
// (TCD-V0-151), or none.
func descendantSource(root, branch, candidate string, results map[wire.Digest][]byte) (wire.Digest, []byte, *snapshot.GateResult, error) {
	digests := make([]wire.Digest, 0, len(results))
	for d := range results {
		digests = append(digests, d)
	}
	slices.Sort(digests)
	h, err := loadProtectedHistory(root, branch)
	if err != nil {
		return "", nil, nil, err
	}
	for _, d := range digests {
		g, err := snapshot.DecodeGateResult(results[d])
		if err != nil {
			continue
		}
		why, err := h.descent(candidate, g.CandidateTreeOid)
		if err != nil {
			return "", nil, nil, err
		}
		if why == "" {
			return d, results[d], g, nil
		}
	}
	return "", nil, nil, nil
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
	tree, err := resolveObject(worktree, "HEAD", "tree")
	if err != nil {
		return "", false, err
	}
	status, err := gitOutput(worktree, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return "", false, err
	}
	return tree, len(status) == 0, nil
}

// observedCandidate is the candidate tree a gate observes in worktree and
// whether it is clean. With extra repositories it is the composite of the
// worktree's HEAD tree and each sibling worktree's HEAD commit, clean only
// when every worktree is, and each sibling must belong to its declared
// repository (CAL-V0-087).
func observedCandidate(worktree string, repos []snapshot.RepositoryRecord) (string, bool, error) {
	tree, clean, err := worktreeTree(worktree)
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
		_, ok, err := worktreeTree(at)
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
func atCandidate(worktree, candidate string, repos []snapshot.RepositoryRecord) error {
	tree, clean, err := observedCandidate(worktree, repos)
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
	env := []string{}
	entries := []wire.Value{}
	for _, name := range names {
		value, set := os.LookupEnv(name)
		if set {
			env = append(env, name+"="+value)
		}
		entries = append(entries, wire.ObjectValue(wire.NewObject().Set("name", wire.String(name)).Set("present", wire.Bool(set))))
	}
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
	env, envDigest := gateEnvironment(def.Env)
	out := &cappedOutput{}
	started := clock()
	run := executeArgv(ctx, def, argv, worktree, env, out)
	ended := clock()
	tree, clean, err := observedCandidate(worktree, repos)
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
	src, err := snapshot.DecodeGateResult(run.source)
	a, ok := lockedAttempt(proof, attemptID)
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
