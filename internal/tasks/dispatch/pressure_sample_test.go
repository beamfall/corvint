//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIssue497_SampleParsing(t *testing.T) {
	for _, row := range []struct {
		raw    string
		darwin bool
		want   float64
	}{{"{ 40.00 20.00 10.00 }", true, 40}, {"40.00 20.00 10.00 3/10 100", false, 40}} {
		got, err := parseLoad([]byte(row.raw), row.darwin)
		if err != nil || got != row.want {
			t.Fatal(got, err)
		}
	}
	for _, raw := range []string{"", "{ NaN 1 1 }", "{ Inf 1 1 }", "{ -1 1 1 }", "{ 1 1 }", "{ 1 1 1 } extra"} {
		if _, err := parseLoad([]byte(raw), true); err == nil {
			t.Fatal(raw)
		}
	}
	total, used, err := parseLinuxSwap([]byte("MemTotal: 999 kB\nSwapTotal: 100 kB\nSwapFree: 25 kB\n"))
	if err != nil || total != 102400 || used != 76800 {
		t.Fatal(total, used, err)
	}
	for _, raw := range []string{"", "SwapTotal: 1 kB", "SwapTotal: 1 kB\nSwapFree: 2 kB", "SwapTotal: 1 kB\nSwapTotal: 1 kB\nSwapFree: 0 kB", "SwapTotal: -1 kB\nSwapFree: 0 kB", "SwapTotal: 18446744073709551615 kB\nSwapFree: 0 kB", "SwapTotal: 1 MB\nSwapFree: 0 kB"} {
		if _, _, err := parseLinuxSwap([]byte(raw)); err == nil {
			t.Fatal(raw)
		}
	}
	total, used, err = parseLinuxSwap([]byte("SwapTotal: 0 kB\nSwapFree: 0 kB"))
	if err != nil {
		t.Fatal(err)
	}
	fraction, ok := (PressureSample{SwapKnown: true, SwapTotalBytes: total, SwapUsedBytes: used}).SwapFraction()
	if !ok || fraction != 0 {
		t.Fatal(fraction, ok)
	}
	if _, ok := (PressureSample{LoadKnown: true, CPUKnown: true, CPUs: 1, LoadAverage: math.Inf(1)}).LoadPerCPU(); ok {
		t.Fatal("nonfinite normalized load")
	}
}

func TestIssue497_BoundedFileReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metric")
	if err := os.WriteFile(path, []byte("1234"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := readPressureFile(context.Background(), osPressureOpen, path, 4)
	if err != nil || string(raw) != "1234" {
		t.Fatal(string(raw), err)
	}
	if _, err := readPressureFile(context.Background(), osPressureOpen, path, 3); err == nil {
		t.Fatal("overflow accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readPressureFile(ctx, osPressureOpen, path, 4); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := readPressureFile(context.Background(), osPressureOpen, path+"-missing", 4); err == nil {
		t.Fatal("missing file accepted")
	}
}

// The subprocess fixture is reached only with explicit trailing arguments.
// It records initialization before the lifecycle injection, so fixture failures
// cannot masquerade as cancellation/output-limit coverage.
func TestIssue497_CommandFixture(t *testing.T) {
	args := os.Args
	if len(args) < 4 || args[len(args)-3] != "pressure-fixture" {
		return
	}
	mode, path := args[len(args)-2], args[len(args)-1]
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(9)
	}
	switch mode {
	case "valid":
		fmt.Print("{ 1.00 2.00 3.00 }")
	case "stdout-limit":
		fmt.Print(strings.Repeat("x", pressureStdoutBytes+1))
	case "stderr-limit":
		fmt.Fprint(os.Stderr, strings.Repeat("x", pressureStderrBytes+1))
	case "sleep":
		time.Sleep(10 * time.Second)
	default:
		os.Exit(8)
	}
	os.Exit(0)
}

func TestIssue497_CommandLifecycle(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := func(mode, path string) []string {
		return []string{executable, "-test.run=^TestIssue497_CommandFixture$", "--", "pressure-fixture", mode, path}
	}
	path := filepath.Join(t.TempDir(), "valid")
	raw, err := runPressureCommand(context.Background(), argv("valid", path))
	if err != nil || !bytes.Equal(raw, []byte("{ 1.00 2.00 3.00 }")) {
		t.Fatal(string(raw), err)
	}
	if _, err := parseLoad(raw, true); err != nil {
		t.Fatal("fixture initialization/encoding", err)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatal("fixture path not reached", err)
	}
	for _, mode := range []string{"stdout-limit", "stderr-limit"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "reached")
			if _, err := runPressureCommand(context.Background(), argv(mode, path)); err == nil {
				t.Fatal("injected output cap not refused")
			}
			if _, err := os.ReadFile(path); err != nil {
				t.Fatal("injection not reached", err)
			}
		})
	}
	t.Run("cancel-and-join", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		path := filepath.Join(t.TempDir(), "reached")
		done := make(chan error, 1)
		go func() { _, err := runPressureCommand(ctx, argv("sleep", path)); done <- err }()
		deadline := time.Now().Add(time.Second)
		var pidRaw []byte
		for {
			// The fixture's WriteFile creates the file before writing the
			// PID, so wait for content rather than mere existence.
			if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
				pidRaw = raw
				break
			}
			if time.Now().After(deadline) {
				cancel()
				<-done
				t.Fatal("fixture never initialized")
			}
			time.Sleep(time.Millisecond)
		}
		pid, err := strconv.Atoi(string(pidRaw))
		if err != nil || pid <= 0 {
			t.Fatal("invalid fixture PID", err)
		}
		cancel()
		err = <-done
		if err == nil {
			t.Fatal("cancel accepted")
		}
		if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
			t.Fatalf("joined fixture still exists: %v", err)
		}
		// Reaching this point requires cmd.Run/Wait to have returned, including
		// joining the leader and closing its output pipes. No goroutine is abandoned.
	})
	t.Run("deadline-and-join", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "reached")
		start := time.Now()
		if _, err := runPressureCommand(context.Background(), argv("sleep", path)); err == nil {
			t.Fatal("deadline accepted")
		}
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal("deadline injection not reached", err)
		}
		if elapsed := time.Since(start); elapsed < pressureCommandTimeout || elapsed > pressureCommandTimeout+time.Second {
			t.Fatal("deadline bound", elapsed)
		}
	})
}

func TestIssue497_LiveSampler(t *testing.T) {
	if os.Getenv("CORVINT_ISSUE497_LIVE") != "1" {
		t.Skip("explicit platform qualification only")
	}
	s := SamplePressure(context.Background())
	load, loadOK := s.LoadPerCPU()
	signals, signalsOK := pressureSignals(issue497Config(), runtime.GOOS, s)
	if !loadOK || !signalsOK || s.SampledAt.IsZero() {
		t.Fatalf("live host metrics unavailable: %+v", s)
	}
	t.Logf("source=%s cpus=%d rawLoad=%g normalizedLoad=%g memorySignal=%s memoryPressureLevel=%d totalSwap=%d usedSwap=%d timestamp=%s", s.Source, s.CPUs, s.LoadAverage, load, signals[1].name, s.MemoryPressureLevel, s.SwapTotalBytes, s.SwapUsedBytes, s.SampledAt.Format(time.RFC3339Nano))
}
