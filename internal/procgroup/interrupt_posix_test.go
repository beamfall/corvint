//go:build darwin || linux

package procgroup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// readySignal closes ready on the first byte the child writes to stdout.
type readySignal struct {
	once  sync.Once
	ready chan struct{}
}

func (r *readySignal) Write(value []byte) (int, error) {
	r.once.Do(func() { close(r.ready) })
	return len(value), nil
}

// trapReadyBound is a hang detector for the shell's readiness line, not a
// performance budget (decision 0082).
const trapReadyBound = 30 * time.Second

// trapSpec runs a shell loop whose traps record which stop signal reached it.
// The shell prints once its traps are installed, and the beforeTimer hook,
// which run calls before it arms the run timer and outside AfterStart's
// one-second allowance, waits for that line, so the timeout never fires at a
// shell that has not installed its traps yet. TerminateGrace gives the TERM
// trap's fork and exec far longer than the 100 ms default before SIGKILL, so
// a loaded host cannot turn a SIGTERM stop into a SIGKILL one (V1-2446, as
// V1-2319 for TestTerminateGraceLetsSlowTermHandlerFinish). Pass both results
// to run.
func trapSpec(t *testing.T, directory, traps string) (Spec, processWaitChannels) {
	t.Helper()
	signal := &readySignal{ready: make(chan struct{})}
	spec := Spec{
		Argv:            []string{"/bin/sh", "-c", traps + "; echo ready; while :; do /bin/sleep 0.05; done"},
		Dir:             directory,
		Env:             []string{"PATH=/usr/bin:/bin", "MARK=" + filepath.Join(directory, "mark")},
		Timeout:         200 * time.Millisecond,
		ShutdownTimeout: 20 * time.Second,
		TerminateGrace:  10 * time.Second,
		StdoutTee:       signal,
	}
	return spec, processWaitChannels{beforeTimer: func() {
		timer := time.NewTimer(trapReadyBound)
		defer timer.Stop()
		select {
		case <-signal.ready:
		case <-timer.C:
			// Run still times out and stops the shell; the test then fails.
			t.Errorf("shell did not report its traps installed within %s", trapReadyBound)
		}
	}}
}

func markExists(directory string) bool {
	_, err := os.Stat(filepath.Join(directory, "mark"))
	return err == nil
}

// PWP-V0-030: without InterruptGrace the stop stays SIGTERM-first.
func TestPWPV0030ZeroInterruptGraceKeepsTerminateFirstStop(t *testing.T) {
	directory := t.TempDir()
	spec, channels := trapSpec(t, directory, `trap '/usr/bin/touch "$MARK"; exit 130' INT`)
	result := run(context.Background(), spec, channels)
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
	spec, channels := trapSpec(t, directory, `trap '/usr/bin/touch "$MARK"; exit 130' INT; trap 'exit 143' TERM`)
	spec.InterruptGrace = 5 * time.Second
	started := time.Now()
	result := run(context.Background(), spec, channels)
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
	spec, channels := trapSpec(t, directory, `trap '' INT; trap '/usr/bin/touch "$MARK"; exit 143' TERM`)
	spec.InterruptGrace = 300 * time.Millisecond
	result := run(context.Background(), spec, channels)
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

// blockingSink models a stopped reader: every write blocks until release.
type blockingSink struct {
	release    chan struct{}
	incomplete atomic.Bool
}

func (b *blockingSink) Write(value []byte) (int, error) {
	<-b.release
	return len(value), nil
}

func (b *blockingSink) TeeIncomplete() { b.incomplete.Store(true) }

// PWP-V0-028: a tee sink that never accepts a write holds neither its drain,
// the capture, nor the other stream's tee; Run returns with both pipes drained
// and the stuck sink is told its copy ended early.
func TestPWPV0028BlockedTeeSinkDoesNotHoldDrains(t *testing.T) {
	sink := &blockingSink{release: make(chan struct{})}
	defer close(sink.release)
	var stderrTee bytes.Buffer
	script := "/usr/bin/head -c 4200000 /dev/zero; /usr/bin/head -c 7000 /dev/zero >&2"
	spec := Spec{Argv: []string{"/bin/sh", "-c", script}, Dir: t.TempDir(), Timeout: 20 * time.Second,
		OutputLimit: 10, StderrLimit: 20, OverflowPolicy: OverflowTruncate, StdoutTee: sink, StderrTee: &stderrTee}
	started := time.Now()
	result := Run(context.Background(), spec)
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Fatalf("Run waited %v on a blocked tee sink", elapsed)
	}
	if result.Err != nil || result.ExitStatus != 0 || !result.PipesDrained || len(result.Stdout) != 10 || len(result.Stderr) != 20 {
		t.Fatalf("blocked tee changed the run: %+v", result)
	}
	if !sink.incomplete.Load() {
		t.Fatal("the blocked sink was not told its copy ended early")
	}
	if stderrTee.Len() != 7000 {
		t.Fatalf("the other stream's tee got %d bytes", stderrTee.Len())
	}
}

// PWP-V0-029: a signal whose send makes the leader exit at once is still the
// recorded stop signal; a signal sent after the observed exit is not.
func TestPWPV0029StopSignalSurvivesExitDuringSend(t *testing.T) {
	recorder := &stopSignalRecorder{}
	send := recorder.wrap(func(int, syscall.Signal) error {
		recorder.leaderGone.Store(true)
		return nil
	})
	if err := send(1, syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	if err := send(1, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if got := recorder.name(); got != "SIGINT" {
		t.Fatalf("stop signal = %q, want SIGINT", got)
	}
}
