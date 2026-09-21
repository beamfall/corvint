//go:build darwin || linux

package procgroup

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestObservedDescendantHelper(t *testing.T) {
	role := os.Getenv("CORVINT_OBSERVED_DESCENDANT_HELPER")
	if role == "" {
		return
	}
	if role == "child" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestObservedDescendantHelper$")
	command.Env = append(os.Environ(), "CORVINT_OBSERVED_DESCENDANT_HELPER=child")
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("CORVINT_DESCENDANT_PID"), []byte(strconv.Itoa(command.Process.Pid)), 0600); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		os.Exit(3)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	time.Sleep(time.Minute)
}

func TestObservedDescendantCancellationReapsEscapedChild(t *testing.T) {
	pidPath := t.TempDir() + "/child.pid"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan Observation, 1)
	go func() {
		done <- Run(ctx, Spec{Argv: []string{os.Args[0], "-test.run=^TestObservedDescendantHelper$"}, Dir: filepath.Dir(pidPath), Env: append(os.Environ(), "CORVINT_OBSERVED_DESCENDANT_HELPER=parent", "CORVINT_DESCENDANT_PID="+pidPath), Timeout: 5 * time.Second, ObserveDescendants: true})
	}()
	var data []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(pidPath)
		if len(data) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(data) == 0 {
		cancel()
		<-done
		t.Fatal("detached fixture failed to start")
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	if pgid, err := syscall.Getpgid(pid); err != nil || pgid != pid {
		cancel()
		<-done
		t.Fatalf("child did not escape group: %d %v", pgid, err)
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	o := <-done
	if !o.Cancelled || o.DescendantObservation == nil || !o.DescendantObservation.Absent || len(o.DescendantObservation.Processes) == 0 {
		t.Fatalf("unresolved cleanup: %+v report=%+v", o, o.DescendantObservation)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("escaped child remains: %v", err)
	}
}

func TestObservedDescendantIdentityReuseDoesNotExpandOwnership(t *testing.T) {
	o := descendantObserver{known: map[int]ObservedProcess{12: {PID: 12, Start: "old"}}}
	o.expand(map[int]ObservedProcess{12: {PID: 12, Start: "new"}, 13: {PID: 13, ParentPID: 12, Start: "child"}})
	if len(o.known) != 1 {
		t.Fatal("PID reuse admitted an unrelated descendant")
	}
	if err := signalObservedProcess(context.Background(), ObservedProcess{PID: os.Getpid(), Start: "not-this-generation"}); err != nil {
		t.Fatal(err)
	}
}
