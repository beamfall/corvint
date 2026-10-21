//go:build unix

package localcompletion

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestAggregateCaptureSignalParentShim is the parent the orphan test kills: it
// starts the capture helper, records the helper pid and blocks until killed.
func TestAggregateCaptureSignalParentShim(t *testing.T) {
	pidFile := os.Getenv("CORVINT_CAPTURE_SIGNAL_SHIM_PIDFILE")
	if pidFile == "" {
		return
	}
	helper := exec.Command(os.Args[0], "-test.run=^TestAggregateCaptureSignalHelper$")
	helper.Env = append(os.Environ(), "CORVINT_CAPTURE_SIGNAL_SHIM_PIDFILE=", "CORVINT_CAPTURE_SIGNAL_PARENT="+strconv.Itoa(os.Getpid()))
	if err := helper.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidFile+".tmp", []byte(strconv.Itoa(helper.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(pidFile+".tmp", pidFile); err != nil {
		t.Fatal(err)
	}
	_ = helper.Wait()
	t.Fatal("capture helper ended while its parent shim was alive")
}

// V1-1498: the signal-capture helper exits after its parent dies by SIGKILL
// without running any cleanup, instead of blocking in signal.NotifyContext.
func TestAggregateCaptureSignalHelperExitsWhenParentDies(t *testing.T) {
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
	ready, pidFile := filepath.Join(scratch, "ready"), filepath.Join(scratch, "helper.pid")
	shim := exec.Command(os.Args[0], "-test.run=^TestAggregateCaptureSignalParentShim$")
	shim.Env = append(os.Environ(), "CORVINT_CAPTURE_SIGNAL_ROOT="+repo.auth.Root, "CORVINT_CAPTURE_SIGNAL_KEY="+repo.session, "CORVINT_CAPTURE_SIGNAL_MODE=check", "CORVINT_CAPTURE_SIGNAL_READY="+ready, "CORVINT_CAPTURE_SIGNAL_SHIM_PIDFILE="+pidFile)
	var output bytes.Buffer
	shim.Stdout, shim.Stderr = &output, &output
	if err := shim.Start(); err != nil {
		t.Fatal(err)
	}
	shimDone := make(chan error, 1)
	go func() { shimDone <- shim.Wait() }()
	helperPID := 0
	t.Cleanup(func() {
		_ = shim.Process.Kill()
		if helperPID > 0 {
			_ = syscall.Kill(helperPID, syscall.SIGKILL)
		}
	})
	hang := time.After(2 * time.Minute)
	for helperPID == 0 || !fileExists(ready) {
		if helperPID == 0 {
			if raw, err := os.ReadFile(pidFile); err == nil {
				helperPID, _ = strconv.Atoi(string(raw))
			}
		}
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
	for syscall.Kill(helperPID, 0) == nil {
		select {
		case <-hang:
			t.Fatalf("capture helper %d outlived its SIGKILLed parent", helperPID)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
