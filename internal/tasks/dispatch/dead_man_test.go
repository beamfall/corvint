//go:build darwin || linux

package dispatch

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

// deadManHelperEnv names the directory where TestDeadManHelper records the
// worker group it launched before it waits to be killed.
const deadManHelperEnv = "CORVINT_DISPATCH_TEST_DEAD_MAN_HELPER"

// TestDeadManHelper runs only inside a child test binary: it launches a
// worker tree as the dispatch fixtures do, records its leader, and waits for
// the parent test to SIGKILL this binary.
func TestDeadManHelper(t *testing.T) {
	dir := os.Getenv(deadManHelperEnv)
	if dir == "" {
		t.Skip("helper for TestATRV0213_FixtureWorkersDieWithTheirTestBinary")
	}
	pid, id, _, err := launch([]string{"/bin/sh", "-c", "/bin/sleep 300 & /bin/sleep 300; wait"}, nil, dir, filepath.Join(dir, "logs"))
	if err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(dir, "worker.tmp")
	if err := os.WriteFile(tmp, []byte(fmt.Sprintf("%d %s\n", pid, id)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, "worker")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Minute)
}

// kernelNames maps each process to its kernel name (ps ucomm: the kernel
// comm on Linux, p_comm on Darwin). On Darwin it can differ from the argv[0]
// that observeProcs reads there: exec through a symlink names the process
// after the symlink's target (ATR-V0-700).
func kernelNames() (map[int]string, error) {
	ps := "/bin/ps"
	if _, err := os.Stat(ps); err != nil {
		ps = "/usr/bin/ps"
	}
	raw, err := exec.Command(ps, "-axo", "pid=,ucomm=").Output()
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		pid, err := strconv.Atoi(f[0])
		if err != nil {
			return nil, fmt.Errorf("unparsable process row %q", line)
		}
		out[pid] = strings.Join(f[1:], " ")
	}
	return out, nil
}

// ATR-V0-213 (V1-2189): a test binary killed by SIGKILL runs neither its
// cleanups nor TestMain, so only the dead-man reaper can stop the worker
// groups it launched. The worker group must be gone within a bounded wait
// after the binary dies.
func TestATRV0213_FixtureWorkersDieWithTheirTestBinary(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "child.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	child := exec.Command(exe, "-test.run=^TestDeadManHelper$", "-test.count=1", "-test.timeout=15m")
	child.Env = append(os.Environ(), deadManHelperEnv+"="+dir)
	child.Stdout, child.Stderr = out, out
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() { _ = child.Wait(); close(exited) }()
	t.Cleanup(func() {
		_ = child.Process.Kill()
		<-exited
	})
	// Wait on observed progress: the worker record appears or the child
	// exits first; the package timeout is the hang bound.
	var worker launchedGroup
	for worker.pid == 0 {
		raw, err := os.ReadFile(filepath.Join(dir, "worker"))
		if err == nil {
			if _, err := fmt.Sscan(string(raw), &worker.pid, &worker.id); err != nil || worker.pid <= 0 || worker.id == "" {
				t.Fatalf("worker record %q: %v", raw, err)
			}
			break
		}
		select {
		case <-exited:
			log, _ := os.ReadFile(filepath.Join(dir, "child.log"))
			t.Fatalf("helper exited before it launched a worker:\n%s", log)
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Cleanup(func() {
		if groupOwned(worker) {
			_ = syscall.Kill(-worker.pid, syscall.SIGKILL)
		}
	})
	if !groupOwned(worker) {
		t.Fatalf("worker group %d is not running under its recorded leader before the kill", worker.pid)
	}
	// ATR-V0-700 (V1-2815): the binary dies by a kill by name, as pkill does,
	// which SIGKILLs every process of the child's tree that carries the test
	// binary's name, by argv[0] (what observeProcs reads on Darwin) or by
	// kernel name. The reaper must carry neither, or it dies too and the
	// worker group is orphaned.
	procs, err := observeProcs()
	if err != nil {
		t.Fatal(err)
	}
	kernel, err := kernelNames()
	if err != nil {
		t.Fatal(err)
	}
	name, kname := procs[child.Process.Pid].comm, kernel[child.Process.Pid]
	var named []int
	reaper := 0
	for pid, p := range procs {
		if pid != child.Process.Pid && p.ppid != child.Process.Pid {
			continue
		}
		switch {
		case p.comm == name || kernel[pid] == kname:
			named = append(named, pid)
		case p.comm == deadManName && kernel[pid] == deadManName:
			reaper = pid
		case pid != worker.pid:
			t.Logf("child process %d is %q (kernel name %q)", pid, p.comm, kernel[pid])
		}
	}
	if name == "" || kname == "" || len(named) != 1 {
		t.Fatalf("processes named %q (kernel name %q) under the child: %v, want only the child", name, kname, named)
	}
	if reaper == 0 {
		var under []string
		for pid, p := range procs {
			if p.ppid == child.Process.Pid {
				under = append(under, fmt.Sprintf("%d %q (kernel name %q)", pid, p.comm, kernel[pid]))
			}
		}
		t.Fatalf("no process under the child is named %q by both argv[0] and kernel name: %v", deadManName, under)
	}
	for _, pid := range named {
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
	}
	<-exited
	// The bound is generous for a loaded host; the reaper acts at once on
	// end of file, and without it the group never empties.
	deadline := time.Now().Add(30 * time.Second)
	for groupPresent(worker.pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if groupPresent(worker.pid) {
		var members []string
		if procs, err := observeProcs(); err == nil {
			for pid, p := range procs {
				if p.pgid == worker.pid {
					members = append(members, fmt.Sprintf("%d(ppid %d %s)", pid, p.ppid, p.comm))
				}
			}
		}
		t.Fatalf("worker group %d outlived its SIGKILLed test binary (owned leader: %v, members %v)", worker.pid, groupOwned(worker), members)
	}
}
