package criterionexperiment

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
)

// Fixture captures exercise offline replay, not provenance authentication. The
// separate live demo exercises the audited native nonfixture read/write APIs.
func fixtureCaptures(t *testing.T, base string) (Captures, *ticket.Record, *snapshot.Attempt) {
	t.Helper()
	rec := fixture.Ticket("AT-0001")
	rec.AcceptanceCriteria = []string{"accepted criterion"}
	d := tw.Sum(fixture.PolicyBytes())
	budget := map[string]snapshot.BudgetField{}
	for _, name := range intent.LaneBudgetNames {
		budget[name] = snapshot.BudgetField{State: "NOT_OBSERVED"}
	}
	a := &snapshot.Attempt{AttemptID: "attempt:acme:main:" + strings.Repeat("a", 32), TicketID: rec.TicketID, TicketRevision: rec.AcceptanceRevision, TicketRecordSha256: rec.FileDigest(), Generation: "1", Phase: "RUNNING", PhaseSinceSeq: "1", Mode: "DEVELOPMENT", PolicySha256: d, ConfigSha256: d, RuntimeID: snapshot.RuntimeExternalAgent, CapabilityProfileSha256: d, BaseCommit: base, Branch: "main", Quiescence: "UNPROVED", SpawnNoExecCount: "0", RetryCount: "0", RepairRound: "0", Budget: budget, ScopeCheck: "UNKNOWN", Lease: &snapshot.Lease{Holder: "test", GrantedSeq: "1", ExpiresAt: "2026-01-01T00:00:00Z"}, Scope: &snapshot.Scope{Source: "REQUESTED", Resources: []ticket.Resource{{Class: "PATH", Key: "sample"}}}}
	return encodeCaptures(t, rec, a, "NONE"), rec, a
}
func encodeCaptures(t *testing.T, rec *ticket.Record, a *snapshot.Attempt, barrier string) Captures {
	t.Helper()
	d := tw.Digest(strings.Repeat("b", 64))
	seq := tw.Size("1")
	snap := &tw.Snapshot{HeadSeq: &seq, HeadReceiptSha256: &d, IntentTreeSha256: &d, PrimaryWorktreeSha256: &d}
	wrap := func(cmd []string, item tw.Value) string {
		r := tw.Result{Command: cmd, Outcome: tw.OutcomeOK, Snapshot: snap, Items: []tw.Value{item}}
		b, e := r.Encode()
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	raw, e := a.Encode()
	if e != nil {
		t.Fatal(e)
	}
	av, e := tw.Parse(raw)
	if e != nil {
		t.Fatal(e)
	}
	policy, e := intent.DecodePolicy(fixture.PolicyBytes())
	if e != nil {
		t.Fatal(e)
	}
	q := tw.ObjectValue(tw.NewObject().Set("policySha256", tw.String(string(policy.PolicySha256()))).Set("barrier", tw.Null()).Set("writeBarrier", tw.String(barrier)).Set("executionCutover", tw.Bool(true)))
	return Captures{Ticket: wrap([]string{"ticket", "show"}, tw.ObjectValue(tw.NewObject().Set("record", rec.Value()))), Queue: wrap([]string{"queue", "status"}, q), Attempt: wrap([]string{"attempt", "show"}, av), Policy: string(policy.Raw)}
}

// CEX-V0-002 and CEX-V0-008: exact native acceptance/configuration, coherent
// outer snapshot, barriers, generation, phase, candidate and lease comparisons.
func TestNativeCapturedBindingsAndLiveStaleness(t *testing.T) {
	base := strings.Repeat("a", 40)
	c, rec, a := fixtureCaptures(t, base)
	b, _, _, e := authorityBinding(c, strings.Repeat("b", 40))
	if e != nil {
		t.Fatal(e)
	}
	policy, e := intent.DecodePolicy([]byte(c.Policy))
	if e != nil {
		t.Fatal(e)
	}
	if string(policy.PolicySha256()) == b.PolicySha256 {
		t.Fatal("fixture failed to distinguish raw and content policy digests")
	}
	p := Plan{Request: Request{Base: base}, Binding: b}
	a.Phase = "BUILT"
	tree := b.CandidateTree
	a.CandidateTreeOid = &tree
	a.Lease.ExpiresAt = "2099-01-01T00:00:00Z"
	good := encodeCaptures(t, rec, a, "NONE")
	if e = liveBinding(good, p, time.Now()); e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*ticket.Record, *snapshot.Attempt){
		"acceptance": func(r *ticket.Record, a *snapshot.Attempt) { r.AcceptanceCriteria = []string{"changed"} }, "revision": func(r *ticket.Record, a *snapshot.Attempt) { r.Revision = "2"; r.AcceptanceRevision = "2" }, "generation": func(r *ticket.Record, a *snapshot.Attempt) { a.Generation = "2" }, "policy": func(r *ticket.Record, a *snapshot.Attempt) {
			a.PolicySha256 = tw.Digest(strings.Repeat("c", 64))
			a.ConfigSha256 = a.PolicySha256
		}, "config": func(r *ticket.Record, a *snapshot.Attempt) { a.ConfigSha256 = tw.Digest(strings.Repeat("c", 64)) }, "candidate": func(r *ticket.Record, a *snapshot.Attempt) { s := strings.Repeat("d", 40); a.CandidateTreeOid = &s }, "phase": func(r *ticket.Record, a *snapshot.Attempt) { a.Phase = "RUNNING" }, "expired": func(r *ticket.Record, a *snapshot.Attempt) { a.Lease.ExpiresAt = "2020-01-01T00:00:00Z" },
	} {
		t.Run(name, func(t *testing.T) {
			_, r, a := fixtureCaptures(t, base)
			a.Phase = "BUILT"
			tree := b.CandidateTree
			a.CandidateTreeOid = &tree
			a.Lease.ExpiresAt = "2099-01-01T00:00:00Z"
			mutate(r, a)
			if liveBinding(encodeCaptures(t, r, a, "NONE"), p, time.Now()) == nil {
				t.Fatal("stale live binding admitted")
			}
		})
	}
	if _, _, _, e = encodeCaptures(t, rec, a, "HOLD").records(); e == nil {
		t.Fatal("barrier admitted")
	}
	moved := good
	moved.Queue = strings.Replace(moved.Queue, `"headSeq":"1"`, `"headSeq":"2"`, 1)
	if _, _, _, e = moved.records(); e == nil {
		t.Fatal("incoherent reads admitted")
	}
	pending := good
	pending.Queue = strings.Replace(pending.Queue, `"pendingRedo":false`, `"pendingRedo":true`, 1)
	if _, _, _, e = pending.records(); e == nil {
		t.Fatal("pending redo admitted")
	}
}

// CEX-V0-003, CEX-V0-006, CEX-V0-007, CEX-V0-009: full immutable CEM/source
// replay, no live task or Go executable dependency, tamper and survivor refusal.
func TestHistoricalVerifyArtifactsAndSurvivors(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	gitTest(t, dir, "init", "-q")
	_ = os.Mkdir(filepath.Join(dir, "sample"), 0700)
	sources := map[string]string{"sample/go.mod": "module example.test/sample\ngo 1.27.1\n", "sample/source.go": "package sample\nconst Value=0\n", "sample/oracle_test.go": "package sample\nimport \"testing\"\nfunc TestRepair(t *testing.T){if Value!=1{t.Fatal(\"EXPECTED\")}}\n", "contract.txt": "accepted criterion\n"}
	for p, s := range sources {
		if e := os.WriteFile(filepath.Join(dir, p), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "base")
	base := gitTest(t, dir, "rev-parse", "HEAD")
	_ = os.WriteFile(filepath.Join(dir, "sample/source.go"), []byte("package sample\nconst Value=1\n"), 0600)
	gitTest(t, dir, "add", ".")
	gitTest(t, dir, "commit", "-qm", "candidate")
	target := gitTest(t, dir, "rev-parse", "HEAD")
	s, e := workflow.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Prepare(ctx, workflow.PrepareOptions{Base: base, Target: target}); e != nil {
		t.Fatal(e)
	}
	cem, e := os.ReadFile(filepath.Join(dir, cw.ExcludedCEMPath))
	if e != nil {
		t.Fatal(e)
	}
	m, e := cw.ParseMap(cem)
	if e != nil {
		t.Fatal(e)
	}
	captures, rec, a := fixtureCaptures(t, base)
	repo, e := gitauth.Open(dir, gitrun.NewDefaultBudget())
	if e != nil {
		t.Fatal(e)
	}
	tree, e := repo.CommitTree(ctx, target)
	if e != nil {
		t.Fatal(e)
	}
	binding, _, _, e := authorityBinding(captures, tree)
	if e != nil {
		t.Fatal(e)
	}
	r := goRequest(t)
	r.Base = base
	r.Target = target
	r.Ticket = rec.TicketID.Raw
	r.Attempt = a.AttemptID
	r.Criteria[0].AcceptanceSha256 = Digest([]byte(rec.AcceptanceCriteria[0]))
	r.Criteria[0].Oracle.Commit = base
	r.Criteria[0].Authority.AnchorCommit = base
	r.Criteria[0].Authority.AnchorSha256 = Digest([]byte(sources["contract.txt"]))
	r.Criteria[0].Hunks = []string{m.Hunks[0].ID}
	r.Criteria[0].Controls = []string{base}
	binaryLink := filepath.Join(t.TempDir(), "go")
	if e = os.Symlink(r.GoBinary, binaryLink); e != nil {
		t.Fatal(e)
	}
	r.GoBinary = binaryLink
	p := Plan{Profile, r, binding, Digest(cem), CanonicalDigest(captures)}
	planDir := t.TempDir()
	for name, b := range map[string][]byte{"plan.json": Encode(p), "cem.json": cem, "captures.json": Encode(captures)} {
		if e = save(planDir, name, b); e != nil {
			t.Fatal(e)
		}
	}
	out := filepath.Join(t.TempDir(), "run")
	planPath := filepath.Join(planDir, "plan.json")
	receiptPath := filepath.Join(out, "receipt.json")
	receipt, e := Run(ctx, dir, planPath, CanonicalDigest(p), out, true, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(binaryLink); e != nil {
		t.Fatal(e)
	}
	summary, e := Verify(ctx, dir, planPath, receiptPath, false)
	if e != nil || summary.State != "VERIFIED_SATISFIED" || summary.RegisteredControls != 1 || summary.KilledControls != 1 {
		t.Fatalf("historical verify %+v %v", summary, e)
	}
	if _, e = Verify(ctx, dir, planPath, receiptPath, true); e == nil {
		t.Fatal("live gate admitted absent Tasks store")
	}
	rawPath := filepath.Join(out, "stdout-002.jsonl")
	original, e := os.ReadFile(rawPath)
	if e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(rawPath, []byte("tampered"), 0600)
	if _, e = Verify(ctx, dir, planPath, receiptPath, false); e == nil {
		t.Fatal("tampered raw output admitted")
	}
	_ = os.WriteFile(rawPath, original, 0600)
	// A caller-reported survivor is an integrity-valid but unresolved experiment.
	survivor := events("pass", "")
	receipt.Scenarios[2].StdoutSha256 = Digest(survivor)
	receipt.Scenarios[2].ExitCode = 0
	receipt.Scenarios[2].Classification = "pass"
	_ = os.WriteFile(rawPath, survivor, 0600)
	_ = save(out, "receipt.json", Encode(receipt))
	summary, e = Verify(ctx, dir, planPath, receiptPath, false)
	if e != nil || summary.State != "VERIFIED_UNRESOLVED" || summary.ExecutedControls != 1 || summary.KilledControls != 0 {
		t.Fatalf("survivor %+v %v", summary, e)
	}
	// Removing a requested row cannot improve the denominator.
	receipt.Scenarios = receipt.Scenarios[:2]
	_ = save(out, "receipt.json", Encode(receipt))
	if _, e = Verify(ctx, dir, planPath, receiptPath, false); e == nil {
		t.Fatal("missing control row admitted")
	}
}
