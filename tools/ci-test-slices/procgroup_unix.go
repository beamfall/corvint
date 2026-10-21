//go:build unix

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/groupreap"
)

// host is the groupreap host the subprocess owner uses; tests record its
// group signals and reap to pin their order (PGO-V0-019). `go` keeps forking
// compilers and test binaries, so the group SIGKILL repeats on each pre-reap
// live observation (PGO-V0-017).
var host = groupreap.Primitives{RepeatKillWhileLive: true}

// runOwned runs cmd as the leader of a new process group owned through its
// unreaped leader (groupreap.Owner), so `go` and the compilers, linkers and
// test binaries it started are all retired. Cancellation interrupts the whole
// group, kills it if the leader has not exited after stopGrace, and returns
// only once the group is retired. Every group signal, including the
// interrupt, is sent while the leader is held unreaped, so no signal can name
// a reused group ID after the reap (V1-1387); after the reap only signal-0
// observations remain. A group whose retirement is not proved within its
// bound (HOLD) is an error, never a success, and so is a platform without
// the owner. A descendant that leaves the group (setsid or setpgid) is not
// reached.
func runOwned(ctx context.Context, cmd *exec.Cmd) error {
	owner, err := groupreap.StartWith(cmd, host)
	if err != nil {
		return err
	}
	select {
	case <-owner.Exited():
	case <-ctx.Done():
		_ = owner.Signal(syscall.SIGINT)
		grace := time.NewTimer(stopGrace)
		select {
		case <-owner.Exited():
		case <-grace.C:
		}
		grace.Stop()
		owner.Stop()
	}
	bound, cancel := context.WithTimeout(context.Background(), stopGrace)
	defer cancel()
	result := owner.FinishBounded(groupreap.RetirementBound{Done: bound.Done()})
	if result.State != groupreap.Released {
		return fmt.Errorf("process group retirement %s: %v", result.State, result.Err)
	}
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), result.WaitErr)
	}
	return result.WaitErr
}
