//go:build darwin || linux

package store_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// completedProgram drives the designated fixture's program through
// implement, review and integration, so its attempt is COMPLETED.
func completedProgram(t *testing.T, f *multiFixture) {
	t.Helper()
	w := openProgram(t, f)()
	a := reviewedProgram(t, w)
	grantIntegration(t, f, programScope(t, f, a))
	if _, err := w.RunRole(context.Background(), "integrator", "g"); err != nil {
		t.Fatalf("integrate: %v", err)
	}
}

// TestATRV0297_ReassignRemovesPriorStageWorktrees proves a reassign removes
// the prior assignment's clean stage worktrees, keeps a dirty one registered
// instead of orphaning it, and that the operator cleanup removes the kept
// worktree from the idle program once it is clean (ATR-V0-296..298).
func TestATRV0297_ReassignRemovesPriorStageWorktrees(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	dirty := ""
	for _, r := range programRecord(t, f).Worktrees {
		if r.Repository == "docs" && !r.Removed {
			dirty = r.Path
		}
	}
	if dirty == "" {
		t.Fatal("no docs stage worktree was registered")
	}
	note := filepath.Join(dirty, "note.txt")
	if err := os.WriteFile(note, []byte("operator edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
		t.Fatalf("reopen after completion: %v, want the idle refusal", err)
	}
	p := programRecord(t, f)
	if p.Assignment != 2 || p.Phase != "ADMITTED" || p.CurrentAttempt != "" {
		t.Fatalf("reassign recorded assignment %d phase %s attempt %q", p.Assignment, p.Phase, p.CurrentAttempt)
	}
	if len(p.Worktrees) != 1 || p.Worktrees[0].Path != dirty || p.Worktrees[0].Removed {
		t.Fatalf("reassign kept %+v, want only the dirty %s", p.Worktrees, dirty)
	}
	if list := multiGit(t, f.s.repo.PrimaryWorktree, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("reassign left a clean queue stage worktree:\n%s", list)
	}
	results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != 1 || results[0].Removed || results[0].Reason == "" {
		t.Fatalf("cleanup of a dirty worktree: %+v %v, want it kept with a reason", results, err)
	}
	multiGit(t, dirty, "checkout", "--", "note.txt")
	results, err = store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != 1 || !results[0].Removed || results[0].Path != dirty {
		t.Fatalf("cleanup of the restored worktree: %+v %v", results, err)
	}
	if p = programRecord(t, f); p.Phase != "ADMITTED" || len(p.Worktrees) != 0 {
		t.Fatalf("cleanup recorded phase %s worktrees %+v, want the removed record dropped (CAL-V0-366)", p.Phase, p.Worktrees)
	}
	if list := multiGit(t, f.extra, "worktree", "list", "--porcelain"); strings.Count(list, "worktree ") != 1 {
		t.Fatalf("docs keeps a stage worktree:\n%s", list)
	}
}

// TestATRV0296_CleanupRefusesLiveAttempt proves the operator cleanup refuses
// while the program's attempt is live and leaves its stage worktree in place.
func TestATRV0296_CleanupRefusesLiveAttempt(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	w := openProgram(t, f)()
	if a, err := w.RunRole(context.Background(), "implementer", ""); err != nil || a.Phase != "BUILT" {
		t.Fatalf("implement: %+v %v", a, err)
	}
	_, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
	if wire.CodeOf(err) != wire.CodeAttemptLive {
		t.Fatalf("cleanup during a live attempt: %v, want ATTEMPT_LIVE", err)
	}
	for _, r := range programRecord(t, f).Worktrees {
		if _, err := os.Lstat(r.Path); r.Removed || err != nil {
			t.Fatalf("stage worktree %s touched: removed %v %v", r.Path, r.Removed, err)
		}
	}
}

// interruptedRemovalEnv carries the config, root and mode to the child owner
// of the CAL-V0-365..371 takeover tests.
const interruptedRemovalEnv = "CORVINT_TEST_INTERRUPTED_REMOVAL"

// startRemovalChild starts TestCALV0365_InterruptedRemovalChild as the
// cleanup owner of f's program in mode.
func startRemovalChild(t *testing.T, f *multiFixture, mode string) *exec.Cmd {
	t.Helper()
	config, err := json.Marshal(f.config)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestCALV0365_InterruptedRemovalChild$", "-test.count=1")
	child.Env = append(os.Environ(), interruptedRemovalEnv+"_CONFIG="+string(config), interruptedRemovalEnv+"_ROOT="+f.s.repo.PrimaryWorktree,
		interruptedRemovalEnv+"_MODE="+mode, interruptedRemovalEnv+"_EXTRA="+f.extra)
	return child
}

// interruptedRemoval runs the child owner in mode until it stops at its
// first WORKTREE_REMOVE write and returns the parked program record.
func interruptedRemoval(t *testing.T, f *multiFixture, mode string) snapshot.Program {
	t.Helper()
	child := startRemovalChild(t, f, mode)
	out, err := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child did not stop at WORKTREE_REMOVE: %v\n%s", err, out)
	}
	p := programRecord(t, f)
	if p.Phase != "WORKTREE_REMOVE" || p.OwnerPID != child.Process.Pid || p.RemoverPID <= 0 || p.RemoverStarted == "" {
		t.Fatalf("interrupted program phase %s owner %d remover %d %q", p.Phase, p.OwnerPID, p.RemoverPID, p.RemoverStarted)
	}
	return p
}

// TestCALV0365_InterruptedRemovalChild is the cleanup owner of the takeover
// tests. At its first WORKTREE_REMOVE write, which names its gated remover,
// it stands in for an owner crash before the remover is released: mode
// "removed" first lets Git remove that worktree, "half" deletes only the work
// tree, "moved" moves it with `git worktree move`, "stopped" stops the remover
// child and exits at once, so the remover outlives its owner, and "live"
// reports PARKED and waits for its stdin to close, then finishes the cleanup
// as a live owner. It is skipped otherwise.
func TestCALV0365_InterruptedRemovalChild(t *testing.T) {
	raw := os.Getenv(interruptedRemovalEnv + "_CONFIG")
	if raw == "" {
		t.Skip("child of the CAL-V0-365 takeover tests")
	}
	var c store.ProgramConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv(interruptedRemovalEnv + "_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	mode := os.Getenv(interruptedRemovalEnv + "_MODE")
	interrupted := errors.New("interrupted at WORKTREE_REMOVE")
	parked := false
	defer store.SetRunFaultForTest(repo.StateDir, func(at string) error {
		if at != "cleanup:WORKTREE_REMOVE" || parked {
			return nil
		}
		parked = true
		entries, err := store.ProgramRecords(context.Background(), repo)
		if err != nil || len(entries) != 1 {
			t.Fatalf("program records: %v", err)
		}
		p := entries[0]
		dir := repo.PrimaryWorktree
		for _, r := range p.Worktrees {
			if r.Path == p.Worktree && r.Repository != "" {
				dir = os.Getenv(interruptedRemovalEnv + "_EXTRA")
			}
		}
		git := func(args ...string) {
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", args, err, out)
			}
		}
		switch mode {
		case "half":
			// git worktree remove deletes the work tree before its
			// registration; stop between the two.
			if err := os.RemoveAll(p.Worktree); err != nil {
				t.Fatal(err)
			}
		case "removed":
			git("worktree", "remove", p.Worktree)
		case "moved":
			git("worktree", "move", p.Worktree, p.Worktree+"-moved")
		case "stopped":
			if err := syscall.Kill(p.RemoverPID, syscall.SIGSTOP); err != nil {
				t.Fatal(err)
			}
			os.Exit(3)
		case "live":
			fmt.Println("PARKED")
			_, _ = io.Copy(io.Discard, os.Stdin)
			return nil
		}
		return interrupted
	})()
	_, err = store.CleanupProgram(context.Background(), repo, operator(), "program", c)
	if mode == "live" {
		if err != nil {
			t.Fatalf("live cleanup: %v", err)
		}
		return
	}
	if errors.Is(err, interrupted) {
		os.Exit(3)
	}
	t.Fatalf("cleanup was not interrupted at WORKTREE_REMOVE: %v", err)
}

// TestCALV0365_TakeoverSettlesInterruptedRemoval proves a replacement owner
// settles a cleanup whose owner died after its WORKTREE_REMOVE write: the
// takeover returns the program to FINISHED with a typed outcome for the
// parked worktree (REMOVAL_ROLLED_BACK when Git still registers it;
// REMOVAL_COMPLETED when Git had removed it, or had deleted the work tree but
// not yet its registration), and the cleanup finishes the remaining
// worktrees; a reassign through OpenWorkflow settles it the same way. A
// rolled-back worktree is left for the next cleanup rather than removed in
// the same invocation (CAL-V0-370): before that fix the one invocation
// reported REMOVAL_ROLLED_BACK and then a removal of the same worktree.
func TestCALV0365_TakeoverSettlesInterruptedRemoval(t *testing.T) {
	for _, mode := range []string{"registered", "removed", "half", "reopen"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			f := designatedFixture(t)
			completedProgram(t, f)
			before := programRecord(t, f)
			p := interruptedRemoval(t, f, mode)
			parked := p.Worktree
			rolledBack := mode == "registered" || mode == "reopen"
			if mode == "reopen" {
				// A reassign takes the program over through OpenWorkflow.
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.OpenWorkflow(context.Background(), f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
					t.Fatalf("reopen after the interrupted removal: %v, want the idle refusal", err)
				}
				if q := programRecord(t, f); q.Assignment != 2 || q.Phase != "ADMITTED" || len(q.Worktrees) != 1 || q.Worktrees[0].Path != parked || q.Epoch != p.Epoch+1 || q.RemoverPID != 0 {
					t.Fatalf("reassign after takeover: assignment %d phase %s epoch %d worktrees %+v", q.Assignment, q.Phase, q.Epoch, q.Worktrees)
				}
			} else {
				results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
				if err != nil || len(results) == 0 {
					t.Fatalf("takeover cleanup: %+v %v", results, err)
				}
				want := store.RemovalCompleted
				if rolledBack {
					want = store.RemovalRolledBack
				}
				if results[0].Path != parked || results[0].Takeover != want || results[0].Removed == rolledBack || (mode == "half") != strings.Contains(results[0].Reason, "git worktree prune") {
					t.Fatalf("takeover item %+v, want %s for %s", results[0], want, parked)
				}
				for _, r := range results[1:] {
					if !r.Removed || r.Takeover != "" || r.Path == parked {
						t.Fatalf("cleanup after takeover reported %+v", r)
					}
				}
				q := programRecord(t, f)
				if kept := len(q.Worktrees); q.Phase != "FINISHED" || q.Epoch != p.Epoch+1 || q.OwnerPID != os.Getpid() || q.RemoverPID != 0 || (kept == 1) != rolledBack || kept > 1 {
					t.Fatalf("settled program phase %s epoch %d (was %d) worktrees %+v", q.Phase, q.Epoch, before.Epoch, q.Worktrees)
				}
			}
			if !rolledBack {
				if _, err := os.Lstat(parked); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("parked worktree %s remains: %v", parked, err)
				}
				return
			}
			if _, err := os.Lstat(parked); err != nil {
				t.Fatalf("rolled-back worktree %s was touched: %v", parked, err)
			}
			results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
			if err != nil || len(results) != 1 || results[0].Path != parked || !results[0].Removed || results[0].Takeover != "" {
				t.Fatalf("next cleanup of the rolled-back worktree: %+v %v", results, err)
			}
			if _, err := os.Lstat(parked); !errors.Is(err, os.ErrNotExist) || len(programRecord(t, f).Worktrees) != 0 {
				t.Fatalf("rolled-back worktree %s remains after the next cleanup: %v", parked, err)
			}
		})
	}
}

// TestCALV0367_LiveCleanupOwnerIsNotTakenOver proves two simultaneously live
// cleanups never both remove: while cleanup A is paused right after its
// WORKTREE_REMOVE write, cleanup B is refused ATTEMPT_LIVE and touches
// nothing, and A then finishes. Before the fix A's write kept the
// OwnerReleased a completed stage left, so B took the program over and ran
// its own removal of the worktree A was removing.
func TestCALV0367_LiveCleanupOwnerIsNotTakenOver(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	if !programRecord(t, f).OwnerReleased {
		t.Fatal("fixture program is not released by its completed stage")
	}
	child := startRemovalChild(t, f, "live")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill() }()
	lines := bufio.NewScanner(stdout)
	for lines.Scan() && lines.Text() != "PARKED" {
	}
	p := programRecord(t, f)
	if p.Phase != "WORKTREE_REMOVE" || p.OwnerPID != child.Process.Pid {
		t.Fatalf("live cleanup A: phase %s owner %d", p.Phase, p.OwnerPID)
	}
	parked := p.Worktree
	_, err = store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
	if wire.CodeOf(err) != wire.CodeAttemptLive || !strings.Contains(err.Error(), "live owner") {
		t.Fatalf("cleanup B beside live cleanup A: %v, want ATTEMPT_LIVE for the live owner", err)
	}
	if q := programRecord(t, f); q.OwnerPID != child.Process.Pid || q.Epoch != p.Epoch || q.Phase != "WORKTREE_REMOVE" || q.OwnerReleased {
		t.Fatalf("cleanup B replaced A: owner %d epoch %d phase %s released %v", q.OwnerPID, q.Epoch, q.Phase, q.OwnerReleased)
	}
	if _, err := os.Lstat(parked); err != nil {
		t.Fatalf("cleanup B touched %s: %v", parked, err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	rest, _ := io.ReadAll(stdout)
	if err := child.Wait(); err != nil {
		t.Fatalf("cleanup A after B's refusal: %v\n%s", err, rest)
	}
	if q := programRecord(t, f); q.Phase != "FINISHED" || q.OwnerPID != child.Process.Pid || len(q.Worktrees) != 0 {
		t.Fatalf("cleanup A finished phase %s owner %d worktrees %+v", q.Phase, q.OwnerPID, q.Worktrees)
	}
}

// TestCALV0368_SurvivingRemoverRefusesTakeover proves a takeover never
// re-runs a removal while the dead owner's remover child may still act: with
// the remover stopped (not yet released) and its owner gone, cleanup refuses
// ATTEMPT_LIVE naming REMOVER_LIVE and touches nothing; once that child is
// gone the takeover settles. A record naming no remover is refused
// REMOVER_UNRECORDED. Before the fix the remover was an unrecorded child, and
// the takeover rolled back and launched a second removal beside it.
func TestCALV0368_SurvivingRemoverRefusesTakeover(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	p := interruptedRemoval(t, f, "stopped")
	defer func() { _ = syscall.Kill(p.RemoverPID, syscall.SIGKILL) }()
	if id, err := supervisor.ProcessIdentity(p.RemoverPID); err != nil || id != p.RemoverStarted {
		t.Fatalf("stopped remover %d not live: %q %v", p.RemoverPID, id, err)
	}
	_, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
	if wire.CodeOf(err) != wire.CodeAttemptLive || !strings.Contains(err.Error(), store.RemoverLive) {
		t.Fatalf("takeover beside a live remover: %v, want ATTEMPT_LIVE %s", err, store.RemoverLive)
	}
	if q := programRecord(t, f); q.Phase != "WORKTREE_REMOVE" || q.Epoch != p.Epoch {
		t.Fatalf("refused takeover wrote: phase %s epoch %d", q.Phase, q.Epoch)
	}
	if _, err := os.Lstat(p.Worktree); err != nil {
		t.Fatalf("refused takeover touched %s: %v", p.Worktree, err)
	}
	if err := syscall.Kill(p.RemoverPID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(20 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		if id, err := supervisor.ProcessIdentity(p.RemoverPID); err == nil && id != p.RemoverStarted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("killed remover was not reaped")
		}
	}
	results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) == 0 || results[0].Path != p.Worktree || results[0].Takeover != store.RemovalRolledBack {
		t.Fatalf("takeover after the remover stopped: %+v %v", results, err)
	}
	unrecorded := p
	unrecorded.RemoverPID, unrecorded.RemoverStarted = 0, ""
	if err := store.RemoverStoppedForTest(unrecorded); wire.CodeOf(err) != wire.CodeQuiescenceUnproved || !strings.Contains(err.Error(), store.RemoverUnrecorded) {
		t.Fatalf("record without a remover: %v, want QUIESCENCE_UNPROVED %s", err, store.RemoverUnrecorded)
	}
}

// TestCALV0371_MovedWorktreeKeepsItsRecord proves a takeover does not treat a
// missing pathname as a completed removal: after `git worktree move` of the
// parked worktree, the record is kept with REMOVAL_ROLLED_BACK naming the new
// location, and the moved worktree stays registered. Before the fix the
// takeover reported REMOVAL_COMPLETED and forgot the worktree.
func TestCALV0371_MovedWorktreeKeepsItsRecord(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	p := interruptedRemoval(t, f, "moved")
	moved := p.Worktree + "-moved"
	results, err := store.CleanupProgram(context.Background(), f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) == 0 {
		t.Fatalf("takeover cleanup: %+v %v", results, err)
	}
	if r := results[0]; r.Path != p.Worktree || r.Removed || r.Takeover != store.RemovalRolledBack || !strings.Contains(r.Reason, "moved to") {
		t.Fatalf("takeover item %+v, want REMOVAL_ROLLED_BACK naming the move", r)
	}
	kept := false
	for _, r := range programRecord(t, f).Worktrees {
		kept = kept || r.Path == p.Worktree
	}
	if !kept {
		t.Fatal("the moved worktree's record was dropped")
	}
	if _, err := os.Lstat(moved); err != nil {
		t.Fatalf("moved worktree %s: %v", moved, err)
	}
}

// TestCALV0369_LegacyRemovedRecordsAreDroppedDurably proves records an older
// binary kept marked removed are dropped by a durable write even when the
// cleanup removes nothing else: a FINISHED program whose every record is
// removed, and an idle ADMITTED program, whose phase admits no other
// self-transition. Before the fix the records were filtered in memory only
// and stayed stored.
func TestCALV0369_LegacyRemovedRecordsAreDroppedDurably(t *testing.T) {
	for _, phase := range []string{"FINISHED", "ADMITTED"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			f := designatedFixture(t)
			completedProgram(t, f)
			ctx := context.Background()
			if phase == "ADMITTED" {
				self, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, ""); !errors.Is(err, store.ErrProgramIdle) {
					t.Fatalf("reassign: %v", err)
				}
			}
			p := programRecord(t, f)
			if p.Phase != phase {
				t.Fatalf("fixture phase %s, want %s", p.Phase, phase)
			}
			// An older binary's records: every stage worktree removed and
			// its record kept marked.
			legacy := []snapshot.WorktreeRecord{}
			for _, r := range p.Worktrees {
				dir := f.s.repo.PrimaryWorktree
				if r.Repository != "" {
					dir = f.extra
				}
				multiGit(t, dir, "worktree", "remove", "--force", r.Path)
				r.Removed = true
				legacy = append(legacy, r)
			}
			if len(legacy) == 0 {
				legacy = append(legacy, snapshot.WorktreeRecord{Path: filepath.Join(t.TempDir(), "gone"), Identity: "1:1", PrivateIdentity: "1:2", Commit: p.Base, Removed: true})
			}
			next := p
			next.Worktrees = legacy
			if phase == "ADMITTED" {
				// The older binary's idle removal: ADMITTED to
				// WORKTREE_REMOVE and back, the record kept marked.
				started, err := supervisor.ProcessIdentity(os.Getpid())
				if err != nil {
					t.Fatal(err)
				}
				removing := p
				removing.Phase, removing.Worktree, removing.RemoverPID, removing.RemoverStarted = "WORKTREE_REMOVE", legacy[0].Path, os.Getpid(), started
				removing.Worktrees = append([]snapshot.WorktreeRecord{}, legacy...)
				removing.Worktrees[0].Removed = false
				programWrite(t, f, "legacy-remove", removing)
				next.Worktree = legacy[0].Path
			}
			programWrite(t, f, "legacy-removed", next)
			if n := len(programRecord(t, f).Worktrees); n != len(legacy) {
				t.Fatalf("legacy fixture stored %d records, want %d", n, len(legacy))
			}
			results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
			if err != nil || len(results) != 0 {
				t.Fatalf("cleanup of legacy records: %+v %v", results, err)
			}
			if q := programRecord(t, f); len(q.Worktrees) != 0 || q.Phase != phase {
				t.Fatalf("cleanup kept legacy records %+v in phase %s", q.Worktrees, q.Phase)
			}
		})
	}
}

// programWrite commits next as the fixture program's record.
func programWrite(t *testing.T, f *multiFixture, request string, next snapshot.Program) {
	t.Helper()
	r, err := store.ProgramTransition(context.Background(), f.s.repo, operator(), fixture.QueueID, request, next)
	if err != nil || r.Outcome.Outcome != mutation.OutcomeCompleted {
		t.Fatalf("program write %s: %+v %v", request, r, err)
	}
}

// TestCALV0366_CleanupDropsRemovedRecords proves a cleanup drops the record
// of every worktree it removes, so the program record holds only worktrees
// still on disk and a repeated cleanup neither grows it nor reports again.
// Before the fix each removed record stayed, marked removed.
func TestCALV0366_CleanupDropsRemovedRecords(t *testing.T) {
	t.Parallel()
	f := designatedFixture(t)
	completedProgram(t, f)
	registered := len(programRecord(t, f).Worktrees)
	if registered < 2 {
		t.Fatalf("fixture registered %d stage worktrees, want at least 2", registered)
	}
	ctx := context.Background()
	results, err := store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != registered {
		t.Fatalf("cleanup: %+v %v", results, err)
	}
	if p := programRecord(t, f); len(p.Worktrees) != 0 || p.Phase != "FINISHED" {
		t.Fatalf("cleanup kept records %+v in phase %s", p.Worktrees, p.Phase)
	}
	results, err = store.CleanupProgram(ctx, f.s.repo, operator(), "program", f.config)
	if err != nil || len(results) != 0 || len(programRecord(t, f).Worktrees) != 0 {
		t.Fatalf("repeated cleanup: %+v %v", results, err)
	}
}
