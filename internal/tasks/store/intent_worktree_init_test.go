package store_test

import (
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

// TestCTWV0780_InitFromFeatureBranchWithLinkedIntentWorktree is the V1-3525
// acceptance: with the intent files only in a primary on a feature branch and
// an intent-branch worktree already added, init refuses with a fix that names
// that worktree's missing projection and never asks to add the worktree again;
// once the worktree holds the same files, init succeeds and its records land
// only there.
func TestCTWV0780_InitFromFeatureBranchWithLinkedIntentWorktree(t *testing.T) {
	fixture.SkipUnsupportedPlatform(t)
	t.Parallel()
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
	gitRun(t, primary, "init", "-q", "-b", "main")
	gitRun(t, primary, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "root")
	gitRun(t, primary, "switch", "-q", "-c", "feature")
	linked := filepath.Join(tmp, "repo-main")
	gitRun(t, primary, "worktree", "add", "-q", linked, "main")
	fixture.Write(t, filepath.Join(primary, intent.Dir, "queue.json"), fixture.QueueBytes())
	fixture.Write(t, filepath.Join(primary, intent.Dir, "policy.json"), fixture.PolicyBytes())

	repo, err := intent.Resolve(primary)
	if err != nil {
		t.Fatal(err)
	}
	report, err := store.Init(context.Background(), repo, operator(), "req-init", now(t))
	text := refusalText(t, report, err, wire.CodeIntentBranchMismatch)
	for _, part := range []string{"queue.json is missing or unreadable", "give '" + linked + "' a .taskman directory", "queue.json", "CTW-V0-780"} {
		if !strings.Contains(text, part) {
			t.Errorf("refusal %q lacks %q", text, part)
		}
	}
	for _, satisfied := range []string{"worktree add", "worktree prune", "worktree repair"} {
		if strings.Contains(text, satisfied) {
			t.Errorf("refusal %q repeats the satisfied step %q", text, satisfied)
		}
	}

	copyTree(t, filepath.Join(primary, intent.Dir), filepath.Join(linked, intent.Dir))
	repo, err = intent.Resolve(primary)
	if err != nil {
		t.Fatal(err)
	}
	if repo.IntentRoot() != linked {
		t.Fatalf("intent root = %q, want %q (fix %q)", repo.IntentRoot(), linked, repo.IntentFix)
	}
	report, err = store.Init(context.Background(), repo, operator(), "req-init-2", now(t))
	if err != nil || report.Receipt == "" {
		t.Fatalf("init with both projections: %v %+v", err, report)
	}
	report = mutate(t, repo, envelope("after-init", mutation.OpCreate, "", "", createPayload("filed after init")))
	if report.Receipt == "" {
		t.Fatalf("create refused: %+v", report)
	}
	id, _ := wire.ParseTicketID("ticketId", report.Ticket)
	rel := filepath.Join(intent.Dir, "tickets", id.Local+".json")
	if _, err := os.Stat(filepath.Join(linked, rel)); err != nil {
		t.Fatalf("the ticket is not in the intent worktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(primary, rel)); !os.IsNotExist(err) {
		t.Fatalf("the ticket leaked into the primary: %v", err)
	}
}
