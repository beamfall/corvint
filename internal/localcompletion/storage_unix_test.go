//go:build unix

package localcompletion

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestAggregateCaptureSignalParentShim is a parent the orphan test kills: it
// starts its child, records the child pid in <pidfile>.<depth> and blocks until
// killed. At depth 1 the child is the capture helper; above it, another shim.
// The shim bounds its own life by its parent's first, so killing any level of
// the chain, the outer test process included, retires every level below it.
func TestAggregateCaptureSignalParentShim(t *testing.T) {
	pidFile := os.Getenv("CORVINT_CAPTURE_SIGNAL_SHIM_PIDFILE")
	if pidFile == "" {
		return
	}
	exitWhenCaptureParentGone(os.Getenv("CORVINT_CAPTURE_SIGNAL_PARENT"))
	depth, err := strconv.Atoi(os.Getenv("CORVINT_CAPTURE_SIGNAL_SHIM_DEPTH"))
	if err != nil || depth < 1 {
		t.Fatalf("invalid shim depth %q", os.Getenv("CORVINT_CAPTURE_SIGNAL_SHIM_DEPTH"))
	}
	self := "CORVINT_CAPTURE_SIGNAL_PARENT=" + strconv.Itoa(os.Getpid())
	child := exec.Command(os.Args[0], "-test.run=^TestAggregateCaptureSignalParentShim$")
	child.Env = append(os.Environ(), self, "CORVINT_CAPTURE_SIGNAL_SHIM_DEPTH="+strconv.Itoa(depth-1))
	if depth == 1 {
		child = exec.Command(os.Args[0], "-test.run=^TestAggregateCaptureSignalHelper$")
		child.Env = append(os.Environ(), self, "CORVINT_CAPTURE_SIGNAL_SHIM_PIDFILE=")
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	name := pidFile + "." + strconv.Itoa(depth)
	if err := os.WriteFile(name+".tmp", []byte(strconv.Itoa(child.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(name+".tmp", name); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	t.Fatal("shim child ended while its parent shim was alive")
}

// V1-1498: the signal-capture helper exits after its parent dies by SIGKILL
// without running any cleanup, instead of blocking in signal.NotifyContext.
// At depth 2 the killed process stands for the outer test process: the inner
// shim and the helper must both be retired.
func TestAggregateCaptureSignalHelperExitsWhenParentDies(t *testing.T) {
	for _, depth := range []int{1, 2} {
		t.Run(fmt.Sprintf("depth-%d", depth), func(t *testing.T) { captureChainRetiresWhenRootDies(t, depth) })
	}
}

func captureChainRetiresWhenRootDies(t *testing.T, depth int) {
	repo, saved, receipt, report := aggregatePublicationFixture(t)
	for _, v := range []struct {
		role string
		raw  []byte
	}{{"outcome", receipt}, {"report", report}} {
		if err := repo.publishAggregateRole(context.Background(), saved, v.role, v.raw); err != nil {
			t.Fatal(err)
		}
	}
	scratch := t.TempDir()
	ready, pidFile := filepath.Join(scratch, "ready"), filepath.Join(scratch, "child.pid")
	shim := exec.Command(os.Args[0], "-test.run=^TestAggregateCaptureSignalParentShim$")
	shim.Env = append(os.Environ(), "CORVINT_CAPTURE_SIGNAL_ROOT="+repo.auth.Root, "CORVINT_CAPTURE_SIGNAL_KEY="+repo.session, "CORVINT_CAPTURE_SIGNAL_MODE=check", "CORVINT_CAPTURE_SIGNAL_READY="+ready, "CORVINT_CAPTURE_SIGNAL_SHIM_PIDFILE="+pidFile, "CORVINT_CAPTURE_SIGNAL_SHIM_DEPTH="+strconv.Itoa(depth), "CORVINT_CAPTURE_SIGNAL_PARENT="+strconv.Itoa(os.Getpid()))
	var output bytes.Buffer
	shim.Stdout, shim.Stderr = &output, &output
	if err := shim.Start(); err != nil {
		t.Fatal(err)
	}
	shimDone := make(chan error, 1)
	go func() { shimDone <- shim.Wait() }()
	// descendants[i] is the pid the shim at depth i+1 recorded; the helper is
	// descendants[0].
	descendants := make([]int, depth)
	t.Cleanup(func() {
		_ = shim.Process.Kill()
		for _, pid := range descendants {
			if pid > 0 {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	recorded := func() bool {
		for i := range descendants {
			if descendants[i] == 0 {
				if raw, err := os.ReadFile(pidFile + "." + strconv.Itoa(i+1)); err == nil {
					descendants[i], _ = strconv.Atoi(string(raw))
				}
			}
			if descendants[i] == 0 {
				return false
			}
		}
		return true
	}
	hang := time.After(2 * time.Minute)
	for !recorded() || !fileExists(ready) {
		select {
		case err := <-shimDone:
			t.Fatalf("parent shim ended before the helper was ready: %v %s", err, output.String())
		case <-hang:
			t.Fatalf("helper never became ready: %s", output.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := shim.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	<-shimDone
	for _, pid := range descendants {
		for !processExited(t, pid) {
			select {
			case <-hang:
				t.Fatalf("process %d of the capture chain outlived its SIGKILLed root", pid)
			case <-time.After(20 * time.Millisecond):
			}
		}
	}
}

// processExited reports whether pid has exited. A reparented child that has
// exited stays a zombie until its new parent reaps it, and a PID 1 that never
// reaps (a bare container entrypoint) keeps it one, so a zombie counts as
// exited; any other state is a live process.
func processExited(t *testing.T, pid int) bool {
	t.Helper()
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	var state string
	if runtime.GOOS == "linux" {
		raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if errors.Is(err, os.ErrNotExist) {
			return true
		}
		if err != nil {
			t.Fatalf("read state of %d: %v", pid, err)
		}
		// The state is the first field after the parenthesised command name.
		end := bytes.LastIndexByte(raw, ')')
		if end < 0 {
			t.Fatalf("unparsed /proc/%d/stat: %q", pid, raw)
		}
		state = strings.TrimSpace(string(raw[end+1:]))
	} else {
		out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		state = strings.TrimSpace(string(out))
		if err != nil && state == "" {
			return true // ps exits non-zero when no such process remains
		}
	}
	return strings.HasPrefix(state, "Z")
}

// processExited counts an unreaped zombie as exited and a blocked live process
// as alive, so the orphan test can fail only for a real survivor.
func TestCaptureProcessExitedTreatsZombieAsExited(t *testing.T) {
	live := exec.Command("/bin/cat")
	stdin, err := live.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := live.Start(); err != nil {
		t.Fatal(err)
	}
	pid := live.Process.Pid
	if processExited(t, pid) {
		t.Fatalf("a blocked live process %d reported exited", pid)
	}
	// Closing stdin ends cat. This process is its parent and does not reap it
	// until Wait below, so from its exit until then it is a zombie, and kill(pid,
	// 0) still succeeds on it.
	_ = stdin.Close()
	hang := time.After(2 * time.Minute)
	for !processExited(t, pid) {
		select {
		case <-hang:
			t.Fatalf("exited, unreaped process %d never reported exited", pid)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("process %d was reaped before Wait, so the zombie path went unchecked: %v", pid, err)
	}
	if err := live.Wait(); err != nil {
		t.Fatal(err)
	}
}
