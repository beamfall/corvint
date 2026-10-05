//go:build darwin || linux

package criterionexperiment

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	cw "github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/workflow"
	tw "github.com/Beamfall/corvint/internal/tasks/wire"
)

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
	tasks := testTasks(t)
	captures := fixtureCaptures(t, base, tasks)
	rec := captures.Verification.Binding
	a := rec
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
	p := Plan{Profile, r, binding, Digest(cem), Digest(tw.EncodeFile(captures.Capture.Value())), captures.Verification.Verifier}
	planDir := t.TempDir()
	for name, b := range map[string][]byte{"plan.json": Encode(p), "cem.json": cem, "captures.json": tw.EncodeFile(captures.Capture.Value())} {
		if e = save(planDir, name, b); e != nil {
			t.Fatal(e)
		}
	}
	out := filepath.Join(t.TempDir(), "run")
	planPath := filepath.Join(planDir, "plan.json")
	receiptPath := filepath.Join(out, "receipt.json")
	receipt, e := Run(ctx, dir, planPath, CanonicalDigest(p), out, true, true, tasks)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(binaryLink); e != nil {
		t.Fatal(e)
	}
	summary, e := Verify(ctx, dir, planPath, receiptPath, false, tasks)
	if e != nil || summary.State != "VERIFIED_SATISFIED" || summary.RegisteredControls != 1 || summary.KilledControls != 1 {
		t.Fatalf("historical verify %+v %v", summary, e)
	}
	if _, e = Verify(ctx, dir, planPath, receiptPath, true, tasks); e == nil {
		t.Fatal("live gate admitted absent Tasks store")
	}
	rawPath := filepath.Join(out, "stdout-002.jsonl")
	original, e := os.ReadFile(rawPath)
	if e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(rawPath, []byte("tampered"), 0600)
	if _, e = Verify(ctx, dir, planPath, receiptPath, false, tasks); e == nil {
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
	summary, e = Verify(ctx, dir, planPath, receiptPath, false, tasks)
	if e != nil || summary.State != "VERIFIED_UNRESOLVED" || summary.ExecutedControls != 1 || summary.KilledControls != 0 {
		t.Fatalf("survivor %+v %v", summary, e)
	}
	// Removing a requested row cannot improve the denominator.
	receipt.Scenarios = receipt.Scenarios[:2]
	_ = save(out, "receipt.json", Encode(receipt))
	if _, e = Verify(ctx, dir, planPath, receiptPath, false, tasks); e == nil {
		t.Fatal("missing control row admitted")
	}
}

func testTasks(t *testing.T) TasksVerifierConfig {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "tasks")
	cmd := exec.Command("go", "build", "-o", exe, "../../cmd/corvint-tasks")
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build Tasks: %v %s", e, b)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	return TasksVerifierConfig{exe, Digest(b)}
}
func fixtureCaptures(t *testing.T, base string, cfg TasksVerifierConfig) Captures {
	t.Helper()
	raw, e := os.ReadFile("testdata/capture.json")
	if e != nil {
		t.Fatal(e)
	}
	c, e := tw.DecodeCriterionCapture(raw)
	if e != nil {
		t.Fatal(e)
	}
	c.Attempt = strings.ReplaceAll(c.Attempt, strings.Repeat("a", 40), base)
	c.Producer.ExecutableSHA256 = tw.Digest(cfg.SHA256)
	first, e := verifyCapture(context.Background(), tw.EncodeFile(c.Value()), cfg)
	if e != nil {
		t.Fatal(e)
	}
	c.Producer = first.Verification.Verifier
	out, e := verifyCapture(context.Background(), tw.EncodeFile(c.Value()), cfg)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

// CEX-V0-008: live applicability is checked separately from native historical parsing.
func TestLiveProjectionStaleness(t *testing.T) {
	tree := strings.Repeat("b", 40)
	a := tw.CriterionBinding{TicketID: tw.TicketID{Raw: "acme:main:AT-0001"}, AcceptanceRevision: "1", AcceptanceCriteria: []string{"criterion"}, AttemptID: "attempt", Generation: "1", Phase: "BUILT", BaseCommit: strings.Repeat("a", 40), CandidateTreeOID: &tree, PolicySHA256: tw.Digest(strings.Repeat("c", 64)), ConfigSHA256: tw.Digest(strings.Repeat("c", 64)), LeaseExpiresAt: "2099-01-01T00:00:00Z"}
	c := Captures{Verification: tw.CriterionVerification{Binding: a}}
	b, _, _, _ := authorityBinding(c, tree)
	p := Plan{Binding: b, Request: Request{Base: a.BaseCommit}}
	if e := liveBinding(c, p, time.Now()); e != nil {
		t.Fatal(e)
	}
	for name, mutate := range map[string]func(*tw.CriterionBinding){"acceptance": func(a *tw.CriterionBinding) { a.AcceptanceCriteria = []string{"changed"} }, "revision": func(a *tw.CriterionBinding) { a.AcceptanceRevision = "2" }, "generation": func(a *tw.CriterionBinding) { a.Generation = "2" }, "candidate": func(a *tw.CriterionBinding) { a.CandidateTreeOID = nil }, "phase": func(a *tw.CriterionBinding) { a.Phase = "RUNNING" }, "expiry": func(a *tw.CriterionBinding) { a.LeaseExpiresAt = "2000-01-01T00:00:00Z" }, "base": func(a *tw.CriterionBinding) { a.BaseCommit = tree }} {
		t.Run(name, func(t *testing.T) {
			bad := c
			mutate(&bad.Verification.Binding)
			if e := liveBinding(bad, p, time.Now()); e == nil {
				t.Fatal("stale live projection admitted")
			}
		})
	}
}
