//go:build darwin || linux

package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// designatedFixture is a multi-repository fixture with one required gate
// whose program owns the queue checkout and designates the docs checkout on
// its main branch as that repository's integration target (CAL-V0-087).
func designatedFixture(t *testing.T) *multiFixture {
	t.Helper()
	f := buildProgramFixture(t, true, true, []string{"verify"})
	f.config.OwnIntegrationCheckout = true
	f.config.Repositories[0].IntegrationBranch = "main"
	return f
}

// openProgram returns a function that opens (or, called again, reopens as a
// restarted supervisor would) the fixture's program.
func openProgram(t *testing.T, f *multiFixture) func() *store.Workflow {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return func() *store.Workflow {
		t.Helper()
		w, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", self, f.config, f.ticketID)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		return w
	}
}

// reviewedProgram runs implement and review, with every required gate, to
// READY_FOR_INTEGRATION.
func reviewedProgram(t *testing.T, w *store.Workflow) *snapshot.Attempt {
	t.Helper()
	ctx := context.Background()
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	a, err := w.RunRole(ctx, "reviewer", "")
	if err != nil || a.Phase != "READY_FOR_INTEGRATION" {
		t.Fatalf("review: %+v %v", a, err)
	}
	return a
}

// programRecord reads the fixture program's record.
func programRecord(t *testing.T, f *multiFixture) snapshot.Program {
	t.Helper()
	entries, err := store.ProgramRecords(context.Background(), f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range entries {
		if p.ID == "program" {
			return p
		}
	}
	t.Fatal("program record absent")
	return snapshot.Program{}
}

// grantIntegration records INTEGRATE grant "g" with one exact scope.
func grantIntegration(t *testing.T, f *multiFixture, scope string) {
	t.Helper()
	rec := f.s.record(t, f.ticketID)
	grant := obj("grantId", str("g"), "actor", str("tester"), "operation", str("INTEGRATE"), "targetRevision", str(string(rec.AcceptanceRevision)), "scope", wire.Strings([]string{scope}))
	if r := mutate(t, f.s.repo, envelope("grant-integrate", mutation.OpGrantApproval, f.ticketID, string(rec.Revision), grant)); r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("grant: %+v", r)
	}
}

// programScope is the exact multi-repository grant scope of the attempt.
func programScope(t *testing.T, f *multiFixture, a *snapshot.Attempt) string {
	t.Helper()
	return transaction.ProgramIntegrationScope(a.BaseCommit, *a.CandidateTreeOid, "main", programRecord(t, f).Repositories)
}

// assertLanded checks that the queue checkout is at the program candidate and
// the docs checkout, still on main, gained exactly its one candidate commit.
func assertLanded(t *testing.T, f *multiFixture, a *snapshot.Attempt, docsBase string) {
	t.Helper()
	p := programRecord(t, f)
	if head := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD"); head != p.CandidateCommit {
		t.Fatalf("queue HEAD %s, want candidate %s", head, p.CandidateCommit)
	}
	if body := multiGit(t, f.s.repo.PrimaryWorktree, "show", "HEAD:hello.txt"); body != "changed" {
		t.Fatalf("queue hello.txt %q", body)
	}
	docs := p.Repositories[0]
	if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != docs.Candidate || docs.Candidate == docsBase {
		t.Fatalf("docs HEAD %s, want candidate %s (base %s)", head, docs.Candidate, docsBase)
	}
	if n := multiGit(t, f.extra, "rev-list", "--count", docsBase+"..HEAD"); n != "1" {
		t.Fatalf("docs gained %s commits, want exactly 1", n)
	}
	if branch := multiGit(t, f.extra, "symbolic-ref", "HEAD"); branch != "refs/heads/main" {
		t.Fatalf("docs branch %s", branch)
	}
	if body := multiGit(t, f.extra, "show", "HEAD:note.txt"); body != "changed" {
		t.Fatalf("docs note.txt %q", body)
	}
	if stored := f.s.attempt(t, a.AttemptID); stored.Phase != "COMPLETED" {
		t.Fatalf("attempt phase %s, want COMPLETED", stored.Phase)
	}
}

// TestCALV0087_DesignatedMultiRepositoryIntegration drives a two-repository
// program end to end: the implement prompt carries the docs Core packet
// (CAL-V0-088); the required gate runs at, and records, the composite
// candidate; the grant names the composite and every designated target; and
// integration lands each candidate only in its designated checkout before
// native completion (CAL-V0-087).
func TestCALV0087_DesignatedMultiRepositoryIntegration(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	docsBase := multiGit(t, f.extra, "rev-parse", "HEAD")
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	prompt, err := os.ReadFile(filepath.Join(f.scripts, "implement-prompt"))
	if err != nil || !strings.Contains(string(prompt), `"repositoryContext":{"docs":{"ok":true`) {
		t.Fatalf("implement prompt lacks the docs context packet: %v", err)
	}
	if len(a.GateResults) != 1 {
		t.Fatalf("gate results %d, want 1", len(a.GateResults))
	}
	raw, err := os.ReadFile(filepath.Join(f.s.repo.StateDir, "evidence", string(a.GateResults[0])))
	if err != nil {
		t.Fatal(err)
	}
	g, err := snapshot.DecodeGateResult(raw)
	if err != nil {
		t.Fatal(err)
	}
	if g.State != "PASSED" || g.ExecutedTreeOid == nil || *g.ExecutedTreeOid != *a.CandidateTreeOid || g.CandidateTreeOid != *a.CandidateTreeOid {
		t.Fatalf("gate %s executed %v, want the composite candidate %s", g.State, g.ExecutedTreeOid, *a.CandidateTreeOid)
	}
	grantIntegration(t, f, programScope(t, f, a))
	if _, err = w.RunRole(context.Background(), "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	assertLanded(t, f, a, docsBase)
}

// TestCALV0087_ExtraWorktreeCleanup proves cleanup after native completion
// removes every registered stage worktree, including each extra repository's
// sibling, leaving each repository with only its own checkout registered.
func TestCALV0087_ExtraWorktreeCleanup(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	grantIntegration(t, f, programScope(t, f, a))
	if _, err := w.RunRole(context.Background(), "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	before := programRecord(t, f).Worktrees
	siblings := 0
	for _, r := range before {
		if r.Repository == "docs" {
			siblings++
		}
	}
	if siblings == 0 {
		t.Fatal("no extra repository worktree was registered")
	}
	if err := w.CleanupWorktrees(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	for _, r := range programRecord(t, f).Worktrees {
		if !r.Removed {
			t.Fatalf("worktree %s not removed", r.Path)
		}
		if _, err := os.Lstat(r.Path); !os.IsNotExist(err) {
			t.Fatalf("worktree %s still exists: %v", r.Path, err)
		}
	}
	for _, repo := range []string{f.s.repo.PrimaryWorktree, f.extra} {
		if list := multiGit(t, repo, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
			t.Fatalf("%s keeps stage worktrees:\n%s", repo, list)
		}
	}
}

// TestCALV0087_GrantMustNameEveryTarget proves a grant whose scope is the
// single-repository formula, which omits the extra repository's designated
// target, is refused before any checkout moves.
func TestCALV0087_GrantMustNameEveryTarget(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	docsBase := multiGit(t, f.extra, "rev-parse", "HEAD")
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	grantIntegration(t, f, transaction.IntegrationScope(a.BaseCommit, *a.CandidateTreeOid, "main"))
	_, err := w.RunRole(context.Background(), "integrator", "g")
	if err == nil || !strings.Contains(err.Error(), string(wire.CodeApprovalMissing)) {
		t.Fatalf("partial grant scope not refused: %v", err)
	}
	if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != docsBase {
		t.Fatal("docs checkout moved under a partial grant")
	}
	if head := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD"); head != a.BaseCommit {
		t.Fatal("queue checkout moved under a partial grant")
	}
}

// TestCALV0087_DesignatedTargetChecked proves a designated extra checkout
// that advanced past its base, or was switched to another branch, after
// review is refused before a grant is recorded or any checkout moves.
func TestCALV0087_DesignatedTargetChecked(t *testing.T) {
	t.Parallel()
	for want, change := range map[string]func(t *testing.T, docs string){
		"TARGET_ADVANCED: repository docs advanced": func(t *testing.T, docs string) {
			if err := os.WriteFile(filepath.Join(docs, "other.txt"), []byte("other\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			multiGit(t, docs, "add", "other.txt")
			multiGit(t, docs, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "advance")
		},
		"designated checkout branch differs": func(t *testing.T, docs string) {
			multiGit(t, docs, "switch", "-q", "-c", "other")
		},
	} {
		t.Run(strings.Fields(want)[0], func(t *testing.T) {
			f := designatedFixture(t)
			w := openProgram(t, f)()
			a := reviewedProgram(t, w)
			grantIntegration(t, f, programScope(t, f, a))
			change(t, f.extra)
			docsHead := multiGit(t, f.extra, "rev-parse", "HEAD")
			_, err := w.RunRole(context.Background(), "integrator", "g")
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want %q, got %v", want, err)
			}
			if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != docsHead {
				t.Fatal("docs checkout moved")
			}
			if head := multiGit(t, f.s.repo.PrimaryWorktree, "rev-parse", "HEAD"); head != a.BaseCommit {
				t.Fatal("queue checkout moved")
			}
			if stored := f.s.attempt(t, a.AttemptID); stored.Supervision.IntegrationGrant != "" || len(stored.PendingEffects) != 0 {
				t.Fatalf("refusal recorded grant %q effects %v", stored.Supervision.IntegrationGrant, stored.PendingEffects)
			}
		})
	}
}

// TestCALV0087_InterruptedIntegrationLandsOnce interrupts integration after
// the docs repository landed and before the queue repository did, and after
// the queue repository landed and before the outcome was recorded; a
// restarted supervisor finishes the integration and each repository gains
// its candidate exactly once.
func TestCALV0087_InterruptedIntegrationLandsOnce(t *testing.T) {
	t.Parallel()
	for _, point := range []string{"integrate-repo:docs", "transition:INTEGRATED"} {
		t.Run(strings.ReplaceAll(point, ":", "-"), func(t *testing.T) {
			f := designatedFixture(t)
			docsBase := multiGit(t, f.extra, "rev-parse", "HEAD")
			open := openProgram(t, f)
			w := open()
			a := reviewedProgram(t, w)
			grantIntegration(t, f, programScope(t, f, a))
			hit := 0
			restore := store.SetRunFaultForTest(f.s.repo.StateDir, func(at string) error {
				if at == point {
					hit++
					return wire.Errorf(wire.CodeLockTimeout, "lock", "injected interruption at %s", at)
				}
				return nil
			})
			_, err := w.RunRole(context.Background(), "integrator", "g")
			restore()
			if hit != 1 || err == nil {
				t.Fatalf("interruption at %s: hit %d err %v", point, hit, err)
			}
			candidate := programRecord(t, f).Repositories[0].Candidate
			if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != candidate {
				t.Fatalf("docs HEAD %s after interruption, want candidate %s", head, candidate)
			}
			if stored := f.s.attempt(t, a.AttemptID); stored.Phase != "READY_FOR_INTEGRATION" || len(stored.PendingEffects) != 1 {
				t.Fatalf("interrupted attempt %s effects %v", stored.Phase, stored.PendingEffects)
			}
			if _, err = open().RunRole(context.Background(), "integrator", "g"); err != nil {
				t.Fatalf("restart after %s: %v", point, err)
			}
			assertLanded(t, f, a, docsBase)
		})
	}
}

// TestCALV0087_UnchangedRepositoryNeedsNoDesignation proves an extra
// repository the stage left unchanged keeps its base as its candidate, needs
// no integration designation and is never moved, while the queue repository
// integrates.
func TestCALV0087_UnchangedRepositoryNeedsNoDesignation(t *testing.T) {
	t.Parallel()
	f := buildProgramFixture(t, true, true, []string{"verify"})
	f.config.OwnIntegrationCheckout = true
	if err := os.WriteFile(filepath.Join(f.scripts, "docs-unchanged"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	docsBase := multiGit(t, f.extra, "rev-parse", "HEAD")
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	p := programRecord(t, f)
	if p.Repositories[0].Candidate != docsBase || p.Repositories[0].IntegrationBranch != "" {
		t.Fatalf("unchanged docs candidate %s designation %q, want base %s", p.Repositories[0].Candidate, p.Repositories[0].IntegrationBranch, docsBase)
	}
	if refs := multiGit(t, f.extra, "for-each-ref", "refs/corvint/"); refs != "" {
		t.Fatalf("unchanged docs gained candidate refs %q", refs)
	}
	grantIntegration(t, f, programScope(t, f, a))
	if _, err := w.RunRole(context.Background(), "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
	if head := multiGit(t, f.extra, "rev-parse", "HEAD"); head != docsBase {
		t.Fatal("unchanged docs checkout moved")
	}
	if stored := f.s.attempt(t, a.AttemptID); stored.Phase != "COMPLETED" {
		t.Fatalf("attempt phase %s", stored.Phase)
	}
}

// TestCALV0088_ExtraRepositoryContextRequired proves each extra repository
// needs its own READY, fresh Core context: a docs worktree whose Core query
// fails refuses the stage before it is dispatched.
func TestCALV0088_ExtraRepositoryContextRequired(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	if err := os.WriteFile(filepath.Join(f.scripts, "no-docs-context"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	w := openProgram(t, f)()
	a, err := w.RunRole(context.Background(), "implementer", "")
	if err == nil || !strings.Contains(err.Error(), "repository docs: CONTEXT_UNAVAILABLE") {
		t.Fatalf("missing docs context not refused: %v", err)
	}
	if _, err = os.Stat(filepath.Join(f.scripts, "implement-args")); !os.IsNotExist(err) {
		t.Fatalf("host launched without docs context: %v", err)
	}
	if a != nil && a.Stage == "implement" && a.Phase != "ADMITTED" {
		t.Fatalf("refused stage reached phase %s", a.Phase)
	}
}
