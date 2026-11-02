// SPDX-License-Identifier: Apache-2.0

package store

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
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// gateLeasePolicy widens repo's policy as poolLeasePolicy does and adds one
// required COMMAND gate `verify` that passes.
func gateLeasePolicy(t *testing.T, repo *intent.Repository) {
	t.Helper()
	v := fixture.PolicyValue()
	v.Obj.Set("policyVersion", wire.String("2"))
	v.Obj.Set("capacity", historyObject("maxActiveAttempts", wire.String("4"), "maxWorkersTotal", wire.String("4"), "classes", wire.Array()))
	budgets, _ := v.Obj.Get("budgets")
	budgets.Obj.Set("requireEnforcedFields", wire.Strings(nil))
	expected := wire.NewObject().Set("exitCode", wire.String("0")).Set("reducer", wire.Null())
	verify := wire.NewObject().Set("gateId", wire.String("verify")).Set("kind", wire.String("COMMAND")).
		Set("argv", wire.Strings([]string{"/bin/sh", "-c", "exit 0"})).Set("cwd", wire.String("WORKTREE")).Set("env", wire.Strings(nil)).
		Set("timeoutSeconds", wire.String("30")).Set("expected", wire.ObjectValue(expected)).Set("evidence", wire.Strings(nil)).
		Set("inputs", wire.Strings(nil)).Set("sharedResource", wire.Null()).Set("reusable", wire.Bool(false)).Set("required", wire.Bool(true))
	v.Obj.Set("gates", wire.Array(wire.ObjectValue(verify)))
	report, err := PolicyUpdate(context.Background(), repo, historyActor, PolicyRequest{QueueID: fixture.QueueID, RequestID: "gate-policy", ExpectedPolicyVersion: "1", Policy: wire.EncodeFile(v)}, WallClock())
	if err != nil || report.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("policy: %+v %v", report, err)
	}
}

// gatedStore is a writer-checkpoint store with the verify gate, one created
// ticket claimed from a Git checkout, and one commit of src/a.go on main:
// the row a gated batch close runs.
type gatedStore struct {
	repo         *intent.Repository
	root         string
	ticket       string
	claim        *Report
	commit, tree string
	now          wire.Timestamp
}

func newGatedStore(t *testing.T) *gatedStore {
	t.Helper()
	repo := writerStore(t, 70)
	gateLeasePolicy(t, repo)
	if run := writerMutate(t, repo, "gate-reseed", nil); !run.completed() {
		t.Fatalf("reseed: %+v %v", run.rep, run.err)
	}
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "worktree")
	holdGit(t, root, "init", "-q", "-b", "main")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "base")
	now := WallClock()
	created := stagedWrite(writerCreate(repo, "gate-ticket", now), nil)
	if !created.completed() {
		t.Fatalf("create: %+v %v", created.rep, created.err)
	}
	s := &gatedStore{repo: repo, root: root, ticket: created.rep.Ticket, now: now}
	claim := stagedWrite(s.lease("gate-claim", transaction.LeaseRequest{Verb: transaction.LeaseClaim, TicketID: s.ticket, Holder: "agent-1", LeaseMinutes: "60"}), nil)
	if !claim.completed() || !claim.fast() {
		t.Fatalf("claim: %+v %v %v", claim.rep, claim.err, claim.stages)
	}
	s.claim = claim.rep
	historyWrite(t, filepath.Join(root, "src", "a.go"), []byte("package src\n"))
	holdGit(t, root, "add", "-A")
	holdGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "change")
	s.commit, s.tree = holdGitOut(t, root, "rev-parse", "HEAD"), holdGitOut(t, root, "rev-parse", "HEAD^{tree}")
	return s
}

func (s *gatedStore) lease(id string, l transaction.LeaseRequest) func(context.Context) (*Report, error) {
	return func(ctx context.Context) (*Report, error) {
		return Lease(ctx, s.repo, historyActor, LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, Lease: l}, s.now)
	}
}

func (s *gatedStore) submit(id string) func(context.Context) (*Report, error) {
	return s.lease(id, transaction.LeaseRequest{Verb: transaction.LeaseSubmit, AttemptID: s.claim.AttemptID, Generation: s.claim.Generation, Tree: s.tree})
}

func (s *gatedStore) gateRun(id string) func(context.Context) (*Report, error) {
	return func(ctx context.Context) (*Report, error) {
		choice := LeaseChoice{QueueID: fixture.QueueID, RequestID: id, Root: s.root, Lease: transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: s.claim.AttemptID, Generation: s.claim.Generation, Gate: "verify"}}
		return GateRun(ctx, s.repo, historyActor, choice, s.root, time.Now)
	}
}

func (s *gatedStore) complete(id string) func(context.Context) (*Report, error) {
	return s.lease(id, transaction.LeaseRequest{Verb: transaction.LeaseComplete, AttemptID: s.claim.AttemptID, Generation: s.claim.Generation, Commit: s.commit})
}

func holdGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, args...)...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// auditConsistent requires the complete audit to find the store consistent
// with its projections.
func auditConsistent(t *testing.T, repo *intent.Repository) *journal.Result {
	t.Helper()
	head, err := writerGuards(repo, "MUTATE")
	if err != nil {
		t.Fatal(err)
	}
	proof, err := journalReader(repo, head).Audit()
	if err != nil || proof.StructuralConsistency != "CONSISTENT" || proof.ProjectionAgreement != "AGREES" {
		t.Fatalf("audit: %+v %v", proof, err)
	}
	return proof
}

// CAL-V0-970 (V1-4315): SUBMIT, GATE_RUN and COMPLETE are served by the
// writer-checkpoint route while the store holds a bound checkpoint, with the
// same bytes and report as the complete route for a committed submit and
// complete, and a gate run recorded on the route.
func TestCALV0970_GateVerbsTakeWriterRoute(t *testing.T) {
	s := newGatedStore(t)
	writerParity(t, s.repo, s.submit("gate-submit"))
	run := stagedWrite(s.gateRun("gate-run"), nil)
	if !run.completed() || !run.fast() || run.declined() != "" || run.rep.Kind != "Transaction" {
		t.Fatalf("gate run on the writer route: %+v %v %v", run.rep, run.err, run.stages)
	}
	// The gate run's receipt took the wall clock; the parity pair shares
	// one later reading.
	s.now = WallClock()
	writerParity(t, s.repo, s.complete("gate-complete"))
	proof := auditConsistent(t, s.repo)
	ticketPath := "intent/tickets/" + s.ticket[strings.LastIndex(s.ticket, ":")+1:] + ".json"
	raw, err := os.ReadFile(filepath.Join(s.repo.IntentRoot(), intent.Dir, "tickets", filepath.Base(ticketPath)))
	if err != nil || !strings.Contains(string(raw), `"status":"COMPLETED"`) {
		t.Fatalf("ticket after complete: %v %s", err, raw)
	}
	if proof.Head == nil || proof.Head.LastSeq.Uint64() == 0 {
		t.Fatalf("head: %+v", proof.Head)
	}
}

// indexAgedHere ages every ticket file of repo out of the racy window and
// writes the index, returning how many tickets it governs.
func indexAgedHere(t *testing.T, repo *intent.Repository) int {
	t.Helper()
	dir := filepath.Join(repo.IntentRoot(), intent.Dir, intent.TicketsDir)
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	for _, n := range names {
		if err := os.Chtimes(filepath.Join(dir, n.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := intent.WriteClosedIndex(repo.IntentRoot(), repo.StateDir); err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(filepath.Join(repo.IntentRoot(), intent.Dir, intent.PolicyFile))
	if err != nil {
		t.Fatal(err)
	}
	return len(intent.ReadClosedIndex(repo.StateDir).WriterSummaries(fixture.QueueID, wire.Sum(policy)))
}

// indexParity runs write on the writer route with an index governing every
// ticket, restores the store, runs it again with no index, and requires the
// same files and the same report (the index is a sibling of the state
// directory and is excluded).
func indexParity(t *testing.T, s *gatedStore, write func(context.Context) (*Report, error)) {
	t.Helper()
	parent := filepath.Dir(s.repo.PrimaryWorktree)
	wcPath, indexPath := journal.WriterCheckpointPath(s.repo.StateDir), intent.ClosedIndexPath(s.repo.StateDir)
	if held := indexAgedHere(t, s.repo); held == 0 {
		t.Fatal("the index governs no ticket")
	}
	backup := filepath.Join(t.TempDir(), "store")
	copyTree(t, parent, backup)
	indexed := stagedWrite(write, nil)
	if !indexed.completed() || !indexed.fast() {
		t.Fatalf("indexed: %+v %v %v", indexed.rep, indexed.err, indexed.stages)
	}
	indexedFiles := storeFiles(t, parent, wcPath)
	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	copyTree(t, backup, parent)
	if err := os.Remove(indexPath); err != nil {
		t.Fatal(err)
	}
	full := stagedWrite(write, nil)
	if !full.completed() || !full.fast() {
		t.Fatalf("full: %+v %v %v", full.rep, full.err, full.stages)
	}
	fullFiles := storeFiles(t, parent, wcPath)
	rel, _ := filepath.Rel(parent, indexPath)
	delete(indexedFiles, rel)
	delete(fullFiles, rel)
	for p, d := range fullFiles {
		if indexedFiles[p] != d {
			t.Errorf("%s: indexed %q, full %q", p, indexedFiles[p], d)
		}
	}
	for p := range indexedFiles {
		if _, ok := fullFiles[p]; !ok {
			t.Errorf("%s: only the indexed write wrote it", p)
		}
	}
	sameDecision(t, indexed, full)
	if t.Failed() {
		t.FailNow()
	}
}

// CAL-V0-971 and TCX-V0-970: the lease writer route models against the
// derived ticket index when it governs the queue and policy; a submit and a
// complete write the same bytes and report with and without it.
func TestCALV0971_LeaseWriterIndexParity(t *testing.T) {
	s := newGatedStore(t)
	indexParity(t, s, s.submit("index-submit"))
	run := stagedWrite(s.gateRun("index-gate"), nil)
	if !run.completed() {
		t.Fatalf("gate run: %+v %v", run.rep, run.err)
	}
	s.now = WallClock()
	indexParity(t, s, s.complete("index-complete"))
}

// CAL-V0-970 fail-closed: a ticket projection rewritten outside the journal
// (a tampered or stale intent file) still refuses the gate verb's write on
// both routes, the index present or not, and nothing is written.
func TestCALV0970_TamperedProjectionRefusesGateWrite(t *testing.T) {
	s := newGatedStore(t)
	if run := stagedWrite(s.submit("tamper-submit"), nil); !run.completed() {
		t.Fatalf("submit: %+v %v", run.rep, run.err)
	}
	if held := indexAgedHere(t, s.repo); held == 0 {
		t.Fatal("the index governs no ticket")
	}
	dir := filepath.Join(s.repo.IntentRoot(), intent.Dir, intent.TicketsDir)
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	target := s.ticket[strings.LastIndex(s.ticket, ":")+1:] + ".json"
	tampered := ""
	for _, n := range names {
		if n.Name() != target {
			tampered = filepath.Join(dir, n.Name())
			break
		}
	}
	raw, err := os.ReadFile(tampered)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw), `"title":"`, `"title":"tampered `, 1)
	if edited == string(raw) {
		t.Fatalf("no title to tamper in %s", tampered)
	}
	historyWrite(t, tampered, []byte(edited))
	before := headSeqOf(t, s.repo)
	for _, with := range []bool{true, false} {
		if !with {
			if err := os.Remove(intent.ClosedIndexPath(s.repo.StateDir)); err != nil {
				t.Fatal(err)
			}
		}
		run := stagedWrite(s.complete("tamper-complete"), nil)
		if run.err == nil && run.rep != nil && run.rep.Outcome.Outcome == mutation.OutcomeCompleted {
			t.Fatalf("index=%v: tampered projection admitted: %+v", with, run.rep)
		}
		if run.fast() {
			t.Fatalf("index=%v: the writer route committed over a tampered projection: %v", with, run.stages)
		}
		if headSeqOf(t, s.repo) != before {
			t.Fatalf("index=%v: a refused write advanced the head", with)
		}
	}
}

// CAL-V0-972: the gate pre-read's checkpoint-resumed audit observes the same
// head, identity, attempt, policy and ticket records as the complete read
// audit.
func TestCALV0972_CheckpointedPreReadMatchesCompleteAudit(t *testing.T) {
	s := newGatedStore(t)
	if run := stagedWrite(s.submit("pre-read-submit"), nil); !run.completed() || !run.fast() {
		t.Fatalf("submit: %+v %v %v", run.rep, run.err, run.stages)
	}
	if _, err := os.Stat(journal.CheckpointPath(s.repo.StateDir)); err != nil {
		t.Fatalf("no read checkpoint retained: %v", err)
	}
	ctx := context.Background()
	fast, err := checkpointedLeaseProof(ctx, s.repo)
	if err != nil {
		t.Fatal(err)
	}
	complete, err := readLeaseProof(ctx, s.repo)
	if err != nil {
		t.Fatal(err)
	}
	// The inventory fingerprint names the listing each audit made (a resumed
	// audit lists the receipt names it walks); the head and the intent tree
	// are the observation both bind.
	if fast.Identity.HeadSha256 != complete.Identity.HeadSha256 || fast.Identity.IntentTreeSha256 != complete.Identity.IntentTreeSha256 || fast.Head.LastSeq != complete.Head.LastSeq {
		t.Fatalf("identity %+v vs %+v", fast.Identity, complete.Identity)
	}
	ticketPath := "intent/tickets/" + s.ticket[strings.LastIndex(s.ticket, ":")+1:] + ".json"
	for _, p := range []string{"attempts/" + s.claim.AttemptID + ".json", "intent/policy.json", "intent/queue.json", ticketPath} {
		a, b := fast.Records[p], complete.Records[p]
		if a.Raw == nil || b.Raw == nil || string(a.Raw) != string(b.Raw) {
			t.Fatalf("%s: checkpointed %d bytes, complete %d bytes", p, len(a.Raw), len(b.Raw))
		}
	}
	a, err := snapshot.DecodeAttempt(fast.Records["attempts/"+s.claim.AttemptID+".json"].Raw)
	if err != nil || a.Phase != "BUILT" {
		t.Fatalf("attempt from the pre-read: %+v %v", a, err)
	}
}
