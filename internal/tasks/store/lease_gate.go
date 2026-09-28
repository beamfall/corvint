package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateWaitDelay bounds how long a finished or killed gate may keep its
// output pipe open through a descendant outside its process group.
const gateWaitDelay = 5 * time.Second

// gateRun is one finished gate run: its encoded result and captured output.
type gateRun struct{ record, output []byte }

// GateRun runs one policy command gate in worktree at the attempt's
// candidate tree, then records it (CAL-V0-016). The gate runs before the
// store lock is taken, so a long gate never holds it; the prepared model
// rechecks the attempt, tree and definition, then binds them under lock. A repeated
// request id reruns the gate before the replay is found. A stale or
// unsubmitted attempt is not run: the model answers it with no gate facts.
// Before reading, it settles the journal through guarded recovery, so a gate run
// retried after a crash recovers as any other writer does (CAL-V0-019).
func GateRun(ctx context.Context, repo *intent.Repository, actor mutation.Binding, choice LeaseChoice, worktree string, clock func() time.Time) (*Report, error) {
	redone, err := settleLease(ctx, repo)
	if err != nil {
		return &Report{}, err
	}
	a, policy, err := unlockedAttempt(ctx, repo, choice.Lease.AttemptID)
	if err != nil {
		return &Report{}, err
	}
	if runnable(a, choice.Lease, clock()) {
		def, err := commandGate(policy, choice.Lease.Gate)
		if err != nil {
			return &Report{}, err
		}
		if err = atCandidate(worktree, *a.CandidateTreeOid); err != nil {
			return &Report{}, err
		}
		run, err := runGate(ctx, def, policy, a, worktree, clock)
		if err != nil {
			return &Report{}, err
		}
		if err = ctx.Err(); err != nil {
			return &Report{}, err
		}
		choice.gate = &run
	}
	now, err := wire.ParseTimestamp("recordedAt", clock().UTC().Format("2006-01-02T15:04:05Z"))
	if err != nil {
		return &Report{}, err
	}
	report, err := Lease(ctx, repo, actor, choice, now)
	if report != nil {
		report.Redone = report.Redone || redone
	}
	return report, err
}

// unlockedAttempt reads the attempt and policy without the store lock; the
// prepared model and locked guard recheck both before committing.
func unlockedAttempt(ctx context.Context, repo *intent.Repository, attemptID string) (*snapshot.Attempt, *intent.Policy, error) {
	path := "attempts/" + attemptID + ".json"
	proof, err := readLeaseProof(ctx, repo)
	if err != nil {
		return nil, nil, err
	}
	record, ok := proof.Records[path]
	if !ok || record.Sha256 == nil {
		return nil, nil, wire.Errorf(wire.CodeMalformed, "attemptId", "attempt does not exist")
	}
	a, err := snapshot.DecodeAttempt(record.Raw)
	if err != nil {
		return nil, nil, err
	}
	policy, err := intent.DecodePolicy(proof.Records["intent/policy.json"].Raw)
	return a, policy, err
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

// atCandidate refuses a worktree whose HEAD tree is not the candidate or
// that has any change or untracked file.
func atCandidate(worktree, candidate string) error {
	tree, clean, err := worktreeTree(worktree)
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

// cappedOutput keeps the first MaxGateOutputBytes of the combined output and
// kills the gate when it writes more.
type cappedOutput struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	overflow bool
	kill     func()
}

func (c *cappedOutput) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	room := wire.MaxGateOutputBytes - c.buf.Len()
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

// execution is what one process run observed.
type execution struct {
	class    string
	exitCode *wire.Count
	signal   *string
}

func execute(ctx context.Context, def *intent.GateDefinition, worktree string, env []string, out *cappedOutput) execution {
	cmd := exec.Command(def.Argv[0], def.Argv[1:]...)
	cmd.Dir, cmd.Env, cmd.Stdin = worktree, env, nil
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = gateWaitDelay // a descendant that escaped the group cannot hold the pipe open
	containGate(cmd)
	var timedOut bool
	var mu sync.Mutex
	out.kill = func() { killGate(cmd) }
	if err := cmd.Start(); err != nil {
		return execution{class: "SPAWN_FAILED"}
	}
	timer := time.AfterFunc(time.Duration(def.TimeoutSeconds.Int())*time.Second, func() {
		mu.Lock()
		timedOut = true
		mu.Unlock()
		killGate(cmd)
	})
	stop := context.AfterFunc(ctx, func() { killGate(cmd) })
	err := cmd.Wait()
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

func runGate(ctx context.Context, def *intent.GateDefinition, policy *intent.Policy, a *snapshot.Attempt, worktree string, clock func() time.Time) (gateRun, error) {
	env, envDigest := gateEnvironment(def.Env)
	out := &cappedOutput{}
	started := clock()
	run := execute(ctx, def, worktree, env, out)
	ended := clock()
	tree, clean, err := worktreeTree(worktree)
	if err != nil {
		return gateRun{}, err
	}
	output := out.buf.Bytes()
	cwd := "WORKTREE"
	candidate := *a.CandidateTreeOid
	inputs := wire.Sum(wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("tree", wire.String(candidate)).Set("inputs", wire.Array()))))
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
// intent branch contains it, plus the attempt's gate results.
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
	reachable, err := isAncestor(root, l.Commit, "refs/heads/"+q.IntentBranch)
	return transaction.GateFacts(nil, nil, nil, tree, reachable, results), err
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
