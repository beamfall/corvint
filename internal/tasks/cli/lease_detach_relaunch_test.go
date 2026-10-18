//go:build darwin || linux

package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestATRV0095_RelaunchWaitsOutTheFinishingChild: after a detached child
// keeps a LOCK_TIMEOUT, a repeat --detach waits for the running lock the
// finishing child still holds, and then relaunches even while that child's
// process is still visible under its recorded identity (ATR-V0-095,
// V1-1671). Before ATR-V0-095 the first case returned the kept LOCK_TIMEOUT
// and the second found the exiting child live, started nothing and then
// reported SUPERVISOR_LOST.
func TestATRV0095_RelaunchWaitsOutTheFinishingChild(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	t.Setenv("CORVINT_TEST_LEASE_DETACH_CEILING_MS", "1500")
	// Every recorded child stays visible with its recorded identity, as the
	// child that kept the result is between its lock release and its reaping.
	t.Cleanup(cli.SetLeaseDetachIdentity(func(int) (string, error) { return "test:exiting", nil }))
	held := holdPreparation(t, repo)
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-exiting", "--detach", "--wait", "30"}
	first := cliRun(t, root, claim...)
	if first.code == 0 || first.code == 75 || !slices.Contains(first.res.Codes, wire.CodeLockTimeout) {
		t.Fatalf("ceiling: code %d %s", first.code, first.stdout)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	// The finishing child still holds the running lock for a moment.
	dir := cli.LeaseDetachDir(repo, "claim-exiting")
	running, err := os.OpenFile(filepath.Join(dir, "running"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer running.Close()
	if err := syscall.Flock(int(running.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(300*time.Millisecond, func() { _ = syscall.Flock(int(running.Fd()), syscall.LOCK_UN) })
	again := cliRun(t, root, claim...)
	if again.code != 0 || again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("repeated --detach after the ceiling: code %d %s", again.code, again.stdout)
	}
	if prev, err := os.ReadFile(filepath.Join(dir, "result.1.json")); err != nil || !bytes.Equal(prev, first.stdout) {
		t.Fatalf("result.1.json: %v %s, want %s", err, prev, first.stdout)
	}
}
