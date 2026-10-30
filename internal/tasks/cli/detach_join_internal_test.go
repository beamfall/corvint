//go:build darwin || linux

package cli

import (
	"sync"
	"testing"
	"time"
)

// detachJoinBound bounds how long a test, and TestMain after the last test,
// waits for the detached children it launched to end.
const detachJoinBound = 60 * time.Second

// detachedChildren are the detached children launched in this binary, by the
// working directory of the launch, each with the channel its launcher closes
// once it has reaped it.
var detachedChildren struct {
	sync.Mutex
	byDir map[string][]<-chan struct{}
}

func init() {
	detachLaunched = func(cwd string, exited <-chan struct{}) {
		detachedChildren.Lock()
		defer detachedChildren.Unlock()
		if detachedChildren.byDir == nil {
			detachedChildren.byDir = map[string][]<-chan struct{}{}
		}
		detachedChildren.byDir[cwd] = append(detachedChildren.byDir[cwd], exited)
	}
}

// joinDetached waits up to bound for every detached child launched from dir,
// or from anywhere when dir is "", to end and be reaped. It returns how many
// still run; it never signals one.
func joinDetached(dir string, bound time.Duration) int {
	detachedChildren.Lock()
	var waits []<-chan struct{}
	for d, chans := range detachedChildren.byDir {
		if dir == "" || d == dir {
			waits = append(waits, chans...)
		}
	}
	detachedChildren.Unlock()
	deadline := time.NewTimer(bound)
	defer deadline.Stop()
	for i, exited := range waits {
		select {
		case <-exited:
		case <-deadline.C:
			left := 0
			for _, rest := range waits[i:] {
				select {
				case <-rest:
				default:
					left++
				}
			}
			return left
		}
	}
	return 0
}

// JoinDetachedChildren registers a cleanup that waits, bounded, for every
// detached claim, pool acquire or run child launched with root as its
// working directory to end before the test's cleanup finishes. A detached
// result is readable before its child exits, so a test that has read it may
// otherwise end with the child still running, which the leak check of
// ATR-V0-310 rightly fails. A child still running at the bound fails the
// test and is not signalled.
func JoinDetachedChildren(t *testing.T, root string) {
	t.Helper()
	t.Cleanup(func() {
		if left := joinDetached(root, detachJoinBound); left > 0 {
			t.Errorf("ATR-V0-310: %d detached children launched under %s were still running %v after the test", left, root, detachJoinBound)
		}
	})
}
