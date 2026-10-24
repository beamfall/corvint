//go:build darwin || linux

package store_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/transaction"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

const killedGateEnv = "CORVINT_TEST_CTS026_KILLED_GATE"

// TestCTSV0026_KilledGateRunLeavesAReadableStore (V1-2506): a native gate run
// SIGKILLed mid-publish, while a staging slot it prepared is still occupied,
// leaves that descriptor-less slot behind, as the Pi adapter's abort does.
// ticket show, queue status and receipt audit then read the store instead of
// refusing MALFORMED "unassigned stage slot" for good: the writer lock is
// free, which proves the slot a killed writer's orphan (CTS-V0-026). The next
// writer still removes it.
func TestCTSV0026_KilledGateRunLeavesAReadableStore(t *testing.T) {
	if os.Getenv(killedGateEnv) != "" {
		killedGateChild(t)
		return
	}
	s := newGateStore(t)
	id := s.ticket(t, "one")
	claim, _ := s.submitted(t, id, "src", 0)
	child := exec.Command(os.Args[0], "-test.run=^TestCTSV0026_KilledGateRunLeavesAReadableStore$", "-test.count=1")
	child.Env = append(os.Environ(), killedGateEnv+"="+strings.Join([]string{s.repo.PrimaryWorktree, s.root, claim.AttemptID, string(claim.Generation), string(s.at(t, 2))}, "\n"))
	out, err := child.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
		t.Fatalf("the gate run was not killed mid-publish: %v\n%s", err, out)
	}
	staging := filepath.Join(s.repo.StateDir, "staging")
	orphans, err := filepath.Glob(filepath.Join(staging, "a*"))
	if err != nil || len(orphans) == 0 {
		t.Fatalf("the killed gate run left no staging slot: %v %v", orphans, err)
	}
	if _, err := os.Lstat(filepath.Join(staging, "active.json")); !os.IsNotExist(err) {
		t.Fatalf("the killed gate run left a stage descriptor: %v", err)
	}
	for _, args := range [][]string{{"ticket", "show", id}, {"queue", "status"}, {"receipt", "audit"}} {
		var stdout bytes.Buffer
		code := cli.Run(cli.Env{Cwd: s.repo.PrimaryWorktree, Args: args, Stdout: &stdout})
		res, err := wire.DecodeResult(stdout.Bytes())
		if code != 0 || err != nil || res.Outcome != wire.OutcomeOK {
			t.Fatalf("%v after a killed gate run: exit %d %v\n%s", args, code, err, stdout.Bytes())
		}
		if args[0] == "receipt" && !strings.Contains(strings.Join(res.Warnings, "\n"), "staging/"+filepath.Base(orphans[0])) {
			t.Fatalf("receipt audit does not name the orphan slot: %v", res.Warnings)
		}
	}
	auditOK(t, s.repo)
	// The next writer clears the orphan under the lock (CAL-V0-019).
	if _, err := s.gate(t, "gate-after-kill", gateOf(claim, "verify"), 3); err != nil {
		t.Fatalf("gate run after the kill: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(staging, "a*")); len(left) != 0 {
		t.Fatalf("the next writer left the orphan: %v", left)
	}
}

// killedGateChild runs the gate and SIGKILLs its own process at the first
// artifact publication that finds a staging slot occupied.
func killedGateChild(t *testing.T) {
	parts := strings.Split(os.Getenv(killedGateEnv), "\n")
	if len(parts) != 5 {
		t.Fatalf("child input %q", parts)
	}
	repo, err := intent.Resolve(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.Parse("2006-01-02T15:04:05Z", parts[4])
	if err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(repo.StateDir, "staging")
	defer store.SetPublishFaultForTest(func(transaction.Artifact) error {
		if slots, _ := filepath.Glob(filepath.Join(staging, "a*")); len(slots) > 0 {
			_ = syscall.Kill(os.Getpid(), syscall.SIGKILL)
			select {}
		}
		return nil
	})()
	lease := transaction.LeaseRequest{Verb: transaction.LeaseGateRun, AttemptID: parts[2], Generation: wire.Size(parts[3]), Gate: "verify"}
	choice := store.LeaseChoice{QueueID: fixture.QueueID, RequestID: "gate-killed", Root: parts[1], Lease: lease}
	report, err := store.GateRun(context.Background(), repo, operator(), choice, parts[1], func() time.Time { return at })
	t.Fatalf("the gate run finished without reaching an occupied slot: %+v %v", report, err)
}
