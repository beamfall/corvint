//go:build darwin || linux

package store_test

import (
	"encoding/json"
	"fmt"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
)

func init() { leaderFault = forgedIdentityLeader; checkTestLeaders = checkOwnTestLeaders }

// forgedIdentityLeader boots with a start identity that is not its own, so
// the supervisor refuses it after the spawn with an empty outcome class. A
// detached holder in its own session keeps standard output open past the
// drain, so the drain cannot prove the output closed.
func forgedIdentityLeader(dir string) {
	raw, e := os.ReadFile(filepath.Join(dir, "capsule"))
	var c supervisor.Capsule
	if e == nil {
		e = json.Unmarshal(raw, &c)
	}
	if e != nil {
		os.Exit(1)
	}
	holder := exec.Command(os.Args[0], "output-holder")
	holder.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	holder.Stdout = os.Stdout
	if e = holder.Start(); e != nil {
		os.Exit(1)
	}
	started, err := supervisor.ProcessIdentity(holder.Process.Pid)
	if err != nil || started == "" {
		_ = holder.Process.Kill()
		_ = holder.Wait()
		os.Exit(1)
	}
	if err = supervisor.Publish(dir, "test-holder", supervisor.Boot{PID: holder.Process.Pid, Started: started}); err != nil {
		_ = holder.Process.Kill()
		_ = holder.Wait()
		os.Exit(1)
	}
	if e = supervisor.Publish(dir, "boot", supervisor.Boot{Effect: c.Effect, PID: os.Getpid(), Started: "forged"}); e != nil {
		os.Exit(1)
	}
	time.Sleep(90 * time.Second)
	os.Exit(1)
}

// ATR-V0-7202: even the deliberately unowned-identity fault helper is reaped
// by its test before returning. These actual identities are test cleanup
// inputs only, never facts supplied to the native workflow.
func reapFaultLeader(t *testing.T, p snapshot.Program) {
	t.Helper()
	owned := map[int]string{}
	if p.LeaderPID > 1 {
		if id, err := supervisor.ProcessIdentity(p.LeaderPID); err == nil && id != "" {
			owned[p.LeaderPID] = id
		}
	}
	var holder supervisor.Boot
	if raw, err := os.ReadFile(filepath.Join(p.EffectDirectory, "test-holder")); err == nil && json.Unmarshal(raw, &holder) == nil && holder.PID > 1 && holder.Started != "" {
		owned[holder.PID] = holder.Started
	}
	t.Cleanup(func() {
		for pid, id := range owned {
			if now, _ := supervisor.ProcessIdentity(pid); now == id {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			live := false
			for pid, id := range owned {
				if now, _ := supervisor.ProcessIdentity(pid); now == id {
					live = true
				}
			}
			if !live {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Error("fault leader/holder outlived cleanup")
	})
}

// At package exit, check only this exact test executable, never other lanes.
// Directory-loss retirement is allowed its bounded scheduling interval.
func checkOwnTestLeaders() error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		raw, err := exec.Command("/bin/ps", "-axo", "pid=,args=").Output()
		if err != nil {
			return err
		}
		owned := map[int]string{}
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 || fields[1] != self || fields[2] != "lane-leader" {
				continue
			}
			pid, err := strconv.Atoi(fields[0])
			if err != nil || pid <= 1 {
				continue
			}
			if id, err := supervisor.ProcessIdentity(pid); err == nil && id != "" {
				owned[pid] = id
			}
		}
		if len(owned) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			for pid, id := range owned {
				if now, _ := supervisor.ProcessIdentity(pid); now == id {
					_ = syscall.Kill(-pid, syscall.SIGKILL)
				}
			}
			return fmt.Errorf("ATR-V0-7202: test lane leaders outlived package cleanup: %v", owned)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// TestATRV7202_ChildExitLeavesRootLeaders: a test child re-executing this
// binary must not take the package-exit leader check, since every live leader
// of the root's parallel tests shares its executable path. Before the fix the
// child waited its grace, killed the root's leader and exited 1 (V1-4782).
func TestATRV7202_ChildExitLeavesRootLeaders(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	leader := exec.Command(self, "lane-leader", "--directory", t.TempDir(), "--capsule", "idle")
	leader.Env = append(os.Environ(), idleLeaderEnv+"=1")
	leader.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	hold, err := leader.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = leader.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- leader.Wait() }()
	t.Cleanup(func() {
		_ = hold.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = leader.Process.Kill()
			<-done
		}
	})
	child := exec.Command(self, "-test.run=^$", "-test.count=1")
	if out, err := child.CombinedOutput(); err != nil {
		t.Fatalf("child exit check: %v\n%s", err, out)
	}
	select {
	case err := <-done:
		done <- err
		t.Fatalf("root leader did not outlive the child: %v", err)
	default:
	}
}
