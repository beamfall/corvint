//go:build darwin && (arm64 || amd64)

package groupreap

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDarwinProcessTableDescribesSelf(t *testing.T) {
	rows, err := darwinRows()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range rows {
		if r.pid == os.Getpid() {
			found = true
			if r.ppid != os.Getppid() || r.uid != os.Geteuid() || r.zombie {
				t.Fatalf("%+v", r)
			}
			id, alive, err := darwinIdentity(r.pid)
			if err != nil || !alive || id.start() != r.start() {
				t.Fatalf("%+v %v %v", id, alive, err)
			}
		}
	}
	if !found {
		t.Fatal("own process missing from kern.proc.all")
	}
}

// TestRetireHelper is re-executed: "detach" starts a Setpgid child and a
// setsid grandchild-of-child, records their pids, then sleeps.
func TestRetireHelper(t *testing.T) {
	switch os.Getenv("CORVINT_RETIRE_MODE") {
	case "detach":
		exe, _ := os.Executable()
		child := exec.Command(exe, "-test.run=^TestRetireHelper$")
		child.Env = append(os.Environ(), "CORVINT_RETIRE_MODE=orphaner")
		child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if child.Start() != nil {
			os.Exit(9)
		}
		for {
			time.Sleep(time.Second)
		}
	case "orphaner":
		exe, _ := os.Executable()
		sleeper := exec.Command(exe, "-test.run=^TestRetireHelper$")
		sleeper.Env = append(os.Environ(), "CORVINT_RETIRE_MODE=sleep")
		sleeper.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if sleeper.Start() != nil {
			os.Exit(9)
		}
		_ = os.WriteFile(os.Getenv("CORVINT_RETIRE_MARKER")+".tmp", []byte(fmt.Sprintf("%d %d", os.Getpid(), sleeper.Process.Pid)), 0600)
		_ = os.Rename(os.Getenv("CORVINT_RETIRE_MARKER")+".tmp", os.Getenv("CORVINT_RETIRE_MARKER"))
		for {
			time.Sleep(time.Second)
		}
	case "sleep":
		time.Sleep(10 * time.Minute)
		os.Exit(0)
	}
}

func readMarker(t *testing.T, path string) []int {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			var pids []int
			for _, f := range strings.Fields(string(b)) {
				n, err := strconv.Atoi(f)
				if err != nil {
					t.Fatal(err)
				}
				pids = append(pids, n)
			}
			return pids
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("readiness marker missing")
	return nil
}

// cleanupIdentity kills pid only if it still names the identity recorded
// while the test owned it.
func cleanupIdentity(t *testing.T, pid int) {
	t.Helper()
	row, alive, err := darwinIdentity(pid)
	if err != nil || !alive || row.zombie {
		return
	}
	start := row.start()
	t.Cleanup(func() {
		if r, ok, e := darwinIdentity(pid); e == nil && ok && r.start() == start {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
}

func alive(pid int) bool {
	r, ok, err := darwinIdentity(pid)
	return err == nil && ok && !r.zombie
}

// TestRetirerRetiresDetachedDescendants binds TRE-V0-025 on the actual
// process table: a Setpgid child escapes the group kill and a setsid
// grandchild escapes both; without the retirer they survive.
func TestRetirerRetiresDetachedDescendants(t *testing.T) {
	for _, mode := range []string{"none", "live-leader", "post-exit"} {
		t.Run(mode, func(t *testing.T) {
			retire := mode != "none"
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "ready")
			cmd := exec.Command(exe, "-test.run=^TestRetireHelper$")
			cmd.Env = append(os.Environ(), "CORVINT_RETIRE_MODE=detach", "CORVINT_RETIRE_MARKER="+marker)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			var r *Retirer
			if retire {
				if r, err = NewRetirer(); err != nil {
					t.Fatal(err)
				}
				cmd.Env = append(cmd.Env, OwnerEnvironmentKey+"="+r.Token())
			}
			done := make(chan error, 1)
			started := make(chan struct{})
			go func() {
				if e := cmd.Start(); e != nil {
					close(started)
					done <- e
					return
				}
				if r != nil {
					r.Leader(cmd.Process.Pid)
				}
				close(started)
				var hook func()
				if r != nil {
					hook = r.Retire
				}
				done <- wait(cmd, hook)
			}()
			<-started
			pids := readMarker(t, marker)
			for _, pid := range pids {
				cleanupIdentity(t, pid)
			}
			if mode == "live-leader" {
				// Timeout path: retire while the leader is live, then kill its group.
				r.Retire()
			}
			// post-exit: the leader dies first and its children are reparented,
			// so only the token proves ownership in the wait hook.
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
			time.Sleep(50 * time.Millisecond)
			for _, pid := range pids {
				if alive(pid) == retire {
					t.Fatalf("retire=%v pid %d alive=%v", retire, pid, alive(pid))
				}
			}
			if r != nil {
				got := r.Result()
				if !got.Clean() || len(got.Retired) != 2 {
					t.Fatalf("%+v", got)
				}
				if mode == "post-exit" && (got.Retired[0].Owner != OwnerToken || got.Retired[1].Owner != OwnerToken) {
					t.Fatalf("reparented descendants not proven by token: %+v", got)
				}
			}
		})
	}
}
