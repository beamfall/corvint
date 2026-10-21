//go:build darwin || linux

package dispatch

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

// launchedGroup is one worker process group a test started: launch makes
// every worker a session and group leader, so pid is also the group ID.
type launchedGroup struct {
	pid int
	id  string
}

var launchedGroups struct {
	sync.Mutex
	all []launchedGroup
}

func init() {
	launchObserved = func(pid int, id string) {
		launchedGroups.Lock()
		launchedGroups.all = append(launchedGroups.all, launchedGroup{pid, id})
		launchedGroups.Unlock()
	}
}

func launchedSince(from int) []launchedGroup {
	launchedGroups.Lock()
	defer launchedGroups.Unlock()
	return append([]launchedGroup(nil), launchedGroups.all[from:]...)
}

func launchedCount() int {
	launchedGroups.Lock()
	defer launchedGroups.Unlock()
	return len(launchedGroups.all)
}

// groupOwned reports whether pid can still name the group a test started:
// either the leader runs with its recorded identity, or no process holds
// the pid. A pid is not reused while its process group exists, so a
// different identity means the recorded group is already gone.
func groupOwned(g launchedGroup) bool {
	id, _ := identityOf(g.pid)
	return id == "" || id == g.id
}

func groupAlive(g launchedGroup) bool {
	return groupOwned(g) && syscall.Kill(-g.pid, 0) == nil
}

// reapLaunched stops every worker group recorded at or after index from
// with SIGKILL and waits, bounded, until each group is empty (V1-0948,
// CAL-V0-353). It returns the groups that were alive before the kill and
// those still alive after the wait.
func reapLaunched(from int) (alive, survivors []int) {
	groups := launchedSince(from)
	for _, g := range groups {
		if groupAlive(g) {
			alive = append(alive, g.pid)
			_ = syscall.Kill(-g.pid, syscall.SIGKILL)
		}
	}
	for _, g := range groups {
		gone := false
		for i := 0; i < 250 && !gone; i++ {
			if gone = !groupAlive(g); !gone {
				time.Sleep(20 * time.Millisecond)
			}
		}
		if !gone {
			survivors = append(survivors, g.pid)
		}
	}
	return alive, survivors
}

// reapOnCleanup registers a cleanup that stops and reaps every worker group
// the test launches from now on, including after t.Fatal. testConfig calls
// it, so it runs after the test's own cleanups such as Dispatcher.Close.
func reapOnCleanup(t *testing.T) {
	t.Helper()
	from := launchedCount()
	t.Cleanup(func() {
		if _, left := reapLaunched(from); len(left) > 0 {
			t.Errorf("V1-0948: worker process groups %v survived SIGKILL", left)
		}
	})
}

// TestMain is the maintained leak check (CAL-V0-353): after every test has
// run its cleanups, no worker group any test launched may still be alive.
// A leaked group is killed and fails the package.
func TestMain(m *testing.M) {
	code := m.Run()
	alive, left := reapLaunched(0)
	if len(alive) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: V1-0948: dispatch tests left worker process groups alive: %v (killed; still alive: %v)\n", alive, left)
		code = 1
	}
	os.Exit(code)
}

func TestCALV0353_CleanupReapsTheWholeWorkerGroup(t *testing.T) {
	from := launchedCount()
	pid, _, exit, err := launch([]string{"/bin/sh", "-c", "/bin/sleep 300 & /bin/sleep 300; wait"}, nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reapLaunched(from) })
	members := func() int {
		procs, err := observeProcs()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for p, row := range procs {
			if row.pgid == pid && p != pid {
				n++
			}
		}
		return n
	}
	for i := 0; i < 250 && members() < 2; i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if n := members(); n < 2 {
		t.Fatalf("worker group has %d descendants, want 2 sleeps", n)
	}
	alive, left := reapLaunched(from)
	if len(alive) != 1 || alive[0] != pid {
		t.Fatalf("leak check saw %v alive, want [%d]: the TestMain check would miss this leak", alive, pid)
	}
	if len(left) > 0 || members() != 0 {
		t.Fatalf("group %d not reaped: survivors %v, %d descendants left", pid, left, members())
	}
	select {
	case <-exit:
	case <-time.After(10 * time.Second):
		t.Fatal("killed leader was not reaped")
	}
	if alive, _ := reapLaunched(from); len(alive) != 0 {
		t.Fatalf("leak check still sees %v after the reap", alive)
	}
}
