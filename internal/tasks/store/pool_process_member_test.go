//go:build darwin || linux

package store

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

// poolMemberRunning reports whether pid stays a running (non-zombie) process
// for the whole window, read from ps rather than from any pool seam.
func poolMemberRunning(pid int, window time.Duration) bool {
	for deadline := time.Now().Add(window); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		state := strings.TrimSpace(string(out))
		if err != nil || state == "" || strings.HasPrefix(state, "Z") {
			return false
		}
	}
	return true
}

// V1-1036 negative control on real process state: a group member still
// running when the leader exits is SIGKILLed before the leader is reaped.
// The pool's first post-reap group listing (poolProbe "-axo", present in the
// base code too) must therefore find the member already killed, or never run
// because the group is gone. The base code, which reaped first and SIGKILLed
// the group only after that listing, leaves the member running throughout.
func TestV11036PoolGroupMemberIsKilledBeforeReap(t *testing.T) {
	for _, captured := range []bool{false, true} {
		t.Run(fmt.Sprintf("captured=%v", captured), func(t *testing.T) {
			dir := t.TempDir()
			pidPath := filepath.Join(dir, "member.pid")
			member := func() int {
				raw, err := os.ReadFile(pidPath)
				if err != nil {
					return 0
				}
				pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
				return pid
			}
			t.Cleanup(func() {
				if pid := member(); pid > 1 {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			var mu sync.Mutex
			listings, survived := 0, false
			previous := poolProbe
			t.Cleanup(func() { poolProbe = previous })
			poolProbe = func(ctx context.Context, argv ...string) ([]byte, error) {
				if len(argv) > 1 && argv[1] == "-axo" {
					mu.Lock()
					listings++
					if listings == 1 {
						if pid := member(); pid > 1 && poolMemberRunning(pid, 500*time.Millisecond) {
							survived = true
						}
					}
					mu.Unlock()
				}
				return previous(ctx, argv...)
			}
			script := "sleep 30 >/dev/null 2>&1 & echo $! > '" + pidPath + "'; exit 0"
			def := &intent.PoolCommand{Argv: []string{"/bin/sh", "-c", script}, TimeoutSeconds: "5"}
			if captured {
				_ = executePoolCaptured(context.Background(), def, dir, nil)
			} else {
				_, _, _ = executePool(context.Background(), def, dir, nil)
			}
			if member() <= 1 {
				t.Fatal("group member pid not recorded")
			}
			mu.Lock()
			defer mu.Unlock()
			if survived {
				t.Fatal("group member was still running at the post-reap listing: the group was not killed before the reap")
			}
			if poolMemberRunning(member(), 50*time.Millisecond) {
				t.Fatal("group member survived the pool run")
			}
		})
	}
}
