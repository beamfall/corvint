package store_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// commandGate is a policy COMMAND gate running script under sh with no
// environment, so scripts use builtins or absolute paths.
func commandGate(id, script, timeout string, required bool) wire.Value {
	return obj("gateId", str(id), "kind", str("COMMAND"), "argv", wire.Strings([]string{"sh", "-c", script}), "cwd", str("WORKTREE"),
		"env", wire.Strings(nil), "timeoutSeconds", str(timeout), "expected", obj("exitCode", str("0"), "reducer", wire.Null()),
		"evidence", wire.Strings(nil), "inputs", wire.Strings(nil), "sharedResource", wire.Null(), "reusable", wire.Bool(true), "required", wire.Bool(required))
}

func newGateStore(t *testing.T) *leaseStore {
	t.Helper()
	return newLeaseStore(t,
		commandGate("verify", "printf ok", "30", true),
		commandGate("fails", "printf no; exit 3", "30", false),
		commandGate("slow", "/bin/sleep 5", "1", false),
		commandGate("dirty", ": > junk", "30", false))
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// commit writes each path with its own name as content, commits them, and
// returns the commit and its tree.
func (s *leaseStore) commit(t *testing.T, paths ...string) (string, string) {
	t.Helper()
	for _, p := range paths {
		fixture.Write(t, filepath.Join(s.root, p), []byte(p))
	}
	gitRun(t, s.root, "add", "-A")
	gitRun(t, s.root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "change")
	return gitOut(t, s.root, "rev-parse", "HEAD"), gitOut(t, s.root, "rev-parse", "HEAD^{tree}")
}

func submitOf(a *store.Report, tree string) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseSubmit, AttemptID: a.AttemptID, Generation: a.Generation, Tree: tree}
}

func gateOf(a *store.Report, gate string) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: a.AttemptID, Generation: a.Generation, Gate: gate}
}

func completeOf(a *store.Report, commit string) transaction.LeaseRequest {
	return transaction.LeaseRequest{Verb: transaction.LeaseComplete, AttemptID: a.AttemptID, Generation: a.Generation, Commit: commit}
}

// gate runs l in the checkout with the clock fixed at t0 plus minutes.
func (s *leaseStore) gate(t *testing.T, requestID string, l transaction.LeaseRequest, minutes int) (*store.Report, error) {
	t.Helper()
	at, err := time.Parse("2006-01-02T15:04:05Z", string(s.at(t, minutes)))
	if err != nil {
		t.Fatal(err)
	}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: requestID, Root: s.root, Lease: l}
	return store.GateRun(context.Background(), s.repo, operator(), choice, s.root, func() time.Time { return at })
}

func (s *leaseStore) passes(t *testing.T, requestID string, l transaction.LeaseRequest, minutes int) {
	t.Helper()
	report, err := s.gate(t, requestID, l, minutes)
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("gate %s: %+v %v", requestID, report, err)
	}
}

func (s *leaseStore) evidence(t *testing.T, digest string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "evidence", digest))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// results decodes the attempt's gate results by gateId.
func (s *leaseStore) results(t *testing.T, id string) map[string]*snapshot.GateResult {
	t.Helper()
	out := map[string]*snapshot.GateResult{}
	for _, d := range s.attempt(t, id).GateResults {
		g, err := snapshot.DecodeGateResult(s.evidence(t, d))
		if err != nil {
			t.Fatal(err)
		}
		out[g.GateID] = g
	}
	return out
}

func (s *leaseStore) record(t *testing.T, id string) *ticket.Record {
	t.Helper()
	full, err := wire.ParseTicketID("ticketId", id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.PrimaryWorktree, intent.Dir, "tickets", full.Local+".json"))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := ticket.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// submitted is a claim over dir/ at t0 plus minutes whose commit of
// dir/a.go is submitted a minute later.
func (s *leaseStore) submitted(t *testing.T, id, dir string, minutes int) (*store.Report, string) {
	t.Helper()
	claim := s.claim(t, "claim-"+id, id, minutes, dir+"/")
	commit, tree := s.commit(t, dir+"/a.go")
	if r := s.lease(t, "submit-"+id, submitOf(claim, tree), minutes+1, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("submit: %+v", r)
	}
	return claim, commit
}

// TestCALV0015_SubmitRecordsTheCandidateTree: submit moves the attempt to
// BUILT at the tree, the same tree again is a no-op, a new tree replaces it,
// and a stale generation is fenced.
func TestCALV0015_SubmitRecordsTheCandidateTree(t *testing.T) {
	s := newGateStore(t)
	claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
	_, tree := s.commit(t, "src/a.go")
	report := s.lease(t, "submit-1", submitOf(claim, tree), 1, nil)
	a := s.attempt(t, claim.AttemptID)
	if report.Outcome.Outcome != mutation.OutcomeCompleted || a.Phase != "BUILT" || a.CandidateTreeOid == nil || *a.CandidateTreeOid != tree || a.ScopeCheck != "WITHIN" {
		t.Fatalf("submit: %+v %+v", report, a)
	}
	if again := s.lease(t, "submit-1", submitOf(claim, tree), 1, nil); again.Kind != "Replay" {
		t.Fatalf("replay: %+v", again)
	}
	before := storeDigest(t, s.repo)
	if same := s.lease(t, "submit-2", submitOf(claim, tree), 2, nil); same.Kind != "NoChange" || storeDigest(t, s.repo) != before {
		t.Fatalf("same tree: %+v", same)
	}
	_, next := s.commit(t, "src/b.go")
	s.lease(t, "submit-3", submitOf(claim, next), 3, nil)
	if a := s.attempt(t, claim.AttemptID); *a.CandidateTreeOid != next {
		t.Fatalf("resubmit: %+v", a)
	}
	stale := submitOf(claim, next)
	stale.Generation = wire.SizeOf(claim.Generation.Uint64() + 1)
	refusedWith(t, s.lease(t, "submit-stale", stale, 4, nil), mutation.OutcomeRevisionConflict, wire.CodeFenced)
	auditOK(t, s.repo)
}

// TestCALV0024_SubmitOutsideTheScopeIsRefused: every path added, deleted
// or modified outside the scope is named and nothing is written; a path
// another change merged on the intent branch after the claim is not the
// candidate's; WHOLE_REPOSITORY covers all.
func TestCALV0024_SubmitOutsideTheScopeIsRefused(t *testing.T) {
	s := newGateStore(t)
	one, two := s.ticket(t, "one"), s.ticket(t, "two")
	s.commit(t, "keep/old.go", "keep/mod.go")
	claim := s.claim(t, "claim-1", one, 0, "src/a.go")
	gitRun(t, s.root, "checkout", "-q", "-b", "agent")
	gitRun(t, s.root, "rm", "-q", "keep/old.go")
	fixture.Write(t, filepath.Join(s.root, "keep/mod.go"), []byte("changed"))
	_, tree := s.commit(t, "src/a.go", "docs/x.md", "lib/y.go")
	before := storeDigest(t, s.repo)
	report := s.lease(t, "submit-1", submitOf(claim, tree), 1, nil)
	refusedWith(t, report, mutation.OutcomeBlocked, wire.CodeOutOfScope)
	if !strings.Contains(report.Detail, "docs/x.md keep/mod.go keep/old.go lib/y.go") || strings.Contains(report.Detail, "src/a.go") {
		t.Fatalf("detail: %q", report.Detail)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused submit wrote")
	}
	gitRun(t, s.root, "checkout", "-q", "main")
	s.commit(t, "other/z.go")
	gitRun(t, s.root, "checkout", "-q", "-b", "rebased")
	_, rebased := s.commit(t, "src/a.go")
	if r := s.lease(t, "submit-2", submitOf(claim, rebased), 2, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("rebased submit: %+v", r)
	}
	s.lease(t, "release-1", releaseOf(claim), 3, nil)
	whole := s.claim(t, "claim-2", two, 4)
	if r := s.lease(t, "submit-3", submitOf(whole, tree), 5, nil); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("whole repository submit: %+v", r)
	}
	auditOK(t, s.repo)
}

// TestCALV0016_GateRunRecordsEachResult: PASSED, FAILED, TIMEOUT and a run
// that dirties the worktree are recorded with their output as evidence, and
// a rerun of the same gate replaces its result.
func TestCALV0016_GateRunRecordsEachResult(t *testing.T) {
	s := newGateStore(t)
	claim, _ := s.submitted(t, s.ticket(t, "one"), "src", 0)
	s.passes(t, "gate-1", gateOf(claim, "verify"), 2)
	a := s.attempt(t, claim.AttemptID)
	g := s.results(t, claim.AttemptID)["verify"]
	if a.Phase != "CHECKING" || g == nil || g.State != "PASSED" || g.OutcomeClass != "EXIT" || g.ExitCode.Int() != 0 || *g.ExecutedCwd != "WORKTREE" {
		t.Fatalf("verify: %+v %+v", a, g)
	}
	if out := s.evidence(t, string(g.Evidence[0].Sha256)); string(out) != "ok" {
		t.Fatalf("output: %q", out)
	}
	first := a.GateResults[0]
	s.passes(t, "gate-2", gateOf(claim, "verify"), 3)
	if again := s.attempt(t, claim.AttemptID).GateResults; len(again) != 1 || again[0] == first {
		t.Fatalf("rerun did not replace: %v", again)
	}
	s.passes(t, "gate-3", gateOf(claim, "fails"), 4)
	s.passes(t, "gate-4", gateOf(claim, "slow"), 5)
	s.passes(t, "gate-5", gateOf(claim, "dirty"), 6)
	results := s.results(t, claim.AttemptID)
	for id, want := range map[string][2]string{"fails": {"FAILED", "EXIT"}, "slow": {"FAILED", "TIMEOUT"}, "dirty": {"BLOCKED", "EXIT"}} {
		if g := results[id]; g == nil || g.State != want[0] || g.OutcomeClass != want[1] {
			t.Fatalf("%s: %+v", id, g)
		}
	}
	if results["fails"].ExitCode.Int() != 3 || *results["dirty"].PorcelainClean {
		t.Fatalf("fails %+v dirty %+v", results["fails"], results["dirty"])
	}
	auditOK(t, s.repo)
}

// TestCALV0016_GateRunRefusesWithoutRunning: an unsubmitted attempt, a dirty
// or moved worktree and an unknown gate are refused and record nothing.
func TestCALV0016_GateRunRefusesWithoutRunning(t *testing.T) {
	s := newGateStore(t)
	claim := s.claim(t, "claim-1", s.ticket(t, "one"), 0, "src/")
	before := storeDigest(t, s.repo)
	report, err := s.gate(t, "gate-0", gateOf(claim, "verify"), 1)
	if err != nil {
		t.Fatal(err)
	}
	refusedWith(t, report, mutation.OutcomeBlocked, wire.CodeTicketState)
	_, tree := s.commit(t, "src/a.go")
	s.lease(t, "submit-1", submitOf(claim, tree), 1, nil)
	before = storeDigest(t, s.repo)
	fixture.Write(t, filepath.Join(s.root, "stray"), []byte("x"))
	if _, err := s.gate(t, "gate-1", gateOf(claim, "verify"), 2); wire.CodeOf(err) != wire.CodeDirtyWorktree {
		t.Fatalf("dirty: %v", err)
	}
	if err := os.Remove(filepath.Join(s.root, "stray")); err != nil {
		t.Fatal(err)
	}
	s.commit(t, "src/b.go")
	if _, err := s.gate(t, "gate-2", gateOf(claim, "verify"), 3); wire.CodeOf(err) != wire.CodeStaleTree {
		t.Fatalf("moved: %v", err)
	}
	if _, err := s.gate(t, "gate-3", gateOf(claim, "absent"), 3); wire.CodeOf(err) != wire.CodeGateUnknown {
		t.Fatalf("unknown gate: %v", err)
	}
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused gate run wrote")
	}
}

// TestCALV0017_CompleteVerifiesTheTicket: a commit of the candidate on the
// intent branch whose required gates passed completes the ticket VERIFIED,
// ends the attempt COMPLETED and frees its reservation in one MANIFEST
// receipt.
func TestCALV0017_CompleteVerifiesTheTicket(t *testing.T) {
	s := newGateStore(t)
	id := s.ticket(t, "one")
	claim, commit := s.submitted(t, id, "src", 0)
	s.passes(t, "gate-1", gateOf(claim, "verify"), 2)
	report := s.lease(t, "complete-1", completeOf(claim, commit), 3, nil)
	if report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("complete: %+v", report)
	}
	a := s.attempt(t, claim.AttemptID)
	if a.Phase != "COMPLETED" || a.Quiescence != "FENCED" || a.ManifestSha256 == nil || len(s.entries(t)) != 0 {
		t.Fatalf("attempt: %+v", a)
	}
	rec := s.record(t, id)
	if rec.Status != ticket.StatusCompleted || rec.Completion == nil || rec.Completion.Kind != "VERIFIED" || *rec.Completion.ManifestSha256 != *a.ManifestSha256 || len(rec.Completion.Evidence) != 1 || string(rec.Completion.Evidence[0]) != a.GateResults[0] {
		t.Fatalf("ticket: %+v", rec)
	}
	m, err := snapshot.DecodeManifest(s.evidence(t, string(*a.ManifestSha256)))
	if err != nil || m.CandidateTreeOid != *a.CandidateTreeOid || len(m.GateResults) != 1 {
		t.Fatalf("manifest: %+v %v", m, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.repo.StateDir, "receipts", report.Receipt))
	if err != nil {
		t.Fatal(err)
	}
	if rc, err := snapshot.DecodeReceipt(raw); err != nil || rc.Kind != "MANIFEST" {
		t.Fatalf("receipt: %+v %v", rc, err)
	}
	if again := s.lease(t, "complete-1", completeOf(claim, commit), 4, nil); again.Kind != "Replay" {
		t.Fatalf("replay: %+v", again)
	}
	auditOK(t, s.repo)
}

// TestCALV0017_CompletionRefusals: each unmet §7.3 row refuses BLOCKED and
// writes nothing.
func TestCALV0017_CompletionRefusals(t *testing.T) {
	s := newGateStore(t)
	id := s.ticket(t, "one")
	claim, commit := s.submitted(t, id, "src", 0)
	tree := gitOut(t, s.root, "rev-parse", "HEAD^{tree}")
	dangling := gitOut(t, s.root, "commit-tree", tree, "-p", commit, "-m", "off-branch")
	before := storeDigest(t, s.repo)
	refuse := func(requestID, commit, code string) {
		t.Helper()
		refusedWith(t, s.lease(t, requestID, completeOf(claim, commit), 3, nil), mutation.OutcomeBlocked, code)
	}
	refuse("complete-missing", commit, wire.CodeMissingGate)
	refuse("complete-unreachable", dangling, wire.CodeStaleTree)
	refuse("complete-base", gitOut(t, s.root, "rev-parse", commit+"~1"), wire.CodeStaleTree)
	if storeDigest(t, s.repo) != before {
		t.Fatal("refused completion wrote")
	}
	s.passes(t, "gate-1", gateOf(claim, "verify"), 2)
	next, nextTree := s.commit(t, "src/b.go")
	s.lease(t, "submit-2", submitOf(claim, nextTree), 2, nil)
	refuse("complete-stale", next, wire.CodeGateStale)
	hold, err := store.Mutate(context.Background(), s.repo, operator(), envelope("hold-1", mutation.OpHold, id, "1", obj("holdId", str("h"), "reason", str("wait"))), s.at(t, 3))
	if err != nil || hold.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("hold: %+v %v", hold, err)
	}
	refuse("complete-held", next, wire.CodeTicketHeld)
}

// TestCALV0017_CompletionNeedsEveryRequiredGateAndApproval: a failed
// ticket-required gate refuses GATE_FAILED, and an APPROVAL_REQUIRED ticket
// without a COMPLETE grant refuses APPROVAL_MISSING.
func TestCALV0017_CompletionNeedsEveryRequiredGateAndApproval(t *testing.T) {
	s := newGateStore(t)
	ids := map[string]string{}
	for class, dir := range map[string]string{"APPROVAL_REQUIRED": "approve", "AUTONOMOUS": "plain"} {
		payload := createPayload(dir)
		payload.Obj.Set("requiredGates", wire.Strings([]string{"fails"}))
		payload.Obj.Set("executionClass", str(class))
		created := mutate(t, s.repo, envelope("create-"+dir, "CREATE", "", "", payload))
		if created.Outcome.Outcome != mutation.OutcomeCompleted {
			t.Fatalf("create %s: %+v", dir, created)
		}
		ids[dir] = created.Ticket
	}
	grant := obj("grantId", str("g"), "actor", str("tester"), "operation", str("RUN"), "targetRevision", str("1"), "scope", wire.Strings(nil))
	if r := mutate(t, s.repo, envelope("grant-run", mutation.OpGrantApproval, ids["approve"], "1", grant)); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("grant: %+v", r)
	}
	s.t0 = now(t)
	approve, approveCommit := s.submitted(t, ids["approve"], "approve", 0)
	refusedWith(t, s.lease(t, "complete-approve", completeOf(approve, approveCommit), 3, nil), mutation.OutcomeBlocked, wire.CodeApprovalMissing)
	plain, commit := s.submitted(t, ids["plain"], "plain", 4)
	s.passes(t, "gate-1", gateOf(plain, "verify"), 6)
	s.passes(t, "gate-2", gateOf(plain, "fails"), 6)
	refusedWith(t, s.lease(t, "complete-plain", completeOf(plain, commit), 7, nil), mutation.OutcomeBlocked, wire.CodeGateFailed)
}
