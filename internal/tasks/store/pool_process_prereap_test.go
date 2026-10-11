//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"sync"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// V1-1036: the pool's own post-reap group observation never signals. The
// first observation is made to report the group live, as a group ID reused
// after the reap would; the runner must keep it as an observation (signal 0
// only) and send no SIGKILL to the numeric group. The real group signal is
// groupreap's, sent while the exited leader is unreaped.
func TestV11036PoolPostReapObservationSendsNoGroupKill(t *testing.T) {
	for _, captured := range []bool{false, true} {
		t.Run(fmt.Sprintf("captured=%v", captured), func(t *testing.T) {
			var mu sync.Mutex
			var kills []int
			observations, leader := 0, 0
			previousSignal, previousProbe := poolGroupSignal, poolProbe
			t.Cleanup(func() { poolGroupSignal, poolProbe = previousSignal, previousProbe })
			poolGroupSignal = func(pid int, signal syscall.Signal) error {
				mu.Lock()
				defer mu.Unlock()
				if pid < 0 && signal != 0 {
					kills = append(kills, -pid)
					return nil
				}
				if pid < 0 {
					observations, leader = observations+1, -pid
					if observations == 1 {
						return nil // the reused-looking group answers signal 0
					}
					return syscall.ESRCH
				}
				return previousSignal(pid, signal)
			}
			poolProbe = func(context.Context, ...string) ([]byte, error) {
				mu.Lock()
				defer mu.Unlock()
				return []byte(fmt.Sprintf("%d S\n", leader)), nil // a live member
			}
			def := &intent.PoolCommand{Argv: []string{"/bin/sh", "-c", "exit 0"}, TimeoutSeconds: "5"}
			var class string
			var clean bool
			if captured {
				result := executePoolCaptured(context.Background(), def, t.TempDir(), nil)
				class, clean = result.Class, result.Clean
			} else {
				class, clean, _ = executePool(context.Background(), def, t.TempDir(), nil)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(kills) != 0 {
				t.Fatalf("post-reap group kill sent to %v", kills)
			}
			if observations < 2 {
				t.Fatalf("post-reap observation not reached: %d", observations)
			}
			if class != "EXIT_ZERO" || !clean {
				t.Fatalf("%s clean=%v", class, clean)
			}
		})
	}
}
