//go:build darwin || linux

package procgroup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// trapSpec runs a shell loop whose traps record which stop signal reached it.
func trapSpec(t *testing.T, directory, traps string) Spec {
	t.Helper()
	return Spec{
		Argv:    []string{"/bin/sh", "-c", traps + "; while :; do /bin/sleep 0.05; done"},
		Dir:     directory,
		Env:     []string{"PATH=/usr/bin:/bin", "MARK=" + filepath.Join(directory, "mark")},
		Timeout: 200 * time.Millisecond,
	}
}

func markExists(directory string) bool {
	_, err := os.Stat(filepath.Join(directory, "mark"))
	return err == nil
}

// PWP-V0-030: without InterruptGrace the stop stays SIGTERM-first.
func TestPWPV0030ZeroInterruptGraceKeepsTerminateFirstStop(t *testing.T) {
	directory := t.TempDir()
	spec := trapSpec(t, directory, `trap '/usr/bin/touch "$MARK"; exit 130' INT`)
	result := Run(context.Background(), spec)
	if !result.TimedOut || markExists(directory) {
		t.Fatalf("default stop sent SIGINT or did not time out: %+v", result)
	}
	if result.StopSignal != "SIGTERM" || result.Signal != "terminated" || !result.OwnedProcessGroupCleanup {
		t.Fatalf("default stop record = %q/%q: %+v", result.StopSignal, result.Signal, result)
	}
}

// PWP-V0-030, PWP-V0-029: a positive grace sends SIGINT first, and a runner
// that exits on it ends the stop there and records SIGINT.
func TestPWPV0030InterruptFirstStopLetsRunnerExitOnSIGINT(t *testing.T) {
	directory := t.TempDir()
	spec := trapSpec(t, directory, `trap '/usr/bin/touch "$MARK"; exit 130' INT; trap 'exit 143' TERM`)
	spec.InterruptGrace = 5 * time.Second
	started := time.Now()
	result := Run(context.Background(), spec)
	if !result.TimedOut || !markExists(directory) {
		t.Fatalf("SIGINT did not reach the runner: %+v", result)
	}
	if result.StopSignal != "SIGINT" || result.ExitStatus != 130 || !result.ExitObserved || !result.OwnedProcessGroupCleanup {
		t.Fatalf("interrupt stop record = %q exit %d: %+v", result.StopSignal, result.ExitStatus, result)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("runner that exited on SIGINT waited out the grace: %s", elapsed)
	}
}

// PWP-V0-030: a runner that ignores SIGINT gets SIGTERM after the grace.
func TestPWPV0030InterruptGraceEscalatesToSIGTERM(t *testing.T) {
	directory := t.TempDir()
	spec := trapSpec(t, directory, `trap '' INT; trap '/usr/bin/touch "$MARK"; exit 143' TERM`)
	spec.InterruptGrace = 300 * time.Millisecond
	result := Run(context.Background(), spec)
	if !result.TimedOut || !markExists(directory) || result.StopSignal != "SIGTERM" || result.ExitStatus != 143 {
		t.Fatalf("escalation record = %q exit %d: %+v", result.StopSignal, result.ExitStatus, result)
	}
}

// PWP-V0-029: a runner that ends on its own records no stop signal.
func TestPWPV0029SelfExitRecordsNoStopSignal(t *testing.T) {
	spec := Spec{Argv: []string{"/bin/sh", "-c", "exit 3"}, Dir: t.TempDir(), Timeout: 5 * time.Second, InterruptGrace: time.Second}
	result := Run(context.Background(), spec)
	if result.StopSignal != "" || result.ExitStatus != 3 || result.TimedOut {
		t.Fatalf("self exit record = %q: %+v", result.StopSignal, result)
	}
}

func TestPWPV0030NegativeInterruptGraceIsInvalid(t *testing.T) {
	spec := Spec{Argv: []string{"/bin/sh", "-c", "exit 0"}, Dir: t.TempDir(), Timeout: time.Second, InterruptGrace: -time.Second}
	result := Run(context.Background(), spec)
	var failure *processError
	if result.Started || !errors.As(result.Err, &failure) || failure.Code != "process-spec-invalid" {
		t.Fatalf("negative grace = %+v", result)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("tee closed") }

// PWP-V0-028: tees receive every byte beyond the truncation bound, and a
// failing tee leaves the capture and the run intact.
func TestPWPV0028TeesReceiveUntruncatedStreams(t *testing.T) {
	script := "/usr/bin/head -c 10000 /dev/zero; /usr/bin/head -c 7000 /dev/zero >&2"
	var stdoutTee, stderrTee bytes.Buffer
	spec := Spec{Argv: []string{"/bin/sh", "-c", script}, Dir: t.TempDir(), Timeout: 10 * time.Second,
		OutputLimit: 10, StderrLimit: 20, OverflowPolicy: OverflowTruncate, StdoutTee: &stdoutTee, StderrTee: &stderrTee}
	result := Run(context.Background(), spec)
	if result.Err != nil || result.ExitStatus != 0 || len(result.Stdout) != 10 || len(result.Stderr) != 20 || !result.OutputOverflow {
		t.Fatalf("truncated capture = %+v", result)
	}
	if stdoutTee.Len() != 10000 || stderrTee.Len() != 7000 {
		t.Fatalf("tee bytes stdout=%d stderr=%d", stdoutTee.Len(), stderrTee.Len())
	}
	spec.StdoutTee, spec.StderrTee = failingWriter{}, failingWriter{}
	result = Run(context.Background(), spec)
	if result.Err != nil || result.ExitStatus != 0 || len(result.Stdout) != 10 || len(result.Stderr) != 20 {
		t.Fatalf("failing tee changed the run: %+v", result)
	}
}
