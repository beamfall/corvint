//go:build unix

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
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

// pgroupHelperEnv makes the test binary act as a qualify caller: it runs
// failingTests over the fixture module the variable names and reports how
// that returned. The parent runs it in a process group of its own, so a
// signal that escapes failingTests's run ends the helper and not the parent.
const pgroupHelperEnv = "LINECOV_PGROUP_HELPER_ROOT"

func init() {
	root := os.Getenv(pgroupHelperEnv)
	if root == "" {
		return
	}
	_, _, err := failingTests(root, "example.com/m", []string{"p"}, time.Minute)
	fmt.Printf("helper-returned interrupted=%t err=%v\n", errors.Is(err, errQualifyInterrupted), err)
	os.Exit(0)
}

// signalModule is a module whose one test runs body.
func signalModule(t *testing.T, body string) string {
	root := t.TempDir()
	write(t, root, "go.mod", "module example.com/m\n\ngo 1.22\n")
	write(t, root, "p/p.go", "package p\n")
	write(t, root, "p/p_test.go", "package p\n\nimport (\n\t\"os\"\n\t\"strconv\"\n\t\"syscall\"\n\t\"testing\"\n\t\"time\"\n)\n\nvar _ = strconv.Itoa\nvar _ = os.Getpid\nvar _ = syscall.Getpid\nvar _ = time.Second\n\nfunc TestSignal(t *testing.T) {\n"+body+"\n}\n")
	return root
}

// startHelper starts this test binary as a qualify caller over root in a
// process group of its own.
func startHelper(t *testing.T, root string, extraEnv ...string) (*exec.Cmd, *strings.Builder) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(append(os.Environ(), pgroupHelperEnv+"="+root), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) })
	return cmd, &out
}

// GLS-V0-051: a mutant whose test signals its own process group, here
// kill(0, SIGTERM), must reach only the go test run it is part of. Before the
// run had a group of its own, the signal ended the qualify process (rc 143)
// and whatever shell shared its group.
func TestQualifyRunCannotSignalCaller(t *testing.T) {
	root := signalModule(t, "\t_ = syscall.Kill(0, syscall.SIGTERM)\n\ttime.Sleep(10 * time.Second)")
	cmd, out := startHelper(t, root)
	err := cmd.Wait()
	if err != nil {
		t.Fatalf("qualify caller did not survive its run's group signal: %v\n%s", err, out)
	}
	got := out.String()
	if !strings.Contains(got, "helper-returned interrupted=false") || !strings.Contains(got, "go test reported no result for p") {
		t.Fatalf("a signalled run must leave its package without a result (invalid mutant), got:\n%s", got)
	}
}

// GLS-V0-052: an isolated run no longer receives the caller's SIGTERM, so
// qualify must kill the run's whole group itself and stop.
func TestQualifyInterruptKillsRunGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	root := signalModule(t, "\t_ = os.WriteFile(os.Getenv(\"LINECOV_PGROUP_PID\"), []byte(strconv.Itoa(os.Getpid())), 0o600)\n\ttime.Sleep(time.Minute)")
	cmd, out := startHelper(t, root, "LINECOV_PGROUP_PID="+pidFile)
	var testPid int
	for deadline := time.Now().Add(2 * time.Minute); testPid == 0; {
		if data, err := os.ReadFile(pidFile); err == nil && len(data) > 0 {
			testPid, _ = strconv.Atoi(string(data))
			continue
		}
		if time.Now().After(deadline) {
			t.Fatalf("fixture test never started:\n%s", out)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	began := time.Now()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("qualify caller did not handle SIGTERM: %v\n%s", err, out)
	}
	if !strings.Contains(out.String(), "helper-returned interrupted=true") {
		t.Fatalf("an interrupted run must return errQualifyInterrupted, got:\n%s", out)
	}
	if elapsed := time.Since(began); elapsed > 30*time.Second {
		t.Fatalf("interrupted run took %v to stop", elapsed)
	}
	// The test binary, a grandchild in the run's group, must be gone; an
	// orphan zombie is reaped by init shortly after the group kill.
	for deadline := time.Now().Add(10 * time.Second); ; {
		if err := syscall.Kill(testPid, 0); errors.Is(err, syscall.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("test binary %d of the interrupted run still exists", testPid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
