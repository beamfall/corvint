package store_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// copyTree copies the regular files under src to dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fixture.Write(t, filepath.Join(dst, rel), raw)
		return nil
	})
	if err != nil {
		t.Fatalf("copy %s: %v", src, err)
	}
}

// moveToFeature registers a linked worktree on the intent branch carrying a
// copy of the primary's committed projection, then moves the primary to a
// feature branch, the way `git switch -c feature` followed by `git worktree
// add ../intent main` would. It returns the re-resolved repository and the
// linked root.
func moveToFeature(t *testing.T, repo *intent.Repository) (*intent.Repository, string) {
	t.Helper()
	admin := filepath.Join(repo.CommonDir, "worktrees", "intent")
	root := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "intent")
	fixture.Write(t, filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/main\n"))
	fixture.Write(t, filepath.Join(admin, "gitdir"), []byte(filepath.Join(root, ".git")+"\n"))
	fixture.Write(t, filepath.Join(admin, "commondir"), []byte("../..\n"))
	fixture.Write(t, filepath.Join(root, ".git"), []byte("gitdir: "+admin+"\n"))
	copyTree(t, filepath.Join(repo.PrimaryWorktree, intent.Dir), filepath.Join(root, intent.Dir))
	fixture.Write(t, filepath.Join(repo.CommonDir, "HEAD"), []byte("ref: refs/heads/feature\n"))
	again, err := intent.Resolve(repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if again.IntentRoot() != root {
		t.Fatalf("intent root = %q, want the linked worktree %q (fix %q)", again.IntentRoot(), root, again.IntentFix)
	}
	return again, root
}

// refusalText is the refusal a writer reported, as an error or as an outcome.
func refusalText(t *testing.T, report *store.Report, err error, code string) string {
	t.Helper()
	if err != nil {
		if wire.CodeOf(err) != code {
			t.Fatalf("err = %v, want %s", err, code)
		}
		return err.Error()
	}
	if report == nil || !report.Outcome.HasCode(code) || report.Receipt != "" {
		t.Fatalf("report = %+v, want a %s refusal with no receipt", report, code)
	}
	return report.Detail
}

// untouched digests the primary's HEAD and index files, the primary and
// linked projections and the state dir, by path and content, so a refusal can
// be shown to write nothing a writer owns anywhere. (A refused writer may
// still refresh the derived journal checkpoint beside the state dir, as it
// does today; that cache is not compared.)
func untouched(t *testing.T, repo *intent.Repository, linked string) wire.Digest {
	t.Helper()
	roots := []string{
		filepath.Join(repo.CommonDir, "HEAD"), filepath.Join(repo.CommonDir, "index"),
		filepath.Join(repo.PrimaryWorktree, intent.Dir), repo.StateDir,
	}
	if linked != "" {
		roots = append(roots, filepath.Join(linked, intent.Dir))
	}
	var all []byte
	for _, root := range roots {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if os.IsNotExist(err) && p == root {
				return nil
			}
			if err != nil || info.IsDir() {
				return err
			}
			raw, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			all = append(append(all, p...), raw...)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	return wire.Sum(all)
}

// TestCTWV0002_FeatureBranchPrimaryFilesIntoTheIntentWorktree is the V1-0325
// acceptance: with the primary on a feature branch and a linked worktree on
// the intent branch, CREATE commits, the projection lands in the linked
// worktree, and the primary's HEAD, index and projection are untouched.
func TestCTWV0002_FeatureBranchPrimaryFilesIntoTheIntentWorktree(t *testing.T) {
	base, _ := initialized(t)
	fixture.Write(t, filepath.Join(base.CommonDir, "index"), []byte("DIRC fixture index bytes"))
	repo, linked := moveToFeature(t, base)
	headBefore, _ := os.ReadFile(filepath.Join(repo.CommonDir, "HEAD"))
	indexBefore, _ := os.ReadFile(filepath.Join(repo.CommonDir, "index"))
	primaryBefore := fixture.TreeSnapshot(t, filepath.Join(repo.PrimaryWorktree, intent.Dir))

	report := mutate(t, repo, envelope("from-feature", mutation.OpCreate, "", "", createPayload("filed from a feature branch")))
	if report.Outcome.Outcome != mutation.OutcomeCompleted || report.Receipt == "" {
		t.Fatalf("outcome = %s (%v) %s", report.Outcome.Outcome, report.Outcome.Codes, report.Detail)
	}
	id, err := wire.ParseTicketID("ticketId", report.Ticket)
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(intent.Dir, "tickets", id.Local+".json")
	if _, err := os.Stat(filepath.Join(linked, rel)); err != nil {
		t.Fatalf("the projection did not land in the intent worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.PrimaryWorktree, rel)); !os.IsNotExist(err) {
		t.Fatalf("the projection leaked into the primary: %v", err)
	}
	headAfter, _ := os.ReadFile(filepath.Join(repo.CommonDir, "HEAD"))
	indexAfter, _ := os.ReadFile(filepath.Join(repo.CommonDir, "index"))
	if !bytes.Equal(headBefore, headAfter) || !bytes.Equal(indexBefore, indexAfter) {
		t.Fatalf("primary HEAD or index changed: %q -> %q", headBefore, headAfter)
	}
	if !fixture.SameTree(primaryBefore, fixture.TreeSnapshot(t, filepath.Join(repo.PrimaryWorktree, intent.Dir))) {
		t.Fatal("the primary projection changed")
	}

	// The same rule applies from inside the linked worktree, and reads load
	// the ticket from the intent root.
	fromLinked, err := intent.Resolve(linked)
	if err != nil {
		t.Fatal(err)
	}
	second := mutate(t, fromLinked, envelope("from-linked", mutation.OpCreate, "", "", createPayload("filed from the intent worktree")))
	if second.Receipt == "" {
		t.Fatalf("second create refused: %+v", second)
	}
	loaded, err := intent.Load(fromLinked.IntentRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, ticket := range []string{report.Ticket, second.Ticket} {
		if _, ok := loaded.Inventory.Get(ticket); !ok {
			t.Errorf("%s is not readable from the intent root", ticket)
		}
	}
}

// TestCTWV0005_MissingIntentWorktreeRefusesWithTheFix keeps today's
// INTENT_BRANCH_MISMATCH when no linked worktree holds the intent branch, adds
// the exact fix, and writes nothing.
func TestCTWV0005_MissingIntentWorktreeRefusesWithTheFix(t *testing.T) {
	base, _ := initialized(t)
	fixture.Write(t, filepath.Join(base.CommonDir, "HEAD"), []byte("ref: refs/heads/feature\n"))
	repo, err := intent.Resolve(base.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	before := untouched(t, repo, "")
	report, err := store.Mutate(context.Background(), repo, operator(), envelope("refused", mutation.OpCreate, "", "", createPayload("must not commit")), now(t))
	text := refusalText(t, report, err, wire.CodeIntentBranchMismatch)
	want := `run "git -C "` + repo.PrimaryWorktree + `" worktree add "` + repo.PrimaryWorktree + `-main" main", or switch the primary checkout to main, then retry from any checkout (CTW-V0-005)`
	if !strings.Contains(text, want) {
		t.Errorf("refusal %q lacks the fix %q", text, want)
	}
	if untouched(t, repo, "") != before {
		t.Fatal("refusal changed the repository")
	}
}

// TestCTWV0004_StaleIntentWorktreeRefusesWithTheFix: a registration whose
// worktree was deleted is not admitted; the refusal names prune and add.
func TestCTWV0004_StaleIntentWorktreeRefusesWithTheFix(t *testing.T) {
	base, _ := initialized(t)
	_, linked := moveToFeature(t, base)
	if err := os.RemoveAll(linked); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(base.PrimaryWorktree)
	if err != nil {
		t.Fatal(err)
	}
	before := untouched(t, repo, "")
	report, err := store.Mutate(context.Background(), repo, operator(), envelope("refused", mutation.OpCreate, "", "", createPayload("must not commit")), now(t))
	text := refusalText(t, report, err, wire.CodeIntentBranchMismatch)
	for _, part := range []string{`linked worktree entry "intent" is stale`, "git worktree prune", "worktree add", "CTW-V0-005"} {
		if !strings.Contains(text, part) {
			t.Errorf("refusal %q lacks %q", text, part)
		}
	}
	if untouched(t, repo, "") != before {
		t.Fatal("refusal changed the repository")
	}
}

// TestCTWV0007_DirtyIntentWorktreeRefusesWithTheFix: an intent worktree whose
// projection no longer matches the journal is refused INTENT_DIVERGED by the
// existing audit, the refusal names the linked projection, and nothing is
// written anywhere.
func TestCTWV0007_DirtyIntentWorktreeRefusesWithTheFix(t *testing.T) {
	base, _ := initialized(t)
	repo, linked := moveToFeature(t, base)
	editJSON(t, filepath.Join(linked, intent.Dir, "policy.json"), func(v wire.Value) {
		v.Obj.Set("policyVersion", str("99"))
	})
	before := untouched(t, repo, linked)
	report, err := store.Mutate(context.Background(), repo, operator(), envelope("refused", mutation.OpCreate, "", "", createPayload("must not commit")), now(t))
	text := refusalText(t, report, err, wire.CodeIntentDiverged)
	for _, part := range []string{filepath.Join(linked, intent.Dir), "linked intent worktree", "reconcile inspect"} {
		if !strings.Contains(text, part) {
			t.Errorf("refusal %q lacks %q", text, part)
		}
	}
	if untouched(t, repo, linked) != before {
		t.Fatal("refusal changed the repository")
	}
}

// TestCTWV0009_IntentBranchPrimaryRefusalsAreUnchanged: from the intent-branch
// primary a drift refusal carries no CTW repair text even when linked
// worktrees exist, and writes still publish into the primary.
func TestCTWV0009_IntentBranchPrimaryRefusalsAreUnchanged(t *testing.T) {
	repo, _ := initialized(t)
	admin := filepath.Join(repo.CommonDir, "worktrees", "side")
	side := filepath.Join(filepath.Dir(repo.PrimaryWorktree), "side")
	fixture.Write(t, filepath.Join(admin, "HEAD"), []byte("ref: refs/heads/side\n"))
	fixture.Write(t, filepath.Join(admin, "gitdir"), []byte(filepath.Join(side, ".git")+"\n"))
	fixture.Write(t, filepath.Join(side, ".git"), []byte("gitdir: "+admin+"\n"))
	repo, err := intent.Resolve(repo.PrimaryWorktree)
	if err != nil || repo.IntentLinked() {
		t.Fatalf("resolve: %v linked=%v", err, repo.IntentLinked())
	}
	created := mutate(t, repo, envelope("primary", mutation.OpCreate, "", "", createPayload("primary")))
	id, _ := wire.ParseTicketID("ticketId", created.Ticket)
	if _, err := os.Stat(filepath.Join(repo.PrimaryWorktree, intent.Dir, "tickets", id.Local+".json")); err != nil {
		t.Fatalf("the intent-branch primary did not receive the projection: %v", err)
	}
	editJSON(t, filepath.Join(repo.PrimaryWorktree, intent.Dir, "policy.json"), func(v wire.Value) {
		v.Obj.Set("policyVersion", str("99"))
	})
	report, err := store.Mutate(context.Background(), repo, operator(), envelope("refused", mutation.OpCreate, "", "", createPayload("must not commit")), now(t))
	if text := refusalText(t, report, err, wire.CodeIntentDiverged); strings.Contains(text, "CTW-V0") || strings.Contains(text, "linked") {
		t.Errorf("intent-branch refusal gained repair text: %q", text)
	}
}

// TestCTWV0002_RealGitWorktreeFilesWithoutSwitchingThePrimary repeats the
// acceptance with real `git worktree add`, checking the primary's branch,
// index file and `git status` before and after.
func TestCTWV0002_RealGitWorktreeFilesWithoutSwitchingThePrimary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(primary, 0o755); err != nil {
		t.Fatal(err)
	}
	commit := func(dir, msg string) {
		gitRun(t, dir, "add", "-A", intent.Dir)
		gitRun(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", msg)
	}
	gitRun(t, primary, "init", "-q", "-b", "main")
	fixture.Write(t, filepath.Join(primary, intent.Dir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(primary, intent.Dir, "policy.json"), fixture.PolicyBytes())
	commit(primary, "queue")
	repo, err := intent.Resolve(primary)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Init(context.Background(), repo, operator(), "req-init", now(t)); err != nil {
		t.Fatalf("init: %v", err)
	}
	commit(primary, "init")
	gitRun(t, primary, "switch", "-q", "-c", "feature")
	linked := filepath.Join(tmp, "intent")
	gitRun(t, primary, "worktree", "add", "-q", linked, "main")

	observe := func() []string {
		head, _ := os.ReadFile(filepath.Join(primary, ".git", "HEAD"))
		index, _ := os.ReadFile(filepath.Join(primary, ".git", "index"))
		c := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
		c.Dir = primary
		status, err := c.Output()
		if err != nil {
			t.Fatalf("git status: %v", err)
		}
		return []string{string(head), string(wire.Sum(index)), string(status)}
	}
	before := observe()
	repo, err = intent.Resolve(primary)
	if err != nil {
		t.Fatal(err)
	}
	if repo.IntentRoot() != linked {
		t.Fatalf("intent root = %q, want %q (fix %q)", repo.IntentRoot(), linked, repo.IntentFix)
	}
	report := mutate(t, repo, envelope("real-git", mutation.OpCreate, "", "", createPayload("filed from a real feature branch")))
	if report.Receipt == "" {
		t.Fatalf("create refused: %+v", report)
	}
	after := observe()
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("primary observation %d changed: %q -> %q", i, before[i], after[i])
		}
	}
	if before[0] != "ref: refs/heads/feature\n" {
		t.Errorf("primary HEAD = %q", before[0])
	}
	c := exec.Command("git", "status", "--porcelain=v1", "--untracked-files=all")
	c.Dir = linked
	out, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	id, _ := wire.ParseTicketID("ticketId", report.Ticket)
	if !strings.Contains(string(out), "?? "+filepath.ToSlash(filepath.Join(intent.Dir, "tickets", id.Local+".json"))) {
		t.Errorf("the linked worktree does not show the new ticket:\n%s", out)
	}
}
