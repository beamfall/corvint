//go:build darwin || linux

package procgroup

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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
	rows, err := descendantSnapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := signalObservedProcess(rows, ObservedProcess{PID: os.Getpid(), Start: "not-this-generation"}); err != nil {
		t.Fatal(err)
	}
}

func TestObservedExitedZombieIsNotSurvivor(t *testing.T) {
	t.Run("AHI-032 exited zombie is not a cleanup survivor", func(t *testing.T) {
		cmd := exec.Command("/bin/sleep", "60")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer cmd.Wait()
		defer cmd.Process.Kill()
		rows, err := descendantSnapshot(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		owned, ok := rows[cmd.Process.Pid]
		if !ok {
			t.Fatal("child identity missing")
		}
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		for {
			rows, err = descendantSnapshot(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if current, ok := rows[owned.PID]; ok && current.Start == owned.Start && strings.HasPrefix(current.State, "Z") {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("fixture did not become an unreaped zombie")
			case <-time.After(20 * time.Millisecond):
			}
		}
		done := make(chan struct{})
		close(done)
		observer := descendantObserver{root: os.Getpid(), known: map[int]ObservedProcess{owned.PID: owned}, stop: make(chan struct{}), done: done}
		report, err := observer.finish()
		if err != nil || !report.Absent {
			t.Fatalf("exited child reported as live: %v %+v", err, report)
		}
	})
}

func TestObservedDescendantSnapshotLossIsRetried(t *testing.T) {
	unavailable := errors.New("descendant snapshot unavailable")
	t.Run("AHI-032 a lost periodic snapshot is a limitation, not a survivor", func(t *testing.T) {
		var calls atomic.Int32
		o := &descendantObserver{snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			if n := calls.Add(1); n > 1 && n < 4 {
				return nil, unavailable
			}
			return descendantSnapshot(ctx)
		}}
		// The root is a child: under this process the observer's own ps runs would be descendants.
		root := exec.Command("/bin/sleep", "60")
		if err := root.Start(); err != nil {
			t.Fatal(err)
		}
		defer root.Wait()
		defer root.Process.Kill()
		started, err := startObserver(o, root.Process.Pid)
		if err != nil {
			t.Fatal(err)
		}
		for calls.Load() < 4 {
			time.Sleep(descendantInterval)
		}
		report, err := started.finish()
		if err != nil || !report.Absent || len(report.Failures) != 0 {
			t.Fatalf("lost periodic snapshot failed cleanup: %v %+v", err, report)
		}
		if last := report.Limitations[len(report.Limitations)-1]; !strings.HasPrefix(last, "2 periodic snapshots were unavailable") {
			t.Fatalf("lost snapshots not disclosed: %q", last)
		}
	})
	t.Run("AHI-032 the final sweep retries an unavailable snapshot", func(t *testing.T) {
		calls := 0
		done := make(chan struct{})
		close(done)
		o := descendantObserver{root: os.Getpid(), known: map[int]ObservedProcess{}, stop: make(chan struct{}), done: done, snap: func(ctx context.Context) (map[int]ObservedProcess, error) {
			if calls++; calls < 4 {
				return nil, unavailable
			}
			return descendantSnapshot(ctx)
		}}
		report, err := o.finish()
		if err != nil || !report.Absent || calls != 4 {
			t.Fatalf("final sweep did not retry: calls=%d %v %+v", calls, err, report)
		}
	})
	t.Run("AHI-032 a snapshot unavailable for the whole settle window still fails", func(t *testing.T) {
		done := make(chan struct{})
		close(done)
		o := descendantObserver{root: os.Getpid(), known: map[int]ObservedProcess{}, stop: make(chan struct{}), done: done, snap: func(context.Context) (map[int]ObservedProcess, error) {
			return nil, unavailable
		}}
		began := time.Now()
		report, err := o.finish()
		if err == nil || report.Absent || len(report.Failures) != 1 || report.Failures[0] != unavailable.Error() {
			t.Fatalf("persistent snapshot loss passed: %v %+v", err, report)
		}
		if waited := time.Since(began); waited < descendantSettle || waited > descendantSettle+descendantSnapshotTimeout {
			t.Fatalf("settle window not bounded: %v", waited)
		}
	})
}
