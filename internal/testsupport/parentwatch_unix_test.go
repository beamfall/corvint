//go:build unix

package testsupport

import (
	"bytes"
	"errors"
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

const parentWatchRole = "CORVINT_TESTSUPPORT_PARENTWATCH_ROLE"

// TestParentWatchHelper is the shim and the helper of
// TestExitWhenParentGoneRetiresTheHelper; it does nothing in an ordinary run.
func TestParentWatchHelper(t *testing.T) {
	switch os.Getenv(parentWatchRole) {
	case "shim":
		helper := exec.Command(os.Args[0], "-test.run=^TestParentWatchHelper$")
		helper.Env = append(os.Environ(), parentWatchRole+"=helper", ParentPIDEntry())
		if err := helper.Start(); err != nil {
			os.Exit(2)
		}
		pidFile := os.Getenv("CORVINT_TESTSUPPORT_PARENTWATCH_PIDFILE")
		if err := os.WriteFile(pidFile+".tmp", []byte(strconv.Itoa(helper.Process.Pid)), 0o600); err != nil || os.Rename(pidFile+".tmp", pidFile) != nil {
			os.Exit(2)
		}
		_ = helper.Wait()
		os.Exit(1)
	case "helper":
		ExitWhenParentGone()
		select {} // stands for a helper waiting on a signal its parent never sends
	}
}

// V1-1498: a helper that waits forever exits once its parent dies by SIGKILL,
// without any cleanup of the parent's running.
func TestExitWhenParentGoneRetiresTheHelper(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "helper.pid")
	shim := exec.Command(os.Args[0], "-test.run=^TestParentWatchHelper$")
	shim.Env = append(os.Environ(), parentWatchRole+"=shim", "CORVINT_TESTSUPPORT_PARENTWATCH_PIDFILE="+pidFile)
	if err := shim.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- shim.Wait() }()
	var helper int
	t.Cleanup(func() {
		_ = shim.Process.Kill()
		if helper > 0 {
			_ = syscall.Kill(helper, syscall.SIGKILL)
		}
	})
	hang := time.After(time.Minute)
	for helper == 0 {
		if raw, err := os.ReadFile(pidFile); err == nil {
			helper, _ = strconv.Atoi(string(raw))
			continue
		}
		select {
		case err := <-done:
			t.Fatalf("shim ended before its helper started: %v", err)
		case <-hang:
			t.Fatal("helper never started")
		case <-time.After(10 * time.Millisecond):
		}
	}
	time.Sleep(200 * time.Millisecond)
	if exited(t, helper) {
		t.Fatal("helper exited while its parent was alive")
	}
	if err := shim.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	<-done
	for !exited(t, helper) {
		select {
		case <-hang:
			t.Fatalf("helper %d outlived its SIGKILLed parent", helper)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// exited reports whether pid has exited; a reparented zombie that its new
// parent has not reaped yet counts as exited.
func exited(t *testing.T, pid int) bool {
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
		state = strings.TrimSpace(string(raw[bytes.LastIndexByte(raw, ')')+1:]))
	} else {
		out, err := exec.Command("/bin/ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
		state = strings.TrimSpace(string(out))
		if err != nil && state == "" {
			return true
		}
	}
	return strings.HasPrefix(state, "Z")
}
