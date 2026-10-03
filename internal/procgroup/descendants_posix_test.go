//go:build darwin || linux

package procgroup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
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
	if len(o.known) != 0 {
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
		// Its own child is observed by the first snapshot and outlives the lost ones.
		root := exec.Command("/bin/sh", "-c", "/bin/sleep 60 & wait")
		ready, err := root.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		root.Args[2] = "/bin/sleep 60 & echo ready; wait"
		if err := root.Start(); err != nil {
			t.Fatal(err)
		}
		defer root.Wait()
		defer root.Process.Kill()
		if _, err := ready.Read(make([]byte, 8)); err != nil {
			t.Fatal(err)
		}
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
		if len(report.Processes) != 1 {
			t.Fatalf("descendant not observed: %+v", report.Processes)
		}
		// The shell reaps the killed child; until then signal 0 still reaches the zombie.
		for deadline := time.Now().Add(5 * time.Second); syscall.Kill(report.Processes[0].PID, 0) == nil; time.Sleep(descendantInterval) {
			if time.Now().After(deadline) {
				t.Fatalf("observed descendant still present: %+v", report.Processes)
			}
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
		if err != nil || !report.Absent || calls < 4 {
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
	t.Run("AHI-032 a blind interval beyond the observation bound fails", func(t *testing.T) {
		done := make(chan struct{})
		close(done)
		o := descendantObserver{root: os.Getpid(), known: map[int]ObservedProcess{}, stop: make(chan struct{}), done: done, missed: 9, seen: time.Now().Add(-descendantMaxGap - time.Second), snap: func(context.Context) (map[int]ObservedProcess, error) {
			return map[int]ObservedProcess{}, nil
		}}
		report, err := o.finish()
		if err == nil || report.Absent || len(report.Failures) != 1 || !strings.Contains(report.Failures[0], "observation bound") {
			t.Fatalf("unobserved run passed: %v %+v", err, report)
		}
	})
}

func TestV10689SnapshotParsing(t *testing.T) {
	valid := "10 1 Sat Oct 3 16:00:00 2026 S\n"
	for _, tc := range []struct {
		name, data string
		valid      bool
	}{
		{"Darwin", valid, true},
		{"Linux zombie", "11 10 Sat Oct 3 16:00:00 2026 Z+\n", true},
		{"duplicate same identity", valid + valid, false},
		{"duplicate different identity", valid + "10 1 Sat Oct 3 16:00:01 2026 S\n", false},
		{"malformed after valid", valid + "broken\n", false},
		{"missing", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := parseDescendantSnapshot([]byte(tc.data))
			if tc.valid {
				if err != nil || len(rows) != 1 {
					t.Fatalf("valid table refused: %v", err)
				}
			} else if err == nil || rows != nil {
				t.Fatal("partial or ambiguous table accepted")
			}
		})
	}
}

// v10689Snapshot gives every fixture scan the same explicit bound as the
// production observer, including scans before readiness and after cancellation.
func v10689Snapshot(parent context.Context) (map[int]ObservedProcess, error) {
	ctx, cancel := context.WithTimeout(parent, descendantSnapshotTimeout)
	defer cancel()
	// A deterministic blocked-snapshot surrogate exercises timeout and signal
	// teardown without relying on an operating-system ps hang.
	if os.Getenv("CORVINT_V10689_BLOCK_SNAPSHOT") == "1" {
		if err := os.WriteFile(filepath.Join(os.Getenv("CORVINT_V10689_DIR"), "snapshot-blocked"), []byte("ready\n"), 0600); err != nil {
			return nil, err
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return descendantSnapshot(ctx)
}

// v10689Wait always has one waiter. os.Process.Kill synchronizes with Wait's
// process state; this never sends a raw PID/PGID signal after a completed wait.
func v10689Wait(command *exec.Cmd, grace time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
	}
	killErr := command.Process.Kill()
	timer.Reset(2 * time.Second)
	select {
	case err := <-done:
		return errors.Join(errors.New("fixture wait exceeded grace; forced child joined"), killErr, err)
	case <-timer.C:
		return errors.Join(errors.New("fixture child wait unresolved after forced termination"), killErr)
	}
}

// TestV10689CancellationHelper supplies explicit readiness and completed child
// waits. The long child has a separate session; the sentinel is a sibling.
func TestV10689CancellationHelper(t *testing.T) {
	role := os.Getenv("CORVINT_V10689_ROLE")
	if role == "" {
		return
	}
	if role == "long" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if role == "short" {
		rows, err := v10689Snapshot(context.Background())
		if err != nil {
			os.Exit(2)
		}
		if err := json.NewEncoder(os.Stdout).Encode(rows[os.Getpid()]); err != nil {
			os.Exit(3)
		}
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
	dir := os.Getenv("CORVINT_V10689_DIR")
	lifetime, end := context.WithTimeout(context.Background(), 20*time.Second)
	defer end()
	ctx, stop := signal.NotifyContext(lifetime, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestV10689CancellationHelper$")
	child.Env = append(os.Environ(), "CORVINT_V10689_ROLE=long")
	child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := child.Start(); err != nil {
		os.Exit(4)
	}
	joined := false
	defer func() {
		if !joined {
			_ = child.Process.Kill()
			waitErr := v10689Wait(child, 2*time.Second)
			if waitErr != nil {
				if _, ok := waitErr.(*exec.ExitError); !ok {
					t.Error(waitErr)
					return
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "long-joined"), []byte("joined\n"), 0600); err != nil {
				t.Error(err)
			}
		}
	}()
	if err := os.WriteFile(filepath.Join(dir, "long-started.json"), []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := v10689Snapshot(ctx)
	if err != nil {
		return
	}
	owned, ok := rows[child.Process.Pid]
	if !ok {
		return
	}
	b, _ := json.Marshal(owned)
	if os.WriteFile(filepath.Join(dir, "long.json"), b, 0600) != nil {
		return
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "observed")); err == nil {
			break
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	var shorts []ObservedProcess
	for n := 0; n < 4; n++ {
		short := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestV10689CancellationHelper$")
		short.Env = append(os.Environ(), "CORVINT_V10689_ROLE=short")
		output, err := short.StdoutPipe()
		if err != nil {
			return
		}
		input, err := short.StdinPipe()
		if err != nil {
			return
		}
		if err := short.Start(); err != nil {
			return
		}
		var acknowledged ObservedProcess
		decodeErr := json.NewDecoder(output).Decode(&acknowledged)
		_ = input.Close()
		waitErr := v10689Wait(short, 2*time.Second)
		if decodeErr != nil || waitErr != nil {
			return
		}
		shorts = append(shorts, acknowledged)
	}
	b, _ = json.Marshal(shorts)
	if os.WriteFile(filepath.Join(dir, "shorts-joined.json"), b, 0600) != nil {
		return
	}
	waitErr := v10689Wait(child, 20*time.Second)
	joined = true
	if waitErr != nil {
		if _, ok := waitErr.(*exec.ExitError); !ok {
			t.Error(waitErr)
			return
		}
	}
	_ = os.WriteFile(filepath.Join(dir, "long-joined"), []byte("joined\n"), 0600)
}

func TestV10689BoundedCancellation(t *testing.T) {
	ctx, cancel := signal.NotifyContext(t.Context(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	dir := t.TempDir()
	sentinel := exec.Command("/bin/sleep", "30")
	if err := sentinel.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = sentinel.Process.Kill()
		if err := v10689Wait(sentinel, 2*time.Second); err != nil {
			if _, ok := err.(*exec.ExitError); !ok {
				t.Error(err)
			}
		}
	}()
	rows, err := v10689Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sentinelID, ok := rows[sentinel.Process.Pid]
	if !ok {
		t.Fatal("sentinel identity unavailable")
	}
	root := exec.Command(os.Args[0], "-test.run=^TestV10689CancellationHelper$")
	root.Env = append(os.Environ(), "CORVINT_V10689_ROLE=parent", "CORVINT_V10689_DIR="+dir)
	if err := root.Start(); err != nil {
		t.Fatal(err)
	}
	rootJoined := false
	var observer *descendantObserver
	defer func() {
		if observer != nil {
			_, _ = observer.finish()
		}
		if !rootJoined {
			_ = root.Process.Signal(syscall.SIGTERM)
			if err := v10689Wait(root, 2*time.Second); err != nil {
				if _, ok := err.(*exec.ExitError); !ok {
					t.Error(err)
				}
			}
		}
	}()
	waitFile := func(name string) []byte {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) && ctx.Err() == nil {
			b, e := os.ReadFile(filepath.Join(dir, name))
			if e == nil && json.Valid(b) {
				return b
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("bounded readiness failed: %s", name)
		return nil
	}
	var child ObservedProcess
	if err := json.Unmarshal(waitFile("long.json"), &child); err != nil {
		t.Fatal(err)
	}
	pgid, err := syscall.Getpgid(child.PID)
	if err != nil || pgid != child.PID {
		t.Fatalf("child session not independent: %d %v", pgid, err)
	}
	observer, err = startDescendantObserver(root.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	acknowledged := false
	for time.Now().Before(deadline) && ctx.Err() == nil {
		observer.mu.Lock()
		owned, exists := observer.known[child.PID]
		observer.mu.Unlock()
		if exists && owned.Start == child.Start {
			acknowledged = true
			break
		}
		time.Sleep(descendantInterval)
	}
	if !acknowledged {
		t.Fatal("observer did not acknowledge exact escaped child identity")
	}
	if err := os.WriteFile(filepath.Join(dir, "observed"), []byte("ack\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var shorts []ObservedProcess
	if err := json.Unmarshal(waitFile("shorts-joined.json"), &shorts); err != nil || len(shorts) != 4 {
		t.Fatalf("short child joins missing: %v", err)
	}
	rows, err = v10689Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current, present := rows[child.PID]; !present || current.Start != child.Start || strings.HasPrefix(current.State, "Z") {
		t.Fatal("escaped child exited before cancellation")
	}
	// Cancellation starts final observation/cleanup after exact identity readiness.
	cancel()
	report, err := observer.finish()
	observer = nil
	if err != nil || !report.Absent {
		t.Fatalf("cancelled observer cleanup failed: %v %+v", err, report)
	}
	if err := v10689Wait(root, 2*time.Second); err != nil {
		rootJoined = true
		t.Fatal(err)
	}
	rootJoined = true
	if _, err := os.Stat(filepath.Join(dir, "long-joined")); err != nil {
		t.Fatal("helper did not join escaped child")
	}
	rows, err = v10689Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, owned := range append(shorts, child) {
		if current, exists := rows[owned.PID]; exists && current.Start == owned.Start {
			t.Fatalf("joined child still present: %+v", current)
		}
	}
	if _, exists := rows[root.Process.Pid]; exists {
		t.Fatal("joined root still present")
	}
	if current, exists := rows[sentinelID.PID]; !exists || current.Start != sentinelID.Start || strings.HasPrefix(current.State, "Z") {
		t.Fatal("unrelated sentinel changed")
	}
	t.Logf("owned root=%d escaped=%+v shortJoined=%+v sentinel=%+v preserved; all owned waits completed", root.Process.Pid, child, shorts, sentinelID)
}

func TestV10689BlockedStartupJoins(t *testing.T) {
	for _, interrupt := range []bool{false, true} {
		t.Run(fmt.Sprint("signal=", interrupt), func(t *testing.T) {
			dir := t.TempDir()
			root := exec.Command(os.Args[0], "-test.run=^TestV10689CancellationHelper$")
			root.Env = append(os.Environ(), "CORVINT_V10689_ROLE=parent", "CORVINT_V10689_DIR="+dir, "CORVINT_V10689_BLOCK_SNAPSHOT=1")
			if err := root.Start(); err != nil {
				t.Fatal(err)
			}
			joined := false
			defer func() {
				if !joined {
					_ = root.Process.Signal(syscall.SIGTERM)
					if err := v10689Wait(root, 2*time.Second); err != nil {
						if _, ok := err.(*exec.ExitError); !ok {
							t.Error(err)
						}
					}
				}
			}()
			began := time.Now()
			deadline := began.Add(3 * time.Second)
			ready := false
			for time.Now().Before(deadline) {
				if _, err := os.Stat(filepath.Join(dir, "snapshot-blocked")); err == nil {
					ready = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !ready {
				t.Fatal("blocked-snapshot fixture never acknowledged startup")
			}
			b, err := os.ReadFile(filepath.Join(dir, "long-started.json"))
			if err != nil {
				t.Fatal(err)
			}
			childPID, err := strconv.Atoi(string(b))
			if err != nil {
				t.Fatal(err)
			}
			if interrupt {
				if err := root.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
			}
			err = v10689Wait(root, 3*time.Second)
			joined = true
			if err != nil {
				t.Fatal(err)
			}
			if time.Since(began) > 4*time.Second {
				t.Fatal("startup failure cleanup exceeded its bound")
			}
			if _, err := os.Stat(filepath.Join(dir, "long-joined")); err != nil {
				t.Fatal("escaped child was not joined during startup failure")
			}
			rows, err := v10689Snapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if _, present := rows[childPID]; present {
				t.Fatal("escaped child remains after failed startup")
			}
			if _, present := rows[root.Process.Pid]; present {
				t.Fatal("helper remains after failed startup")
			}
			t.Logf("blocked startup signal=%v root=%d escaped=%d both joined and absent", interrupt, root.Process.Pid, childPID)
		})
	}
}
