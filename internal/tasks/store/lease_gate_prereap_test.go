//go:build darwin || linux

package store

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-1047: a lease gate's group is SIGKILLed while its exited leader is
// still unreaped. Here the leader exits at once and leaves a group member
// that writes past the output cap a second later. With the old order the
// leader was reaped first and that overflow later triggered a group kill
// after the reap (OUTPUT_LIMIT); now the member is gone before the reap, so
// the run is the leader's own exit and no kill follows the reap.
func TestV11047LeaseGateKillsGroupBeforeTheReap(t *testing.T) {
	over := wire.MaxGateOutputBytes + 1<<20
	script := "(/bin/sleep 1; /usr/bin/head -c " + strconv.Itoa(over) + " /dev/zero) & exit 0"
	def := &intent.GateDefinition{Argv: []string{"/bin/sh", "-c", script}, TimeoutSeconds: wire.CountOf(30)}
	out := &cappedOutput{}
	run := execute(context.Background(), def, t.TempDir(), []string{}, out)
	if run.class != "EXIT" || run.exitCode == nil || run.exitCode.Int() != 0 {
		t.Fatalf("class=%s exit=%v signal=%v overflow=%v, want EXIT 0", run.class, run.exitCode, run.signal, out.overflow)
	}
	if out.overflow {
		t.Fatal("the leftover group member wrote after the leader was reaped")
	}
}

// PGO-V0-009: RunBatchCommand sends no group signal of its own after the
// gate is reaped. The gate leaves a background member; reapGate's pre-reap
// sweep must retire it, and the store's group-signal hook must stay unused,
// because after the reap the gate's pid, and so its group id, can name a
// process that reused it.
func TestPGOV0009BatchCommandSendsNoGroupSignalAfterTheReap(t *testing.T) {
	var signalled []int
	saved := signalGateGroup
	signalGateGroup = func(pid int, sig syscall.Signal) error {
		signalled = append(signalled, pid)
		return saved(pid, sig)
	}
	t.Cleanup(func() { signalGateGroup = saved })
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "member.pid")
	script := "/bin/sleep 30 & echo $! > " + pidFile + "; exit 0"
	run := RunBatchCommand(context.Background(), []string{"/bin/sh", "-c", script}, dir, []string{}, 30)
	if !run.Passed() {
		t.Fatalf("run = %+v, want EXIT 0", run)
	}
	if len(signalled) != 0 {
		t.Fatalf("store signalled groups %v after the gate was reaped", signalled)
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	member, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(member, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(member, syscall.SIGKILL)
			t.Fatalf("background member %d outlived the batch command", member)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
