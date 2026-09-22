//go:build darwin || linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fakeProcessFault(root, mode string) {
	cmd := exec.Command("/bin/sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	must(true, cmd.Start())
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	must(true, os.WriteFile(filepath.Join(root, ".git", "fault-child"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600))
	if mode == "flood" {
		time.Sleep(raceScale * 60 * time.Millisecond)
		for {
			if _, err := os.Stdout.Write(make([]byte, 8192)); err != nil {
				return
			}
		}
	}
	time.Sleep(30 * time.Second)
}
func childPID(t *testing.T, root string) int {
	t.Helper()
	deadline := time.Now().Add(raceScale * 3 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(filepath.Join(root, ".git", "fault-child")); err == nil {
			pid, e := strconv.Atoi(string(raw))
			if e == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("separate-group child did not start")
	return 0
}
func ownedChild(t *testing.T, pid int) string {
	t.Helper()
	rows, err := processSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	birth := rows[pid].birth
	if birth == "" {
		t.Fatal("child identity unavailable")
	}
	t.Cleanup(func() {
		rows, err := processSnapshot(context.Background())
		if err == nil && sameProcess(rows[pid], birth) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	return birth
}
func assertChildGone(t *testing.T, pid int, birth string) {
	t.Helper()
	rows, err := processSnapshot(context.Background())
	if err != nil || sameProcess(rows[pid], birth) {
		t.Fatalf("separate group survived: pid=%d err=%v", pid, err)
	}
}
func TestSelfuseSeparateGroupsCleanedBeforeFallback(t *testing.T) {
	for _, mode := range []string{"hang", "flood"} {
		t.Run(mode, func(t *testing.T) {
			root, plan := fixture(t, mode)
			out := filepath.Join(t.TempDir(), "result")
			type result struct {
				receipt object
				failure any
			}
			done := make(chan result, 1)
			go func() {
				var r result
				defer func() { r.failure = recover(); done <- r }()
				r.receipt = investigate(context.Background(), options{root: root, plan: plan, out: out, corvint: must(os.Executable()), mode: "batch", timeout: raceScale * 500 * time.Millisecond, limit: 65536})
			}()
			pid := childPID(t, root)
			birth := ownedChild(t, pid)
			var r result
			select {
			case r = <-done:
			case <-time.After(raceScale * 8 * time.Second):
				t.Fatal("investigation did not finish")
			}
			if r.failure != nil {
				t.Fatal(r.failure)
			}
			if r.receipt["ok"] != true || obj(r.receipt["measurement"])["corvint_processes"] != 4 {
				t.Fatal(r.receipt)
			}
			assertChildGone(t, pid, birth)
			rows := obj(r.receipt["measurement"])["corvint_process_rows"].([]any)
			cleaned := obj(rows[0])["descendants_cleaned"].([]int)
			found := false
			for _, p := range cleaned {
				found = found || p == pid
			}
			if !found {
				t.Fatal("controlled child absent from cleanup evidence", cleaned)
			}
		})
	}
}
func TestSelfuseCancellationWritesReceiptAndCleansChild(t *testing.T) {
	root, plan := fixture(t, "hang")
	out := filepath.Join(t.TempDir(), "result")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan any, 1)
	go func() {
		defer func() { done <- recover() }()
		investigate(ctx, options{root: root, plan: plan, out: out, corvint: must(os.Executable()), mode: "batch", timeout: 10 * time.Second, limit: captureLimit})
	}()
	pid := childPID(t, root)
	birth := ownedChild(t, pid)
	cancel()
	select {
	case failure := <-done:
		if failure == nil || !strings.Contains(fmt.Sprint(failure), "interrupted") {
			t.Fatal(failure)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation exceeded bound")
	}
	assertChildGone(t, pid, birth)
	r := obj(parse(must(os.ReadFile(filepath.Join(out, "receipt.json")))))
	if r["ok"] != false || obj(r["failure"])["type"] != "native-investigation-failure" {
		t.Fatal(r)
	}
}
func TestDescendantSnapshotFailureAndExitedLeaderRemainUnverified(t *testing.T) {
	d := newDescendantCleanup()
	d.snapshot = func(context.Context) (map[int]processRow, error) {
		return nil, fmt.Errorf("controlled snapshot failure")
	}
	if d.stop(context.Background(), 123) == nil || d.failure == nil {
		t.Fatal("snapshot failure promoted")
	}
	d = newDescendantCleanup()
	d.snapshot = func(context.Context) (map[int]processRow, error) { return map[int]processRow{}, nil }
	if d.stop(context.Background(), 123) != nil || d.status != "NOT_OBSERVED: leader-exit-race" {
		t.Fatal(d)
	}
	rows := map[int]processRow{1: {parent: 0, birth: "one"}, 2: {parent: 1, birth: "two"}, 3: {parent: 2, birth: "three"}, 9: {parent: 0, birth: "nine"}}
	if len(descendants(rows, 1)) != 2 || sameProcess(processRow{birth: "reused"}, "old") {
		t.Fatal("ownership/identity mismatch")
	}
}

// TestDescendantsHandlesParentCycle guards against a ppid cycle from PID
// reuse: the parent chain 1 -> 6 -> 5 -> 1 loops back through the ancestor
// itself, making it its own (indirect) ancestor. The walk must still
// terminate and must not treat the ancestor as its own descendant.
func TestDescendantsHandlesParentCycle(t *testing.T) {
	rows := map[int]processRow{
		1: {parent: 6, birth: "one"},  // ancestor's parent is inside the cycle
		5: {parent: 1, birth: "five"}, // child of the ancestor
		6: {parent: 5, birth: "six"},  // 6's parent closes the loop back to 1
	}
	got := descendants(rows, 1)
	want := map[int]string{5: "five", 6: "six"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("descendants(cycle) = %v, want %v", got, want)
	}
}

func TestCLISignalWritesFailureAndLeavesNoChild(t *testing.T) {
	root, plan := fixture(t, "hang")
	out := filepath.Join(t.TempDir(), "result")
	bin := must(os.Executable())
	cmd := exec.Command(bin, "--selfuse-cli", "--root", root, "--plan", plan, "--out", out, "--corvint", bin, "--timeout", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	finished := false
	defer func() {
		if !finished {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
	}()
	pid := childPID(t, root)
	birth := ownedChild(t, pid)
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		finished = true
		if err == nil {
			t.Fatal("interrupted CLI returned success")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("signal cleanup exceeded bound")
	}
	assertChildGone(t, pid, birth)
	r := obj(parse(must(os.ReadFile(filepath.Join(out, "receipt.json")))))
	if r["ok"] != false || obj(r["failure"])["type"] != "native-investigation-failure" {
		t.Fatal(r)
	}
}
