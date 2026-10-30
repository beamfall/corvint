//go:build darwin || linux

package cli_test

import (
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestV11721_LaunchLockWaitIsBoundedByWait: while a concurrent launcher of
// the same record holds its launch lock, --detach waits for that lock only
// within the caller's --wait: --wait 0 and no --wait return LOCK_TIMEOUT at
// once, --wait 1 within its bound, and nothing is recorded (ATR-V0-024,
// ATR-V0-095, V1-1721). Before, the launch-lock wait ran a fixed 10 seconds
// outside the --wait deadline.
func TestV11721_LaunchLockWaitIsBoundedByWait(t *testing.T) {
	root, repo, id := detachClaimStore(t)
	dir := cli.LeaseDetachDir(repo, "claim-contended")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	launcher, err := os.OpenFile(filepath.Join(dir, "lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = launcher.Close() })
	if err := syscall.Flock(int(launcher.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	claim := []string{"claim", id, "--holder", "agent", "--request-id", "claim-contended", "--detach"}
	for _, c := range []struct {
		wait []string
		want time.Duration
	}{
		{[]string{"--wait", "0"}, 0},
		{nil, 0},
		{[]string{"--wait", "1"}, time.Second},
	} {
		started := time.Now()
		x := cliRun(t, root, append(append([]string{}, claim...), c.wait...)...)
		if elapsed := time.Since(started); elapsed > c.want+2*time.Second {
			t.Fatalf("%v returned after %v under launcher contention, bound %v", c.wait, elapsed, c.want)
		}
		if x.code == 0 || !slices.Contains(x.res.Codes, wire.CodeLockTimeout) {
			t.Fatalf("%v under launcher contention: code %d %s, want LOCK_TIMEOUT", c.wait, x.code, x.stdout)
		}
		if _, err := os.Lstat(filepath.Join(dir, "request.json")); !os.IsNotExist(err) {
			t.Fatalf("%v recorded the request without the launch lock: %v", c.wait, err)
		}
	}
	// Once the other launcher releases, the same --detach launches.
	if err := syscall.Flock(int(launcher.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	if again := cliRun(t, root, append(append([]string{}, claim...), "--wait", "30")...); again.code != 0 || again.res.Outcome != wire.OutcomeOK {
		t.Fatalf("--detach after the release: code %d %s", again.code, again.stdout)
	}
}
