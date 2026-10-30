//go:build darwin || linux

package store_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0170_ReadOnlyStageEditIsTypedAndRecoverable is the V1-0832
// regression: a review stage that exits cleanly after editing the bound
// candidate ends the role with a non-retryable DIRTY_WORKTREE, the program
// FINISHED with its owner released and the attempt WAITING on an operator
// question with no worker. After the operator restores the worktree and
// answers, the reviewer runs again. Before the fix the role returned a
// retryable "read-only stage changed candidate", left the attempt STOPPING
// with worker=true, and a rerun refused RESOURCE_COLLISION.
func TestATRV0170_ReadOnlyStageEditIsTypedAndRecoverable(t *testing.T) {
	t.Parallel()
	f := buildProgramFixture(t, false, false, nil)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	w, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if a, err := w.RunRole(ctx, "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	flag := filepath.Join(f.scripts, "review-edit")
	if err := os.WriteFile(flag, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := w.RunRole(ctx, "reviewer", "")
	if wire.CodeOf(err) != wire.CodeDirtyWorktree || !wire.RetryForbidden(err) || !strings.Contains(err.Error(), "read-only stage changed candidate") {
		t.Fatalf("edited candidate: want non-retryable %s, got %s %v", wire.CodeDirtyWorktree, wire.CodeOf(err), err)
	}
	if a == nil || a.Phase != "WAITING" || a.Supervision == nil || a.Supervision.Worker || a.Supervision.QuestionID == "" || a.WorktreePath == nil {
		t.Fatalf("attempt after edited candidate: %+v", a)
	}
	entries, err := store.ProgramRecords(ctx, f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range entries {
		if p.ID == "program" {
			found = true
			if p.Phase != "FINISHED" || !p.OwnerReleased {
				t.Fatalf("program after edited candidate: phase %s released %v", p.Phase, p.OwnerReleased)
			}
		}
	}
	if !found {
		t.Fatal("program record missing")
	}

	// Recovery: restore the candidate, answer, and review again.
	if err := os.Remove(flag); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", *a.WorktreePath, "checkout", "--", ".").CombinedOutput(); err != nil {
		t.Fatalf("restore: %v %s", err, out)
	}
	if err := w.Answer(a.Supervision.QuestionID, "restored", a.TicketRevision); err != nil {
		t.Fatalf("answer: %v", err)
	}
	if a, err = w.RunRole(ctx, "reviewer", ""); err != nil || a.Phase != "READY_FOR_INTEGRATION" {
		t.Fatalf("review after recovery: %+v %v", a, err)
	}
}
