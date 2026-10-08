//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0197_RunRoleSelectsAnsweredWaitOfItsStage proves `run --role`
// selects an answered WAITING attempt only for the role of its stage
// (V1-0827): an answered integrate-stage wait is not picked by an implementer
// run, which leaves the attempt, its session and the program untouched, and
// is picked and resumed by an integrator run under its recorded grant.
func TestCALV0197_RunRoleSelectsAnsweredWaitOfItsStage(t *testing.T) {
	f := newContinuationFixture(t, continuationOptions{continuations: "1"})
	f.config.OwnIntegrationCheckout = true
	w := f.open(t)
	ctx := context.Background()
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	a, err := w.RunRole(ctx, "reviewer", "")
	if err != nil || a.Phase != "READY_FOR_INTEGRATION" {
		t.Fatalf("review: %+v %v", a, err)
	}
	rec := f.s.record(t, f.ticketID)
	grant := obj("grantId", str("g"), "actor", str("tester"), "operation", str("INTEGRATE"), "targetRevision", str(string(rec.AcceptanceRevision)), "scope", wire.Strings([]string{transaction.IntegrationScope(a.BaseCommit, *a.CandidateTreeOid, "main")}))
	if r := mutate(t, f.s.repo, envelope("grant-integrate", mutation.OpGrantApproval, f.ticketID, string(rec.Revision), grant)); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("grant: %+v", r)
	}
	base := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD")
	f.touch(t, "stall-integrate", "stuck")
	if _, err = w.RunRole(ctx, "integrator", "g"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded integrate continuation error %v", err)
	}
	f.assertWaitingCheckpoint(t, a.AttemptID, "integrate-session", 4)
	if err = os.Remove(filepath.Join(f.scripts, "stuck")); err != nil {
		t.Fatal(err)
	}
	if err = f.open(t).Resume(true); err != nil {
		t.Fatalf("operator retry: %v", err)
	}
	waiting := f.s.attempt(t, a.AttemptID)
	if waiting.Phase != "WAITING" || waiting.Stage != "integrate" || waiting.Supervision.Answer == "" {
		t.Fatalf("answered wait phase %s stage %s answer %q", waiting.Phase, waiting.Stage, waiting.Supervision.Answer)
	}
	before := f.program(t)

	raw, err := json.Marshal(f.config)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(fixture.TempDirOutside(t), "config.json")
	if err = os.WriteFile(config, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_TASKS_ACTOR", "tester")
	run := func(role string) *wire.Result {
		t.Helper()
		var out bytes.Buffer
		cli.Run(cli.Env{Cwd: f.s.repo.PrimaryWorktree, Args: []string{"run", "--program", "program", "--config", config, "--role", role}, Stdout: &out})
		res, err := wire.DecodeResult(out.Bytes())
		if err != nil {
			t.Fatalf("run --role %s: %v\n%s", role, err, out.Bytes())
		}
		return res
	}

	if res := run("implementer"); res.Outcome != wire.OutcomeOK || len(res.Items) != 0 {
		t.Fatalf("implementer run on an answered integrate wait: %+v", res)
	}
	after := f.s.attempt(t, a.AttemptID)
	if after.Phase != "WAITING" || after.Stage != "integrate" || after.Generation != waiting.Generation || after.Supervision.Answer != waiting.Supervision.Answer || after.Supervision.Turns != waiting.Supervision.Turns {
		t.Fatalf("implementer run moved the integrate wait: phase %s stage %s turns %d", after.Phase, after.Stage, after.Supervision.Turns.Int())
	}
	if p := f.program(t); p.Phase != before.Phase || p.OwnerReleased != before.OwnerReleased || p.Worktree != before.Worktree {
		t.Fatalf("implementer run changed the program: %s/%v, want %s/%v", p.Phase, p.OwnerReleased, before.Phase, before.OwnerReleased)
	}
	if args := f.lines(t, "resume-args"); len(args) != 1 {
		t.Fatalf("implementer run launched a host resume: %q", args)
	}

	if res := run("integrator"); res.Outcome != wire.OutcomeOK || len(res.Items) != 1 {
		t.Fatalf("integrator run on its answered wait: %+v", res)
	}
	if stored := f.s.attempt(t, a.AttemptID); stored.Phase != "COMPLETED" {
		t.Fatalf("attempt phase %s, want COMPLETED", stored.Phase)
	}
	if n := multiGit(t, f.s.repo.PrimaryWorktree, "rev-list", "--count", base+"..HEAD"); n != "1" {
		t.Fatalf("queue checkout gained %s commits, want exactly 1", n)
	}
}

// TestCALV0197_RoleSelectsByStage checks the shared role selection for every
// stage over an answered, an unanswered and an unsupervised wait and the
// unchanged per-role phases.
func TestCALV0197_RoleSelectsByStage(t *testing.T) {
	stages := []string{"implement", "review", "integrate"}
	for _, want := range stages {
		for _, at := range stages {
			answered := &snapshot.Attempt{Phase: "WAITING", Stage: at, Supervision: &snapshot.Supervision{Answer: "go on"}}
			if got := store.RoleSelects(want, answered); got != (want == at) {
				t.Errorf("role for %s selects an answered %s wait: %v", want, at, got)
			}
			for _, a := range []*snapshot.Attempt{
				{Phase: "WAITING", Stage: at, Supervision: &snapshot.Supervision{}},
				{Phase: "WAITING", Stage: at},
			} {
				if store.RoleSelects(want, a) {
					t.Errorf("role for %s selects an unanswered %s wait", want, at)
				}
			}
		}
	}
	for stage, phases := range map[string][]string{"implement": {"ADMITTED", "RETURNED", "COMPLETED"}, "review": {"BUILT"}, "integrate": {"READY_FOR_INTEGRATION"}} {
		for _, phase := range phases {
			if !store.RoleSelects(stage, &snapshot.Attempt{Phase: phase}) {
				t.Errorf("role for %s does not select %s", stage, phase)
			}
		}
	}
	if store.RoleSelects("review", &snapshot.Attempt{Phase: "READY_FOR_INTEGRATION"}) || store.RoleSelects("integrate", &snapshot.Attempt{Phase: "BUILT"}) || store.RoleSelects("", &snapshot.Attempt{Phase: "COMPLETED"}) {
		t.Error("a role selects another role's phase")
	}
}
