//go:build darwin || linux

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

// workerRecoveryEnv carries the config, root and ticket to the child owner
// of TestCALV0074_WorkerRecoveryFromStopping.
const workerRecoveryEnv = "CORVINT_TEST_WORKER_RECOVERY"

// TestCALV0074_WorkerRecoveryChild is the owner process of
// TestCALV0074_WorkerRecoveryFromStopping. It runs the implement stage and
// exits once the program and the attempt are both STOPPING with the worker
// still recorded, before the stop is proved or the program FINISHED,
// standing in for an owner crash. It is skipped otherwise.
func TestCALV0074_WorkerRecoveryChild(t *testing.T) {
	raw := os.Getenv(workerRecoveryEnv + "_CONFIG")
	if raw == "" {
		t.Skip("child of TestCALV0074_WorkerRecoveryFromStopping")
	}
	var c store.ProgramConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	repo, err := intent.Resolve(os.Getenv(workerRecoveryEnv + "_ROOT"))
	if err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	w, err := store.OpenWorkflow(context.Background(), repo, operator(), "program", self, c, os.Getenv(workerRecoveryEnv+"_TICKET"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	store.SetRunFaultForTest(func(at string) error {
		if at == "refresh:STOPPING" {
			os.Exit(3)
		}
		return nil
	})
	_, err = w.RunRole(context.Background(), "implementer", "")
	t.Fatalf("owner was not interrupted at STOPPING: %v", err)
}

// TestCALV0074_WorkerRecoveryFromStopping proves that a replacement owner
// recovering a dead owner's STOPPING program whose attempt still records its
// worker settles it FINISHED with proved quiescence and its usage
// unobserved: the stage's host output was lost with the owner, so usage the
// program knew before cannot stay known. Token counters and turns are
// unchanged (CAL-V0-074, V1-1063; as CAL-V0-210 for a recorded proved stop).
// Before the fix the recovery kept usage known and the transaction refused it
// MALFORMED, leaving the program STOPPING.
func TestCALV0074_WorkerRecoveryFromStopping(t *testing.T) {
	ctx := context.Background()
	f := newClaudeFixture(t, supervisor.HostClaudeCode)
	config, err := json.Marshal(f.config)
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestCALV0074_WorkerRecoveryChild$", "-test.count=1")
	child.Env = append(os.Environ(), workerRecoveryEnv+"_CONFIG="+string(config), workerRecoveryEnv+"_ROOT="+f.s.repo.PrimaryWorktree, workerRecoveryEnv+"_TICKET="+f.ticketID)
	out, err := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("child did not stop at STOPPING: %v\n%s", err, out)
	}
	p := f.program(t, "program")
	if p.OwnerPID != child.Process.Pid || p.OwnerReleased || p.Phase != "STOPPING" || !p.UsageKnown || p.ResultClass == "NO_EXEC" {
		t.Fatalf("interrupted program %+v", p)
	}
	attempts, err := store.ProgramAttempts(ctx, f.s.repo)
	if err != nil {
		t.Fatal(err)
	}
	if a := attempts["program"]; a == nil || a.Supervision == nil || !a.Supervision.Worker || a.Phase != "STOPPING" {
		t.Fatalf("interrupted attempt %+v", a)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.OpenWorkflow(ctx, f.s.repo, operator(), "program", self, f.config, f.ticketID)
	q := f.program(t, "program")
	if q.Phase != "FINISHED" || q.Quiescence != "PROVED" || q.Epoch != p.Epoch+1 || q.OwnerPID != os.Getpid() {
		t.Fatalf("worker recovery from STOPPING not admitted (%v): %+v", err, q)
	}
	if q.UsageKnown || q.Turns != p.Turns || q.InputTokens != p.InputTokens || q.OutputTokens != p.OutputTokens {
		t.Fatalf("recovered program usage %+v from %+v", q, p)
	}
	if err != nil {
		t.Fatalf("takeover after recovery: %v", err)
	}
	if a := again.Attempt(); a == nil || a.Supervision == nil || a.Supervision.Worker {
		t.Fatalf("recovered attempt still records its worker: %+v", a)
	}
}
