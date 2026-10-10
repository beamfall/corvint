//go:build darwin || linux

package dispatch

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

// V1-1036: launch kills an unidentified worker's group while the leader is
// still unreaped, so the numeric group ID cannot name a reused group. The
// seam sends the real kill, gives a concurrent Wait time to reap, then
// requires the leader to still exist as an unreaped zombie.
func TestV11036LaunchKillsUnidentifiedGroupBeforeReap(t *testing.T) {
	previousIdentity, previousKill := processIdentity, launchGroupKill
	t.Cleanup(func() { processIdentity, launchGroupKill = previousIdentity, previousKill })
	processIdentity = func(int) (string, error) { return "", errors.New("injected") }
	calls := 0
	var held error
	launchGroupKill = func(pid int, signal syscall.Signal) error {
		calls++
		err := syscall.Kill(pid, signal)
		time.Sleep(200 * time.Millisecond)
		held = syscall.Kill(-pid, 0) // the leader (pid == pgid) is unreaped
		return err
	}
	_, _, _, err := launch([]string{"/bin/sleep", "30"}, nil, t.TempDir(), t.TempDir())
	var started *startedError
	if !errors.As(err, &started) {
		t.Fatalf("launch error = %v, want startedError", err)
	}
	select {
	case <-started.exit:
	case <-time.After(10 * time.Second):
		t.Fatal("killed leader was not reaped")
	}
	if calls != 1 {
		t.Fatalf("group kill calls = %d, want 1", calls)
	}
	if held != nil && !errors.Is(held, syscall.EPERM) {
		t.Fatalf("leader was reaped before the group kill returned: %v", held)
	}
}
